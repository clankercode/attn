package audio

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/clankercode/attn/internal/notify"
)

// useTempLockDir points the playback lock at a per-test directory, so tests
// never touch (or wait on) the real /tmp/attn-tool lock.
func useTempLockDir(t *testing.T) {
	t.Helper()
	original := lockDir
	lockDir = t.TempDir()
	t.Cleanup(func() { lockDir = original })
}

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
	useTempLockDir(t)
	originalSpawn := spawnDetachedPlayback
	originalPlay := playFileFn
	spawnCalled := false

	spawnDetachedPlayback = func(path string, lock *lockState, meta notify.Meta) error {
		spawnCalled = true
		if path == "" {
			t.Fatal("expected non-empty path")
		}
		if lock == nil {
			t.Fatal("expected non-nil lock")
		}
		return nil
	}
	playFileFn = func(ctx context.Context, path string) error {
		t.Fatalf("foreground playFile should not be called for bg mode: %s", path)
		return nil
	}
	t.Cleanup(func() {
		spawnDetachedPlayback = originalSpawn
		playFileFn = originalPlay
	})

	outputPath := filepath.Join(t.TempDir(), "sample.wav")
	err := PlayAndSave(testWAVData(), outputPath, true, false, false, notify.Meta{Disabled: true})
	if err != nil {
		t.Fatalf("PlayAndSave() error = %v", err)
	}
	if !spawnCalled {
		t.Fatal("expected detached spawner to be called for bg mode")
	}
}

func TestForegroundPlayAndSaveCallsForegroundPlayer(t *testing.T) {
	useTempLockDir(t)
	originalSpawn := spawnDetachedPlayback
	originalPlay := playFileFn
	foregroundCalled := false

	spawnDetachedPlayback = func(path string, lock *lockState, meta notify.Meta) error {
		t.Fatalf("detached spawner should not be called for fg mode: %s", path)
		return nil
	}
	playFileFn = func(ctx context.Context, path string) error {
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
	err := PlayAndSave(testWAVData(), outputPath, true, true, false, notify.Meta{Disabled: true})
	if err != nil {
		t.Fatalf("PlayAndSave() error = %v", err)
	}
	if !foregroundCalled {
		t.Fatal("expected foreground player to be called for fg mode")
	}
}

// fakeServer is a minimal notify.Server that invokes an action as soon as
// a notification with that action is shown.
type fakeServer struct {
	events  chan notify.Event
	onShow  func(id uint32, sp notify.Spec)
	shown   []notify.Spec
	closed  []uint32
	stopped bool
	up      chan struct{} // closed by the first Show
	upOnce  sync.Once
}

func newFakeServer() *fakeServer {
	return &fakeServer{events: make(chan notify.Event, 8), up: make(chan struct{})}
}

func (f *fakeServer) Show(replaceID uint32, sp notify.Spec) (uint32, error) {
	f.shown = append(f.shown, sp)
	f.upOnce.Do(func() { close(f.up) })
	id := replaceID
	if id == 0 {
		id = 7
	}
	if f.onShow != nil {
		f.onShow(id, sp)
	}
	return id, nil
}
func (f *fakeServer) Close(id uint32)             { f.closed = append(f.closed, id) }
func (f *fakeServer) Events() <-chan notify.Event { return f.events }
func (f *fakeServer) Markup() bool                { return true }
func (f *fakeServer) Shutdown()                   { f.stopped = true }

func TestDetachedPlaybackStopCancelsOnlyPlayback(t *testing.T) {
	originalPlay, originalConnect := playFileFn, connectNotify
	t.Cleanup(func() { playFileFn, connectNotify = originalPlay, originalConnect })

	srv := newFakeServer()
	srv.onShow = func(id uint32, sp notify.Spec) {
		if len(srv.shown) == 1 {
			srv.events <- notify.Event{ID: id, Action: notify.ActionStop}
		}
	}
	connectNotify = func() (notify.Server, error) { return srv, nil }
	playFileFn = func(ctx context.Context, path string) error {
		<-ctx.Done() // plays until Stop cancels it
		return ctx.Err()
	}

	released := 0
	meta := notify.Meta{Text: "hello <world>", Linger: 0}
	if err := playDetached("sample.wav", meta, func() { released++ }); err != nil {
		t.Fatalf("playDetached() error = %v", err)
	}
	if released != 1 {
		t.Fatalf("expected lock released once, got %d", released)
	}
	if got := srv.shown[0].Body; got != "hello &lt;world&gt;" {
		t.Fatalf("expected escaped full text in body, got %q", got)
	}
	if len(srv.closed) != 1 || !srv.stopped {
		t.Fatalf("expected notification closed and server shut down, closed=%v stopped=%v", srv.closed, srv.stopped)
	}
}

func TestForegroundPlaybackShowsNotificationWithoutLinger(t *testing.T) {
	useTempLockDir(t)
	originalPlay, originalConnect := playFileFn, connectNotify
	t.Cleanup(func() { playFileFn, connectNotify = originalPlay, originalConnect })

	srv := newFakeServer()
	connectNotify = func() (notify.Server, error) { return srv, nil }
	// Finish once the notification is up (it connects alongside playback).
	playFileFn = func(ctx context.Context, path string) error { <-srv.up; return nil }

	outputPath := filepath.Join(t.TempDir(), "out.wav")
	meta := notify.Meta{Text: "fg message", Linger: time.Hour}
	if err := PlayAndSave(testWAVData(), outputPath, true, true, false, meta); err != nil {
		t.Fatalf("PlayAndSave() error = %v", err)
	}
	if len(srv.shown) != 1 || srv.shown[0].Body != "fg message" {
		t.Fatalf("expected one playing notification with the message, got %+v", srv.shown)
	}
	if len(srv.closed) != 1 {
		t.Fatalf("expected fg notification closed at playback end, got %v", srv.closed)
	}
}

func TestSignalDuringPlaybackClosesNotification(t *testing.T) {
	originalPlay, originalConnect := playFileFn, connectNotify
	t.Cleanup(func() { playFileFn, connectNotify = originalPlay, originalConnect })

	srv := newFakeServer()
	connectNotify = func() (notify.Server, error) { return srv, nil }
	var playErr error
	playFileFn = func(ctx context.Context, path string) error {
		<-srv.up
		// Handled by runWithNotification, so this does not kill the test.
		syscall.Kill(os.Getpid(), syscall.SIGTERM)
		<-ctx.Done()
		playErr = ctx.Err()
		return playErr
	}

	released := 0
	err := runWithNotification("sample.wav", notify.Meta{Text: "m", Linger: time.Hour},
		func() { released++ }, reacquireLock)
	var intr *Interrupted
	if !errors.As(err, &intr) || intr.Signal != syscall.SIGTERM || intr.ExitCode() != 143 {
		t.Fatalf("err = %v, want *Interrupted{SIGTERM} (exit 143)", err)
	}
	if !errors.Is(playErr, context.Canceled) || released != 1 {
		t.Fatalf("playErr=%v released=%d", playErr, released)
	}
	if len(srv.shown) != 1 || len(srv.closed) != 1 || !srv.stopped {
		t.Fatalf("shown=%d closed=%v stopped=%v (no linger; notification closed)", len(srv.shown), srv.closed, srv.stopped)
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
