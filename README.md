# Episod

A Go-based backend service for generating 90-120 second faceless short-form videos using AI. The system generates video plans, creates narration with text-to-speech, generates styled images, and renders complete videos ready for social media.

## Features

- **AI-Powered Video Planning**: Uses OpenAI to generate coherent multi-clip video structures
- **Text-to-Speech**: Generates natural narration using Cartesia TTS
- **Styled Image Generation**: Creates consistent visual assets using Google Gemini with custom style presets
- **Automated Video Rendering**: Uses FFmpeg to combine audio and images into polished clips
- **Modular Architecture**: Separate API and Worker services for scalability
- **Queue-based Processing**: Redis-backed job queue for reliable async processing
- **Supabase Storage**: Stores all generated assets (audio, images, videos) in Supabase Storage
- **Series Support**: Database schema ready for recurring video series (future feature)

## Architecture

The system is split into two main components:

### API Service
- REST API for creating and managing projects
- Handles project creation and status queries
- Enqueues jobs for processing
- Never performs heavy computation

### Worker Service
- Processes jobs from Redis queues
- Orchestrates the video generation pipeline:
  1. **Plan Generation**: OpenAI creates multi-clip structure
  2. **Clip Processing**: For each clip:
     - Generate audio (Cartesia TTS)
     - Generate image (Gemini with style preset)
     - Render clip video (FFmpeg)
  3. **Final Rendering**: Concatenate all clips into final video

## Tech Stack

- **Language**: Go 1.22
- **Database**: PostgreSQL (via Supabase)
- **Queue**: Redis
- **Storage**: Supabase Storage
- **AI Services**:
  - OpenAI (GPT-4) for video planning
  - Cartesia for text-to-speech
  - Google Gemini for image generation
- **Video Processing**: FFmpeg

## Prerequisites

- Go 1.22+
- PostgreSQL 15+
- Redis 7+
- FFmpeg installed
- API keys for:
  - OpenAI
  - Cartesia
  - Google Gemini
  - Supabase (URL + Service Key)

## Quick Start

### 1. Clone and Setup

```bash
git clone <repository-url>
cd episod
cp .env.example .env
# Edit .env with your API keys
```

### 2. Install Dependencies

```bash
go mod download
```

### 3. Setup Database

```bash
# Create database
createdb faceless

# Run migrations
make migrate
# Or manually:
psql -d faceless -f migrations/001_initial_schema.sql
```

### 4. Run with Docker Compose (Recommended)

```bash
# Make sure to set environment variables in .env file
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

### 5. Run Locally

```bash
# Terminal 1: Start PostgreSQL and Redis (or use Docker)
docker-compose up postgres redis

# Terminal 2: Run the API + Worker
make run
```

The API will be available at `http://localhost:8080`

## API Endpoints

### Create Project
```bash
POST /v1/projects
Content-Type: application/json

{
  "topic": "The History of Pizza",
  "target_duration_seconds": 105,
  "graphics_preset_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479" // optional
}

Response:
{
  "project_id": "uuid",
  "status": "queued"
}
```

### Get Project Status
```bash
GET /v1/projects/{id}

Response:
{
  "id": "uuid",
  "topic": "The History of Pizza",
  "status": "completed",
  "clips": [...],
  "final_video_url": "https://..."
}
```

### Download Final Video
```bash
GET /v1/projects/{id}/download
# Returns redirect to signed download URL
```

### Get Debug Info
```bash
GET /v1/projects/{id}/debug/jobs
# Returns job execution timeline and errors
```

### Get Clip Details
```bash
GET /v1/projects/{projectId}/clips/{clipId}
# Returns full clip details with asset URLs
```

### Health Check
```bash
GET /health
```

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable | Description | Default |
|----------|-------------|---------|
| `API_PORT` | HTTP server port | `8080` |
| `WORKER_ENABLED` | Enable worker processing | `true` |
| `DATABASE_URL` | PostgreSQL connection string | - |
| `REDIS_URL` | Redis connection string | `redis://localhost:6379` |
| `SUPABASE_URL` | Supabase project URL | - |
| `SUPABASE_SERVICE_KEY` | Supabase service role key | - |
| `SUPABASE_STORAGE_BUCKET` | Storage bucket name | `files` |
| `OPENAI_API_KEY` | OpenAI API key | - |
| `CARTESIA_API_KEY` | Cartesia API key | - |
| `CARTESIA_API_URL` | Cartesia API endpoint | `https://api.cartesia.ai` |
| `CARTESIA_VOICE_ID` | Default voice ID (optional) | - |
| `GEMINI_API_KEY` | Google Gemini API key | - |
| `MAX_CONCURRENT_JOBS` | Worker concurrency | `5` |

**Note:** For detailed Cartesia setup including voice selection and emotion control, see [docs/CARTESIA_SETUP.md](docs/CARTESIA_SETUP.md)

## Database Schema

The system uses a PostgreSQL database with the following main tables:

- **projects**: Video generation projects
- **clips**: Individual video clips within a project
- **assets**: Generated files (audio, images, videos)
- **jobs**: Job queue records and status
- **graphics_presets**: Reusable visual style definitions
- **series**: (Future) Recurring video series templates

See `migrations/001_initial_schema.sql` for full schema.

## Graphics Presets

Graphics presets define consistent visual styles for generated images. A default "Luminous Regal" preset is included.

Example preset structure:
```json
{
  "style_json": {
    "color_palette": ["deep purples", "golds", "blacks"],
    "lighting": "dramatic high-contrast with soft glows",
    "composition": "cinematic wide shots with centered subjects",
    "mood": "mysterious, elegant, authoritative"
  },
  "prompt_addition": "Cinematic quality, 8K resolution, professional photography"
}
```

## Development

### Project Structure

```
episod/
├── cmd/
│   └── api/              # Main application entry point
├── internal/
│   ├── api/              # HTTP handlers and routes
│   ├── config/           # Configuration management
│   ├── db/               # Database layer
│   ├── models/           # Data models
│   ├── queue/            # Redis queue implementation
│   ├── services/         # External service integrations
│   │   ├── openai.go     # OpenAI integration
│   │   ├── cartesia.go   # Cartesia TTS integration
│   │   ├── gemini.go     # Gemini image generation
│   │   └── ffmpeg.go     # FFmpeg rendering
│   ├── storage/          # Supabase storage client
│   └── worker/           # Background job processor
├── migrations/           # Database migrations
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

### Running Tests

```bash
make test
```

### Code Formatting

```bash
make fmt
```

### Building

```bash
make build
# Binary will be in bin/api
```

## Pipeline Flow

1. **User creates project** via `POST /v1/projects`
2. **API** creates project record and enqueues `generate_plan` job
3. **Worker** picks up job and calls OpenAI to generate video plan
4. **Worker** creates clip records and enqueues `process_clip` jobs for each
5. **For each clip**, Worker:
   - Generates audio with Cartesia
   - Generates image with Gemini (using style preset)
   - Renders clip video with FFmpeg
6. **When all clips are done**, Worker enqueues `render_final` job
7. **Worker** concatenates all clip videos into final video
8. **Final video** uploaded to Supabase Storage
9. **Project status** updated to `completed`

## Error Handling

- All errors are captured and stored in database
- Jobs track attempts and error messages
- Projects can fail at any stage without losing prior work
- Debug endpoint shows full job timeline for troubleshooting

## Future Enhancements (Out of Scope for V1)

- **Series Management**: Create recurring video series with consistent themes
- **Auto Topic Generation**: Generate video topics from series guidance
- **Video Animation**: Veo integration for image-to-video animation
- **Captions**: Automatic subtitle generation
- **Remotion Templates**: Custom timeline templates
- **Auto Publishing**: Direct upload to social platforms

## Troubleshooting

### FFmpeg not found
```bash
# macOS
brew install ffmpeg

# Ubuntu/Debian
apt-get install ffmpeg

# Alpine (Docker)
apk add ffmpeg
```

### Database connection issues
- Verify `DATABASE_URL` is correct
- Check PostgreSQL is running
- Ensure database exists

### Queue not processing
- Verify Redis is running
- Check `WORKER_ENABLED=true`
- Review worker logs for errors

### Video generation fails
- Check API keys are valid
- Verify Supabase storage bucket exists and is accessible
- Check FFmpeg is installed and in PATH
- Review job error messages in `/v1/projects/{id}/debug/jobs`

## License

[Your License Here]

## Contributing

[Contribution Guidelines]

## Support

For issues and questions, please open a GitHub issue.

## Example Usage

### Create a new video project
```bash
curl -X POST http://localhost:8080/v1/projects \
-H "Content-Type: application/json" \
-H "X-API-Key: YOUR_API_KEY_HERE" \
-d '{"topic": "Introduce Nelson Mandela in 30 sec","target_duration_seconds": 30}'
```

### Check project status
```bash
curl -s http://localhost:8080/v1/projects/PROJECT_ID_HERE \
-H "X-API-Key: YOUR_API_KEY_HERE" | jq
```

### Create another project
```bash
curl -X POST http://localhost:8080/v1/projects \
-H "Content-Type: application/json" \
-H "X-API-Key: YOUR_API_KEY_HERE" \
-d '{"topic": "The Untold History of Abraham Lincoln","target_duration_seconds": 30}'
```

## Performance Notes

Pipeline A: Image (50s) → Upload (5s) → xAI Video (60-120s)    ──┐
                                                                 ├→ Render
Pipeline B: Audio (2s) → Upload (1s) → Whisper (3s)            ──┘
Total wall time: ~120-175s per clip (audio pipeline runs "for free" during image gen)