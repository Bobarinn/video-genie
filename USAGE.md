 Health Check

  # Check if API is running
  curl http://localhost:8080/health

  ---
  Projects

  1. Create Project

  # Basic project creation
  curl -X POST http://localhost:8080/v1/projects \
    -H "Content-Type: application/json" \
    -d '{
      "topic": "The Future of Artificial Intelligence",
      "target_duration_seconds": 90
    }'

  # With custom duration
  curl -X POST http://localhost:8080/v1/projects \
    -H "Content-Type: application/json" \
    -d '{
      "topic": "Top 10 Space Discoveries",
      "target_duration_seconds": 120
    }'

  # With specific graphics preset (optional)
  curl -X POST http://localhost:8080/v1/projects \
    -H "Content-Type: application/json" \
    -d '{
      "topic": "History of Ancient Rome",
      "target_duration_seconds": 90,
      "graphics_preset_id": "YOUR_PRESET_UUID"
    }'

  # With series ID (for future series feature)
  curl -X POST http://localhost:8080/v1/projects \
    -H "Content-Type: application/json" \
    -d '{
      "topic": "Episode 5: Climate Change",
      "target_duration_seconds": 90,
      "series_id": "YOUR_SERIES_UUID"
    }'

  Response:
  {
    "project_id": "8157f767-4200-4f0e-8e53-79deabec6ec5",
    "status": "queued"
  }

  ---
  2. Get Project Details

  # Get project with all clips and status
  curl http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5

  # Pretty print with jq
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5 | jq

  # Get only the status
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5 | jq -r '.project.status'

  # Get clip count
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5 | jq '.clips | length'

  Response:
  {
    "project": {
      "id": "8157f767-4200-4f0e-8e53-79deabec6ec5",
      "topic": "The Future of Artificial Intelligence",
      "status": "completed",
      "target_duration_seconds": 90,
      "plan_version": 1,
      "created_at": "2026-02-08T00:41:42.123Z",
      "updated_at": "2026-02-08T00:43:15.456Z"
    },
    "clips": [
      {
        "clip": {
          "id": "713e1e69-0e36-4eea-a684-76b04f2a5dd4",
          "project_id": "8157f767-4200-4f0e-8e53-79deabec6ec5",
          "clip_index": 0,
          "script": "Artificial intelligence is transforming our world...",
          "status": "rendered",
          "duration_ms": 15000,
          "image_prompt": "A futuristic AI brain...",
          "created_at": "2026-02-08T00:41:42.123Z"
        },
        "audio_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/...",
        "image_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/...",
        "clip_video_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/..."
      }
    ],
    "graphics_preset": {
      "id": "...",
      "name": "Luminous Regal",
      "style_json": {...}
    },
    "final_video_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/.../final.mp4"
  }

  ---
  3. Download Final Video

  # Download final video (redirects to signed URL)
  curl -L http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/download \
    -o video.mp4

  # With custom filename
  curl -L http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/download \
    -o "AI_Video_$(date +%Y%m%d).mp4"

  # Just get the redirect URL (without downloading)
  curl -sI http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/download | grep -i location

  ---
  4. Debug Jobs (Troubleshooting)

  # Get all jobs for a project
  curl http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/debug/jobs

  # Pretty print with jq
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/debug/jobs | jq

  # Filter only failed jobs
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/debug/jobs | jq '.[] | select(.status == "failed")'

  # Count jobs by status
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/debug/jobs | jq 'group_by(.status) | map({status: .[0].status, count: length})'

  Response:
  [
    {
      "id": "41bff29f-8cec-435c-9114-2721fe6a759a",
      "project_id": "8157f767-4200-4f0e-8e53-79deabec6ec5",
      "type": "generate_plan",
      "status": "succeeded",
      "created_at": "2026-02-08T00:41:42.123Z",
      "updated_at": "2026-02-08T00:41:58.456Z"
    },
    {
      "id": "5061470c-1a47-4daa-ac74-91732335069e",
      "project_id": "8157f767-4200-4f0e-8e53-79deabec6ec5",
      "clip_id": "713e1e69-0e36-4eea-a684-76b04f2a5dd4",
      "type": "process_clip",
      "status": "failed",
      "error_message": "failed to render clip: ffmpeg render clip failed",
      "created_at": "2026-02-08T00:41:58.123Z",
      "updated_at": "2026-02-08T00:42:18.456Z"
    }
  ]

  ---
  Clips

  5. Get Individual Clip Details

  # Get specific clip details
  curl http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/clips/713e1e69-0e36-4eea-a684-76b04f2a5dd4

  # Pretty print
  curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5/clips/713e1e69-0e36-4eea-a684-76b04f2a5dd4 | jq

  Response:
  {
    "clip": {
      "id": "713e1e69-0e36-4eea-a684-76b04f2a5dd4",
      "project_id": "8157f767-4200-4f0e-8e53-79deabec6ec5",
      "clip_index": 0,
      "script": "Artificial intelligence is transforming our world...",
      "voice_style_instruction": "energetic and excited",
      "image_prompt": "A futuristic AI brain with glowing neural networks",
      "video_prompt": "Slow zoom into the brain with particles flowing",
      "status": "rendered",
      "duration_ms": 15000,
      "created_at": "2026-02-08T00:41:42.123Z",
      "updated_at": "2026-02-08T00:42:10.456Z"
    },
    "audio_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/.../clip_0_audio.mp3",
    "image_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/.../clip_0_image.png",
    "clip_video_url": "https://your-project.supabase.co/storage/v1/object/public/faceless-videos/.../clip_713e1e69-0e36-4eea-a684-76b04f2a5dd4.mp4"
  }

  ---
  Monitoring Scripts

  Watch project status continuously

  # Check status every 2 seconds
  watch -n 2 "curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5 | jq -r '.project.status'"

  # Monitor with more details
  watch -n 2 "curl -s http://localhost:8080/v1/projects/8157f767-4200-4f0e-8e53-79deabec6ec5 | jq '{status: .project.status, clips: (.clips | length), completed: ([.clips[].clip.status] | map(select(. == \"rendered\")) | length)}'"

  Full workflow example

  # 1. Create project and save ID
  PROJECT_ID=$(curl -s -X POST http://localhost:8080/v1/projects \
    -H "Content-Type: application/json" \
    -d '{"topic": "The Future of AI", "target_duration_seconds": 90}' \
    | jq -r '.project_id')

  echo "Project ID: $PROJECT_ID"

  # 2. Check status
  curl -s http://localhost:8080/v1/projects/$PROJECT_ID | jq '.project.status'

  # 3. Wait for completion (poll every 5 seconds)
  while true; do
    STATUS=$(curl -s http://localhost:8080/v1/projects/$PROJECT_ID | jq -r '.project.status')
    echo "Status: $STATUS"
    if [ "$STATUS" = "completed" ]; then
      break
    fi
    sleep 5
  done

  # 4. Download video
  curl -L http://localhost:8080/v1/projects/$PROJECT_ID/download -o final_video.mp4

  echo "Video downloaded: final_video.mp4"

  ---
  Status Values

  - queued - Project created, waiting to start
  - planning - Generating video plan with OpenAI
  - generating - Creating clips (TTS, images, rendering)
  - rendering - Concatenating clips into final video
  - completed - Final video ready
  - failed - Error occurred (check jobs endpoint)

  ---
  Error Responses

  All errors return JSON with an error field:

  {
    "error": "Project not found"
  }

  HTTP Status Codes:
  - 200 - Success
  - 201 - Created
  - 307 - Temporary Redirect (download endpoint)
  - 400 - Bad Request
  - 404 - Not Found
  - 500 - Internal Server Error