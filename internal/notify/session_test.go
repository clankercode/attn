package notify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeServer struct {
	events   chan Event
	onShow   func(id uint32, s Spec)
	shown    []Spec
	closed   []uint32
	shutdown bool
}

func newFake() *fakeServer { return &fakeServer{events: make(chan Event, 16)} }

func (f *fakeServer) Show(replaceID uint32, s Spec) (uint32, error) {
	f.shown = append(f.shown, s)
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
		Server:  srv,
		Play:    blockUntilCancel,
		Release: func() { released++ },
		After:   lingerNow,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := srv.summaries(); len(got) != 2 || got[1] != "p (stopped)" {
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
		Server: srv, Play: blockUntilCancel, Release: func() {},
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
		if s.Actions[0] == ActionReplay && len(srv.shown) == 2 {
			srv.events <- Event{ID: id, Action: ActionReplay}
		}
		if s.Actions[0] == ActionReplay && len(srv.shown) == 4 {
			srv.events <- Event{ID: id, Closed: true}
		}
	}
	plays, reacquired, released := 0, 0, 0
	err := Run(Meta{Text: "m", Linger: time.Minute}, Deps{
		Server:  srv,
		Play:    func(context.Context) error { plays++; return nil },
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
		Server:  srv,
		Play:    func(context.Context) error { plays++; return nil },
		Release: func() {},
		Reacquire: func() (func(), error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("busy")
			}
			return func() {}, nil
		},
		After: lingerNever,
	})
	if err != nil || plays != 2 || attempts != 2 {
		t.Fatalf("err=%v plays=%d attempts=%d", err, plays, attempts)
	}
	got := srv.summaries()
	want := []string{"🔊 p", "p", "p (busy, try Replay again)", "🔊 p", "p"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %q, want %q", got, want)
	}
	if busy := srv.shown[2]; busy.Actions[0] != ActionReplay || busy.Actions[2] != ActionCopy {
		t.Fatalf("busy actions = %v", busy.Actions)
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
			Server:  srv,
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
			Server: srv, Play: func(context.Context) error { return nil },
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
		Server: srv,
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
		Server: srv, Play: func(context.Context) error { return boom }, Release: func() {},
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
			Server: tc.srv, Play: func(context.Context) error { plays++; return nil },
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
			Server: srv, Play: func(context.Context) error { time.Sleep(10 * time.Millisecond); return nil },
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
