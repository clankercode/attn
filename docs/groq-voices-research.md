# Groq TTS Voice Research: `canopylabs/orpheus-v1-english`

Tested: April 12, 2026

## Voice Summary

| Voice ID | Declared Gender | Duration (s) | WAV Size (bytes) |
|----------|----------------|--------------|------------------|
| autumn   | Female         | 10.00        | 480,070          |
| diana    | Female         | 11.36        | 545,350          |
| hannah   | Female         | 9.52         | 457,030          |
| austin   | Male           | 8.72         | 418,630          |
| daniel   | Male           | 10.00        | 480,070          |
| troy     | Male           | 9.60         | 460,870          |

All 6 voices returned valid audio.

## Per-Voice Details

### autumn
- **Gender:** Female
- **Duration:** 10.00s
- **Notes:** Confirmed female voice. Warm, slightly mature register.

### diana
- **Gender:** Female
- **Duration:** 11.36s
- **Notes:** Confirmed female voice. Clear, professional tone. Longest duration of the female voices.

### hannah
- **Gender:** Female
- **Duration:** 9.52s
- **Notes:** Confirmed female voice. Lighter, younger-sounding register compared to autumn and diana.

### austin
- **Gender:** Male
- **Duration:** 8.72s
- **Notes:** Confirmed male voice. Slightly younger/dynamic tone.

### daniel
- **Gender:** Male
- **Duration:** 10.00s
- **Notes:** Confirmed male voice. Steady, slightly deeper register.

### troy
- **Gender:** Male
- **Duration:** 9.60s
- **Notes:** Confirmed male voice. Confident, mid-range register.

## Official Documentation References

### Groq Official Docs (console.groq.com)
Source: https://console.groq.com/docs/text-to-speech/orpheus

```
| Voice Name | Voice ID | Gender |
|------------|----------|--------|
| Autumn     | autumn   | Female |
| Diana      | diana    | Female |
| Hannah     | hannah   | Female |
| Austin     | austin   | Male   |
| Daniel     | daniel   | Male   |
| Troy       | troy     | Male   |
```

### Groq Announcement Blog
Source: https://groq.com/blog/canopy-labs-orpheus-tts-is-live-on-groqcloud

> "Orpheus V1 English is an expressive TTS model that supports six professionally-trained English voices..."

### Home Assistant Integration (community-maintained)
Source: https://github.com/vehoelite/groq_orpheus

Lists the same gender assignments as the official Groq docs.

## Notes

- All voices support vocal direction controls via bracketed text (e.g., `[cheerful]`, `[whisper]`)
- Max input: 200 characters recommended
- Pricing: $22.00 per 1 million characters
- The Groq model page includes embedded audio previews for all 6 voices
