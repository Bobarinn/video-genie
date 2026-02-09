package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bobarin/faceless/internal/models"
	"github.com/google/uuid"
)

func (db *DB) CreateAsset(ctx context.Context, asset *models.Asset) error {
	query := `
		INSERT INTO assets (
			id, project_id, clip_id, type, storage_bucket,
			storage_path, content_type, byte_size
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at
	`

	return db.QueryRowContext(
		ctx, query,
		asset.ID, asset.ProjectID, asset.ClipID, asset.Type,
		asset.StorageBucket, asset.StoragePath, asset.ContentType, asset.ByteSize,
	).Scan(&asset.CreatedAt)
}

func (db *DB) GetAsset(ctx context.Context, id uuid.UUID) (*models.Asset, error) {
	query := `
		SELECT
			id, project_id, clip_id, type, storage_bucket,
			storage_path, content_type, byte_size, created_at
		FROM assets
		WHERE id = $1
	`

	asset := &models.Asset{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&asset.ID, &asset.ProjectID, &asset.ClipID, &asset.Type,
		&asset.StorageBucket, &asset.StoragePath, &asset.ContentType,
		&asset.ByteSize, &asset.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("asset not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get asset: %w", err)
	}

	return asset, nil
}

func (db *DB) GetProjectAssets(ctx context.Context, projectID uuid.UUID) ([]models.Asset, error) {
	query := `
		SELECT
			id, project_id, clip_id, type, storage_bucket,
			storage_path, content_type, byte_size, created_at
		FROM assets
		WHERE project_id = $1
		ORDER BY created_at
	`

	rows, err := db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to query assets: %w", err)
	}
	defer rows.Close()

	var assets []models.Asset
	for rows.Next() {
		var asset models.Asset
		err := rows.Scan(
			&asset.ID, &asset.ProjectID, &asset.ClipID, &asset.Type,
			&asset.StorageBucket, &asset.StoragePath, &asset.ContentType,
			&asset.ByteSize, &asset.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan asset: %w", err)
		}
		assets = append(assets, asset)
	}

	return assets, nil
}
