package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/bobarin/episod/internal/models"
	openai "github.com/sashabaranov/go-openai"
)

type OpenAIService struct {
	client *openai.Client
}

func NewOpenAIService(apiKey string) *OpenAIService {
	return &OpenAIService{
		client: openai.NewClient(apiKey),
	}
}

// ClipPlan represents a single clip in the generation plan
type ClipPlan struct {
	ClipIndex             int    `json:"clip_index"`
	Script                string `json:"script"`
	VoiceStyleInstruction string `json:"voice_style_instruction"`
	ImagePrompt           string `json:"image_prompt"`
	VideoPrompt           string `json:"video_prompt"`
	EstimatedDurationSec  int    `json:"estimated_duration_sec"`
}

// VideoPlan represents the complete plan for video generation
type VideoPlan struct {
	Clips              []ClipPlan `json:"clips"`
	TotalEstimatedSec  int        `json:"total_estimated_sec"`
	NarrativeStructure string     `json:"narrative_structure"`
}

// PlanOptions holds per-project customization passed into plan generation.
// All fields are optional pointers — nil means "use defaults".
type PlanOptions struct {
	Tone        *string                // "documentary", "dramatic", "comedic", etc.
	Preset      *models.GraphicsPreset // Visual style preset (name, description, style_json, prompt_addition)
	AspectRatio *string                // "9:16", "16:9", "1:1"
	CTA         *string                // Call-to-action text for the final clip
	Language    *string                // ISO 639-1 code ("en", "es", "fr", ...)
}

// GeneratePlan generates a video plan using OpenAI structured output.
// opts carries per-project customization; nil fields use global defaults.
func (s *OpenAIService) GeneratePlan(ctx context.Context, topic string, targetDuration int, seriesGuidance *string, opts *PlanOptions) (*VideoPlan, error) {
	// Build system prompt
	systemPrompt := buildPlanSystemPrompt(targetDuration, seriesGuidance, opts)

	// Build user prompt
	userPrompt := buildPlanUserPrompt(topic, targetDuration, opts)

	// Call OpenAI with structured output (using JSON mode)
	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "gpt-5-mini", // gpt-5-mini best for reasoning and cost efficiency
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 1.0,
	})

	if err != nil {
		return nil, fmt.Errorf("openai request failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response from openai")
	}

	rawContent := resp.Choices[0].Message.Content
	const maxLogLen = 2000

	// Parse the JSON response
	var plan VideoPlan
	if err := json.Unmarshal([]byte(rawContent), &plan); err != nil {
		log.Printf("[OpenAI plan] parse failed: %v", err)
		if len(rawContent) > maxLogLen {
			log.Printf("[OpenAI plan] raw response (truncated): %s...", rawContent[:maxLogLen])
		} else {
			log.Printf("[OpenAI plan] raw response: %s", rawContent)
		}
		return nil, fmt.Errorf("failed to parse plan: %w", err)
	}

	// Validate plan
	if len(plan.Clips) == 0 {
		log.Printf("[OpenAI plan] plan has no clips (total_estimated_sec=%d, narrative_structure=%q)", plan.TotalEstimatedSec, plan.NarrativeStructure)
		if len(rawContent) > maxLogLen {
			log.Printf("[OpenAI plan] raw response (truncated): %s...", rawContent[:maxLogLen])
		} else {
			log.Printf("[OpenAI plan] raw response: %s", rawContent)
		}
		planJSON, _ := json.MarshalIndent(plan, "", "  ")
		log.Printf("[OpenAI plan] parsed plan: %s", string(planJSON))
		return nil, fmt.Errorf("plan has no clips")
	}

	// Validate all required fields on each clip
	for i, clip := range plan.Clips {
		var missing []string
		if clip.Script == "" {
			missing = append(missing, "script")
		}
		if clip.VoiceStyleInstruction == "" {
			missing = append(missing, "voice_style_instruction")
		}
		if clip.ImagePrompt == "" {
			missing = append(missing, "image_prompt")
		}
		if clip.VideoPrompt == "" {
			missing = append(missing, "video_prompt")
		}
		if clip.EstimatedDurationSec == 0 {
			missing = append(missing, "estimated_duration_sec")
		}
		if len(missing) > 0 {
			log.Printf("[OpenAI plan] clip %d missing required fields: %v", i, missing)
			if len(rawContent) > maxLogLen {
				log.Printf("[OpenAI plan] raw response (truncated): %s...", rawContent[:maxLogLen])
			} else {
				log.Printf("[OpenAI plan] raw response: %s", rawContent)
			}
			return nil, fmt.Errorf("clip %d missing required fields: %v", i, missing)
		}
	}

	log.Printf("[OpenAI plan] plan generated: %d clips, total_estimated_sec=%d, narrative=%q",
		len(plan.Clips), plan.TotalEstimatedSec, plan.NarrativeStructure)

	return &plan, nil
}

// ---------------------------------------------------------------------------
// Whisper Transcription — word-level timestamps for subtitle generation
// ---------------------------------------------------------------------------

// WordTimestamp represents a single word with its precise timing from Whisper.
// Used to generate TikTok-style word-by-word highlighted subtitles.
type WordTimestamp struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"` // seconds
	End   float64 `json:"end"`   // seconds
}

// TranscribeAudio sends audio to OpenAI Whisper and returns word-level timestamps.
// The audio bytes should be the raw TTS output (before any silence prepend).
// The caller is responsible for adding any time offset (e.g., for prepended silence).
func (s *OpenAIService) TranscribeAudio(ctx context.Context, audioData []byte, language string) ([]WordTimestamp, error) {
	if language == "" {
		language = "en"
	}

	resp, err := s.client.CreateTranscription(ctx, openai.AudioRequest{
		Model:    openai.Whisper1,
		Reader:   bytes.NewReader(audioData),
		FilePath: "audio.mp3", // Filename hint for the API (required by the library)
		Format:   openai.AudioResponseFormatVerboseJSON,
		Language: language,
		TimestampGranularities: []openai.TranscriptionTimestampGranularity{
			openai.TranscriptionTimestampGranularityWord,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("whisper transcription failed: %w", err)
	}

	if len(resp.Words) == 0 {
		return nil, fmt.Errorf("whisper returned no word timestamps (text: %q)", resp.Text)
	}

	words := make([]WordTimestamp, len(resp.Words))
	for i, w := range resp.Words {
		words[i] = WordTimestamp{
			Word:  strings.TrimSpace(w.Word),
			Start: w.Start,
			End:   w.End,
		}
	}

	log.Printf("[Whisper] Transcribed %d words (duration: %.1fs, text: %q)",
		len(words), resp.Duration, truncateString(resp.Text, 80))

	return words, nil
}

// truncateString truncates a string to maxLen and appends "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func buildPlanSystemPrompt(targetDuration int, seriesGuidance *string, opts *PlanOptions) string {
	// Resolve customization with defaults
	tone := "documentary"
	visualStyle := "Hyper Realistic"
	visualStyleDesc := ""
	aspectRatio := "9:16"
	language := "en"
	cta := ""

	if opts != nil {
		if opts.Tone != nil && *opts.Tone != "" {
			tone = *opts.Tone
		}
		if opts.Preset != nil {
			visualStyle = opts.Preset.Name
			if opts.Preset.Description != nil && *opts.Preset.Description != "" {
				visualStyleDesc = *opts.Preset.Description
			}
			if opts.Preset.PromptAddition != nil && *opts.Preset.PromptAddition != "" {
				if visualStyleDesc != "" {
					visualStyleDesc += " "
				}
				visualStyleDesc += *opts.Preset.PromptAddition
			}
		}
		if opts.AspectRatio != nil && *opts.AspectRatio != "" {
			aspectRatio = *opts.AspectRatio
		}
		if opts.Language != nil && *opts.Language != "" {
			language = *opts.Language
		}
		if opts.CTA != nil && *opts.CTA != "" {
			cta = *opts.CTA
		}
	}

	// Determine orientation description from aspect ratio
	orientationDesc := "portrait-format viewing (like TikTok/Reels/Shorts)"
	if aspectRatio == "16:9" {
		orientationDesc = "landscape-format viewing (like YouTube)"
	} else if aspectRatio == "1:1" {
		orientationDesc = "square-format viewing (like Instagram feed)"
	} else if aspectRatio == "4:5" {
		orientationDesc = "tall rectangular viewing (like Instagram portrait)"
	}

	// Build visual style section from preset
	visualStyleSection := fmt.Sprintf(`VISUAL STYLE: %s
All image_prompt and video_prompt fields must describe scenes in the "%s" visual aesthetic. Embed this style naturally into descriptions — it should feel like every frame was rendered in this style.`, visualStyle, visualStyle)
	if visualStyleDesc != "" {
		visualStyleSection += fmt.Sprintf("\nStyle guidance: %s", visualStyleDesc)
	}

	basePrompt := fmt.Sprintf(`You are an expert video content strategist creating short-form video plans for %s (%s aspect ratio).

TONE: %s
The entire script, narration, and mood must match a "%s" tone. Let this guide the vocabulary, pacing, and emotional register of every clip.

%s

LANGUAGE: %s
All script text must be written in the "%s" language. voice_style_instruction should still be written in English (for the TTS engine), but the spoken script itself must be in the specified language.

Your task is to create a compelling %d-second video plan with multiple clips.

WRITING PROCESS - THINK LIKE A STORYTELLER, NOT A CLIP MACHINE:
Before writing any individual clip, mentally compose the ENTIRE narrative as one flowing story — as if you were writing a single short essay or monologue. Think about:
- What's the hook that makes someone stop scrolling?
- What's the journey? What does the listener learn, feel, or discover?
- What's the payoff? Why was this worth their time?

Only AFTER you have the full narrative arc in mind should you divide it into clips. Each clip should feel like a natural breath in a continuous story — not an isolated paragraph. When you read all the scripts back-to-back, they should sound like one person telling one cohesive, engaging story.

Guidelines:
- Each clip should be approximately 10 seconds of spoken narration (estimated_duration_sec = 10 for most clips)
- Scripts should be written so that when read aloud at a natural pace, they take about 8-10 seconds. This is roughly 2-3 short sentences per clip.
- AI video generation produces 12-second clips, so the audio MUST be slightly shorter than the video. Never write scripts longer than ~10 seconds of speech.
- Create a strong hook in the first clip — not a generic intro, but something that creates genuine curiosity or surprise
- Build narrative momentum across clips — each clip should make the listener want to hear the next one
- End with a satisfying conclusion that feels earned, not abrupt`, orientationDesc, aspectRatio,
		tone, tone, visualStyleSection, language, language, targetDuration)

	// Add CTA instruction if provided
	if cta != "" {
		basePrompt += fmt.Sprintf(`
- The FINAL clip's script MUST end with this call-to-action: "%s". Weave it naturally into the narration — do not just append it mechanically.`, cta)
	} else {
		basePrompt += `
- End with a satisfying conclusion or call-to-action`
	}

	basePrompt += fmt.Sprintf(`
- Voice instructions should match the mood of each clip

SCRIPT WRITING - CRITICAL (scripts are read aloud as voiceover narration):
The script field is narration that will be converted to speech via text-to-speech and played as a voiceover. It is NOT text displayed on screen. Write it to be LISTENED to, not read.

Story quality:
- Write like a great storyteller, not an encyclopedia. The listener should feel something — curiosity, awe, surprise, amusement. Facts alone are boring; facts wrapped in a story are compelling.
- Avoid the "list of facts" trap. Don't just state fact after fact — connect them. Use cause and effect, contrast, tension, and revelation. "Most people think X. But actually..." is more engaging than "X is true. Y is also true."
- Vary the emotional register. Don't stay at the same intensity the whole time. Build, release, build again. A moment of quiet makes the next dramatic beat hit harder.
- Make transitions invisible. Each clip should flow into the next as if they were always meant to be together. The listener should never feel a jarring shift. Use connective tissue: callbacks, thematic threads, narrative momentum.

Spoken delivery:
- Use SHORT, punchy sentences. Break up long sentences into 2-3 shorter ones.
- Write conversationally, as if talking directly to the listener. Use contractions (don't, isn't, they're).
- Avoid jargon, complex clauses, or parenthetical asides that trip up speech synthesis.
- Use natural speech rhythm: vary sentence length. Mix short declarative statements with slightly longer descriptive ones.
- Add natural pauses with punctuation: commas, periods, ellipses (...), and em dashes (—) to create breathing room.
- Avoid starting the very first word with a critical word that cannot be missed — TTS sometimes clips the first syllable. Lead with a soft opener.
- Each script should feel like one thought or moment, not a wall of text. Aim for 2-3 sentences per clip.
- Read each script aloud in your head. If it sounds rushed, shorten it. If it sounds like a Wikipedia article, rewrite it.

IMAGE PROMPTS - CRITICAL:
Every image_prompt MUST be a complete, detailed scene description rendered in the "%s" visual style. Include ALL of the following:

1. SUBJECT(S) - The main focus:
   - Who or what is in the scene (person, object, symbol)
   - Specific details: appearance, pose, expression, clothing, age/era if relevant
   - Position in frame (e.g., "centered", "upper third", "walking toward camera")

2. BACKGROUND - The setting/environment:
   - Location (street, room, landscape, office, marketplace, etc.)
   - Architectural or natural elements (buildings, trees, furniture, sky)
   - Time of day and lighting (golden hour, midday sun, dim interior, etc.)

3. SURROUNDINGS / CONTEXT - The full picture:
   - People around the subject (passersby on a street, crowd in background, lone figure in distance)
   - Objects, props, or environmental details that add context
   - Atmosphere (busy market, empty hallway, rainy street, bustling café)
   - Depth layers: foreground, midground, background elements

4. FORMAT (%s) - Optimize for the target aspect ratio:
   - Compose for %s framing
   - Think about what looks compelling in this format

IMAGE_PROMPT + VIDEO_PROMPT - THEY WORK TOGETHER:
image_prompt defines the visual scene. video_prompt describes how that scene comes to life as a 12-second cinematic video clip. AI video generation will animate the image, so write video_prompt as a film director's shot description.

video_prompt guidelines:
- Describe the motion, camera movement, and atmosphere you want to see in the scene.
- Write in present tense as a continuous action: "The camera slowly pushes in as wind moves through the trees..."
- Include subject motion (what characters/objects do), environmental motion (weather, particles, light), and camera direction.
- Motion should feel cinematic and natural — not frantic or chaotic.
- Do NOT include audio cues, dialogue, or sound descriptions — the video is silent (narration is separate).

ALL FIELDS ARE REQUIRED - DO NOT LEAVE ANY FIELD EMPTY:
Every clip MUST have ALL of these fields populated with meaningful content:
- script: The narration text (what the voiceover says). NEVER empty. Must be conversational, engaging, and match the clip's scene.
- voice_style_instruction: How the narrator should deliver the script. Should describe tone, pace, and emotion. Always favor SLOW, DELIBERATE pacing for clarity. NEVER empty.
- image_prompt: Full scene description as detailed above. NEVER empty.
- video_prompt: Motion/change description tied to the image. NEVER empty.
- clip_index: Zero-based index (0, 1, 2, 3...). First clip=0, second=1, etc.
- estimated_duration_sec: Approximate seconds (target: 10 for most clips, range 8-12). NEVER zero.

Top-level fields (also required):
- total_estimated_sec: Sum of all clip durations; should approximate %d seconds. NEVER zero.
- narrative_structure: Brief description of the overall arc (hook, build, payoff). NEVER empty.

If ANY field is empty or zero, the plan is INVALID and will be rejected.

Structure your response as JSON matching the required schema.`, visualStyle, aspectRatio, aspectRatio, targetDuration)

	if seriesGuidance != nil && *seriesGuidance != "" {
		basePrompt += fmt.Sprintf("\n\nSeries Guidance:\n%s", *seriesGuidance)
	}

	return basePrompt
}

// buildPlanUserPrompt constructs the user-facing prompt with customization context.
func buildPlanUserPrompt(topic string, targetDuration int, opts *PlanOptions) string {
	prompt := fmt.Sprintf("Generate a compelling short-form video plan for the topic: \"%s\"\n\nTarget duration: %d seconds", topic, targetDuration)

	// Add customization context so the model has it in the user turn too
	if opts != nil {
		var extras []string
		if opts.Tone != nil && *opts.Tone != "" {
			extras = append(extras, fmt.Sprintf("Tone: %s", *opts.Tone))
		}
		if opts.Preset != nil {
			extras = append(extras, fmt.Sprintf("Visual style: %s", opts.Preset.Name))
		}
		if opts.Language != nil && *opts.Language != "" && *opts.Language != "en" {
			extras = append(extras, fmt.Sprintf("Language: %s", *opts.Language))
		}
		if opts.CTA != nil && *opts.CTA != "" {
			extras = append(extras, fmt.Sprintf("End with CTA: \"%s\"", *opts.CTA))
		}
		if len(extras) > 0 {
			prompt += "\n\nCustomization:\n- " + strings.Join(extras, "\n- ")
		}
	}

	return prompt
}
