package platform

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindExtensionDir(t *testing.T) {
	root, err := os.MkdirTemp("", "conduit-ext-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	missing, ok := FindExtensionDir([]string{filepath.Join(root, "nope")})
	if ok || missing != "" {
		t.Fatalf("missing dir returned %q ok=%v", missing, ok)
	}

	dir := filepath.Join(root, "extension")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	found, ok := FindExtensionDir([]string{filepath.Join(root, "nope"), dir})
	if !ok {
		t.Fatal("expected the directory with manifest.json")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found != abs {
		t.Fatalf("found %q want %q", found, abs)
	}
}
