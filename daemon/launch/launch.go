package launch

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"conduit/daemon/internal/boot"
	"conduit/daemon/internal/cleanup"
	"conduit/daemon/internal/instance"
	"conduit/daemon/internal/platform"
)

const (
	DefaultHTTPPort = 18800
	DefaultRPCPort  = 18801
)

// ErrAlreadyRunning is returned when another Conduit process owns the
// per-config-directory instance lock.
var ErrAlreadyRunning = boot.ErrAlreadyRunning

// Options selects ports and directories for one Conduit process.
type Options struct {
	HTTPPort    int
	RPCPort     int
	DownloadDir string
	ConfigDir   string
}

// App is a running Conduit daemon with its window hooks.
type App struct {
	inner *boot.Running
}

// Start loads config, starts the download engine when it can, and listens for HTTP.
func Start(ctx context.Context, opts Options) (*App, error) {
	running, err := boot.Start(ctx, boot.Options{
		HTTPPort:    opts.HTTPPort,
		RPCPort:     opts.RPCPort,
		DownloadDir: opts.DownloadDir,
		ConfigDir:   opts.ConfigDir,
	})
	if err != nil {
		return nil, err
	}
	return &App{inner: running}, nil
}

// EngineError returns the aria2 startup error. The HTTP server can still be up.
func (a *App) EngineError() error {
	if a == nil || a.inner == nil {
		return nil
	}
	return a.inner.EngineErr
}

// Port returns the HTTP port.
func (a *App) Port() int {
	if a == nil || a.inner == nil || a.inner.Server == nil {
		return DefaultHTTPPort
	}
	return a.inner.Server.Port()
}

// SetShowHandler registers the callback for POST /api/app/show.
func (a *App) SetShowHandler(fn func()) {
	if a == nil || a.inner == nil || a.inner.Server == nil {
		return
	}
	a.inner.Server.SetShowHandler(fn)
}

// SetQuitHandler registers the callback for POST /api/app/quit.
func (a *App) SetQuitHandler(fn func()) {
	if a == nil || a.inner == nil || a.inner.Server == nil {
		return
	}
	a.inner.Server.SetQuitHandler(fn)
}

// OpenExtensionGuide opens the extension guide in the system browser.
func (a *App) OpenExtensionGuide() error {
	return platform.OpenURL(fmt.Sprintf("http://127.0.0.1:%d/extension", a.Port()))
}

// Shutdown stops the HTTP server, aria2, and releases the instance lock.
func (a *App) Shutdown(ctx context.Context) error {
	if a == nil || a.inner == nil {
		return nil
	}
	return a.inner.Shutdown(ctx)
}

// WaitForInstanceStop waits until the instance lock can be acquired, which
// means the previous Conduit process has finished shutting down and released
// the engine. It is used by the uninstaller before deleting files.
func WaitForInstanceStop(configDir string, timeout time.Duration) error {
	if configDir == "" {
		dir, err := boot.DefaultConfigDir()
		if err != nil {
			return err
		}
		configDir = dir
	}
	abs, err := filepath.Abs(configDir)
	if err != nil {
		return err
	}
	configDir = abs

	deadline := time.Now().Add(timeout)
	for {
		lock, err := instance.Acquire(configDir)
		if err == nil {
			_ = lock.Close()
			return nil
		}
		if !errors.Is(err, instance.ErrAlreadyRunning) {
			return err
		}
		if time.Now().After(deadline) {
			return errors.New("Conduit is still running")
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// UninstallCleanup removes Conduit-owned runtime files, stopping any leftover
// engine first. mode is "keep" or "purge".
func UninstallCleanup(mode, configDir string) error {
	opts, err := cleanup.DefaultOptions(mode)
	if err != nil {
		return err
	}
	if configDir != "" {
		opts.ConfigDir = configDir
	}
	return cleanup.Run(opts)
}
