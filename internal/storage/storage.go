package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	// Upload timeout per attempt — generous for large 12MB+ images
	uploadTimeout = 180 * time.Second

	// Download timeout
	downloadTimeout = 120 * time.Second

	// Retry configuration
	maxRetries     = 4
	baseRetryDelay = 1 * time.Second
	maxRetryDelay  = 30 * time.Second
)

type Storage struct {
	url        string
	serviceKey string
	Bucket     string
	client     *http.Client
}

func New(url, serviceKey, bucket string) *Storage {
	return &Storage{
		url:        url,
		serviceKey: serviceKey,
		Bucket:     bucket,
		client: &http.Client{
			Timeout: uploadTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 20,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Upload uploads a file to Supabase Storage with retries and exponential backoff.
// Uses PUT with Content-Length and x-upsert for reliable large file uploads.
func (s *Storage) Upload(ctx context.Context, path string, data []byte, contentType string) error {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.url, s.Bucket, path)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelay(attempt)
			log.Printf("[Storage] Upload retry %d/%d for %s (waiting %v)...", attempt, maxRetries, path, delay)

			select {
			case <-ctx.Done():
				return fmt.Errorf("upload cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		// Each attempt gets its own generous timeout, independent of caller's ctx
		uploadCtx, cancel := context.WithTimeout(ctx, uploadTimeout)

		req, err := http.NewRequestWithContext(uploadCtx, "PUT", url, bytes.NewReader(data))
		if err != nil {
			cancel()
			return fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+s.serviceKey)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Content-Length", fmt.Sprintf("%d", len(data)))
		req.Header.Set("x-upsert", "true")

		resp, err := s.client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("failed to upload: %w", err)
			if isRetryableError(err) {
				log.Printf("[Storage] Upload attempt %d failed (retryable): %v", attempt+1, err)
				continue
			}
			return lastErr
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
			if attempt > 0 {
				log.Printf("[Storage] Upload succeeded on attempt %d for %s", attempt+1, path)
			}
			return nil
		}

		lastErr = fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))

		if isRetryableStatus(resp.StatusCode) {
			log.Printf("[Storage] Upload attempt %d returned status %d (retryable): %s", attempt+1, resp.StatusCode, truncate(string(body), 200))
			continue
		}

		// Non-retryable status (400, 401, 403, 404, 413, etc.)
		return lastErr
	}

	return fmt.Errorf("upload failed after %d attempts: %w", maxRetries+1, lastErr)
}

// UploadFile uploads a file from a local path
func (s *Storage) UploadFile(ctx context.Context, storagePath, localPath string, contentType string) error {
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", localPath, err)
	}

	return s.Upload(ctx, storagePath, data, contentType)
}

// Download downloads a file from Supabase Storage with retries
func (s *Storage) Download(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.url, s.Bucket, path)

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := retryDelay(attempt)
			log.Printf("[Storage] Download retry %d/%d for %s (waiting %v)...", attempt, maxRetries, path, delay)

			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("download cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		dlCtx, cancel := context.WithTimeout(ctx, downloadTimeout)

		req, err := http.NewRequestWithContext(dlCtx, "GET", url, nil)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Authorization", "Bearer "+s.serviceKey)

		resp, err := s.client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("failed to download: %w", err)
			if isRetryableError(err) {
				log.Printf("[Storage] Download attempt %d failed (retryable): %v", attempt+1, err)
				continue
			}
			return nil, lastErr
		}

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			cancel()
			if err != nil {
				lastErr = fmt.Errorf("failed to read download body: %w", err)
				log.Printf("[Storage] Download attempt %d read failed: %v", attempt+1, err)
				continue
			}
			return data, nil
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		lastErr = fmt.Errorf("download failed with status %d: %s", resp.StatusCode, string(body))

		if isRetryableStatus(resp.StatusCode) {
			log.Printf("[Storage] Download attempt %d returned status %d (retryable)", attempt+1, resp.StatusCode)
			continue
		}

		return nil, lastErr
	}

	return nil, fmt.Errorf("download failed after %d attempts: %w", maxRetries+1, lastErr)
}

// GetPublicURL returns the public URL for a file
func (s *Storage) GetPublicURL(path string) string {
	return fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.url, s.Bucket, path)
}

// GetSignedURL creates a signed URL for temporary access
func (s *Storage) GetSignedURL(ctx context.Context, path string, expiresIn int) (string, error) {
	url := fmt.Sprintf("%s/storage/v1/object/sign/%s/%s", s.url, s.Bucket, path)

	body := fmt.Sprintf(`{"expiresIn": %d}`, expiresIn)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBufferString(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get signed URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed with status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		SignedURL string `json:"signedURL"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to parse signed URL response: %w", err)
	}

	return s.url + result.SignedURL, nil
}

// GenerateStoragePath creates a storage path for an asset
func (s *Storage) GenerateStoragePath(projectID uuid.UUID, filename string) string {
	return filepath.Join(projectID.String(), filename)
}

// retryDelay calculates exponential backoff with jitter: base * 2^attempt + random jitter
func retryDelay(attempt int) time.Duration {
	delay := float64(baseRetryDelay) * math.Pow(2, float64(attempt-1))
	if delay > float64(maxRetryDelay) {
		delay = float64(maxRetryDelay)
	}
	// Add 0–25% jitter to avoid thundering herd
	jitter := delay * 0.25 * rand.Float64()
	return time.Duration(delay + jitter)
}

// isRetryableError checks if a network-level error is worth retrying
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "deadline exceeded") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "broken pipe")
}

// isRetryableStatus checks if an HTTP status code is worth retrying
func isRetryableStatus(status int) bool {
	return status == http.StatusTooManyRequests || // 429
		status == http.StatusRequestTimeout || // 408
		status == http.StatusBadGateway || // 502
		status == http.StatusServiceUnavailable || // 503
		status == http.StatusGatewayTimeout // 504
}

// truncate limits a string to maxLen characters for log output
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
