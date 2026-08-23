package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// LLMP Grok TTS — POST {base}/audio/speech on the llm-api-passthrough
// gateway (OpenAI-shaped body: model/input/voice/response_format).
// The gateway routes "grok-tts" to its Grok TTS upstream.
// Auth: LLMP_API_KEY, config llmp.key_file, or LLMP_KEY_FILE / ~/.llmp key file.
// Key files are read in-memory only (not injected into the process environment).

// DefaultLLMPBaseURL is the canonical public origin of the gateway.
const DefaultLLMPBaseURL = "https://omni-dyn-00.amaroolabs.com/v1"

type llmpGrokProvider struct {
	voice string
}

func newLlmpGrok(voice, _ string) Provider {
	if voice == "" {
		voice = "eve"
	}
	return &llmpGrokProvider{voice: voice}
}

func (g *llmpGrokProvider) Name() string { return "llmp-grok" }

func (g *llmpGrokProvider) Synthesize(ctx context.Context, text, voice, _ string) (*AudioOutput, error) {
	apiKey, keySource := ResolveLLMPAPIKeyWithSource()
	if apiKey == "" {
		return nil, fmt.Errorf("no LLMP credential: set LLMP_API_KEY, put key_file under llmp: in ~/.config/attn/config.yaml, or create ~/.llmp (override path with LLMP_KEY_FILE)")
	}

	if voice == "" {
		voice = g.voice
	}
	// Grok voice IDs are case-insensitive; normalize for stable request bodies.
	voice = strings.ToLower(strings.TrimSpace(voice))

	language := os.Getenv("GROK_TTS_LANGUAGE")
	if language == "" {
		language = os.Getenv("XAI_TTS_LANGUAGE")
	}
	if language == "" {
		language = "en"
	}

	base := ResolveLLMPBaseURL()
	url := strings.TrimRight(base, "/") + "/audio/speech"
	payload := map[string]any{
		"model":           "grok-tts",
		"input":           text,
		"voice":           voice,
		"language":        language,
		"response_format": "mp3",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llmp-grok request failed (%s): %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := truncateErrBody(string(bodyBytes))
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("llmp-grok unauthorized (401) via %s — check the Consumer key: %s", keySource, msg)
		}
		return nil, fmt.Errorf("llmp-grok API error: %s — %s", resp.Status, msg)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("llmp-grok API returned JSON (likely error): %s", truncateErrBody(string(bodyBytes)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &AudioOutput{Data: data}, nil
}

// LLMPConfig carries the llmp: stanza from the attn config file so the tts
// package can resolve credentials without importing cli (which would be an
// import cycle). Set via SetLLMPConfig during config load.
type LLMPConfig struct {
	KeyFile string
	BaseURL string
}

var llmpConfig LLMPConfig

// SetLLMPConfig records the config-file llmp stanza for credential/base
// resolution. Called once from cli.LoadConfig.
func SetLLMPConfig(c LLMPConfig) { llmpConfig = c }

// ResolveLLMPBaseURL returns the gateway OpenAI-compatible base URL.
// Order: LLMP_BASE_URL → config llmp.base_url → DefaultLLMPBaseURL.
func ResolveLLMPBaseURL() string {
	if b := strings.TrimSpace(os.Getenv("LLMP_BASE_URL")); b != "" {
		return b
	}
	if b := strings.TrimSpace(llmpConfig.BaseURL); b != "" {
		return b
	}
	return DefaultLLMPBaseURL
}

// ResolveLLMPAPIKey returns the first usable LLMP Consumer key.
// Order: LLMP_API_KEY → config llmp.key_file → LLMP_KEY_FILE → ~/.llmp.
// Does not mutate the process environment.
func ResolveLLMPAPIKey() string {
	k, _ := ResolveLLMPAPIKeyWithSource()
	return k
}

// ResolveLLMPAPIKeyWithSource is like ResolveLLMPAPIKey but also returns a
// short label describing where the key came from (for error messages only).
func ResolveLLMPAPIKeyWithSource() (key, source string) {
	if k := strings.TrimSpace(os.Getenv("LLMP_API_KEY")); k != "" {
		return k, "LLMP_API_KEY"
	}
	path := llmpKeyFilePath()
	if path == "" {
		return "", ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", ""
	}
	k := strings.TrimSpace(string(data))
	if k == "" {
		return "", ""
	}
	return k, path
}

// llmpKeyFilePath resolves which key file to read.
// Order: config llmp.key_file → LLMP_KEY_FILE → ~/.llmp.
func llmpKeyFilePath() string {
	if p := expandHome(strings.TrimSpace(llmpConfig.KeyFile)); p != "" {
		return p
	}
	if p := expandHome(strings.TrimSpace(os.Getenv("LLMP_KEY_FILE"))); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, ".llmp")
}

// expandHome expands a leading ~/ (or bare ~) to the user's home directory.
func expandHome(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
