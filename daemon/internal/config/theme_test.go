package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The colour scheme follows the system until the user pins one.
func TestThemeDefaultsToSystem(t *testing.T) {
	m, err := NewManager(filepath.Join(t.TempDir(), "config.json"), nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if cfg := m.Get(); cfg.Theme != ThemeSystem {
		t.Fatalf("theme on a new install = %q, want %q", cfg.Theme, ThemeSystem)
	}
}

// Only the three known schemes are kept; anything else falls back to system.
func TestThemeRejectsUnknownValues(t *testing.T) {
	cases := map[string]string{
		"dark":   ThemeDark,
		"light":  ThemeLight,
		"system": ThemeSystem,
		"neon":   ThemeSystem,
		"DARK":   ThemeSystem,
	}
	for in, want := range cases {
		m, err := NewManager(filepath.Join(t.TempDir(), "config.json"), nil)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		if _, err := m.Patch([]byte(`{"theme":"` + in + `"}`)); err != nil {
			t.Fatalf("Patch(%q): %v", in, err)
		}
		if got := m.Get().Theme; got != want {
			t.Errorf("theme %q stored as %q, want %q", in, got, want)
		}
	}
}

// A pinned theme survives a restart, and a save that does not name a theme keeps it.
func TestThemePersistsAndSurvivesSaves(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	m, err := NewManager(path, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if _, err := m.Patch([]byte(`{"theme":"dark"}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	reloaded, err := NewManager(path, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Get().Theme; got != ThemeDark {
		t.Fatalf("theme after restart = %q, want %q", got, ThemeDark)
	}

	// A full save without a theme field, like the first-run wizard's, keeps the pinned scheme.
	cfg := reloaded.Get()
	cfg.Theme = ""
	if _, err := reloaded.Update(cfg); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if got := reloaded.Get().Theme; got != ThemeDark {
		t.Fatalf("a save without a theme reset it to %q", got)
	}

	// A patch that names only another field leaves the theme alone.
	if _, err := reloaded.Patch([]byte(`{"maxConnections":8}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if got := reloaded.Get().Theme; got != ThemeDark {
		t.Fatalf("an unrelated patch changed the theme to %q", got)
	}
}

// A config file written before the setting existed reads as system.
func TestThemeMissingFromOlderFileIsSystem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	older := []byte(`{"downloadDir":"/tmp/downloads","maxConnections":16,"maxConcurrentDownloads":5,"maxDownloadLimit":"0"}`)
	if err := os.WriteFile(path, older, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	reloaded, err := NewManager(path, nil)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.Get().Theme; got != ThemeSystem {
		t.Fatalf("theme from an older file = %q, want %q", got, ThemeSystem)
	}
}
