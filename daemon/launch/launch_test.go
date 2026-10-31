package launch

import (
	"path/filepath"
	"testing"
	"time"

	"conduit/daemon/internal/instance"
)

func TestWaitForInstanceStopReturnsWhenLockIsReleased(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	lock, err := instance.Acquire(configDir)
	if err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = lock.Close()
		close(released)
	}()

	if err := WaitForInstanceStop(configDir, 2*time.Second); err != nil {
		t.Fatalf("wait for instance stop: %v", err)
	}
	<-released
}

func TestWaitForInstanceStopTimesOutWhileLocked(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), "config")
	lock, err := instance.Acquire(configDir)
	if err != nil {
		t.Fatalf("acquire test lock: %v", err)
	}
	defer lock.Close()

	if err := WaitForInstanceStop(configDir, 250*time.Millisecond); err == nil {
		t.Fatal("expected a timeout while the lock is held")
	}
}
