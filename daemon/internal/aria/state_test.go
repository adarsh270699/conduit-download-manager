package aria

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEngineStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "engine.json")
	want := engineState{
		Version:      engineStateVersion,
		PID:          os.Getpid(),
		CreationTime: 42,
		BinaryPath:   filepath.Join(dir, "bin", "aria2c"),
		RPCPort:      18801,
		RPCSecret:    "secret",
		StartedAt:    time.Now().UTC().Truncate(time.Second),
	}
	if err := writeEngineState(path, want); err != nil {
		t.Fatalf("write state: %v", err)
	}
	got, err := readEngineState(path)
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if got == nil {
		t.Fatal("expected state")
	}
	if got.PID != want.PID || got.BinaryPath != want.BinaryPath || got.RPCSecret != want.RPCSecret {
		t.Fatalf("state mismatch: %+v", got)
	}
}

func TestStateMatchesLiveProcessIdentity(t *testing.T) {
	created, exe, err := processIdentity(os.Getpid())
	if err != nil {
		t.Fatalf("read own process identity: %v", err)
	}
	state := &engineState{
		PID:          os.Getpid(),
		CreationTime: created,
		BinaryPath:   exe,
	}
	if !stateMatchesProcess(state) {
		t.Fatal("expected the current process to match its identity")
	}

	wrongCreation := *state
	wrongCreation.CreationTime++
	if stateMatchesProcess(&wrongCreation) {
		t.Fatal("creation time mismatch must not match")
	}

	wrongPath := *state
	wrongPath.BinaryPath = filepath.Join(t.TempDir(), "other.exe")
	if stateMatchesProcess(&wrongPath) {
		t.Fatal("path mismatch must not match")
	}
}

func TestWaitForRPCRejectsWrongSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","error":{"code":1,"message":"Unauthorized"}}`))
	}))
	defer server.Close()

	sup := &Supervisor{}
	sup.running = true
	port := serverPort(t, server.URL)
	err := sup.waitForRPC(context.Background(), port, "wrong-secret", "", 2*time.Second)
	if err == nil || !strings.Contains(err.Error(), "another engine") {
		t.Fatalf("expected an ownership conflict, got %v", err)
	}
}

func TestWaitForRPCReady(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":{"version":"1.37.0","enabledFeatures":[]}}`))
	}))
	defer server.Close()

	sup := &Supervisor{}
	sup.running = true
	port := serverPort(t, server.URL)
	if err := sup.waitForRPC(context.Background(), port, "secret", "", 2*time.Second); err != nil {
		t.Fatalf("expected readiness, got %v", err)
	}
}

func TestWaitForRPCRejectsDifferentEngineOptions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "jsonrpc") {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":{"version":"1.37.0","enabledFeatures":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	// The fake server answers getVersion but not getGlobalOption with our
	// session path, so the engine must be rejected.
	sup := &Supervisor{}
	sup.cfg.DownloadDir = t.TempDir()
	sup.running = true
	port := serverPort(t, server.URL)
	err := sup.waitForRPC(context.Background(), port, "secret", filepath.Join(t.TempDir(), "aria2.session"), 2*time.Second)
	if err == nil {
		t.Fatal("expected a non-matching engine to be rejected")
	}
}

func serverPort(t *testing.T, rawURL string) int {
	t.Helper()
	trimmed := strings.TrimPrefix(rawURL, "http://")
	trimmed = strings.TrimSuffix(trimmed, "/")
	idx := strings.LastIndex(trimmed, ":")
	if idx < 0 {
		t.Fatalf("unexpected test server URL %q", rawURL)
	}
	port := 0
	for _, ch := range trimmed[idx+1:] {
		if ch < '0' || ch > '9' {
			t.Fatalf("unexpected port in %q", rawURL)
		}
		port = port*10 + int(ch-'0')
	}
	return port
}
