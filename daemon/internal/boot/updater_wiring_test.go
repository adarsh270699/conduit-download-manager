package boot

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"conduit/daemon/internal/aria"
)

// The update service must come up with the daemon even when no engine is
// installed, so a user with a broken engine can still update Conduit.
func TestStartBuildsUpdateServiceWithoutEngine(t *testing.T) {
	t.Setenv(aria.EngineEnvVar, "")
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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

	if running.Updater == nil {
		t.Fatal("Running must expose the update service")
	}
	st := running.Updater.Status()
	if st.State != "idle" || st.CurrentVersion == "" {
		t.Fatalf("a fresh service must start idle with a version: %+v", st)
	}
}
