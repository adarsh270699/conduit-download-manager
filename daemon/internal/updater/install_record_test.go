package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Install refuses a record that names a file download did not write, so an
// edited update.json cannot make Conduit run an arbitrary file.
func TestInstallRefusesRecordedPathDownloadDidNotWrite(t *testing.T) {
	svc := installService(t, supportedChannel(t), &fakeDownloads{})
	body := []byte("not an installer")
	outside := filepath.Join(svc.configDir, "elsewhere.bin")
	if err := os.WriteFile(outside, body, 0o644); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	svc.rec = record{DownloadedVersion: "1.3.0", DownloadedPath: outside, DownloadedSHA256: sha256Hex(body)}
	svc.status.State = StateDownloaded
	svc.mu.Unlock()

	if err := svc.Install(context.Background()); !errors.Is(err, ErrRecordMismatch) {
		t.Fatalf("Install: %v, want ErrRecordMismatch", err)
	}
	if cmd := svc.TakePendingInstall(); cmd != nil {
		t.Fatalf("a record that names another file was queued: %+v", cmd)
	}
	if st := svc.Status().State; st != StateDownloaded {
		t.Fatalf("state %q, want downloaded", st)
	}
}
