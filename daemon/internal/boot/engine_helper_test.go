package boot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"conduit/daemon/internal/aria"
)

// needsEngine skips tests that run a real aria2c. Set CONDUIT_ARIA2C to run them.
func needsEngine(t *testing.T) {
	t.Helper()
	if os.Getenv(aria.EngineEnvVar) == "" {
		t.Skipf("set %s to an aria2c binary to run this test", aria.EngineEnvVar)
	}
}

// Without an engine the daemon must still serve /api/health and say what is missing.
func TestStartServesHealthWhenEngineMissing(t *testing.T) {
	t.Setenv(aria.EngineEnvVar, "")
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	httpPort := freePort(t)
	running, err := Start(ctx, Options{
		HTTPPort:    httpPort,
		RPCPort:     freePort(t),
		DownloadDir: filepath.Join(root, "downloads"),
		ConfigDir:   filepath.Join(root, "config"),
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	defer func() { _ = running.Shutdown(ctx) }()

	if !errors.Is(running.EngineErr, aria.ErrEngineMissing) {
		t.Fatalf("EngineErr = %v, want ErrEngineMissing", running.EngineErr)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", httpPort))
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()
	var health map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		t.Fatalf("decode health: %v", err)
	}
	if health["engine"] != "missing" {
		t.Fatalf("engine = %q, want missing", health["engine"])
	}
	if health["engineHint"] == "" {
		t.Fatal("expected engineHint when the engine is missing")
	}
}
