//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxAutostart(t *testing.T) {
	// Determine the autostart desktop file path
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(os.Getenv("HOME"), ".config")
	}
	desktopFile := filepath.Join(configHome, "autostart", "conduit.desktop")

	// Backup any existing desktop file to restore after the test
	existingContent, hadExisting := os.ReadFile(desktopFile)
	defer func() {
		if hadExisting == nil {
			_ = os.MkdirAll(filepath.Dir(desktopFile), 0755)
			_ = os.WriteFile(desktopFile, existingContent, 0644)
		} else {
			_ = os.Remove(desktopFile)
		}
	}()

	mockExec := "/usr/local/bin/conduit-verify-test"
	auto, err := NewAppAutostart(mockExec)
	if err != nil {
		t.Fatalf("NewAppAutostart failed: %v", err)
	}

	// Clean initial state for test
	_ = auto.Disable()
	if auto.IsEnabled() {
		t.Fatal("expected autostart to be disabled after initial Disable()")
	}

	// 1. Enable autostart
	if err := auto.Enable(); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	if !auto.IsEnabled() {
		t.Fatal("expected autostart to be enabled after Enable()")
	}

	// Verify desktop file exists and contains expected keys
	content, err := os.ReadFile(desktopFile)
	if err != nil {
		t.Fatalf("failed to read desktop file at %s: %v", desktopFile, err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "[Desktop Entry]") {
		t.Fatalf("missing [Desktop Entry] in %s", strContent)
	}
	if !strings.Contains(strContent, "Conduit Download Accelerator") {
		t.Fatalf("missing DisplayName in %s", strContent)
	}
	if !strings.Contains(strContent, `"`+mockExec+`" "`+BackgroundFlag+`"`) {
		t.Fatalf("missing Exec path %s in %s", mockExec, strContent)
	}

	// 2. Disable autostart
	if err := auto.Disable(); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}

	if auto.IsEnabled() {
		t.Fatal("expected autostart to be disabled after Disable()")
	}

	if _, err := os.Stat(desktopFile); !os.IsNotExist(err) {
		t.Fatalf("expected desktop file to be removed after Disable(), got err: %v", err)
	}
}
