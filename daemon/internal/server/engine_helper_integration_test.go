//go:build integration

package server

import (
	"os"
	"testing"

	"conduit/daemon/internal/aria"
)

// needsEngine skips tests that run a real aria2c. Set CONDUIT_ARIA2C to run them.
func needsEngine(t *testing.T) {
	t.Helper()
	if os.Getenv(aria.EngineEnvVar) == "" {
		t.Skipf("set %s to an aria2c binary to run this test", aria.EngineEnvVar)
	}
}
