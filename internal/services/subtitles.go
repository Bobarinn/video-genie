package services

import (
	"fmt"
	"os"
	"strings"
)

// ---------------------------------------------------------------------------
// TikTok-Style ASS Subtitle Generator
//
// Generates word-by-word highlighted subtitles in ASS (Advanced SubStation Alpha)
// format. Words are shown in small chunks (3-4 at a time) with the currently
// spoken word highlighted in a purple "pill" background.
//
// Visual style:
//   - Bold white uppercase text, centered at bottom of portrait (1080x1920) video
//   - Dark outline on all words for readability on any background
//   - Active word: thick purple border creating a "pill highlight" effect
//   - Smooth transitions: each chunk appears/disappears as a group
// ---------------------------------------------------------------------------

const (
	// How many words to show at once (TikTok typically shows 3-4)
	wordsPerChunk = 4

	// ASS font configuration — must match a font installed in the Docker container.
	// Noto Sans is installed via Alpine's font-noto package. If unavailable, libass
	// falls back to the default fontconfig match (typically DejaVu Sans on Alpine).
	// We also install fontconfig and run fc-cache in the Dockerfile to ensure discovery.
	subtitleFontName = "Noto Sans"

	// ASS colors are in &HAABBGGRR format (hex, note: BGR not RGB)
	assColorWhite     = "&H00FFFFFF" // pure white
	assColorBlack     = "&H00000000" // pure black (for outline)
	assColorPurple    = "&H00CC3299" // #9932CC in BGR — rich purple for highlight
	assColorSemiBlack = "&H80000000" // 50% transparent black (for shadow)
)

// SubtitleParams holds resolution-aware subtitle styling parameters.
type SubtitleParams struct {
	PlayResX         int
	PlayResY         int
	FontSize         int
	OutlineNormal    int
	OutlineHighlight int
	MarginV          int
}

// SubtitleParamsForResolution returns font sizes and margins scaled to the render resolution.
func SubtitleParamsForResolution(res RenderResolution) SubtitleParams {
	if res.Width >= 2160 {
		// 4K: 2160x3840
		return SubtitleParams{
			PlayResX: 2160, PlayResY: 3840,
			FontSize: 124, OutlineNormal: 6, OutlineHighlight: 16, MarginV: 440,
		}
	}
	// 1080p: 1080x1920 (default)
	return SubtitleParams{
		PlayResX: 1080, PlayResY: 1920,
		FontSize: 62, OutlineNormal: 3, OutlineHighlight: 8, MarginV: 220,
	}
}

// GenerateASSSubtitles creates a TikTok-style ASS subtitle file from word timestamps.
//
// Parameters:
//   - words: word-level timestamps from Whisper transcription
//   - outputPath: path to write the .ass file
//   - silenceOffsetSec: time offset to add to all timestamps (e.g., 0.5 for 500ms prepended silence)
//   - params: resolution-aware subtitle styling (font size, margins, etc.)
//
// The generated subtitles show words in chunks of ~4, with the active word
// highlighted in purple. All text is bold, uppercase, centered at the bottom.
func GenerateASSSubtitles(words []WordTimestamp, outputPath string, silenceOffsetSec float64, params SubtitleParams) error {
	if len(words) == 0 {
		return fmt.Errorf("no words to generate subtitles from")
	}

	// Group words into display chunks
	chunks := chunkWords(words, wordsPerChunk)

	// Build ASS content
	var sb strings.Builder

	// Script header — PlayRes must match the render resolution so font sizes look correct
	sb.WriteString("[Script Info]\n")
	sb.WriteString("ScriptType: v4.00+\n")
	sb.WriteString(fmt.Sprintf("PlayResX: %d\n", params.PlayResX))
	sb.WriteString(fmt.Sprintf("PlayResY: %d\n", params.PlayResY))
	sb.WriteString("WrapStyle: 0\n")
	sb.WriteString("ScaledBorderAndShadow: yes\n")
	sb.WriteString("\n")

	// Style definitions
	sb.WriteString("[V4+ Styles]\n")
	sb.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")

	// Default style: bold white text with black outline, bottom-center aligned
	sb.WriteString(fmt.Sprintf(
		"Style: Default,%s,%d,%s,%s,%s,%s,-1,0,0,0,100,100,2,0,1,%d,0,2,40,40,%d,1\n",
		subtitleFontName, params.FontSize,
		assColorWhite,         // PrimaryColour (text)
		assColorWhite,         // SecondaryColour
		assColorBlack,         // OutlineColour
		assColorSemiBlack,     // BackColour (shadow)
		params.OutlineNormal,  // Outline thickness
		params.MarginV,        // MarginV (distance from bottom)
	))

	sb.WriteString("\n")

	// Events (dialogue lines)
	sb.WriteString("[Events]\n")
	sb.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	// Generate dialogue lines for each word in each chunk
	for _, chunk := range chunks {
		for wordIdx, word := range chunk {
			// Calculate timing with silence offset
			startTime := word.Start + silenceOffsetSec
			var endTime float64

			if wordIdx < len(chunk)-1 {
				// End when the next word starts (seamless transition)
				endTime = chunk[wordIdx+1].Start + silenceOffsetSec
			} else {
				// Last word in chunk: end at the word's own end time
				endTime = word.End + silenceOffsetSec
			}

			// Build the display text with the active word highlighted
			displayText := buildHighlightedChunkText(chunk, wordIdx, params.OutlineHighlight)

			// Write the dialogue line
			sb.WriteString(fmt.Sprintf(
				"Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
				formatASSTime(startTime),
				formatASSTime(endTime),
				displayText,
			))
		}
	}

	// Write to file
	if err := os.WriteFile(outputPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write ASS subtitle file: %w", err)
	}

	return nil
}

// chunkWords groups words into display chunks of the specified size.
// It also breaks at sentence boundaries (., !, ?) to keep chunks natural.
func chunkWords(words []WordTimestamp, chunkSize int) [][]WordTimestamp {
	var chunks [][]WordTimestamp
	var current []WordTimestamp

	for _, word := range words {
		current = append(current, word)

		// Break chunk if we've reached the target size
		// OR if the word ends with sentence-ending punctuation
		isSentenceEnd := strings.ContainsAny(word.Word, ".!?")
		if len(current) >= chunkSize || (isSentenceEnd && len(current) >= 2) {
			chunks = append(chunks, current)
			current = nil
		}
	}

	// Don't forget the last partial chunk
	if len(current) > 0 {
		chunks = append(chunks, current)
	}

	return chunks
}

// buildHighlightedChunkText builds the ASS-formatted text for a chunk where
// the word at activeIdx is highlighted with a purple pill background.
//
// Output example: "THE {\3c&H9932CC&\bord8}HISTORY{\r} OF COFFEE"
func buildHighlightedChunkText(chunk []WordTimestamp, activeIdx int, outlineHighlight int) string {
	var parts []string

	for i, word := range chunk {
		cleanWord := strings.ToUpper(strings.TrimSpace(word.Word))
		if cleanWord == "" {
			continue
		}

		if i == activeIdx {
			// Highlighted word: thick purple border creates the "pill" effect
			// \3c sets outline color, \bord sets outline thickness
			// \r resets back to the default style after this word
			parts = append(parts, fmt.Sprintf(
				"{\\3c%s\\bord%d}%s{\\r}",
				assColorPurple, outlineHighlight, cleanWord,
			))
		} else {
			// Normal word: just the text (default style applies: white + black outline)
			parts = append(parts, cleanWord)
		}
	}

	return strings.Join(parts, " ")
}

// formatASSTime converts seconds to ASS timestamp format: H:MM:SS.CC (centiseconds)
func formatASSTime(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}

	hours := int(seconds) / 3600
	minutes := (int(seconds) % 3600) / 60
	secs := int(seconds) % 60
	centiseconds := int((seconds - float64(int(seconds))) * 100)

	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, secs, centiseconds)
}
