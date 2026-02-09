package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	// Server
	APIPort            string
	WorkerEnabled      bool
	BackendAPIKey      string // API key for authenticating requests (empty = no auth, dev mode)
	CorsAllowedOrigins string // Comma-separated allowed origins (empty = *, dev mode)

	// Database
	DatabaseURL string

	// Redis
	RedisURL string

	// Supabase
	SupabaseURL         string
	SupabaseServiceKey  string
	SupabaseStorageBucket string

	// OpenAI (used for text planning)
	OpenAIKey string

	// Gemini (used for image generation)
	GeminiKey                 string
	GeminiStyleReferenceImage string

	// Veo (used for video generation from still images — legacy, kept for reference)
	VeoEnabled bool   // Feature flag: when true, clips get AI-generated video via Veo instead of Ken Burns effects
	VeoModel   string // Veo model identifier (default: veo-3.1-generate-preview)

	// xAI (used for video generation via Grok Imagine Video)
	XAIEnabled bool   // Feature flag: when true, clips get AI-generated video via xAI instead of Ken Burns effects
	XAIAPIKey  string // xAI API key for Grok Imagine Video

	// ElevenLabs (preferred TTS provider)
	ElevenLabsKey     string
	ElevenLabsVoiceID string

	// Cartesia (legacy TTS provider — used when ElevenLabs key is not set)
	CartesiaKey     string
	CartesiaURL     string
	CartesiaVoiceID string

	// Audio
	BackgroundMusicPath string // Path to default background music file

	// Worker
	MaxConcurrentJobs int
}

func Load() (*Config, error) {
	// Load .env file if it exists (ignore error in production)
	_ = godotenv.Load()

	cfg := &Config{
		APIPort:               getEnv("API_PORT", "8080"),
		WorkerEnabled:         getEnvBool("WORKER_ENABLED", true),
		BackendAPIKey:         getEnv("BACKEND_API_KEY", ""),
		CorsAllowedOrigins:    getEnv("CORS_ALLOWED_ORIGINS", ""),
		DatabaseURL:           getEnv("DATABASE_URL", ""),
		RedisURL:              getEnv("REDIS_URL", "redis://localhost:6379"),
		SupabaseURL:           getEnv("SUPABASE_URL", ""),
		SupabaseServiceKey:    getEnv("SUPABASE_SERVICE_KEY", ""),
		SupabaseStorageBucket: getEnv("SUPABASE_STORAGE_BUCKET", "faceless-videos"),
		OpenAIKey:             getEnv("OPENAI_API_KEY", ""),
		GeminiKey:                 getEnv("GEMINI_API_KEY", ""),
		GeminiStyleReferenceImage: getEnv("GEMINI_STYLE_REFERENCE_IMAGE", "assets/style-reference/sample.jpeg"),
		VeoEnabled:                getEnvBool("VEO_ENABLED", false),
		VeoModel:                  getEnv("VEO_MODEL", "veo-3.1-generate-preview"),
		XAIEnabled:                getEnvBool("XAI_VIDEO_ENABLED", false),
		XAIAPIKey:                 getEnv("XAI_API_KEY", ""),
		ElevenLabsKey:             getEnv("ELEVENLABS_API_KEY", ""),
		ElevenLabsVoiceID:        getEnv("ELEVENLABS_VOICE_ID", ""),
		CartesiaKey:               getEnv("CARTESIA_API_KEY", ""),
		CartesiaURL:           getEnv("CARTESIA_API_URL", "https://api.cartesia.ai"),
		CartesiaVoiceID:       getEnv("CARTESIA_VOICE_ID", ""),
		BackgroundMusicPath:   getEnv("BACKGROUND_MUSIC_PATH", "assets/music/music.mp3"),
		MaxConcurrentJobs:     getEnvInt("MAX_CONCURRENT_JOBS", 5),
	}

	// Validate required fields
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	if cfg.OpenAIKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required")
	}

	if cfg.GeminiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY is required")
	}

	// At least one TTS provider must be configured
	if cfg.ElevenLabsKey == "" && cfg.CartesiaKey == "" {
		return nil, fmt.Errorf("either ELEVENLABS_API_KEY or CARTESIA_API_KEY is required for TTS")
	}

	if cfg.SupabaseURL == "" || cfg.SupabaseServiceKey == "" {
		return nil, fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY are required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		b, err := strconv.ParseBool(value)
		if err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		i, err := strconv.Atoi(value)
		if err == nil {
			return i
		}
	}
	return defaultValue
}
