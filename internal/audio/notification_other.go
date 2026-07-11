//go:build !linux

package audio

func startSilenceNotificationImpl(func()) func() {
	return func() {}
}
