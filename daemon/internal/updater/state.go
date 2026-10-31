// Package updater checks GitHub for a newer stable Conduit release, downloads
// the installer for this OS, verifies it, and installs it on request. It owns
// update.json and the updates/ folder under the config directory. It never
// writes config.json or aria2.session.
package updater

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// State is the updater's place in the pipeline. GET /api/update reports it as is.
type State string

const (
	StateIdle        State = "idle"
	StateChecking    State = "checking"
	StateUpToDate    State = "upToDate"
	StateAvailable   State = "available"
	StateDownloading State = "downloading"
	StateDownloaded  State = "downloaded"
	StateInstalling  State = "installing"
	StateError       State = "error"
)

// Progress counts the installer bytes received so far.
type Progress struct {
	Done  int64 `json:"done"`
	Total int64 `json:"total"`
}

// Status is the body of GET /api/update.
type Status struct {
	State          State      `json:"state"`
	CurrentVersion string     `json:"currentVersion"`
	LatestVersion  string     `json:"latestVersion,omitempty"`
	Progress       Progress   `json:"progress"`
	Error          string     `json:"error,omitempty"`
	LastCheckedAt  *time.Time `json:"lastCheckedAt,omitempty"`
	// Installable reports whether this copy can install an update. Only copies
	// made by the Windows setup or the .deb package can.
	Installable bool `json:"installable"`
}

// record is what survives a restart. It lives in update.json.
type record struct {
	LastCheckedAt     *time.Time `json:"lastCheckedAt,omitempty"`
	LatestVersion     string     `json:"latestVersion,omitempty"`
	DownloadedVersion string     `json:"downloadedVersion,omitempty"`
	DownloadedPath    string     `json:"downloadedPath,omitempty"`
	DownloadedSHA256  string     `json:"downloadedSha256,omitempty"`
}

const (
	recordName     = "update.json"
	updatesDirName = "updates"
)

// loadRecord reads update.json. A missing file is an empty record. A temp file
// left by an interrupted save is removed and never read.
func loadRecord(configDir string) (record, error) {
	path := filepath.Join(configDir, recordName)
	if err := os.Remove(path + ".tmp"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return record{}, fmt.Errorf("remove stale %s.tmp: %w", recordName, err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return record{}, nil
	}
	if err != nil {
		return record{}, fmt.Errorf("read %s: %w", recordName, err)
	}
	var rec record
	if err := json.Unmarshal(data, &rec); err != nil {
		return record{}, fmt.Errorf("parse %s: %w", recordName, err)
	}
	return rec, nil
}

// saveRecord writes update.json through a temp file and a rename, as
// config.json is written, so a crash never leaves a half-written file.
func saveRecord(configDir string, rec record) error {
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(configDir, recordName)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
