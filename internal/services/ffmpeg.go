package services

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ---------------------------------------------------------------------------
// Motion effect types — each clip gets one randomly combined with a subtle breathing pulse
// ---------------------------------------------------------------------------

// ClipEffect defines the type of Ken Burns / motion effect applied to a still image
type ClipEffect string

const (
	EffectZoomIn        ClipEffect = "zoom_in"          // Strong zoom toward center
	EffectZoomOut       ClipEffect = "zoom_out"         // Starts zoomed, pulls back wide
	EffectPanDown       ClipEffect = "pan_down"         // Drifts top to bottom
	EffectPanUp         ClipEffect = "pan_up"           // Drifts bottom to top
	EffectPanLeft       ClipEffect = "pan_left"         // Drifts right to left
	EffectPanRight      ClipEffect = "pan_right"        // Drifts left to right
	EffectZoomInPanUp   ClipEffect = "zoom_in_pan_up"   // Zoom in while drifting up
	EffectZoomInPanDown ClipEffect = "zoom_in_pan_down" // Zoom in while drifting down
	EffectZoomInPanLeft ClipEffect = "zoom_in_pan_left" // Zoom in while drifting left
	EffectZoomInPanRight ClipEffect = "zoom_in_pan_right" // Zoom in while drifting right
)

// allEffects is the pool from which a random effect is chosen per clip
var allEffects = []ClipEffect{
	EffectZoomIn,
	EffectZoomOut,
	EffectPanDown,
	EffectPanUp,
	EffectPanLeft,
	EffectPanRight,
	EffectZoomInPanUp,
	EffectZoomInPanDown,
	EffectZoomInPanLeft,
	EffectZoomInPanRight,
}

// RandomEffect picks a random motion effect for a clip
func RandomEffect() ClipEffect {
	return allEffects[rand.Intn(len(allEffects))]
}

// Output / rendering constants — 4K portrait (2160x3840) at 30fps
const (
	outputWidth  = 2160
	outputHeight = 3840
	videoFPS     = 30

	// Breathing pulse: a subtle zoom oscillation layered on top of the primary motion.
	// Because the subject is centered and dominant, this creates the illusion of the
	// subject gently "breathing" or pulsing while background edges shift slightly.
	// Amplitude: ±0.03 zoom (3% oscillation) at ~0.12 rad/frame ≈ one full breath every ~2 seconds.
	breathAmplitude = 0.03
	breathFrequency = 0.12
)

// ---------------------------------------------------------------------------
// FFmpegService
// ---------------------------------------------------------------------------

type FFmpegService struct {
	tempDir string
}

func NewFFmpegService(tempDir string) *FFmpegService {
	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		panic(fmt.Sprintf("failed to create temp dir: %v", err))
	}

	return &FFmpegService{
		tempDir: tempDir,
	}
}

// PrependSilence adds a silence buffer at the start of an audio file.
// This prevents the first word from being clipped and creates natural pauses between clips.
func (s *FFmpegService) PrependSilence(ctx context.Context, inputAudioPath, outputAudioPath string, silenceMs int) error {
	delayFilter := fmt.Sprintf("adelay=%d|%d", silenceMs, silenceMs)

	args := []string{
		"-i", inputAudioPath,
		"-af", delayFilter,
		"-y",
		outputAudioPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg prepend silence failed: %w", err)
	}

	return nil
}

// RenderClipWithEffect creates a video clip from a still image and audio,
// applying a Ken Burns motion effect (zoom/pan) combined with a subtle breathing
// pulse that makes the centered subject appear to gently pulse or breathe.
// durationMs is the audio duration used to calculate the frame count for the effect.
// If subtitlePath is non-empty, TikTok-style ASS subtitles are burned into the video.
func (s *FFmpegService) RenderClipWithEffect(ctx context.Context, imagePath, audioPath, outputPath string, effect ClipEffect, durationMs int, subtitlePath string) error {
	vf := buildMotionFilter(effect, durationMs)

	// Append ASS subtitle burn-in if a subtitle file was generated
	if subtitlePath != "" {
		// Escape colons and backslashes in the path for FFmpeg filter syntax
		escapedPath := escapeFFmpegFilterPath(subtitlePath)
		vf += fmt.Sprintf(",ass='%s'", escapedPath)
		log.Printf("[FFmpeg] Burning in subtitles from %s", subtitlePath)
	}

	log.Printf("[FFmpeg] Rendering with effect=%s, duration=%dms, filter=%s", effect, durationMs, vf)

	args := []string{
		"-i", imagePath,  // Single image input (zoompan handles duration)
		"-i", audioPath,  // Audio input
		"-vf", vf,        // Motion effect + subtitles filter chain
		"-c:v", "libx264",
		"-c:a", "aac",
		"-b:a", "192k",
		"-pix_fmt", "yuv420p",
		"-shortest", // End when the shorter stream (audio) ends
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg render clip failed (effect=%s): %w", effect, err)
	}

	return nil
}

// escapeFFmpegFilterPath escapes special characters in file paths for FFmpeg filter syntax.
// FFmpeg filter strings treat colons, backslashes, and single quotes specially.
func escapeFFmpegFilterPath(path string) string {
	// Replace backslashes first, then colons (relevant for Windows paths and filter syntax)
	path = strings.ReplaceAll(path, "\\", "\\\\")
	path = strings.ReplaceAll(path, ":", "\\:")
	path = strings.ReplaceAll(path, "'", "'\\''")
	return path
}

// buildMotionFilter constructs the FFmpeg -vf filter chain for a given effect.
// It combines a zoompan filter (primary motion: pan/zoom) with a subtle breathing
// pulse — a gentle zoom oscillation that makes the centered subject appear to
// "breathe" or pulse while the background edges shift slightly.
//
// Pipeline: image → zoompan (motion + breathing pulse baked into z expression) → output 1080x1920
//
// The source images are 3072x5504 and output is 1080x1920, so we have ~3x resolution
// headroom for panning and zooming without any quality loss.
func buildMotionFilter(effect ClipEffect, durationMs int) string {
	// Calculate total frames — add 2-second buffer so zoompan always produces
	// enough frames; -shortest will trim to audio length
	totalFrames := (durationMs * videoFPS / 1000) + videoFPS*2
	if totalFrames < videoFPS {
		totalFrames = videoFPS // minimum 1 second
	}

	// Breathing pulse expression: a gentle sine oscillation added to the base zoom.
	// This creates a subtle ±3% zoom oscillation (~one full "breath" every 2 seconds).
	// Because the subject is centered and fills most of the frame, the subject appears
	// to gently pulse/breathe while background edges shift — a living-portrait effect.
	breathExpr := fmt.Sprintf("%.3f*sin(on*%.3f)", breathAmplitude, breathFrequency)

	// Build the zoompan z/x/y expressions based on effect type
	// Center expressions (reused):
	//   cx = "iw/2-(iw/zoom/2)"  — horizontally centered
	//   cy = "ih/2-(ih/zoom/2)"  — vertically centered
	//   maxX = "iw-iw/zoom"      — max X pan range
	//   maxY = "ih-ih/zoom"      — max Y pan range
	var zExpr, xExpr, yExpr string

	switch effect {

	// --- Pure zoom effects (strong, 50% range) + breathing pulse ---

	case EffectZoomIn:
		// Zoom from 1.0 → 1.5 centered — very noticeable push-in
		zExpr = fmt.Sprintf("1.0+0.5*on/%d+%s", totalFrames, breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = "ih/2-(ih/zoom/2)"

	case EffectZoomOut:
		// Zoom from 1.5 → 1.0 centered — dramatic reveal
		zExpr = fmt.Sprintf("1.5-0.5*on/%d+%s", totalFrames, breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = "ih/2-(ih/zoom/2)"

	// --- Pure pan effects (1.3x zoom = 30% crop, full traverse) + breathing pulse ---

	case EffectPanDown:
		// Fixed 1.3x zoom + breathing pulse, camera drifts from top to bottom
		zExpr = fmt.Sprintf("1.3+%s", breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = fmt.Sprintf("(ih-ih/zoom)*on/%d", totalFrames)

	case EffectPanUp:
		// Fixed 1.3x zoom + breathing pulse, camera drifts from bottom to top
		zExpr = fmt.Sprintf("1.3+%s", breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = fmt.Sprintf("(ih-ih/zoom)*(1-on/%d)", totalFrames)

	case EffectPanRight:
		// Fixed 1.3x zoom + breathing pulse, camera drifts from left to right
		zExpr = fmt.Sprintf("1.3+%s", breathExpr)
		xExpr = fmt.Sprintf("(iw-iw/zoom)*on/%d", totalFrames)
		yExpr = "ih/2-(ih/zoom/2)"

	case EffectPanLeft:
		// Fixed 1.3x zoom + breathing pulse, camera drifts from right to left
		zExpr = fmt.Sprintf("1.3+%s", breathExpr)
		xExpr = fmt.Sprintf("(iw-iw/zoom)*(1-on/%d)", totalFrames)
		yExpr = "ih/2-(ih/zoom/2)"

	// --- Zoom + pan combos (zoom 1.0→1.4 while drifting) + breathing pulse ---

	case EffectZoomInPanUp:
		// Zoom 1.0 → 1.4 while drifting upward
		zExpr = fmt.Sprintf("1.0+0.4*on/%d+%s", totalFrames, breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = fmt.Sprintf("max(0,(ih-ih/zoom)*(1-on/%d))", totalFrames)

	case EffectZoomInPanDown:
		// Zoom 1.0 → 1.4 while drifting downward
		zExpr = fmt.Sprintf("1.0+0.4*on/%d+%s", totalFrames, breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = fmt.Sprintf("min(ih-ih/zoom,(ih-ih/zoom)*on/%d)", totalFrames)

	case EffectZoomInPanRight:
		// Zoom 1.0 → 1.4 while drifting right
		zExpr = fmt.Sprintf("1.0+0.4*on/%d+%s", totalFrames, breathExpr)
		xExpr = fmt.Sprintf("min(iw-iw/zoom,(iw-iw/zoom)*on/%d)", totalFrames)
		yExpr = "ih/2-(ih/zoom/2)"

	case EffectZoomInPanLeft:
		// Zoom 1.0 → 1.4 while drifting left
		zExpr = fmt.Sprintf("1.0+0.4*on/%d+%s", totalFrames, breathExpr)
		xExpr = fmt.Sprintf("max(0,(iw-iw/zoom)*(1-on/%d))", totalFrames)
		yExpr = "ih/2-(ih/zoom/2)"

	default:
		// Fallback: noticeable zoom in with breathing
		zExpr = fmt.Sprintf("1.0+0.4*on/%d+%s", totalFrames, breathExpr)
		xExpr = "iw/2-(iw/zoom/2)"
		yExpr = "ih/2-(ih/zoom/2)"
	}

	// Zoompan: reads a single image, produces a video stream with the combined
	// motion effect (pan/zoom) and breathing pulse. Output is the final resolution.
	zoompan := fmt.Sprintf(
		"zoompan=z='%s':x='%s':y='%s':d=%d:s=%dx%d:fps=%d",
		zExpr, xExpr, yExpr,
		totalFrames,
		outputWidth, outputHeight,
		videoFPS,
	)

	return zoompan
}

// MixBackgroundMusic takes a video file and mixes looping background music underneath
// the existing narration audio. The music loops if shorter than the video, and is
// cut when the video ends. Music volume is set low so narration remains dominant.
//
// musicPath: path to the background music file (mp3/wav/etc.)
// If musicPath is empty or the file doesn't exist, returns nil (no-op).
func (s *FFmpegService) MixBackgroundMusic(ctx context.Context, videoPath, musicPath, outputPath string) error {
	// Skip if no music path provided
	if musicPath == "" {
		log.Printf("[FFmpeg] No background music path provided, skipping")
		return nil
	}

	// Check if music file exists
	if _, err := os.Stat(musicPath); os.IsNotExist(err) {
		log.Printf("[FFmpeg] Background music file not found at %s, skipping", musicPath)
		return nil
	}

	log.Printf("[FFmpeg] Mixing background music from %s", musicPath)

	// Filter complex explanation:
	// [0:a] = narration audio from the video (keep at full volume)
	// [1:a] = background music (set to 12% volume — subtle, won't overpower narration)
	// amix: combines both, duration=first means end when the video ends
	// dropout_transition=3: 3-second fade-out at the end for smooth finish
	filterComplex := "[0:a]volume=1.0[narration];[1:a]volume=0.12[music];[narration][music]amix=inputs=2:duration=first:dropout_transition=3[aout]"

	args := []string{
		"-i", videoPath,           // Input 0: concatenated video with narration
		"-stream_loop", "-1",      // Loop the music infinitely
		"-i", musicPath,           // Input 1: background music
		"-filter_complex", filterComplex,
		"-map", "0:v",            // Video from the concatenated file (no re-encoding)
		"-map", "[aout]",         // Mixed audio output
		"-c:v", "copy",           // Copy video stream as-is (fast!)
		"-c:a", "aac",            // Re-encode audio with AAC
		"-b:a", "192k",           // Audio bitrate
		"-shortest",              // End when the shortest mapped stream ends
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg mix background music failed: %w", err)
	}

	return nil
}

// RenderClipFromVideo combines an AI-generated video (xAI or Veo) with narration audio.
// The video's native audio track is discarded and replaced with the narration.
// If the video is shorter than the audio, the last frame is frozen using tpad
// to extend the video until the audio finishes.
// If the video is longer than the audio, -shortest trims it to match.
// If subtitlePath is non-empty, TikTok-style ASS subtitles are burned into the video.
func (s *FFmpegService) RenderClipFromVideo(ctx context.Context, videoPath, audioPath, outputPath string, subtitlePath string) error {
	log.Printf("[FFmpeg] Combining AI video with narration audio")

	// Build filter: tpad for frame freezing, optionally chain ASS subtitles
	filterExpr := "[0:v]tpad=stop_mode=clone:stop_duration=60"
	if subtitlePath != "" {
		escapedPath := escapeFFmpegFilterPath(subtitlePath)
		filterExpr += fmt.Sprintf(",ass='%s'", escapedPath)
		log.Printf("[FFmpeg] Burning in subtitles from %s", subtitlePath)
	}
	filterExpr += "[v]"

	args := []string{
		"-i", videoPath,  // Input 0: Veo video (visual + Veo audio we'll discard)
		"-i", audioPath,  // Input 1: Narration audio
		"-filter_complex", filterExpr,
		"-map", "[v]",    // Use the padded video stream
		"-map", "1:a",    // Use narration audio only (discard Veo audio)
		"-c:v", "libx264",
		"-c:a", "aac",
		"-b:a", "192k",
		"-pix_fmt", "yuv420p",
		"-shortest",      // End when the shorter stream finishes
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg render clip from video failed: %w", err)
	}

	return nil
}

// ConcatenateClips combines multiple video clips into one final video
func (s *FFmpegService) ConcatenateClips(ctx context.Context, clipPaths []string, outputPath string) error {
	if len(clipPaths) == 0 {
		return fmt.Errorf("no clips to concatenate")
	}

	// Create a concat list file
	listPath := filepath.Join(s.tempDir, "concat_list.txt")
	f, err := os.Create(listPath)
	if err != nil {
		return fmt.Errorf("failed to create concat list: %w", err)
	}

	for _, path := range clipPaths {
		// Write in FFmpeg concat format
		fmt.Fprintf(f, "file '%s'\n", path)
	}
	f.Close()
	defer os.Remove(listPath)

	// FFmpeg concat command
	args := []string{
		"-f", "concat",
		"-safe", "0",
		"-i", listPath,
		"-c", "copy", // Copy without re-encoding
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concatenate failed: %w", err)
	}

	return nil
}

// GetAudioDuration returns the duration of an audio file in milliseconds
func (s *FFmpegService) GetAudioDuration(ctx context.Context, audioPath string) (int, error) {
	// Use ffprobe to get duration
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var durationSec float64
	if _, err := fmt.Sscanf(string(output), "%f", &durationSec); err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return int(durationSec * 1000), nil
}

// GetVideoDuration returns the duration of a video file in milliseconds using ffprobe.
func (s *FFmpegService) GetVideoDuration(ctx context.Context, videoPath string) (int, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	}

	cmd := exec.CommandContext(ctx, "ffprobe", args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe video duration failed: %w", err)
	}

	var durationSec float64
	if _, err := fmt.Sscanf(strings.TrimSpace(string(output)), "%f", &durationSec); err != nil {
		return 0, fmt.Errorf("failed to parse video duration: %w", err)
	}

	return int(durationSec * 1000), nil
}

// CreateTempFile creates a temporary file in the service's temp directory
func (s *FFmpegService) CreateTempFile(filename string) string {
	return filepath.Join(s.tempDir, filename)
}

// Cleanup removes temporary files
func (s *FFmpegService) Cleanup(paths ...string) {
	for _, path := range paths {
		os.Remove(path)
	}
}
