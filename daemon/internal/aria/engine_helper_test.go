package aria

import (
	"os"
	"path/filepath"
	"testing"
)

// requireEngineBinary returns the aria2c binary that tests run. Set
// CONDUIT_ARIA2C to a binary to run them; without it they are skipped.
func requireEngineBinary(t *testing.T) string {
	t.Helper()
	path := os.Getenv(EngineEnvVar)
	if path == "" {
		t.Skipf("set %s to an aria2c binary to run this test", EngineEnvVar)
	}
	return path
}

func copyFile(t *testing.T, from, to string) {
	t.Helper()
	data, err := os.ReadFile(from)
	if err != nil {
		t.Fatalf("read %s: %v", from, err)
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", to, err)
	}
	if err := os.WriteFile(to, data, 0o755); err != nil {
		t.Fatalf("write %s: %v", to, err)
	}
}
