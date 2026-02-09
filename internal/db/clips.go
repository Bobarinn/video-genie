package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bobarin/faceless/internal/models"
	"github.com/google/uuid"
)

func (db *DB) CreateClip(ctx context.Context, clip *models.Clip) error {
	query := `
		INSERT INTO clips (
			id, project_id, clip_index, script, voice_style_instruction,
			image_prompt, video_prompt, estimated_duration_sec, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING created_at, updated_at
	`

	return db.QueryRowContext(
		ctx, query,
		clip.ID, clip.ProjectID, clip.ClipIndex, clip.Script,
		clip.VoiceStyleInstruction, clip.ImagePrompt, clip.VideoPrompt,
		clip.EstimatedDurationSec, clip.Status,
	).Scan(&clip.CreatedAt, &clip.UpdatedAt)
}

func (db *DB) GetClip(ctx context.Context, id uuid.UUID) (*models.Clip, error) {
	query := `
		SELECT
			id, project_id, clip_index, script, voice_style_instruction,
			image_prompt, video_prompt, estimated_duration_sec, status,
			audio_asset_id, image_asset_id, clip_video_asset_id,
			audio_duration_ms, rendered_duration_ms, error_message,
			created_at, updated_at
		FROM clips
		WHERE id = $1
	`

	clip := &models.Clip{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&clip.ID, &clip.ProjectID, &clip.ClipIndex, &clip.Script,
		&clip.VoiceStyleInstruction, &clip.ImagePrompt, &clip.VideoPrompt,
		&clip.EstimatedDurationSec, &clip.Status,
		&clip.AudioAssetID, &clip.ImageAssetID, &clip.ClipVideoAssetID,
		&clip.AudioDurationMs, &clip.RenderedDurationMs, &clip.ErrorMessage,
		&clip.CreatedAt, &clip.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("clip not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get clip: %w", err)
	}

	return clip, nil
}

func (db *DB) GetProjectClips(ctx context.Context, projectID uuid.UUID) ([]models.Clip, error) {
	query := `
		SELECT
			id, project_id, clip_index, script, voice_style_instruction,
			image_prompt, video_prompt, estimated_duration_sec, status,
			audio_asset_id, image_asset_id, clip_video_asset_id,
			audio_duration_ms, rendered_duration_ms, error_message,
			created_at, updated_at
		FROM clips
		WHERE project_id = $1
		ORDER BY clip_index
	`

	rows, err := db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query clips: %w", err)
	}
	defer rows.Close()

	var clips []models.Clip
	for rows.Next() {
		var clip models.Clip
		err := rows.Scan(
			&clip.ID, &clip.ProjectID, &clip.ClipIndex, &clip.Script,
			&clip.VoiceStyleInstruction, &clip.ImagePrompt, &clip.VideoPrompt,
			&clip.EstimatedDurationSec, &clip.Status,
			&clip.AudioAssetID, &clip.ImageAssetID, &clip.ClipVideoAssetID,
			&clip.AudioDurationMs, &clip.RenderedDurationMs, &clip.ErrorMessage,
			&clip.CreatedAt, &clip.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan clip: %w", err)
		}
		clips = append(clips, clip)
	}

	return clips, nil
}

func (db *DB) UpdateClipStatus(ctx context.Context, id uuid.UUID, status models.ClipStatus) error {
	query := `UPDATE clips SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := db.ExecContext(ctx, query, status, id)
	return err
}

func (db *DB) UpdateClipAudio(ctx context.Context, id, assetID uuid.UUID, durationMs int) error {
	query := `
		UPDATE clips
		SET audio_asset_id = $1, audio_duration_ms = $2, status = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := db.ExecContext(ctx, query, assetID, durationMs, models.ClipStatusVoiced, id)
	return err
}

func (db *DB) UpdateClipImage(ctx context.Context, id, assetID uuid.UUID) error {
	query := `
		UPDATE clips
		SET image_asset_id = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := db.ExecContext(ctx, query, assetID, models.ClipStatusImaged, id)
	return err
}

func (db *DB) UpdateClipVideo(ctx context.Context, id, assetID uuid.UUID) error {
	query := `
		UPDATE clips
		SET clip_video_asset_id = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := db.ExecContext(ctx, query, assetID, models.ClipStatusRendered, id)
	return err
}

func (db *DB) UpdateClipError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	query := `
		UPDATE clips
		SET status = $1, error_message = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := db.ExecContext(ctx, query, models.ClipStatusFailed, errorMessage, id)
	return err
}

// GetProjectThumbnailAssetID returns the image_asset_id of clip_index=0 for a project.
// Returns nil if clip 0 doesn't exist or has no image yet.
func (db *DB) GetProjectThumbnailAssetID(ctx context.Context, projectID uuid.UUID) (*uuid.UUID, error) {
	query := `SELECT image_asset_id FROM clips WHERE project_id = $1 AND clip_index = 0 LIMIT 1`
	var assetID *uuid.UUID
	err := db.QueryRowContext(ctx, query, projectID).Scan(&assetID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return assetID, nil
}

// GetProjectClipCount returns the number of clips for a project.
func (db *DB) GetProjectClipCount(ctx context.Context, projectID uuid.UUID) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clips WHERE project_id = $1`, projectID).Scan(&count)
	return count, err
}

func (db *DB) UpdateClipRenderedDuration(ctx context.Context, id uuid.UUID, durationMs int) error {
	query := `UPDATE clips SET rendered_duration_ms = $1, updated_at = NOW() WHERE id = $2`
	_, err := db.ExecContext(ctx, query, durationMs, id)
	return err
}

func (db *DB) AreAllClipsRendered(ctx context.Context, projectID uuid.UUID) (bool, error) {
	query := `
		SELECT COUNT(*) = 0
		FROM clips
		WHERE project_id = $1 AND status != $2
	`

	var allRendered bool
	err := db.QueryRowContext(ctx, query, projectID, models.ClipStatusRendered).Scan(&allRendered)
	return allRendered, err
}
