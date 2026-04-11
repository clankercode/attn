# MiniMax TTS Voice Research - speech-2.8-hd

## Test Results

| Voice ID | Gender | Duration (s) | Notes |
|----------|--------|--------------|-------|
| Wise_Woman | Female | 10.91 | Default voice, composed and authoritative |
| Friendly_Person | Neutral | 10.79 | Approachable and conversational |
| Deep_Voice_Man | Male | 10.27 | Rich, commanding bass |
| Calm_Woman | Female | 9.33 | Gentle and measured |
| Casual_Guy | Male | 10.40 | Relaxed and natural |
| Lively_Girl | Female | 9.21 | High-energy and bright |
| Patient_Man | Male | 12.88 | Slow, measured delivery |
| Young_Knight | Male | 8.95 | Confident and dramatic |
| Determined_Man | Male | 9.21 | Assertive tone |
| Lovely_Girl | Female | 10.15 | Pleasant, engaging |
| Decent_Boy | Male | 9.04 | Young male voice |
| Imposing_Manner | Male | 11.95 | Authoritative presence |
| Elegant_Man | Male | 8.61 | Refined and smooth |
| Abbess | Female | 10.53 | Mature female (monastic leader) |
| Sweet_Girl_2 | Female | 8.74 | Pleasant, gentle girl |
| Inspirational_girl | Female | 8.74 | Uplifting and energetic |

## Gender Breakdown

- **Female**: Wise_Woman, Calm_Woman, Lively_Girl, Lovely_Girl, Abbess, Sweet_Girl_2, Inspirational_girl
- **Male**: Deep_Voice_Man, Casual_Guy, Patient_Man, Young_Knight, Determined_Man, Decent_Boy, Imposing_Manner, Elegant_Man
- **Neutral/Unclear**: Friendly_Person

## References

### Official Documentation
- [MiniMax T2A HTTP API](https://platform.minimax.io/docs/api-reference/speech-t2a-http) - Official API docs with voice IDs
- [System Voice ID List](https://platform.minimax.io/docs/faq/system-voice-id) - Complete system voice list
- [MiniMax T2A Introduction](https://www.minimax.io/platform/document/t2a_api_intro?key=68ad77666602726333000457) - Platform overview

### Third-Party References
- [WaveSpeedAI - MiniMax Speech 2.8 HD](https://wavespeed.ai/models/minimax/speech-2.8-hd) - Interactive playground with voice list
- [Scenario.com - MiniMax Speech 2.8 Essentials](https://help.scenario.com/en/articles/minimax-speech-2-8-the-essentials) - Voice gallery and descriptions

## Voice Descriptions from Documentation

| Voice ID | Description |
|----------|-------------|
| Wise_Woman | Composed and authoritative; ideal for brand storytelling (Default) |
| Deep_Voice_Man | Rich, commanding bass; perfect for trailers and drama |
| Friendly_Person | Approachable and conversational; great for tutorials |
| Inspirational_girl | Uplifting and energetic; suited for lifestyle content |
| Calm_Woman | Gentle and measured; designed for meditation and wellness |
| Casual_Guy | Relaxed and natural; perfect for podcasts and gaming |
| Young_Knight | Confident and dramatic; ideal for fantasy NPCs |
| Elegant_Man | Refined and smooth; suited for luxury brand narration |
| Lively_Girl | High-energy and bright; great for social media |

## Test Method

API endpoint: `POST https://api.minimax.io/v1/t2a_v2`

Test text: "Hello this is a test of my voice. My name is the voice ID being tested. I speak with a distinct character that reveals my gender and personality."

Model: `speech-2.8-hd`

All 16 voice IDs returned valid audio (no API errors).
