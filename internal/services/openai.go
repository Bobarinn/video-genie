package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

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
	Tone        *string // "documentary", "dramatic", "comedic", etc.
	VisualStyle *string // "cinematic watercolor", "photorealistic", etc.
	AspectRatio *string // "9:16", "16:9", "1:1"
	CTA         *string // Call-to-action text for the final clip
	Language    *string // ISO 639-1 code ("en", "es", "fr", ...)
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
	visualStyle := "cinematic watercolor"
	aspectRatio := "9:16"
	language := "en"
	cta := ""

	if opts != nil {
		if opts.Tone != nil && *opts.Tone != "" {
			tone = *opts.Tone
		}
		if opts.VisualStyle != nil && *opts.VisualStyle != "" {
			visualStyle = *opts.VisualStyle
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

	basePrompt := fmt.Sprintf(`You are an expert video content strategist creating short-form video plans for %s (%s aspect ratio).

TONE: %s
The entire script, narration, and mood must match a "%s" tone. Let this guide the vocabulary, pacing, and emotional register of every clip.

VISUAL STYLE: %s
All image_prompt and video_prompt fields must describe scenes in the "%s" visual aesthetic. Embed this style naturally into descriptions — it should feel like every frame was rendered in this style.

LANGUAGE: %s
All script text must be written in the "%s" language. voice_style_instruction should still be written in English (for the TTS engine), but the spoken script itself must be in the specified language.

Your task is to create a compelling %d-second video plan with multiple clips.

Guidelines:
- Each clip should be 8-15 seconds
- Create a strong hook in the first clip (dramatic opening, surprising fact, or compelling question)
- Build narrative momentum across clips
- End with a satisfying conclusion`, orientationDesc, aspectRatio,
		tone, tone, visualStyle, visualStyle, language, language, targetDuration)

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
The script field is narration that will be converted to speech via text-to-speech and played as a voiceover. It is NOT text displayed on screen. Write scripts specifically for SPOKEN delivery:
- Use SHORT, punchy sentences. Break up long sentences into 2-3 shorter ones.
- Write conversationally, as if talking directly to the listener. Use contractions (don't, isn't, they're).
- Avoid jargon, complex clauses, or parenthetical asides that trip up speech synthesis.
- Use natural speech rhythm: vary sentence length. Mix short declarative statements with slightly longer descriptive ones.
- Add natural pauses with punctuation: commas, periods, ellipses (...), and em dashes (—) to create breathing room.
- Start each clip's script with a brief transitional beat — not an abrupt jump. E.g., "Now...", "But here's the thing.", "And then...", "Picture this."
- Avoid starting the very first word with a critical word that cannot be missed — TTS sometimes clips the first syllable. Lead with a soft opener or a brief pause word.
- Each script should feel like one thought or moment, not a wall of text. Aim for 2-5 sentences per clip.
- Read each script aloud in your head. If it sounds rushed, shorten it. If it sounds monotonous, vary the rhythm.

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
image_prompt and video_prompt are NOT separate entities. The image_prompt defines the static starting point; the video_prompt describes what MOTION or CHANGE should happen to that scene. The video will be generated using AI video generation, so write video_prompt as a cinematic video direction.

video_prompt MUST directly relate to the scene in image_prompt. Write it as a CINEMATIC VIDEO DESCRIPTION following these elements:

1. SUBJECT & ACTION (required): What the subject(s) do — keep motion SUBTLE and REALISTIC. Think living photograph, not action movie.
   - Good: "The woman's hair sways gently in a soft breeze. She slowly turns her head, a faint smile forming."
   - Good: "The old man's chest rises and falls with a slow breath. His fingers tap once on the wooden table."
   - Bad: "Woman runs across the street and jumps" (too dramatic, unrealistic)

2. ENVIRONMENTAL MOTION (required): How the environment subtly comes alive:
   - Gentle atmospheric effects: dust motes floating in light, leaves rustling, smoke curling, water rippling, clouds drifting
   - Lighting shifts: golden light slowly intensifying, shadows gradually lengthening, candlelight flickering
   - Weather hints: a light breeze, gentle rain beginning, morning mist slowly dissipating

3. CAMERA MOVEMENT (optional but encouraged): Subtle, cinematic camera work:
   - "Slow, barely perceptible push-in toward the subject's face"
   - "Gentle dolly shot drifting left to right"
   - "Static shot with shallow depth of field, background softly shifting"
   - Avoid: dramatic swoops, fast pans, shaky cam

4. COMPOSITION & FOCUS (optional): Cinematic framing cues:
   - "Close-up maintaining sharp focus on the subject's eyes, background softly blurred"
   - "Wide shot, deep focus, the entire scene gently alive"
   - "Shallow depth of field, foreground candle flame in focus"

5. AMBIANCE (optional): Mood reinforcement:
   - "Warm golden tones, soft ambient glow"
   - "Cool blue twilight atmosphere"

CRITICAL video_prompt rules:
- Motion must be MINIMAL and REALISTIC. The video should feel like a painting that has subtly come to life — not a Hollywood action sequence.
- Favor: gentle breathing, slow blinks, hair swaying, fabric shifting, ambient particles, light flickering, slow camera drift.
- Avoid: fast movement, morphing, teleportation, dramatic gestures, style changes between frames.
- Each video_prompt should read like a cinematographer's shot description.
- Always describe the motion in present tense, as a continuous action.
- Do NOT include audio cues, dialogue, or sound descriptions — the video is silent (narration is separate).

ALL FIELDS ARE REQUIRED - DO NOT LEAVE ANY FIELD EMPTY:
Every clip MUST have ALL of these fields populated with meaningful content:
- script: The narration text (what the voiceover says). NEVER empty. Must be conversational, engaging, and match the clip's scene.
- voice_style_instruction: How the narrator should deliver the script. Should describe tone, pace, and emotion. Always favor SLOW, DELIBERATE pacing for clarity. NEVER empty.
- image_prompt: Full scene description as detailed above. NEVER empty.
- video_prompt: Motion/change description tied to the image. NEVER empty.
- clip_index: Zero-based index (0, 1, 2, 3...). First clip=0, second=1, etc.
- estimated_duration_sec: Approximate seconds (8-15). NEVER zero.

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
		if opts.VisualStyle != nil && *opts.VisualStyle != "" {
			extras = append(extras, fmt.Sprintf("Visual style: %s", *opts.VisualStyle))
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
