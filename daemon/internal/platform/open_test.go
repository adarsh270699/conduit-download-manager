package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveOpenCommandWindows(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "conduit-platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "sample.zip")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Existing file on Windows -> /select,<path>
	cmd, args, err := ResolveOpenCommand("windows", testFile)
	if err != nil {
		t.Fatalf("ResolveOpenCommand failed: %v", err)
	}
	if cmd != "explorer.exe" {
		t.Fatalf("expected explorer.exe, got %s", cmd)
	}
	expectedArg := "/select," + testFile
	if len(args) != 1 || args[0] != expectedArg {
		t.Fatalf("expected arg '%s', got %v", expectedArg, args)
	}

	// 2. Directory on Windows -> folder path
	cmd, args, err = ResolveOpenCommand("windows", tmpDir)
	if err != nil {
		t.Fatalf("ResolveOpenCommand failed: %v", err)
	}
	if len(args) != 1 || args[0] != tmpDir {
		t.Fatalf("expected arg '%s', got %v", tmpDir, args)
	}

	// 3. Non-existent file on Windows -> parent folder
	missingFile := filepath.Join(tmpDir, "missing.zip")
	cmd, args, err = ResolveOpenCommand("windows", missingFile)
	if err != nil {
		t.Fatalf("ResolveOpenCommand failed: %v", err)
	}
	if len(args) != 1 || args[0] != tmpDir {
		t.Fatalf("expected arg '%s', got %v", tmpDir, args)
	}
}

func TestResolveOpenCommandLinux(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "conduit-platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "sample.zip")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// 1. Existing file on Linux -> parent dir with xdg-open
	cmd, args, err := ResolveOpenCommand("linux", testFile)
	if err != nil {
		t.Fatalf("ResolveOpenCommand failed: %v", err)
	}
	if cmd != "xdg-open" {
		t.Fatalf("expected xdg-open, got %s", cmd)
	}
	if len(args) != 1 || args[0] != tmpDir {
		t.Fatalf("expected parent dir '%s', got %v", tmpDir, args)
	}

	// 2. Directory on Linux -> dir with xdg-open
	cmd, args, err = ResolveOpenCommand("linux", tmpDir)
	if err != nil {
		t.Fatalf("ResolveOpenCommand failed: %v", err)
	}
	if len(args) != 1 || args[0] != tmpDir {
		t.Fatalf("expected dir '%s', got %v", tmpDir, args)
	}
}

func TestOpenFolderRunnerOverride(t *testing.T) {
	var capturedCmd string
	var capturedArgs []string

	oldRunner := runner
	defer func() { runner = oldRunner }()

	runner = func(name string, args ...string) error {
		capturedCmd = name
		capturedArgs = args
		return nil
	}

	if err := OpenFolder("/tmp"); err != nil {
		t.Fatalf("OpenFolder failed: %v", err)
	}

	if capturedCmd == "" || len(capturedArgs) == 0 {
		t.Fatal("expected runner to be called with command and args")
	}

	if err := OpenFolder(""); err == nil {
		t.Fatal("expected error on empty target path")
	}
}

func TestResolveOpenFileCommand(t *testing.T) {
	cmd, args := ResolveOpenFileCommand("windows", `C:\dl\setup.exe`)
	if cmd != "rundll32.exe" || len(args) != 2 || args[0] != "url.dll,FileProtocolHandler" {
		t.Fatalf("unexpected windows command: %s %v", cmd, args)
	}
	cmd, args = ResolveOpenFileCommand("linux", "/dl/a.pdf")
	if cmd != "xdg-open" || len(args) != 1 || args[0] != "/dl/a.pdf" {
		t.Fatalf("unexpected linux command: %s %v", cmd, args)
	}
	if err := OpenFile(""); err == nil {
		t.Fatal("expected error on empty path")
	}
}

func TestResolveOpenURLCommand(t *testing.T) {
	testURL := "http://127.0.0.1:18800/#settings"

	cmd, args, err := ResolveOpenURLCommand("windows", testURL)
	if err != nil || cmd != "rundll32.exe" || len(args) != 2 || args[1] != testURL {
		t.Fatalf("unexpected windows command: %s %v", cmd, args)
	}

	cmd, args, err = ResolveOpenURLCommand("linux", testURL)
	if err != nil || cmd != "xdg-open" || len(args) != 1 || args[0] != testURL {
		t.Fatalf("unexpected linux command: %s %v", cmd, args)
	}

	cmd, args, err = ResolveOpenURLCommand("darwin", testURL)
	if err != nil || cmd != "open" || len(args) != 1 || args[0] != testURL {
		t.Fatalf("unexpected darwin command: %s %v", cmd, args)
	}
}

func TestOpenURLRunner(t *testing.T) {
	var capturedCmd string
	var capturedArgs []string

	oldRunner := runner
	defer func() { runner = oldRunner }()

	runner = func(name string, args ...string) error {
		capturedCmd = name
		capturedArgs = args
		return nil
	}

	if err := OpenURL("http://127.0.0.1:18800/#settings"); err != nil {
		t.Fatalf("OpenURL failed: %v", err)
	}

	if capturedCmd == "" || len(capturedArgs) == 0 {
		t.Fatal("expected runner to be called with command and args")
	}

	if err := OpenURL(""); err == nil {
		t.Fatal("expected error on empty target URL")
	}
}
