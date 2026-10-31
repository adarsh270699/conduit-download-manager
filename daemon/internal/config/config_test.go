package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type MockAutostart struct {
	enabled bool
}

func (m *MockAutostart) IsEnabled() bool {
	return m.enabled
}

func (m *MockAutostart) Enable() error {
	m.enabled = true
	return nil
}

func (m *MockAutostart) Disable() error {
	m.enabled = false
	return nil
}

func TestDefaultConfig(t *testing.T) {
	cfg, err := DefaultConfig()
	if err != nil {
		t.Fatalf("DefaultConfig failed: %v", err)
	}

	if cfg.DownloadDir == "" {
		t.Fatal("expected non-empty default download dir")
	}

	if cfg.MaxConnections != DefaultMaxConnections {
		t.Fatalf("expected max connections %d, got %d", DefaultMaxConnections, cfg.MaxConnections)
	}

	if cfg.MaxConcurrentDownloads != DefaultMaxConcurrentDownloads {
		t.Fatalf("expected concurrent downloads %d, got %d", DefaultMaxConcurrentDownloads, cfg.MaxConcurrentDownloads)
	}

	if cfg.AutoStart {
		t.Fatal("expected autostart to default to false")
	}

	if !cfg.NeedsSetup() {
		t.Fatal("expected a new default config to require setup")
	}
}

func TestValidateAndClamp(t *testing.T) {
	cfg := Config{
		MaxConnections:         999,
		MaxConcurrentDownloads: -5,
	}
	cfg.ValidateAndClamp()

	if cfg.MaxConnections != 16 {
		t.Fatalf("expected clamped max connections 16, got %d", cfg.MaxConnections)
	}

	if cfg.MaxConcurrentDownloads != DefaultMaxConcurrentDownloads {
		t.Fatalf("expected clamped concurrent downloads %d, got %d", DefaultMaxConcurrentDownloads, cfg.MaxConcurrentDownloads)
	}

	cfg2 := Config{
		MaxConnections: 0,
	}
	cfg2.ValidateAndClamp()
	if cfg2.MaxConnections != DefaultMaxConnections {
		t.Fatalf("expected default connections %d, got %d", DefaultMaxConnections, cfg2.MaxConnections)
	}
}

func TestManagerLoadAndSave(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "conduit-cfg-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "conduit", "config.json")
	mockAuto := &MockAutostart{enabled: false}

	// 1. Initial load -> creates default file
	mgr, err := NewManager(configPath, mockAuto)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	c1 := mgr.Get()
	if c1.MaxConnections != DefaultMaxConnections {
		t.Fatalf("expected %d, got %d", DefaultMaxConnections, c1.MaxConnections)
	}

	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// 2. Update configuration
	newCfg := Config{
		DownloadDir:            "/custom/download/path",
		MaxConnections:         8,
		MaxConcurrentDownloads: 3,
		AutoStart:              true,
	}

	updated, err := mgr.Update(newCfg)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if updated.MaxConnections != 8 || updated.MaxConcurrentDownloads != 3 || !updated.AutoStart {
		t.Fatalf("unexpected updated config: %+v", updated)
	}

	if !mockAuto.IsEnabled() {
		t.Fatal("expected mock autostart to be enabled after update")
	}

	// 3. Reload from disk with another manager instance
	mgr2, err := NewManager(configPath, mockAuto)
	if err != nil {
		t.Fatalf("NewManager reload failed: %v", err)
	}

	c2 := mgr2.Get()
	if c2.DownloadDir != "/custom/download/path" || c2.MaxConnections != 8 || c2.MaxConcurrentDownloads != 3 || !c2.AutoStart {
		t.Fatalf("reloaded config mismatch: %+v", c2)
	}

	// 4. Disable autostart
	newCfg.AutoStart = false
	_, err = mgr.Update(newCfg)
	if err != nil {
		t.Fatalf("Update to disable autostart failed: %v", err)
	}
	if mockAuto.IsEnabled() {
		t.Fatal("expected mock autostart to be disabled after update")
	}
}

func TestSetupCompletePresence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "conduit-cfg-setup-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	legacyPath := filepath.Join(tmpDir, "legacy.json")
	legacy := []byte(`{"downloadDir":"/tmp/downloads","maxConnections":4,"maxConcurrentDownloads":2,"autoStart":false}`)
	if err := os.WriteFile(legacyPath, legacy, 0644); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	legacyMgr, err := NewManager(legacyPath, &MockAutostart{})
	if err != nil {
		t.Fatalf("legacy NewManager: %v", err)
	}
	if legacyMgr.Get().NeedsSetup() {
		t.Fatal("existing config without setupComplete must skip the wizard")
	}

	freshPath := filepath.Join(tmpDir, "fresh.json")
	freshMgr, err := NewManager(freshPath, &MockAutostart{})
	if err != nil {
		t.Fatalf("fresh NewManager: %v", err)
	}
	if !freshMgr.Get().NeedsSetup() {
		t.Fatal("new config must require setup")
	}

	saved, err := os.ReadFile(freshPath)
	if err != nil {
		t.Fatalf("read fresh config: %v", err)
	}
	if !bytesContains(saved, []byte(`"setupComplete": false`)) {
		t.Fatalf("new config file missing setupComplete false: %s", saved)
	}

	_, err = freshMgr.Update(Config{
		DownloadDir:            "/kept",
		MaxConnections:         8,
		MaxConcurrentDownloads: 2,
	})
	if err != nil {
		t.Fatalf("update without setupComplete: %v", err)
	}
	if !freshMgr.Get().NeedsSetup() {
		t.Fatal("update that omits setupComplete must keep the wizard pending")
	}

	done := true
	_, err = freshMgr.Update(Config{
		DownloadDir:            "/kept",
		MaxConnections:         8,
		MaxConcurrentDownloads: 2,
		SetupComplete:          &done,
	})
	if err != nil {
		t.Fatalf("complete setup: %v", err)
	}
	if freshMgr.Get().NeedsSetup() {
		t.Fatal("setupComplete true must finish the wizard")
	}
}

func bytesContains(haystack, needle []byte) bool {
	return len(haystack) >= len(needle) && (len(needle) == 0 || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle []byte) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

func TestPatchKeepsFieldsTheClientDidNotSend(t *testing.T) {
	auto := &MockAutostart{}
	mgr, err := NewManager(filepath.Join(t.TempDir(), "config.json"), auto)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Update(Config{DownloadDir: "/dl", MaxConnections: 4, MaxConcurrentDownloads: 2, AutoStart: true}); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.Patch([]byte(`{"maxDownloadLimit":"5M"}`))
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if got.MaxDownloadLimit != "5M" {
		t.Fatalf("limit = %q, want 5M", got.MaxDownloadLimit)
	}
	if got.DownloadDir != "/dl" || got.MaxConnections != 4 || got.MaxConcurrentDownloads != 2 || !got.AutoStart {
		t.Fatalf("patch reset unrelated settings: %+v", got)
	}
	if !auto.IsEnabled() {
		t.Fatal("a speed-limit patch must not disable autostart")
	}
}

func TestPatchDoesNotAliasSetupComplete(t *testing.T) {
	mgr, err := NewManager(filepath.Join(t.TempDir(), "config.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Patch([]byte(`{"setupComplete":true}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.Patch([]byte(`{"setupComplete":"nope"}`)); err == nil {
		t.Fatal("expected a type error")
	}
	if mgr.Get().NeedsSetup() {
		t.Fatal("a rejected patch must leave the stored config untouched")
	}
}

func TestInvalidDownloadLimitIsRejected(t *testing.T) {
	mgr, err := NewManager(filepath.Join(t.TempDir(), "config.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"fast", "5MB", "-1", "1G"} {
		if _, err := mgr.Patch([]byte(`{"maxDownloadLimit":"` + bad + `"}`)); !errors.Is(err, ErrInvalid) {
			t.Fatalf("limit %q: err = %v, want ErrInvalid", bad, err)
		}
	}
	for _, good := range []string{"0", "512K", "10M", "1048576"} {
		if _, err := mgr.Patch([]byte(`{"maxDownloadLimit":"` + good + `"}`)); err != nil {
			t.Fatalf("limit %q rejected: %v", good, err)
		}
	}
}

func TestLoadRefreshesExistingAutostartEntry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"downloadDir":"/dl","autoStart":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	auto := &countingAutostart{MockAutostart: MockAutostart{enabled: true}}
	if _, err := NewManager(path, auto); err != nil {
		t.Fatal(err)
	}
	if auto.enables != 1 {
		t.Fatalf("Enable called %d times, want 1 so old entries gain %s", auto.enables, BackgroundFlag)
	}
}

type countingAutostart struct {
	MockAutostart
	enables int
}

func (c *countingAutostart) Enable() error {
	c.enables++
	return c.MockAutostart.Enable()
}

func TestEnginePathMustBeAbsolute(t *testing.T) {
	mgr, err := NewManager(filepath.Join(t.TempDir(), "config.json"), nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg := mgr.Get()
	cfg.EnginePath = "aria2c"
	if _, err := mgr.Update(cfg); !errors.Is(err, ErrInvalid) {
		t.Fatalf("relative enginePath: err = %v, want ErrInvalid", err)
	}

	abs := filepath.Join(t.TempDir(), "aria2c")
	cfg.EnginePath = abs
	updated, err := mgr.Update(cfg)
	if err != nil {
		t.Fatalf("absolute enginePath rejected: %v", err)
	}
	if updated.EnginePath != abs {
		t.Fatalf("EnginePath = %q, want %q", updated.EnginePath, abs)
	}
}
