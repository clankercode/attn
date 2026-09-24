//go:build linux

package audio

import "golang.org/x/sys/unix"

// closeInheritedFDsOnExec marks every fd >= 3 close-on-exec so children
// only get what os/exec passes explicitly. Best-effort: kernels before 5.11
// lack CLOSE_RANGE_CLOEXEC.
func closeInheritedFDsOnExec() {
	_ = unix.CloseRange(3, ^uint(0), unix.CLOSE_RANGE_CLOEXEC)
}
