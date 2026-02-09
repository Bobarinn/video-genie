package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// Default Cartesia API version
	CartesiaAPIVersion = "2024-06-10"

	// Default voice ID (you should replace with actual voice IDs from Cartesia)
	DefaultVoiceID = "a0e99841-438c-4a64-b679-ae501e7d6091" // Example voice ID
)

type CartesiaService struct {
	apiKey         string
	apiURL         string
	apiVersion     string
	defaultVoiceID string
	client         *http.Client
}

func NewCartesiaService(apiKey, apiURL string) *CartesiaService {
	return &CartesiaService{
		apiKey:         apiKey,
		apiURL:         apiURL,
		apiVersion:     CartesiaAPIVersion,
		defaultVoiceID: DefaultVoiceID,
		client:         &http.Client{Timeout: 60 * time.Second},
	}
}

// NewCartesiaServiceWithVoice creates a new Cartesia service with a custom default voice
func NewCartesiaServiceWithVoice(apiKey, apiURL, voiceID string) *CartesiaService {
	if voiceID == "" {
		voiceID = DefaultVoiceID
	}
	return &CartesiaService{
		apiKey:         apiKey,
		apiURL:         apiURL,
		apiVersion:     CartesiaAPIVersion,
		defaultVoiceID: voiceID,
		client:         &http.Client{Timeout: 60 * time.Second},
	}
}

// CartesiaRequest matches the actual Cartesia API specification
type CartesiaRequest struct {
	ModelID      string                  `json:"model_id"`
	Transcript   string                  `json:"transcript"`
	Voice        CartesiaVoiceSpecifier  `json:"voice"`
	Language     *string                 `json:"language,omitempty"`
	OutputFormat CartesiaOutputFormat    `json:"output_format"`
	Config       *CartesiaGenerationConfig `json:"generation_config,omitempty"`
}

type CartesiaVoiceSpecifier struct {
	Mode string `json:"mode"`
	ID   string `json:"id"`
}

type CartesiaOutputFormat struct {
	Container  string `json:"container"`
	Encoding   string `json:"encoding,omitempty"`
	SampleRate int    `json:"sample_rate"`
	BitRate    int    `json:"bit_rate,omitempty"`
}

type CartesiaGenerationConfig struct {
	Volume  *float64 `json:"volume,omitempty"`  // 0.5 to 2.0
	Speed   *float64 `json:"speed,omitempty"`   // 0.6 to 1.5
	Emotion *string  `json:"emotion,omitempty"` // e.g., "neutral", "excited", "calm"
}

// CartesiaResponse is the Cartesia-specific response (kept for backward compatibility).
// The GenerateSpeech method returns the common *TTSResponse instead.
type CartesiaResponse struct {
	AudioData  []byte
	DurationMs int
	SampleRate int
	Format     string
}

// Ensure CartesiaService implements TTSService at compile time.
var _ TTSService = (*CartesiaService)(nil)

// GenerateSpeechOptions provides configuration for speech generation
type GenerateSpeechOptions struct {
	VoiceID  string
	Language string
	Emotion  string
	Speed    float64
	Volume   float64
}

// GenerateSpeech generates audio from text using Cartesia TTS.
// Implements the TTSService interface.
func (s *CartesiaService) GenerateSpeech(ctx context.Context, text, voiceStyle string) (*TTSResponse, error) {
	// Parse emotion from voiceStyle (simple heuristic)
	emotion := parseEmotionFromStyle(voiceStyle)

	opts := GenerateSpeechOptions{
		VoiceID:  s.defaultVoiceID,
		Language: "en",
		Emotion:  emotion,
		Speed:    0.85, // Slower pace for clear, natural-sounding narration
		Volume:   1.4,  // Louder output for mobile viewing
	}

	return s.GenerateSpeechWithOptions(ctx, text, opts)
}

// GenerateSpeechWithOptions generates audio with detailed Cartesia-specific configuration.
func (s *CartesiaService) GenerateSpeechWithOptions(ctx context.Context, text string, opts GenerateSpeechOptions) (*TTSResponse, error) {
	// Build request body
	reqBody := CartesiaRequest{
		ModelID:    "sonic-english", // Use sonic-english or sonic-multilingual
		Transcript: text,
		Voice: CartesiaVoiceSpecifier{
			Mode: "id",
			ID:   opts.VoiceID,
		},
		OutputFormat: CartesiaOutputFormat{
			Container:  "mp3",
			SampleRate: 44100,
			BitRate:    192000,
		},
	}

	// Add language if specified
	if opts.Language != "" {
		reqBody.Language = &opts.Language
	}

	// Add generation config if using sonic-3 model
	if opts.Emotion != "" || opts.Speed != 1.0 || opts.Volume != 1.0 {
		config := &CartesiaGenerationConfig{}

		if opts.Emotion != "" {
			config.Emotion = &opts.Emotion
		}

		if opts.Speed != 1.0 {
			speed := opts.Speed
			config.Speed = &speed
		}

		if opts.Volume != 1.0 {
			volume := opts.Volume
			config.Volume = &volume
		}

		reqBody.Config = config
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/tts/bytes", s.apiURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set required headers
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cartesia-Version", s.apiVersion)

	// Make request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("cartesia returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read audio data
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio: %w", err)
	}

	// Calculate duration (approximate based on text length)
	durationMs := estimateAudioDuration(text, opts.Speed)

	return &TTSResponse{
		AudioData:  audioData,
		DurationMs: durationMs,
		Format:     "mp3",
	}, nil
}

// parseEmotionFromStyle attempts to extract emotion from voice style instruction
func parseEmotionFromStyle(style string) string {
	// Map common descriptive words to Cartesia emotions
	emotionMap := map[string]string{
		"energetic":     "excited",
		"engaging":      "enthusiastic",
		"mysterious":    "mysterious",
		"serious":       "calm",
		"authoritative": "confident",
		"dramatic":      "intense",
		"calm":          "calm",
		"peaceful":      "peaceful",
		"excited":       "excited",
		"happy":         "happy",
		"sad":           "sad",
		"angry":         "angry",
		"scared":        "scared",
		"confident":     "confident",
	}

	// Convert to lowercase for matching
	styleLower := bytes.ToLower([]byte(style))

	// Check for emotion keywords
	for keyword, emotion := range emotionMap {
		if bytes.Contains(styleLower, []byte(keyword)) {
			return emotion
		}
	}

	// Default to neutral
	return "neutral"
}

// estimateAudioDuration estimates duration based on text length and speed
// Average speaking rate is ~140 words per minute at normal speed (narration pace, not conversational)
func estimateAudioDuration(text string, speed float64) int {
	words := len(bytes.Fields([]byte(text)))
	baseWPM := 140.0 // words per minute (narration baseline, slightly slower than conversation)

	// Adjust for speed — lower speed = fewer WPM = longer duration
	actualWPM := baseWPM * speed

	minutes := float64(words) / actualWPM
	return int(minutes * 60 * 1000) // Convert to milliseconds
}
