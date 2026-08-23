# attn

A lightweight CLI tool for text-to-speech audio generation and playback with support for multiple TTS providers.

## Features

- **Multiple TTS Providers**: Support for Groq, Grok (xAI), Minimax, and MiMo APIs
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
attn --provider llmp-grok "Using Grok via the LLMP gateway"
attn --provider groq "Using Groq API"
attn --provider grok "Using Grok (xAI) TTS"
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

### History

Every generation is recorded (text, provider, voice, cwd, output path) to
`$XDG_DATA_HOME/attn/history.jsonl` (~/.local/share/attn/history.jsonl).
Older entries without `cwd` still load; the TUI shows `(not recorded)` for missing optional fields.
Browse and replay past audio in an interactive TUI:

```bash
attn history
```

- Shows the original text, provider, voice, model, cwd, and file for each entry
- **Enter/Space** plays (or stops) the cached audio, `/` filters (text, provider, voice, cwd, model, style), `d` deletes (with confirm)
- Audio cached before history existed is listed as `legacy` entries
- When stdout is not a terminal, a plain list is printed instead of the TUI
- Set `ATTN_NO_HISTORY=1` to disable recording

## Configuration

### Config file

Create `~/.config/attn/config.yaml` to set API keys, provider order, and voice policy:

```yaml
# Tried in order when --provider / TTS_PROVIDER are unset.
# If the first fails at runtime, the next is tried automatically.
provider_priority:
  - llmp-grok
  - grok
  - mimo      # Xiaomi MiMo
  - minimax

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

grok:
  # api_key optional — auto-loads from XAI_API_KEY or ~/.grok*/auth.json
  preferred: [eve, ara, leo]
  alert_voice: rex

llmp:
  # Consumer key file for the llm-api-passthrough gateway (default ~/.llmp;
  # LLMP_API_KEY / LLMP_KEY_FILE env vars also work).
  # key_file: ~/.llmp
  # base_url: "https://omni-dyn-00.amaroolabs.com/v1"   # optional; LLMP_BASE_URL also works
  preferred: [eve, ara, leo]             # same Grok voice roster as grok:
  alert_voice: rex

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
| Provider resolution | `--provider` → `TTS_PROVIDER` → first known `provider_priority` → built-in `llmp-grok`, `grok`, `mimo`, `minimax` |
| Provider fallback | When auto-selected (no `--provider` / `TTS_PROVIDER`), failed synthesis retries the rest of `provider_priority`. Explicit provider choice does not fall back. |

For Groq and MiMo (closed catalogs), preferred names that are not in the built-in list are dropped. MiniMax allows preferred IDs outside the curated subset (custom/system voices).

Explicit CLI flags always win over the config file. Check resolution without calling an API:

```bash
attn --dry-run "hello"
# [dry-run] provider=llmp-grok voice=eve → /home/you/.tts-output/....mp3
```

### Environment Variables

- `GROQ_API_KEY`: API key for Groq TTS provider
- `XAI_API_KEY` / `GROK_API_KEY`: API key for Grok (xAI) TTS (also auto-detected)
- `LLMP_API_KEY`: Consumer key for the LLMP gateway (llmp-grok provider)
- `LLMP_KEY_FILE`: path to the LLMP Consumer key file (default `~/.llmp`)
- `LLMP_BASE_URL`: LLMP gateway OpenAI-compatible base URL (default `https://omni-dyn-00.amaroolabs.com/v1`)
- `MINIMAX_API_KEY`: API key for Minimax TTS provider
- `MIMO_API_KEY`: API key for MiMo TTS provider
- `TTS_PROVIDER`: default provider (`llmp-grok`, `minimax`, `groq`, `grok`, or `mimo`)
- `GROK_TTS_LANGUAGE` / `XAI_TTS_LANGUAGE`: BCP-47 language for Grok TTS — both `grok` and `llmp-grok` (default `en`)

Environment variables override keys from the config file when both are set.

### Providers

#### Groq

1. Accept terms at: https://console.groq.com/playground?model=canopylabs%2Forpheus-v1-english
2. Set `GROQ_API_KEY` or put `api_key` under `groq:` in the config file

```bash
attn --provider groq "Test message"
```

#### Grok (xAI)

1. Create an API key at https://console.x.ai/team/default/api-keys, **or** sign in with the Grok CLI so `~/.grok/auth.json` exists
2. Credential resolution order:
   1. `XAI_API_KEY` env
   2. `GROK_API_KEY` env
   3. `api_key` under `grok:` in the config file
   4. Auto-detect OIDC token from (first hit wins):
      `~/.grok/auth.json`, `~/.grok1/auth.json`, `~/.grok2/auth.json`,
      `~/.grok-1/auth.json`, `~/.grok-2/auth.json`

```bash
attn --provider grok "Test message"
attn --provider grok --voice eve "Hello from Eve"
attn --provider grok --list-voices
```

#### LLMP Grok (llm-api-passthrough gateway)

Same Grok voices served through the llm-api-passthrough gateway
(`POST {base}/audio/speech` with an OpenAI-shaped body; the gateway routes
`grok-tts` to its Grok TTS upstream). This is the first provider in the
built-in default priority; in auto-selection mode it is tried first whenever
an LLMP credential resolves, and skipped silently when none does. The
Grok-shaped `/tts` alias is not used — the gateway rejects it with
`input must be a non-empty string`. `--model` is ignored (same as `grok`);
the gateway routes on `grok-tts`.

1. Put your Consumer key in `~/.llmp` (or set `LLMP_API_KEY`)
2. Credential resolution order:
   1. `LLMP_API_KEY` env
   2. `key_file` under `llmp:` in the config file
   3. `LLMP_KEY_FILE` env
   4. `~/.llmp`
3. Base URL resolution order: `LLMP_BASE_URL` env → `base_url` under `llmp:` →
   `https://omni-dyn-00.amaroolabs.com/v1`

```bash
attn --provider llmp-grok "Test message"
attn --provider llmp-grok --voice eve "Hello from Eve"
attn --provider llmp-grok --list-voices
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
just test-grok
just test-llmp
just test-minimax
```

## License

[Add your license here]
