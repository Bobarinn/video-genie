#!/bin/bash

# Test script for Faceless Video Generator API

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "Testing Faceless Video Generator API at $BASE_URL"
echo "================================================"

# Test 1: Health check
echo -e "\n1. Testing health endpoint..."
curl -s "$BASE_URL/health" | jq .
echo "✓ Health check passed"

# Test 2: Create project
echo -e "\n2. Creating new project..."
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/projects" \
  -H "Content-Type: application/json" \
  -d '{
    "topic": "The Amazing History of Coffee",
    "target_duration_seconds": 90
  }')

echo "$RESPONSE" | jq .

PROJECT_ID=$(echo "$RESPONSE" | jq -r '.project_id')

if [ "$PROJECT_ID" = "null" ] || [ -z "$PROJECT_ID" ]; then
  echo "✗ Failed to create project"
  exit 1
fi

echo "✓ Project created: $PROJECT_ID"

# Test 3: Get project status
echo -e "\n3. Getting project status..."
curl -s "$BASE_URL/v1/projects/$PROJECT_ID" | jq .
echo "✓ Retrieved project status"

# Test 4: Monitor progress
echo -e "\n4. Monitoring project progress..."
echo "Checking status every 10 seconds (press Ctrl+C to stop)..."

while true; do
  STATUS=$(curl -s "$BASE_URL/v1/projects/$PROJECT_ID" | jq -r '.status')
  echo "Status: $STATUS ($(date +%H:%M:%S))"

  if [ "$STATUS" = "completed" ]; then
    echo "✓ Project completed!"

    # Get final video URL
    VIDEO_URL=$(curl -s "$BASE_URL/v1/projects/$PROJECT_ID" | jq -r '.final_video_url')
    echo "Final video: $VIDEO_URL"
    break
  elif [ "$STATUS" = "failed" ]; then
    echo "✗ Project failed"
    curl -s "$BASE_URL/v1/projects/$PROJECT_ID/debug/jobs" | jq .
    exit 1
  fi

  sleep 10
done

# Test 5: Get debug info
echo -e "\n5. Getting job debug info..."
curl -s "$BASE_URL/v1/projects/$PROJECT_ID/debug/jobs" | jq .
echo "✓ Retrieved debug info"

echo -e "\n================================================"
echo "All tests passed! ✓"
echo "Project ID: $PROJECT_ID"
