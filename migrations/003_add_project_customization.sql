-- Migration 003: Add per-project customization fields
-- These columns allow each project to override global defaults for tone, style,
-- voice, language, etc. — enabling richer API requests without changing env config.
--
-- All fields are optional (nullable) with sensible defaults applied at the
-- application layer, so existing projects are unaffected.

-- Tone controls the narrative voice and script writing style.
-- Examples: "documentary", "dramatic", "educational", "comedic", "inspirational"
ALTER TABLE projects ADD COLUMN IF NOT EXISTS tone TEXT;

-- Visual style guides both Gemini image generation and xAI video generation.
-- Examples: "cinematic watercolor", "photorealistic", "anime", "oil painting", "3D render"
ALTER TABLE projects ADD COLUMN IF NOT EXISTS visual_style TEXT;

-- Aspect ratio for generated images and video.
-- Default is 9:16 (portrait, TikTok/Reels/Shorts). Other options: "16:9", "1:1", "4:5"
ALTER TABLE projects ADD COLUMN IF NOT EXISTS aspect_ratio TEXT DEFAULT '9:16';

-- Per-project ElevenLabs voice override. When set, this voice is used instead of
-- the global default from ELEVENLABS_VOICE_ID env var.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS voice_id TEXT;

-- Call-to-action text for the last clip's script ending.
-- Examples: "Follow for part 2!", "Subscribe for more stories", "Like if you learned something new"
ALTER TABLE projects ADD COLUMN IF NOT EXISTS cta TEXT;

-- Music mood hint for background music selection.
-- Examples: "calm", "epic", "upbeat", "dark", "nostalgic"
-- Future: used to pick from a music library. Currently informational.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS music_mood TEXT;

-- URL to a custom style reference image (stored in Supabase Storage or any public URL).
-- When set, Gemini uses this instead of the default sample.jpeg for style guidance.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS sample_image_url TEXT;

-- Language code for script generation, TTS, and Whisper transcription.
-- ISO 639-1 codes: "en", "es", "fr", "pt", "de", "ja", "ko", "ar", etc.
ALTER TABLE projects ADD COLUMN IF NOT EXISTS language TEXT DEFAULT 'en';
