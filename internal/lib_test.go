package internal

import (
	"bytes"
	"encoding/binary"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestRunDryRunDoesNotRequireAPIKey(t *testing.T) {
	if os.Getenv("ATTN_DRY_RUN_CHILD") == "1" {
		os.Unsetenv("GROQ_API_KEY")
		os.Unsetenv("MINIMAX_API_KEY")
		Run([]string{"--dry-run", "-o", "/tmp/attn-dry-run.mp3", "hello"})
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestRunDryRunDoesNotRequireAPIKey")
	cmd.Env = append(os.Environ(), "ATTN_DRY_RUN_CHILD=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dry run should not require provider credentials: %v\n%s", err, output)
	}
}

func TestDebugPlayFileBackgroundReturnsBeforeAudioFinishes(t *testing.T) {
	if os.Getenv("ATTN_DEBUG_PLAY_CHILD") == "1" {
		Run([]string{"--debug-play-file", os.Getenv("ATTN_DEBUG_PLAY_FILE")})
		return
	}

	attnBin := "/tmp/attn-bg-test-bin"
	cmdBuild := exec.Command("go", "build", "-o", attnBin, "./cmd/attn")
	cmdBuild.Dir = "/home/xertrov/src/utils-attn"
	buildOut, err := cmdBuild.CombinedOutput()
	if err != nil {
		t.Skipf("could not build attn: %v\n%s", err, buildOut)
	}
	defer os.Remove(attnBin)

	tmpWav := "/tmp/attn-bg-test.wav"
	defer os.Remove(tmpWav)

	if err := createSilentWav(tmpWav, 2*time.Second, 44100); err != nil {
		t.Fatalf("createSilentWav: %v", err)
	}

	childEnv := append(os.Environ(), "ATTN_DEBUG_PLAY_CHILD=1", "ATTN_DEBUG_PLAY_FILE="+tmpWav)
	cmd := exec.Command(attnBin, "--debug-play-file", tmpWav)
	cmd.Env = childEnv

	start := time.Now()
	if err := cmd.Start(); err != nil {
		t.Fatalf("start attn: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		if err != nil {
			t.Fatalf("attn exited with error: %v", err)
		}
		if elapsed >= 1500*time.Millisecond {
			t.Fatalf("attn --debug-play-file took %v, expected <1.5s (audio is 2s, should return early)", elapsed)
		}
	case <-time.After(5 * time.Second):
		cmd.Process.Kill()
		t.Fatal("attn did not return within 5s")
	}
}

func createSilentWav(path string, dur time.Duration, sampleRate int) error {
	numSamples := int64(float64(dur.Seconds()) * float64(sampleRate))
	numChannels := uint16(2)
	bitsPerSample := uint16(16)
	dataSize := numSamples * int64(numChannels) * int64(bitsPerSample/8)

	var buf bytes.Buffer
	writeStr := func(s string) { buf.WriteString(s) }
	writeU32 := func(v uint32) { binary.Write(&buf, binary.LittleEndian, v) }
	writeU16 := func(v uint16) { binary.Write(&buf, binary.LittleEndian, v) }

	writeStr("RIFF")
	writeU32(uint32(36 + dataSize))
	writeStr("WAVE")
	writeStr("fmt ")
	writeU32(16)
	writeU16(1)
	writeU16(numChannels)
	writeU32(uint32(sampleRate))
	writeU32(uint32(sampleRate) * uint32(numChannels) * uint32(bitsPerSample) / 8)
	writeU16(numChannels * bitsPerSample / 2)
	writeU16(bitsPerSample)
	writeStr("data")
	writeU32(uint32(dataSize))

	silence := make([]byte, dataSize)
	buf.Write(silence)

	return os.WriteFile(path, buf.Bytes(), 0644)
}
