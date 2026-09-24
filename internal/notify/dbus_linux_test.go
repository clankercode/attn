//go:build linux

package notify

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestToEvent(t *testing.T) {
	cases := []struct {
		sig  *dbus.Signal
		want Event
		ok   bool
	}{
		{&dbus.Signal{Name: busName + ".ActionInvoked", Body: []any{uint32(5), "copy"}}, Event{ID: 5, Action: "copy"}, true},
		{&dbus.Signal{Name: busName + ".NotificationClosed", Body: []any{uint32(5), uint32(2)}}, Event{ID: 5, Closed: true}, true},
		{&dbus.Signal{Name: busName + ".ActionInvoked", Body: []any{"x", "copy"}}, Event{}, false},
		{&dbus.Signal{Name: "other.Signal", Body: []any{uint32(5), "copy"}}, Event{}, false},
		{nil, Event{}, false},
	}
	for i, tc := range cases {
		got, ok := toEvent(tc.sig)
		if got != tc.want || ok != tc.ok {
			t.Errorf("case %d: got (%+v, %v) want (%+v, %v)", i, got, ok, tc.want, tc.ok)
		}
	}
}
