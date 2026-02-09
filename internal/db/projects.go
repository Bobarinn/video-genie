package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bobarin/faceless/internal/models"
	"github.com/google/uuid"
)

func (db *DB) CreateProject(ctx context.Context, project *models.Project) error {
	query := `
		INSERT INTO projects (
			id, user_id, series_id, topic, target_duration_seconds,
			graphics_preset_id, status, plan_version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	return db.QueryRowContext(
		ctx, query,
		project.ID, project.UserID, project.SeriesID, project.Topic,
		project.TargetDurationSeconds, project.GraphicsPresetID,
		project.Status, project.PlanVersion,
	).Scan(&project.CreatedAt, &project.UpdatedAt)
}

func (db *DB) GetProject(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	query := `
		SELECT
			id, user_id, series_id, topic, target_duration_seconds,
			graphics_preset_id, status, plan_version, final_video_asset_id,
			error_code, error_message, created_at, updated_at
		FROM projects
		WHERE id = $1
	`

	project := &models.Project{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&project.ID, &project.UserID, &project.SeriesID, &project.Topic,
		&project.TargetDurationSeconds, &project.GraphicsPresetID,
		&project.Status, &project.PlanVersion, &project.FinalVideoAssetID,
		&project.ErrorCode, &project.ErrorMessage,
		&project.CreatedAt, &project.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// ListProjects returns projects ordered by creation date (newest first).
// Supports optional status filter, limit, and offset for pagination.
func (db *DB) ListProjects(ctx context.Context, status string, limit, offset int) ([]models.Project, error) {
	var (
		rows *sql.Rows
		err  error
	)

	baseSelect := `
		SELECT
			id, user_id, series_id, topic, target_duration_seconds,
			graphics_preset_id, status, plan_version, final_video_asset_id,
			error_code, error_message, created_at, updated_at
		FROM projects
	`

	if status != "" {
		query := baseSelect + ` WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		rows, err = db.QueryContext(ctx, query, status, limit, offset)
	} else {
		query := baseSelect + ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		rows, err = db.QueryContext(ctx, query, limit, offset)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	var projects []models.Project
	for rows.Next() {
		var p models.Project
		if err := rows.Scan(
			&p.ID, &p.UserID, &p.SeriesID, &p.Topic,
			&p.TargetDurationSeconds, &p.GraphicsPresetID,
			&p.Status, &p.PlanVersion, &p.FinalVideoAssetID,
			&p.ErrorCode, &p.ErrorMessage,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// CountProjects returns the total number of projects, optionally filtered by status.
func (db *DB) CountProjects(ctx context.Context, status string) (int, error) {
	var count int
	if status != "" {
		err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE status = $1`, status).Scan(&count)
		return count, err
	}
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&count)
	return count, err
}

func (db *DB) UpdateProjectStatus(ctx context.Context, id uuid.UUID, status models.ProjectStatus) error {
	query := `UPDATE projects SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := db.ExecContext(ctx, query, status, id)
	return err
}

func (db *DB) UpdateProjectError(ctx context.Context, id uuid.UUID, errorCode, errorMessage string) error {
	query := `
		UPDATE projects
		SET status = $1, error_code = $2, error_message = $3, updated_at = NOW()
		WHERE id = $4
	`
	_, err := db.ExecContext(ctx, query, models.ProjectStatusFailed, errorCode, errorMessage, id)
	return err
}

func (db *DB) SetProjectFinalVideo(ctx context.Context, projectID, assetID uuid.UUID) error {
	query := `
		UPDATE projects
		SET final_video_asset_id = $1, status = $2, updated_at = NOW()
		WHERE id = $3
	`
	_, err := db.ExecContext(ctx, query, assetID, models.ProjectStatusCompleted, projectID)
	return err
}

func (db *DB) GetGraphicsPreset(ctx context.Context, id uuid.UUID) (*models.GraphicsPreset, error) {
	query := `
		SELECT id, name, style_json, prompt_addition, created_at, updated_at
		FROM graphics_presets
		WHERE id = $1
	`

	preset := &models.GraphicsPreset{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&preset.ID, &preset.Name, &preset.StyleJSON,
		&preset.PromptAddition, &preset.CreatedAt, &preset.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("graphics preset not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get graphics preset: %w", err)
	}

	return preset, nil
}

func (db *DB) GetDefaultGraphicsPreset(ctx context.Context) (*models.GraphicsPreset, error) {
	query := `
		SELECT id, name, style_json, prompt_addition, created_at, updated_at
		FROM graphics_presets
		ORDER BY created_at
		LIMIT 1
	`

	preset := &models.GraphicsPreset{}
	err := db.QueryRowContext(ctx, query).Scan(
		&preset.ID, &preset.Name, &preset.StyleJSON,
		&preset.PromptAddition, &preset.CreatedAt, &preset.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no default graphics preset found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get default graphics preset: %w", err)
	}

	return preset, nil
}
