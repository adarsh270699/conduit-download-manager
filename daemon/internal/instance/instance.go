// Package instance provides a per-user, per-config-directory process lock.
// The lock is released by the operating system if the process crashes, so a
// stale lock file never blocks the next launch.
package instance

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrAlreadyRunning is returned when another live process holds the lock.
var ErrAlreadyRunning = errors.New("another Conduit instance is already running")

// Lock is an acquired single-instance lock. Call Close to release it.
type Lock struct {
	file *os.File
	path string
}

// Acquire takes an exclusive lock inside configDir. It creates the directory
// and the lock file when they do not exist.
func Acquire(configDir string) (*Lock, error) {
	if configDir == "" {
		return nil, errors.New("instance lock requires a config directory")
	}
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return nil, fmt.Errorf("create instance directory: %w", err)
	}

	path := filepath.Join(configDir, "instance.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open instance lock: %w", err)
	}

	ok, err := tryLock(f)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock instance file: %w", err)
	}
	if !ok {
		_ = f.Close()
		return nil, ErrAlreadyRunning
	}

	return &Lock{file: f, path: path}, nil
}

// Close releases the lock and closes the underlying file.
func (l *Lock) Close() error {
	if l == nil || l.file == nil {
		return nil
	}
	unlockErr := unlock(l.file)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

// Path returns the lock file path.
func (l *Lock) Path() string {
	if l == nil {
		return ""
	}
	return l.path
}
