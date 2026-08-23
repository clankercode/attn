package internal

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/clankercode/attn/internal/cli"
	"github.com/clankercode/attn/internal/tts"
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

type emptyAudioProvider struct{}

func (p *emptyAudioProvider) Name() string { return "empty" }

func (p *emptyAudioProvider) Synthesize(ctx context.Context, text, voice, model string) (*tts.AudioOutput, error) {
	return &tts.AudioOutput{Data: []byte{}}, nil
}

func TestRunExitsNonZeroOnEmptyAudio(t *testing.T) {
	origFactory := providerFactory
	providerFactory = func(t tts.ProviderType, voice, model string) tts.Provider {
		return &emptyAudioProvider{}
	}
	t.Cleanup(func() { providerFactory = origFactory })

	code := run([]string{"hello"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for empty audio, got %d", code)
	}
}

type failThenOKProvider struct {
	name string
	fail bool
}

func (p *failThenOKProvider) Name() string { return p.name }

func (p *failThenOKProvider) Synthesize(ctx context.Context, text, voice, model string) (*tts.AudioOutput, error) {
	if p.fail {
		return nil, fmt.Errorf("simulated %s failure", p.name)
	}
	return &tts.AudioOutput{Data: []byte("fake-audio-data")}, nil
}

func TestSynthesizeFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTS_PROVIDER", "")
	t.Setenv("ATTN_NO_HISTORY", "1")
	cli.ResetConfigForTest(filepath.Join(tmp, "missing.yaml"))
	t.Cleanup(func() { cli.ResetConfigForTest("") })

	var tried []string
	origFactory := providerFactory
	providerFactory = func(pt tts.ProviderType, voice, model string) tts.Provider {
		tried = append(tried, string(pt))
		// First default candidate is llmp-grok — fail it; succeed on anything else.
		return &failThenOKProvider{name: string(pt), fail: pt == tts.ProviderLlmpGrok}
	}
	t.Cleanup(func() { providerFactory = origFactory })

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	oldStderr := os.Stderr
	os.Stderr = stderrW
	code := run([]string{"--silent", "-o", filepath.Join(tmp, "out.mp3"), "hello"})
	stderrW.Close()
	os.Stderr = oldStderr
	stderrBytes, _ := io.ReadAll(stderrR)
	stderrR.Close()

	if code != 0 {
		t.Fatalf("expected exit 0 after fallback, got %d; stderr=%s", code, stderrBytes)
	}
	if len(tried) < 2 {
		t.Fatalf("expected at least 2 provider attempts, got %v", tried)
	}
	if tried[0] != "llmp-grok" {
		t.Fatalf("expected first attempt llmp-grok, got %v", tried)
	}
	if !strings.Contains(string(stderrBytes), "llmp-grok failed") {
		t.Fatalf("expected fallback warning in stderr, got %q", stderrBytes)
	}
	if !strings.Contains(string(stderrBytes), "trying") {
		t.Fatalf("expected 'trying' in stderr, got %q", stderrBytes)
	}
}

func TestExplicitProviderNoFallback(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("TTS_PROVIDER", "")
	t.Setenv("ATTN_NO_HISTORY", "1")
	cli.ResetConfigForTest(filepath.Join(tmp, "missing.yaml"))
	t.Cleanup(func() { cli.ResetConfigForTest("") })

	var tried []string
	origFactory := providerFactory
	providerFactory = func(pt tts.ProviderType, voice, model string) tts.Provider {
		tried = append(tried, string(pt))
		return &failThenOKProvider{name: string(pt), fail: true}
	}
	t.Cleanup(func() { providerFactory = origFactory })

	code := run([]string{"--provider", "grok", "--silent", "-o", filepath.Join(tmp, "out.mp3"), "hello"})
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if len(tried) != 1 || tried[0] != "grok" {
		t.Fatalf("explicit provider must not fall back; tried %v", tried)
	}
}

func TestAdjustOutputExt(t *testing.T) {
	if got := adjustOutputExt("/tmp/x.mp3", tts.ProviderMimo); got != "/tmp/x.wav" {
		t.Fatalf("mp3→wav for mimo: %q", got)
	}
	if got := adjustOutputExt("/tmp/x.wav", tts.ProviderGrok); got != "/tmp/x.mp3" {
		t.Fatalf("wav→mp3 for grok: %q", got)
	}
	if got := adjustOutputExt("/tmp/x.mp3", tts.ProviderGrok); got != "/tmp/x.mp3" {
		t.Fatalf("same ext unchanged: %q", got)
	}
	if got := adjustOutputExt("/tmp/x.bin", tts.ProviderMimo); got != "/tmp/x.bin" {
		t.Fatalf("non-audio ext unchanged: %q", got)
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
