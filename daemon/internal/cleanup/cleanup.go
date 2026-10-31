// Package cleanup implements the uninstall-time sweep. It removes only files
// and processes owned by this Conduit installation. Downloads are never
// touched.
package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/boot"
	"conduit/daemon/internal/instance"
)

// Mode selects how much user data the uninstaller removes.
type Mode string

const (
	// Keep removes runtime files but keeps config.json and aria2.session for a
	// future reinstall.
	Keep Mode = "keep"
	// Purge removes everything Conduit owns, including settings and session.
	Purge Mode = "purge"
)

// Options gives the cleanup helper explicit paths. Empty fields use the normal
// per-user locations; tests override them with temporary directories.
type Options struct {
	Mode           Mode
	ConfigDir      string
	LocalDataDir   string
	LegacyWebView  string
	RemoveSettings bool
}

// DefaultOptions resolves the per-user paths and the requested mode.
func DefaultOptions(mode string) (Options, error) {
	parsed, err := ParseMode(mode)
	if err != nil {
		return Options{}, err
	}

	configDir, err := boot.DefaultConfigDir()
	if err != nil {
		return Options{}, err
	}

	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		return Options{}, fmt.Errorf("locate user cache directory: %w", err)
	}

	configRoot, err := os.UserConfigDir()
	if err != nil {
		return Options{}, fmt.Errorf("locate user config directory: %w", err)
	}

	return Options{
		Mode:           parsed,
		ConfigDir:      configDir,
		LocalDataDir:   filepath.Join(cacheRoot, "conduit"),
		LegacyWebView:  filepath.Join(configRoot, "Conduit.exe"),
		RemoveSettings: parsed == Purge,
	}, nil
}

// ParseMode validates the value passed by the uninstaller.
func ParseMode(value string) (Mode, error) {
	switch Mode(strings.ToLower(strings.TrimSpace(value))) {
	case Keep:
		return Keep, nil
	case Purge:
		return Purge, nil
	default:
		return "", fmt.Errorf("invalid cleanup mode %q: expected keep or purge", value)
	}
}

// Run stops managed engines and removes runtime files. In keep mode,
// config.json and aria2.session are preserved. In purge mode everything
// Conduit owns is removed.
func Run(opts Options) error {
	if opts.Mode != Keep && opts.Mode != Purge {
		return fmt.Errorf("invalid cleanup mode %q", opts.Mode)
	}
	if opts.ConfigDir == "" {
		return errors.New("cleanup requires a config directory")
	}

	_, statErr := os.Stat(opts.ConfigDir)
	configDirExisted := statErr == nil

	lock, err := acquireLockWithRetry(opts.ConfigDir, 12*time.Second)
	if err != nil {
		if errors.Is(err, instance.ErrAlreadyRunning) {
			return fmt.Errorf("Conduit is still running; quit it before cleaning up: %w", err)
		}
		return err
	}

	var errs []error

	if err := aria.StopManagedEngines(opts.ConfigDir); err != nil {
		errs = append(errs, fmt.Errorf("stop managed engines: %w", err))
	}
	if err := aria.RemoveManagedEngineFiles(opts.ConfigDir); err != nil {
		errs = append(errs, fmt.Errorf("remove engine files: %w", err))
	}

	if opts.LocalDataDir != "" {
		target := opts.LocalDataDir
		if opts.Mode == Keep {
			target = filepath.Join(opts.LocalDataDir, "webview")
		}
		if err := removeAllWithRetry(target, 8); err != nil {
			errs = append(errs, fmt.Errorf("remove local app data: %w", err))
		}
		if opts.Mode == Keep {
			// Remove the now-empty local data directory, but never a non-empty
			// one that may hold another Conduit version's files.
			_ = os.Remove(opts.LocalDataDir)
		}
	}
	if opts.LegacyWebView != "" {
		if err := removeAllWithRetry(opts.LegacyWebView, 8); err != nil {
			errs = append(errs, fmt.Errorf("remove legacy WebView data: %w", err))
		}
		if err := removeAllWithRetry(opts.LegacyWebView+".cleanup", 8); err != nil {
			errs = append(errs, fmt.Errorf("remove quarantined legacy WebView data: %w", err))
		}
	}

	for _, name := range []string{"config.json.tmp", "engine.json.tmp"} {
		path := filepath.Join(opts.ConfigDir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s: %w", name, err))
		}
	}

	// Release the lock before deleting the lock file or the config directory.
	if err := lock.Close(); err != nil {
		errs = append(errs, fmt.Errorf("release instance lock: %w", err))
	}

	if opts.Mode == Purge || opts.RemoveSettings {
		if err := removeAllWithRetry(opts.ConfigDir, 8); err != nil {
			errs = append(errs, fmt.Errorf("remove config directory: %w", err))
		}
	} else {
		lockPath := filepath.Join(opts.ConfigDir, "instance.lock")
		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove instance lock: %w", err))
		}
		if !configDirExisted {
			_ = os.Remove(opts.ConfigDir)
		}
	}

	return errors.Join(errs...)
}

func acquireLockWithRetry(configDir string, timeout time.Duration) (*instance.Lock, error) {
	deadline := time.Now().Add(timeout)
	for {
		lock, err := instance.Acquire(configDir)
		if err == nil {
			return lock, nil
		}
		if !errors.Is(err, instance.ErrAlreadyRunning) || time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func removeAllWithRetry(path string, attempts int) error {
	if path == "" {
		return nil
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		lastErr = os.RemoveAll(path)
		if lastErr == nil {
			if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
				return nil
			} else if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("%s still exists after removal", path)
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return lastErr
}
