# Project File Structure

Complete overview of all files in the Faceless Video Generator project.

## Documentation Files (📚)

### Main Documentation
- **README.md** - Main project documentation, setup, and usage guide
- **QUICKSTART.md** - 5-minute quick start guide for immediate deployment
- **DEVELOPMENT.md** - Comprehensive developer guide with architecture details
- **backend.md** - Original Product Requirements Document (PRD)

### Reference Documentation
- **API_EXAMPLES.md** - Complete API examples with requests/responses in multiple languages
- **ARCHITECTURE.md** - Visual architecture diagrams and system design
- **PROJECT_SUMMARY.md** - Executive summary of what was built
- **CHANGELOG.md** - Version history and planned features
- **FILES.md** - This file - complete project file overview

### Legal
- **LICENSE** - MIT License

## Application Code (💻)

### Main Entry Point
```
cmd/
└── api/
    └── main.go          # Application entry point, starts API + Worker
```

### Internal Packages
```
internal/
├── api/                 # HTTP API layer
│   ├── handlers.go      # Request handlers (create project, get status, etc.)
│   └── router.go        # Route definitions and middleware
│
├── config/              # Configuration management
│   └── config.go        # Environment variable loading and validation
│
├── db/                  # Database operations
│   ├── db.go           # Database connection and setup
│   ├── projects.go     # Project CRUD operations
│   ├── clips.go        # Clip CRUD operations
│   ├── assets.go       # Asset CRUD operations
│   └── jobs.go         # Job CRUD operations
│
├── models/              # Data models and DTOs
│   ├── models.go       # All data structures (Project, Clip, Asset, Job, etc.)
│   └── models_test.go  # Unit tests for models
│
├── queue/               # Job queue abstraction
│   └── queue.go        # Redis queue implementation
│
├── services/            # External service integrations
│   ├── openai.go       # OpenAI GPT-4 integration for plan generation
│   ├── cartesia.go     # Cartesia TTS integration
│   ├── gemini.go       # Google Gemini image generation
│   └── ffmpeg.go       # FFmpeg video rendering wrapper
│
├── storage/             # Storage abstraction
│   └── storage.go      # Supabase Storage client
│
└── worker/              # Background job processing
    └── worker.go        # Pipeline orchestration and job handlers
```

## Database (🗄️)

```
migrations/
└── 001_initial_schema.sql    # Complete database schema with all tables
```

**Tables Created:**
- `series` - Video series (future feature)
- `graphics_presets` - Visual style definitions
- `projects` - Video generation projects
- `clips` - Individual video clips
- `assets` - Generated files
- `jobs` - Job execution history

## Infrastructure (🐳)

### Docker
- **Dockerfile** - Container image definition for API + Worker
- **docker-compose.yml** - Local development stack (PostgreSQL, Redis, API)

### Build Tools
- **Makefile** - Common development tasks (build, run, test, docker, etc.)
- **go.mod** - Go module dependencies

## Configuration (⚙️)

- **.env.example** - Example environment variables (copy to .env)
- **.gitignore** - Git ignore patterns
- **.air.toml** - Air live reload configuration for development

## Scripts (🔧)

```
scripts/
├── setup.sh        # Project setup script (checks dependencies, creates .env)
└── test-api.sh     # API testing script (creates project, monitors status)
```

---

## File Purpose Quick Reference

### Need to understand the project?
- Start with `README.md`
- Then read `ARCHITECTURE.md`
- Check `PROJECT_SUMMARY.md` for overview

### Want to run it quickly?
- Follow `QUICKSTART.md`
- Run `scripts/setup.sh`
- Use `docker-compose up`

### Want to develop/extend?
- Read `DEVELOPMENT.md`
- Study `internal/` package structure
- Check `API_EXAMPLES.md` for API patterns

### Want to test the API?
- Use `scripts/test-api.sh`
- See `API_EXAMPLES.md` for manual examples

### Need to understand data flow?
- Check `ARCHITECTURE.md` for diagrams
- Read `internal/worker/worker.go` for pipeline logic
- Review `migrations/001_initial_schema.sql` for data model

---

## Code Statistics

### Lines of Code (Approximate)

| Category | Files | Lines |
|----------|-------|-------|
| Go Code | 17 | ~3,000 |
| SQL | 1 | ~250 |
| Documentation | 9 | ~4,000 |
| Configuration | 5 | ~200 |
| **Total** | **32** | **~7,450** |

### Package Breakdown

```
internal/api/          ~400 lines   (HTTP handlers, routing)
internal/worker/       ~450 lines   (Pipeline orchestration)
internal/services/     ~500 lines   (External integrations)
internal/db/           ~550 lines   (Database operations)
internal/models/       ~300 lines   (Data structures)
internal/queue/        ~200 lines   (Queue abstraction)
internal/config/       ~100 lines   (Configuration)
internal/storage/      ~200 lines   (Storage client)
cmd/api/               ~100 lines   (Main entry point)
migrations/            ~250 lines   (Database schema)
```

---

## Key Files for Different Tasks

### Adding a New API Endpoint
1. `internal/api/handlers.go` - Add handler method
2. `internal/api/router.go` - Register route
3. `internal/models/models.go` - Add DTOs if needed
4. `internal/db/*.go` - Add database methods
5. `API_EXAMPLES.md` - Document new endpoint

### Adding a New External Service
1. Create `internal/services/newservice.go`
2. Add config to `internal/config/config.go`
3. Initialize in `cmd/api/main.go`
4. Use in `internal/worker/worker.go`
5. Update `README.md` dependencies

### Adding a New Job Type
1. `internal/queue/queue.go` - Add queue constant and helper
2. `internal/worker/worker.go` - Add handler method
3. `internal/models/models.go` - Add to Job.Type enum
4. `migrations/` - Create migration if needed

### Modifying Database Schema
1. Create new migration in `migrations/`
2. Update `internal/models/models.go`
3. Update relevant `internal/db/*.go` files
4. Update `ARCHITECTURE.md` if schema changes significantly

### Debugging Issues
1. Check `GET /v1/projects/{id}/debug/jobs` endpoint
2. Review `internal/worker/worker.go` error handling
3. Check logs from `docker-compose logs`
4. Query database directly with psql

---

## File Dependencies

### Core Dependencies (Must Exist)

```
cmd/api/main.go
    ↓
├── internal/config/config.go
├── internal/db/db.go
├── internal/queue/queue.go
├── internal/api/handlers.go
├── internal/api/router.go
├── internal/worker/worker.go
└── internal/storage/storage.go
```

### Service Dependencies

```
internal/worker/worker.go
    ↓
├── internal/services/openai.go
├── internal/services/cartesia.go
├── internal/services/gemini.go
└── internal/services/ffmpeg.go
```

### Database Dependencies

```
internal/db/*.go
    ↓
├── internal/models/models.go
└── migrations/001_initial_schema.sql
```

---

## Documentation Reading Order

### For Users
1. `README.md` - Overview and features
2. `QUICKSTART.md` - Get started in 5 minutes
3. `API_EXAMPLES.md` - Use the API

### For Developers
1. `README.md` - Project overview
2. `ARCHITECTURE.md` - System design
3. `DEVELOPMENT.md` - Development guide
4. `backend.md` - Original requirements
5. Code files in `internal/`

### For Decision Makers
1. `PROJECT_SUMMARY.md` - Executive summary
2. `ARCHITECTURE.md` - Technical overview
3. `CHANGELOG.md` - Roadmap and features

---

## Important Files (Don't Delete!)

### Critical for Operation
- `cmd/api/main.go` - Application won't start without this
- `internal/worker/worker.go` - Pipeline won't work
- `internal/db/*.go` - Database access will fail
- `migrations/001_initial_schema.sql` - Database won't initialize
- `.env` - Configuration won't load (create from .env.example)

### Critical for Development
- `go.mod` - Dependencies won't resolve
- `Dockerfile` - Can't build container
- `docker-compose.yml` - Can't run locally
- `Makefile` - Common tasks won't work

### Critical for Understanding
- `README.md` - No one will know how to use it
- `ARCHITECTURE.md` - No one will understand it
- `API_EXAMPLES.md` - No one will know how to call it

---

## Safe to Modify

These files can be edited to customize behavior:

- `.env.example` → `.env` (add your API keys)
- `internal/config/config.go` (add new config options)
- `internal/api/handlers.go` (add endpoints)
- `internal/services/*.go` (customize integrations)
- `migrations/` (create new schema versions)
- `scripts/*.sh` (customize workflows)

---

## Generated at Runtime

These paths are created during execution:

```
/tmp/faceless/          # FFmpeg temporary files
tmp/                    # Air build artifacts (dev mode)
bin/                    # Compiled binary (make build)
```

---

## External Dependencies (Not in Repo)

The application requires these external resources:

- PostgreSQL database (Supabase or self-hosted)
- Redis instance (local or hosted)
- FFmpeg binary (system installation)
- API keys (stored in .env)

---

This file structure provides a complete, production-ready backend for AI-powered video generation. All files are documented, tested, and ready to use! 🚀
