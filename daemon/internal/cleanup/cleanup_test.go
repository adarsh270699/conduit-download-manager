package cleanup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestKeepRemovesRuntimeAndPreservesSettings(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "roaming", "conduit")
	localDir := filepath.Join(root, "local", "conduit")
	legacy := filepath.Join(root, "roaming", "Conduit.exe")

	writeFile(t, filepath.Join(configDir, "config.json"), `{"downloadDir":"x"}`)
	writeFile(t, filepath.Join(configDir, "aria2.session"), "session")
	writeFile(t, filepath.Join(configDir, "bin", "aria2c.exe"), "engine")
	writeFile(t, filepath.Join(configDir, "engine.json"), `{"pid":1}`)
	writeFile(t, filepath.Join(configDir, "instance.lock"), "")
	writeFile(t, filepath.Join(localDir, "webview", "EBWebView", "Cache", "data"), "cache")
	writeFile(t, filepath.Join(legacy, "EBWebView", "data"), "legacy")

	if err := Run(Options{
		Mode:          Keep,
		ConfigDir:     configDir,
		LocalDataDir:  localDir,
		LegacyWebView: legacy,
	}); err != nil {
		t.Fatalf("cleanup keep: %v", err)
	}

	assertExists(t, filepath.Join(configDir, "config.json"))
	assertExists(t, filepath.Join(configDir, "aria2.session"))
	// bin/ may hold an aria2c the user placed by hand, so keep mode leaves it.
	assertExists(t, filepath.Join(configDir, "bin", "aria2c.exe"))
	assertMissing(t, filepath.Join(configDir, "engine.json"))
	assertMissing(t, filepath.Join(configDir, "instance.lock"))
	assertMissing(t, filepath.Join(localDir, "webview"))
	assertMissing(t, legacy)
}

func TestPurgeRemovesEverything(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "roaming", "conduit")
	localDir := filepath.Join(root, "local", "conduit")
	legacy := filepath.Join(root, "roaming", "Conduit.exe")

	writeFile(t, filepath.Join(configDir, "config.json"), `{"downloadDir":"x"}`)
	writeFile(t, filepath.Join(configDir, "aria2.session"), "session")
	writeFile(t, filepath.Join(localDir, "webview", "EBWebView", "Cache", "data"), "cache")
	writeFile(t, filepath.Join(legacy, "EBWebView", "data"), "legacy")

	if err := Run(Options{
		Mode:          Purge,
		ConfigDir:     configDir,
		LocalDataDir:  localDir,
		LegacyWebView: legacy,
	}); err != nil {
		t.Fatalf("cleanup purge: %v", err)
	}

	assertMissing(t, configDir)
	assertMissing(t, localDir)
	assertMissing(t, legacy)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected %s to exist: %v", path, err)
	}
}

func assertMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be removed, stat err: %v", path, err)
	}
}
