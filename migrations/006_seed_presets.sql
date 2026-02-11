-- Migration 006: Create tone_presets table, add slug to graphics_presets, seed both
--
-- These are the creative control layers for the app. Users pick a tone and visual
-- style from these presets; the AI prompt builder injects the description into
-- generation prompts to constrain the creative direction without boxing the AI in.
--
-- Design principle: enums describe INTENT, not TECHNIQUE.

-- ═══════════════════════════════════════════════════════════════════════════
-- TONE PRESETS — how the story is told
-- ═══════════════════════════════════════════════════════════════════════════

CREATE TABLE IF NOT EXISTS tone_presets (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slug         TEXT NOT NULL UNIQUE,                     -- machine name, e.g. "documentary"
    display_name TEXT NOT NULL,                            -- human-readable, e.g. "Documentary"
    description  TEXT NOT NULL,                            -- injected into OpenAI plan prompt
    created_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at   TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Auto-update updated_at
CREATE TRIGGER update_tone_presets_updated_at BEFORE UPDATE ON tone_presets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- RLS (consistent with all other tables)
ALTER TABLE tone_presets ENABLE ROW LEVEL SECURITY;
ALTER TABLE tone_presets FORCE ROW LEVEL SECURITY;

-- Seed the 8 tone presets
INSERT INTO tone_presets (slug, display_name, description) VALUES
(
    'documentary',
    'Documentary',
    'Narrate the story in a factual, informative, and authoritative manner. Focus on clarity, context, and historical accuracy. Use neutral but engaging language, smooth pacing, and a confident narrator tone that feels educational rather than dramatic.'
),
(
    'dramatic',
    'Dramatic',
    'Tell the story with heightened emotion and tension. Use vivid language, suspenseful pacing, and emotionally charged narration. Emphasize conflict, stakes, and turning points to keep the viewer emotionally invested.'
),
(
    'mysterious',
    'Mysterious',
    'Present the story as an unfolding mystery. Use curiosity-driven language, slower pacing, and subtle suspense. Ask implicit questions, reveal information gradually, and maintain an atmosphere of intrigue throughout.'
),
(
    'inspirational',
    'Inspirational',
    'Frame the story as uplifting and motivational. Highlight resilience, achievement, and hope. Use warm, encouraging language and a confident, optimistic narrator tone that leaves the viewer feeling inspired.'
),
(
    'educational_simplified',
    'Educational (Simplified)',
    'Explain the story in a clear, simple, and accessible way. Avoid jargon, keep sentences short, and prioritize understanding. The narration should feel like a skilled teacher explaining complex ideas in an easy-to-grasp manner.'
),
(
    'storytelling',
    'Storytelling',
    'Tell the story as a narrative with a beginning, middle, and end. Focus on flow, character, and progression rather than facts alone. Use natural, engaging language that feels like someone telling a story aloud.'
),
(
    'cinematic',
    'Cinematic',
    'Deliver the story with a cinematic feel. Use strong imagery, deliberate pacing, and powerful narration. Treat each moment like a scene in a film, emphasizing atmosphere, scale, and emotional impact.'
),
(
    'calm_reflective',
    'Calm & Reflective',
    'Narrate in a calm, thoughtful, and reflective tone. Use slower pacing and gentle language. The story should feel meditative, allowing the viewer to absorb ideas without urgency or tension.'
)
ON CONFLICT (slug) DO NOTHING;

-- ═══════════════════════════════════════════════════════════════════════════
-- VISUAL STYLE PRESETS — how the story looks
-- Uses the existing graphics_presets table. Add a slug column for clean
-- lookups and seed with the 8 visual style presets.
-- ═══════════════════════════════════════════════════════════════════════════

-- Add slug column (unique, nullable for the legacy row that predates slugs)
ALTER TABLE graphics_presets ADD COLUMN IF NOT EXISTS slug TEXT UNIQUE;

-- Add a description column for the full AI prompt injection text.
-- This is separate from prompt_addition (which is a short suffix) — description
-- is a complete creative directive.
ALTER TABLE graphics_presets ADD COLUMN IF NOT EXISTS description TEXT;

-- Set slug on the existing legacy preset
UPDATE graphics_presets
SET slug = 'luminous_regal',
    description = 'Generate visuals in a luminous, regal editorial watercolor style. Use deep purples, golds, and blacks with dramatic high-contrast lighting and soft glows. Composition should be cinematic with centered subjects. The mood is mysterious, elegant, and authoritative with high detail and smooth gradients.'
WHERE id = 'f47ac10b-58cc-4372-a567-0e02b2c3d479';

-- Seed the 8 visual style presets
INSERT INTO graphics_presets (id, slug, name, description, style_json, prompt_addition) VALUES
(
    uuid_generate_v4(),
    'cinematic_watercolor',
    'Cinematic Watercolor',
    'Generate painterly visuals that resemble high-quality watercolor illustrations with cinematic lighting. Use visible paper texture, soft gradients, and warm, glowing highlights. Avoid hard outlines. Scenes should feel artistic, atmospheric, and emotionally rich.',
    '{"style": "watercolor", "lighting": "cinematic warm", "texture": "paper"}',
    'Cinematic watercolor illustration, painterly brush strokes, warm atmospheric lighting'
),
(
    uuid_generate_v4(),
    'illustrated_editorial',
    'Illustrated Editorial',
    'Create clean, stylized editorial illustrations with strong composition and clear subject focus. Use bold color blocks, subtle texture, and controlled lighting. The visuals should feel modern, expressive, and suitable for storytelling articles or magazines.',
    '{"style": "editorial illustration", "lighting": "controlled studio", "texture": "subtle"}',
    'Clean editorial illustration, bold color blocks, strong composition, modern expressive style'
),
(
    uuid_generate_v4(),
    'anime_inspired',
    'Anime Inspired',
    'Render visuals inspired by anime-style illustration. Use expressive characters, clear facial features, dynamic framing, and dramatic lighting. Maintain a polished, hand-drawn look rather than hyper-realism.',
    '{"style": "anime", "lighting": "dramatic", "texture": "cel-shaded"}',
    'Anime-style illustration, expressive characters, dynamic framing, polished hand-drawn look'
),
(
    uuid_generate_v4(),
    'cartoon_stylized',
    'Cartoon Stylized',
    'Produce simplified, stylized cartoon visuals with exaggerated shapes and clear silhouettes. Use bright but balanced colors and minimal texture. The visuals should feel playful, approachable, and easy to read at a glance.',
    '{"style": "cartoon", "lighting": "bright flat", "texture": "minimal"}',
    'Stylized cartoon, exaggerated shapes, bright balanced colors, playful and approachable'
),
(
    uuid_generate_v4(),
    'hyper_realistic',
    'Hyper Realistic',
    'Generate highly realistic visuals with lifelike lighting, textures, and depth. Scenes should resemble cinematic photography or film stills. Pay close attention to realism, scale, and environmental detail.',
    '{"style": "photorealistic", "lighting": "natural cinematic", "texture": "high detail"}',
    'Hyper-realistic photography, cinematic film still, lifelike lighting and textures, extreme detail'
),
(
    uuid_generate_v4(),
    'digital_painting',
    'Digital Painting',
    'Create polished digital paintings with visible brush strokes and rich color blending. Use dramatic lighting and strong contrast while maintaining an artistic, hand-crafted feel rather than photorealism.',
    '{"style": "digital painting", "lighting": "dramatic contrast", "texture": "visible brush strokes"}',
    'Polished digital painting, visible brush strokes, rich color blending, dramatic lighting'
),
(
    uuid_generate_v4(),
    'minimalist_abstract',
    'Minimalist Abstract',
    'Generate abstract, minimal visuals that suggest ideas rather than literal scenes. Use simple shapes, limited color palettes, and symbolic imagery. Focus on mood and concept over detail.',
    '{"style": "minimalist abstract", "lighting": "flat ambient", "texture": "clean geometric"}',
    'Minimalist abstract art, simple shapes, limited color palette, symbolic imagery, mood over detail'
),
(
    uuid_generate_v4(),
    'low_poly_3d',
    'Low Poly 3D',
    'Render scenes using low-poly 3D aesthetics. Use geometric forms, flat shading, and simplified environments. The visuals should feel modern, clean, and stylized rather than realistic.',
    '{"style": "low poly 3D", "lighting": "flat shading", "texture": "geometric faceted"}',
    'Low-poly 3D render, geometric forms, flat shading, simplified clean environments, modern stylized'
)
ON CONFLICT (slug) DO NOTHING;

-- Create index on slug for fast lookups
CREATE INDEX IF NOT EXISTS idx_graphics_presets_slug ON graphics_presets(slug);
CREATE INDEX IF NOT EXISTS idx_tone_presets_slug ON tone_presets(slug);
