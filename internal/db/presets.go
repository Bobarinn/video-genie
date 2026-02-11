package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bobarin/episod/internal/models"
)

// GetTonePresetBySlug retrieves a tone preset by its slug (e.g. "documentary").
func (db *DB) GetTonePresetBySlug(ctx context.Context, slug string) (*models.TonePreset, error) {
	query := `
		SELECT id, slug, display_name, description, created_at, updated_at
		FROM tone_presets
		WHERE slug = $1
	`

	preset := &models.TonePreset{}
	err := db.QueryRowContext(ctx, query, slug).Scan(
		&preset.ID, &preset.Slug, &preset.DisplayName, &preset.Description,
		&preset.CreatedAt, &preset.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("tone preset not found: %s", slug)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get tone preset by slug: %w", err)
	}

	return preset, nil
}

// ListTonePresets returns all tone presets ordered by display name.
func (db *DB) ListTonePresets(ctx context.Context) ([]models.TonePreset, error) {
	query := `
		SELECT id, slug, display_name, description, created_at, updated_at
		FROM tone_presets
		ORDER BY display_name
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tone presets: %w", err)
	}
	defer rows.Close()

	var presets []models.TonePreset
	for rows.Next() {
		var p models.TonePreset
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.DisplayName, &p.Description,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan tone preset: %w", err)
		}
		presets = append(presets, p)
	}

	return presets, nil
}
