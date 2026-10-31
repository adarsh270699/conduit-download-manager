package cleanup

import (
	"path/filepath"
	"testing"
)

// Downloaded installers and the update record are not settings. Keep mode
// removes them, so a reinstall starts with a clean update state.
func TestKeepRemovesDownloadedInstallersAndUpdateRecord(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "roaming", "conduit")
	localDir := filepath.Join(root, "local", "conduit")
	legacy := filepath.Join(root, "roaming", "Conduit.exe")

	writeFile(t, filepath.Join(configDir, "config.json"), `{"downloadDir":"x"}`)
	writeFile(t, filepath.Join(configDir, "aria2.session"), "session")
	writeFile(t, filepath.Join(configDir, "update.json"), `{"latestVersion":"1.3.0"}`)
	writeFile(t, filepath.Join(configDir, "updates", "1.3.0", "Conduit-windows-amd64-setup.exe"), "installer")

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
	assertMissing(t, filepath.Join(configDir, "updates"))
	assertMissing(t, filepath.Join(configDir, "update.json"))
}

func TestPurgeRemovesDownloadedInstallers(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "roaming", "conduit")
	localDir := filepath.Join(root, "local", "conduit")
	legacy := filepath.Join(root, "roaming", "Conduit.exe")

	writeFile(t, filepath.Join(configDir, "config.json"), `{"downloadDir":"x"}`)
	writeFile(t, filepath.Join(configDir, "updates", "1.3.0", "Conduit-windows-amd64-setup.exe"), "installer")

	if err := Run(Options{
		Mode:          Purge,
		ConfigDir:     configDir,
		LocalDataDir:  localDir,
		LegacyWebView: legacy,
	}); err != nil {
		t.Fatalf("cleanup purge: %v", err)
	}

	assertMissing(t, filepath.Join(configDir, "updates"))
	assertMissing(t, configDir)
}
