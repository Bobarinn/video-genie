-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create enums
CREATE TYPE project_status AS ENUM ('queued', 'planning', 'generating', 'rendering', 'completed', 'failed');
CREATE TYPE clip_status AS ENUM ('pending', 'voiced', 'imaged', 'rendered', 'failed');
CREATE TYPE asset_type AS ENUM ('plan_json', 'audio', 'image', 'clip_video', 'final_video', 'logs');
CREATE TYPE job_status AS ENUM ('queued', 'running', 'succeeded', 'failed');

-- Series table (future feature, but create now)
CREATE TABLE series (
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
CREATE TABLE graphics_presets (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name TEXT NOT NULL,
    style_json JSONB NOT NULL,
    prompt_addition TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Projects table
CREATE TABLE projects (
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
CREATE TABLE clips (
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
CREATE TABLE assets (
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
CREATE TABLE jobs (
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

-- Create indexes
CREATE INDEX idx_projects_status ON projects(status);
CREATE INDEX idx_projects_created_at ON projects(created_at DESC);
CREATE INDEX idx_clips_project_id ON clips(project_id);
CREATE INDEX idx_clips_status ON clips(status);
CREATE INDEX idx_assets_project_id ON assets(project_id);
CREATE INDEX idx_assets_clip_id ON assets(clip_id);
CREATE INDEX idx_jobs_project_id ON jobs(project_id);
CREATE INDEX idx_jobs_status ON jobs(status);

-- Add foreign key for final_video_asset_id (must be after assets table creation)
ALTER TABLE projects ADD CONSTRAINT fk_projects_final_video_asset
    FOREIGN KEY (final_video_asset_id) REFERENCES assets(id);

-- Add foreign key for series default_graphics_preset_id
ALTER TABLE series ADD CONSTRAINT fk_series_default_graphics_preset
    FOREIGN KEY (default_graphics_preset_id) REFERENCES graphics_presets(id);

-- Create updated_at trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Add triggers for updated_at
CREATE TRIGGER update_series_updated_at BEFORE UPDATE ON series
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_graphics_presets_updated_at BEFORE UPDATE ON graphics_presets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_projects_updated_at BEFORE UPDATE ON projects
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_clips_updated_at BEFORE UPDATE ON clips
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Insert default graphics preset
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
);
