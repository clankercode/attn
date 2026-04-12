package audio

import (
	"encoding/binary"
	"path/filepath"
	"testing"
)

func TestPlaybackCommandForPipeWire(t *testing.T) {
	name, args := playbackCommand("pw-play", 32000)

	if name != "pw-play" {
		t.Fatalf("expected pw-play, got %q", name)
	}
	want := []string{"--raw", "--format", "s16", "--rate", "32000", "--channels", "2", "--latency", "50ms", "-"}
	if len(args) != len(want) {
		t.Fatalf("expected %d args, got %d: %#v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d: expected %q, got %q", i, want[i], args[i])
		}
	}
}

func TestPlaybackCommandForPulseAudio(t *testing.T) {
	name, args := playbackCommand("pacat", 44100)

	if name != "pacat" {
		t.Fatalf("expected pacat, got %q", name)
	}
	want := []string{"--raw", "--format=s16le", "--rate=44100", "--channels=2", "--latency-msec=50", "-"}
	if len(args) != len(want) {
		t.Fatalf("expected %d args, got %d: %#v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Fatalf("arg %d: expected %q, got %q", i, want[i], args[i])
		}
	}
}

func TestSamplesToPCM16LE(t *testing.T) {
	pcm := samplesToPCM16LE([][2]float64{{0.0, 0.5}, {-1.0, 1.0}})

	if len(pcm) != 8 {
		t.Fatalf("expected 8 bytes, got %d", len(pcm))
	}
	got := []int16{
		int16(binary.LittleEndian.Uint16(pcm[0:2])),
		int16(binary.LittleEndian.Uint16(pcm[2:4])),
		int16(binary.LittleEndian.Uint16(pcm[4:6])),
		int16(binary.LittleEndian.Uint16(pcm[6:8])),
	}
	want := []int16{0, 16384, -32768, 32767}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sample %d: expected %d, got %d", i, want[i], got[i])
		}
	}
}

func TestPlayAndSaveBackgroundCallsDetachedSpawnerNotForeground(t *testing.T) {
	originalSpawn := spawnDetachedPlayback
	originalPlay := playFileFn
	spawnCalled := false

	spawnDetachedPlayback = func(path string, lock *lockState) error {
		spawnCalled = true
		if path == "" {
			t.Fatal("expected non-empty path")
		}
		if lock == nil {
			t.Fatal("expected non-nil lock")
		}
		return nil
	}
	playFileFn = func(path string) error {
		t.Fatalf("foreground playFile should not be called for bg mode: %s", path)
		return nil
	}
	t.Cleanup(func() {
		spawnDetachedPlayback = originalSpawn
		playFileFn = originalPlay
	})

	outputPath := filepath.Join(t.TempDir(), "sample.wav")
	err := PlayAndSave(testWAVData(), outputPath, true, false, false)
	if err != nil {
		t.Fatalf("PlayAndSave() error = %v", err)
	}
	if !spawnCalled {
		t.Fatal("expected detached spawner to be called for bg mode")
	}
}

func TestForegroundPlayAndSaveCallsForegroundPlayer(t *testing.T) {
	originalSpawn := spawnDetachedPlayback
	originalPlay := playFileFn
	foregroundCalled := false

	spawnDetachedPlayback = func(path string, lock *lockState) error {
		t.Fatalf("detached spawner should not be called for fg mode: %s", path)
		return nil
	}
	playFileFn = func(path string) error {
		foregroundCalled = true
		if path == "" {
			t.Fatal("expected non-empty path")
		}
		return nil
	}
	t.Cleanup(func() {
		spawnDetachedPlayback = originalSpawn
		playFileFn = originalPlay
	})

	outputPath := filepath.Join(t.TempDir(), "sample.wav")
	err := PlayAndSave(testWAVData(), outputPath, true, true, false)
	if err != nil {
		t.Fatalf("PlayAndSave() error = %v", err)
	}
	if !foregroundCalled {
		t.Fatal("expected foreground player to be called for fg mode")
	}
}

func testWAVData() []byte {
	return []byte{
		'R', 'I', 'F', 'F',
		40, 0, 0, 0,
		'W', 'A', 'V', 'E',
		'f', 'm', 't', ' ',
		16, 0, 0, 0,
		1, 0,
		2, 0,
		0x80, 0x3E, 0, 0,
		0x00, 0xFA, 0x00, 0x00,
		4, 0,
		16, 0,
		'd', 'a', 't', 'a',
		4, 0, 0, 0,
		0, 0, 0, 0,
	}
}
