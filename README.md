# attn

A lightweight CLI tool for text-to-speech audio generation and playback with support for multiple TTS providers.

## Features

- **Multiple TTS Providers**: Support for Groq and Minimax APIs
- **Local Playback**: Direct audio playback via PipeWire or system audio
- **Background Playback**: Non-blocking audio output (by default)
- **Silence Action**: Linux background playback shows a desktop notification with a Silence button
- **Alert Mode**: Generate attention-grabbing audio notifications
- **Dry Run**: Generate audio without requiring API keys (useful for testing)
- **Cross-Platform**: Works on Linux, macOS, and Windows
- **Simple CLI**: Intuitive command-line interface

## Installation

### With `go install`

```bash
go install github.com/clankercode/attn/cmd/attn@latest
go install github.com/clankercode/attn/cmd/tts@latest
```

Make sure `$(go env GOBIN)` (or `~/go/bin`) is on your `PATH`.

### Build from Source

```bash
git clone https://github.com/clankercode/attn.git
cd attn
just build
just install
```

This will build the tool and install symlinks to `~/.local/bin/attn` and `~/.local/bin/tts`.

## Usage

### Basic Text-to-Speech

```bash
attn "Hello, this is a test message"
```

### Alert Mode

Generate an attention-grabbing alert:

```bash
attn --alert "Important notification"
```

### Save to File

```bash
attn -o output.mp3 "Save this message to a file"
```

### Specify Provider

```bash
attn --provider groq "Using Groq API"
attn --provider minimax "Using Minimax API"
```

### Foreground Playback

By default, audio plays in the background. To wait for playback to complete:

```bash
attn --foreground "Wait for this to finish"
```

### Silence Background Playback

On Linux desktops with a notification daemon, background playback shows an
`attn is playing` notification. Click **Silence** to stop that audio without
affecting other `attn` commands. If desktop notifications are unavailable,
playback proceeds normally.

### Dry Run

Test without requiring API keys:

```bash
attn --dry-run "This won't call any API"
```

## Configuration

### Config file

Create `~/.config/attn/config.yaml` to set API keys, provider order, and voice policy:

```yaml
# When --provider / TTS_PROVIDER are unset, use the first known name.
provider_priority:
  - groq
  - minimax
  - mimo

# Global bans are always merged with per-provider bans.
# Prefer per-provider `preferred` lists — global preferred is only safe if
# every name is valid for every provider that inherits the list.
voices:
  banned: [troy]

groq:
  api_key: "gsk_..."
  preferred: [daniel, autumn, diana]   # random pool (not ranked priority)
  banned: [troy]                       # excluded from auto-selection
  alert_voice: daniel                  # used with --alert when --voice is unset

minimax:
  api_key: "..."
  preferred: [Deep_Voice_Man, Wise_Woman, Calm_Woman]
  alert_voice: Deep_Voice_Man

mimo:
  api_key: "..."
  # base_url: "https://..."            # optional
  preferred: [mimo_default]
  alert_voice: mimo_default
```

**Selection rules**

| Situation | Behavior |
|-----------|----------|
| `--voice NAME` | Always used (ignores preferred/banned) |
| `--alert` without `--voice` | `alert_voice` for the provider, else built-in default |
| Normal speech, no `--voice` | Uniform random from `preferred` pool minus banned; if preferred is empty, from full catalog minus banned |
| Preferred all banned / unknown | Fall back to catalog minus banned |
| Catalog entirely banned | Built-in alert default voice (never re-enables banned names in the pool) |
| Provider resolution | `--provider` → `TTS_PROVIDER` → first known `provider_priority` → `minimax` |

For Groq and MiMo (closed catalogs), preferred names that are not in the built-in list are dropped. MiniMax allows preferred IDs outside the curated subset (custom/system voices).

Explicit CLI flags always win over the config file. Check resolution without calling an API:

```bash
attn --dry-run "hello"
# [dry-run] provider=groq voice=autumn → /home/you/.tts-output/....wav
```

### Environment Variables

- `GROQ_API_KEY`: API key for Groq TTS provider
- `MINIMAX_API_KEY`: API key for Minimax TTS provider
- `MIMO_API_KEY`: API key for MiMo TTS provider
- `TTS_PROVIDER`: default provider (`minimax`, `groq`, or `mimo`)

Environment variables override keys from the config file when both are set.

### Providers

#### Groq

1. Accept terms at: https://console.groq.com/playground?model=canopylabs%2Forpheus-v1-english
2. Set `GROQ_API_KEY` or put `api_key` under `groq:` in the config file

```bash
attn --provider groq "Test message"
```

#### Minimax

1. Obtain API key with TTS access from Minimax
2. Set `MINIMAX_API_KEY` or put `api_key` under `minimax:` in the config file

```bash
attn --provider minimax "Test message"
```

## Help

```bash
attn --help
```

## Development

### Build

```bash
just build
```

### Test

```bash
just test
```

### Test with Providers

```bash
just test-groq
just test-minimax
```

## License

[Add your license here]
