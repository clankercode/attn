package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseKeepsLockFilePath(t *testing.T) {
	originalLockDir := lockDir
	lockDir = t.TempDir()
	t.Cleanup(func() {
		lockDir = originalLockDir
	})

	lock, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}

	if err := lock.Release(); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(lockDir, "lock")); err != nil {
		t.Fatalf("expected lock file to remain after release: %v", err)
	}
}

func TestForegroundPlaybackStillAcquiresLock(t *testing.T) {
	originalLockDir := lockDir
	lockDir = t.TempDir()
	t.Cleanup(func() {
		lockDir = originalLockDir
	})

	lock, err := AcquireLock()
	if err != nil {
		t.Fatalf("AcquireLock() error = %v", err)
	}
	defer lock.Release()

	if _, err := WaitForLock(150); err == nil {
		t.Fatal("expected wait to fail while foreground holder owns the lock")
	}
}
