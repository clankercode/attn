package tts

import (
	"os"
	"path/filepath"
	"testing"
)

func resetLLMPState(t *testing.T) {
	t.Helper()
	prev := llmpConfig
	t.Cleanup(func() { llmpConfig = prev })
	llmpConfig = LLMPConfig{}
	for _, env := range []string{"LLMP_API_KEY", "LLMP_KEY_FILE", "LLMP_BASE_URL"} {
		os.Unsetenv(env)
	}
}

func TestResolveLLMPBaseURLDefault(t *testing.T) {
	resetLLMPState(t)
	if got := ResolveLLMPBaseURL(); got != DefaultLLMPBaseURL {
		t.Fatalf("got %q, want %q", got, DefaultLLMPBaseURL)
	}
}

func TestResolveLLMPBaseURLConfig(t *testing.T) {
	resetLLMPState(t)
	llmpConfig = LLMPConfig{BaseURL: "http://10.42.0.8:24584/v1"}
	if got := ResolveLLMPBaseURL(); got != "http://10.42.0.8:24584/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveLLMPBaseURLEnvWins(t *testing.T) {
	resetLLMPState(t)
	llmpConfig = LLMPConfig{BaseURL: "http://config.example/v1"}
	t.Setenv("LLMP_BASE_URL", "http://env.example/v1")
	if got := ResolveLLMPBaseURL(); got != "http://env.example/v1" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveLLMPAPIKeyFromConfigKeyFile(t *testing.T) {
	resetLLMPState(t)
	dir := t.TempDir()
	kf := filepath.Join(dir, "key")
	if err := os.WriteFile(kf, []byte("  sk-llmp-test-123\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	llmpConfig = LLMPConfig{KeyFile: kf}
	key, src := ResolveLLMPAPIKeyWithSource()
	if key != "sk-llmp-test-123" {
		t.Fatalf("key = %q", key)
	}
	if src != kf {
		t.Fatalf("source = %q, want %q", src, kf)
	}
}

func TestResolveLLMPAPIKeyConfigKeyFileTilde(t *testing.T) {
	resetLLMPState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	kf := filepath.Join(home, ".llmp")
	if err := os.WriteFile(kf, []byte("sk-llmp-tilde\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	llmpConfig = LLMPConfig{KeyFile: "~/.llmp"}
	if key := ResolveLLMPAPIKey(); key != "sk-llmp-tilde" {
		t.Fatalf("key = %q", key)
	}
}

func TestResolveLLMPAPIKeyDefaultDotLlmp(t *testing.T) {
	resetLLMPState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	kf := filepath.Join(home, ".llmp")
	if err := os.WriteFile(kf, []byte("sk-llmp-home"), 0o600); err != nil {
		t.Fatal(err)
	}
	if key := ResolveLLMPAPIKey(); key != "sk-llmp-home" {
		t.Fatalf("key = %q", key)
	}
}

func TestResolveLLMPAPIKeyEnvWins(t *testing.T) {
	resetLLMPState(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.WriteFile(filepath.Join(home, ".llmp"), []byte("file-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLMP_API_KEY", "env-key")
	key, src := ResolveLLMPAPIKeyWithSource()
	if key != "env-key" || src != "LLMP_API_KEY" {
		t.Fatalf("key=%q src=%q", key, src)
	}
}

func TestResolveLLMPAPIKeyEnvKeyFile(t *testing.T) {
	resetLLMPState(t)
	dir := t.TempDir()
	kf := filepath.Join(dir, "envkey")
	if err := os.WriteFile(kf, []byte("env-file-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LLMP_KEY_FILE", kf)
	if key := ResolveLLMPAPIKey(); key != "env-file-key" {
		t.Fatalf("key = %q", key)
	}
}

func TestResolveLLMPAPIKeyMissing(t *testing.T) {
	resetLLMPState(t)
	t.Setenv("HOME", t.TempDir())
	if key := ResolveLLMPAPIKey(); key != "" {
		t.Fatalf("expected empty key, got %q", key)
	}
}

func TestLlmpGrokProviderDefaults(t *testing.T) {
	p := newLlmpGrok("", "")
	if p.Name() != "llmp-grok" {
		t.Fatalf("name = %q", p.Name())
	}
	gp, ok := p.(*llmpGrokProvider)
	if !ok {
		t.Fatalf("unexpected type %T", p)
	}
	if gp.voice != "eve" {
		t.Fatalf("default voice = %q", gp.voice)
	}
}

func TestLlmpGrokNoCredentialError(t *testing.T) {
	resetLLMPState(t)
	t.Setenv("HOME", t.TempDir())
	p := newLlmpGrok("", "")
	_, err := p.Synthesize(t.Context(), "hello", "", "")
	if err == nil {
		t.Fatal("expected error without credential")
	}
}

func TestLlmpGrokCatalogAndAlertVoice(t *testing.T) {
	cat := Catalog(ProviderLlmpGrok)
	if len(cat) != len(VoiceListGrok) {
		t.Fatalf("catalog len = %d, want %d", len(cat), len(VoiceListGrok))
	}
	if got := DefaultAlertVoice(ProviderLlmpGrok); got != "rex" {
		t.Fatalf("alert voice = %q", got)
	}
	if closedCatalog(ProviderLlmpGrok) {
		t.Fatal("llmp-grok should be an open catalog (custom voice IDs allowed)")
	}
}

func TestLlmpGrokVoiceSelectionNormalizesCase(t *testing.T) {
	// Explicit voice should be lowercased like grok.
	if got := SelectVoice(ProviderLlmpGrok, VoicePrefs{}, false, "EVE"); got != "eve" {
		t.Fatalf("explicit voice = %q", got)
	}
	// Alert voice default.
	if got := SelectVoice(ProviderLlmpGrok, VoicePrefs{}, true, ""); got != "rex" {
		t.Fatalf("alert voice = %q", got)
	}
	// Preferred pool honored (case-insensitive).
	got := SelectVoice(ProviderLlmpGrok, VoicePrefs{Preferred: []string{"ARA"}}, false, "")
	if got != "ara" {
		t.Fatalf("preferred voice = %q", got)
	}
	// Banned respected: ban everything except one catalog voice.
	banned := append([]string(nil), VoiceListGrok...)
	pool := EffectivePool(ProviderLlmpGrok, VoicePrefs{Banned: banned})
	if len(pool) != 0 {
		t.Fatalf("pool = %v, want empty", pool)
	}
}

func TestProviderCandidatesIncludesLlmpGrok(t *testing.T) {
	cands := ProviderCandidates("", nil)
	if len(cands) == 0 || cands[0] != ProviderLlmpGrok {
		t.Fatalf("first default candidate = %v", cands)
	}
	// Explicit alias.
	if got := ResolveProvider("llmp", nil); got != ProviderLlmpGrok {
		t.Fatalf("ResolveProvider(llmp) = %q", got)
	}
	if got := ResolveProvider("llmp-grok", nil); got != ProviderLlmpGrok {
		t.Fatalf("ResolveProvider(llmp-grok) = %q", got)
	}
	// Priority list with the new provider.
	cands = ProviderCandidates("", []string{"mimo", "llmp-grok", "grok"})
	if len(cands) != 3 || cands[0] != ProviderMimo || cands[1] != ProviderLlmpGrok || cands[2] != ProviderGrok {
		t.Fatalf("candidates = %v", cands)
	}
	// Unknown-only priority falls back to llmp-grok.
	cands = ProviderCandidates("", []string{"bogus"})
	if len(cands) != 1 || cands[0] != ProviderLlmpGrok {
		t.Fatalf("candidates = %v", cands)
	}
}
