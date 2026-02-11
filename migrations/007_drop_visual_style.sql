-- Migration 007: Remove visual_style column from projects
--
-- The visual_style free-text field is replaced by the graphics_preset_id
-- foreign key. All visual styling (name, description, style_json, prompt_addition)
-- now comes from the graphics_presets table.
--
-- This is a one-way migration. To roll back, re-add the column:
--   ALTER TABLE projects ADD COLUMN visual_style TEXT;

ALTER TABLE projects DROP COLUMN IF EXISTS visual_style;
