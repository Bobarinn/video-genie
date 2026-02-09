package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/bobarin/faceless/internal/db"
	"github.com/bobarin/faceless/internal/models"
	"github.com/bobarin/faceless/internal/queue"
	"github.com/bobarin/faceless/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handler struct {
	db      *db.DB
	queue   *queue.Queue
	storage *storage.Storage
}

func NewHandler(database *db.DB, q *queue.Queue, stor *storage.Storage) *Handler {
	return &Handler{
		db:      database,
		queue:   q,
		storage: stor,
	}
}

// CreateProject handles POST /v1/projects
func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate
	if req.Topic == "" {
		respondError(w, http.StatusBadRequest, "Topic is required")
		return
	}

	// Set defaults
	targetDuration := 105
	if req.TargetDurationSeconds != nil {
		targetDuration = *req.TargetDurationSeconds
	}

	// Get graphics preset
	var presetID uuid.UUID
	if req.GraphicsPresetID != nil {
		presetID = *req.GraphicsPresetID
	} else {
		// Get default preset
		preset, err := h.db.GetDefaultGraphicsPreset(r.Context())
		if err != nil {
			respondError(w, http.StatusInternalServerError, "Failed to get default preset")
			return
		}
		presetID = preset.ID
	}

	// Create project
	project := &models.Project{
		ID:                    uuid.New(),
		SeriesID:              req.SeriesID,
		Topic:                 req.Topic,
		TargetDurationSeconds: targetDuration,
		GraphicsPresetID:      &presetID,
		Status:                models.ProjectStatusQueued,
		PlanVersion:           1,
	}

	if err := h.db.CreateProject(r.Context(), project); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create project")
		return
	}

	// Create and enqueue job
	jobID := uuid.New()
	job := &models.Job{
		ID:        jobID,
		ProjectID: project.ID,
		Type:      "generate_plan",
		Status:    models.JobStatusQueued,
	}

	if err := h.db.CreateJob(r.Context(), job); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}

	if err := h.queue.EnqueueGeneratePlan(r.Context(), project.ID, jobID); err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to enqueue job")
		return
	}

	// Return response
	respondJSON(w, http.StatusCreated, models.CreateProjectResponse{
		ProjectID: project.ID,
		Status:    project.Status,
	})
}

// ListProjects handles GET /v1/projects
// Query params:
//   - status: filter by project status (queued, planning, generating, rendering, completed, failed)
//   - limit:  max results per page (default 20, max 100)
//   - offset: number of results to skip (default 0)
func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	// Parse query params
	statusFilter := r.URL.Query().Get("status")
	if statusFilter != "" {
		// Validate status value
		switch models.ProjectStatus(statusFilter) {
		case models.ProjectStatusQueued, models.ProjectStatusPlanning,
			models.ProjectStatusGenerating, models.ProjectStatusRendering,
			models.ProjectStatusCompleted, models.ProjectStatusFailed:
			// valid
		default:
			respondError(w, http.StatusBadRequest, "Invalid status filter. Allowed: queued, planning, generating, rendering, completed, failed")
			return
		}
	}

	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > 100 {
		limit = 100
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get total count
	total, err := h.db.CountProjects(r.Context(), statusFilter)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to count projects")
		return
	}

	// Get projects
	projects, err := h.db.ListProjects(r.Context(), statusFilter, limit, offset)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to list projects")
		return
	}

	// Build lightweight summaries — no clips array, just thumbnail + final video URL
	summaries := make([]models.ProjectSummary, 0, len(projects))
	for _, project := range projects {
		summary := models.ProjectSummary{
			ID:                    project.ID,
			Topic:                 project.Topic,
			TargetDurationSeconds: project.TargetDurationSeconds,
			Status:                project.Status,
			ErrorCode:             project.ErrorCode,
			ErrorMessage:          project.ErrorMessage,
			CreatedAt:             project.CreatedAt,
			UpdatedAt:             project.UpdatedAt,
		}

		// Clip count
		if count, err := h.db.GetProjectClipCount(r.Context(), project.ID); err == nil {
			summary.ClipCount = count
		}

		// Thumbnail: clip 0's image URL
		if thumbAssetID, err := h.db.GetProjectThumbnailAssetID(r.Context(), project.ID); err == nil && thumbAssetID != nil {
			if asset, err := h.db.GetAsset(r.Context(), *thumbAssetID); err == nil {
				url := h.storage.GetPublicURL(asset.StoragePath)
				summary.ThumbnailURL = &url
			}
		}

		// Final video URL
		if project.FinalVideoAssetID != nil {
			if asset, err := h.db.GetAsset(r.Context(), *project.FinalVideoAssetID); err == nil {
				url := h.storage.GetPublicURL(asset.StoragePath)
				summary.FinalVideoURL = &url
			}
		}

		summaries = append(summaries, summary)
	}

	respondJSON(w, http.StatusOK, models.ListProjectsResponse{
		Projects: summaries,
		Total:    total,
		Limit:    limit,
		Offset:   offset,
	})
}

// GetProject handles GET /v1/projects/{id}
func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	project, err := h.db.GetProject(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	// Get clips
	clips, err := h.db.GetProjectClips(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get clips")
		return
	}

	// Get graphics preset
	var preset *models.GraphicsPreset
	if project.GraphicsPresetID != nil {
		preset, _ = h.db.GetGraphicsPreset(r.Context(), *project.GraphicsPresetID)
	}

	// Build response
	response := models.ProjectResponse{
		Project:        *project,
		Clips:          h.buildClipResponses(r.Context(), clips),
		GraphicsPreset: preset,
	}

	// Add final video URL if available
	if project.FinalVideoAssetID != nil {
		asset, err := h.db.GetAsset(r.Context(), *project.FinalVideoAssetID)
		if err == nil {
			url := h.storage.GetPublicURL(asset.StoragePath)
			response.FinalVideoURL = &url
		}
	}

	respondJSON(w, http.StatusOK, response)
}

// GetProjectDownload handles GET /v1/projects/{id}/download
func (h *Handler) GetProjectDownload(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	project, err := h.db.GetProject(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Project not found")
		return
	}

	if project.FinalVideoAssetID == nil {
		respondError(w, http.StatusNotFound, "Video not ready")
		return
	}

	asset, err := h.db.GetAsset(r.Context(), *project.FinalVideoAssetID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Asset not found")
		return
	}

	// Get signed URL (valid for 1 hour)
	signedURL, err := h.storage.GetSignedURL(r.Context(), asset.StoragePath, 3600)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to generate download URL")
		return
	}

	http.Redirect(w, r, signedURL, http.StatusTemporaryRedirect)
}

// GetProjectJobs handles GET /v1/projects/{id}/debug/jobs
func (h *Handler) GetProjectJobs(w http.ResponseWriter, r *http.Request) {
	projectID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid project ID")
		return
	}

	jobs, err := h.db.GetProjectJobs(r.Context(), projectID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get jobs")
		return
	}

	respondJSON(w, http.StatusOK, jobs)
}

// GetClip handles GET /v1/projects/{projectId}/clips/{clipId}
func (h *Handler) GetClip(w http.ResponseWriter, r *http.Request) {
	clipID, err := uuid.Parse(chi.URLParam(r, "clipId"))
	if err != nil {
		respondError(w, http.StatusBadRequest, "Invalid clip ID")
		return
	}

	clip, err := h.db.GetClip(r.Context(), clipID)
	if err != nil {
		respondError(w, http.StatusNotFound, "Clip not found")
		return
	}

	response := h.buildClipResponse(r.Context(), *clip)
	respondJSON(w, http.StatusOK, response)
}

// Helper methods
func (h *Handler) buildClipResponses(ctx context.Context, clips []models.Clip) []models.ClipResponse {
	responses := make([]models.ClipResponse, len(clips))
	for i, clip := range clips {
		responses[i] = h.buildClipResponse(ctx, clip)
	}
	return responses
}

func (h *Handler) buildClipResponse(ctx context.Context, clip models.Clip) models.ClipResponse {
	response := models.ClipResponse{
		Clip: clip,
	}

	// Add asset URLs
	if clip.AudioAssetID != nil {
		if asset, err := h.db.GetAsset(ctx, *clip.AudioAssetID); err == nil {
			url := h.storage.GetPublicURL(asset.StoragePath)
			response.AudioURL = &url
		}
	}

	if clip.ImageAssetID != nil {
		if asset, err := h.db.GetAsset(ctx, *clip.ImageAssetID); err == nil {
			url := h.storage.GetPublicURL(asset.StoragePath)
			response.ImageURL = &url
		}
	}

	if clip.ClipVideoAssetID != nil {
		if asset, err := h.db.GetAsset(ctx, *clip.ClipVideoAssetID); err == nil {
			url := h.storage.GetPublicURL(asset.StoragePath)
			response.ClipVideoURL = &url
		}
	}

	return response
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

// Health check
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
