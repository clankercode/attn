package tts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadGrokAuthJSONPrefersNonExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	// expired + fresh entries
	content := `{
  "https://auth.x.ai::old": {
    "key": "expired-token",
    "expires_at": "2020-01-01T00:00:00Z"
  },
  "https://auth.x.ai::new": {
    "key": "fresh-token",
    "expires_at": "2099-01-01T00:00:00Z"
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	key, ok := readGrokAuthJSON(path, time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC))
	if !ok || key != "fresh-token" {
		t.Fatalf("expected fresh-token (ok), got key=%q ok=%v", key, ok)
	}
}

func TestReadGrokAuthJSONFallsBackToExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")
	content := `{
  "https://auth.x.ai::old": {
    "key": "only-expired",
    "expires_at": "2020-01-01T00:00:00Z"
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	key, ok := readGrokAuthJSON(path, time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC))
	if key != "only-expired" {
		t.Fatalf("expected expired fallback key, got %q", key)
	}
	if ok {
		t.Fatal("expected notExpired=false for expired token")
	}
}

func TestLoadGrokKeyFromHomeDirsOrder(t *testing.T) {
	home := t.TempDir()
	// Create .grok2 with a key; .grok should win if present.
	for _, dir := range []string{".grok", ".grok2"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeAuth := func(dir, key string) {
		path := filepath.Join(home, dir, "auth.json")
		body := `{"https://auth.x.ai::x":{"key":"` + key + `","expires_at":"2099-01-01T00:00:00Z"}}`
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeAuth(".grok2", "from-grok2")
	// Without .grok/auth.json, .grok1 then .grok2...
	got, src := loadGrokKeyFromHomeDirs(func() (string, error) { return home, nil })
	if got != "from-grok2" {
		t.Fatalf("expected from-grok2, got %q (src=%s)", got, src)
	}
	writeAuth(".grok", "from-grok")
	got, src = loadGrokKeyFromHomeDirs(func() (string, error) { return home, nil })
	if got != "from-grok" {
		t.Fatalf("expected .grok to win, got %q (src=%s)", got, src)
	}
}

func TestLoadGrokKeyHyphenatedDirs(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".grok-1"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"https://auth.x.ai::x":{"key":"hyphen-key","expires_at":"2099-01-01T00:00:00Z"}}`
	if err := os.WriteFile(filepath.Join(home, ".grok-1", "auth.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _ := loadGrokKeyFromHomeDirs(func() (string, error) { return home, nil })
	if got != "hyphen-key" {
		t.Fatalf("expected hyphen-key, got %q", got)
	}
}

func TestLoadGrokKeyPrefersFreshOverEarlierExpired(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".grok", ".grok1"} {
		if err := os.MkdirAll(filepath.Join(home, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	expired := `{"https://auth.x.ai::x":{"key":"expired-first","expires_at":"2020-01-01T00:00:00Z"}}`
	fresh := `{"https://auth.x.ai::x":{"key":"fresh-second","expires_at":"2099-01-01T00:00:00Z"}}`
	if err := os.WriteFile(filepath.Join(home, ".grok", "auth.json"), []byte(expired), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".grok1", "auth.json"), []byte(fresh), 0o600); err != nil {
		t.Fatal(err)
	}
	got, src := loadGrokKeyFromHomeDirs(func() (string, error) { return home, nil })
	if got != "fresh-second" {
		t.Fatalf("expected fresh-second over earlier expired, got %q src=%s", got, src)
	}
	if !strings.Contains(src, ".grok1") {
		t.Fatalf("expected source to mention .grok1, got %q", src)
	}
}

func TestResolveGrokAPIKeyEnvWins(t *testing.T) {
	t.Setenv("XAI_API_KEY", "env-xai")
	t.Setenv("GROK_API_KEY", "env-grok")
	if got := ResolveGrokAPIKey(); got != "env-xai" {
		t.Fatalf("XAI_API_KEY should win, got %q", got)
	}
	t.Setenv("XAI_API_KEY", "")
	if got := ResolveGrokAPIKey(); got != "env-grok" {
		t.Fatalf("GROK_API_KEY should be used when XAI unset, got %q", got)
	}
}

func TestValidateVoiceGrokOpenCatalog(t *testing.T) {
	if !ValidateVoice(ProviderGrok, "Eve") {
		t.Fatal("expected Eve to validate")
	}
	if !ValidateVoice(ProviderGrok, "eve") {
		t.Fatal("expected eve to validate")
	}
	// Custom / cloned voice IDs are allowed (open catalog).
	if !ValidateVoice(ProviderGrok, "my-custom-clone-id") {
		t.Fatal("expected custom voice id to validate for open catalog")
	}
	if ValidateVoice(ProviderGrok, "") {
		t.Fatal("empty voice should not validate")
	}
}

func TestDefaultAlertVoiceGrok(t *testing.T) {
	if got := DefaultAlertVoice(ProviderGrok); got != "rex" {
		t.Fatalf("expected rex, got %q", got)
	}
}

func TestResolveProviderGrok(t *testing.T) {
	if got := ResolveProvider("grok", nil); got != ProviderGrok {
		t.Fatalf("got %s", got)
	}
	if got := ResolveProvider("", []string{"nope", "grok", "minimax"}); got != ProviderGrok {
		t.Fatalf("expected grok from priority, got %s", got)
	}
}

func TestCatalogGrok(t *testing.T) {
	cat := Catalog(ProviderGrok)
	if len(cat) != len(VoiceListGrok) {
		t.Fatalf("catalog length %d != VoiceListGrok %d", len(cat), len(VoiceListGrok))
	}
	// Open catalog: custom preferred kept.
	pool := EffectivePool(ProviderGrok, VoicePrefs{Preferred: []string{"eve", "my-custom-clone"}})
	if len(pool) != 2 {
		t.Fatalf("expected open-catalog preferred keep, got %v", pool)
	}
}
