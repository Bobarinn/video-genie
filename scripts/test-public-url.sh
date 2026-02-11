#!/bin/bash
# Test if Supabase public URLs are accessible.
# Usage: ./scripts/test-public-url.sh
#
# This lists recent image assets and tests if their public URLs return 200 OK.

set -euo pipefail

# Load .env
if [ -f .env ]; then
  export $(grep -v '^#' .env | grep -v '^\s*$' | xargs)
fi

BUCKET="${SUPABASE_STORAGE_BUCKET:-files}"

echo "=== Supabase Public URL Test ==="
echo "Supabase URL: $SUPABASE_URL"
echo "Bucket: $BUCKET"
echo ""

# List objects in the bucket via Supabase Storage API
echo "--- Fetching recent image assets from bucket ---"
RESPONSE=$(curl -s -w "\n%{http_code}" \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" \
  "${SUPABASE_URL}/storage/v1/object/list/${BUCKET}" \
  -d '{"prefix":"projects/","limit":5,"sortBy":{"column":"created_at","order":"desc"}}' \
  -H "Content-Type: application/json")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

echo "List API status: $HTTP_CODE"

if [ "$HTTP_CODE" != "200" ]; then
  echo "ERROR: Could not list bucket contents: $BODY"
  echo ""
  echo "Trying a known project path instead..."
fi

# Find a real image file by querying the database
echo ""
echo "--- Checking database for recent image assets ---"
IMAGE_PATH=$(psql "$DATABASE_URL" -t -A -c "
  SELECT storage_path FROM assets 
  WHERE type = 'image' 
  ORDER BY created_at DESC 
  LIMIT 1
" 2>/dev/null || echo "")

if [ -z "$IMAGE_PATH" ]; then
  echo "No image assets found in database."
  echo ""
  echo "--- Testing with manual URL ---"
  echo "Paste an image storage path (e.g. projects/xxx/clip_0_image.png):"
  read -r IMAGE_PATH
fi

if [ -z "$IMAGE_PATH" ]; then
  echo "No path provided. Exiting."
  exit 1
fi

echo "Image storage path: $IMAGE_PATH"
echo ""

# Test 1: Public URL
PUBLIC_URL="${SUPABASE_URL}/storage/v1/object/public/${BUCKET}/${IMAGE_PATH}"
echo "--- Test 1: Public URL ---"
echo "URL: $PUBLIC_URL"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$PUBLIC_URL")
echo "Status: $HTTP_STATUS"
if [ "$HTTP_STATUS" = "200" ]; then
  echo "✅ Public URL works! xAI should be able to fetch this."
else
  echo "❌ Public URL returned $HTTP_STATUS"
  echo "   -> Make sure the '$BUCKET' bucket is set to PUBLIC in Supabase Dashboard > Storage"
fi
echo ""

# Test 2: Authenticated download (should always work)
echo "--- Test 2: Authenticated download ---"
AUTH_URL="${SUPABASE_URL}/storage/v1/object/${BUCKET}/${IMAGE_PATH}"
echo "URL: $AUTH_URL"
HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" \
  "$AUTH_URL")
echo "Status: $HTTP_STATUS"
if [ "$HTTP_STATUS" = "200" ]; then
  echo "✅ Authenticated download works (file exists and is accessible)."
else
  echo "❌ Authenticated download returned $HTTP_STATUS (file may not exist)"
fi
echo ""

# Test 3: Signed URL
echo "--- Test 3: Signed URL ---"
SIGN_RESPONSE=$(curl -s \
  -H "apikey: $SUPABASE_SERVICE_KEY" \
  -H "Authorization: Bearer $SUPABASE_SERVICE_KEY" \
  -H "Content-Type: application/json" \
  -d '{"expiresIn": 3600}' \
  "${SUPABASE_URL}/storage/v1/object/sign/${BUCKET}/${IMAGE_PATH}")
echo "Sign response: $SIGN_RESPONSE"

SIGNED_PATH=$(echo "$SIGN_RESPONSE" | python3 -c "import sys,json; print(json.load(sys.stdin).get('signedURL',''))" 2>/dev/null || echo "")

if [ -n "$SIGNED_PATH" ]; then
  SIGNED_URL="${SUPABASE_URL}${SIGNED_PATH}"
  echo "Signed URL: $SIGNED_URL"
  HTTP_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$SIGNED_URL")
  echo "Status: $HTTP_STATUS"
  if [ "$HTTP_STATUS" = "200" ]; then
    echo "✅ Signed URL works!"
  else
    echo "❌ Signed URL returned $HTTP_STATUS"
  fi
else
  echo "❌ Could not generate signed URL"
fi

echo ""
echo "=== Summary ==="
echo "If Test 1 (Public URL) fails but Test 2 (Authenticated) passes:"
echo "  -> The bucket is PRIVATE. Go to Supabase Dashboard > Storage > '$BUCKET' > Settings > toggle Public ON."
echo ""
echo "If all tests fail:"
echo "  -> Check your SUPABASE_URL and SUPABASE_SERVICE_KEY in .env"
