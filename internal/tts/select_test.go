package tts

import (
	"math/rand"
	"testing"
)

func withSeed(t *testing.T, seed int64) {
	t.Helper()
	prev := randomSource
	randomSource = rand.New(rand.NewSource(seed))
	t.Cleanup(func() { randomSource = prev })
}

func TestSelectVoiceExplicitWins(t *testing.T) {
	got := SelectVoice(ProviderGroq, VoicePrefs{
		Preferred: []string{"autumn"},
		Banned:    []string{"daniel"},
		Alert:     "hannah",
	}, true, "troy")
	if got != "troy" {
		t.Fatalf("explicit voice should win, got %q", got)
	}
}

func TestSelectVoiceAlertUsesConfigThenDefault(t *testing.T) {
	got := SelectVoice(ProviderGroq, VoicePrefs{Alert: "hannah"}, true, "")
	if got != "hannah" {
		t.Fatalf("expected configured alert voice, got %q", got)
	}

	got = SelectVoice(ProviderGroq, VoicePrefs{}, true, "")
	if got != "daniel" {
		t.Fatalf("expected default groq alert voice, got %q", got)
	}

	got = SelectVoice(ProviderMinimax, VoicePrefs{}, true, "")
	if got != "Deep_Voice_Man" {
		t.Fatalf("expected default minimax alert voice, got %q", got)
	}

	got = SelectVoice(ProviderMimo, VoicePrefs{}, true, "")
	if got != "mimo_default" {
		t.Fatalf("expected default mimo alert voice, got %q", got)
	}

	got = SelectVoice(ProviderGrok, VoicePrefs{}, true, "")
	if got != "rex" {
		t.Fatalf("expected default grok alert voice, got %q", got)
	}
}

func TestSelectVoiceUsesPreferredPool(t *testing.T) {
	withSeed(t, 1)
	prefs := VoicePrefs{Preferred: []string{"autumn", "diana"}}
	for i := 0; i < 20; i++ {
		got := SelectVoice(ProviderGroq, prefs, false, "")
		if got != "autumn" && got != "diana" {
			t.Fatalf("got voice outside preferred pool: %q", got)
		}
	}
}

func TestSelectVoiceHonorsBanned(t *testing.T) {
	withSeed(t, 42)
	// Ban every groq voice except autumn.
	banned := []string{"diana", "hannah", "austin", "daniel", "troy"}
	prefs := VoicePrefs{Banned: banned}
	for i := 0; i < 10; i++ {
		got := SelectVoice(ProviderGroq, prefs, false, "")
		if got != "autumn" {
			t.Fatalf("expected only autumn to remain, got %q", got)
		}
	}
}

func TestSelectVoicePreferredAllBannedFallsBackToCatalog(t *testing.T) {
	withSeed(t, 7)
	prefs := VoicePrefs{
		Preferred: []string{"troy"},
		Banned:    []string{"troy"},
	}
	got := SelectVoice(ProviderGroq, prefs, false, "")
	if got == "troy" {
		t.Fatal("banned preferred voice should not be selected when catalog alternatives exist")
	}
	if !ValidateVoice(ProviderGroq, got) {
		t.Fatalf("fallback should be a catalog voice, got %q", got)
	}
}

func TestEffectivePoolAllBannedReturnsEmpty(t *testing.T) {
	prefs := VoicePrefs{Banned: append([]string(nil), VoiceListGroq...)}
	pool := EffectivePool(ProviderGroq, prefs)
	if len(pool) != 0 {
		t.Fatalf("expected empty pool when everything banned, got %v", pool)
	}
	// SelectVoice must still return something usable.
	got := SelectVoice(ProviderGroq, prefs, false, "")
	if got != DefaultAlertVoice(ProviderGroq) {
		t.Fatalf("expected alert default fallback, got %q", got)
	}
}

func TestEffectivePoolFiltersClosedCatalogPreferred(t *testing.T) {
	// Global-style mixed preferred: minimax name should not enter groq pool.
	prefs := VoicePrefs{Preferred: []string{"daniel", "Deep_Voice_Man", "autumn"}}
	pool := EffectivePool(ProviderGroq, prefs)
	for _, v := range pool {
		if v == "Deep_Voice_Man" {
			t.Fatalf("closed catalog should drop unknown preferred %q; pool=%v", v, pool)
		}
	}
	if len(pool) != 2 {
		t.Fatalf("expected daniel+autumn, got %v", pool)
	}

	// MiniMax keeps non-catalog preferred IDs (open catalog).
	prefs = VoicePrefs{Preferred: []string{"custom_voice_id", "Wise_Woman"}}
	pool = EffectivePool(ProviderMinimax, prefs)
	if len(pool) != 2 {
		t.Fatalf("minimax should keep custom preferred ids, got %v", pool)
	}
}

func TestNormalizePrefsTrims(t *testing.T) {
	prefs := NormalizePrefs(VoicePrefs{
		Preferred: []string{"  autumn ", ""},
		Banned:    []string{" troy"},
		Alert:     " hannah ",
	})
	if len(prefs.Preferred) != 1 || prefs.Preferred[0] != "autumn" {
		t.Fatalf("preferred: %v", prefs.Preferred)
	}
	if len(prefs.Banned) != 1 || prefs.Banned[0] != "troy" {
		t.Fatalf("banned: %v", prefs.Banned)
	}
	if prefs.Alert != "hannah" {
		t.Fatalf("alert: %q", prefs.Alert)
	}
}

func TestResolveProvider(t *testing.T) {
	if got := ResolveProvider("groq", []string{"minimax"}); got != ProviderGroq {
		t.Fatalf("explicit should win, got %s", got)
	}
	if got := ResolveProvider("", []string{"mimo", "groq"}); got != ProviderMimo {
		t.Fatalf("expected first priority, got %s", got)
	}
	if got := ResolveProvider("", []string{"nope", "groq"}); got != ProviderGroq {
		t.Fatalf("expected skip unknown, got %s", got)
	}
	if got := ResolveProvider("", []string{"grok"}); got != ProviderGrok {
		t.Fatalf("expected grok from priority, got %s", got)
	}
	if got := ResolveProvider("", nil); got != ProviderLlmpGrok {
		t.Fatalf("expected built-in default llmp-grok, got %s", got)
	}
	if got := ResolveProvider("", []string{}); got != ProviderLlmpGrok {
		t.Fatalf("expected built-in default llmp-grok for empty priority, got %s", got)
	}
	// Default chain: llmp-grok → grok → mimo → minimax (first wins).
	if got := ResolveProvider("", DefaultProviderPriority); got != ProviderLlmpGrok {
		t.Fatalf("expected llmp-grok first in default priority, got %s", got)
	}
	if got := ResolveProvider("xiaomi", nil); got != ProviderMimo {
		t.Fatalf("xiaomi alias should resolve to mimo, got %s", got)
	}
	if got := ResolveProvider("", []string{"xiaomi", "minimax"}); got != ProviderMimo {
		t.Fatalf("priority xiaomi should resolve to mimo, got %s", got)
	}
}

func TestProviderCandidates(t *testing.T) {
	got := ProviderCandidates("groq", []string{"minimax", "mimo"})
	if len(got) != 1 || got[0] != ProviderGroq {
		t.Fatalf("explicit should be single-element, got %v", got)
	}

	got = ProviderCandidates("", []string{"mimo", "groq", "minimax"})
	if len(got) != 3 || got[0] != ProviderMimo || got[1] != ProviderGroq || got[2] != ProviderMinimax {
		t.Fatalf("expected ordered priority, got %v", got)
	}

	got = ProviderCandidates("", []string{"grok", "nope", "grok", "mimo"})
	if len(got) != 2 || got[0] != ProviderGrok || got[1] != ProviderMimo {
		t.Fatalf("expected dedupe + skip unknown, got %v", got)
	}

	got = ProviderCandidates("", []string{"nope", "also-nope"})
	if len(got) != 1 || got[0] != ProviderLlmpGrok {
		t.Fatalf("all unknown should fall back to llmp-grok, got %v", got)
	}

	got = ProviderCandidates("", nil)
	if len(got) != 4 || got[0] != ProviderLlmpGrok || got[1] != ProviderGrok || got[2] != ProviderMimo || got[3] != ProviderMinimax {
		t.Fatalf("nil priority should use default, got %v", got)
	}

	got = ProviderCandidates("xiaomi", nil)
	if len(got) != 1 || got[0] != ProviderMimo {
		t.Fatalf("xiaomi alias, got %v", got)
	}
}

func TestRandomVoiceStillWorks(t *testing.T) {
	// Backward-compat: RandomVoice remains a thin wrapper over full catalog.
	withSeed(t, 99)
	v := RandomVoice(ProviderGroq)
	if !ValidateVoice(ProviderGroq, v) {
		t.Fatalf("invalid voice %q", v)
	}
}
