package api

import (
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// RouterConfig holds settings for the API router.
// Passed from main.go so the router can configure CORS and auth from env vars.
type RouterConfig struct {
	// BackendAPIKey is the key that must be provided in X-API-Key or Authorization: Bearer <key>.
	// If empty, auth middleware is skipped (development mode).
	BackendAPIKey string

	// CorsAllowedOrigins is a comma-separated list of allowed origins.
	// If empty, defaults to "*" (development mode).
	CorsAllowedOrigins string
}

func NewRouter(h *Handler, cfg RouterConfig) *chi.Mux {
	r := chi.NewRouter()

	// Global middleware (applied to all routes including /health)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)

	// CORS: restrict origins when configured, otherwise allow all (dev mode)
	allowedOrigins := []string{"*"}
	if cfg.CorsAllowedOrigins != "" {
		origins := strings.Split(cfg.CorsAllowedOrigins, ",")
		trimmed := make([]string, 0, len(origins))
		for _, o := range origins {
			if s := strings.TrimSpace(o); s != "" {
				trimmed = append(trimmed, s)
			}
		}
		if len(trimmed) > 0 {
			allowedOrigins = trimmed
		}
	}

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-API-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check — public, no auth required
	r.Get("/health", h.Health)

	// API routes — protected by API key auth
	r.Route("/v1", func(r chi.Router) {
		// Apply auth middleware only to /v1 routes
		if cfg.BackendAPIKey != "" {
			r.Use(APIKeyAuth(cfg.BackendAPIKey))
		}

		// Projects
		r.Get("/projects", h.ListProjects)
		r.Post("/projects", h.CreateProject)
		r.Get("/projects/{id}", h.GetProject)
		r.Get("/projects/{id}/download", h.GetProjectDownload)
		r.Get("/projects/{id}/debug/jobs", h.GetProjectJobs)

		// Clips
		r.Get("/projects/{projectId}/clips/{clipId}", h.GetClip)

		// Presets — available creative options for project creation
		r.Get("/presets/tones", h.ListTonePresets)
		r.Get("/presets/visual-styles", h.ListVisualStylePresets)
	})

	return r
}
