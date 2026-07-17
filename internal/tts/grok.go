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
	"time"
)

// Grok (xAI) TTS — https://api.x.ai/v1/tts
// Auth: XAI_API_KEY / GROK_API_KEY, config, or OIDC JWT from Grok CLI auth.json.
// Profile tokens are read in-memory only (not injected into the process environment).

type grokProvider struct {
	voice string
}

func newGrok(voice, _ string) Provider {
	if voice == "" {
		voice = "eve"
	}
	return &grokProvider{voice: voice}
}

func (g *grokProvider) Name() string { return "grok" }

func (g *grokProvider) Synthesize(ctx context.Context, text, voice, _ string) (*AudioOutput, error) {
	apiKey, keySource := ResolveGrokAPIKeyWithSource()
	if apiKey == "" {
		return nil, fmt.Errorf("no Grok/xAI credential: set XAI_API_KEY or GROK_API_KEY, put api_key under grok: in ~/.config/attn/config.yaml, or sign in so ~/.grok/auth.json exists (also checks ~/.grok{1,2} and ~/.grok-{1,2})")
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

	url := "https://api.x.ai/v1/tts"
	payload := map[string]any{
		"text":     text,
		"voice_id": voice,
		"language": language,
		"output_format": map[string]any{
			"codec":       "mp3",
			"sample_rate": 24000,
			"bit_rate":    128000,
		},
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
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		msg := truncateErrBody(string(bodyBytes))
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("grok/xAI unauthorized (401) via %s — re-run Grok CLI login or set a fresh XAI_API_KEY: %s", keySource, msg)
		}
		return nil, fmt.Errorf("grok/xAI API error: %s — %s", resp.Status, msg)
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("grok/xAI API returned JSON (likely error): %s", truncateErrBody(string(bodyBytes)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &AudioOutput{Data: data}, nil
}

func truncateErrBody(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

// ResolveGrokAPIKey returns the first usable xAI/Grok credential.
// Order: XAI_API_KEY → GROK_API_KEY → ~/.grok*/auth.json scan.
// Does not mutate the process environment.
func ResolveGrokAPIKey() string {
	k, _ := ResolveGrokAPIKeyWithSource()
	return k
}

// ResolveGrokAPIKeyWithSource is like ResolveGrokAPIKey but also returns a short
// label describing where the key came from (for error messages only).
func ResolveGrokAPIKeyWithSource() (key, source string) {
	if k := strings.TrimSpace(os.Getenv("XAI_API_KEY")); k != "" {
		return k, "XAI_API_KEY"
	}
	if k := strings.TrimSpace(os.Getenv("GROK_API_KEY")); k != "" {
		return k, "GROK_API_KEY"
	}
	k, src := loadGrokKeyFromHomeDirs(os.UserHomeDir)
	return k, src
}

// grokAuthDirs is the ordered list of profile dirs under $HOME to scan for auth.json.
// Includes both ~/.grok{1,2} and the hyphenated ~/.grok-{1,2} layouts.
var grokAuthDirs = []string{
	".grok",
	".grok1",
	".grok2",
	".grok-1",
	".grok-2",
}

// loadGrokKeyFromHomeDirs scans Grok CLI profile dirs for an OIDC access token
// (or API key) stored in auth.json. homeFn is injectable for tests.
// Prefer a non-expired token across all profiles; if every known token is past
// expires_at, still return the first found key (CLI expires_at can lag actual JWT
// validity) with source labelled as profile path.
func loadGrokKeyFromHomeDirs(homeFn func() (string, error)) (key, source string) {
	home, err := homeFn()
	if err != nil || home == "" {
		return "", ""
	}
	var fallback, fallbackSrc string
	now := time.Now()
	for _, dir := range grokAuthDirs {
		path := filepath.Join(home, dir, "auth.json")
		k, expiresOK := readGrokAuthJSON(path, now)
		if k == "" {
			continue
		}
		src := "~/" + dir + "/auth.json"
		if expiresOK {
			return k, src
		}
		if fallback == "" {
			fallback = k
			fallbackSrc = src + " (expires_at past; may still work)"
		}
	}
	return fallback, fallbackSrc
}

// readGrokAuthJSON parses a Grok CLI auth.json file.
// Returns the key and whether it is not known-expired.
// Entries without a parseable expires_at are treated as non-expired.
func readGrokAuthJSON(path string, now time.Time) (key string, notExpired bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", false
	}

	type entry struct {
		Key       string `json:"key"`
		ExpiresAt string `json:"expires_at"`
	}

	var bestKey string
	var bestOK bool
	for _, rawEntry := range raw {
		var e entry
		if err := json.Unmarshal(rawEntry, &e); err != nil {
			continue
		}
		k := strings.TrimSpace(e.Key)
		if k == "" {
			continue
		}
		ok := true
		if e.ExpiresAt != "" {
			if exp, err := time.Parse(time.RFC3339Nano, e.ExpiresAt); err == nil {
				ok = now.Before(exp)
			} else if exp, err := time.Parse(time.RFC3339, e.ExpiresAt); err == nil {
				ok = now.Before(exp)
			}
		}
		if ok {
			return k, true
		}
		if bestKey == "" {
			bestKey = k
			bestOK = false
		}
	}

	// Also accept a top-level {"key":"..."} style if nested entries had none.
	if bestKey == "" {
		var flat struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(data, &flat); err == nil {
			bestKey = strings.TrimSpace(flat.Key)
			bestOK = bestKey != "" // no expires_at → treat as usable
		}
	}
	return bestKey, bestOK
}
