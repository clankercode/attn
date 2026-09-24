//go:build linux

package notify

import (
	"context"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	busName = "org.freedesktop.Notifications"
	objPath = dbus.ObjectPath("/org/freedesktop/Notifications")

	// callTimeout bounds each call to the notification server, so a wedged
	// server (e.g. a hung plasmashell) cannot stall playback or the caller.
	callTimeout = 2 * time.Second
)

type dbusServer struct {
	conn    *dbus.Conn
	obj     dbus.BusObject
	markup  bool
	signals chan *dbus.Signal
	events  chan Event
	done    chan struct{}
	once    sync.Once
}

// Connect opens the session-bus notification server. It fails on headless
// hosts; callers then run without a notification.
func Connect() (Server, error) {
	// Without an address godbus falls back to autolaunching a bus daemon;
	// only allow the standard per-user socket.
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		if _, err := os.Stat("/run/user/" + strconv.Itoa(os.Getuid()) + "/bus"); err != nil {
			return nil, err
		}
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	for _, member := range []string{"ActionInvoked", "NotificationClosed"} {
		if err := conn.AddMatchSignal(
			dbus.WithMatchSender(busName),
			dbus.WithMatchInterface(busName),
			dbus.WithMatchMember(member),
		); err != nil {
			conn.Close()
			return nil, err
		}
	}
	s := &dbusServer{
		conn:    conn,
		obj:     conn.Object(busName, objPath),
		signals: make(chan *dbus.Signal, 16),
		events:  make(chan Event, 16),
		done:    make(chan struct{}),
	}
	// Also a liveness probe: no answer means no usable notification server.
	var caps []string
	if err := s.call(busName + ".GetCapabilities").Store(&caps); err != nil {
		conn.Close()
		return nil, err
	}
	for _, c := range caps {
		if c == "body-markup" {
			s.markup = true
		}
	}
	conn.Signal(s.signals)
	go s.pump()
	return s, nil
}

func (s *dbusServer) pump() {
	defer close(s.events)
	for {
		select {
		case <-s.done:
			return
		case sig, ok := <-s.signals:
			if !ok {
				return
			}
			ev, ok := toEvent(sig)
			if !ok {
				continue
			}
			select {
			case s.events <- ev:
			case <-s.done:
				return
			}
		}
	}
}

func toEvent(sig *dbus.Signal) (Event, bool) {
	if sig == nil || len(sig.Body) < 2 {
		return Event{}, false
	}
	id, ok := sig.Body[0].(uint32)
	if !ok {
		return Event{}, false
	}
	switch sig.Name {
	case busName + ".ActionInvoked":
		action, ok := sig.Body[1].(string)
		return Event{ID: id, Action: action}, ok
	case busName + ".NotificationClosed":
		return Event{ID: id, Closed: true}, true
	}
	return Event{}, false
}

func (s *dbusServer) Show(replaceID uint32, sp Spec) (uint32, error) {
	hints := make(map[string]dbus.Variant, len(sp.Hints))
	for k, v := range sp.Hints {
		hints[k] = dbus.MakeVariant(v)
	}
	var id uint32
	err := s.call(busName+".Notify",
		sp.AppName, replaceID, sp.Icon, sp.Summary, sp.Body,
		sp.Actions, hints, sp.Timeout,
	).Store(&id)
	return id, err
}

// call makes a method call bounded by callTimeout.
func (s *dbusServer) call(method string, args ...any) *dbus.Call {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	return s.obj.CallWithContext(ctx, method, 0, args...)
}

// Close is fire-and-forget: the reply carries nothing we need.
func (s *dbusServer) Close(id uint32) {
	s.obj.Call(busName+".CloseNotification", dbus.FlagNoReplyExpected, id)
}

func (s *dbusServer) Events() <-chan Event { return s.events }
func (s *dbusServer) Markup() bool         { return s.markup }

func (s *dbusServer) Shutdown() {
	s.once.Do(func() {
		close(s.done)
		s.conn.RemoveSignal(s.signals)
		s.conn.Close()
	})
}
