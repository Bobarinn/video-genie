package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"google.golang.org/genai"
)

// ---------------------------------------------------------------------------
// Veo 3.1 Video Generation Service
// Uses the Google Gen AI SDK to generate videos from still images.
// The generated image is passed as the first frame, and the video_prompt
// from the clip describes the motion/action that should happen.
// ---------------------------------------------------------------------------

const (
	defaultVeoModel    = "veo-3.1-generate-preview"
	veoPollInterval    = 10 * time.Second
	veoMaxPollDuration = 5 * time.Minute // Max time to wait for a single video
)

// VeoService handles video generation via Google's Veo 3.1 model.
// It's optional — when nil or disabled, the worker falls back to
// Ken Burns motion effects on still images.
type VeoService struct {
	apiKey string
	model  string
}

// NewVeoService creates a new Veo video generation service.
// apiKey: the Gemini API key (same key works for both Gemini and Veo)
// model: the Veo model to use (empty string defaults to veo-3.1-generate-preview)
func NewVeoService(apiKey, model string) *VeoService {
	if model == "" {
		model = defaultVeoModel
	}
	return &VeoService{
		apiKey: apiKey,
		model:  model,
	}
}

// buildVeoPrompt enhances the raw video_prompt from OpenAI with Veo-specific
// instructions for style consistency and realistic, minimal motion.
// It also sanitizes celebrity name references to avoid Veo's safety filters.
func buildVeoPrompt(rawPrompt string) string {
	return fmt.Sprintf(`%s

Visual style direction: Match the hyperrealistic painting style of the input image exactly. Maintain the warm golden radiance, luminous cinematic atmosphere, and photorealistic subject detail from the source frame. Do NOT alter the art style, color grading, or rendering quality — the video should look like the painting has come to life.

Motion direction: Generate subtle, natural, realistic movement. Less is more — favor gentle, grounded motion over dramatic or exaggerated movement. Examples of good motion:
- Gentle breeze moving hair or fabric folds
- Subtle chest breathing or a slow blink
- Soft ambient particles (dust motes, light flicker, gentle smoke)
- Slow, barely perceptible camera drift or push-in
- Leaves rustling, water rippling, clouds drifting slowly
- Candle flames flickering, shadows shifting with light

Avoid: sudden jerky movements, unrealistic morphing, style changes between frames, cartoonish motion, or overly dramatic camera swoops. The movement should feel like a living photograph — cinematic, calm, and grounded in reality.

Important: This is a fictional artistic scene. All subjects are unnamed, generic figures. Do not identify or associate any subject with a real person, celebrity, or public figure. Treat all figures as original artistic characters.

No generated audio or dialogue. Silent video only.`, rawPrompt)
}

// GenerateVideo generates a video using Veo with the provided image as the first frame.
//
// The async operation is polled internally with a configurable timeout (5 minutes).
// This blocks the calling goroutine — this is intentional and fits the existing
// worker architecture where each clip is processed in its own goroutine.
//
// Parameters:
//   - prompt: describes the motion/action for the video (the clip's video_prompt)
//   - imageData: raw bytes of the still image to use as the first frame
//   - imageMimeType: MIME type of the image (e.g., "image/png")
//
// Returns the raw video bytes (MP4) or an error.
func (s *VeoService) GenerateVideo(ctx context.Context, prompt string, imageData []byte, imageMimeType string) ([]byte, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  s.apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	// Enhance the raw prompt with Veo-specific style and motion instructions
	enhancedPrompt := buildVeoPrompt(prompt)

	// Build the first frame from the generated still image
	firstFrame := &genai.Image{
		ImageBytes: imageData,
		MIMEType:   imageMimeType,
	}

	// Configure video generation: portrait 9:16, 4K resolution, allow people in image-to-video mode
	config := &genai.GenerateVideosConfig{
		AspectRatio:      "9:16",
		Resolution:       "4k",
		PersonGeneration: "allow_adult",
		NumberOfVideos:   1,
	}

	log.Printf("[Veo] Starting video generation (model=%s, promptLen=%d, enhancedLen=%d, imageSize=%d bytes)", s.model, len(prompt), len(enhancedPrompt), len(imageData))

	// Start the async video generation operation with the enhanced prompt
	operation, err := client.Models.GenerateVideos(ctx, s.model, enhancedPrompt, firstFrame, config)
	if err != nil {
		return nil, fmt.Errorf("failed to start video generation: %w", err)
	}

	log.Printf("[Veo] Operation started: %s", operation.Name)

	// Poll until done, cancelled, or timed out
	deadline := time.Now().Add(veoMaxPollDuration)
	pollCount := 0
	for !operation.Done {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("video generation timed out after %v (polled %d times)", veoMaxPollDuration, pollCount)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("video generation cancelled: %w", ctx.Err())
		case <-time.After(veoPollInterval):
		}

		pollCount++
		operation, err = client.Operations.GetVideosOperation(ctx, operation, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to poll operation (attempt %d): %w", pollCount, err)
		}

		log.Printf("[Veo] Poll %d: done=%v", pollCount, operation.Done)
	}

	// Check for operation-level errors (e.g. invalid request, quota exceeded)
	if operation.Error != nil && len(operation.Error) > 0 {
		errJSON, _ := json.Marshal(operation.Error)
		return nil, fmt.Errorf("video generation operation failed: %s", string(errJSON))
	}

	// Check if the response exists
	if operation.Response == nil {
		// Log any metadata that might contain clues
		if operation.Metadata != nil {
			metaJSON, _ := json.Marshal(operation.Metadata)
			log.Printf("[Veo] Operation metadata: %s", string(metaJSON))
		}
		return nil, fmt.Errorf("no response in completed operation after %d polls (operation: %s)", pollCount, operation.Name)
	}

	// Check if videos were blocked by RAI (Responsible AI) safety filters
	if operation.Response.RAIMediaFilteredCount > 0 {
		reasons := "unknown"
		if len(operation.Response.RAIMediaFilteredReasons) > 0 {
			reasons = strings.Join(operation.Response.RAIMediaFilteredReasons, ", ")
		}
		return nil, fmt.Errorf("video blocked by safety filters: %d video(s) filtered, reasons: %s", operation.Response.RAIMediaFilteredCount, reasons)
	}

	// Check if any videos were actually generated
	if len(operation.Response.GeneratedVideos) == 0 {
		respJSON, _ := json.Marshal(operation.Response)
		return nil, fmt.Errorf("no videos in response (full response: %s)", string(respJSON))
	}

	// Validate the video object has data
	video := operation.Response.GeneratedVideos[0]
	if video.Video == nil {
		return nil, fmt.Errorf("generated video object is nil")
	}

	log.Printf("[Veo] Video ready, downloading...")

	// Download the generated video
	downloadURI := genai.NewDownloadURIFromVideo(video.Video)
	videoBytes, err := client.Files.Download(ctx, downloadURI, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to download generated video: %w", err)
	}

	if len(videoBytes) == 0 {
		return nil, fmt.Errorf("downloaded video is empty (0 bytes)")
	}

	log.Printf("[Veo] Video generated successfully (%d bytes, %d polls)", len(videoBytes), pollCount)

	return videoBytes, nil
}
