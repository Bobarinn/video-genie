package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bobarin/episod/internal/api"
	"github.com/bobarin/episod/internal/config"
	"github.com/bobarin/episod/internal/db"
	"github.com/bobarin/episod/internal/queue"
	"github.com/bobarin/episod/internal/services"
	"github.com/bobarin/episod/internal/storage"
	"github.com/bobarin/episod/internal/worker"
)

func main() {
	log.Println("Starting Episod API...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to database
	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()
	log.Println("Connected to database")

	// Connect to Redis queue
	q, err := queue.New(cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to queue: %v", err)
	}
	defer q.Close()
	log.Println("Connected to Redis queue")

	// Initialize storage
	stor := storage.New(cfg.SupabaseURL, cfg.SupabaseServiceKey, cfg.SupabaseStorageBucket)
	log.Println("Initialized Supabase storage")

	// Create API handler
	handler := api.NewHandler(database, q, stor)
	router := api.NewRouter(handler, api.RouterConfig{
		BackendAPIKey:      cfg.BackendAPIKey,
		CorsAllowedOrigins: cfg.CorsAllowedOrigins,
	})

	if cfg.BackendAPIKey != "" {
		log.Println("API key authentication enabled")
	} else {
		log.Println("WARNING: No BACKEND_API_KEY set — API is unprotected (dev mode)")
	}

	// Start HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.APIPort,
		Handler: router,
	}

	// Start worker if enabled
	var workerCtx context.Context
	var workerCancel context.CancelFunc
	if cfg.WorkerEnabled {
		log.Println("Worker enabled, starting background processing...")

		// Initialize services
		openaiSvc := services.NewOpenAIService(cfg.OpenAIKey)
		geminiSvc := services.NewGeminiServiceWithStyleReference(cfg.GeminiKey, cfg.GeminiStyleReferenceImage)
		ffmpegSvc := services.NewFFmpegService("/tmp/episod", services.ParseResolution(cfg.RenderResolution))

		// Initialize TTS provider — ElevenLabs preferred, Cartesia as legacy fallback
		var ttsSvc services.TTSService
		if cfg.ElevenLabsKey != "" {
			ttsSvc = services.NewElevenLabsServiceWithVoice(cfg.ElevenLabsKey, cfg.ElevenLabsVoiceID)
			log.Printf("TTS provider: ElevenLabs (voice: %s, model: eleven_flash_v2_5)", cfg.ElevenLabsVoiceID)
		} else {
			ttsSvc = services.NewCartesiaServiceWithVoice(cfg.CartesiaKey, cfg.CartesiaURL, cfg.CartesiaVoiceID)
			log.Printf("TTS provider: Cartesia (legacy, voice: %s)", cfg.CartesiaVoiceID)
		}

		// Initialize Veo service (legacy, optional — nil when disabled)
		var veoSvc *services.VeoService
		if cfg.VeoEnabled {
			veoSvc = services.NewVeoService(cfg.GeminiKey, cfg.VeoModel)
			log.Printf("Veo video generation enabled (model: %s)", cfg.VeoModel)
		}

		// Initialize xAI Video service (preferred over Veo — nil when disabled)
		var xaiVideoSvc *services.XAIVideoService
		if cfg.XAIEnabled && cfg.XAIAPIKey != "" {
			xaiVideoSvc = services.NewXAIVideoService(cfg.XAIAPIKey)
			log.Println("xAI Grok Imagine Video generation enabled")
		} else if !cfg.VeoEnabled {
			log.Println("AI video generation disabled — using Ken Burns effects")
		}

		// Create worker
		w := worker.New(database, q, stor, openaiSvc, ttsSvc, geminiSvc, veoSvc, xaiVideoSvc, ffmpegSvc, cfg.BackgroundMusicPath)

		// Start worker in background
		workerCtx, workerCancel = context.WithCancel(context.Background())
		go w.Start(workerCtx, cfg.MaxConcurrentJobs)
	}

	// Start server in goroutine
	go func() {
		log.Printf("API server listening on :%s", cfg.APIPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Shutdown worker
	if workerCancel != nil {
		workerCancel()
	}

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}
