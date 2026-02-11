## Backend PRD — Episod (V1) with Series-ready foundation

### 1) Goal

Build an API-first backend that generates a **90–120s short-form video** from a **topic** + **duration**, using a default **graphics style preset**, by:

* generating a coherent multi-clip plan (OpenAI structured output)
* generating clip narration (Cartesia)
* generating still images (Gemini image model) using your **style JSON + prompt addition**
* rendering each clip as an mp4 where the still image lasts exactly the narration duration
* stitching clips into a final mp4 with FFmpeg

System must be modular (API service vs Worker), debuggable, and store artifacts in a way that can be edited later from a frontend.

---

### 2) In Scope (Build now)

* Create project via API (topic, duration)
* Generate plan (clips with script + voice style + image prompt + video prompt placeholder)
* Generate per-clip audio (Cartesia)
* Generate per-clip image (Gemini) with graphics preset JSON always applied
* Render per-clip mp4 (still image + audio length)
* Concatenate clip mp4s → final video
* Persist and expose:

  * Project progress
  * Clip details and artifacts
  * Debug artifacts (plan JSON, job logs)
* Support partial inspection when failure happens (don't lose work)

---

### 3) Out of Scope (Later)

* Series creation + "generate topics from series"
* Veo image-to-video animation
* Captions
* Remotion timeline templates
* Auto publishing

Important: we will design today's schema and orchestration so Series can be added later as a clean extension.

---

## 4) Core Concepts

### 4.1 Project (V1)

A single video-generation request, producing one final video.

### 4.2 Clip (V1)

A unit of output with:

* script line(s)
* voice delivery instruction
* image prompt
* generated audio/image
* rendered clip mp4

### 4.3 Graphics Preset (V1)

Your style JSON + prompt addition applied consistently for images (and later videos).

### 4.4 Series (Future-ready)

A reusable "show bible":

* name, description, narrative rules, sample scripts
* default graphics preset
* default voice style (optional)
* topic generation guidance (later)
  Projects can optionally belong to a series (series_id nullable now).

---

## 5) Architecture

### 5.1 Services

API Service (Go)

* Creates projects
* Reads status
* Returns artifact URLs
* Enqueues jobs
* Never does heavy generation/rendering

Worker Service (Go)

* Orchestrates the pipeline
* Calls OpenAI / Cartesia / Gemini
* Runs FFmpeg rendering
* Uploads assets to Supabase Storage
* Updates DB statuses
* Writes job logs and errors

Queue (Redis recommended)

* generate_plan
* process_clip
* render_final

Supabase

* Postgres: state
* Storage: artifacts

Why this split matters:

* You can debug each stage independently
* You can rerun single clip jobs
* You can scale workers without touching API

---

## 6) Data Model (Supabase Postgres) — Series-ready

### 6.1 Tables

series (future feature, but create table now)

* id (uuid)
* name (text)
* description (text)
* guidance (text)  // narrative constraints, tone, do/don't
* sample_script (text, optional)
* default_graphics_preset_id (uuid)
* default_voice_profile (jsonb, optional)
* created_at, updated_at

graphics_presets

* id (uuid)
* name (text)
* style_json (jsonb)  // your Luminous Regal JSON
* prompt_addition (text)
* created_at, updated_at

projects

* id (uuid)
* user_id (uuid nullable for now)
* series_id (uuid nullable)  // important for future
* topic (text)
* target_duration_seconds (int) default 105
* graphics_preset_id (uuid) // in V1: set from default preset, later: from series default
* status (enum: queued, planning, generating, rendering, completed, failed)
* plan_version (int default 1)
* created_at, updated_at
* final_video_asset_id (uuid nullable)
* error_code (text nullable)
* error_message (text nullable)

clips

* id (uuid)
* project_id (uuid)
* clip_index (int)
* script (text)
* voice_style_instruction (text)
* image_prompt (text)
* video_prompt (text) // stored for future animation use
* status (enum: pending, voiced, imaged, rendered, failed)
* audio_asset_id (uuid nullable)
* image_asset_id (uuid nullable)
* clip_video_asset_id (uuid nullable)
* audio_duration_ms (int nullable)
* created_at, updated_at
* error_message (text nullable)

assets

* id (uuid)
* project_id (uuid)
* clip_id (uuid nullable)
* type (enum: plan_json, audio, image, clip_video, final_video, logs)
* storage_bucket (text)
* storage_path (text)
* content_type (text)
* byte_size (bigint)
* created_at

jobs

* id (uuid)
* project_id (uuid)
* clip_id (uuid nullable)
* type (text) // generate_plan | process_clip | render_final
* status (enum: queued, running, succeeded, failed)
* attempts (int)
* started_at, finished_at
* error_message (text nullable)
* logs_asset_id (uuid nullable)

This design makes "editable frontend later" easy:

* clip fields are canonical and stored
* assets are attached and replaceable
* reruns can generate new assets without losing structure

---

## 7) Prompting System (V1)

### 7.1 Graphics preset injection (Gemini image)

Worker builds final Gemini prompt like this:

* base prompt = clip.image_prompt
* style JSON = graphics_presets.style_json
* prompt addition = graphics_presets.prompt_addition

Final prompt assembly (conceptually):
base_prompt

* "\n\nSTYLE_PROFILE_JSON:\n" + style_json
* "\n\nPROMPT_ADDITION:\n" + prompt_addition

In V1, graphics preset is selected from project.graphics_preset_id.
In future, project.graphics_preset_id defaults from series.default_graphics_preset_id unless overridden.

### 7.2 Plan generation constraints (OpenAI structured output)

Plan generator input includes:

* topic
* target_duration_seconds
* default clip duration range (e.g., 8–15s)
* output schema enforcement
* "short-form hook" rules (strong first clip)

Series-ready now:

* even if you don't build series yet, your plan generator function should accept an optional "series_guidance" field (empty in V1), so adding series later is just plugging in content.

---

## 8) API Design (V1)

POST /v1/projects
Request:

* topic (string)
* target_duration_seconds (int optional, default 105)
* graphics_preset_id (uuid optional, default preset)
* series_id (uuid optional, accepted but not required)
  Response:
* project_id
* status

Behavior:

* Create project row (series_id optional)
* Enqueue generate_plan job

GET /v1/projects/{id}
Returns:

* project status + progress summary
* clips array with fields + asset URLs/paths
* final video URL if completed
* error info if failed

GET /v1/projects/{id}/download
Returns signed URL or streams final mp4

GET /v1/projects/{id}/debug/jobs
Returns job timeline and errors (for fast debugging)

GET /v1/projects/{id}/clips/{clip_id}
Returns full clip details + artifacts

Note on "editable later":

* In V1, we don't build update endpoints, but the schema and clip fields already support it.
* Later you'll add:

  * PATCH clip.script / image_prompt / voice_style_instruction
  * POST regenerate clip audio/image/render

---

## 9) Pipeline (Worker)

Stage A: Generate plan

* OpenAI produces JSON with N clips
* Insert clips into DB
* Store plan.json as asset
* Project status: planning → generating
* Enqueue process_clip for each clip

Stage B: Process clip (for each clip)

1. TTS (Cartesia)

* Input: clip.script + clip.voice_style_instruction
* Output: audio file, duration_ms
* Upload audio → assets table update
* Clip status: pending → voiced

2. Image generation (Gemini)

* Input: composed prompt (clip.image_prompt + style JSON + prompt addition)
* Output: image png
* Upload image → assets table update
* Clip status: voiced → imaged

3. Render clip (FFmpeg)

* Input: image.png + audio file
* Output: clip.mp4 with duration = audio duration
* Upload clip mp4 → assets table update
* Clip status: imaged → rendered

Stage C: Final render

* When all clips rendered:

  * concatenate clip mp4s in order
  * upload final.mp4
  * set project.final_video_asset_id
  * project status → completed

Failure behavior:

* If a clip fails, mark clip failed and project failed
* Do not delete prior assets; keep for inspection
* jobs table stores error_message and optional provider response snippet (sanitized)

---

## 10) Rendering Details (V1)

Per-clip render rule:

* still image must display for exactly audio length
* audio attached as narration

Concatenation rule:

* mp4 clips concatenated in index order

(You can standardize output resolution now: 1080x1920 for shorts, and later add 1920x1080 option.)

---

## 11) Expansion Path: Series (Later, no rewrite)

When you add Series later:

* Create series in DB and attach a default graphics preset
* Update POST /projects:

  * if series_id provided, project.graphics_preset_id = series.default_graphics_preset_id (unless overridden)
  * series.guidance and sample_script are passed into OpenAI plan generation input
* Add POST /series/{id}/generate-topics:

  * OpenAI outputs a list of candidate project topics
  * store them as drafts or immediately create projects

Because the schema already has:

* series table
* projects.series_id
* per-series default preset
  You won't need a migration that reshapes the system—just add endpoints and logic.

---

## 12) Definition of Done (V1)

* One API call produces a final mp4 in Supabase Storage
* Intermediate artifacts exist per clip (audio, image, clip mp4)
* Project status moves across stages
* Failures are inspectable (jobs table + stored artifacts)
* Database structure already supports adding Series without breaking changes

---

If you want, I can generate the next two "build-ready" artifacts:

1. The exact OpenAI JSON schema for clip plan (so your output is strict and editable).
2. The minimal Go service layout (folders + packages) for API service + Worker service + queue handlers, wired to Supabase.
