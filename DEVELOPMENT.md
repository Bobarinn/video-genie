# Development Guide

This guide provides detailed information for developers working on the Faceless Video Generator backend.

## Project Overview

The Faceless Video Generator is a modular Go backend that orchestrates AI services to create short-form videos from text topics. The system is designed for reliability, debuggability, and future extensibility.

## Architecture Principles

### Separation of Concerns

- **API Service**: Handles HTTP requests, validation, and job enqueueing
- **Worker Service**: Processes jobs asynchronously
- **Database**: Single source of truth for state
- **Queue**: Decouples API from heavy processing
- **Storage**: External asset storage (Supabase)

### Data Flow

```
Client → API → Database + Queue → Worker → External Services → Storage → Database
                                      ↓
                               (OpenAI, Cartesia, Gemini, FFmpeg)
```

### State Management

Every significant operation updates the database:
- Projects track overall status
- Clips track per-clip progress
- Jobs track execution history
- Assets track generated files

This ensures:
- Progress is never lost
- Failures are debuggable
- Retries are possible
- Frontend can poll for status

## Key Design Decisions

### Why Go?

- Excellent concurrency support (goroutines)
- Strong typing prevents errors
- Fast compilation and execution
- Great standard library
- Simple deployment (single binary)

### Why PostgreSQL?

- JSONB support for flexible schemas
- Strong consistency guarantees
- Excellent tooling
- Can handle both relational and document data

### Why Redis for Queue?

- Fast, in-memory operations
- Simple list-based queue implementation
- Blocking pop (BLPOP) for efficient workers
- Widely supported and reliable

### Why Separate API and Worker?

- API remains responsive even during heavy processing
- Workers can scale independently
- Failed jobs don't crash the API
- Can deploy API and Worker separately

### Why Supabase Storage?

- Built on PostgreSQL (consistency)
- Automatic signed URLs
- Row-level security support
- Easy CDN integration
- Good developer experience

## Code Organization

```
internal/
├── api/           # HTTP layer
│   ├── handlers.go    # Request handlers
│   └── router.go      # Route definitions
├── config/        # Configuration loading
├── db/            # Database operations
│   ├── db.go          # Connection management
│   ├── projects.go    # Project queries
│   ├── clips.go       # Clip queries
│   ├── assets.go      # Asset queries
│   └── jobs.go        # Job queries
├── models/        # Data structures
│   └── models.go      # All models in one file
├── queue/         # Queue abstraction
│   └── queue.go       # Redis queue implementation
├── services/      # External service clients
│   ├── openai.go      # OpenAI integration
│   ├── cartesia.go    # Cartesia TTS
│   ├── gemini.go      # Gemini image gen
│   └── ffmpeg.go      # FFmpeg wrapper
├── storage/       # Storage abstraction
│   └── storage.go     # Supabase client
└── worker/        # Job processing
    └── worker.go      # Pipeline orchestration
```

## Database Schema Design

### Projects Table

Central record for each video generation request.

Key fields:
- `status`: Tracks progress through pipeline
- `graphics_preset_id`: Links to style definition
- `series_id`: Future: links to recurring series
- `final_video_asset_id`: Points to completed video

### Clips Table

Individual segments of the video.

Key fields:
- `clip_index`: Determines order in final video
- `script`: Narration text
- `image_prompt`: What to generate visually
- `status`: Tracks clip processing (pending → voiced → imaged → rendered)
- Asset IDs: Links to generated audio/image/video

### Assets Table

All generated files (audio, images, videos, plans).

Key fields:
- `type`: Enum for asset type
- `storage_path`: Location in Supabase Storage
- `project_id` and `clip_id`: Links back to source

### Jobs Table

Execution history for debugging.

Key fields:
- `type`: Which job handler ran
- `status`: Current state
- `attempts`: Number of retries
- `error_message`: What went wrong

## Adding New Features

### Adding a New API Endpoint

1. Add handler method to `internal/api/handlers.go`
2. Register route in `internal/api/router.go`
3. Add any new models to `internal/models/models.go`
4. Add database methods to appropriate `internal/db/*.go` file

Example:
```go
// handlers.go
func (h *Handler) GetProjectAnalytics(w http.ResponseWriter, r *http.Request) {
    projectID := chi.URLParam(r, "id")
    // Implementation
}

// router.go
r.Get("/projects/{id}/analytics", h.GetProjectAnalytics)
```

### Adding a New Job Type

1. Define job type constant in `internal/queue/queue.go`
2. Add enqueue helper method
3. Add handler method in `internal/worker/worker.go`
4. Register handler in `Start()` method

Example:
```go
// queue.go
const QueueGenerateCaption = "queue:generate_caption"

func (q *Queue) EnqueueGenerateCaption(ctx context.Context, projectID, clipID uuid.UUID) error {
    // Implementation
}

// worker.go
func (w *Worker) handleGenerateCaption(ctx context.Context, job *queue.Job) error {
    // Implementation
}

// In Start()
go w.processQueue(ctx, queue.QueueGenerateCaption, w.handleGenerateCaption)
```

### Adding a New External Service

1. Create new file in `internal/services/`
2. Define service struct with API credentials
3. Implement methods for API calls
4. Add configuration to `internal/config/config.go`
5. Initialize in `cmd/api/main.go`
6. Use in worker handlers

Example:
```go
// services/elevenlabs.go
type ElevenLabsService struct {
    apiKey string
    client *http.Client
}

func NewElevenLabsService(apiKey string) *ElevenLabsService {
    return &ElevenLabsService{
        apiKey: apiKey,
        client: &http.Client{Timeout: 60 * time.Second},
    }
}

func (s *ElevenLabsService) GenerateVoice(ctx context.Context, text string) ([]byte, error) {
    // Implementation
}
```

## Testing Strategy

### Unit Tests

Test individual functions and methods:
```go
func TestGenerateStoragePath(t *testing.T) {
    stor := storage.New("url", "key", "bucket")
    projectID := uuid.New()
    path := stor.GenerateStoragePath(projectID, "test.mp4")

    if !strings.Contains(path, projectID.String()) {
        t.Error("path should contain project ID")
    }
}
```

### Integration Tests

Test database operations with test database:
```go
func TestCreateProject(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()

    project := &models.Project{
        ID:    uuid.New(),
        Topic: "Test Topic",
    }

    err := db.CreateProject(context.Background(), project)
    if err != nil {
        t.Fatalf("failed to create project: %v", err)
    }
}
```

### End-to-End Tests

Use `scripts/test-api.sh` to test the full pipeline.

## Common Tasks

### Adding a Database Column

1. Add field to model in `internal/models/models.go`
2. Update SQL queries in `internal/db/*.go`
3. Create migration file: `migrations/00X_description.sql`
4. Test migration on dev database

### Changing Queue Behavior

Queue implementation is in `internal/queue/queue.go`.

To change timeout, concurrency, or retry logic:
1. Update `Worker.Start()` parameters
2. Modify `processQueue()` logic
3. Update `MAX_CONCURRENT_JOBS` config

### Adding a Graphics Preset

Insert into database:
```sql
INSERT INTO graphics_presets (name, style_json, prompt_addition) VALUES (
    'Neon Cyberpunk',
    '{"color_palette": ["neon pink", "electric blue"], "mood": "futuristic"}',
    'Cyberpunk aesthetic, neon lighting, 4K'
);
```

Or via API (future endpoint).

## Debugging

### Check Job Status

```bash
curl http://localhost:8080/v1/projects/{id}/debug/jobs | jq
```

Shows:
- Job execution timeline
- Which jobs succeeded/failed
- Error messages
- Attempt counts

### Check Database State

```bash
psql $DATABASE_URL -c "SELECT id, status, error_message FROM projects;"
psql $DATABASE_URL -c "SELECT id, clip_index, status FROM clips WHERE project_id = 'xxx';"
```

### Check Queue Length

```bash
redis-cli LLEN queue:generate_plan
redis-cli LLEN queue:process_clip
redis-cli LLEN queue:render_final
```

### View Worker Logs

```bash
# Docker
make docker-logs

# Local
# Workers log to stdout
```

### Common Issues

**Worker not processing jobs**
- Check `WORKER_ENABLED=true`
- Verify Redis connection
- Check worker logs for errors

**FFmpeg fails**
- Ensure FFmpeg installed: `which ffmpeg`
- Check temp directory permissions: `ls -la /tmp/faceless`
- Verify input files exist

**Supabase upload fails**
- Check `SUPABASE_SERVICE_KEY` is service role key (not anon key)
- Verify bucket exists and is accessible
- Check storage path doesn't have invalid characters

**OpenAI/Cartesia/Gemini failures**
- Verify API keys are correct
- Check rate limits
- Review error messages in jobs table

## Performance Considerations

### Concurrent Processing

Workers process jobs concurrently. Increase `MAX_CONCURRENT_JOBS` for more throughput, but be aware of:
- API rate limits (OpenAI, Gemini, Cartesia)
- Memory usage (FFmpeg operations)
- Storage bandwidth

### Database Connections

Connection pool settings in `internal/db/db.go`:
```go
db.SetMaxOpenConns(25)  // Max concurrent connections
db.SetMaxIdleConns(5)   // Idle connection pool
```

Adjust based on worker concurrency.

### Asset Storage

Consider:
- Image file size (PNG vs JPEG)
- Video quality settings
- Storage costs

Optimize FFmpeg settings in `internal/services/ffmpeg.go`.

### Queue Optimization

For high throughput:
- Use multiple worker instances
- Implement job prioritization
- Add Redis clustering

## Security Considerations

### API Keys

- Never commit API keys
- Use environment variables
- Rotate keys regularly
- Use separate keys for dev/prod

### Database Access

- Use connection strings with strong passwords
- Enable SSL in production
- Limit database user permissions

### Storage Access

- Use Supabase service role key securely
- Implement signed URLs for downloads
- Set appropriate bucket policies

### Input Validation

All user inputs are validated:
- Topic length limits
- Duration bounds
- UUID format checks

## Deployment

### Building for Production

```bash
CGO_ENABLED=0 GOOS=linux go build -o api cmd/api/main.go
```

### Docker Deployment

```bash
docker build -t faceless-api .
docker push your-registry/faceless-api
```

### Environment Variables

Ensure all required env vars are set in production:
- Database URL (with SSL)
- Redis URL
- All API keys
- Supabase credentials

### Health Checks

Use `/health` endpoint for:
- Load balancer health checks
- Container orchestration (Kubernetes)
- Monitoring systems

### Monitoring

Recommended:
- Log aggregation (CloudWatch, Datadog)
- Error tracking (Sentry)
- Metrics (Prometheus)
- Uptime monitoring

## Future Enhancements

### Series Support

Schema is ready. To implement:
1. Add series CRUD endpoints
2. Link projects to series on creation
3. Pass series guidance to OpenAI
4. Add topic generation endpoint

### Edit Support

Schema supports it. To implement:
1. Add PATCH endpoints for clip fields
2. Add regenerate endpoints (audio/image/video)
3. Implement selective re-rendering
4. Handle plan versioning

### Veo Integration

For image-to-video animation:
1. Add Veo service in `internal/services/`
2. Modify clip rendering to use Veo when available
3. Update database schema for video prompts
4. Add configuration for Veo vs still images

## Contributing

1. Create feature branch from `main`
2. Make changes
3. Run tests: `make test`
4. Format code: `make fmt`
5. Create pull request

## Resources

- [Go Documentation](https://golang.org/doc/)
- [Chi Router](https://github.com/go-chi/chi)
- [PostgreSQL Docs](https://www.postgresql.org/docs/)
- [Redis Commands](https://redis.io/commands)
- [FFmpeg Docs](https://ffmpeg.org/documentation.html)
- [OpenAI API](https://platform.openai.com/docs)
- [Supabase Docs](https://supabase.com/docs)
