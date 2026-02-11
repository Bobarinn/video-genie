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
)

// ---------------------------------------------------------------------------
// ElevenLabs Text-to-Speech Service
// Uses ElevenLabs REST API to convert text into high-quality speech audio.
// Model: eleven_flash_v2_5 (Flash v2.5 — fast, 32 languages, ~75ms latency)
// ---------------------------------------------------------------------------

const (
	elevenLabsBaseURL     = "https://api.elevenlabs.io"
	elevenLabsDefaultModel = "eleven_flash_v2_5"
	elevenLabsDefaultVoice = "pNInz6obpgDQGcFmaJgB" // Default voice ID
	elevenLabsOutputFormat = "mp3_44100_128"           // High-quality MP3
)

// ElevenLabsService handles text-to-speech via ElevenLabs API.
type ElevenLabsService struct {
	apiKey   string
	voiceID  string
	modelID  string
	client   *http.Client
}

// Ensure ElevenLabsService implements TTSService at compile time.
var _ TTSService = (*ElevenLabsService)(nil)

// NewElevenLabsService creates a new ElevenLabs TTS service with defaults.
func NewElevenLabsService(apiKey string) *ElevenLabsService {
	return &ElevenLabsService{
		apiKey:  apiKey,
		voiceID: elevenLabsDefaultVoice,
		modelID: elevenLabsDefaultModel,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

// NewElevenLabsServiceWithVoice creates an ElevenLabs service with a custom voice ID.
func NewElevenLabsServiceWithVoice(apiKey, voiceID string) *ElevenLabsService {
	if voiceID == "" {
		voiceID = elevenLabsDefaultVoice
	}
	return &ElevenLabsService{
		apiKey:  apiKey,
		voiceID: voiceID,
		modelID: elevenLabsDefaultModel,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// Request types
// ---------------------------------------------------------------------------

type elevenLabsRequest struct {
	Text          string                `json:"text"`
	ModelID       string                `json:"model_id"`
	VoiceSettings *elevenLabsVoiceSettings `json:"voice_settings,omitempty"`
	Speed         *float64              `json:"speed,omitempty"`
}

type elevenLabsVoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style,omitempty"`
	UseSpeakerBoost bool    `json:"use_speaker_boost,omitempty"`
}

// GenerateSpeech converts text to speech using ElevenLabs.
// Implements the TTSService interface.
// voiceID overrides the service-level default when non-empty.
func (s *ElevenLabsService) GenerateSpeech(ctx context.Context, text, voiceStyle, voiceID string) (*TTSResponse, error) {
	// Use per-request voice override if provided, otherwise fall back to service default
	effectiveVoice := s.voiceID
	if voiceID != "" {
		effectiveVoice = voiceID
	}

	// Build request body
	speed := 0.85 // Slightly slower for clear narration delivery
	reqBody := elevenLabsRequest{
		Text:    text,
		ModelID: s.modelID,
		Speed:   &speed,
		VoiceSettings: &elevenLabsVoiceSettings{
			Stability:       0.60, // Moderate stability — allows some emotional range
			SimilarityBoost: 0.80, // High voice consistency
			Style:           0.35, // Mild style exaggeration for natural delivery
			UseSpeakerBoost: true,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ElevenLabs request: %w", err)
	}

	// Build URL: POST /v1/text-to-speech/{voice_id}?output_format=mp3_44100_128
	url := fmt.Sprintf("%s/v1/text-to-speech/%s?output_format=%s",
		elevenLabsBaseURL, effectiveVoice, elevenLabsOutputFormat)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create ElevenLabs request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", s.apiKey)

	log.Printf("[ElevenLabs] Generating speech (voiceID=%s, model=%s, textLen=%d, speed=%.2f)",
		effectiveVoice, s.modelID, len(text), speed)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ElevenLabs returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read audio data — the response body IS the audio file
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ElevenLabs audio response: %w", err)
	}

	if len(audioData) == 0 {
		return nil, fmt.Errorf("ElevenLabs returned empty audio")
	}

	// Estimate duration (ElevenLabs doesn't return duration in the response headers for this endpoint)
	durationMs := estimateAudioDuration(text, speed)

	log.Printf("[ElevenLabs] Speech generated (%d bytes, estimated %dms)", len(audioData), durationMs)

	return &TTSResponse{
		AudioData:  audioData,
		DurationMs: durationMs,
		Format:     "mp3",
	}, nil
}
