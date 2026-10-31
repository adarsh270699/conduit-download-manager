package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"conduit/daemon/internal/release"
)

// A download saved by an earlier run is still ready to install after a restart,
// so "Install and restart" and automatic install do not wait for a new check.
func TestRestartKeepsAReadyInstaller(t *testing.T) {
	ver, _ := release.Parse("9.9.9")
	name, err := installerName(runtime.GOOS, ver)
	if err != nil {
		t.Skipf("no installer on %s: %v", runtime.GOOS, err)
	}

	cases := []struct {
		name      string
		current   string
		onDisk    bool // the installer file exists at the recorded path
		elsewhere bool // the record names a file outside the updates folder
		want      State
	}{
		{"ready after restart", "1.0.0", true, false, StateDownloaded},
		{"already installed", "9.9.9", true, false, StateIdle},
		{"file was deleted", "1.0.0", false, false, StateIdle},
		{"record names another file", "1.0.0", true, true, StateIdle},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			recorded := filepath.Join(dir, updatesDirName, "9.9.9", name)
			if c.elsewhere {
				recorded = filepath.Join(dir, name)
			}
			if c.onDisk {
				if err := os.MkdirAll(filepath.Dir(recorded), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(recorded, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveRecord(dir, record{
				LatestVersion:     "9.9.9",
				DownloadedVersion: "9.9.9",
				DownloadedPath:    recorded,
				DownloadedSHA256:  "00",
			}); err != nil {
				t.Fatal(err)
			}

			svc, err := New(Options{
				ConfigDir:      dir,
				CurrentVersion: c.current,
				Channel:        func() Channel { return ChannelNone },
			})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			if got := svc.Status().State; got != c.want {
				t.Fatalf("state after restart %q, want %q", got, c.want)
			}
		})
	}
}
