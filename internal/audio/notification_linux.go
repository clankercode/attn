//go:build linux

package audio

import (
	"sync"
	"sync/atomic"

	"github.com/godbus/dbus/v5"
)

const (
	notificationsBusName = "org.freedesktop.Notifications"
	notificationsPath    = "/org/freedesktop/Notifications"
	silenceActionID      = "silence"
)

func startSilenceNotificationImpl(onSilence func()) func() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return func() {}
	}

	matchRule := "type='signal',sender='org.freedesktop.Notifications',interface='org.freedesktop.Notifications',member='ActionInvoked'"
	if err := conn.BusObject().Call("org.freedesktop.DBus.AddMatch", 0, matchRule).Err; err != nil {
		conn.Close()
		return func() {}
	}

	signals := make(chan *dbus.Signal, 1)
	conn.Signal(signals)

	var notificationID uint32
	call := conn.Object(notificationsBusName, dbus.ObjectPath(notificationsPath)).Call(
		notificationsBusName+".Notify",
		0,
		"attn",
		uint32(0),
		"audio-volume-high",
		"attn is playing",
		"Click Silence to stop this background audio.",
		[]string{silenceActionID, "Silence"},
		map[string]dbus.Variant{},
		int32(-1),
	)
	if err := call.Store(&notificationID); err != nil {
		conn.RemoveSignal(signals)
		conn.Close()
		return func() {}
	}

	var active atomic.Bool
	active.Store(true)
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		for {
			select {
			case signal := <-signals:
				if signal == nil || signal.Name != notificationsBusName+".ActionInvoked" || len(signal.Body) != 2 {
					continue
				}
				id, idOK := signal.Body[0].(uint32)
				action, actionOK := signal.Body[1].(string)
				if idOK && actionOK && id == notificationID && action == silenceActionID && active.Load() {
					onSilence()
				}
			case <-done:
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			active.Store(false)
			close(done)
			conn.RemoveSignal(signals)
			conn.Object(notificationsBusName, dbus.ObjectPath(notificationsPath)).Call(
				notificationsBusName+".CloseNotification", 0, notificationID,
			)
			conn.Close()
		})
	}
}
