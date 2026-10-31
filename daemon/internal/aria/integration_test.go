//go:build integration

package aria

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLiveAriaSupervisorAndRPC(t *testing.T) {
	requireEngineBinary(t)
	tmpDir, err := os.MkdirTemp("", "conduit-int-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	downloadDir := filepath.Join(tmpDir, "downloads")
	configDir := filepath.Join(tmpDir, "config")
	port := getFreePort(t)

	sup, err := NewSupervisor(SupervisorConfig{
		ConfigDir:   configDir,
		DownloadDir: downloadDir,
		RPCPort:     port,
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer func() {
		_ = sup.Stop()
	}()

	client := NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", port), sup.RPCSecret())

	// 1. Verify GetVersion
	ver, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	t.Logf("aria2 version: %s, features: %v", ver.Version, ver.EnabledFeatures)
	if ver.Version != "1.37.0" {
		t.Errorf("expected version 1.37.0, got %s", ver.Version)
	}

	// 2. Start mock HTTP download server
	fileContent := "Conduit live integration download payload 1234567890"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(fileContent)))
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(fileContent))
	}))
	defer ts.Close()

	// 3. AddURI
	gid, err := client.AddURI(ctx, []string{ts.URL + "/testfile.txt"}, &DownloadOptions{
		Out: "testfile.txt",
	})
	if err != nil {
		t.Fatalf("AddURI failed: %v", err)
	}
	if gid == "" {
		t.Fatal("expected non-empty GID")
	}

	// 4. Poll until stopped (complete)
	var completed bool
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		stopped, err := client.TellStopped(ctx, 0, 10)
		if err == nil {
			for _, item := range stopped {
				if item.GID == gid && item.Status == "complete" {
					completed = true
					break
				}
			}
		}
		if completed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !completed {
		t.Fatalf("download with gid %s did not complete within deadline", gid)
	}

	// 5. Verify downloaded file on disk
	downloadedFilePath := filepath.Join(downloadDir, "testfile.txt")
	data, err := os.ReadFile(downloadedFilePath)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(data) != fileContent {
		t.Fatalf("file content mismatch: expected '%s', got '%s'", fileContent, string(data))
	}

	// 6. Stop supervisor and verify clean exit
	if err := sup.Stop(); err != nil {
		t.Fatalf("failed to stop supervisor: %v", err)
	}

	time.Sleep(300 * time.Millisecond)
	if sup.IsRunning() {
		t.Fatal("supervisor still running after Stop()")
	}
}
