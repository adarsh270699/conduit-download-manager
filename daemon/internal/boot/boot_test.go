package boot

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"
)

func TestStartRejectsSecondInstanceForSameConfigDir(t *testing.T) {
	needsEngine(t)
	root := t.TempDir()
	configDir := filepath.Join(root, "config")
	downloadDir := filepath.Join(root, "downloads")

	httpPort := freePort(t)
	rpcPort := freePort(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	running, err := Start(ctx, Options{
		HTTPPort:    httpPort,
		RPCPort:     rpcPort,
		DownloadDir: downloadDir,
		ConfigDir:   configDir,
	})
	if err != nil {
		t.Fatalf("first start: %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = running.Shutdown(shutdownCtx)
	})

	_, err = Start(ctx, Options{
		HTTPPort:    freePort(t),
		RPCPort:     freePort(t),
		DownloadDir: downloadDir,
		ConfigDir:   configDir,
	})
	if !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second start: expected ErrAlreadyRunning, got %v", err)
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// aria2 can exit on its own (crash, killed by the user, OOM). The daemon must
// bring it back instead of reporting "Engine offline" until the app restarts.
func TestEngineIsRestartedAfterItExits(t *testing.T) {
	needsEngine(t)
	oldInterval := engineCheckInterval
	engineCheckInterval = 100 * time.Millisecond
	t.Cleanup(func() { engineCheckInterval = oldInterval })

	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	running, err := Start(ctx, Options{
		HTTPPort:    freePort(t),
		RPCPort:     freePort(t),
		DownloadDir: filepath.Join(root, "downloads"),
		ConfigDir:   filepath.Join(root, "config"),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = running.Shutdown(shutdownCtx)
	})
	if _, err := running.Config.Patch([]byte(`{"maxDownloadLimit":"3M"}`)); err != nil {
		t.Fatal(err)
	}

	if err := running.Supervisor.Stop(); err != nil {
		t.Fatalf("stop engine: %v", err)
	}
	// The watcher restarts aria2, then applies the saved settings to it.
	deadline := time.Now().Add(20 * time.Second)
	for {
		if running.Supervisor.EngineState() == "ready" {
			opts, err := running.RPC.GetGlobalOption(ctx)
			if err == nil && opts["max-overall-download-limit"] == "3145728" {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("engine was not restarted with the saved settings")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
