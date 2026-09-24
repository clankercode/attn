package notify

import (
	"context"
	"errors"
	"time"
)

// Event is a notification-server signal relevant to attn.
type Event struct {
	ID uint32
	// Action is the invoked action key; empty for a close event.
	Action string
	// Closed is true when the server closed the notification (dismissed,
	// expired, or closed by a call).
	Closed bool
}

// Server is the subset of org.freedesktop.Notifications attn uses.
type Server interface {
	// Show creates (replaceID 0) or replaces a notification.
	Show(replaceID uint32, s Spec) (uint32, error)
	Close(id uint32)
	Events() <-chan Event
	// Markup reports whether the server parses bodies as markup.
	Markup() bool
	Shutdown()
}

// ErrBusy is what Deps.Reacquire returns when other audio is playing; the
// notification then says so and keeps offering Replay.
var ErrBusy = errors.New("other audio is playing")

// Deps are the side effects a Session drives. Play is required; the rest
// may be nil.
type Deps struct {
	// Connect opens the notification server. It runs alongside the first
	// Play, so a slow or wedged server never delays audio. Nil, or an
	// error, means no notification UI.
	Connect func() (Server, error)
	// Play plays the audio once; it must return promptly when ctx is
	// cancelled (Stop, or Done).
	Play func(ctx context.Context) error
	// Release drops the playback lock held for the first play; nil when
	// the caller releases it.
	Release func()
	// Reacquire takes the playback lock again for Replay, failing with
	// ErrBusy while other audio plays.
	Reacquire func() (release func(), err error)
	// Copy puts text on the clipboard.
	Copy func(text string) error
	// Done ends the session early (e.g. on SIGTERM): playback is cancelled
	// and the notification closed. Nil never fires.
	Done <-chan struct{}
	// After is a test seam for the linger timer (default time.After).
	After func(time.Duration) <-chan time.Time
}

// Run plays the message with its notification: Stop / Copy while speaking,
// then Replay / Close / Copy text for m.Linger before closing it. Playback
// errors are returned; a user Stop, Close, or Done is not an error.
func Run(m Meta, d Deps) error {
	s := &session{m: m, d: d, release: d.Release}
	if s.d.After == nil {
		s.d.After = time.After
	}
	defer s.cleanup()
	return s.run()
}

type session struct {
	m   Meta
	d   Deps
	srv Server // nil until connected, or when there is no UI
	// pending delivers the server while Connect is still running.
	pending <-chan Server
	id      uint32
	gone    bool // server closed our notification; show no more UI
	broken  bool // a Show failed or timed out; show no more UI
	dead    bool // event stream ended
	release func()
}

func (s *session) run() error {
	s.connect()
	for {
		stopped, err := s.playOnce()
		if err != nil || s.interrupted() || s.m.Linger <= 0 {
			return err
		}
		if !s.awaitServer() || !s.ui() {
			return nil
		}
		phase := PhaseDone
		if stopped {
			phase = PhaseStopped
		}
		s.show(phase)
		// Nothing on screen to linger for.
		if !s.ui() || s.id == 0 {
			return nil
		}
		if !s.linger() {
			return nil
		}
	}
}

// connect starts opening the notification server in the background.
func (s *session) connect() {
	if s.m.Disabled || s.d.Connect == nil {
		return
	}
	c := make(chan Server, 1)
	s.pending = c
	go func(connect func() (Server, error)) {
		srv, err := connect()
		if err != nil {
			srv = nil
		}
		c <- srv
	}(s.d.Connect)
}

// connected records the outcome of connect.
func (s *session) connected(srv Server) {
	s.pending = nil
	s.srv = srv
}

// awaitServer waits for a connection still in progress (Connect bounds its
// own calls). It reports false if Done fired first.
func (s *session) awaitServer() bool {
	if s.pending == nil {
		return true
	}
	select {
	case srv := <-s.pending:
		s.connected(srv)
		return true
	case <-s.d.Done:
		return false
	}
}

func (s *session) interrupted() bool {
	select {
	case <-s.d.Done:
		return true
	default:
		return false
	}
}

// playOnce plays with the Playing notification up, handling Stop / Copy.
func (s *session) playOnce() (stopped bool, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the audio first: a slow or wedged notification server must
	// never delay playback (or the lock it holds).
	done := make(chan error, 1)
	go func() { done <- s.d.Play(ctx) }()
	s.show(PhasePlaying) // on Replay; the first play shows once connected

	for {
		select {
		case srv := <-s.pending:
			s.connected(srv)
			s.show(PhasePlaying)
		case <-s.d.Done:
			cancel()
			<-done
			s.releaseLock()
			return true, nil
		case err = <-done:
			s.releaseLock()
			if stopped || errors.Is(err, context.Canceled) {
				return true, nil
			}
			return false, err
		case ev, ok := <-s.events():
			if !ok {
				s.lost()
				continue
			}
			if !s.mine(ev) {
				continue
			}
			switch {
			case ev.Closed:
				// Dismissed mid-speech: keep playing, but no more UI.
				s.gone = true
			case ev.Action == ActionStop:
				stopped = true
				cancel()
			case ev.Action == ActionCopy:
				s.copy()
			}
		}
	}
}

// linger waits for Replay (true) or for Close / dismissal / timeout / Done (false).
func (s *session) linger() bool {
	timeout := s.d.After(s.m.Linger)
	for {
		select {
		case <-timeout:
			return false
		case <-s.d.Done:
			return false
		case ev, ok := <-s.events():
			if !ok {
				s.lost()
				return false
			}
			if !s.mine(ev) {
				continue
			}
			switch {
			case ev.Closed:
				s.gone = true
				return false
			case ev.Action == ActionCopy:
				s.copy()
			case ev.Action == ActionClose:
				return false
			case ev.Action == ActionReplay:
				if s.d.Reacquire == nil {
					continue
				}
				rel, err := s.d.Reacquire()
				if errors.Is(err, ErrBusy) {
					// Another message is playing: say so and keep lingering.
					s.show(PhaseBusy)
					if !s.ui() {
						return false
					}
				}
				if err != nil {
					continue
				}
				s.release = rel
				return true
			}
		}
	}
}

// ui reports whether the notification may still be shown or updated.
func (s *session) ui() bool { return s.srv != nil && !s.gone && !s.broken }

// show creates or updates the notification. A failure (including a
// timeout) means the server is unusable, so no further UI is attempted.
func (s *session) show(p Phase) {
	if !s.ui() {
		return
	}
	id, err := s.srv.Show(s.id, Build(s.m, p, s.srv.Markup()))
	if err != nil {
		s.broken = true
		return
	}
	s.id = id
}

func (s *session) events() <-chan Event {
	if s.srv == nil || s.dead {
		return nil // blocks forever in select
	}
	return s.srv.Events()
}

// lost handles the server connection going away: no more UI or events.
func (s *session) lost() {
	s.gone = true
	s.dead = true
}

func (s *session) mine(ev Event) bool { return s.id != 0 && ev.ID == s.id }

func (s *session) copy() {
	if s.d.Copy != nil {
		_ = s.d.Copy(s.m.Text)
	}
}

func (s *session) releaseLock() {
	if s.release != nil {
		s.release()
		s.release = nil
	}
}

func (s *session) cleanup() {
	s.releaseLock()
	if c := s.pending; c != nil {
		// Still connecting: nothing was shown; shut the server down
		// whenever it arrives.
		go func() {
			if srv := <-c; srv != nil {
				srv.Shutdown()
			}
		}()
	}
	if s.srv == nil {
		return
	}
	if s.id != 0 && !s.gone {
		s.srv.Close(s.id)
	}
	s.srv.Shutdown()
}
