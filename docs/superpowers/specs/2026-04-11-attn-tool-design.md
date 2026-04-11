# attn-tool: LLM Attention-Alert CLI

## Overview

A single Go binary that speaks text aloud via TTS and saves audio to disk. Two symlinks (`attn` and `tts`) point to the same binary, which behaves identically regardless of which name invoked it.

Primary use case: LLMs alert their human operator when they need attention — e.g., `attn "I'm done with X"`.

## Architecture

```
attn-tool (binary)
├── cmd/attn/main.go    # symlink → calls lib
├── cmd/tts/main.go     # symlink → calls lib
└── internal/
    ├── tts.go          # provider interface + MiniMax + Groq implementations
    ├── audio.go        # mpv playback + alert sound prepending
    └── cli.go          # flag parsing + save path logic
```

## CLI Surface

```
attn "message"                           # speak + save to ~/.tts-output/
tts "Hello world"                        # speak + save to ~/.tts-output/
attn "msg" -o custom.mp3                 # save to custom path, no playback
attn "msg" --provider groq               # use Groq instead of default provider
attn "msg" --provider minimax --voice female-shaonv
attn "msg" --alert                       # prepend alert tone + use alert voice
attn "msg" --model speech-2.8-hd        # MiniMax HD model
attn "msg" --voice Arista-PlayAI         # Groq voice
```

### Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-o, --output <path>` | Output file path | `~/.tts-output/<unix-timestamp>.mp3` |
| `--provider <name>` | TTS provider (`minimax` or `groq`) | `TTS_PROVIDER` env var, else `minimax` |
| `--voice <id>` | Voice ID for the provider | Provider default |
| `--model <id>` | Model ID (MiniMax only) | `speech-2.8-turbo` |
| `--alert` | Prepend alert tone and use alert voice | `false` |
| `-h, --help` | Help | |
| `--version` | Version | |

## Behavior

1. Resolve provider from `TTS_PROVIDER` env var or `--provider` flag
2. Resolve API key from `MINIMAX_API_KEY` or `GROQ_API_KEY` (per provider)
3. Call TTS API with text, model, and voice settings
4. If `--alert`: prepend a short alert beep/tone to the audio
5. Pipe audio to `mpv --quiet -` for playback (unless `-o` is given without playback intent)
6. Save to `~/.tts-output/<unix-timestamp>.mp3` or the path specified by `-o`

### Alert Mode

When `--alert` is passed:
- A short attention-getting tone is prepended to the audio
- Default "alert voice" is used unless `--voice` overrides it

### Save Behavior

- Default save path: `~/.tts-output/<unix-timestamp>.mp3`
- If `-o` is given: save to specified path, no playback
- If `-o` is NOT given: playback via mpv, then save

## Providers

### MiniMax

- Endpoint: `https://api.minimax.io/v1/t2a_v2` (sync) or WebSocket for streaming
- API key env: `MINIMAX_API_KEY`
- Models: `speech-2.8-turbo` (default), `speech-2.8-hd`, `speech-2.6-turbo`, `speech-2.6-hd`
- 300+ voice IDs, 40 languages
- Text max: 10,000 chars (sync), 50,000 (async)

### Groq

- Endpoint: `https://api.groq.com/openai/v1/audio/speech` (OpenAI-compatible)
- API key env: `GROQ_API_KEY`
- Models: `playai-tts` (default), `canopylabs/orpheus-v1-english`
- Voices: multiple PlayAI voices (Arista, Fritz, etc.)
- Text max: ~10,000 chars recommended

## Dependencies

- Go 1.21+
- `mpv` in PATH for audio playback
- `MINIMAX_API_KEY` and/or `GROQ_API_KEY` in environment

## Installation

After build:
```bash
ln -s attn-tool /usr/local/bin/attn
ln -s attn-tool /usr/local/bin/tts
```

Or via Homebrew (future):
```bash
brew install attn-tool
```
