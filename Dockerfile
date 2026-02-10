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
# font-noto-cjk covers Chinese/Japanese/Korean characters for multi-language support.
# font-noto-emoji covers emoji rendering in subtitles.
# fontconfig is needed by FFmpeg/libass to discover installed fonts.
# If Noto Sans is missing, libass falls back to the default fontconfig match
# (typically DejaVu Sans on Alpine), so subtitles never break — just look different.
RUN apk add --no-cache ffmpeg ca-certificates \
    font-noto font-noto-cjk font-noto-emoji fontconfig \
    && fc-cache -f

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/api /app/api

# Copy style reference assets
COPY --from=builder /app/assets/style-reference /app/assets/style-reference

# Copy background music assets
COPY --from=builder /app/assets/music /app/assets/music

# Create temp directory for FFmpeg operations
RUN mkdir -p /tmp/episod && chmod 777 /tmp/episod

# Expose API port
EXPOSE 8080

# Run the application
CMD ["/app/api"]
