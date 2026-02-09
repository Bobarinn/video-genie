package services

import "context"

// ---------------------------------------------------------------------------
// TTSService — common interface for text-to-speech providers
// Both ElevenLabs and Cartesia implement this interface so the worker
// can use whichever is configured without knowing the underlying provider.
// ---------------------------------------------------------------------------

// TTSResponse is the common response type from any TTS provider.
type TTSResponse struct {
	AudioData  []byte
	DurationMs int
	Format     string // "mp3", "wav", etc.
}

// TTSService is the interface that any TTS provider must implement.
type TTSService interface {
	// GenerateSpeech converts text to audio using the provider's default settings.
	// voiceStyle is a human-readable description of the desired delivery style
	// (e.g., "slow, mysterious, and low-pitched"). The provider may or may not use it.
	GenerateSpeech(ctx context.Context, text, voiceStyle string) (*TTSResponse, error)
}
