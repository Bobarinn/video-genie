# Quick Start Guide

Get the Episod running in under 5 minutes.

## Prerequisites

- Docker and Docker Compose installed
- API keys for:
  - OpenAI (https://platform.openai.com)
  - Cartesia (https://cartesia.ai)
  - Google Gemini (https://ai.google.dev)
  - Supabase project (https://supabase.com)

## Setup

### 1. Clone and Configure

```bash
cd episod
cp .env.example .env
```

Edit `.env` and add your API keys:
```bash
# Required - Add your API keys
OPENAI_API_KEY=sk-...
CARTESIA_API_KEY=...
GEMINI_API_KEY=...
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_KEY=eyJ...
SUPABASE_STORAGE_BUCKET=files

# Optional - Set a specific Cartesia voice
# CARTESIA_VOICE_ID=a0e99841-438c-4a64-b679-ae501e7d6091
```

**Note:** The system will use a default voice if `CARTESIA_VOICE_ID` is not set. For voice selection and customization, see [docs/CARTESIA_SETUP.md](docs/CARTESIA_SETUP.md)

### 2. Create Supabase Bucket

1. Go to your Supabase project dashboard
2. Navigate to Storage
3. Create a new bucket named `files`
4. Make it public (or use signed URLs)

### 3. Start Services

```bash
make docker-up
```

This will:
- Start PostgreSQL database
- Start Redis queue
- Run database migrations
- Start API + Worker service

### 4. Check Status

```bash
# View logs
make docker-logs

# Check health
curl http://localhost:8080/health
```

## Create Your First Video

### Using curl

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "The Amazing History of Coffee",
    "target_duration_seconds": 90
  }'
```

Response:
```json
{
  "project_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "queued"
}
```

### Check Progress

```bash
# Replace {project_id} with your project ID
curl http://localhost:8080/v1/projects/{project_id}
```

Status will progress through:
1. `queued` - Job is waiting
2. `planning` - Generating video plan
3. `generating` - Creating clips (audio + images)
4. `rendering` - Assembling final video
5. `completed` - Video is ready!

### Using the Test Script

```bash
./scripts/test-api.sh
```

This will:
- Create a test project
- Monitor progress
- Show the final video URL when complete

## Understanding the Pipeline

When you create a project, this happens:

1. **Plan Generation** (~10-30 seconds)
   - OpenAI generates 6-10 clip structure
   - Each clip has script, image prompt, and voice style

2. **Clip Processing** (~30-90 seconds per clip, parallel)
   - Generate audio with Cartesia TTS
   - Generate image with Gemini
   - Render clip video with FFmpeg

3. **Final Rendering** (~10-20 seconds)
   - Concatenate all clips
   - Upload to Supabase Storage

**Total time**: Approximately 2-5 minutes for a 90-second video

## API Endpoints

### Create Project
```bash
POST /v1/projects
Body: { "topic": "...", "target_duration_seconds": 90 }
```

### Get Project Status
```bash
GET /v1/projects/{id}
```

### Download Video
```bash
GET /v1/projects/{id}/download
```

### Debug Jobs
```bash
GET /v1/projects/{id}/debug/jobs
```

### Get Clip Details
```bash
GET /v1/projects/{projectId}/clips/{clipId}
```

## Common Issues

### "Failed to connect to database"

Check PostgreSQL is running:
```bash
docker-compose ps postgres
```

### "Failed to connect to queue"

Check Redis is running:
```bash
docker-compose ps redis
```

### "OpenAI request failed"

- Verify `OPENAI_API_KEY` is correct
- Check you have API credits
- Review error in `/v1/projects/{id}/debug/jobs`

### "Failed to upload to Supabase"

- Check `SUPABASE_SERVICE_KEY` (not anon key!)
- Verify bucket exists and name matches `.env`
- Ensure storage bucket has correct permissions

### "FFmpeg not found"

If running locally (not Docker):
```bash
# macOS
brew install ffmpeg

# Ubuntu
sudo apt-get install ffmpeg
```

### Video generation takes too long

Increase worker concurrency:
```bash
# In .env
MAX_CONCURRENT_JOBS=10
```

Restart services:
```bash
make docker-down
make docker-up
```

## Next Steps

### Customize Graphics Style

Edit the default graphics preset in the database:

```sql
UPDATE graphics_presets
SET
  style_json = '{"color_palette": ["blue", "white"], "mood": "calm"}',
  prompt_addition = 'Minimalist style, clean composition'
WHERE name = 'Luminous Regal';
```

### Add a New Graphics Preset

```sql
INSERT INTO graphics_presets (name, style_json, prompt_addition) VALUES (
  'Dark Cinematic',
  '{"color_palette": ["black", "red", "gold"], "lighting": "low-key dramatic", "mood": "intense"}',
  'Cinematic lighting, moody atmosphere, 8K'
);
```

Use in API request:
```json
{
  "topic": "...",
  "graphics_preset_id": "your-preset-uuid"
}
```

### Scale Workers

Run multiple worker instances:

```bash
# Terminal 1
WORKER_ENABLED=true go run cmd/api/main.go

# Terminal 2
WORKER_ENABLED=true go run cmd/api/main.go

# Terminal 3
WORKER_ENABLED=false go run cmd/api/main.go  # API only
```

### Monitor with Logs

```bash
# Follow all logs
docker-compose logs -f

# Just API
docker-compose logs -f api

# Just database
docker-compose logs -f postgres
```

## Development

### Run Locally (without Docker)

```bash
# Start dependencies
docker-compose up postgres redis

# Run migrations
make migrate

# Start API + Worker
make run

# Or with live reload
make dev
```

### Run Tests

```bash
make test
```

### Format Code

```bash
make fmt
```

## Production Deployment

See [DEVELOPMENT.md](DEVELOPMENT.md) for detailed deployment instructions.

Quick checklist:
- [ ] Set strong database passwords
- [ ] Enable database SSL
- [ ] Use production Redis instance
- [ ] Set up monitoring (health checks, logs)
- [ ] Configure rate limiting
- [ ] Set up backups (database, storage)
- [ ] Use separate API keys for prod
- [ ] Configure CORS properly
- [ ] Set up CDN for video delivery

## Get Help

- Read [README.md](README.md) for full documentation
- Check [DEVELOPMENT.md](DEVELOPMENT.md) for architecture details
- Review [backend.md](backend.md) for the PRD

## Resources

- OpenAI Docs: https://platform.openai.com/docs
- Cartesia Docs: https://docs.cartesia.ai
- Gemini Docs: https://ai.google.dev/docs
- Supabase Docs: https://supabase.com/docs
- FFmpeg Docs: https://ffmpeg.org/documentation.html

---

**That's it!** You now have a fully functional AI video generator. 🎬

Create as many videos as you want:
```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"topic": "YOUR_TOPIC_HERE", "target_duration_seconds": 90}'
```
