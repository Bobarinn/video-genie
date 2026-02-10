package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/bobarin/episod/internal/models"
	"github.com/google/uuid"
)

// CreateUser inserts a new user record.
// The ID should match the Supabase Auth user ID set by the backend after JWT verification.
func (db *DB) CreateUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, display_name, avatar_url, plan)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at
	`

	return db.QueryRowContext(
		ctx, query,
		user.ID, user.Email, user.DisplayName, user.AvatarURL, user.Plan,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
}

// GetUser retrieves a user by their ID.
func (db *DB) GetUser(ctx context.Context, id uuid.UUID) (*models.User, error) {
	query := `
		SELECT id, email, display_name, avatar_url, plan, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	user := &models.User{}
	err := db.QueryRowContext(ctx, query, id).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL,
		&user.Plan, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return user, nil
}

// GetUserByEmail retrieves a user by their email address.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	query := `
		SELECT id, email, display_name, avatar_url, plan, created_at, updated_at
		FROM users
		WHERE email = $1
	`

	user := &models.User{}
	err := db.QueryRowContext(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.DisplayName, &user.AvatarURL,
		&user.Plan, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}

	return user, nil
}

// UpsertUser creates or updates a user record.
// Useful for the "login or register" flow — after verifying a Supabase Auth JWT,
// the backend upserts the user to ensure a local record exists.
func (db *DB) UpsertUser(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (id, email, display_name, avatar_url, plan)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (id) DO UPDATE SET
			email = EXCLUDED.email,
			display_name = COALESCE(EXCLUDED.display_name, users.display_name),
			avatar_url = COALESCE(EXCLUDED.avatar_url, users.avatar_url),
			updated_at = NOW()
		RETURNING created_at, updated_at
	`

	return db.QueryRowContext(
		ctx, query,
		user.ID, user.Email, user.DisplayName, user.AvatarURL, user.Plan,
	).Scan(&user.CreatedAt, &user.UpdatedAt)
}

// UpdateUser updates mutable user profile fields.
func (db *DB) UpdateUser(ctx context.Context, id uuid.UUID, displayName, avatarURL *string) error {
	query := `
		UPDATE users
		SET display_name = COALESCE($1, display_name),
		    avatar_url = COALESCE($2, avatar_url),
		    updated_at = NOW()
		WHERE id = $3
	`
	result, err := db.ExecContext(ctx, query, displayName, avatarURL, id)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser removes a user record. Projects and series are preserved (ON DELETE SET NULL).
func (db *DB) DeleteUser(ctx context.Context, id uuid.UUID) error {
	result, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
