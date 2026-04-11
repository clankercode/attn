package audio

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

var lockDir = "/tmp/attn-tool"

var ErrAlreadyPlaying = errors.New("audio already playing")

type lockState struct {
	file *os.File
}

func AcquireLock() (*lockState, error) {
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	lockFile, err := os.OpenFile(lockDir+"/lock", os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	err = unix.Flock(int(lockFile.Fd()), unix.LOCK_NB|unix.LOCK_EX)
	if err != nil {
		lockFile.Close()
		if err == unix.EWOULDBLOCK {
			return nil, ErrAlreadyPlaying
		}
		return nil, fmt.Errorf("acquire lock: %w", err)
	}

	fmt.Fprintf(os.Stderr, "[LOCK] Acquired by PID %d\n", os.Getpid())
	return &lockState{file: lockFile}, nil
}

func WaitForLock(timeoutMs int) (*lockState, error) {
	deadline := time.Now().Add(time.Duration(timeoutMs) * time.Millisecond)
	for time.Now().Before(deadline) {
		lock, err := AcquireLock()
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, ErrAlreadyPlaying) {
			return nil, err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, ErrAlreadyPlaying
}

func (s *lockState) Release() error {
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}
