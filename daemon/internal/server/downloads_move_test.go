package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"conduit/daemon/internal/aria"
)

func TestMoveFileKeepsBothFilesWhenCopyFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	if err := os.WriteFile(src, []byte("source-bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("dest-bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	oldRename := renameFile
	renameFile = func(oldPath, newPath string) error {
		return errors.New("rename failed")
	}
	t.Cleanup(func() { renameFile = oldRename })

	oldCopy := copyFile
	copyFile = func(w io.Writer, r io.Reader) (int64, error) {
		return 0, errors.New("copy failed")
	}
	t.Cleanup(func() { copyFile = oldCopy })

	if err := moveFileWithFallback(src, dst); err == nil {
		t.Fatal("expected copy failure")
	}

	gotSrc, err := os.ReadFile(src)
	if err != nil || string(gotSrc) != "source-bytes" {
		t.Fatalf("source = %q, err %v", gotSrc, err)
	}
	gotDst, err := os.ReadFile(dst)
	if err != nil || string(gotDst) != "dest-bytes" {
		t.Fatalf("destination = %q, err %v", gotDst, err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("temp file left behind: %v", names)
	}
}

func TestChangeDirKeepsTaskWhenMoveFails(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	dir := t.TempDir()
	src := filepath.Join(dir, "sample.bin")
	if err := os.WriteFile(src, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	ariaControl := src + ".aria2"
	if err := os.WriteFile(ariaControl, []byte("control"), 0644); err != nil {
		t.Fatal(err)
	}

	mockRPC.activeItems = []aria.DownloadStatus{
		{
			GID:    "gid-move",
			Status: "paused",
			Dir:    dir,
			Files: []aria.FileInfo{
				{
					Path: src,
					URIs: []aria.FileURI{{URI: "http://example.com/sample.bin"}},
				},
			},
		},
	}

	oldMove := moveFile
	moveFile = func(srcPath, dstPath string) error {
		return errors.New("disk full")
	}
	t.Cleanup(func() { moveFile = oldMove })

	body, err := json.Marshal(map[string]string{
		"action": "change_dir",
		"newDir": filepath.Join(dir, "dest"),
	})
	if err != nil {
		t.Fatal(err)
	}

	handler := srv.Routes()
	req := httptest.NewRequest(http.MethodPost, "/api/downloads/gid-move/action", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("expected an error when the file move fails")
	}
	if mockRPC.removedGID != "" {
		t.Fatalf("aria2 task was removed: %s", mockRPC.removedGID)
	}
	if len(mockRPC.addedURIs) != 0 {
		t.Fatalf("download was re-added: %v", mockRPC.addedURIs)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("source file missing: %v", err)
	}
	if _, err := os.Stat(ariaControl); err != nil {
		t.Fatalf("control file missing: %v", err)
	}
}

func TestMoveFileCopiesWhenRenameFails(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")
	if err := os.WriteFile(src, []byte("source-bytes"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("dest-bytes"), 0644); err != nil {
		t.Fatal(err)
	}

	oldRename := renameFile
	renames := 0
	renameFile = func(oldPath, newPath string) error {
		renames++
		if renames == 1 {
			return errors.New("cross device")
		}
		return os.Rename(oldPath, newPath)
	}
	t.Cleanup(func() { renameFile = oldRename })

	if err := moveFileWithFallback(src, dst); err != nil {
		t.Fatalf("move failed: %v", err)
	}
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("source still exists: %v", err)
	}
	got, err := os.ReadFile(dst)
	if err != nil || string(got) != "source-bytes" {
		t.Fatalf("destination = %q, err %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "dst.bin" {
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		t.Fatalf("unexpected files left behind: %v", names)
	}
}
