# Cartesia TTS Setup Guide

This guide explains how to set up and configure Cartesia Text-to-Speech for the Episod.

## Getting Started with Cartesia

### 1. Create a Cartesia Account

1. Visit [https://cartesia.ai](https://cartesia.ai)
2. Sign up for an account
3. Navigate to your dashboard

### 2. Get Your API Key

1. Go to your Cartesia dashboard
2. Navigate to API Keys section
3. Create a new API key
4. Copy the key (it starts with `sk-...`)

### 3. Choose a Voice

Cartesia provides a library of voices. You need to select a voice ID for your videos.

#### Finding Voice IDs

**Option 1: Use Cartesia Playground**
1. Go to [Cartesia Playground](https://play.cartesia.ai)
2. Browse available voices
3. Each voice has a unique ID (UUID format)
4. Test voices and note the ID of your favorite

**Option 2: Use the Cartesia API**

```bash
curl https://api.cartesia.ai/voices \
  -H "Cartesia-Version: 2024-06-10" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

This returns a list of available voices with their IDs.

#### Popular Voice Examples

Here are some example voice IDs (verify these are available in your account):

```
a0e99841-438c-4a64-b679-ae501e7d6091  # Professional Male
b7d50908-b17c-442d-ad8d-810c63997ed9  # Professional Female
c2ac25f9-ecc4-4f56-9095-651354df60c0  # Friendly Male
d76c4d19-8e3f-4e6a-9a05-6b5c8e3d7f2a  # Friendly Female
```

## Configuration

### Basic Setup (.env file)

```bash
# Required: Your Cartesia API key
CARTESIA_API_KEY=sk-your-actual-api-key-here

# Required: API base URL (default is correct)
CARTESIA_API_URL=https://api.cartesia.ai

# Optional: Default voice ID (if not set, uses built-in default)
CARTESIA_VOICE_ID=a0e99841-438c-4a64-b679-ae501e7d6091
```

### Advanced Configuration

The system automatically configures:
- **API Version**: `2024-06-10` (configured in code)
- **Output Format**: MP3 at 44.1kHz, 192kbps
- **Model**: `sonic-english` for English content

## Voice Style and Emotion

The system intelligently maps voice style instructions to Cartesia emotions.

### Automatic Emotion Detection

When you provide a voice style instruction in your video plan, the system automatically detects the emotion:

```
"energetic and engaging" → excited
"serious and authoritative" → confident
"calm and peaceful" → calm
"mysterious" → mysterious
"dramatic" → intense
```

### Supported Emotions

Cartesia supports these emotions (using `sonic-3` model):

**Primary Emotions:**
- `neutral` (default)
- `calm`
- `angry`
- `content`
- `sad`
- `scared`

**Extended Emotions:**
- `happy`, `excited`, `enthusiastic`, `elated`, `euphoric`
- `peaceful`, `serene`, `grateful`, `affectionate`
- `confident`, `proud`, `determined`
- `anxious`, `panicked`, `alarmed`
- `disappointed`, `hurt`, `guilty`, `bored`
- `curious`, `mysterious`, `contemplative`
- And many more (see Cartesia documentation)

### Example Usage in Video Plans

When OpenAI generates your video plan, it includes voice style instructions like:

```json
{
  "voice_style_instruction": "Energetic and engaging, building excitement"
}
```

The system automatically:
1. Detects "energetic" → maps to `excited` emotion
2. Sends to Cartesia with `emotion: "excited"`
3. Generates audio with that emotional tone

## Advanced Features

### Speed Control

The system supports speed adjustment (0.6x to 1.5x):

```go
// In your code modifications
opts := GenerateSpeechOptions{
    Speed: 1.2,  // 20% faster
}
```

### Volume Control

Adjust volume (0.5x to 2.0x):

```go
opts := GenerateSpeechOptions{
    Volume: 1.5,  // 50% louder
}
```

### Multi-Language Support

Cartesia supports multiple languages. Set in `.env`:

```bash
# Supported: en, fr, de, es, pt, zh, ja, hi, it, ko, nl, pl, ru, sv, tr, etc.
CARTESIA_LANGUAGE=en
```

Or configure per-request in code:

```go
opts := GenerateSpeechOptions{
    Language: "es",  // Spanish
}
```

## Testing Your Setup

### 1. Verify API Key

```bash
curl https://api.cartesia.ai/voices \
  -H "Cartesia-Version: 2024-06-10" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

Should return a list of voices.

### 2. Test TTS Generation

```bash
curl -X POST https://api.cartesia.ai/tts/bytes \
  -H "Cartesia-Version: 2024-06-10" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_id": "sonic-english",
    "transcript": "Hello! This is a test of Cartesia text to speech.",
    "voice": {
      "mode": "id",
      "id": "a0e99841-438c-4a64-b679-ae501e7d6091"
    },
    "output_format": {
      "container": "mp3",
      "sample_rate": 44100,
      "bit_rate": 192000
    }
  }' \
  --output test.mp3
```

Should download `test.mp3` with the spoken text.

## Troubleshooting

### Error: "Invalid API key"

- Verify your API key starts with `sk-`
- Check it's correctly set in `.env` file
- Ensure no extra spaces or quotes in `.env`

### Error: "Voice not found"

- The voice ID may not exist in your account
- List available voices with the API
- Update `CARTESIA_VOICE_ID` in `.env`

### Error: "Rate limit exceeded"

- You've hit API rate limits
- Check your Cartesia plan limits
- Reduce `MAX_CONCURRENT_JOBS` in `.env`
- Wait a few minutes and try again

### Audio Quality Issues

If generated audio quality is poor:

1. **Check voice selection** - Try different voices
2. **Adjust speed** - Slower speeds often sound better
3. **Simplify script** - Complex sentences may not render well
4. **Check emotion** - Some emotions may not suit all voices

### Generation Takes Too Long

- Typical generation: 5-15 seconds per clip
- Longer scripts take more time
- Network latency can add delays
- Check Cartesia status page for service issues

## Cost Considerations

Cartesia pricing (check their website for current rates):

- **Free tier**: Limited requests per month
- **Pro tier**: Higher limits and priority processing
- **Enterprise**: Custom limits

Typical costs:
- ~100 characters = ~$0.01-0.02 (varies by plan)
- 90-second video (~1000 characters) = ~$0.10-0.30

Cost optimization:
1. Use appropriate voice (some may cost more)
2. Optimize scripts (remove filler words)
3. Cache generated audio (don't regenerate unnecessarily)

## Best Practices

### 1. Voice Selection

- **Professional content**: Use professional voices
- **Casual content**: Use friendly voices
- **Technical content**: Use clear, articulate voices
- **Consistency**: Use same voice across a series

### 2. Script Writing

For best TTS results:

✅ **Good:**
```
"The discovery of coffee changed everything.
People finally had a way to stay awake during boring meetings."
```

❌ **Avoid:**
```
"The discovery of coffee (which happened in Ethiopia, btw)
changed everything!!! People FINALLY had a way to stay awake
during boring meetings... or did they???"
```

**Tips:**
- Use natural punctuation (periods, commas)
- Avoid excessive exclamation marks
- Spell out numbers: "twenty" not "20"
- Use standard spellings, not phonetic
- Break long sentences into shorter ones

### 3. Emotion Control

- Use emotions that match content tone
- Don't overuse extreme emotions
- Test different emotions for best results
- `neutral` is safest for unclear content

### 4. Performance

- Default settings work well for most use cases
- Only adjust speed/volume if needed
- Faster speeds may reduce quality
- Test changes with your voice before production

## Voice Library Management

### Creating Voice Collections

For different content types, maintain voice IDs:

```bash
# Professional content
VOICE_PROFESSIONAL_MALE=a0e99841-438c-4a64-b679-ae501e7d6091
VOICE_PROFESSIONAL_FEMALE=b7d50908-b17c-442d-ad8d-810c63997ed9

# Casual content
VOICE_FRIENDLY_MALE=c2ac25f9-ecc4-4f56-9095-651354df60c0
VOICE_FRIENDLY_FEMALE=d76c4d19-8e3f-4e6a-9a05-6b5c8e3d7f2a

# Storytelling
VOICE_NARRATOR_DEEP=e8f93a2b-1c4d-4e5f-8a9b-7c6d5e4f3a2b
```

### Per-Series Voice Configuration

When you implement Series (future feature), you can configure default voices per series in the database:

```sql
UPDATE series
SET default_voice_profile = '{"voice_id": "a0e99841-438c-4a64-b679-ae501e7d6091"}'
WHERE name = 'Tech Explained';
```

## API Reference

### Request Format

```json
{
  "model_id": "sonic-english",
  "transcript": "Your text here",
  "voice": {
    "mode": "id",
    "id": "voice-uuid-here"
  },
  "output_format": {
    "container": "mp3",
    "sample_rate": 44100,
    "bit_rate": 192000
  },
  "generation_config": {
    "emotion": "excited",
    "speed": 1.0,
    "volume": 1.0
  }
}
```

### Available Models

- `sonic-english` - Best for English content
- `sonic-multilingual` - Supports 30+ languages
- `sonic-3` - Latest model with emotion control

### Output Formats

**MP3** (recommended):
```json
{
  "container": "mp3",
  "sample_rate": 44100,
  "bit_rate": 192000
}
```

**WAV** (higher quality, larger files):
```json
{
  "container": "wav",
  "encoding": "pcm_s16le",
  "sample_rate": 44100
}
```

## Resources

- [Cartesia Documentation](https://docs.cartesia.ai)
- [Cartesia Playground](https://play.cartesia.ai)
- [Cartesia API Reference](https://docs.cartesia.ai/reference)
- [Voice Library](https://cartesia.ai/voices)
- [Pricing](https://cartesia.ai/pricing)

## Support

For Cartesia-specific issues:
- Check [Cartesia Status](https://status.cartesia.ai)
- Contact Cartesia support
- Join Cartesia Discord community

For integration issues:
- See main [README.md](../README.md)
- Check [DEVELOPMENT.md](../DEVELOPMENT.md)
- Review [API_EXAMPLES.md](../API_EXAMPLES.md)
