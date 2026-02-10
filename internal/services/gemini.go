package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bobarin/episod/internal/models"
)

// NOTE: defaultStyleJSON and defaultPromptAddition are intentionally commented out.
// The style reference image (sample.jpeg) alone guides the visual style, which reduces
// token consumption and lets Gemini interpret the scene more naturally.
//
// To re-enable explicit style instructions, uncomment these constants and update
// composeImagePrompt() to include them in the prompt text.

// const defaultStyleJSON = `{ ... }`   // Omitted — style reference image is sufficient
// const defaultPromptAddition = "..."   // Omitted — style reference image is sufficient

const geminiModel = "gemini-3-pro-image-preview"

type GeminiService struct {
	apiKey             string
	styleReferencePath string
	styleImageCache    []byte
	styleMimeType      string
	client             *http.Client
}

func NewGeminiService(apiKey string) *GeminiService {
	return &GeminiService{
		apiKey:             apiKey,
		styleReferencePath: "assets/style-reference/sample.jpeg",
		client:             &http.Client{Timeout: 300 * time.Second},
	}
}

// NewGeminiServiceWithStyleReference creates a Gemini service with a custom style reference image path
func NewGeminiServiceWithStyleReference(apiKey, styleReferencePath string) *GeminiService {
	if styleReferencePath == "" {
		styleReferencePath = "assets/style-reference/sample.jpeg"
	}
	return &GeminiService{
		apiKey:             apiKey,
		styleReferencePath: styleReferencePath,
		client:             &http.Client{Timeout: 300 * time.Second},
	}
}

// Gemini API request/response structures
type GeminiGenerateContentRequest struct {
	Contents         []GeminiContent         `json:"contents"`
	GenerationConfig *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

type GeminiGenerationConfig struct {
	ResponseModalities []string           `json:"responseModalities,omitempty"`
	ImageConfig        *GeminiImageConfig `json:"imageConfig,omitempty"`
}

type GeminiImageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageSize   string `json:"imageSize,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inlineData,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

type GeminiGenerateContentResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
}

type GeminiCandidate struct {
	Content GeminiResponseContent `json:"content"`
}

type GeminiResponseContent struct {
	Parts []GeminiResponsePart `json:"parts"`
}

type GeminiResponsePart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inlineData,omitempty"`
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ImageGenOptions holds per-project overrides for image generation.
// Nil fields mean "use the service-level or global default".
type ImageGenOptions struct {
	VisualStyle    *string // e.g. "photorealistic", "anime", "oil painting"
	AspectRatio    *string // "9:16", "16:9", "1:1", "4:5"
	SampleImageURL *string // URL to a custom style reference image (overrides default sample.jpeg)
}

// GenerateImage generates a single image using Gemini with style reference + style instructions.
// Each call is independent — safe for parallel execution across clips.
// opts carries per-project overrides; nil means use service defaults.
func (s *GeminiService) GenerateImage(ctx context.Context, basePrompt string, preset *models.GraphicsPreset, opts *ImageGenOptions) ([]byte, error) {
	// Resolve aspect ratio — per-project override or default
	aspectRatio := "9:16"
	if opts != nil && opts.AspectRatio != nil && *opts.AspectRatio != "" {
		aspectRatio = *opts.AspectRatio
	}

	// Resolve style reference image:
	// 1. Per-project URL (opts.SampleImageURL) — download it
	// 2. Service-level local file (s.styleReferencePath)
	var styleData []byte
	var mimeType string

	if opts != nil && opts.SampleImageURL != nil && *opts.SampleImageURL != "" {
		// Download style reference from URL
		var err error
		styleData, mimeType, err = s.downloadStyleImage(ctx, *opts.SampleImageURL)
		if err != nil {
			log.Printf("[Gemini] WARNING: could not download custom style image %s: %v (falling back to default)", *opts.SampleImageURL, err)
			// Fall through to load from disk
		}
	}

	if styleData == nil {
		// Load from local file
		var err error
		styleData, mimeType, err = s.loadStyleReferenceImage()
		if err != nil {
			log.Printf("[Gemini] WARNING: could not load style reference image: %v (proceeding without)", err)
			return s.generateWithoutStyleRef(ctx, basePrompt, preset, opts, aspectRatio)
		}
	}

	// Build prompt: style instructions + scene
	promptText := composeImagePrompt(basePrompt, preset, opts, aspectRatio)

	// Build request with style reference image + text prompt
	parts := []GeminiPart{
		{Text: promptText},
		{
			InlineData: &GeminiInlineData{
				MimeType: mimeType,
				Data:     base64.StdEncoding.EncodeToString(styleData),
			},
		},
	}

	reqBody := GeminiGenerateContentRequest{
		Contents: []GeminiContent{
			{Role: "user", Parts: parts},
		},
		GenerationConfig: &GeminiGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			ImageConfig: &GeminiImageConfig{
				AspectRatio: aspectRatio,
				ImageSize:   "4K",
			},
		},
	}

	return s.doGenerateContent(ctx, reqBody)
}

// generateWithoutStyleRef generates an image using only the text prompt (fallback if no style ref)
func (s *GeminiService) generateWithoutStyleRef(ctx context.Context, basePrompt string, preset *models.GraphicsPreset, opts *ImageGenOptions, aspectRatio string) ([]byte, error) {
	promptText := composeImagePrompt(basePrompt, preset, opts, aspectRatio)

	reqBody := GeminiGenerateContentRequest{
		Contents: []GeminiContent{
			{Role: "user", Parts: []GeminiPart{{Text: promptText}}},
		},
		GenerationConfig: &GeminiGenerationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
			ImageConfig: &GeminiImageConfig{
				AspectRatio: aspectRatio,
				ImageSize:   "4K",
			},
		},
	}

	return s.doGenerateContent(ctx, reqBody)
}

func (s *GeminiService) loadStyleReferenceImage() ([]byte, string, error) {
	// Return cached if available
	if s.styleImageCache != nil {
		return s.styleImageCache, s.styleMimeType, nil
	}

	path := s.styleReferencePath
	if path == "" {
		path = "assets/style-reference/sample.jpeg"
	}

	paths := []string{
		path,
		filepath.Join(".", path),
		filepath.Join("/app", path),
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			log.Printf("[Gemini] Loaded style reference image from %s (%d bytes)", p, len(data))
			break
		}
	}

	if err != nil {
		return nil, "", fmt.Errorf("could not load style reference from %v: %w", paths, err)
	}

	mimeType := "image/jpeg"
	if filepath.Ext(path) == ".png" {
		mimeType = "image/png"
	}

	// Cache it
	s.styleImageCache = data
	s.styleMimeType = mimeType

	return data, mimeType, nil
}

func (s *GeminiService) doGenerateContent(ctx context.Context, reqBody GeminiGenerateContentRequest) ([]byte, error) {
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", geminiModel, s.apiKey)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gemini returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var geminiResp GeminiGenerateContentResponse
	if err := json.Unmarshal(bodyBytes, &geminiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(geminiResp.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}

	var textParts []string
	for _, part := range geminiResp.Candidates[0].Content.Parts {
		if part.InlineData != nil && part.InlineData.Data != "" {
			imageData, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to decode base64 image: %w", err)
			}
			return imageData, nil
		}
		if part.Text != "" {
			textParts = append(textParts, part.Text)
		}
	}

	if len(textParts) > 0 {
		return nil, fmt.Errorf("gemini returned text instead of image: %s", textParts[0][:min(200, len(textParts[0]))])
	}
	return nil, fmt.Errorf("no image data found in response (got %d parts, none with inlineData)", len(geminiResp.Candidates[0].Content.Parts))
}

// downloadStyleImage fetches a style reference image from a URL.
func (s *GeminiService) downloadStyleImage(ctx context.Context, url string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to download style image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("style image download returned status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read style image body: %w", err)
	}

	// Detect MIME type from Content-Type header, default to jpeg
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/jpeg"
	}

	log.Printf("[Gemini] Downloaded custom style reference from URL (%d bytes, %s)", len(data), mimeType)
	return data, mimeType, nil
}

// composeImagePrompt builds the full prompt: style reference instruction + scene description.
// The heavy lifting for style is done by the sample image passed as inline data — the text
// prompt only needs to describe the scene and remind Gemini to follow the reference style.
func composeImagePrompt(basePrompt string, preset *models.GraphicsPreset, opts *ImageGenOptions, aspectRatio string) string {
	var prompt bytes.Buffer

	prompt.WriteString("STYLE REFERENCE: Use the attached reference image as the style guide. Copy ONLY the artistic style, brushwork, lighting, color palette, and realism from the reference image. Do NOT copy the subject, people, or scene from the reference.\n\n")

	// If a per-project visual style is specified, inject it prominently
	if opts != nil && opts.VisualStyle != nil && *opts.VisualStyle != "" {
		prompt.WriteString(fmt.Sprintf("VISUAL STYLE: Render this scene in a \"%s\" aesthetic. This overrides any conflicting style cues.\n\n", *opts.VisualStyle))
	}

	if preset != nil && preset.StyleJSON != nil {
		prompt.WriteString("ADDITIONAL PRESET GUIDANCE:\n")
		styleJSON, _ := json.MarshalIndent(preset.StyleJSON, "", "  ")
		prompt.Write(styleJSON)
		prompt.WriteString("\n\n")
	}

	if preset != nil && preset.PromptAddition != nil && *preset.PromptAddition != "" {
		prompt.WriteString(*preset.PromptAddition)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString("SCENE TO DEPICT:\n")
	prompt.WriteString(basePrompt)

	// Build orientation label
	orientLabel := "Portrait"
	if aspectRatio == "16:9" {
		orientLabel = "Landscape"
	} else if aspectRatio == "1:1" {
		orientLabel = "Square"
	} else if aspectRatio == "4:5" {
		orientLabel = "Tall"
	}
	prompt.WriteString(fmt.Sprintf("\n\nOutput: %s %s, highest quality 4K.", orientLabel, aspectRatio))

	return prompt.String()
}
