package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/clankercode/attn/internal/tts"
)

func TestParseReturnsErrorForInvalidFlags(t *testing.T) {
	ResetConfigForTest(filepath.Join(t.TempDir(), "missing.yaml"))
	t.Cleanup(func() { ResetConfigForTest("") })

	_, err := Parse([]string{"--definitely-not-a-real-flag", "hello"})
	if err == nil {
		t.Fatal("expected invalid flag to return an error")
	}
}

func TestParseGeneratesUniqueDefaultOutputs(t *testing.T) {
	// Isolate from the developer's real config.
	ResetConfigForTest(filepath.Join(t.TempDir(), "missing.yaml"))
	t.Cleanup(func() { ResetConfigForTest("") })
	t.Setenv("TTS_PROVIDER", "")

	first, err := Parse([]string{"hello"})
	if err != nil {
		t.Fatalf("first parse failed: %v", err)
	}
	second, err := Parse([]string{"hello"})
	if err != nil {
		t.Fatalf("second parse failed: %v", err)
	}
	if first.Output == second.Output {
		t.Fatalf("expected unique output paths, got %q", first.Output)
	}
	if filepath.Ext(first.Output) != ".mp3" {
		t.Fatalf("expected default llmp-grok output to be .mp3, got %q", first.Output)
	}
	if first.Provider != "llmp-grok" {
		t.Fatalf("expected default provider llmp-grok, got %q", first.Provider)
	}
}

func TestParseUsesProviderPriorityFromConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
provider_priority:
  - groq
  - minimax
groq:
  preferred: [autumn, diana]
  banned: [troy]
  alert_voice: hannah
voices:
  banned: [austin]
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ResetConfigForTest(path)
	t.Cleanup(func() { ResetConfigForTest("") })
	t.Setenv("TTS_PROVIDER", "")

	cfg, err := Parse([]string{"hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Provider != "groq" {
		t.Fatalf("expected provider from priority list, got %q", cfg.Provider)
	}
	if cfg.ProviderExplicit {
		t.Fatal("auto-selected provider should not be marked explicit")
	}
	if len(cfg.ProviderCandidates) != 2 || cfg.ProviderCandidates[0] != "groq" || cfg.ProviderCandidates[1] != "minimax" {
		t.Fatalf("expected candidates [groq minimax], got %v", cfg.ProviderCandidates)
	}

	// TTS_PROVIDER beats provider_priority.
	t.Setenv("TTS_PROVIDER", "minimax")
	ResetConfigForTest(path) // reload same file after cache clear for clean env interaction
	cfg, err = Parse([]string{"hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Provider != "minimax" {
		t.Fatalf("TTS_PROVIDER should beat provider_priority, got %q", cfg.Provider)
	}
	if !cfg.ProviderExplicit {
		t.Fatal("TTS_PROVIDER should mark provider as explicit")
	}
	if len(cfg.ProviderCandidates) != 1 || cfg.ProviderCandidates[0] != "minimax" {
		t.Fatalf("explicit should be single candidate, got %v", cfg.ProviderCandidates)
	}
	t.Setenv("TTS_PROVIDER", "")
	ResetConfigForTest(path)

	cfg, err = Parse([]string{"hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Provider != "groq" {
		t.Fatalf("expected provider from priority list after clearing env, got %q", cfg.Provider)
	}
	if cfg.VoicePrefs.Alert != "hannah" {
		t.Fatalf("expected alert_voice hannah, got %q", cfg.VoicePrefs.Alert)
	}
	if len(cfg.VoicePrefs.Preferred) != 2 || cfg.VoicePrefs.Preferred[0] != "autumn" {
		t.Fatalf("unexpected preferred: %v", cfg.VoicePrefs.Preferred)
	}
	// global + provider banned merged
	banned := map[string]bool{}
	for _, b := range cfg.VoicePrefs.Banned {
		banned[b] = true
	}
	if !banned["troy"] || !banned["austin"] {
		t.Fatalf("expected merged bans troy+austin, got %v", cfg.VoicePrefs.Banned)
	}

	// Explicit flag still wins over priority.
	cfg, err = Parse([]string{"--provider", "minimax", "hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Provider != "minimax" {
		t.Fatalf("expected explicit provider, got %q", cfg.Provider)
	}
}

func TestVoicePrefsForPrefersProviderOverGlobal(t *testing.T) {
	cfg := &ConfigFile{
		Voices: GlobalVoices{Preferred: []string{"global-only"}, Banned: []string{"gban"}},
		Groq: ProviderConfig{
			Preferred:  []string{"daniel"},
			Banned:     []string{"troy"},
			AlertVoice: "hannah",
		},
	}
	prefs := VoicePrefsFor(cfg, tts.ProviderGroq)
	if len(prefs.Preferred) != 1 || prefs.Preferred[0] != "daniel" {
		t.Fatalf("provider preferred should override global, got %v", prefs.Preferred)
	}
	if prefs.Alert != "hannah" {
		t.Fatalf("alert: %q", prefs.Alert)
	}

	// Minimax has no preferred → falls back to global preferred.
	prefs = VoicePrefsFor(cfg, tts.ProviderMinimax)
	if len(prefs.Preferred) != 1 || prefs.Preferred[0] != "global-only" {
		t.Fatalf("expected global preferred for minimax, got %v", prefs.Preferred)
	}
	if len(prefs.Banned) != 1 || prefs.Banned[0] != "gban" {
		t.Fatalf("expected global ban only, got %v", prefs.Banned)
	}
}

func TestParseGrokProviderFromPriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := `
provider_priority:
  - grok
  - minimax
grok:
  preferred: [eve, ara]
  alert_voice: rex
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ResetConfigForTest(path)
	t.Cleanup(func() { ResetConfigForTest("") })
	t.Setenv("TTS_PROVIDER", "")

	cfg, err := Parse([]string{"hello"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.Provider != "grok" {
		t.Fatalf("expected grok from priority, got %q", cfg.Provider)
	}
	if cfg.VoicePrefs.Alert != "rex" {
		t.Fatalf("expected alert rex, got %q", cfg.VoicePrefs.Alert)
	}
	if len(cfg.VoicePrefs.Preferred) != 2 {
		t.Fatalf("preferred: %v", cfg.VoicePrefs.Preferred)
	}
	if filepath.Ext(cfg.Output) != ".mp3" {
		t.Fatalf("expected .mp3 output for grok, got %q", cfg.Output)
	}
}

func TestWriteHelpIncludesExamplesAndDefaults(t *testing.T) {
	var buf bytes.Buffer
	writeHelp(&buf)
	out := buf.String()

	checks := []string{
		"attn speaks text and saves the generated audio.",
		"Examples:",
		"attn \"Build finished.\"",
		"attn --wait \"test two.\"",
		"attn --provider groq --voice daniel \"Heads up.\"",
		"attn --provider grok --voice eve",
		"Common flags:",
		"--alert",
		"--model",
		"--silent",
		"--debug-play-file",
		"Defaults:",
		"TTS_PROVIDER",
		"provider_priority",
		"voice: random from preferred pool",
		"banned",
		"alert_voice",
		"minimax|groq|grok|mimo",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected help output to contain %q\n%s", check, out)
		}
	}
}

func TestLoadConfigInvalidYAMLWarnsAndUsesEmptyDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("provider_priority: [\n  - : broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	ResetConfigForTest(path)
	t.Cleanup(func() { ResetConfigForTest("") })

	// Capture stderr
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	cfg := LoadConfig()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	if cfg == nil {
		t.Fatal("expected empty config object after invalid YAML, got nil")
	}
	if len(cfg.ProviderPriority) != 0 {
		t.Fatalf("expected empty defaults, got priority %v", cfg.ProviderPriority)
	}
	if !strings.Contains(buf.String(), "warning: ignoring invalid config") {
		t.Fatalf("expected stderr warning, got %q", buf.String())
	}
	// Second load is cached — no second warning, still empty.
	cfg2 := LoadConfig()
	if cfg2 != cfg {
		t.Fatal("expected cached empty config")
	}
}
