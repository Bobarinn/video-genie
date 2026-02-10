package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bobarin/episod/internal/models"
	"github.com/google/uuid"
)

func (db *DB) CreateJob(ctx context.Context, job *models.Job) error {
	query := `
		INSERT INTO jobs (
			id, project_id, clip_id, type, status, attempts
		) VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at
	`

	return db.QueryRowContext(
		ctx, query,
		job.ID, job.ProjectID, job.ClipID, job.Type, job.Status, job.Attempts,
	).Scan(&job.CreatedAt)
}

func (db *DB) GetJob(ctx context.Context, id uuid.UUID) (*models.Job, error) {
	query := `
		SELECT
			id, project_id, clip_id, type, status, attempts,
			started_at, finished_at, error_message, logs_asset_id, created_at
		FROM jobs
		WHERE id = $1
	`

	job := &models.Job{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&job.ID, &job.ProjectID, &job.ClipID, &job.Type, &job.Status,
		&job.Attempts, &job.StartedAt, &job.FinishedAt, &job.ErrorMessage,
		&job.LogsAssetID, &job.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

func (db *DB) GetProjectJobs(ctx context.Context, projectID uuid.UUID) ([]models.Job, error) {
	query := `
		SELECT
			id, project_id, clip_id, type, status, attempts,
			started_at, finished_at, error_message, logs_asset_id, created_at
		FROM jobs
		WHERE project_id = $1
		ORDER BY created_at
	`

	rows, err := db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query jobs: %w", err)
	}
	defer rows.Close()

	var jobs []models.Job
	for rows.Next() {
		var job models.Job
		err := rows.Scan(
			&job.ID, &job.ProjectID, &job.ClipID, &job.Type, &job.Status,
			&job.Attempts, &job.StartedAt, &job.FinishedAt, &job.ErrorMessage,
			&job.LogsAssetID, &job.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

func (db *DB) UpdateJobStatus(ctx context.Context, id uuid.UUID, status models.JobStatus) error {
	now := time.Now()
	query := `UPDATE jobs SET status = $1, started_at = $2 WHERE id = $3`

	if status == models.JobStatusSucceeded || status == models.JobStatusFailed {
		query = `UPDATE jobs SET status = $1, finished_at = $2 WHERE id = $3`
	}

	_, err := db.ExecContext(ctx, query, status, now, id)
	return err
}

func (db *DB) UpdateJobError(ctx context.Context, id uuid.UUID, errorMessage string) error {
	query := `
		UPDATE jobs
		SET status = $1, error_message = $2, finished_at = $3, attempts = attempts + 1
		WHERE id = $4
	`
	_, err := db.ExecContext(ctx, query, models.JobStatusFailed, errorMessage, time.Now(), id)
	return err
}
