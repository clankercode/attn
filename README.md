# attn

A lightweight CLI tool for text-to-speech audio generation and playback with support for multiple TTS providers.

## Features

- **Multiple TTS Providers**: Support for Groq and Minimax APIs
- **Local Playback**: Direct audio playback via PipeWire or system audio
- **Background Playback**: Non-blocking audio output (by default)
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

### Dry Run

Test without requiring API keys:

```bash
attn --dry-run "This won't call any API"
```

## Configuration

### Environment Variables

- `GROQ_API_KEY`: API key for Groq TTS provider
- `MINIMAX_API_KEY`: API key for Minimax TTS provider

### Providers

#### Groq

1. Accept terms at: https://console.groq.com/playground?model=canopylabs%2Forpheus-v1-english
2. Set `GROQ_API_KEY` environment variable

```bash
attn --provider groq "Test message"
```

#### Minimax

1. Obtain API key with TTS access from Minimax
2. Set `MINIMAX_API_KEY` environment variable

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
