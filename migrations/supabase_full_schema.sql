-- =============================================================================
-- FULL SCHEMA — Supabase-compatible, idempotent
--
-- Run this ONCE in the Supabase SQL Editor (Dashboard → SQL Editor → New Query)
-- or via psql: psql "$DATABASE_URL" -f migrations/supabase_full_schema.sql
--
-- It combines migrations 001–006 with IF NOT EXISTS / DO NOTHING guards
-- so it's safe to run multiple times.
-- =============================================================================

-- ═════════════════════════════════════════════════════════════════════════════
-- 001: Core schema
-- ═════════════════════════════════════════════════════════════════════════════

-- Enable UUID extension (Supabase has it already, but just in case)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enums (idempotent via DO block)
DO $$ BEGIN
    CREATE TYPE project_status AS ENUM ('queued', 'planning', 'generating', 'rendering', 'completed', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE clip_status AS ENUM ('pending', 'voiced', 'imaged', 'rendered', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE asset_type AS ENUM ('plan_json', 'audio', 'image', 'clip_video', 'final_video', 'logs');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

DO $$ BEGIN
    CREATE TYPE job_status AS ENUM ('queued', 'running', 'succeeded', 'failed');
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Series table
CREATE TABLE IF NOT EXISTS series (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    description TEXT,
    guidance TEXT,
    sample_script TEXT,
    default_graphics_preset_id UUID,
    default_voice_profile JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Graphics presets table
CREATE TABLE IF NOT EXISTS graphics_presets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    style_json JSONB NOT NULL,
    prompt_addition TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID,
    series_id UUID REFERENCES series(id),
    topic TEXT NOT NULL,
    target_duration_seconds INTEGER DEFAULT 105,
    graphics_preset_id UUID REFERENCES graphics_presets(id),
    status project_status DEFAULT 'queued',
    plan_version INTEGER DEFAULT 1,
    final_video_asset_id UUID,
    error_code TEXT,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Clips table
CREATE TABLE IF NOT EXISTS clips (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    clip_index INTEGER NOT NULL,
    script TEXT NOT NULL,
    voice_style_instruction TEXT,
    image_prompt TEXT NOT NULL,
    video_prompt TEXT,
    status clip_status DEFAULT 'pending',
    audio_asset_id UUID,
    image_asset_id UUID,
    clip_video_asset_id UUID,
    audio_duration_ms INTEGER,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(project_id, clip_index)
);

-- Assets table
CREATE TABLE IF NOT EXISTS assets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    type asset_type NOT NULL,
    storage_bucket TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    content_type TEXT,
    byte_size BIGINT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Jobs table
CREATE TABLE IF NOT EXISTS jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    clip_id UUID REFERENCES clips(id) ON DELETE CASCADE,
    type TEXT NOT NULL,
    status job_status DEFAULT 'queued',
    attempts INTEGER DEFAULT 0,
    started_at TIMESTAMP WITH TIME ZONE,
    finished_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    logs_asset_id UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX IF NOT EXISTS idx_projects_status ON projects(status);
CREATE INDEX IF NOT EXISTS idx_projects_created_at ON projects(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_clips_project_id ON clips(project_id);
CREATE INDEX IF NOT EXISTS idx_clips_status ON clips(status);
CREATE INDEX IF NOT EXISTS idx_assets_project_id ON assets(project_id);
CREATE INDEX IF NOT EXISTS idx_assets_clip_id ON assets(clip_id);
CREATE INDEX IF NOT EXISTS idx_jobs_project_id ON jobs(project_id);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);

-- Foreign key: projects.final_video_asset_id → assets.id
DO $$ BEGIN
    ALTER TABLE projects ADD CONSTRAINT fk_projects_final_video_asset
        FOREIGN KEY (final_video_asset_id) REFERENCES assets(id);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- Foreign key: series.default_graphics_preset_id → graphics_presets.id
DO $$ BEGIN
    ALTER TABLE series ADD CONSTRAINT fk_series_default_graphics_preset
        FOREIGN KEY (default_graphics_preset_id) REFERENCES graphics_presets(id);
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

-- updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers (drop-if-exists + create for idempotency)
DROP TRIGGER IF EXISTS update_series_updated_at ON series;
CREATE TRIGGER update_series_updated_at BEFORE UPDATE ON series
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_graphics_presets_updated_at ON graphics_presets;
CREATE TRIGGER update_graphics_presets_updated_at BEFORE UPDATE ON graphics_presets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_projects_updated_at ON projects;
CREATE TRIGGER update_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_clips_updated_at ON clips;
CREATE TRIGGER update_clips_updated_at BEFORE UPDATE ON clips
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Default graphics preset (seed)
INSERT INTO graphics_presets (id, name, style_json, prompt_addition) VALUES (
    'f47ac10b-58cc-4372-a567-0e02b2c3d479',
    'Luminous Regal',
    '{
        "color_palette": ["deep purples", "golds", "blacks"],
        "lighting": "dramatic high-contrast with soft glows",
        "composition": "cinematic wide shots with centered subjects",
        "mood": "mysterious, elegant, authoritative",
        "detail_level": "high detail with smooth gradients"
    }',
    'Cinematic quality, 8K resolution, professional photography, award-winning composition'
) ON CONFLICT (id) DO NOTHING;


-- ═════════════════════════════════════════════════════════════════════════════
-- 002: Add clip duration tracking columns
-- ═════════════════════════════════════════════════════════════════════════════

ALTER TABLE clips ADD COLUMN IF NOT EXISTS estimated_duration_sec INTEGER;
ALTER TABLE clips ADD COLUMN IF NOT EXISTS rendered_duration_ms INTEGER;


-- ═════════════════════════════════════════════════════════════════════════════
-- 003: Add per-project customization fields
-- ═════════════════════════════════════════════════════════════════════════════

ALTER TABLE projects ADD COLUMN IF NOT EXISTS tone TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS visual_style TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS aspect_ratio TEXT DEFAULT '9:16';
ALTER TABLE projects ADD COLUMN IF NOT EXISTS voice_id TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS cta TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS music_mood TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS sample_image_url TEXT;
ALTER TABLE projects ADD COLUMN IF NOT EXISTS language TEXT DEFAULT 'en';


-- ═════════════════════════════════════════════════════════════════════════════
-- 004: Enable Row Level Security on all tables
-- ═════════════════════════════════════════════════════════════════════════════

ALTER TABLE series           ENABLE ROW LEVEL SECURITY;
ALTER TABLE graphics_presets ENABLE ROW LEVEL SECURITY;
ALTER TABLE projects         ENABLE ROW LEVEL SECURITY;
ALTER TABLE clips            ENABLE ROW LEVEL SECURITY;
ALTER TABLE assets           ENABLE ROW LEVEL SECURITY;
ALTER TABLE jobs             ENABLE ROW LEVEL SECURITY;

ALTER TABLE series           FORCE ROW LEVEL SECURITY;
ALTER TABLE graphics_presets FORCE ROW LEVEL SECURITY;
ALTER TABLE projects         FORCE ROW LEVEL SECURITY;
ALTER TABLE clips            FORCE ROW LEVEL SECURITY;
ALTER TABLE assets           FORCE ROW LEVEL SECURITY;
ALTER TABLE jobs             FORCE ROW LEVEL SECURITY;


-- ═════════════════════════════════════════════════════════════════════════════
-- 005: Users table and user_id foreign keys
-- ═════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS users (
    id           UUID PRIMARY KEY,
    email        TEXT NOT NULL UNIQUE,
    display_name TEXT,
    avatar_url   TEXT,
    plan         TEXT DEFAULT 'free',
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at DESC);

DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

-- Wire projects.user_id → users.id
DO $$ BEGIN
    ALTER TABLE projects
        ADD CONSTRAINT fk_projects_user
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL;
EXCEPTION WHEN duplicate_object THEN NULL;
END $$;

CREATE INDEX IF NOT EXISTS idx_projects_user_id ON projects(user_id);

-- Add user_id to series
ALTER TABLE series ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_series_user_id ON series(user_id);


-- ═════════════════════════════════════════════════════════════════════════════
-- 006: Tone presets + visual style presets (seed data)
-- ═════════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS tone_presets (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug         TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    description  TEXT NOT NULL,
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

DROP TRIGGER IF EXISTS update_tone_presets_updated_at ON tone_presets;
CREATE TRIGGER update_tone_presets_updated_at BEFORE UPDATE ON tone_presets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

ALTER TABLE tone_presets ENABLE ROW LEVEL SECURITY;
ALTER TABLE tone_presets FORCE ROW LEVEL SECURITY;

-- Seed tone presets
INSERT INTO tone_presets (slug, display_name, description) VALUES
('documentary', 'Documentary', 'Narrate the story in a factual, informative, and authoritative manner. Focus on clarity, context, and historical accuracy. Use neutral but engaging language, smooth pacing, and a confident narrator tone that feels educational rather than dramatic.'),
('dramatic', 'Dramatic', 'Tell the story with heightened emotion and tension. Use vivid language, suspenseful pacing, and emotionally charged narration. Emphasize conflict, stakes, and turning points to keep the viewer emotionally invested.'),
('mysterious', 'Mysterious', 'Present the story as an unfolding mystery. Use curiosity-driven language, slower pacing, and subtle suspense. Ask implicit questions, reveal information gradually, and maintain an atmosphere of intrigue throughout.'),
('inspirational', 'Inspirational', 'Frame the story as uplifting and motivational. Highlight resilience, achievement, and hope. Use warm, encouraging language and a confident, optimistic narrator tone that leaves the viewer feeling inspired.'),
('educational_simplified', 'Educational (Simplified)', 'Explain the story in a clear, simple, and accessible way. Avoid jargon, keep sentences short, and prioritize understanding. The narration should feel like a skilled teacher explaining complex ideas in an easy-to-grasp manner.'),
('storytelling', 'Storytelling', 'Tell the story as a narrative with a beginning, middle, and end. Focus on flow, character, and progression rather than facts alone. Use natural, engaging language that feels like someone telling a story aloud.'),
('cinematic', 'Cinematic', 'Deliver the story with a cinematic feel. Use strong imagery, deliberate pacing, and powerful narration. Treat each moment like a scene in a film, emphasizing atmosphere, scale, and emotional impact.'),
('calm_reflective', 'Calm & Reflective', 'Narrate in a calm, thoughtful, and reflective tone. Use slower pacing and gentle language. The story should feel meditative, allowing the viewer to absorb ideas without urgency or tension.')
ON CONFLICT (slug) DO NOTHING;

-- Add slug + description to graphics_presets
ALTER TABLE graphics_presets ADD COLUMN IF NOT EXISTS slug TEXT UNIQUE;
ALTER TABLE graphics_presets ADD COLUMN IF NOT EXISTS description TEXT;

-- Update legacy preset with slug
UPDATE graphics_presets
SET slug = 'luminous_regal',
    description = 'Generate visuals in a luminous, regal editorial watercolor style. Use deep purples, golds, and blacks with dramatic high-contrast lighting and soft glows. Composition should be cinematic with centered subjects. The mood is mysterious, elegant, and authoritative with high detail and smooth gradients.'
WHERE id = 'f47ac10b-58cc-4372-a567-0e02b2c3d479';

-- Seed visual style presets
INSERT INTO graphics_presets (id, slug, name, description, style_json, prompt_addition) VALUES
(uuid_generate_v4(), 'cinematic_watercolor', 'Cinematic Watercolor', 'Generate painterly visuals that resemble high-quality watercolor illustrations with cinematic lighting. Use visible paper texture, soft gradients, and warm, glowing highlights. Avoid hard outlines. Scenes should feel artistic, atmospheric, and emotionally rich.', '{"style": "watercolor", "lighting": "cinematic warm", "texture": "paper"}', 'Cinematic watercolor illustration, painterly brush strokes, warm atmospheric lighting'),
(uuid_generate_v4(), 'illustrated_editorial', 'Illustrated Editorial', 'Create clean, stylized editorial illustrations with strong composition and clear subject focus. Use bold color blocks, subtle texture, and controlled lighting. The visuals should feel modern, expressive, and suitable for storytelling articles or magazines.', '{"style": "editorial illustration", "lighting": "controlled studio", "texture": "subtle"}', 'Clean editorial illustration, bold color blocks, strong composition, modern expressive style'),
(uuid_generate_v4(), 'anime_inspired', 'Anime Inspired', 'Render visuals inspired by anime-style illustration. Use expressive characters, clear facial features, dynamic framing, and dramatic lighting. Maintain a polished, hand-drawn look rather than hyper-realism.', '{"style": "anime", "lighting": "dramatic", "texture": "cel-shaded"}', 'Anime-style illustration, expressive characters, dynamic framing, polished hand-drawn look'),
(uuid_generate_v4(), 'cartoon_stylized', 'Cartoon Stylized', 'Produce simplified, stylized cartoon visuals with exaggerated shapes and clear silhouettes. Use bright but balanced colors and minimal texture. The visuals should feel playful, approachable, and easy to read at a glance.', '{"style": "cartoon", "lighting": "bright flat", "texture": "minimal"}', 'Stylized cartoon, exaggerated shapes, bright balanced colors, playful and approachable'),
(uuid_generate_v4(), 'hyper_realistic', 'Hyper Realistic', 'Generate highly realistic visuals with lifelike lighting, textures, and depth. Scenes should resemble cinematic photography or film stills. Pay close attention to realism, scale, and environmental detail.', '{"style": "photorealistic", "lighting": "natural cinematic", "texture": "high detail"}', 'Hyper-realistic photography, cinematic film still, lifelike lighting and textures, extreme detail'),
(uuid_generate_v4(), 'digital_painting', 'Digital Painting', 'Create polished digital paintings with visible brush strokes and rich color blending. Use dramatic lighting and strong contrast while maintaining an artistic, hand-crafted feel rather than photorealism.', '{"style": "digital painting", "lighting": "dramatic contrast", "texture": "visible brush strokes"}', 'Polished digital painting, visible brush strokes, rich color blending, dramatic lighting'),
(uuid_generate_v4(), 'minimalist_abstract', 'Minimalist Abstract', 'Generate abstract, minimal visuals that suggest ideas rather than literal scenes. Use simple shapes, limited color palettes, and symbolic imagery. Focus on mood and concept over detail.', '{"style": "minimalist abstract", "lighting": "flat ambient", "texture": "clean geometric"}', 'Minimalist abstract art, simple shapes, limited color palette, symbolic imagery, mood over detail'),
(uuid_generate_v4(), 'low_poly_3d', 'Low Poly 3D', 'Render scenes using low-poly 3D aesthetics. Use geometric forms, flat shading, and simplified environments. The visuals should feel modern, clean, and stylized rather than realistic.', '{"style": "low poly 3D", "lighting": "flat shading", "texture": "geometric faceted"}', 'Low-poly 3D render, geometric forms, flat shading, simplified clean environments, modern stylized')
ON CONFLICT (slug) DO NOTHING;

-- Indexes for slug lookups
CREATE INDEX IF NOT EXISTS idx_graphics_presets_slug ON graphics_presets(slug);
CREATE INDEX IF NOT EXISTS idx_tone_presets_slug ON tone_presets(slug);


-- ═════════════════════════════════════════════════════════════════════════════
-- Done! All tables, indexes, RLS, triggers, and seed data are in place.
-- ═════════════════════════════════════════════════════════════════════════════
