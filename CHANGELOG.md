# Changelog

All notable changes to the Episod project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-15

### Added - Initial Release

#### Core Features
- Complete video generation pipeline from topic to final MP4
- AI-powered video planning with OpenAI GPT-4
- Text-to-speech narration using Cartesia
- Styled image generation using Google Gemini
- Automated video rendering with FFmpeg
- Multi-clip video structure with concurrent processing

#### Architecture
- REST API service with Chi router
- Background worker service for async processing
- Redis-based job queue system
- PostgreSQL database with complete schema
- Supabase Storage integration for assets
- Separation of API and Worker services

#### API Endpoints
- `POST /v1/projects` - Create new video project
- `GET /v1/projects/{id}` - Get project status and details
- `GET /v1/projects/{id}/download` - Download final video
- `GET /v1/projects/{id}/debug/jobs` - Debug job execution
- `GET /v1/projects/{projectId}/clips/{clipId}` - Get clip details
- `GET /health` - Health check endpoint

#### Database Schema
- `projects` table - Video generation projects
- `clips` table - Individual video clips
- `assets` table - Generated files storage
- `jobs` table - Job execution history
- `graphics_presets` table - Visual style definitions
- `series` table - Future series support (schema only)

#### External Integrations
- OpenAI API for video plan generation
- Cartesia API for text-to-speech
- Google Gemini API for image generation
- Supabase Storage for asset management
- FFmpeg for video processing

#### Development Tools
- Docker Compose for local development
- Database migrations
- Setup script for quick start
- API testing script
- Makefile for common tasks
- Environment-based configuration

#### Documentation
- Comprehensive README with setup instructions
- Quick Start guide for 5-minute deployment
- Development guide with architecture details
- API examples with code samples in multiple languages
- Architecture diagrams and flow charts
- Project summary and roadmap
- Backend PRD specification

#### Graphics Presets
- Default "Luminous Regal" preset included
- Support for custom style JSON and prompt additions
- Consistent styling across all generated images

### Technical Details

#### Performance
- Concurrent clip processing (configurable)
- Typical video generation: 2-5 minutes
- Support for 90-120 second videos
- 6-10 clips per video on average

#### Reliability
- State persisted at each pipeline stage
- Failed jobs don't lose prior work
- Full job execution history for debugging
- Automatic retry support (foundation)

#### Scalability
- Independent API and Worker scaling
- Queue-based architecture
- Database connection pooling
- Multiple worker instances supported

### Known Limitations

- No authentication/authorization (add in production)
- No rate limiting (add in production)
- No webhook notifications
- No editing/regeneration endpoints (schema supports it)
- No automated captions
- No video animation (still images only)
- No series management (schema ready, not implemented)

### Dependencies

- Go 1.22+
- PostgreSQL 15+
- Redis 7+
- FFmpeg
- OpenAI API
- Cartesia API
- Google Gemini API
- Supabase

## [1.0.1] - 2024-01-15

### Fixed
- Updated Cartesia TTS integration to match official API specification
- Corrected API endpoint from `/tts` to `/tts/bytes`
- Added required `Cartesia-Version` header
- Fixed request body structure to match Cartesia API spec

### Added
- Voice ID configuration support (`CARTESIA_VOICE_ID`)
- Emotion control with 50+ emotion options
- Speed control (0.6x to 1.5x)
- Volume control (0.5x to 2.0x)
- Automatic emotion detection from voice style descriptions
- Comprehensive Cartesia setup guide (`docs/CARTESIA_SETUP.md`)
- New service constructors: `NewCartesiaServiceWithVoice()`
- Advanced speech generation: `GenerateSpeechWithOptions()`

### Changed
- Updated default Cartesia API URL to `https://api.cartesia.ai`
- Improved audio duration estimation with speed adjustment
- Enhanced error messages for Cartesia API failures

### Documentation
- Added `docs/CARTESIA_SETUP.md` - Complete Cartesia setup guide
- Added `CARTESIA_UPDATE.md` - Migration and update guide
- Updated `README.md` with Cartesia configuration
- Updated `QUICKSTART.md` with voice setup instructions

## [Unreleased]

### Planned for v1.2

- [ ] Clip regeneration endpoints
- [ ] Webhook notifications for completed videos
- [ ] User authentication system
- [ ] Rate limiting middleware
- [ ] Enhanced error messages
- [ ] Retry logic for failed jobs

### Planned for v2.0

- [ ] Series management (create, update, delete)
- [ ] Auto topic generation from series
- [ ] Video animation with Veo
- [ ] Automatic caption generation
- [ ] Edit existing projects
- [ ] Admin dashboard

### Planned for v3.0

- [ ] Frontend web interface
- [ ] Direct social media publishing
- [ ] Analytics and insights
- [ ] A/B testing for styles
- [ ] Multi-language support
- [ ] Advanced scheduling

---

## Version History

| Version | Date | Description |
|---------|------|-------------|
| 1.0.0 | 2024-01-15 | Initial release with complete V1 features |

## Contributing

See [DEVELOPMENT.md](DEVELOPMENT.md) for contribution guidelines.

## Support

For issues and feature requests, please use GitHub Issues.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
