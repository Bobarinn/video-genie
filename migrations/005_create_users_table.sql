-- Migration 005: Create users table and wire up user_id foreign keys
--
-- This table stores app-level user profiles. The `id` is a UUID that matches
-- the Supabase Auth user ID (auth.users.id) — the Go backend sets it after
-- verifying the JWT from the frontend.
--
-- We intentionally do NOT add a FK to auth.users because:
--   1. The auth schema may not be accessible from the public schema.
--   2. It couples the app to Supabase Auth internals.
--   3. The Go backend is the gatekeeper — it only inserts verified user IDs.
--
-- Tables that gain a user_id FK:
--   - projects  (already has user_id column, just needs the FK constraint)
--   - series    (new column — series can be user-owned)

-- ── Users table ─────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS users (
    id           UUID PRIMARY KEY,                         -- matches Supabase Auth user ID
    email        TEXT NOT NULL UNIQUE,
    display_name TEXT,
    avatar_url   TEXT,
    plan         TEXT DEFAULT 'free',                      -- "free", "pro", "enterprise" (future billing)
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for email lookups (unique constraint already creates one, but explicit for clarity)
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);

-- Auto-update updated_at on changes
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ── Enable RLS on users table ───────────────────────────────────────────

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

-- ── Wire up projects.user_id → users.id ─────────────────────────────────
-- The column already exists (nullable UUID, no FK). Add the constraint.
-- ON DELETE SET NULL: if a user is deleted, their projects remain but become unowned.

ALTER TABLE projects
    ADD CONSTRAINT fk_projects_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;

-- Index for filtering projects by user (critical for "my projects" queries)
CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id);

-- ── Add user_id to series table ─────────────────────────────────────────
-- Series are user-owned (null = system/global series).

ALTER TABLE series ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_series_user_id ON series(user_id);
