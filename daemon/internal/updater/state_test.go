package updater

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMissingRecordIsEmpty(t *testing.T) {
	rec, err := loadRecord(t.TempDir())
	if err != nil {
		t.Fatalf("load missing record: %v", err)
	}
	if rec != (record{}) {
		t.Fatalf("missing record should be empty, got %+v", rec)
	}
}

func TestRecordRoundTrip(t *testing.T) {
	dir := t.TempDir()
	checked := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	want := record{
		LastCheckedAt:     &checked,
		LatestVersion:     "1.3.0",
		DownloadedVersion: "1.3.0",
		DownloadedPath:    filepath.Join(dir, "updates", "1.3.0", "setup.exe"),
		DownloadedSHA256:  "abc123",
	}
	if err := saveRecord(dir, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := loadRecord(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got.LatestVersion != want.LatestVersion ||
		got.DownloadedVersion != want.DownloadedVersion ||
		got.DownloadedPath != want.DownloadedPath ||
		got.DownloadedSHA256 != want.DownloadedSHA256 ||
		got.LastCheckedAt == nil || !got.LastCheckedAt.Equal(checked) {
		t.Fatalf("round trip lost a field:\nwant %+v\ngot  %+v", want, got)
	}
	if _, err := os.Stat(filepath.Join(dir, recordName+".tmp")); !os.IsNotExist(err) {
		t.Fatalf("save left a temp file behind: %v", err)
	}
}

func TestLeftoverTempIsRemovedAndNotRead(t *testing.T) {
	dir := t.TempDir()
	tmp := filepath.Join(dir, recordName+".tmp")
	if err := os.WriteFile(tmp, []byte(`{"latestVersion":"9.9.9"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := loadRecord(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if rec.LatestVersion != "" {
		t.Fatalf("temp file was read as state: %+v", rec)
	}
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Fatalf("stale temp file was not removed: %v", err)
	}
}

func TestUnknownKeysAreIgnored(t *testing.T) {
	dir := t.TempDir()
	body := `{"latestVersion":"1.3.0","someFutureKey":{"nested":true}}`
	if err := os.WriteFile(filepath.Join(dir, recordName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rec, err := loadRecord(dir)
	if err != nil {
		t.Fatalf("a newer file must still load: %v", err)
	}
	if rec.LatestVersion != "1.3.0" {
		t.Fatalf("known key lost: %+v", rec)
	}
}

func TestCorruptRecordIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, recordName), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRecord(dir); err == nil {
		t.Fatal("corrupt update.json should report an error")
	}
}

func TestStateNames(t *testing.T) {
	want := map[State]string{
		StateIdle:        "idle",
		StateChecking:    "checking",
		StateUpToDate:    "upToDate",
		StateAvailable:   "available",
		StateDownloading: "downloading",
		StateDownloaded:  "downloaded",
		StateInstalling:  "installing",
		StateError:       "error",
	}
	for state, name := range want {
		if string(state) != name {
			t.Fatalf("state %q reported as %q", name, state)
		}
	}
}
