package audio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func PlayAndSave(data []byte, outputPath string, doPlay bool) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	if doPlay {
		return PlayMpv(outputPath)
	}
	return nil
}

func PlayMpv(path string) error {
	cmd := exec.Command("mpv", "--quiet", "--no-terminal", path)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd.Run()
}

func ConcatWAV(parts ...[]byte) ([]byte, error) {
	if len(parts) == 1 {
		return parts[0], nil
	}
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out, nil
}
