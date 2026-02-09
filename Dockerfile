# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code (includes sample.jpeg for style reference)
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api ./cmd/api

# Runtime stage
FROM alpine:latest

# Install runtime dependencies (FFmpeg + fonts for subtitle rendering)
# font-noto provides "Noto Sans" — a clean, modern sans-serif font used for
# TikTok-style ASS subtitles burned into the video by FFmpeg's ass filter.
# fontconfig is needed by FFmpeg/libass to discover installed fonts.
RUN apk add --no-cache ffmpeg ca-certificates font-noto fontconfig \
    && fc-cache -f

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/api /app/api

# Copy style reference assets
COPY --from=builder /app/assets/style-reference /app/assets/style-reference

# Copy background music assets
COPY --from=builder /app/assets/music /app/assets/music

# Create temp directory for FFmpeg operations
RUN mkdir -p /tmp/faceless && chmod 777 /tmp/faceless

# Expose API port
EXPOSE 8080

# Run the application
CMD ["/app/api"]
