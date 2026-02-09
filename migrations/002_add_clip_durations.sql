-- Add estimated_duration_sec (from AI plan) and rendered_duration_ms (actual) to clips table.
-- These columns help track whether xAI video generation tokens are being wasted
-- by comparing estimated vs actual clip durations.

ALTER TABLE clips ADD COLUMN IF NOT EXISTS estimated_duration_sec INTEGER;
ALTER TABLE clips ADD COLUMN IF NOT EXISTS rendered_duration_ms INTEGER;
