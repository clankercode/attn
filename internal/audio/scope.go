package audio

import (
	"os"
	"os/exec"
	"time"
)

// Test seams.
var (
	lookPathFn     = exec.LookPath
	scopeProbeWait = 500 * time.Millisecond
)

// scopeArgv wraps argv in `systemd-run --user --scope` when attn runs inside
// a systemd unit (INVOCATION_ID is set), else returns nil.
//
// When a unit's main process exits (e.g. a Type=oneshot timer script that
// calls attn), systemd kills everything left in the unit's cgroup, which
// would cut off the detached playback and close its notification within
// milliseconds. A transient scope of its own lets playback and the lingering
// notification outlive the unit. systemd-run --scope execs argv in place, so
// the PID, environment and inherited lock fd are preserved.
func scopeArgv(argv []string) []string {
	if os.Getenv("INVOCATION_ID") == "" {
		return nil
	}
	run, err := lookPathFn("systemd-run")
	if err != nil {
		return nil
	}
	return append([]string{
		run, "--user", "--scope", "--quiet", "--collect",
		"--description=attn playback", "--",
	}, argv...)
}

// scopeStarted reports whether a started `systemd-run --scope` command got
// going: false if it exits non-zero within wait (no user manager, refused
// scope), true if it is still running or finished cleanly.
func scopeStarted(cmd *exec.Cmd, wait time.Duration) bool {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err == nil
	case <-time.After(wait):
		return true
	}
}
