package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bobarin/episod/internal/models"
)

// ---------------------------------------------------------------------------
// xAI Grok Imagine Video Generation Service
// Uses the xAI REST API to generate videos from text prompts + optional images.
// Follows a deferred request pattern: submit generation → poll by request_id → download.
// ---------------------------------------------------------------------------

const (
	xaiBaseURL           = "https://api.x.ai/v1"
	xaiVideoModel        = "grok-imagine-video"
	xaiInitialDelay      = 15 * time.Second // Wait before first poll (videos typically take 30-40s)
	xaiPollMinInterval   = 5 * time.Second  // Start polling every 5s
	xaiPollMaxInterval   = 20 * time.Second // Cap at 20s between polls
	xaiPollBackoffFactor = 1.5              // Multiply interval by 1.5 each attempt
	xaiMaxPollDuration   = 5 * time.Minute  // Hard timeout per clip
	xaiMinDuration       = 1                // xAI minimum video duration
	xaiMaxDuration       = 15               // xAI maximum video duration
	xaiDefaultDuration   = 12               // seconds (1-15 allowed)
	xaiDefaultAspect     = "9:16"           // portrait for mobile
	xaiDefaultResolution = "720p"           // 720p or 480p supported
)

// XAIVideoService handles video generation via xAI's Grok Imagine Video API.
// It's optional — when nil or disabled, the worker falls back to
// Ken Burns motion effects on still images.
type XAIVideoService struct {
	apiKey     string
	httpClient *http.Client
}

// NewXAIVideoService creates a new xAI video generation service.
func NewXAIVideoService(apiKey string) *XAIVideoService {
	return &XAIVideoService{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second, // Timeout for individual HTTP calls, not the full poll cycle
		},
	}
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// xaiGenerationRequest is the body for POST /v1/videos/generations
type xaiGenerationRequest struct {
	Prompt      string          `json:"prompt"`
	Model       string          `json:"model"`
	Image       *xaiImageInput  `json:"image,omitempty"`
	Duration    int             `json:"duration,omitempty"`
	AspectRatio string          `json:"aspect_ratio,omitempty"`
	Resolution  string          `json:"resolution,omitempty"`
}

// xaiImageInput is an image reference for image-to-video generation
type xaiImageInput struct {
	URL string `json:"url"`
}

// xaiGenerationResponse is the response from POST /v1/videos/generations
type xaiGenerationResponse struct {
	RequestID string `json:"request_id"`
}

// xaiVideoResult is the unified response from GET /v1/videos/{request_id}.
//
// xAI returns two different shapes depending on state:
//   - Pending: {"status":"pending"}
//   - Completed: {"video":{"url":"...","duration":8,"respect_moderation":true},"model":"grok-imagine-video"}
//     (note: no "status" field when completed — status will be "")
//   - Failed: {"status":"failed","error":"..."}
type xaiVideoResult struct {
	Status string          `json:"status"`          // "pending", "failed", or "" (completed)
	Video  *xaiVideoOutput `json:"video,omitempty"` // Present when generation is complete
	Model  string          `json:"model,omitempty"` // Present when generation is complete
	Error  string          `json:"error"`           // Error message if failed
}

// xaiVideoOutput is the nested video object in a completed generation response.
type xaiVideoOutput struct {
	URL               string `json:"url"`
	Duration          int    `json:"duration"`
	RespectModeration bool   `json:"respect_moderation"`
}

// VideoGenOptions holds per-project overrides for xAI video generation.
type VideoGenOptions struct {
	Preset      *models.GraphicsPreset // Visual style preset (name, description, style_json, prompt_addition)
	AspectRatio *string                // "9:16", "16:9", "1:1"
}

// buildXAIVideoPrompt enhances the raw video_prompt with xAI-specific instructions
// and the full graphics preset style details.
func buildXAIVideoPrompt(rawPrompt string, opts *VideoGenOptions) string {
	// Build visual style section from the graphics preset
	var styleSection string
	if opts != nil && opts.Preset != nil {
		styleSection = fmt.Sprintf("Visual style: \"%s\".", opts.Preset.Name)
		if opts.Preset.Description != nil && *opts.Preset.Description != "" {
			styleSection += " " + *opts.Preset.Description
		}
		if opts.Preset.PromptAddition != nil && *opts.Preset.PromptAddition != "" {
			styleSection += " " + *opts.Preset.PromptAddition
		}
	} else {
		styleSection = "Match the style and mood of the input image."
	}

	return fmt.Sprintf(`%s

%s
Maintain visual consistency with the input image throughout the video. Preserve the color palette, lighting, and artistic quality from the source frame.

Generate natural, cinematic movement that brings the scene to life. Silent video only — no generated audio or dialogue.`, rawPrompt, styleSection)
}

// GenerateVideo generates a video using xAI Grok Imagine Video.
//
// If imageURL is non-empty, it's used as the source image for image-to-video generation.
// The async operation is polled internally with a configurable timeout.
//
// Parameters:
//   - prompt: describes the motion/action for the video (the clip's video_prompt)
//   - imageURL: publicly accessible URL of the source image (empty = text-only generation)
//   - durationSec: desired video duration in seconds (clamped to xAI's 1-15s range, 0 = use default 8s)
//   - opts: per-project overrides for visual style and aspect ratio (nil = use defaults)
//
// Returns the raw video bytes (MP4) or an error.
func (s *XAIVideoService) GenerateVideo(ctx context.Context, prompt string, imageURL string, durationSec int, opts *VideoGenOptions) ([]byte, error) {
	enhancedPrompt := buildXAIVideoPrompt(prompt, opts)

	// Clamp duration to xAI's allowed range
	if durationSec <= 0 {
		durationSec = xaiDefaultDuration
	}
	if durationSec < xaiMinDuration {
		durationSec = xaiMinDuration
	}
	if durationSec > xaiMaxDuration {
		durationSec = xaiMaxDuration
	}

	// Resolve aspect ratio — per-project override or default
	aspectRatio := xaiDefaultAspect
	if opts != nil && opts.AspectRatio != nil && *opts.AspectRatio != "" {
		aspectRatio = *opts.AspectRatio
	}

	// Step 1: Submit generation request
	reqBody := xaiGenerationRequest{
		Prompt:      enhancedPrompt,
		Model:       xaiVideoModel,
		Duration:    durationSec,
		AspectRatio: aspectRatio,
		Resolution:  xaiDefaultResolution,
	}

	if imageURL != "" {
		reqBody.Image = &xaiImageInput{URL: imageURL}
	}

	log.Printf("[xAI Video] Starting video generation (promptLen=%d, enhancedLen=%d, hasImage=%v, duration=%ds, aspect=%s)",
		len(prompt), len(enhancedPrompt), imageURL != "", durationSec, aspectRatio)

	requestID, err := s.submitGeneration(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to submit video generation: %w", err)
	}

	log.Printf("[xAI Video] Generation submitted, request_id=%s", requestID)

	// Step 2: Poll for completion
	result, err := s.pollForResult(ctx, requestID)
	if err != nil {
		return nil, err
	}

	log.Printf("[xAI Video] Video ready (duration=%ds), downloading from URL...", result.Video.Duration)

	// Step 3: Download the video from the returned URL
	videoBytes, err := s.downloadVideo(ctx, result.Video.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download generated video: %w", err)
	}

	if len(videoBytes) == 0 {
		return nil, fmt.Errorf("downloaded video is empty (0 bytes)")
	}

	log.Printf("[xAI Video] Video downloaded successfully (%d bytes)", len(videoBytes))
	return videoBytes, nil
}

// submitGeneration sends the initial video generation request and returns the request_id.
func (s *XAIVideoService) submitGeneration(ctx context.Context, reqBody xaiGenerationRequest) (string, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", xaiBaseURL+"/videos/generations", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("xAI returned status %d: %s", resp.StatusCode, string(body))
	}

	var genResp xaiGenerationResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return "", fmt.Errorf("failed to parse generation response: %w (body: %s)", err, string(body))
	}

	if genResp.RequestID == "" {
		return "", fmt.Errorf("no request_id in generation response: %s", string(body))
	}

	return genResp.RequestID, nil
}

// pollForResult polls GET /v1/videos/{request_id} until the video is ready or an error occurs.
//
// Polling strategy: exponential backoff starting at 5s, scaling by 1.5x up to a 20s cap.
// An initial delay of 15s avoids wasting API calls on guaranteed "pending" responses.
// Hard timeout: 5 minutes per clip.
//
// Detection logic: xAI returns two different response shapes:
//   - Pending: {"status":"pending"} — status field is "pending"
//   - Completed: {"video":{"url":"...","duration":8},"model":"..."} — no status field, video object present
//   - Failed: {"status":"failed","error":"..."} — status is "failed"
func (s *XAIVideoService) pollForResult(ctx context.Context, requestID string) (*xaiVideoResult, error) {
	deadline := time.Now().Add(xaiMaxPollDuration)
	pollCount := 0
	currentInterval := xaiPollMinInterval

	// Wait before the first poll — xAI video generation typically takes 30-40s,
	// so the first 15s are guaranteed to be "pending".
	log.Printf("[xAI Video] Waiting %v before first poll (videos typically take 30-40s)...", xaiInitialDelay)
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("video generation cancelled during initial wait: %w", ctx.Err())
	case <-time.After(xaiInitialDelay):
	}

	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("video generation timed out after %v (polled %d times, request_id=%s)", xaiMaxPollDuration, pollCount, requestID)
		}

		pollCount++

		result, err := s.getVideoResult(ctx, requestID)
		if err != nil {
			return nil, fmt.Errorf("failed to poll video result (attempt %d): %w", pollCount, err)
		}

		// Detection: when xAI completes, it returns a "video" object with no "status" field.
		// When pending, it returns {"status":"pending"} with no "video" object.
		if result.Video != nil && result.Video.URL != "" {
			log.Printf("[xAI Video] Poll %d: completed (video url present, duration=%ds)", pollCount, result.Video.Duration)
			return result, nil
		}

		log.Printf("[xAI Video] Poll %d: status=%s (next poll in %v)", pollCount, result.Status, currentInterval)

		switch result.Status {
		case "failed":
			errMsg := result.Error
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nil, fmt.Errorf("video generation failed: %s (request_id=%s)", errMsg, requestID)

		default:
			// Still pending — wait with exponential backoff before next poll
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("video generation cancelled: %w", ctx.Err())
			case <-time.After(currentInterval):
			}

			// Increase interval: 5s → 7.5s → 11.25s → 16.8s → 20s (capped)
			next := time.Duration(float64(currentInterval) * xaiPollBackoffFactor)
			if next > xaiPollMaxInterval {
				next = xaiPollMaxInterval
			}
			currentInterval = next
		}
	}
}

// getVideoResult fetches the current status of a video generation request.
func (s *XAIVideoService) getVideoResult(ctx context.Context, requestID string) (*xaiVideoResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/videos/%s", xaiBaseURL, requestID), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Accept both 200 (completed) and 202 (still processing) as valid poll responses.
	// xAI returns 202 with {"status":"pending"} while the video is being generated.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("xAI returned status %d: %s", resp.StatusCode, string(body))
	}

	var result xaiVideoResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse video result: %w (body: %s)", err, string(body))
	}

	return &result, nil
}

// downloadVideo fetches the video bytes from the given URL.
func (s *XAIVideoService) downloadVideo(ctx context.Context, videoURL string) ([]byte, error) {
	// Use a longer timeout for video download (videos can be large)
	downloadClient := &http.Client{Timeout: 120 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "GET", videoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("video download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read video data: %w", err)
	}

	return data, nil
}
