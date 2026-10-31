package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/config"
)

type mockAriaClient struct {
	addedURIs      []string
	addedOpts      *aria.DownloadOptions
	activeItems    []aria.DownloadStatus
	waitingItems   []aria.DownloadStatus
	stoppedItems   []aria.DownloadStatus
	pausedGID      string
	unpausedGID    string
	removedGID     string
	removeErr      error
	changedOptions map[string]string
	version        *aria.VersionInfo
	options        map[string]map[string]string
}

func (m *mockAriaClient) GetOption(ctx context.Context, gid string) (map[string]string, error) {
	return m.options[gid], nil
}

func (m *mockAriaClient) AddURI(ctx context.Context, uris []string, opts *aria.DownloadOptions) (string, error) {
	m.addedURIs = uris
	m.addedOpts = opts
	return "test-gid-999", nil
}

func (m *mockAriaClient) TellActive(ctx context.Context, keys ...string) ([]aria.DownloadStatus, error) {
	return m.activeItems, nil
}

func (m *mockAriaClient) TellWaiting(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error) {
	return m.waitingItems, nil
}

func (m *mockAriaClient) TellStopped(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error) {
	return m.stoppedItems, nil
}

func (m *mockAriaClient) Pause(ctx context.Context, gid string) error {
	m.pausedGID = gid
	return nil
}

func (m *mockAriaClient) Unpause(ctx context.Context, gid string) error {
	m.unpausedGID = gid
	return nil
}

func (m *mockAriaClient) Remove(ctx context.Context, gid string) error {
	m.removedGID = gid
	return m.removeErr
}

func (m *mockAriaClient) ForceRemove(ctx context.Context, gid string) error {
	m.removedGID = gid
	return nil
}

func (m *mockAriaClient) RemoveDownloadResult(ctx context.Context, gid string) error {
	return nil
}

func (m *mockAriaClient) ChangeGlobalOption(ctx context.Context, opts map[string]string) error {
	m.changedOptions = opts
	return nil
}

func (m *mockAriaClient) ChangePosition(ctx context.Context, gid string, pos int, how string) (int, error) {
	return 1, nil
}

func (m *mockAriaClient) TellStatus(ctx context.Context, gid string, keys ...string) (*aria.DownloadStatus, error) {
	for _, it := range m.activeItems {
		if it.GID == gid {
			return &it, nil
		}
	}
	for _, it := range m.waitingItems {
		if it.GID == gid {
			return &it, nil
		}
	}
	for _, it := range m.stoppedItems {
		if it.GID == gid {
			return &it, nil
		}
	}
	return &aria.DownloadStatus{GID: gid, Dir: "/mock/download/dir"}, nil
}

func (m *mockAriaClient) Shutdown(ctx context.Context) error {
	return nil
}

func (m *mockAriaClient) GetVersion(ctx context.Context) (*aria.VersionInfo, error) {
	if m.version != nil {
		return m.version, nil
	}
	return &aria.VersionInfo{Version: "1.37.0"}, nil
}

func setupTestServer(t *testing.T) (*Server, *mockAriaClient, *config.Manager, func()) {
	tmpDir, err := os.MkdirTemp("", "conduit-srv-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	configPath := filepath.Join(tmpDir, "config.json")
	cfgMgr, err := config.NewManager(configPath, nil)
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}

	mockRPC := &mockAriaClient{}
	srv := NewServer(18800, mockRPC, cfgMgr)

	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return srv, mockRPC, cfgMgr, cleanup
}

func TestHealthEndpoint(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	handler := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse json: %v", err)
	}

	if resp["status"] != "ok" || resp["version"] != ServerVersion {
		t.Fatalf("unexpected health response: %v", resp)
	}
}

func TestDownloadEndpointsReturn503WithoutEngine(t *testing.T) {
	srv := NewServer(18800, nil, nil)
	handler := srv.Routes()

	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(`{"url":"https://example.com/file.zip"}`)),
		httptest.NewRequest(http.MethodPost, "/api/downloads/gid-1/action", strings.NewReader(`{"action":"pause"}`)),
	}
	for _, req := range requests {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s: expected 503, got %d", req.Method, req.URL.Path, rec.Code)
		}
	}
}

func TestCORSHeaders(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	handler := srv.Routes()

	// 1. A foreign extension is rejected at the request, not just in CORS headers.
	req := httptest.NewRequest(http.MethodOptions, "/api/downloads", nil)
	req.Header.Set("Origin", "chrome-extension://abcdefghijklmnop")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for foreign origin, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("foreign extension origin was allowed: %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// 2. This extension, read from its manifest, is allowed.
	manifest, err := readExtensionManifest()
	if err != nil {
		t.Fatalf("read extension manifest: %v", err)
	}
	origins, err := extensionOrigins(manifest)
	if err != nil {
		t.Fatalf("extension origins: %v", err)
	}
	var chromeOrigin string
	var firefoxOrigin string
	for _, origin := range origins {
		switch {
		case strings.HasPrefix(origin, "chrome-extension://"):
			chromeOrigin = origin
		case strings.HasPrefix(origin, "moz-extension://"):
			firefoxOrigin = origin
		}
	}
	if chromeOrigin == "" || firefoxOrigin == "" {
		t.Fatalf("manifest origins = %v", origins)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", chromeOrigin)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != chromeOrigin {
		t.Fatalf("this extension origin was rejected: %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	// Firefox uses a per-profile UUID rather than the Gecko add-on ID as the
	// request origin. Both forms must work, because the extension cannot know
	// which UUID the browser will assign.
	for _, origin := range []string{
		firefoxOrigin,
		"moz-extension://11111111-2222-3333-4444-555555555555",
	} {
		req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
		req.Header.Set("Origin", origin)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Fatalf("Firefox extension origin %q was rejected: %s", origin, rec.Header().Get("Access-Control-Allow-Origin"))
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://127.0.0.1:18800")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://127.0.0.1:18800" {
		t.Fatalf("loopback origin was rejected: %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://wails.localhost")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Header().Get("Access-Control-Allow-Origin") != "http://wails.localhost" {
		t.Fatalf("Wails bootstrap origin was rejected: %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}

	req = httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req.Header.Set("Origin", "http://wails.localhost.evil.example")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("lookalike Wails origin was allowed: %d", rec.Code)
	}

	// 3. A website origin is rejected.
	req2 := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	req2.Header.Set("Origin", "https://malicious-site.com")
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for untrusted origin, got %d", rec2.Code)
	}
	if rec2.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("expected empty allow-origin for untrusted origin, got: %s", rec2.Header().Get("Access-Control-Allow-Origin"))
	}
}

// TestForeignOriginCannotTriggerStateChanges is the CSRF regression test.
// A cross-site "simple" POST reaches the handler even when CORS would hide the
// response, so the server must reject the request before the handler runs.
func TestForeignOriginCannotTriggerStateChanges(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	quit := false
	srv.SetQuitHandler(func() { quit = true })
	handler := srv.Routes()

	payload := `{"url":"https://example.com/evil.bin","filename":"evil.bin"}`
	req := httptest.NewRequest(http.MethodPost, "/api/downloads", strings.NewReader(payload))
	req.RemoteAddr = "127.0.0.1:4000"
	req.Header.Set("Origin", "https://malicious-site.com")
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-site download, got %d", rec.Code)
	}
	if len(mockRPC.addedURIs) != 0 {
		t.Fatalf("cross-site request reached the engine: %v", mockRPC.addedURIs)
	}

	quitReq := httptest.NewRequest(http.MethodPost, "/api/app/quit", nil)
	quitReq.RemoteAddr = "127.0.0.1:4000"
	quitReq.Header.Set("Origin", "https://malicious-site.com")
	quitRec := httptest.NewRecorder()
	handler.ServeHTTP(quitRec, quitReq)

	if quitRec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for cross-site quit, got %d", quitRec.Code)
	}
	if quit {
		t.Fatal("cross-site request invoked the quit callback")
	}
}

func TestCreateDownload(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	handler := srv.Routes()

	// 1. Valid download creation with headers & filename
	payload := CreateDownloadRequest{
		URL:       "https://example.com/test.zip",
		Filename:  "test.zip",
		Referrer:  "https://example.com/",
		UserAgent: "Mozilla/5.0",
		Cookies:   "sid=123",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/downloads", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["gid"] != "test-gid-999" {
		t.Fatalf("expected gid 'test-gid-999', got %s", resp["gid"])
	}

	if mockRPC.addedOpts == nil || mockRPC.addedOpts.Out != "test.zip" {
		t.Fatalf("expected Out 'test.zip', got %+v", mockRPC.addedOpts)
	}
	if len(mockRPC.addedOpts.Headers) != 3 {
		t.Fatalf("expected 3 headers, got %v", mockRPC.addedOpts.Headers)
	}

	// 2. Missing URL
	badReq := httptest.NewRequest(http.MethodPost, "/api/downloads", bytes.NewReader([]byte(`{"url":""}`)))
	badRec := httptest.NewRecorder()
	handler.ServeHTTP(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty url, got %d", badRec.Code)
	}

	// 3. Unsupported URL schemes must be rejected at the API boundary.
	for _, rawURL := range []string{"file:///etc/passwd", "javascript:alert(1)", "https:///missing-host"} {
		schemePayload := CreateDownloadRequest{URL: rawURL}
		schemeBody, _ := json.Marshal(schemePayload)
		schemeReq := httptest.NewRequest(http.MethodPost, "/api/downloads", bytes.NewReader(schemeBody))
		schemeRec := httptest.NewRecorder()
		handler.ServeHTTP(schemeRec, schemeReq)
		if schemeRec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for URL %q, got %d", rawURL, schemeRec.Code)
		}
	}

	// 4. Path traversal attempt in filename must be rejected with 400
	traversalPayload := CreateDownloadRequest{
		URL:      "https://example.com/exploit.bin",
		Filename: "../../.bashrc",
	}
	tBody, _ := json.Marshal(traversalPayload)
	tReq := httptest.NewRequest(http.MethodPost, "/api/downloads", bytes.NewReader(tBody))
	tRec := httptest.NewRecorder()
	handler.ServeHTTP(tRec, tReq)
	if tRec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal attempt, got %d", tRec.Code)
	}

	// 4. Generic fallback filename "download" should not set opts.Out
	genericPayload := CreateDownloadRequest{
		URL:      "https://example.com/dl?id=123",
		Filename: "download",
	}
	gBody, _ := json.Marshal(genericPayload)
	gReq := httptest.NewRequest(http.MethodPost, "/api/downloads", bytes.NewReader(gBody))
	gRec := httptest.NewRecorder()
	handler.ServeHTTP(gRec, gReq)
	if gRec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for generic filename, got %d", gRec.Code)
	}
	if mockRPC.addedOpts.Out != "" {
		t.Fatalf("expected empty Out for generic filename 'download', got %q", mockRPC.addedOpts.Out)
	}
}

func TestListDownloads(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	mockRPC.activeItems = []aria.DownloadStatus{
		{
			GID:             "gid-active-1",
			Status:          "active",
			TotalLength:     2048,
			CompletedLength: 1024,
			DownloadSpeed:   512,
			Dir:             "/downloads",
			Files: []aria.FileInfo{
				{Path: "/downloads/myfile.bin"},
			},
		},
	}
	mockRPC.stoppedItems = []aria.DownloadStatus{
		{
			GID:             "gid-stopped-2",
			Status:          "complete",
			TotalLength:     4096,
			CompletedLength: 4096,
			Files: []aria.FileInfo{
				{Path: "/downloads/done.iso"},
			},
		},
		{
			GID:             "gid-removed-3",
			Status:          "removed",
			TotalLength:     1024,
			CompletedLength: 512,
			Files: []aria.FileInfo{
				{Path: "/downloads/removed.bin"},
			},
		},
	}

	handler := srv.Routes()

	req := httptest.NewRequest(http.MethodGet, "/api/downloads", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var items []DownloadItem
	if err := json.Unmarshal(rec.Body.Bytes(), &items); err != nil {
		t.Fatalf("failed to decode list downloads response: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 downloads, got %d", len(items))
	}
	if items[0].Filename != "myfile.bin" || items[0].Status != "active" {
		t.Fatalf("unexpected active download item: %+v", items[0])
	}
	if items[1].Filename != "done.iso" || items[1].Status != "complete" {
		t.Fatalf("unexpected stopped download item: %+v", items[1])
	}
}

// pagedClient serves stopped downloads in fixed pages and ignores the num
// argument, mimicking an engine that returns a full page.
type pagedClient struct {
	*mockAriaClient
	stoppedPages [][]aria.DownloadStatus
}

func (c *pagedClient) TellStopped(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error) {
	page := offset / downloadListPageSize
	if page < 0 || page >= len(c.stoppedPages) {
		return nil, nil
	}
	return c.stoppedPages[page], nil
}

func TestListDownloadsPaginatesStoppedResults(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	first := make([]aria.DownloadStatus, downloadListPageSize)
	for i := range first {
		first[i] = aria.DownloadStatus{
			GID:    fmt.Sprintf("gid-%03d", i),
			Status: "error",
		}
	}
	srv.rpcClient = &pagedClient{
		mockAriaClient: mockRPC,
		stoppedPages: [][]aria.DownloadStatus{
			first,
			{{GID: "gid-last", Status: "error"}},
		},
	}

	items, err := srv.getDownloadItems(context.Background())
	if err != nil {
		t.Fatalf("getDownloadItems failed: %v", err)
	}
	if len(items) != downloadListPageSize+1 {
		t.Fatalf("expected %d downloads across two pages, got %d", downloadListPageSize+1, len(items))
	}
	if items[len(items)-1].GID != "gid-last" {
		t.Fatalf("last item = %q, want gid-last", items[len(items)-1].GID)
	}
}

func TestDownloadActions(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	handler := srv.Routes()

	// 1. Pause action
	req := httptest.NewRequest(http.MethodPost, "/api/downloads/gid-123/action", bytes.NewReader([]byte(`{"action":"pause"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if mockRPC.pausedGID != "gid-123" {
		t.Fatalf("expected paused GID 'gid-123', got %s", mockRPC.pausedGID)
	}

	// 2. Resume action
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-123/action", bytes.NewReader([]byte(`{"action":"resume"}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if mockRPC.unpausedGID != "gid-123" {
		t.Fatalf("expected unpaused GID 'gid-123', got %s", mockRPC.unpausedGID)
	}

	// 3. Remove action
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-123/action", bytes.NewReader([]byte(`{"action":"remove"}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if mockRPC.removedGID != "gid-123" {
		t.Fatalf("expected removed GID 'gid-123', got %s", mockRPC.removedGID)
	}

	// 3b. Remove action with deleteFile: true (removes both file and .aria2)
	tmpDir, err := os.MkdirTemp("", "conduit-remove-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	testFilePath := filepath.Join(tmpDir, "sample.bin")
	ariaFilePath := testFilePath + ".aria2"
	_ = os.WriteFile(testFilePath, []byte("download data"), 0644)
	_ = os.WriteFile(ariaFilePath, []byte("aria chunk data"), 0644)

	mockRPC.activeItems = []aria.DownloadStatus{
		{
			GID: "gid-file-test",
			Files: []aria.FileInfo{
				{Path: testFilePath},
			},
		},
	}

	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-file-test/action", bytes.NewReader([]byte(`{"action":"remove","deleteFile":true}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for remove with deleteFile, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(testFilePath); !os.IsNotExist(err) {
		t.Fatalf("expected test file to be deleted, got err: %v", err)
	}
	if _, err := os.Stat(ariaFilePath); !os.IsNotExist(err) {
		t.Fatalf("expected aria2 control file to be deleted, got err: %v", err)
	}

	// 3c. Remove action with deleteFile: true when main file is already absent (should still delete .aria2)
	_ = os.WriteFile(ariaFilePath, []byte("lingering aria chunk"), 0644)
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-file-test/action", bytes.NewReader([]byte(`{"action":"remove","deleteFile":true}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for remove with lingering .aria2, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(ariaFilePath); !os.IsNotExist(err) {
		t.Fatalf("expected lingering .aria2 file to be deleted even if main file was absent, got err: %v", err)
	}

	// 3d. A failed removal must not delete the file, because the transfer may
	// still be running and writing to it.
	keepFilePath := filepath.Join(tmpDir, "keep.bin")
	keepAriaPath := keepFilePath + ".aria2"
	_ = os.WriteFile(keepFilePath, []byte("still downloading"), 0644)
	_ = os.WriteFile(keepAriaPath, []byte("aria chunk data"), 0644)
	mockRPC.activeItems = []aria.DownloadStatus{
		{
			GID:   "gid-remove-fails",
			Files: []aria.FileInfo{{Path: keepFilePath}},
		},
	}
	mockRPC.removeErr = errors.New("engine is busy")
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-remove-fails/action", bytes.NewReader([]byte(`{"action":"remove","deleteFile":true}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	mockRPC.removeErr = nil
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for failed remove, got %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(keepFilePath); err != nil {
		t.Fatalf("file was deleted after a failed remove: %v", err)
	}
	if _, err := os.Stat(keepAriaPath); err != nil {
		t.Fatalf("control file was deleted after a failed remove: %v", err)
	}
	mockRPC.activeItems = nil

	// 4. Move up and move down queue actions
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-123/action", bytes.NewReader([]byte(`{"action":"move_up"}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for move_up, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-123/action", bytes.NewReader([]byte(`{"action":"move_down"}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for move_down, got %d", rec.Code)
	}

	// 5. Change download directory mid-download
	newTargetDir := filepath.Join(tmpDir, "new_downloads_folder")
	moveBody, err := json.Marshal(map[string]string{
		"action": "change_dir",
		"newDir": newTargetDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-file-test/action", bytes.NewReader(moveBody))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for change_dir, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Invalid action
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-123/action", bytes.NewReader([]byte(`{"action":"fly"}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d", rec.Code)
	}
}

func TestConfigEndpoints(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	handler := srv.Routes()

	// 1. GET /api/config
	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	var current config.Config
	_ = json.Unmarshal(rec.Body.Bytes(), &current)
	if current.MaxConnections != config.DefaultMaxConnections {
		t.Fatalf("expected default connections %d, got %d", config.DefaultMaxConnections, current.MaxConnections)
	}

	// 2. PUT /api/config
	updatePayload := config.Config{
		DownloadDir:            "/new/downloads/folder",
		MaxConnections:         12,
		MaxConcurrentDownloads: 4,
		AutoStart:              false,
		MaxDownloadLimit:       "2M",
	}
	updateBody, _ := json.Marshal(updatePayload)
	putReq := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(updateBody))
	putRec := httptest.NewRecorder()
	handler.ServeHTTP(putRec, putReq)

	if putRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update, got %d", putRec.Code)
	}
	var updated config.Config
	_ = json.Unmarshal(putRec.Body.Bytes(), &updated)
	if updated.MaxConnections != 12 || updated.MaxConcurrentDownloads != 4 || updated.MaxDownloadLimit != "2M" {
		t.Fatalf("unexpected updated config: %+v", updated)
	}

	// Verify aria2 options were synchronized
	if mockRPC.changedOptions == nil || mockRPC.changedOptions["max-connection-per-server"] != "12" || mockRPC.changedOptions["max-overall-download-limit"] != "2M" {
		t.Fatalf("expected aria2 options to synchronize max-overall-download-limit: %+v", mockRPC.changedOptions)
	}
}

func TestStaticWebDashboard(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	handler := srv.Routes()

	// 1. GET / serves index.html
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Conduit") || !strings.Contains(body, "transferTable") {
		t.Fatalf("expected index.html content, got: %s", body[:min(len(body), 200)])
	}

	// 2. GET /styles.css serves CSS
	req = httptest.NewRequest(http.MethodGet, "/styles.css", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /styles.css, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--bg-primary") {
		t.Fatalf("expected styles.css content, got: %s", rec.Body.String()[:min(len(rec.Body.String()), 200)])
	}

	// 3. GET /app.js serves JavaScript
	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /app.js, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "initConnection") {
		t.Fatalf("expected app.js content, got: %s", rec.Body.String()[:min(len(rec.Body.String()), 200)])
	}

	// 4. SPA route fallback (e.g. /active) serves index.html
	req = httptest.NewRequest(http.MethodGet, "/active", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SPA fallback /active, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Conduit") {
		t.Fatalf("expected SPA fallback to index.html, got: %s", rec.Body.String()[:min(len(rec.Body.String()), 200)])
	}
}

func TestHTMLContentGuard(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	tmpDir, err := os.MkdirTemp("", "conduit-html-guard-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Create an unexpected HTML error page file saved with a hash name
	htmlErrorFile := filepath.Join(tmpDir, "b9b6daba29ced239bd9ad0bcf7c83ac46a5321b7468da74c")
	htmlContent := `<!DOCTYPE html><html><head><title>Ops matey no downloads here - AnkerGames</title></head><body><p>This download link has expired.</p></body></html>`
	if err := os.WriteFile(htmlErrorFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("failed to write test html file: %v", err)
	}

	status := aria.DownloadStatus{
		GID:         "gid-html-err-1",
		Status:      "complete",
		TotalLength: int64(len(htmlContent)),
		Dir:         tmpDir,
		Files: []aria.FileInfo{
			{Path: htmlErrorFile},
		},
	}

	item := srv.mapToDownloadItem(status)
	if item.Status != "error" {
		t.Fatalf("expected status 'error' for unexpected HTML file, got %q", item.Status)
	}
	if !strings.Contains(item.ErrorMessage, "Ops matey no downloads here - AnkerGames") {
		t.Fatalf("expected error message to contain page title, got %q", item.ErrorMessage)
	}

	// 2. Legitimate binary file should remain complete
	binFile := filepath.Join(tmpDir, "archive.zip")
	binContent := []byte{0x50, 0x4B, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00} // PK zip header
	if err := os.WriteFile(binFile, binContent, 0644); err != nil {
		t.Fatalf("failed to write test bin file: %v", err)
	}

	binStatus := aria.DownloadStatus{
		GID:         "gid-bin-ok",
		Status:      "complete",
		TotalLength: int64(len(binContent)),
		Dir:         tmpDir,
		Files: []aria.FileInfo{
			{Path: binFile},
		},
	}

	binItem := srv.mapToDownloadItem(binStatus)
	if binItem.Status != "complete" {
		t.Fatalf("expected status 'complete' for legitimate binary file, got %q (err: %s)", binItem.Status, binItem.ErrorMessage)
	}

	// 3. Intended HTML file should remain complete
	intendedHTMLFile := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(intendedHTMLFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("failed to write test intended html file: %v", err)
	}

	intendedStatus := aria.DownloadStatus{
		GID:         "gid-html-intended",
		Status:      "complete",
		TotalLength: int64(len(htmlContent)),
		Dir:         tmpDir,
		Files: []aria.FileInfo{
			{Path: intendedHTMLFile},
		},
	}

	intendedItem := srv.mapToDownloadItem(intendedStatus)
	if intendedItem.Status != "complete" {
		t.Fatalf("expected status 'complete' for intended .html file, got %q (err: %s)", intendedItem.Status, intendedItem.ErrorMessage)
	}

	// 4. An XML feed that uses <header> and <title> is not an HTML page. The
	// <head> check must match the tag itself, not the start of <header>.
	feedFile := filepath.Join(tmpDir, "notes.xml")
	feedContent := `<?xml version="1.0"?><rss version="2.0"><channel><title>Release notes</title><header>Notes for this release</header></channel></rss>`
	if err := os.WriteFile(feedFile, []byte(feedContent), 0644); err != nil {
		t.Fatalf("failed to write test feed file: %v", err)
	}

	feedStatus := aria.DownloadStatus{
		GID:         "gid-xml-feed",
		Status:      "complete",
		TotalLength: int64(len(feedContent)),
		Dir:         tmpDir,
		Files: []aria.FileInfo{
			{Path: feedFile},
		},
	}

	feedItem := srv.mapToDownloadItem(feedStatus)
	if feedItem.Status != "complete" {
		t.Fatalf("expected status 'complete' for XML feed with <header>, got %q (err: %s)", feedItem.Status, feedItem.ErrorMessage)
	}
}

func TestOpenFolderRequiresDownloadPath(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	var opened string
	old := openFolder
	openFolder = func(path string) error {
		opened = path
		return nil
	}
	t.Cleanup(func() { openFolder = old })

	mockRPC.stoppedItems = []aria.DownloadStatus{
		{GID: "gid-no-path", Status: "complete"},
	}

	handler := srv.Routes()
	req := httptest.NewRequest(http.MethodPost, "/api/downloads/gid-no-path/action", bytes.NewReader([]byte(`{"action":"open_folder"}`)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatal("expected an error when the download has no file path and no directory")
	}
	if opened != "" {
		t.Fatalf("opened %q; the global download directory must stay unused", opened)
	}

	downloadDir := filepath.Join(t.TempDir(), "downloads")
	mockRPC.stoppedItems = []aria.DownloadStatus{
		{GID: "gid-dir-only", Status: "complete", Dir: downloadDir},
	}
	req = httptest.NewRequest(http.MethodPost, "/api/downloads/gid-dir-only/action", bytes.NewReader([]byte(`{"action":"open_folder"}`)))
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when the download directory is known, got %d: %s", rec.Code, rec.Body.String())
	}
	if opened != downloadDir {
		t.Fatalf("opened %q, want download directory %q", opened, downloadDir)
	}
}

// Retry must re-send the cookies, referrer, and user agent, or downloads
// behind a login fail again as soon as they are retried.
func TestRetryKeepsRequestHeaders(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	mockRPC.stoppedItems = []aria.DownloadStatus{{
		GID:    "gid-err",
		Status: "error",
		Dir:    "/dl",
		Files: []aria.FileInfo{{
			Path: "/dl/private.zip",
			URIs: []aria.FileURI{{URI: "https://files.example.com/private.zip"}},
		}},
	}}
	mockRPC.options = map[string]map[string]string{
		"gid-err": {"header": "Cookie: session=abc\nReferer: https://example.com/\nUser-Agent: Test/1\n"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/downloads/gid-err/action", strings.NewReader(`{"action":"retry"}`))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("retry status = %d: %s", rec.Code, rec.Body.String())
	}
	if mockRPC.addedOpts == nil {
		t.Fatal("retry did not re-add the download")
	}
	want := []string{"Cookie: session=abc", "Referer: https://example.com/", "User-Agent: Test/1"}
	if strings.Join(mockRPC.addedOpts.Headers, "|") != strings.Join(want, "|") {
		t.Fatalf("headers = %q, want %q", mockRPC.addedOpts.Headers, want)
	}
	if mockRPC.addedOpts.Out != "private.zip" || mockRPC.addedOpts.Dir != "/dl" {
		t.Fatalf("retry lost the destination: %+v", mockRPC.addedOpts)
	}
}
