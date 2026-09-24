package audio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/faiface/beep"
)

// fakeSink puts an executable shell script called name first on PATH. It
// records its pid (exec keeps it) and then runs body. Returns the pid file.
func fakeSink(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "pid")
	script := "#!/bin/sh\necho $$ > '" + pidFile + "'\nexec " + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return pidFile
}

// waitForPid polls pidFile until the fake sink has written its pid.
func waitForPid(t *testing.T, pidFile string) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if raw, err := os.ReadFile(pidFile); err == nil {
			if pid, err := strconv.Atoi(strings.TrimSpace(string(raw))); err == nil {
				return pid
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("fake sink never started")
	return 0
}

// assertCancelledPromptly cancels run's context ~100ms into playback and
// checks it returns context.Canceled quickly and the sink process is gone.
func assertCancelledPromptly(t *testing.T, pidFile string, run func(ctx context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()

	pid := waitForPid(t, pidFile)
	time.Sleep(100 * time.Millisecond)
	cancelled := time.Now()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
		if d := time.Since(cancelled); d > 2*time.Second {
			t.Fatalf("returned %v after cancel", d)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("did not return within 2s of cancel")
	}
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("fake sink pid %d still exists (kill 0: %v)", pid, err)
	}
}

func TestStreamToSinkStopsOnCancel(t *testing.T) {
	pidFile := fakeSink(t, "pw-play", "cat >/dev/null")
	format := beep.Format{SampleRate: 44100, NumChannels: 2, Precision: 2}
	assertCancelledPromptly(t, pidFile, func(ctx context.Context) error {
		return streamToSink(ctx, beep.Silence(-1), format, "pw-play")
	})
}

func TestPlayFileDirectStopsOnCancel(t *testing.T) {
	pidFile := fakeSink(t, "mpv", "sleep 30")
	assertCancelledPromptly(t, pidFile, func(ctx context.Context) error {
		return playFileDirect(ctx, "unused.wav", "mpv")
	})
}

func TestReacquireLockDoesNotWait(t *testing.T) {
	originalLockDir := lockDir
	lockDir = t.TempDir()
	t.Cleanup(func() { lockDir = originalLockDir })

	held, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	start := time.Now()
	if _, err := reacquireLock(); !errors.Is(err, ErrAlreadyPlaying) {
		t.Fatalf("reacquireLock() error = %v, want ErrAlreadyPlaying", err)
	}
	if d := time.Since(start); d > 500*time.Millisecond {
		t.Fatalf("reacquireLock blocked for %v", d)
	}

	held.Release()
	release, err := reacquireLock()
	if err != nil {
		t.Fatalf("reacquireLock() after release: %v", err)
	}
	release()
}
