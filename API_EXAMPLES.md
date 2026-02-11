# API Examples

Complete examples of API requests and responses for the Episod.

## Base URL

```
http://localhost:8080  (development)
https://your-domain.com (production)
```

---

## 1. Health Check

### Request

```bash
curl http://localhost:8080/health
```

### Response (200 OK)

```json
{
  "status": "ok"
}
```

---

## 2. Create Project

### Request - Basic

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "The Amazing History of Coffee"
  }'
```

### Request - With All Options

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "The Science Behind Lucid Dreams",
    "target_duration_seconds": 120,
    "graphics_preset_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    "series_id": null
  }'
```

### Response (201 Created)

```json
{
  "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "status": "queued"
}
```

### Error Response (400 Bad Request)

```json
{
  "error": "Topic is required"
}
```

---

## 3. Get Project Status

### Request

```bash
PROJECT_ID="a1b2c3d4-e5f6-7890-abcd-ef1234567890"
curl http://localhost:8080/v1/projects/$PROJECT_ID
```

### Response - Queued/Planning

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "user_id": null,
  "series_id": null,
  "topic": "The Amazing History of Coffee",
  "target_duration_seconds": 105,
  "graphics_preset_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "status": "planning",
  "plan_version": 1,
  "final_video_asset_id": null,
  "error_code": null,
  "error_message": null,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:30:05Z",
  "clips": [],
  "final_video_url": null,
  "graphics_preset": {
    "id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
    "name": "Luminous Regal",
    "style_json": {
      "color_palette": ["deep purples", "golds", "blacks"],
      "lighting": "dramatic high-contrast with soft glows",
      "composition": "cinematic wide shots with centered subjects",
      "mood": "mysterious, elegant, authoritative",
      "detail_level": "high detail with smooth gradients"
    },
    "prompt_addition": "Cinematic quality, 8K resolution, professional photography, award-winning composition",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-01T00:00:00Z"
  }
}
```

### Response - Generating (with clips)

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "topic": "The Amazing History of Coffee",
  "target_duration_seconds": 105,
  "graphics_preset_id": "f47ac10b-58cc-4372-a567-0e02b2c3d479",
  "status": "generating",
  "plan_version": 1,
  "created_at": "2024-01-15T10:30:00Z",
  "updated_at": "2024-01-15T10:31:00Z",
  "clips": [
    {
      "id": "clip-001",
      "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
      "clip_index": 0,
      "script": "Picture this: a humble goat herder in ancient Ethiopia notices his goats dancing with unusual energy after eating red berries from a mysterious plant.",
      "voice_style_instruction": "Energetic and mysterious, building intrigue",
      "image_prompt": "Ancient Ethiopian highlands at golden hour, a goat herder watching energetic goats near coffee plants with red berries, dramatic lighting, cinematic composition",
      "video_prompt": "Camera slowly zooms into the coffee plant while goats move energetically in the background",
      "status": "voiced",
      "audio_asset_id": "asset-audio-001",
      "image_asset_id": null,
      "clip_video_asset_id": null,
      "audio_duration_ms": 12000,
      "error_message": null,
      "created_at": "2024-01-15T10:30:30Z",
      "updated_at": "2024-01-15T10:30:45Z",
      "audio_url": "https://your-project.supabase.co/storage/v1/object/public/files/a1b2c3d4.../clip_0_audio.mp3",
      "image_url": null,
      "clip_video_url": null
    },
    {
      "id": "clip-002",
      "clip_index": 1,
      "script": "This accidental discovery would spark a global phenomenon that would change the world forever.",
      "voice_style_instruction": "Building excitement, slightly faster pace",
      "image_prompt": "Map showing coffee spreading from Ethiopia across the world, golden routes glowing, mystical atmosphere",
      "status": "pending",
      "audio_asset_id": null,
      "image_asset_id": null,
      "clip_video_asset_id": null,
      "audio_duration_ms": null,
      "created_at": "2024-01-15T10:30:30Z",
      "updated_at": "2024-01-15T10:30:30Z",
      "audio_url": null,
      "image_url": null,
      "clip_video_url": null
    }
  ],
  "final_video_url": null
}
```

### Response - Completed

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "topic": "The Amazing History of Coffee",
  "status": "completed",
  "clips": [
    {
      "id": "clip-001",
      "clip_index": 0,
      "status": "rendered",
      "audio_url": "https://.../clip_0_audio.mp3",
      "image_url": "https://.../clip_0_image.png",
      "clip_video_url": "https://.../clip_0.mp4"
    },
    {
      "id": "clip-002",
      "clip_index": 1,
      "status": "rendered",
      "audio_url": "https://.../clip_1_audio.mp3",
      "image_url": "https://.../clip_1_image.png",
      "clip_video_url": "https://.../clip_1.mp4"
    }
  ],
  "final_video_url": "https://your-project.supabase.co/storage/v1/object/public/files/a1b2c3d4.../final.mp4"
}
```

### Response - Failed

```json
{
  "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "topic": "The Amazing History of Coffee",
  "status": "failed",
  "error_code": "plan_generation_failed",
  "error_message": "openai request failed: rate limit exceeded",
  "clips": [],
  "final_video_url": null
}
```

---

## 4. Download Final Video

### Request

```bash
curl -L http://localhost:8080/v1/projects/$PROJECT_ID/download \
  -o video.mp4
```

The `-L` flag follows the redirect to the signed URL.

### Response (307 Temporary Redirect)

Headers:
```
Location: https://your-project.supabase.co/storage/v1/object/sign/files/a1b2c3d4.../final.mp4?token=...
```

### Error Response (404 Not Found)

```json
{
  "error": "Video not ready"
}
```

---

## 5. Get Debug Jobs

### Request

```bash
curl http://localhost:8080/v1/projects/$PROJECT_ID/debug/jobs
```

### Response (200 OK)

```json
[
  {
    "id": "job-001",
    "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "clip_id": null,
    "type": "generate_plan",
    "status": "succeeded",
    "attempts": 1,
    "started_at": "2024-01-15T10:30:05Z",
    "finished_at": "2024-01-15T10:30:25Z",
    "error_message": null,
    "logs_asset_id": null,
    "created_at": "2024-01-15T10:30:00Z"
  },
  {
    "id": "job-002",
    "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "clip_id": "clip-001",
    "type": "process_clip",
    "status": "succeeded",
    "attempts": 1,
    "started_at": "2024-01-15T10:30:30Z",
    "finished_at": "2024-01-15T10:31:15Z",
    "error_message": null,
    "created_at": "2024-01-15T10:30:25Z"
  },
  {
    "id": "job-003",
    "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "clip_id": "clip-002",
    "type": "process_clip",
    "status": "failed",
    "attempts": 1,
    "started_at": "2024-01-15T10:30:30Z",
    "finished_at": "2024-01-15T10:30:50Z",
    "error_message": "failed to generate image: gemini returned status 429: rate limit exceeded",
    "created_at": "2024-01-15T10:30:25Z"
  },
  {
    "id": "job-004",
    "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "clip_id": null,
    "type": "render_final",
    "status": "running",
    "attempts": 0,
    "started_at": "2024-01-15T10:32:00Z",
    "finished_at": null,
    "error_message": null,
    "created_at": "2024-01-15T10:31:55Z"
  }
]
```

---

## 6. Get Clip Details

### Request

```bash
CLIP_ID="clip-001"
curl http://localhost:8080/v1/projects/$PROJECT_ID/clips/$CLIP_ID
```

### Response (200 OK)

```json
{
  "id": "clip-001",
  "project_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
  "clip_index": 0,
  "script": "Picture this: a humble goat herder in ancient Ethiopia notices his goats dancing with unusual energy after eating red berries from a mysterious plant.",
  "voice_style_instruction": "Energetic and mysterious, building intrigue",
  "image_prompt": "Ancient Ethiopian highlands at golden hour, a goat herder watching energetic goats near coffee plants with red berries, dramatic lighting, cinematic composition",
  "video_prompt": "Camera slowly zooms into the coffee plant while goats move energetically in the background",
  "status": "rendered",
  "audio_asset_id": "asset-audio-001",
  "image_asset_id": "asset-image-001",
  "clip_video_asset_id": "asset-video-001",
  "audio_duration_ms": 12000,
  "error_message": null,
  "created_at": "2024-01-15T10:30:30Z",
  "updated_at": "2024-01-15T10:31:15Z",
  "audio_url": "https://your-project.supabase.co/storage/v1/object/public/files/a1b2c3d4.../clip_0_audio.mp3",
  "image_url": "https://your-project.supabase.co/storage/v1/object/public/files/a1b2c3d4.../clip_0_image.png",
  "clip_video_url": "https://your-project.supabase.co/storage/v1/object/public/files/a1b2c3d4.../clip_0.mp4"
}
```

---

## Complete Workflow Example

### 1. Create project and capture ID

```bash
RESPONSE=$(curl -s -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"topic": "The Rise of Artificial Intelligence", "target_duration_seconds": 90}')

PROJECT_ID=$(echo $RESPONSE | jq -r '.project_id')
echo "Created project: $PROJECT_ID"
```

### 2. Poll for status

```bash
while true; do
  STATUS=$(curl -s http://localhost:8080/v1/projects/$PROJECT_ID | jq -r '.status')
  echo "Status: $STATUS ($(date +%H:%M:%S))"

  if [ "$STATUS" = "completed" ]; then
    echo "Video ready!"
    break
  elif [ "$STATUS" = "failed" ]; then
    echo "Generation failed"
    curl -s http://localhost:8080/v1/projects/$PROJECT_ID/debug/jobs | jq .
    exit 1
  fi

  sleep 10
done
```

### 3. Get final video URL

```bash
VIDEO_URL=$(curl -s http://localhost:8080/v1/projects/$PROJECT_ID | jq -r '.final_video_url')
echo "Video available at: $VIDEO_URL"
```

### 4. Download video

```bash
curl -L http://localhost:8080/v1/projects/$PROJECT_ID/download -o my_video.mp4
echo "Downloaded to my_video.mp4"
```

---

## Error Responses

### 400 Bad Request

```json
{
  "error": "Invalid request body"
}
```

```json
{
  "error": "Topic is required"
}
```

### 404 Not Found

```json
{
  "error": "Project not found"
}
```

```json
{
  "error": "Clip not found"
}
```

```json
{
  "error": "Video not ready"
}
```

### 500 Internal Server Error

```json
{
  "error": "Failed to create project"
}
```

---

## Using with Different Tools

### cURL

```bash
curl -X POST http://localhost:8080/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"topic": "Space Exploration"}'
```

### HTTPie

```bash
http POST http://localhost:8080/v1/projects \
  topic="Space Exploration" \
  target_duration_seconds:=90
```

### JavaScript (fetch)

```javascript
const response = await fetch('http://localhost:8080/v1/projects', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    topic: 'The Future of Renewable Energy',
    target_duration_seconds: 120
  })
});

const data = await response.json();
console.log('Project ID:', data.project_id);
```

### Python (requests)

```python
import requests
import time

# Create project
response = requests.post(
    'http://localhost:8080/v1/projects',
    json={
        'topic': 'The Mysteries of Deep Ocean',
        'target_duration_seconds': 90
    }
)

project_id = response.json()['project_id']
print(f'Created project: {project_id}')

# Poll for completion
while True:
    status_response = requests.get(
        f'http://localhost:8080/v1/projects/{project_id}'
    )
    status = status_response.json()['status']

    print(f'Status: {status}')

    if status == 'completed':
        video_url = status_response.json()['final_video_url']
        print(f'Video ready: {video_url}')
        break
    elif status == 'failed':
        print('Generation failed')
        break

    time.sleep(10)
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type CreateProjectRequest struct {
    Topic                 string `json:"topic"`
    TargetDurationSeconds int    `json:"target_duration_seconds"`
}

type CreateProjectResponse struct {
    ProjectID string `json:"project_id"`
    Status    string `json:"status"`
}

type ProjectResponse struct {
    ID              string `json:"id"`
    Status          string `json:"status"`
    FinalVideoURL   string `json:"final_video_url,omitempty"`
}

func main() {
    // Create project
    reqBody, _ := json.Marshal(CreateProjectRequest{
        Topic:                 "The Evolution of Technology",
        TargetDurationSeconds: 90,
    })

    resp, err := http.Post(
        "http://localhost:8080/v1/projects",
        "application/json",
        bytes.NewBuffer(reqBody),
    )
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    var createResp CreateProjectResponse
    json.NewDecoder(resp.Body).Decode(&createResp)

    fmt.Printf("Created project: %s\n", createResp.ProjectID)

    // Poll for completion
    for {
        resp, err := http.Get(
            fmt.Sprintf("http://localhost:8080/v1/projects/%s", createResp.ProjectID),
        )
        if err != nil {
            panic(err)
        }

        var projectResp ProjectResponse
        json.NewDecoder(resp.Body).Decode(&projectResp)
        resp.Body.Close()

        fmt.Printf("Status: %s\n", projectResp.Status)

        if projectResp.Status == "completed" {
            fmt.Printf("Video ready: %s\n", projectResp.FinalVideoURL)
            break
        } else if projectResp.Status == "failed" {
            fmt.Println("Generation failed")
            break
        }

        time.Sleep(10 * time.Second)
    }
}
```

---

## Rate Limiting Considerations

While V1 doesn't include built-in rate limiting, be aware of external API limits:

- **OpenAI**: ~50 requests/minute (depends on tier)
- **Cartesia**: ~100 requests/minute (depends on plan)
- **Gemini**: ~60 requests/minute (depends on plan)

Respect these limits when creating multiple projects concurrently.

## Best Practices

1. **Poll Responsibly**: Use 10-30 second intervals when polling status
2. **Handle Errors**: Always check response status codes
3. **Store Project IDs**: Save project IDs to check status later
4. **Use Debug Endpoint**: Check `/debug/jobs` when troubleshooting failures
5. **Download Once**: Cache downloaded videos, don't repeatedly download

## Next Steps

- See [README.md](README.md) for setup instructions
- See [QUICKSTART.md](QUICKSTART.md) for quick deployment
- See [DEVELOPMENT.md](DEVELOPMENT.md) for API extension guide
