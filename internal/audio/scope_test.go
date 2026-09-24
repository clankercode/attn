package audio

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestScopeArgvOnlyInsideSystemdUnit(t *testing.T) {
	orig := lookPathFn
	t.Cleanup(func() { lookPathFn = orig })
	lookPathFn = func(string) (string, error) { return "/usr/bin/systemd-run", nil }

	t.Setenv("INVOCATION_ID", "")
	if got := scopeArgv([]string{"/bin/attn", "/a.mp3"}); got != nil {
		t.Fatalf("outside a unit: want nil, got %v", got)
	}

	t.Setenv("INVOCATION_ID", "abc123")
	got := scopeArgv([]string{"/bin/attn", "/a.mp3"})
	want := "/usr/bin/systemd-run --user --scope --quiet --collect --description=attn playback -- /bin/attn /a.mp3"
	if strings.Join(got, " ") != want {
		t.Fatalf("inside a unit:\n got %q\nwant %q", strings.Join(got, " "), want)
	}

	lookPathFn = func(string) (string, error) { return "", errors.New("not found") }
	if got := scopeArgv([]string{"/bin/attn"}); got != nil {
		t.Fatalf("without systemd-run: want nil, got %v", got)
	}
}

func TestScopeStarted(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want bool
	}{
		{"fails fast", []string{"sh", "-c", "exit 1"}, false},
		{"finishes cleanly", []string{"true"}, true},
		{"still running", []string{"sleep", "5"}, true},
	} {
		cmd := exec.Command(tc.args[0], tc.args[1:]...)
		if err := cmd.Start(); err != nil {
			t.Fatalf("%s: start: %v", tc.name, err)
		}
		if got := scopeStarted(cmd, 300*time.Millisecond); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
		if cmd.ProcessState == nil {
			_ = cmd.Process.Kill()
		}
	}
}
