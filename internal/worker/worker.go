package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bobarin/episod/internal/db"
	"github.com/bobarin/episod/internal/models"
	"github.com/bobarin/episod/internal/queue"
	"github.com/bobarin/episod/internal/services"
	"github.com/bobarin/episod/internal/storage"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

type Worker struct {
	db                  *db.DB
	queue               *queue.Queue
	storage             *storage.Storage
	openai              *services.OpenAIService
	tts                 services.TTSService       // TTS provider (ElevenLabs preferred, Cartesia legacy)
	gemini              *services.GeminiService
	veo                 *services.VeoService      // Optional: nil when VEO_ENABLED=false (legacy)
	xaiVideo            *services.XAIVideoService // Optional: nil when XAI_VIDEO_ENABLED=false
	ffmpeg              *services.FFmpegService
	backgroundMusicPath string // Path to background music file (empty = no music)

	// Per-service semaphores — prevents rate-limit errors and resource exhaustion
	// when multiple clips process concurrently. Each semaphore bounds the number
	// of in-flight requests to that provider across all goroutines.
	uploadSem chan struct{} // Supabase Storage uploads (bound: 3)
	geminiSem chan struct{} // Gemini image generation (bound: 2)
	ttsSem    chan struct{} // TTS API calls (bound: 4)
	xaiSem    chan struct{} // xAI video generation (bound: 2)
	renderSem chan struct{} // FFmpeg render processes (bound: 2 — CPU intensive)
}

func New(
	database *db.DB,
	q *queue.Queue,
	stor *storage.Storage,
	openaiSvc *services.OpenAIService,
	ttsSvc services.TTSService,
	geminiSvc *services.GeminiService,
	veoSvc *services.VeoService,
	xaiVideoSvc *services.XAIVideoService,
	ffmpegSvc *services.FFmpegService,
	backgroundMusicPath string,
) *Worker {
	return &Worker{
		db:                  database,
		queue:               q,
		storage:             stor,
		openai:              openaiSvc,
		tts:                 ttsSvc,
		gemini:              geminiSvc,
		veo:                 veoSvc,
		xaiVideo:            xaiVideoSvc,
		ffmpeg:              ffmpegSvc,
		backgroundMusicPath: backgroundMusicPath,
		uploadSem:           make(chan struct{}, 3), // Supabase concurrent uploads
		geminiSem:           make(chan struct{}, 2), // Gemini image gen (heavy, rate-limited)
		ttsSem:              make(chan struct{}, 4), // TTS calls (lightweight, higher throughput)
		xaiSem:              make(chan struct{}, 2), // xAI video gen (long-running, quota-sensitive)
		renderSem:           make(chan struct{}, 2), // FFmpeg renders (CPU/RAM intensive)
	}
}

// withSemaphore wraps a function call with a semaphore to bound concurrency.
// It acquires a slot, runs fn, and releases the slot when done.
// If the context is cancelled while waiting, it returns immediately.
func (w *Worker) withSemaphore(ctx context.Context, sem chan struct{}, label string, fn func() error) error {
	log.Printf("[%s] waiting for slot...", label)
	select {
	case sem <- struct{}{}:
		// Acquired slot
	case <-ctx.Done():
		return fmt.Errorf("%s cancelled while waiting for slot: %w", label, ctx.Err())
	}
	defer func() { <-sem }()

	log.Printf("[%s] acquired slot, running...", label)
	return fn()
}

// uploadWithLimit wraps an upload call with the upload semaphore.
func (w *Worker) uploadWithLimit(ctx context.Context, label string, fn func() error) error {
	return w.withSemaphore(ctx, w.uploadSem, "Upload:"+label, fn)
}

// Start begins processing jobs from all queues
func (w *Worker) Start(ctx context.Context, concurrency int) {
	log.Printf("Worker started with concurrency: %d", concurrency)

	// Start workers for each queue type
	for i := 0; i < concurrency; i++ {
		go w.processQueue(ctx, queue.QueueGeneratePlan, w.handleGeneratePlan)
		go w.processQueue(ctx, queue.QueueProcessClip, w.handleProcessClip)
		go w.processQueue(ctx, queue.QueueRenderFinal, w.handleRenderFinal)
	}

	<-ctx.Done()
	log.Println("Worker shutting down...")
}

func (w *Worker) processQueue(ctx context.Context, queueName string, handler func(context.Context, *queue.Job) error) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			job, err := w.queue.Dequeue(ctx, queueName, 5*time.Second)
			if err != nil {
				log.Printf("Error dequeuing from %s: %v", queueName, err)
				continue
			}

			if job == nil {
				continue // No job available, retry
			}

			log.Printf("Processing job %s (type: %s, project: %s)", job.ID, job.Type, job.ProjectID)

			// Update job status to running
			if err := w.db.UpdateJobStatus(ctx, job.ID, models.JobStatusRunning); err != nil {
				log.Printf("Failed to update job status: %v", err)
			}

			// Handle the job
			if err := handler(ctx, job); err != nil {
				log.Printf("Job %s failed: %v", job.ID, err)
				w.db.UpdateJobError(ctx, job.ID, err.Error())
			} else {
				log.Printf("Job %s completed successfully", job.ID)
				w.db.UpdateJobStatus(ctx, job.ID, models.JobStatusSucceeded)
			}
		}
	}
}

// handleGeneratePlan generates the video plan, creates clip records,
// and enqueues process_clip jobs for each clip (images generated independently per clip)
func (w *Worker) handleGeneratePlan(ctx context.Context, job *queue.Job) error {
	log.Printf("Generating plan for project %s", job.ProjectID)

	// Update project status
	if err := w.db.UpdateProjectStatus(ctx, job.ProjectID, models.ProjectStatusPlanning); err != nil {
		return fmt.Errorf("failed to update project status: %w", err)
	}

	// Get project details
	project, err := w.db.GetProject(ctx, job.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Get series guidance if applicable
	var seriesGuidance *string
	if project.SeriesID != nil {
		// Future: fetch series guidance
	}

	// Load graphics preset for plan generation
	var planPreset *models.GraphicsPreset
	if project.GraphicsPresetID != nil {
		planPreset, err = w.db.GetGraphicsPreset(ctx, *project.GraphicsPresetID)
		if err != nil {
			return fmt.Errorf("failed to get graphics preset for plan: %w", err)
		}
	}

	// Build per-project plan options from the project's customization fields
	planOpts := &services.PlanOptions{
		Tone:        project.Tone,
		Preset:      planPreset,
		AspectRatio: project.AspectRatio,
		CTA:         project.CTA,
		Language:    project.Language,
	}

	// Generate plan with OpenAI
	plan, err := w.openai.GeneratePlan(ctx, project.Topic, project.TargetDurationSeconds, seriesGuidance, planOpts)
	if err != nil {
		w.db.UpdateProjectError(ctx, job.ProjectID, "plan_generation_failed", err.Error())
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	// Store plan as JSON asset
	planJSON, _ := json.MarshalIndent(plan, "", "  ")
	planAsset := &models.Asset{
		ID:            uuid.New(),
		ProjectID:     job.ProjectID,
		Type:          models.AssetTypePlanJSON,
		StorageBucket: w.storage.Bucket,
		StoragePath:   w.storage.GenerateStoragePath(job.ProjectID, "plan.json"),
		ContentType:   strPtr("application/json"),
		ByteSize:      int64Ptr(int64(len(planJSON))),
	}

	if err := w.uploadWithLimit(ctx, "plan.json", func() error {
		return w.storage.Upload(ctx, planAsset.StoragePath, planJSON, "application/json")
	}); err != nil {
		return fmt.Errorf("failed to upload plan: %w", err)
	}

	if err := w.db.CreateAsset(ctx, planAsset); err != nil {
		return fmt.Errorf("failed to save plan asset: %w", err)
	}

	// Create clip records and enqueue process_clip for each
	for i, clipPlan := range plan.Clips {
		clip := &models.Clip{
			ID:                    uuid.New(),
			ProjectID:             job.ProjectID,
			ClipIndex:             i,
			Script:                clipPlan.Script,
			VoiceStyleInstruction: &clipPlan.VoiceStyleInstruction,
			ImagePrompt:           clipPlan.ImagePrompt,
			VideoPrompt:           &clipPlan.VideoPrompt,
			EstimatedDurationSec:  intPtr(clipPlan.EstimatedDurationSec),
			Status:                models.ClipStatusPending,
		}

		if err := w.db.CreateClip(ctx, clip); err != nil {
			return fmt.Errorf("failed to create clip: %w", err)
		}

		// Enqueue process_clip immediately — images are generated independently per clip
		clipJobID := uuid.New()
		clipJob := &models.Job{
			ID:        clipJobID,
			ProjectID: job.ProjectID,
			ClipID:    &clip.ID,
			Type:      "process_clip",
			Status:    models.JobStatusQueued,
		}

		if err := w.db.CreateJob(ctx, clipJob); err != nil {
			return fmt.Errorf("failed to create clip job: %w", err)
		}

		if err := w.queue.EnqueueProcessClip(ctx, job.ProjectID, clip.ID, clipJobID); err != nil {
			return fmt.Errorf("failed to enqueue clip job: %w", err)
		}

		log.Printf("Enqueued process_clip for clip %d/%d (id: %s)", i+1, len(plan.Clips), clip.ID)
	}

	// Update project status to generating
	return w.db.UpdateProjectStatus(ctx, job.ProjectID, models.ProjectStatusGenerating)
}

// handleProcessClip processes a single clip: image generation, TTS, and video render
func (w *Worker) handleProcessClip(ctx context.Context, job *queue.Job) error {
	if job.ClipID == nil {
		return fmt.Errorf("clip ID missing")
	}

	log.Printf("Processing clip %s for project %s", *job.ClipID, job.ProjectID)

	// Get clip
	clip, err := w.db.GetClip(ctx, *job.ClipID)
	if err != nil {
		return fmt.Errorf("failed to get clip: %w", err)
	}

	// Get project
	project, err := w.db.GetProject(ctx, job.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to get project: %w", err)
	}

	// Get graphics preset
	var preset *models.GraphicsPreset
	if project.GraphicsPresetID != nil {
		preset, err = w.db.GetGraphicsPreset(ctx, *project.GraphicsPresetID)
		if err != nil {
			return fmt.Errorf("failed to get graphics preset: %w", err)
		}
	}

	// ─────────────────────────────────────────────────────────────────────
	// Concurrent pipelines: visual + audio run in parallel, then converge
	// at the render step which needs outputs from both.
	//
	// Pipeline A (visual): Image gen → Upload → xAI video gen
	// Pipeline B (audio):  TTS → Upload → Whisper transcription
	//
	// errgroup.WithContext gives us:
	//   - automatic context cancellation if either pipeline fails
	//   - clean error propagation (first error wins)
	//   - goroutine lifecycle management
	// ─────────────────────────────────────────────────────────────────────

	// Shared results — written by one goroutine each, read only after g.Wait()
	var (
		imageData      []byte
		imageAsset     *models.Asset
		aiVideoData    []byte // nil = use Ken Burns fallback
		audioAsset     *models.Asset
		audioData      []byte // raw TTS bytes (for Whisper)
		wordTimestamps []services.WordTimestamp
	)

	g, gctx := errgroup.WithContext(ctx)

	// Build per-project options from the project's customization fields
	imageOpts := &services.ImageGenOptions{
		AspectRatio: project.AspectRatio,
	}
	videoOpts := &services.VideoGenOptions{
		Preset:      preset,
		AspectRatio: project.AspectRatio,
	}
	// Per-project voice ID (empty string = use service default)
	projectVoiceID := ""
	if project.VoiceID != nil && *project.VoiceID != "" {
		projectVoiceID = *project.VoiceID
	}
	// Per-project language for Whisper transcription
	whisperLanguage := "en"
	if project.Language != nil && *project.Language != "" {
		whisperLanguage = *project.Language
	}

	// ── Pipeline A: Visual (image → upload → AI video) ─────────────────
	g.Go(func() error {
		// A1: Generate image (bounded by geminiSem)
		log.Printf("Clip %d: generating image...", clip.ClipIndex)
		if err := w.withSemaphore(gctx, w.geminiSem, fmt.Sprintf("Gemini:clip_%d", clip.ClipIndex), func() error {
			var genErr error
			imageData, genErr = w.gemini.GenerateImage(gctx, clip.ImagePrompt, preset, imageOpts)
			return genErr
		}); err != nil {
			w.db.UpdateClipError(gctx, clip.ID, fmt.Sprintf("Image generation failed: %v", err))
			return fmt.Errorf("failed to generate image: %w", err)
		}
		log.Printf("Clip %d: image generated (%d bytes), uploading...", clip.ClipIndex, len(imageData))

		// A2: Upload image to Supabase
		imageAsset = &models.Asset{
			ID:            uuid.New(),
			ProjectID:     job.ProjectID,
			ClipID:        &clip.ID,
			Type:          models.AssetTypeImage,
			StorageBucket: w.storage.Bucket,
			StoragePath:   w.storage.GenerateStoragePath(job.ProjectID, fmt.Sprintf("clip_%d_image.png", clip.ClipIndex)),
			ContentType:   strPtr("image/png"),
			ByteSize:      int64Ptr(int64(len(imageData))),
		}

		if err := w.uploadWithLimit(gctx, fmt.Sprintf("clip_%d_image", clip.ClipIndex), func() error {
			return w.storage.Upload(gctx, imageAsset.StoragePath, imageData, "image/png")
		}); err != nil {
			return fmt.Errorf("failed to upload image: %w", err)
		}

		if err := w.db.CreateAsset(gctx, imageAsset); err != nil {
			return fmt.Errorf("failed to save image asset: %w", err)
		}
		if err := w.db.UpdateClipImage(gctx, clip.ID, imageAsset.ID); err != nil {
			return fmt.Errorf("failed to update clip image: %w", err)
		}

		// A3: AI video generation (non-critical — failure falls back to Ken Burns)
		if w.xaiVideo != nil && clip.VideoPrompt != nil && *clip.VideoPrompt != "" {
			// Use the public URL for xAI image-to-video generation.
			// The Supabase bucket must be set to "public" in the dashboard.
			// Signed URLs can fail with 404 due to format/policy mismatches.
			imagePublicURL := w.storage.GetPublicURL(imageAsset.StoragePath)

			// Use estimated_duration_sec from the plan to control xAI video length.
			// This prevents generating video longer than needed (wasting xAI tokens).
			// xAI clamps this to 1-15s internally; 0 means use default (8s).
			xaiDuration := 0
			if clip.EstimatedDurationSec != nil {
				xaiDuration = *clip.EstimatedDurationSec
			}

			log.Printf("Clip %d: generating xAI video from image (url=%s, duration=%ds)...", clip.ClipIndex, imagePublicURL, xaiDuration)
			if xaiErr := w.withSemaphore(gctx, w.xaiSem, fmt.Sprintf("xAI:clip_%d", clip.ClipIndex), func() error {
				var genErr error
				aiVideoData, genErr = w.xaiVideo.GenerateVideo(gctx, *clip.VideoPrompt, imagePublicURL, xaiDuration, videoOpts)
				return genErr
			}); xaiErr != nil {
				log.Printf("Clip %d: xAI video generation failed, falling back to Ken Burns effects: %v", clip.ClipIndex, xaiErr)
				aiVideoData = nil
			} else {
				log.Printf("Clip %d: xAI video generated (%d bytes)", clip.ClipIndex, len(aiVideoData))
			}
		} else if w.veo != nil && clip.VideoPrompt != nil && *clip.VideoPrompt != "" {
			log.Printf("Clip %d: generating Veo video from image...", clip.ClipIndex)
			aiVideoData, err = w.veo.GenerateVideo(gctx, *clip.VideoPrompt, imageData, "image/png")
			if err != nil {
				log.Printf("Clip %d: Veo video generation failed, falling back to Ken Burns effects: %v", clip.ClipIndex, err)
				aiVideoData = nil
			} else {
				log.Printf("Clip %d: Veo video generated (%d bytes)", clip.ClipIndex, len(aiVideoData))
			}
		}

		return nil
	})

	// ── Pipeline B: Audio (TTS → upload → Whisper) ─────────────────────
	g.Go(func() error {
		// B1: Generate audio
		voiceStyle := "natural and engaging"
		if clip.VoiceStyleInstruction != nil {
			voiceStyle = *clip.VoiceStyleInstruction
		}

		log.Printf("Clip %d: generating audio...", clip.ClipIndex)
		var audioResp *services.TTSResponse
		if err := w.withSemaphore(gctx, w.ttsSem, fmt.Sprintf("TTS:clip_%d", clip.ClipIndex), func() error {
			var genErr error
			audioResp, genErr = w.tts.GenerateSpeech(gctx, clip.Script, voiceStyle, projectVoiceID)
			return genErr
		}); err != nil {
			w.db.UpdateClipError(gctx, clip.ID, fmt.Sprintf("TTS failed: %v", err))
			return fmt.Errorf("failed to generate audio: %w", err)
		}
		audioData = audioResp.AudioData
		log.Printf("Clip %d: audio generated (%d bytes)", clip.ClipIndex, len(audioData))

		// B2: Upload audio to Supabase
		audioAsset = &models.Asset{
			ID:            uuid.New(),
			ProjectID:     job.ProjectID,
			ClipID:        &clip.ID,
			Type:          models.AssetTypeAudio,
			StorageBucket: w.storage.Bucket,
			StoragePath:   w.storage.GenerateStoragePath(job.ProjectID, fmt.Sprintf("clip_%d_audio.mp3", clip.ClipIndex)),
			ContentType:   strPtr("audio/mpeg"),
			ByteSize:      int64Ptr(int64(len(audioData))),
		}

		if err := w.uploadWithLimit(gctx, fmt.Sprintf("clip_%d_audio", clip.ClipIndex), func() error {
			return w.storage.Upload(gctx, audioAsset.StoragePath, audioData, "audio/mpeg")
		}); err != nil {
			return fmt.Errorf("failed to upload audio: %w", err)
		}

		if err := w.db.CreateAsset(gctx, audioAsset); err != nil {
			return fmt.Errorf("failed to save audio asset: %w", err)
		}
		if err := w.db.UpdateClipAudio(gctx, clip.ID, audioAsset.ID, audioResp.DurationMs); err != nil {
			return fmt.Errorf("failed to update clip audio: %w", err)
		}

		// B3: Whisper transcription for subtitles (non-critical — failure is OK)
		log.Printf("Clip %d: transcribing audio for subtitles (lang=%s)...", clip.ClipIndex, whisperLanguage)
		wordTimestamps, err = w.openai.TranscribeAudio(gctx, audioData, whisperLanguage)
		if err != nil {
			log.Printf("Clip %d: WARNING — Whisper transcription failed, rendering without subtitles: %v", clip.ClipIndex, err)
			wordTimestamps = nil
		} else {
			log.Printf("Clip %d: transcribed %d words for subtitles", clip.ClipIndex, len(wordTimestamps))
		}

		return nil
	})

	// ── Wait for both pipelines to complete ────────────────────────────
	if err := g.Wait(); err != nil {
		return fmt.Errorf("clip processing failed: %w", err)
	}

	// ── Render: needs results from both pipelines (bounded by renderSem) ─
	log.Printf("Clip %d: both pipelines complete, rendering video...", clip.ClipIndex)

	if err := w.withSemaphore(ctx, w.renderSem, fmt.Sprintf("Render:clip_%d", clip.ClipIndex), func() error {
		return w.renderClip(ctx, job.ProjectID, clip.ID, audioData, imageData, aiVideoData, wordTimestamps)
	}); err != nil {
		w.db.UpdateClipError(ctx, clip.ID, fmt.Sprintf("Render failed: %v", err))
		return fmt.Errorf("failed to render clip: %w", err)
	}

	log.Printf("Clip %d: rendering complete", clip.ClipIndex)

	// Check if all clips are rendered, trigger final render
	allRendered, err := w.db.AreAllClipsRendered(ctx, job.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to check clip status: %w", err)
	}

	if allRendered {
		log.Printf("All clips rendered for project %s, enqueuing final render", job.ProjectID)

		finalJobID := uuid.New()
		finalJob := &models.Job{
			ID:        finalJobID,
			ProjectID: job.ProjectID,
			Type:      "render_final",
			Status:    models.JobStatusQueued,
		}

		if err := w.db.CreateJob(ctx, finalJob); err != nil {
			return fmt.Errorf("failed to create final render job: %w", err)
		}

		if err := w.queue.EnqueueRenderFinal(ctx, job.ProjectID, finalJobID); err != nil {
			return fmt.Errorf("failed to enqueue final render: %w", err)
		}

		w.db.UpdateProjectStatus(ctx, job.ProjectID, models.ProjectStatusRendering)
	}

	return nil
}

// renderClip renders a single clip video from image/video and audio.
//
// Two rendering paths:
//   - AI video path (aiVideoData != nil): combines the AI-generated video (xAI or Veo) with
//     narration audio. The video's native audio is discarded. If shorter than narration,
//     the last frame is frozen to fill the gap.
//   - Ken Burns path (aiVideoData == nil): applies zoom/pan motion effects + breathing pulse
//     to the still image, synced to the narration audio duration.
//
// In both paths:
//   - A 500ms silence buffer is prepended to the audio for natural pauses.
//   - If word timestamps are available, TikTok-style subtitles are burned into the video.
func (w *Worker) renderClip(ctx context.Context, projectID, clipID uuid.UUID, audioData, imageData, aiVideoData []byte, wordTimestamps []services.WordTimestamp) error {
	// Create temp file paths
	audioRawPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("audio_raw_%s.mp3", clipID.String()))
	audioPaddedPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("audio_padded_%s.mp3", clipID.String()))
	outputPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("clip_%s.mp4", clipID.String()))
	subtitlePath := w.ffmpeg.CreateTempFile(fmt.Sprintf("subs_%s.ass", clipID.String()))

	defer w.ffmpeg.Cleanup(audioRawPath, audioPaddedPath, outputPath, subtitlePath)

	// Write audio bytes directly to temp file — no re-download from storage needed
	// since we already have the TTS output in memory from Pipeline B.
	if err := os.WriteFile(audioRawPath, audioData, 0644); err != nil {
		return fmt.Errorf("failed to write audio file: %w", err)
	}

	// Prepend 500ms silence buffer so the first word isn't clipped
	// and there's a natural breathing pause between clips
	const silenceMs = 500
	silenceUsed := true
	if err := w.ffmpeg.PrependSilence(ctx, audioRawPath, audioPaddedPath, silenceMs); err != nil {
		log.Printf("Warning: could not prepend silence, using raw audio: %v", err)
		audioPaddedPath = audioRawPath
		silenceUsed = false
	}

	// Generate ASS subtitle file if word timestamps are available
	// The silence offset ensures subtitles align with the padded audio
	subtitleFile := "" // empty = no subtitles
	if len(wordTimestamps) > 0 {
		silenceOffsetSec := 0.0
		if silenceUsed {
			silenceOffsetSec = float64(silenceMs) / 1000.0
		}
		subParams := services.SubtitleParamsForResolution(w.ffmpeg.Resolution)
		if err := services.GenerateASSSubtitles(wordTimestamps, subtitlePath, silenceOffsetSec, subParams); err != nil {
			log.Printf("Warning: failed to generate subtitles, rendering without: %v", err)
		} else {
			subtitleFile = subtitlePath
			log.Printf("Generated TikTok-style subtitles (%d words, offset=%.1fs)", len(wordTimestamps), silenceOffsetSec)
		}
	}

	if aiVideoData != nil {
		// ── AI video path: xAI/Veo generated video + narration audio ───
		log.Printf("Rendering clip with AI video (%d bytes)", len(aiVideoData))

		aiVideoPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("aivideo_%s.mp4", clipID.String()))
		defer w.ffmpeg.Cleanup(aiVideoPath)

		if err := os.WriteFile(aiVideoPath, aiVideoData, 0644); err != nil {
			return fmt.Errorf("failed to write AI video file: %w", err)
		}

		if err := w.ffmpeg.RenderClipFromVideo(ctx, aiVideoPath, audioPaddedPath, outputPath, subtitleFile); err != nil {
			return fmt.Errorf("ffmpeg render from AI video failed: %w", err)
		}
	} else {
		// ── Ken Burns path: still image + motion effects ────────────────
		audioDurationMs, err := w.ffmpeg.GetAudioDuration(ctx, audioPaddedPath)
		if err != nil {
			log.Printf("Warning: could not get audio duration, estimating 10s: %v", err)
			audioDurationMs = 10000
		}

		effect := services.RandomEffect()
		log.Printf("Rendering clip with Ken Burns effect=%s, audioDuration=%dms", effect, audioDurationMs)

		imagePath := w.ffmpeg.CreateTempFile(fmt.Sprintf("image_%s.png", clipID.String()))
		defer w.ffmpeg.Cleanup(imagePath)

		if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
			return fmt.Errorf("failed to write image file: %w", err)
		}

		if err := w.ffmpeg.RenderClipWithEffect(ctx, imagePath, audioPaddedPath, outputPath, effect, audioDurationMs, subtitleFile); err != nil {
			return err
		}
	}

	// Measure actual rendered clip duration (for analytics — compare vs estimated to optimize xAI token usage)
	renderedDurationMs, err := w.ffmpeg.GetVideoDuration(ctx, outputPath)
	if err != nil {
		log.Printf("Warning: could not measure rendered clip duration: %v", err)
	} else {
		log.Printf("Clip rendered: actual duration = %dms", renderedDurationMs)
		if dbErr := w.db.UpdateClipRenderedDuration(ctx, clipID, renderedDurationMs); dbErr != nil {
			log.Printf("Warning: could not store rendered clip duration: %v", dbErr)
		}
	}

	// Read rendered video
	videoData, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to read rendered video: %w", err)
	}

	// Upload video
	videoAsset := &models.Asset{
		ID:            uuid.New(),
		ProjectID:     projectID,
		ClipID:        &clipID,
		Type:          models.AssetTypeClipVideo,
		StorageBucket: w.storage.Bucket,
		StoragePath:   w.storage.GenerateStoragePath(projectID, fmt.Sprintf("clip_%s.mp4", clipID.String())),
		ContentType:   strPtr("video/mp4"),
		ByteSize:      int64Ptr(int64(len(videoData))),
	}

	if err := w.uploadWithLimit(ctx, fmt.Sprintf("clip_%s_video", clipID.String()[:8]), func() error {
		return w.storage.Upload(ctx, videoAsset.StoragePath, videoData, "video/mp4")
	}); err != nil {
		return fmt.Errorf("failed to upload clip video: %w", err)
	}

	if err := w.db.CreateAsset(ctx, videoAsset); err != nil {
		return fmt.Errorf("failed to save video asset: %w", err)
	}

	return w.db.UpdateClipVideo(ctx, clipID, videoAsset.ID)
}

// handleRenderFinal concatenates all clips into final video
func (w *Worker) handleRenderFinal(ctx context.Context, job *queue.Job) error {
	log.Printf("Rendering final video for project %s", job.ProjectID)

	// Get all clips ordered by index
	clips, err := w.db.GetProjectClips(ctx, job.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to get clips: %w", err)
	}

	// Collect clip video paths
	var clipPaths []string
	for _, clip := range clips {
		if clip.ClipVideoAssetID == nil {
			return fmt.Errorf("clip %d has no video", clip.ClipIndex)
		}

		asset, err := w.db.GetAsset(ctx, *clip.ClipVideoAssetID)
		if err != nil {
			return fmt.Errorf("failed to get clip video asset: %w", err)
		}

		// Download clip video from storage
		videoData, err := w.storage.Download(ctx, asset.StoragePath)
		if err != nil {
			return fmt.Errorf("failed to download clip video: %w", err)
		}

		// Write to temp file
		tempPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("clip_%d.mp4", clip.ClipIndex))
		if err := os.WriteFile(tempPath, videoData, 0644); err != nil {
			return fmt.Errorf("failed to write clip video file: %w", err)
		}

		clipPaths = append(clipPaths, tempPath)
	}

	defer w.ffmpeg.Cleanup(clipPaths...)

	// Step 1: Concatenate all clips into one video
	concatPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("concat_%s.mp4", job.ProjectID.String()))
	defer w.ffmpeg.Cleanup(concatPath)

	if err := w.ffmpeg.ConcatenateClips(ctx, clipPaths, concatPath); err != nil {
		w.db.UpdateProjectError(ctx, job.ProjectID, "concat_failed", err.Error())
		return fmt.Errorf("failed to concatenate clips: %w", err)
	}

	// Step 2: Mix background music into the concatenated video
	// Music loops if shorter than video, and ends when the video ends
	outputPath := w.ffmpeg.CreateTempFile(fmt.Sprintf("final_%s.mp4", job.ProjectID.String()))
	defer w.ffmpeg.Cleanup(outputPath)

	if w.backgroundMusicPath != "" {
		if err := w.ffmpeg.MixBackgroundMusic(ctx, concatPath, w.backgroundMusicPath, outputPath); err != nil {
			// Music mixing failed — fall back to the concatenated video without music
			log.Printf("Warning: background music mixing failed, using video without music: %v", err)
			outputPath = concatPath
		}
	} else {
		// No music configured — use the concatenated video as-is
		outputPath = concatPath
	}

	// Read final video
	videoData, err := os.ReadFile(outputPath)
	if err != nil {
		return fmt.Errorf("failed to read final video: %w", err)
	}

	// Upload final video
	finalAsset := &models.Asset{
		ID:            uuid.New(),
		ProjectID:     job.ProjectID,
		Type:          models.AssetTypeFinalVideo,
		StorageBucket: w.storage.Bucket,
		StoragePath:   w.storage.GenerateStoragePath(job.ProjectID, "final.mp4"),
		ContentType:   strPtr("video/mp4"),
		ByteSize:      int64Ptr(int64(len(videoData))),
	}

	if err := w.uploadWithLimit(ctx, "final_video", func() error {
		return w.storage.Upload(ctx, finalAsset.StoragePath, videoData, "video/mp4")
	}); err != nil {
		w.db.UpdateProjectError(ctx, job.ProjectID, "upload_failed", err.Error())
		return fmt.Errorf("failed to upload final video: %w", err)
	}

	if err := w.db.CreateAsset(ctx, finalAsset); err != nil {
		return fmt.Errorf("failed to save final video asset: %w", err)
	}

	// Update project
	return w.db.SetProjectFinalVideo(ctx, job.ProjectID, finalAsset.ID)
}

// Helper functions
func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}
