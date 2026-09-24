//go:build !linux

package notify

import "errors"

// Connect is unavailable off Linux; playback runs without a notification.
func Connect() (Server, error) {
	return nil, errors.New("desktop notifications unsupported on this platform")
}
