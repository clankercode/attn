# attn-tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Single Go binary `attn-tool` symlinked as `attn` and `tts` that speaks text via MiniMax or Groq TTS, plays audio with mpv, and saves to `~/.tts-output/`.

**Architecture:** Three-layer design — CLI layer (flag parsing), TTS provider layer (MiniMax + Groq), audio layer (mpv playback + alert tone + file saving). Provider selected via `TTS_PROVIDER` env var with `--provider` flag override.

**Tech Stack:** Go 1.21+, `mpv`, `minimax-go` (or direct HTTP), Groq OpenAI-compatible API.

---

## File Structure

```
attn-tool/
├── go.mod
├── cmd/
│   ├── attn/
│   │   └── main.go        # calls lib.Run(os.Args[1:])
│   └── tts/
│       └── main.go        # calls lib.Run(os.Args[1:])
├── internal/
│   ├── lib.go             # entry point: Run([]string) — wires everything
│   ├── cli/
│   │   └── flags.go       # flag parsing into Config struct
│   ├── tts/
│   │   ├── provider.go    # Provider interface + ProviderType enum
│   │   ├── minimax.go     # MiniMax TTS implementation
│   │   └── groq.go        # Groq TTS implementation
│   └── audio/
│       ├── player.go      # mpv playback + file saving
│       └── alert.go       # alert tone generation (WAV bytes)
└── alert_tone.go          # embed alert WAV data
```

---

## Task 1: Project Skeleton + Go Module

**Files:**
- Create: `go.mod`
- Create: `cmd/attn/main.go`
- Create: `cmd/tts/main.go`
- Create: `internal/lib.go`

- [ ] **Step 1: Initialize Go module**

```bash
go mod init attn-tool
```

- [ ] **Step 2: Create cmd/attn/main.go**

```go
package main

import "attn-tool/internal"

func main() {
    internal.Run(os.Args[1:])
}
```

- [ ] **Step 3: Create cmd/tts/main.go** (identical)

```go
package main

import "attn-tool/internal"

func main() {
    internal.Run(os.Args[1:])
}
```

- [ ] **Step 4: Create internal/lib.go**

```go
package internal

import (
    "os"
    "attn-tool/internal/cli"
)

func Run(args []string) {
    cfg := cli.Parse(args)
    // TODO: implement
}
```

- [ ] **Step 5: Commit**

```bash
git add go.mod cmd/ internal/
git commit -m "init: project skeleton"
```

---

## Task 2: Config + Flag Parsing

**Files:**
- Create: `internal/cli/flags.go`
- Modify: `internal/lib.go`

- [ ] **Step 1: Write cli/flags.go**

```go
package cli

import (
    "flag"
    "os"
    "time"
)

type Config struct {
    Text     string
    Output   string
    Provider string
    Voice    string
    Model    string
    Alert    bool
}

func Parse(args []string) Config {
    fs := flag.NewFlagSet("attn-tool", flag.ContinueOnError)
    fs.Usage = func() {
        println("Usage: attn [options] \"message\"")
        fs.PrintDefaults()
    }

    var (
        output   = fs.String("o", "", "Output file path (default: ~/.tts-output/<timestamp>.mp3)")
        provider = fs.String("provider", os.Getenv("TTS_PROVIDER"), "Provider: minimax or groq")
        voice    = fs.String("voice", "", "Voice ID")
        model    = fs.String("model", "", "Model ID (provider-specific)")
        alert    = fs.Bool("alert", false, "Prepend alert tone and use alert voice")
    )

    fs.Parse(args)

    text := ""
    if args := fs.Args(); len(args) > 0 {
        text = args[0]
    }

    outPath := *output
    if outPath == "" {
        ts := time.Now().Unix()
        home, _ := os.UserHomeDir()
        outPath = home + "/.tts-output/" + fmt.Sprintf("%d", ts) + ".mp3"
    }

    providerVal := *provider
    if providerVal == "" {
        providerVal = "minimax"
    }

    return Config{
        Text:     text,
        Output:   outPath,
        Provider: providerVal,
        Voice:    *voice,
        Model:    *model,
        Alert:    *alert,
    }
}
```

- [ ] **Step 2: Verify it compiles (stub Run)**

Run: `go build ./...`

- [ ] **Step 3: Commit**

```bash
git add internal/cli/flags.go internal/lib.go
git commit -m "feat: add flag parsing with Config struct"
```

---

## Task 3: TTS Provider Interface

**Files:**
- Create: `internal/tts/provider.go`
- Create: `internal/tts/minimax.go`
- Create: `internal/tts/groq.go`

**Provider interface:**

```go
type AudioOutput struct {
    Data []byte // MP3 bytes
}

type Provider interface {
    Name() string
    Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error)
}
```

**ProviderType enum:**
```go
type ProviderType string
const (
    ProviderMinimax ProviderType = "minimax"
    ProviderGroq    ProviderType = "groq"
)
```

**NewProvider func:**
```go
func NewProvider(t ProviderType, voice, model string) Provider {
    switch t {
    case ProviderGroq:
        return newGroq(voice, model)
    default:
        return newMinimax(voice, model)
    }
}
```

- [ ] **Step 1: Write internal/tts/provider.go**

```go
package tts

import "context"

type AudioOutput struct {
    Data []byte
}

type Provider interface {
    Name() string
    Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error)
}

type ProviderType string

const (
    ProviderMinimax ProviderType = "minimax"
    ProviderGroq    ProviderType = "groq"
)

func NewProvider(t ProviderType, voice, model string) Provider {
    switch t {
    case ProviderGroq:
        return newGroq(voice, model)
    default:
        return newMinimax(voice, model)
    }
}
```

- [ ] **Step 2: Write internal/tts/minimax.go**

```go
package tts

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "os"
)

type minimaxProvider struct {
    voice string
    model string
}

func newMinimax(voice, model string) Provider {
    if voice == "" {
        voice = "female-shaonv"
    }
    if model == "" {
        model = "speech-2.8-turbo"
    }
    return &minimaxProvider{voice: voice, model: model}
}

func (m *minimaxProvider) Name() string { return "minimax" }

func (m *minimaxProvider) Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error) {
    apiKey := os.Getenv("MINIMAX_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("MINIMAX_API_KEY not set")
    }

    if voice == "" {
        voice = m.voice
    }
    if model == "" {
        model = m.model
    }

    url := "https://api.minimax.io/v1/t2a_v2"
    payload := map[string]any{
        "model": model,
        "text":  text,
        "voice_setting": map[string]any{
            "voice_id": voice,
            "speed":    1.0,
            "vol":      1.0,
        },
    }
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("minimax API error: %s", resp.Status)
    }

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return &AudioOutput{Data: data}, nil
}
```

- [ ] **Step 3: Write internal/tts/groq.go**

```go
package tts

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
)

type groqProvider struct {
    voice string
    model string
}

func newGroq(voice, model string) Provider {
    if voice == "" {
        voice = "Arista-PlayAI"
    }
    if model == "" {
        model = "playai-tts"
    }
    return &groqProvider{voice: voice, model: model}
}

func (g *groqProvider) Name() string { return "groq" }

func (g *groqProvider) Synthesize(ctx context.Context, text, voice, model string) (*AudioOutput, error) {
    apiKey := os.Getenv("GROQ_API_KEY")
    if apiKey == "" {
        return nil, fmt.Errorf("GROQ_API_KEY not set")
    }

    if voice == "" {
        voice = g.voice
    }
    if model == "" {
        model = g.model
    }

    url := "https://api.groq.com/openai/v1/audio/speech"
    payload := map[string]any{
        "model": model,
        "voice": voice,
        "input": text,
    }
    body, _ := json.Marshal(payload)
    req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("groq API error: %s — %s", resp.Status, string(bodyBytes))
    }

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    return &AudioOutput{Data: data}, nil
}
```

- [ ] **Step 4: Verify compilation**

Run: `go build ./internal/tts/`

- [ ] **Step 5: Commit**

```bash
git add internal/tts/
git commit -m "feat: add TTS provider interface with MiniMax and Groq implementations"
```

---

## Task 4: Audio Layer (Alert Tone + mpv + File Saving)

**Files:**
- Create: `internal/audio/alert.go`
- Create: `internal/audio/player.go`
- Create: `alert_tone_data.go` (embedded WAV bytes)

**Alert tone:** A ~0.3s WAV beep at 800Hz. Embed as `//go:embed alert_tone.wav` or inline base64-decoded bytes in `alert_tone_data.go`.

- [ ] **Step 1: Generate a short WAV alert tone**

Use Python or a shell tool to create a 0.3s 800Hz sine wave WAV file:

```bash
python3 -c "
import struct, math
freq, dur, sr = 800, 0.3, 44100
samples = int(sr * dur)
wav = struct.pack('<4sI4sIHHIIHHII4sI',
    b'RIFF', 36 + samples * 2, b'WAVE',
    b'fmt ', 16, 1, 1, sr, sr*2, 2, 16,
    b'data', samples * 2)
for i in range(samples):
    t = i / sr
    env = min(1.0, min(t / 0.01, (dur - t) / 0.01)) # attack/release
    s = int(16000 * env * math.sin(2 * math.pi * freq * t))
    wav += struct.pack('<h', max(-32768, min(32767, s)))
with open('alert_tone.wav', 'wb') as f:
    f.write(wav)
"
```

This creates `alert_tone.wav`.

- [ ] **Step 2: Write internal/audio/alert.go**

```go
package audio

import _ "embed"

//go:embed alert_tone.wav
var alertTone []byte

func AlertTone() []byte {
    return alertTone
}
```

- [ ] **Step 3: Write internal/audio/player.go**

```go
package audio

import (
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
)

func PlayAndSave(data []byte, outputPath string, doPlay bool) error {
    if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
        return fmt.Errorf("create output dir: %w", err)
    }

    if err := os.WriteFile(outputPath, data, 0644); err != nil {
        return fmt.Errorf("write file: %w", err)
    }

    if doPlay {
        return plaympv(outputPath)
    }
    return nil
}

func plaympv(path string) error {
    cmd := exec.Command("mpv", "--quiet", "--no-terminal", path)
    cmd.Stdout = io.Discard
    cmd.Stderr = io.Discard
    return cmd.Run()
}

func ConcatWAV(parts ...[]byte) ([]byte, error) {
    // Byte-concatenate MP3 frames. This works reliably with mpv
    // even without re-encoding, since mpv's MP3 demuxer handles
    // concatenated frames gracefully.
    if len(parts) == 1 {
        return parts[0], nil
    }
    var out []byte
    for _, p := range parts {
        out = append(out, p...)
    }
    return out, nil
}
```

- [ ] **Step 4: Write alert_tone_data.go**

```go
package main

import _ "embed"

//go:embed alert_tone.wav
var alertTone []byte

func AlertTone() []byte {
    return alertTone
}
```

Place `alert_tone.wav` at project root alongside this file.

- [ ] **Step 5: Commit**

```bash
git add alert_tone.wav internal/audio/
git commit -m "feat: add audio layer with alert tone and mpv playback"
```

---

## Task 5: Wire It All Together in lib.go

**Files:**
- Modify: `internal/lib.go`

- [ ] **Step 1: Write full internal/lib.go**

```go
package internal

import (
    "context"
    "fmt"
    "os"
    "os/exec"

    "attn-tool/internal/audio"
    "attn-tool/internal/cli"
    "attn-tool/internal/tts"
)

func Run(args []string) {
    cfg := cli.Parse(args)

    if cfg.Text == "" {
        fmt.Fprintln(os.Stderr, "error: no text provided")
        os.Exit(1)
    }

    providerType := tts.ProviderType(cfg.Provider)
    provider := tts.NewProvider(providerType, cfg.Voice, cfg.Model)

    voice := cfg.Voice
    if voice == "" {
        voice = defaultVoice(providerType, cfg.Alert)
    }

    ctx := context.Background()
    audioOut, err := provider.Synthesize(ctx, cfg.Text, voice, cfg.Model)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }

    finalAudio := audioOut.Data

    if cfg.Alert {
        alertTone := audio.AlertTone()
        finalAudio, _ = audio.ConcatWAV(alertTone, finalAudio)
    }

    doPlay := cfg.Output == "" || !isFileOutput(cfg.Output)
    if err := audio.PlayAndSave(finalAudio, cfg.Output, doPlay); err != nil {
        fmt.Fprintf(os.Stderr, "error: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("Saved to %s\n", cfg.Output)
}

func defaultVoice(pt tts.ProviderType, alert bool) string {
    if alert {
        switch pt {
        case tts.ProviderGroq:
            return "Arista-PlayAI"
        default:
            return "male-tianmei"
        }
    }
    switch pt {
    case tts.ProviderGroq:
        return "Arista-PlayAI"
    default:
        return "female-shaonv"
    }
}

func isFileOutput(path string) bool {
    cmd := exec.Command("mpv", "--quiet", "--no-terminal", "--pause", path)
    cmd.Stdout = io.Discard
    cmd.Stderr = io.Discard
    return cmd.Run() == nil
}
```

Fix imports (add missing `io`).

- [ ] **Step 2: Verify full build**

Run: `go build -o attn-tool ./cmd/attn && go build -o attn-tool ./cmd/tts`

- [ ] **Step 3: Commit**

```bash
git commit -m "feat: wire TTS providers, audio layer, and CLI together"
```

---

## Task 6: Install Targets + Symlinks

**Files:**
- Create: `Makefile` or shell script for installation

- [ ] **Step 1: Write Makefile**

```makefile
.PHONY: install build

install: build
	install -d $(DESTDIR)/usr/local/bin
	ln -sf $(PWD)/attn-tool $(DESTDIR)/usr/local/bin/attn
	ln -sf $(PWD)/attn-tool $(DESTDIR)/usr/local/bin/tts

build:
	go build -o attn-tool ./cmd/attn
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "build: add Makefile with install targets"
```

---

## Task 7: Basic Smoke Test

- [ ] **Step 1: If MINIMAX_API_KEY is set, test minimax path**

```bash
MINIMAX_API_KEY=... ./attn-tool "test" -o /tmp/test_out.mp3
# verify /tmp/test_out.mp3 exists and has content
```

- [ ] **Step 2: If GROQ_API_KEY is set, test groq path**

```bash
GROQ_API_KEY=... ./attn-tool "test" --provider groq -o /tmp/test_out_groq.mp3
```

- [ ] **Step 3: Test alert mode**

```bash
./attn-tool "test alert" --alert -o /tmp/test_alert.mp3
```

---

## Spec Coverage Check

| Spec Requirement | Task |
|---|---|
| Two symlinks, one binary | Task 6 |
| `attn "msg"` and `tts "msg"` CLI | Task 1, 2 |
| MiniMax + Groq providers | Task 3 |
| `TTS_PROVIDER` env + `--provider` flag | Task 2 |
| `--voice`, `--model`, `--alert` flags | Task 2 |
| Play via mpv | Task 4 |
| Save to `~/.tts-output/<ts>.mp3` default | Task 2, 4 |
| `-o` override output path | Task 2, 4 |
| `--alert` prepends tone + alert voice | Task 4, 5 |
