package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Enums
type ProjectStatus string

const (
	ProjectStatusQueued     ProjectStatus = "queued"
	ProjectStatusPlanning   ProjectStatus = "planning"
	ProjectStatusGenerating ProjectStatus = "generating"
	ProjectStatusRendering  ProjectStatus = "rendering"
	ProjectStatusCompleted  ProjectStatus = "completed"
	ProjectStatusFailed     ProjectStatus = "failed"
)

type ClipStatus string

const (
	ClipStatusPending  ClipStatus = "pending"
	ClipStatusVoiced   ClipStatus = "voiced"
	ClipStatusImaged   ClipStatus = "imaged"
	ClipStatusRendered ClipStatus = "rendered"
	ClipStatusFailed   ClipStatus = "failed"
)

type AssetType string

const (
	AssetTypePlanJSON   AssetType = "plan_json"
	AssetTypeAudio      AssetType = "audio"
	AssetTypeImage      AssetType = "image"
	AssetTypeClipVideo  AssetType = "clip_video"
	AssetTypeFinalVideo AssetType = "final_video"
	AssetTypeLogs       AssetType = "logs"
)

type JobStatus string

const (
	JobStatusQueued    JobStatus = "queued"
	JobStatusRunning   JobStatus = "running"
	JobStatusSucceeded JobStatus = "succeeded"
	JobStatusFailed    JobStatus = "failed"
)

// JSONB is a custom type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// Models

type User struct {
	ID          uuid.UUID  `json:"id"`
	Email       string     `json:"email"`
	DisplayName *string    `json:"display_name,omitempty"`
	AvatarURL   *string    `json:"avatar_url,omitempty"`
	Plan        *string    `json:"plan,omitempty"` // "free", "pro", "enterprise"
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Series struct {
	ID                      uuid.UUID  `json:"id"`
	UserID                  *uuid.UUID `json:"user_id,omitempty"` // nil = system/global series
	Name                    string     `json:"name"`
	Description             *string    `json:"description,omitempty"`
	Guidance                *string    `json:"guidance,omitempty"`
	SampleScript            *string    `json:"sample_script,omitempty"`
	DefaultGraphicsPresetID *uuid.UUID `json:"default_graphics_preset_id,omitempty"`
	DefaultVoiceProfile     JSONB      `json:"default_voice_profile,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type GraphicsPreset struct {
	ID             uuid.UUID `json:"id"`
	Slug           *string   `json:"slug,omitempty"`            // Machine name, e.g. "cinematic_watercolor"
	Name           string    `json:"name"`                      // Display name
	Description    *string   `json:"description,omitempty"`     // Full AI prompt directive
	StyleJSON      JSONB     `json:"style_json"`
	PromptAddition *string   `json:"prompt_addition,omitempty"` // Short suffix for image prompt
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TonePreset struct {
	ID          uuid.UUID `json:"id"`
	Slug        string    `json:"slug"`         // Machine name, e.g. "documentary"
	DisplayName string    `json:"display_name"` // Human-readable, e.g. "Documentary"
	Description string    `json:"description"`  // Full AI prompt directive
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Project struct {
	ID                     uuid.UUID      `json:"id"`
	UserID                 *uuid.UUID     `json:"user_id,omitempty"`
	SeriesID               *uuid.UUID     `json:"series_id,omitempty"`
	Topic                  string         `json:"topic"`
	TargetDurationSeconds  int            `json:"target_duration_seconds"`
	GraphicsPresetID       *uuid.UUID     `json:"graphics_preset_id,omitempty"`
	Status                 ProjectStatus  `json:"status"`
	PlanVersion            int            `json:"plan_version"`
	FinalVideoAssetID      *uuid.UUID     `json:"final_video_asset_id,omitempty"`
	// Per-project customization (all optional, defaults applied at creation)
	Tone                   *string        `json:"tone,omitempty"`             // "documentary", "dramatic", "educational", etc.
	AspectRatio            *string        `json:"aspect_ratio,omitempty"`     // "9:16", "16:9", "1:1", "4:5"
	VoiceID                *string        `json:"voice_id,omitempty"`         // ElevenLabs voice override
	CTA                    *string        `json:"cta,omitempty"`              // Call-to-action for last clip
	MusicMood              *string        `json:"music_mood,omitempty"`       // "calm", "epic", "upbeat", etc.
	SampleImageURL         *string        `json:"sample_image_url,omitempty"` // Custom style reference image URL
	Language               *string        `json:"language,omitempty"`         // ISO 639-1: "en", "es", "fr", etc.
	ErrorCode              *string        `json:"error_code,omitempty"`
	ErrorMessage           *string        `json:"error_message,omitempty"`
	CreatedAt              time.Time      `json:"created_at"`
	UpdatedAt              time.Time      `json:"updated_at"`
}

type Clip struct {
	ID                    uuid.UUID   `json:"id"`
	ProjectID             uuid.UUID   `json:"project_id"`
	ClipIndex             int         `json:"clip_index"`
	Script                string      `json:"script"`
	VoiceStyleInstruction *string     `json:"voice_style_instruction,omitempty"`
	ImagePrompt           string      `json:"image_prompt"`
	VideoPrompt           *string     `json:"video_prompt,omitempty"`
	EstimatedDurationSec  *int        `json:"estimated_duration_sec,omitempty"` // From AI plan
	Status                ClipStatus  `json:"status"`
	AudioAssetID          *uuid.UUID  `json:"audio_asset_id,omitempty"`
	ImageAssetID          *uuid.UUID  `json:"image_asset_id,omitempty"`
	ClipVideoAssetID      *uuid.UUID  `json:"clip_video_asset_id,omitempty"`
	AudioDurationMs       *int        `json:"audio_duration_ms,omitempty"`
	RenderedDurationMs    *int        `json:"rendered_duration_ms,omitempty"` // Actual rendered clip duration
	ErrorMessage          *string     `json:"error_message,omitempty"`
	CreatedAt             time.Time   `json:"created_at"`
	UpdatedAt             time.Time   `json:"updated_at"`
}

type Asset struct {
	ID            uuid.UUID  `json:"id"`
	ProjectID     uuid.UUID  `json:"project_id"`
	ClipID        *uuid.UUID `json:"clip_id,omitempty"`
	Type          AssetType  `json:"type"`
	StorageBucket string     `json:"storage_bucket"`
	StoragePath   string     `json:"storage_path"`
	ContentType   *string    `json:"content_type,omitempty"`
	ByteSize      *int64     `json:"byte_size,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
}

type Job struct {
	ID           uuid.UUID  `json:"id"`
	ProjectID    uuid.UUID  `json:"project_id"`
	ClipID       *uuid.UUID `json:"clip_id,omitempty"`
	Type         string     `json:"type"`
	Status       JobStatus  `json:"status"`
	Attempts     int        `json:"attempts"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	ErrorMessage *string    `json:"error_message,omitempty"`
	LogsAssetID  *uuid.UUID `json:"logs_asset_id,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// DTOs for API responses
type ProjectResponse struct {
	Project
	Clips           []ClipResponse `json:"clips,omitempty"`
	FinalVideoURL   *string        `json:"final_video_url,omitempty"`
	GraphicsPreset  *GraphicsPreset `json:"graphics_preset,omitempty"`
}

type ClipResponse struct {
	Clip
	AudioURL     *string `json:"audio_url,omitempty"`
	ImageURL     *string `json:"image_url,omitempty"`
	ClipVideoURL *string `json:"clip_video_url,omitempty"`
}

// ProjectSummary is a lightweight DTO for the list endpoint — no clips array,
// just core project fields plus a thumbnail URL from clip 0's image.
type ProjectSummary struct {
	ID                    uuid.UUID      `json:"id"`
	Topic                 string         `json:"topic"`
	TargetDurationSeconds int            `json:"target_duration_seconds"`
	Tone                  *string        `json:"tone,omitempty"`
	Language              *string        `json:"language,omitempty"`
	Status                ProjectStatus  `json:"status"`
	ThumbnailURL          *string        `json:"thumbnail_url,omitempty"`
	FinalVideoURL         *string        `json:"final_video_url,omitempty"`
	ClipCount             int            `json:"clip_count"`
	ErrorCode             *string        `json:"error_code,omitempty"`
	ErrorMessage          *string        `json:"error_message,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
}

type ListProjectsResponse struct {
	Projects []ProjectSummary `json:"projects"`
	Total    int              `json:"total"`
	Limit    int              `json:"limit"`
	Offset   int              `json:"offset"`
}

type CreateProjectRequest struct {
	Topic                 string     `json:"topic"`
	TargetDurationSeconds *int       `json:"target_duration_seconds,omitempty"` // Default: 60
	GraphicsPresetID      *uuid.UUID `json:"graphics_preset_id,omitempty"`
	SeriesID              *uuid.UUID `json:"series_id,omitempty"`
	Tone                  *string    `json:"tone,omitempty"`             // Default: "documentary"
	AspectRatio           *string    `json:"aspect_ratio,omitempty"`     // Default: "9:16"
	VoiceID               *string    `json:"voice_id,omitempty"`         // Default: env ELEVENLABS_VOICE_ID
	CTA                   *string    `json:"cta,omitempty"`              // Optional call-to-action
	MusicMood             *string    `json:"music_mood,omitempty"`       // Optional music mood hint
	SampleImageURL        *string    `json:"sample_image_url,omitempty"` // Optional custom style reference
	Language              *string    `json:"language,omitempty"`         // Default: "en"
}

type CreateProjectResponse struct {
	ProjectID uuid.UUID     `json:"project_id"`
	Status    ProjectStatus `json:"status"`
}
