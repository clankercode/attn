package audio

import (
	"encoding/binary"
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
