package config

import (
	"path/filepath"
	"testing"
)

// Automatic update settings are off for a new install.
func TestUpdateSettingsDefaultOff(t *testing.T) {
	m, err := NewManager(filepath.Join(t.TempDir(), "config.json"), nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if cfg := m.Get(); cfg.AutoCheckUpdates || cfg.AutoInstallUpdates {
		t.Fatalf("automatic updates on by default: %+v", cfg)
	}
}

// The two switches survive a restart, and a patch changes only the fields it names.
func TestUpdateSettingsPatchPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m, err := NewManager(path, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := m.Patch([]byte(`{"autoCheckUpdates":true,"autoInstallUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	reloaded, err := NewManager(path, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	cfg := reloaded.Get()
	if !cfg.AutoCheckUpdates || !cfg.AutoInstallUpdates {
		t.Fatalf("update switches not saved: %+v", cfg)
	}

	if _, err := reloaded.Patch([]byte(`{"autoInstallUpdates":false}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if cfg := reloaded.Get(); !cfg.AutoCheckUpdates || cfg.AutoInstallUpdates {
		t.Fatalf("a patch that names one switch changed the other: %+v", cfg)
	}
}
