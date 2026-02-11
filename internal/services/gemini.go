package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/bobarin/episod/internal/models"
)

const geminiModel = "gemini-3-pro-image-preview"

type GeminiService struct {
	apiKey string
	client *http.Client
}

func NewGeminiService(apiKey string) *GeminiService {
	return &GeminiService{
		apiKey: apiKey,
		client: &http.Client{Timeout: 300 * time.Second},
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
	AspectRatio *string // "9:16", "16:9", "1:1", "4:5"
}

// GenerateImage generates a single image using Gemini guided by the graphics preset.
// Gemini uses its own creative interpretation plus the preset's style instructions.
// Each call is independent — safe for parallel execution across clips.
// In the future, presets may include their own sample reference image from the database.
func (s *GeminiService) GenerateImage(ctx context.Context, basePrompt string, preset *models.GraphicsPreset, opts *ImageGenOptions) ([]byte, error) {
	// Resolve aspect ratio — per-project override or default
	aspectRatio := "9:16"
	if opts != nil && opts.AspectRatio != nil && *opts.AspectRatio != "" {
		aspectRatio = *opts.AspectRatio
	}

	// Build prompt from preset style instructions + scene description
	promptText := composeImagePrompt(basePrompt, preset, aspectRatio)

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

// composeImagePrompt builds the full prompt from the graphics preset + scene description.
// Gemini uses its creative interpretation guided by the preset's style instructions.
func composeImagePrompt(basePrompt string, preset *models.GraphicsPreset, aspectRatio string) string {
	var prompt bytes.Buffer

	// Visual style from graphics preset
	if preset != nil {
		prompt.WriteString(fmt.Sprintf("VISUAL STYLE: Render this scene in a \"%s\" aesthetic.\n", preset.Name))
		if preset.Description != nil && *preset.Description != "" {
			prompt.WriteString(fmt.Sprintf("Style guidance: %s\n", *preset.Description))
		}
		prompt.WriteString("\n")

		if preset.StyleJSON != nil {
			prompt.WriteString("STYLE PARAMETERS:\n")
			styleJSON, _ := json.MarshalIndent(preset.StyleJSON, "", "  ")
			prompt.Write(styleJSON)
			prompt.WriteString("\n\n")
		}

		if preset.PromptAddition != nil && *preset.PromptAddition != "" {
			prompt.WriteString(*preset.PromptAddition)
			prompt.WriteString("\n\n")
		}
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
