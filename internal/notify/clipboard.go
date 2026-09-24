package notify

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	lookPath = exec.LookPath
	runCopy  = func(ctx context.Context, name string, args []string, text string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.Stdin = strings.NewReader(text)
		// Stdout/stderr stay nil (/dev/null): wl-copy and xclip fork a
		// clipboard server that would otherwise hold our pipes open.
		return cmd.Run()
	}
)

// CopyText puts text on the desktop clipboard via wl-copy (Wayland) or
// xclip (X11).
func CopyText(text string) error {
	name, args, err := clipboardCommand()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return runCopy(ctx, name, args, text)
}

func clipboardCommand() (string, []string, error) {
	if waylandAvailable() {
		if p, err := lookPath("wl-copy"); err == nil {
			return p, nil, nil
		}
	}
	if os.Getenv("DISPLAY") != "" {
		if p, err := lookPath("xclip"); err == nil {
			return p, []string{"-selection", "clipboard"}, nil
		}
	}
	return "", nil, errors.New("no clipboard tool (wl-copy or xclip) available")
}

// waylandAvailable is true when WAYLAND_DISPLAY is set or the default
// wayland-0 socket exists (agent shells often lack the env var).
func waylandAvailable() bool {
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return true
	}
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, "wayland-0"))
	return err == nil
}
