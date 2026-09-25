package notify

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeServer struct {
	events   chan Event
	onShow   func(id uint32, s Spec)
	shown    []Spec
	closed   []uint32
	shutdown bool
	up       chan struct{} // closed by the first Show
	upOnce   sync.Once
}

func newFake() *fakeServer {
	return &fakeServer{events: make(chan Event, 16), up: make(chan struct{})}
}

func (f *fakeServer) Show(replaceID uint32, s Spec) (uint32, error) {
	f.shown = append(f.shown, s)
	f.upOnce.Do(func() { close(f.up) })
	id := replaceID
	if id == 0 {
		id = 42
	}
	if f.onShow != nil {
		f.onShow(id, s)
	}
	return id, nil
}
func (f *fakeServer) Close(id uint32)      { f.closed = append(f.closed, id) }
func (f *fakeServer) Events() <-chan Event { return f.events }
func (f *fakeServer) Markup() bool         { return true }
func (f *fakeServer) Shutdown()            { f.shutdown = true }

func (f *fakeServer) summaries() []string {
	var out []string
	for _, s := range f.shown {
		out = append(out, s.Summary)
	}
	return out
}

// lingerNever never fires; lingerNow fires immediately.
func lingerNever(time.Duration) <-chan time.Time { return nil }
func lingerNow(time.Duration) <-chan time.Time {
	c := make(chan time.Time, 1)
	c <- time.Now()
	return c
}

func blockUntilCancel(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }

// serve is a Connect that returns srv at once.
func serve(srv Server) func() (Server, error) {
	return func() (Server, error) { return srv, nil }
}

// playShown is a Play that finishes (with err) once the notification is up:
// Connect runs alongside Play, so an instant Play would race it.
func (f *fakeServer) playShown(err error) func(context.Context) error {
	return func(ctx context.Context) error {
		select {
		case <-f.up:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestStopThenLingerTimeoutClosesNotification(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		if len(srv.shown) == 1 {
			srv.events <- Event{ID: 999, Action: ActionStop} // someone else's
			srv.events <- Event{ID: id, Action: ActionStop}
		}
	}
	released := 0
	err := Run(Meta{Text: "m", Project: "p", Linger: time.Minute}, Deps{
		Connect: serve(srv),
		Play:    blockUntilCancel,
		Release: func() { released++ },
		After:   lingerNow,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := srv.summaries(); len(got) != 2 || got[1] != "Stopped — p" {
		t.Fatalf("summaries = %v", got)
	}
	if released != 1 || len(srv.closed) != 1 || srv.closed[0] != 42 || !srv.shutdown {
		t.Fatalf("released=%d closed=%v shutdown=%v", released, srv.closed, srv.shutdown)
	}
}

func TestCopyDuringPlaybackAndAfter(t *testing.T) {
	srv := newFake()
	var copied []string
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 1:
			srv.events <- Event{ID: id, Action: ActionCopy}
			srv.events <- Event{ID: id, Action: ActionStop}
		case 2:
			srv.events <- Event{ID: id, Action: ActionCopy}
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	err := Run(Meta{Text: "the full text", Linger: time.Minute}, Deps{
		Connect: serve(srv), Play: blockUntilCancel, Release: func() {},
		Copy:  func(s string) error { copied = append(copied, s); return nil },
		After: lingerNever,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(copied) != 2 || copied[0] != "the full text" {
		t.Fatalf("copied = %v", copied)
	}
	if len(srv.closed) != 0 {
		t.Fatalf("user-closed notification must not be closed again: %v", srv.closed)
	}
}

func TestReplayReacquiresLockAndPlaysAgain(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 2: // finished: replay
			srv.events <- Event{ID: id, Action: ActionReplay}
		case 4: // finished again: dismiss
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	plays, reacquired, released := 0, 0, 0
	err := Run(Meta{Text: "m", Linger: time.Minute}, Deps{
		Connect: serve(srv),
		Play:    func(ctx context.Context) error { plays++; return srv.playShown(nil)(ctx) },
		Release: func() { released++ },
		Reacquire: func() (func(), error) {
			reacquired++
			return func() { released++ }, nil
		},
		After: lingerNever,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if plays != 2 || reacquired != 1 || released != 2 {
		t.Fatalf("plays=%d reacquired=%d released=%d", plays, reacquired, released)
	}
	if got := srv.summaries(); len(got) != 4 {
		t.Fatalf("expected playing, done, playing, done; got %v", got)
	}
}

func TestReplayBusyShowsBusyAndKeepsLingering(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 2, 3: // done, then busy: press Replay each time
			srv.events <- Event{ID: id, Action: ActionReplay}
		case 5: // done after the replay
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	plays, attempts := 0, 0
	err := Run(Meta{Text: "m", Project: "p", Linger: time.Minute}, Deps{
		Connect: serve(srv),
		Play:    func(ctx context.Context) error { plays++; return srv.playShown(nil)(ctx) },
		Release: func() {},
		Reacquire: func() (func(), error) {
			attempts++
			if attempts == 1 {
				return nil, ErrBusy
			}
			return func() {}, nil
		},
		After: lingerNever,
	})
	if err != nil || plays != 2 || attempts != 2 {
		t.Fatalf("err=%v plays=%d attempts=%d", err, plays, attempts)
	}
	got := srv.summaries()
	want := []string{"Speaking — p", "Finished — p", "Busy — p (try Replay again)", "Speaking — p", "Finished — p"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
	if busy := srv.shown[2]; strings.Join(busy.Actions, ",") != strings.Join(lingerActions(), ",") {
		t.Fatalf("busy actions = %v", busy.Actions)
	}
}

func TestCloseDuringLingerEndsSession(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		if len(srv.shown) == 2 {
			srv.events <- Event{ID: id, Action: ActionClose}
		}
	}
	finished := make(chan error, 1)
	go func() {
		finished <- Run(Meta{Text: "m", Project: "p", Linger: time.Hour}, Deps{
			Connect: serve(srv),
			Play:    srv.playShown(nil),
			Release: func() {},
			After:   lingerNever,
		})
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close must end the linger instead of waiting out the timer")
	}
	if len(srv.shown) != 2 || srv.shown[1].Summary != "Finished — p" {
		t.Fatalf("summaries = %v", srv.summaries())
	}
	if strings.Join(srv.shown[1].Actions, ",") != strings.Join(lingerActions(), ",") {
		t.Fatalf("linger actions = %v", srv.shown[1].Actions)
	}
	if len(srv.closed) != 1 || srv.closed[0] != 42 || !srv.shutdown {
		t.Fatalf("closed=%v shutdown=%v", srv.closed, srv.shutdown)
	}
}

func TestCloseAfterStopEndsSession(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 1:
			srv.events <- Event{ID: id, Action: ActionStop}
		case 2:
			srv.events <- Event{ID: id, Action: ActionClose}
		}
	}
	finished := make(chan error, 1)
	go func() {
		finished <- Run(Meta{Text: "m", Project: "p", Linger: time.Hour}, Deps{
			Connect: serve(srv),
			Play:    blockUntilCancel,
			Release: func() {},
			After:   lingerNever,
		})
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close after Stop must end the linger")
	}
	if got := srv.summaries(); len(got) != 2 || got[1] != "Stopped — p" {
		t.Fatalf("summaries = %v", got)
	}
	if strings.Join(srv.shown[1].Actions, ",") != strings.Join(lingerActions(), ",") {
		t.Fatalf("stopped actions = %v", srv.shown[1].Actions)
	}
	if len(srv.closed) != 1 || !srv.shutdown {
		t.Fatalf("closed=%v shutdown=%v", srv.closed, srv.shutdown)
	}
}

// failingServer's Show waits (bounded) for playback to start, then fails,
// like a wedged notification server hitting the call timeout.
type failingServer struct {
	*fakeServer
	playing chan struct{}
	sawPlay bool
}

func (f *failingServer) Show(uint32, Spec) (uint32, error) {
	f.shown = append(f.shown, Spec{})
	select {
	case <-f.playing:
		f.sawPlay = true
	case <-time.After(2 * time.Second):
	}
	return 0, errors.New("notify: timeout")
}

func TestShowFailureStillPlaysAndDoesNotLinger(t *testing.T) {
	srv := &failingServer{fakeServer: newFake(), playing: make(chan struct{})}
	released := 0
	done := make(chan error, 1)
	go func() {
		done <- Run(Meta{Text: "m", Linger: time.Hour}, Deps{
			Connect: serve(srv),
			Play:    func(context.Context) error { close(srv.playing); return nil },
			Release: func() { released++ },
			After:   lingerNever,
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run lingered with no notification on screen")
	}
	if !srv.sawPlay {
		t.Fatal("playback must start before the notification is shown")
	}
	if len(srv.shown) != 1 || len(srv.closed) != 0 || !srv.shutdown || released != 1 {
		t.Fatalf("shown=%d closed=%v shutdown=%v released=%d", len(srv.shown), srv.closed, srv.shutdown, released)
	}
}

func TestDoneShowFailureClosesAndDoesNotLinger(t *testing.T) {
	srv := &erroringServer{fakeServer: newFake(), failFrom: 2}
	done := make(chan error, 1)
	go func() {
		done <- Run(Meta{Text: "m", Linger: time.Hour}, Deps{
			Connect: serve(srv), Play: srv.playShown(nil),
			Release: func() {}, After: lingerNever,
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run lingered after the notification update failed")
	}
	// The playing notification (id 42) is still up; it must be closed.
	if len(srv.closed) != 1 || srv.closed[0] != 42 {
		t.Fatalf("closed = %v", srv.closed)
	}
}

// erroringServer fails the failFrom'th and later Show calls.
type erroringServer struct {
	*fakeServer
	failFrom int
}

func (e *erroringServer) Show(replaceID uint32, s Spec) (uint32, error) {
	id, _ := e.fakeServer.Show(replaceID, s)
	if len(e.shown) >= e.failFrom {
		return 0, errors.New("notify: timeout")
	}
	return id, nil
}

func TestDismissDuringPlaybackKeepsPlayingWithoutMoreUI(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) { srv.events <- Event{ID: id, Closed: true} }
	finish := make(chan struct{})
	played := false
	go func() { time.Sleep(20 * time.Millisecond); close(finish) }()
	err := Run(Meta{Text: "m", Linger: time.Minute}, Deps{
		Connect: serve(srv),
		Play: func(ctx context.Context) error {
			select {
			case <-finish:
				played = true
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		Release: func() {},
		After:   lingerNever,
	})
	if err != nil || !played {
		t.Fatalf("err=%v played=%v (dismiss must not stop audio)", err, played)
	}
	if len(srv.shown) != 1 || len(srv.closed) != 0 {
		t.Fatalf("shown=%d closed=%v", len(srv.shown), srv.closed)
	}
}

func TestPlayErrorIsReturnedAndNotificationClosed(t *testing.T) {
	srv := newFake()
	boom := errors.New("no sink")
	err := Run(Meta{Text: "m", Linger: time.Minute}, Deps{
		Connect: serve(srv), Play: srv.playShown(boom), Release: func() {},
		After: lingerNever,
	})
	if !errors.Is(err, boom) || len(srv.closed) != 1 {
		t.Fatalf("err=%v closed=%v", err, srv.closed)
	}
}

func TestNoServerOrDisabledJustPlays(t *testing.T) {
	for _, tc := range []struct {
		name string
		srv  Server
		meta Meta
	}{
		{"nil server", nil, Meta{Text: "m", Linger: time.Minute}},
		{"disabled", newFake(), Meta{Text: "m", Linger: time.Minute, Disabled: true}},
	} {
		plays := 0
		err := Run(tc.meta, Deps{
			Connect: serve(tc.srv), Play: func(context.Context) error { plays++; return nil },
			Release: func() {}, After: lingerNever,
		})
		if err != nil || plays != 1 {
			t.Fatalf("%s: err=%v plays=%d", tc.name, err, plays)
		}
		if f, ok := tc.srv.(*fakeServer); ok && len(f.shown) != 0 {
			t.Fatalf("%s: disabled must show nothing", tc.name)
		}
	}
}

func TestEventStreamEndingDoesNotSpin(t *testing.T) {
	srv := newFake()
	close(srv.events)
	done := make(chan error, 1)
	go func() {
		done <- Run(Meta{Text: "m", Linger: time.Minute}, Deps{
			Connect: serve(srv), Play: func(context.Context) error { time.Sleep(10 * time.Millisecond); return nil },
			Release: func() {}, After: lingerNever,
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run hung after the event stream closed")
	}
}

func TestReplayOtherErrorKeepsLingeringWithoutBusy(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		if len(srv.shown) == 2 { // done: Replay fails, then dismiss
			srv.events <- Event{ID: id, Action: ActionReplay}
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	plays := 0
	err := Run(Meta{Text: "m", Project: "p", Linger: time.Minute}, Deps{
		Connect:   serve(srv),
		Play:      func(ctx context.Context) error { plays++; return srv.playShown(nil)(ctx) },
		Reacquire: func() (func(), error) { return nil, errors.New("lock: permission denied") },
		After:     lingerNever,
	})
	if err != nil || plays != 1 {
		t.Fatalf("err=%v plays=%d", err, plays)
	}
	if got := srv.summaries(); strings.Join(got, "|") != "Speaking — p|Finished — p" {
		t.Fatalf("summaries = %q (a non-busy error must not claim busy)", got)
	}
}

func TestDoneDuringPlayCancelsPlaybackAndCloses(t *testing.T) {
	srv := newFake()
	done := make(chan struct{})
	srv.onShow = func(uint32, Spec) { close(done) } // signal while speaking
	released := 0
	var playErr error
	err := Run(Meta{Text: "m", Linger: time.Minute}, Deps{
		Connect: serve(srv),
		Play:    func(ctx context.Context) error { playErr = blockUntilCancel(ctx); return playErr },
		Release: func() { released++ },
		Done:    done,
		After:   lingerNever,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !errors.Is(playErr, context.Canceled) {
		t.Fatalf("play not cancelled: %v", playErr)
	}
	if len(srv.shown) != 1 || released != 1 {
		t.Fatalf("shown=%q released=%d (no linger after Done)", srv.summaries(), released)
	}
	if len(srv.closed) != 1 || srv.closed[0] != 42 || !srv.shutdown {
		t.Fatalf("closed=%v shutdown=%v", srv.closed, srv.shutdown)
	}
}

func TestDoneDuringLingerCloses(t *testing.T) {
	srv := newFake()
	done := make(chan struct{})
	srv.onShow = func(id uint32, s Spec) {
		if len(srv.shown) == 2 {
			close(done)
		}
	}
	finished := make(chan error, 1)
	go func() {
		finished <- Run(Meta{Text: "m", Linger: time.Hour}, Deps{
			Connect: serve(srv), Play: srv.playShown(nil),
			Done: done, After: lingerNever,
		})
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run kept lingering after Done")
	}
	if len(srv.closed) != 1 || srv.closed[0] != 42 || !srv.shutdown {
		t.Fatalf("closed=%v shutdown=%v", srv.closed, srv.shutdown)
	}
}

func TestSlowConnectDoesNotDelayPlayback(t *testing.T) {
	srv := newFake()
	started := make(chan struct{})
	sawPlay := false
	connect := func() (Server, error) {
		select {
		case <-started:
			sawPlay = true
		case <-time.After(2 * time.Second):
		}
		return srv, nil
	}
	err := Run(Meta{Text: "m"}, Deps{
		Connect: connect,
		Play: func(ctx context.Context) error {
			close(started)
			return srv.playShown(nil)(ctx)
		},
	})
	if err != nil || !sawPlay {
		t.Fatalf("err=%v sawPlay=%v (playback must start while connecting)", err, sawPlay)
	}
	if len(srv.shown) != 1 || len(srv.closed) != 1 {
		t.Fatalf("shown=%d closed=%v", len(srv.shown), srv.closed)
	}
}

func TestConnectStillPendingWhenPlaybackEnds(t *testing.T) {
	srv := newFake()
	unblock := make(chan struct{})
	shutdown := make(chan struct{})
	connect := func() (Server, error) {
		<-unblock
		return &shutdownServer{fakeServer: srv, shutdown: shutdown}, nil
	}
	start := time.Now()
	err := Run(Meta{Text: "m"}, Deps{Connect: connect, Play: func(context.Context) error { return nil }})
	if err != nil || time.Since(start) > time.Second {
		t.Fatalf("err=%v after %v: Run must not wait for the server without linger", err, time.Since(start))
	}
	close(unblock)
	select {
	case <-shutdown:
	case <-time.After(2 * time.Second):
		t.Fatal("late server was never shut down")
	}
	if len(srv.shown) != 0 {
		t.Fatalf("nothing may be shown after playback: %q", srv.summaries())
	}
}

// shutdownServer reports Shutdown on a channel.
type shutdownServer struct {
	*fakeServer
	shutdown chan struct{}
}

func (s *shutdownServer) Shutdown() { close(s.shutdown) }

// shortCopyNote makes the Copied / Copy failed title revert quickly.
func shortCopyNote(t *testing.T) {
	t.Helper()
	orig := copyNoteTime
	copyNoteTime = 10 * time.Millisecond
	t.Cleanup(func() { copyNoteTime = orig })
}

func TestDefaultActionCopiesDuringPlayback(t *testing.T) {
	shortCopyNote(t)
	srv := newFake()
	var copied []string
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 1: // speaking: click the body
			srv.events <- Event{ID: id, Action: ActionDefault}
		case 3: // title restored: stop
			srv.events <- Event{ID: id, Action: ActionStop}
		case 4: // finished: dismiss
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	err := Run(Meta{Text: "the text", Project: "p", Linger: time.Hour}, Deps{
		Connect: serve(srv), Play: blockUntilCancel, Release: func() {},
		Copy:  func(s string) error { copied = append(copied, s); return nil },
		After: lingerNever,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(copied) != 1 || copied[0] != "the text" {
		t.Fatalf("copied = %v", copied)
	}
	want := []string{"Speaking — p", "Copied", "Speaking — p", "Stopped — p"}
	if got := srv.summaries(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
}

func TestCopyNoteRevertsToPhaseTitle(t *testing.T) {
	shortCopyNote(t)
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 2: // finished: copy
			srv.events <- Event{ID: id, Action: ActionCopy}
		case 4: // title restored: dismiss
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	err := Run(Meta{Text: "m", Project: "p", Linger: time.Hour}, Deps{
		Connect: serve(srv), Play: srv.playShown(nil),
		Copy:  func(string) error { return nil },
		After: lingerNever,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"Speaking — p", "Finished — p", "Copied", "Finished — p"}
	if got := srv.summaries(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
}

func TestCopyFailureNotesCopyFailed(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 2: // finished: copy fails
			srv.events <- Event{ID: id, Action: ActionCopy}
		case 3: // failure note: dismiss
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	err := Run(Meta{Text: "m", Project: "p", Linger: time.Hour}, Deps{
		Connect: serve(srv), Play: srv.playShown(nil),
		Copy:  func(string) error { return errors.New("no clipboard tool") },
		After: lingerNever,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	want := []string{"Speaking — p", "Finished — p", "Copy failed"}
	if got := srv.summaries(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
}

func TestRunSkippedShowsPopupAndCloseEnds(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		if len(srv.shown) == 1 {
			srv.events <- Event{ID: id, Action: ActionClose}
		}
	}
	err := RunSkipped(Meta{Text: "m", Project: "p", Linger: time.Minute}, Deps{
		Connect:   serve(srv),
		Reacquire: func() (func(), error) { return nil, ErrBusy },
		After:     lingerNever,
	})
	if err != nil {
		t.Fatalf("RunSkipped: %v", err)
	}
	if got := srv.summaries(); strings.Join(got, "|") != "Skipped — p" {
		t.Fatalf("summaries = %q", got)
	}
	if len(srv.closed) != 1 || !srv.shutdown {
		t.Fatalf("closed=%v shutdown=%v", srv.closed, srv.shutdown)
	}
}

func TestRunSkippedWithoutLingerShowsNothing(t *testing.T) {
	srv := newFake()
	err := RunSkipped(Meta{Text: "m", Linger: 0}, Deps{Connect: serve(srv)})
	if err != nil || len(srv.shown) != 0 {
		t.Fatalf("err=%v shown=%q", err, srv.summaries())
	}
}

func TestRunSkippedReplayBusyKeepsOffering(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 1: // skipped: replay is refused
			srv.events <- Event{ID: id, Action: ActionReplay}
		case 2: // busy: close
			srv.events <- Event{ID: id, Action: ActionClose}
		}
	}
	attempts := 0
	err := RunSkipped(Meta{Text: "m", Project: "p", Linger: time.Minute}, Deps{
		Connect: serve(srv),
		Reacquire: func() (func(), error) {
			attempts++
			return nil, ErrBusy
		},
		After: lingerNever,
	})
	if err != nil || attempts != 1 {
		t.Fatalf("err=%v attempts=%d", err, attempts)
	}
	want := []string{"Skipped — p", "Busy — p (try Replay again)"}
	if got := srv.summaries(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
}

func TestRunSkippedReplayPlaysWhenFree(t *testing.T) {
	srv := newFake()
	srv.onShow = func(id uint32, s Spec) {
		switch len(srv.shown) {
		case 1: // skipped: replay goes ahead
			srv.events <- Event{ID: id, Action: ActionReplay}
		case 3: // finished after the replay: dismiss
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	plays := 0
	err := RunSkipped(Meta{Text: "m", Project: "p", Linger: time.Minute}, Deps{
		Connect:   serve(srv),
		Play:      func(ctx context.Context) error { plays++; return srv.playShown(nil)(ctx) },
		Reacquire: func() (func(), error) { return func() {}, nil },
		After:     lingerNever,
	})
	if err != nil || plays != 1 {
		t.Fatalf("err=%v plays=%d", err, plays)
	}
	want := []string{"Skipped — p", "Speaking — p", "Finished — p"}
	if got := srv.summaries(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
}
