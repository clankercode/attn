package cli

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseReturnsErrorForInvalidFlags(t *testing.T) {
	_, err := Parse([]string{"--definitely-not-a-real-flag", "hello"})
	if err == nil {
		t.Fatal("expected invalid flag to return an error")
	}
}

func TestParseGeneratesUniqueDefaultOutputs(t *testing.T) {
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
		t.Fatalf("expected default minimax output to be .mp3, got %q", first.Output)
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
		"Common flags:",
		"--alert",
		"--model",
		"--silent",
		"--debug-play-file",
		"Defaults:",
		"provider: minimax",
		"voice: random for normal playback",
	}
	for _, check := range checks {
		if !strings.Contains(out, check) {
			t.Fatalf("expected help output to contain %q\n%s", check, out)
		}
	}
}
