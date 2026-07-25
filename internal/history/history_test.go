package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupEnv(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmp, "data"))
	return tmp
}

func writeAudio(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("RIFFfake"), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestRecordAndLoad(t *testing.T) {
	tmp := setupEnv(t)
	audioPath := filepath.Join(tmp, ".tts-output", "1778000000000000001.mp3")
	writeAudio(t, audioPath)

	e := Entry{
		Time:       time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC),
		Text:       "hello world",
		SpokenText: "... hello world.",
		Provider:   "grok",
		Voice:      "eve",
		Path:       audioPath,
		Bytes:      8,
	}
	if err := Record(e); err != nil {
		t.Fatalf("Record: %v", err)
	}

	entries, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	got := entries[0]
	if got.Text != "hello world" || got.Provider != "grok" || got.Voice != "eve" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
	if got.Missing {
		t.Error("entry should not be marked missing")
	}
	if got.Legacy {
		t.Error("recorded entry should not be legacy")
	}
}

func TestLoadSkipsCorruptLines(t *testing.T) {
	tmp := setupEnv(t)
	audioPath := filepath.Join(tmp, ".tts-output", "x.mp3")
	writeAudio(t, audioPath)

	e := Entry{Time: time.Now(), Text: "good", Provider: "groq", Voice: "daniel", Path: audioPath}
	if err := Record(e); err != nil {
		t.Fatal(err)
	}
	// Append garbage after the valid line.
	f, err := os.OpenFile(Path(), os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("{not json\n\n")
	f.Close()

	entries, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(entries) != 1 || entries[0].Text != "good" {
		t.Fatalf("corrupt lines broke load: %+v", entries)
	}
}

func TestLegacyScan(t *testing.T) {
	setupEnv(t)
	dir := DefaultOutputDir()
	legacyPath := filepath.Join(dir, "1778000000000000002.mp3")
	writeAudio(t, legacyPath)

	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 legacy entry, got %d", len(entries))
	}
	got := entries[0]
	if !got.Legacy {
		t.Error("expected legacy entry")
	}
	want := time.Unix(0, 1778000000000000002)
	if !got.Time.Equal(want) {
		t.Errorf("timestamp from filename: got %v want %v", got.Time, want)
	}
	if got.Label() != "1778000000000000002.mp3" {
		t.Errorf("legacy label should be filename, got %q", got.Label())
	}
}

func TestLegacySkippedWhenRecorded(t *testing.T) {
	setupEnv(t)
	audioPath := filepath.Join(DefaultOutputDir(), "1778000000000000003.wav")
	writeAudio(t, audioPath)

	e := Entry{Time: time.Now(), Text: "spoken", Provider: "mimo", Voice: "default_zh", Path: audioPath}
	if err := Record(e); err != nil {
		t.Fatal(err)
	}

	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("recorded file should not also appear as legacy: %d entries", len(entries))
	}
	if entries[0].Legacy {
		t.Error("entry should be the recorded one, not legacy")
	}
}

func TestMissingMarked(t *testing.T) {
	setupEnv(t)
	e := Entry{Time: time.Now(), Text: "gone", Provider: "grok", Voice: "eve", Path: "/nonexistent/x.mp3"}
	if err := Record(e); err != nil {
		t.Fatal(err)
	}
	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || !entries[0].Missing {
		t.Fatalf("expected missing entry, got %+v", entries)
	}
}

func TestDeleteRemovesFileAndRecord(t *testing.T) {
	tmp := setupEnv(t)
	audioPath := filepath.Join(tmp, ".tts-output", "del.mp3")
	writeAudio(t, audioPath)

	e := Entry{Time: time.Now(), Text: "delete me", Provider: "grok", Voice: "eve", Path: audioPath}
	if err := Record(e); err != nil {
		t.Fatal(err)
	}
	if err := Delete(e); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Error("audio file should be removed")
	}
	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("history record should be removed, got %+v", entries)
	}
}

func TestDeleteLegacyOnlyRemovesFile(t *testing.T) {
	setupEnv(t)
	audioPath := filepath.Join(DefaultOutputDir(), "1778000000000000004.mp3")
	writeAudio(t, audioPath)

	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected legacy entry, got %d", len(entries))
	}
	if err := Delete(entries[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(audioPath); !os.IsNotExist(err) {
		t.Error("legacy audio file should be removed")
	}
}

func TestNewestFirst(t *testing.T) {
	tmp := setupEnv(t)
	for i, text := range []string{"first", "second", "third"} {
		p := filepath.Join(tmp, ".tts-output", text+".mp3")
		writeAudio(t, p)
		e := Entry{
			Time:     time.Date(2026, 7, 25, 10, i, 0, 0, time.UTC),
			Text:     text,
			Provider: "grok",
			Voice:    "eve",
			Path:     p,
		}
		if err := Record(e); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 || entries[0].Text != "third" || entries[2].Text != "first" {
		var order []string
		for _, en := range entries {
			order = append(order, en.Text)
		}
		t.Fatalf("not newest-first: %s", strings.Join(order, ","))
	}
}
