package instance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireIsExclusiveAndReusableAfterClose(t *testing.T) {
	dir := t.TempDir()

	first, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	second, err := Acquire(dir)
	if !errors.Is(err, ErrAlreadyRunning) {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("second acquire: expected ErrAlreadyRunning, got %v", err)
	}

	if err := first.Close(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}

	third, err := Acquire(dir)
	if err != nil {
		t.Fatalf("third acquire after release: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatalf("release third lock: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "instance.lock")); err != nil {
		t.Fatalf("lock file missing: %v", err)
	}
}
