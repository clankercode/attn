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

// Deps are the side effects a Session drives. Play and Release are
// required; the rest may be nil.
type Deps struct {
	// Server is nil when no notification UI is available.
	Server Server
	// Play plays the audio once; it must return promptly when ctx is
	// cancelled (Stop).
	Play func(ctx context.Context) error
	// Release drops the playback lock held for the first play.
	Release func()
	// Reacquire takes the playback lock again for Replay.
	Reacquire func() (release func(), err error)
	// Copy puts text on the clipboard.
	Copy func(text string) error
	// After is a test seam for the linger timer (default time.After).
	After func(time.Duration) <-chan time.Time
}

// Run plays the message with its notification: Stop / Copy while speaking,
// then Replay / Copy for m.Linger before closing it. Playback errors are
// returned; a user Stop is not an error.
func Run(m Meta, d Deps) error {
	s := &session{m: m, d: d, release: d.Release}
	if s.d.After == nil {
		s.d.After = time.After
	}
	if m.Disabled {
		s.d.Server = nil
	}
	defer s.cleanup()
	return s.run()
}

type session struct {
	m       Meta
	d       Deps
	id      uint32
	gone    bool // server closed our notification; show no more UI
	dead    bool // event stream ended
	release func()
}

func (s *session) run() error {
	for {
		stopped, err := s.playOnce()
		if err != nil {
			return err
		}
		if s.d.Server == nil || s.gone || s.m.Linger <= 0 {
			return nil
		}
		phase := PhaseDone
		if stopped {
			phase = PhaseStopped
		}
		s.show(phase)
		if !s.linger() {
			return nil
		}
	}
}

// playOnce plays with the Playing notification up, handling Stop / Copy.
func (s *session) playOnce() (stopped bool, err error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.show(PhasePlaying)

	done := make(chan error, 1)
	go func() { done <- s.d.Play(ctx) }()

	for {
		select {
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

// linger waits for Replay (true) or for dismissal / timeout (false).
func (s *session) linger() bool {
	timeout := s.d.After(s.m.Linger)
	for {
		select {
		case <-timeout:
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
			case ev.Action == ActionReplay:
				if s.d.Reacquire == nil {
					continue
				}
				rel, err := s.d.Reacquire()
				if err != nil {
					// Another message is playing; stay in the lingering state.
					continue
				}
				s.release = rel
				return true
			}
		}
	}
}

func (s *session) show(p Phase) {
	if s.d.Server == nil || s.gone {
		return
	}
	id, err := s.d.Server.Show(s.id, Build(s.m, p, s.d.Server.Markup()))
	if err != nil {
		return
	}
	s.id = id
}

func (s *session) events() <-chan Event {
	if s.d.Server == nil || s.dead {
		return nil // blocks forever in select
	}
	return s.d.Server.Events()
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
	if s.d.Server == nil {
		return
	}
	if s.id != 0 && !s.gone {
		s.d.Server.Close(s.id)
	}
	s.d.Server.Shutdown()
}
