//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOpenFolderSelectsNativeFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.zip")
	if err := os.WriteFile(testFile, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	var selected string
	oldSelect := selectItemFn
	selectItemFn = func(path string) error {
		selected = path
		return nil
	}
	t.Cleanup(func() { selectItemFn = oldSelect })

	if err := OpenFolder(testFile); err != nil {
		t.Fatalf("OpenFolder failed: %v", err)
	}
	if strings.Contains(selected, "/") {
		t.Fatalf("selected path still uses forward slashes: %s", selected)
	}
	if !strings.EqualFold(selected, testFile) {
		t.Fatalf("selected %s, want %s", selected, testFile)
	}

	selected = ""
	slashPath := strings.ReplaceAll(testFile, `\`, `/`)
	if err := OpenFolder(slashPath); err != nil {
		t.Fatalf("OpenFolder forward-slash path failed: %v", err)
	}
	if strings.Contains(selected, "/") {
		t.Fatalf("forward-slash input was not normalized: %s", selected)
	}
	if !strings.EqualFold(selected, testFile) {
		t.Fatalf("selected %s, want %s", selected, testFile)
	}
}

func TestOpenFolderOpensDirectoryWithoutSelect(t *testing.T) {
	tmpDir := t.TempDir()

	var opened string
	oldDir := openDirFn
	openDirFn = func(path string) error {
		opened = path
		return nil
	}
	t.Cleanup(func() { openDirFn = oldDir })

	oldSelect := selectItemFn
	selectItemFn = func(path string) error {
		t.Fatalf("file select called for directory %s", path)
		return nil
	}
	t.Cleanup(func() { selectItemFn = oldSelect })

	if err := OpenFolder(tmpDir); err != nil {
		t.Fatalf("OpenFolder directory failed: %v", err)
	}
	if !strings.EqualFold(opened, tmpDir) {
		t.Fatalf("opened %s, want %s", opened, tmpDir)
	}

	opened = ""
	missing := filepath.Join(tmpDir, "missing.zip")
	if err := OpenFolder(missing); err != nil {
		t.Fatalf("OpenFolder missing file failed: %v", err)
	}
	if !strings.EqualFold(opened, tmpDir) {
		t.Fatalf("opened %s, want parent %s", opened, tmpDir)
	}
}

func TestQuoteWindowsArg(t *testing.T) {
	got := quoteWindowsArg(`C:\Users\podda\Downloads\file.zip`)
	want := `"C:\Users\podda\Downloads\file.zip"`
	if got != want {
		t.Fatalf("quoteWindowsArg = %s, want %s", got, want)
	}

	got = quoteWindowsArg(`C:\Users\podda\My Downloads\file.zip`)
	want = `"C:\Users\podda\My Downloads\file.zip"`
	if got != want {
		t.Fatalf("quoteWindowsArg spaces = %s, want %s", got, want)
	}

	got = quoteWindowsArg(`C:\`)
	want = `"C:\\"`
	if got != want {
		t.Fatalf("quoteWindowsArg root = %s, want %s", got, want)
	}
}
