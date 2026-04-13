package tts

import "testing"

func TestRandomVoiceReturnsKnownVoice(t *testing.T) {
	voice := RandomVoice(ProviderMinimax)
	if !ValidateVoice(ProviderMinimax, voice) {
		t.Fatalf("expected a valid minimax voice, got %q", voice)
	}

	voice = RandomVoice(ProviderGroq)
	if !ValidateVoice(ProviderGroq, voice) {
		t.Fatalf("expected a valid groq voice, got %q", voice)
	}

	voice = RandomVoice(ProviderMimo)
	if !ValidateVoice(ProviderMimo, voice) {
		t.Fatalf("expected a valid mimo voice, got %q", voice)
	}
}

func TestRandomVoiceFallsBackToMinimaxSet(t *testing.T) {
	voice := RandomVoice(ProviderType("unknown"))
	if !ValidateVoice(ProviderMinimax, voice) {
		t.Fatalf("expected fallback voice from minimax set, got %q", voice)
	}
}
