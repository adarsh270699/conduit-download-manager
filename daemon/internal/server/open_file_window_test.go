package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/config"
)

func postAction(t *testing.T, srv *Server, gid, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/downloads/"+gid+"/action", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	return rec
}

func TestOpenFileLaunchesOnlyCompleteExistingFiles(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	var opened string
	old := openFile
	openFile = func(path string) error {
		opened = path
		return nil
	}
	t.Cleanup(func() { openFile = old })

	file := filepath.Join(t.TempDir(), "setup.exe")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	mockRPC.stoppedItems = []aria.DownloadStatus{
		{GID: "done", Status: "complete", Files: []aria.FileInfo{{Path: file}}},
		{GID: "gone", Status: "complete", Files: []aria.FileInfo{{Path: file + ".missing"}}},
		{GID: "failed", Status: "error", Files: []aria.FileInfo{{Path: file}}},
	}

	if rec := postAction(t, srv, "done", `{"action":"open_file"}`); rec.Code != http.StatusOK {
		t.Fatalf("complete file: got %d: %s", rec.Code, rec.Body.String())
	}
	if opened != file {
		t.Fatalf("opened %q, want %q", opened, file)
	}

	opened = ""
	for _, gid := range []string{"gone", "failed"} {
		if rec := postAction(t, srv, gid, `{"action":"open_file"}`); rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: got %d, want 400", gid, rec.Code)
		}
	}
	if opened != "" {
		t.Fatalf("opened %q for an unavailable file", opened)
	}
}

func TestDownloadItemCarriesSourceURL(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	item := srv.mapToDownloadItem(aria.DownloadStatus{
		GID:    "g",
		Status: "active",
		Files:  []aria.FileInfo{{URIs: []aria.FileURI{{URI: "https://example.com/a.zip"}}}},
	})
	if item.URL != "https://example.com/a.zip" {
		t.Fatalf("URL = %q", item.URL)
	}
}

func TestObserveDownloadsShowsWindowOnEnabledEvents(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()

	shown := 0
	srv.SetShowHandler(func() { shown++ })
	if _, err := cfgMgr.Patch([]byte(`{"showOnStart":true,"showOnComplete":true,"showOnError":true}`)); err != nil {
		t.Fatal(err)
	}

	// Baseline: what already exists at startup never pops the window.
	srv.observeDownloads([]DownloadItem{{GID: "old", Status: "error"}, {GID: "run", Status: "active"}})
	if shown != 0 {
		t.Fatalf("baseline showed the window %d times", shown)
	}

	// A new download starts.
	list := []DownloadItem{{GID: "old", Status: "error"}, {GID: "run", Status: "active"}, {GID: "new", Status: "active"}}
	srv.observeDownloads(list)
	if shown != 1 {
		t.Fatalf("start: shown = %d, want 1", shown)
	}

	// Unchanged list: nothing.
	srv.observeDownloads(list)
	if shown != 1 {
		t.Fatalf("repeat: shown = %d, want 1", shown)
	}

	// One finishes and one fails in the same tick: the window shows once.
	srv.observeDownloads([]DownloadItem{{GID: "old", Status: "error"}, {GID: "run", Status: "complete"}, {GID: "new", Status: "error"}})
	if shown != 2 {
		t.Fatalf("finish and fail in one tick: shown = %d, want 2", shown)
	}
}

func TestObserveDownloadsEventsAreIndependent(t *testing.T) {
	cases := []struct {
		name  string
		patch string
		step  []DownloadItem
		want  int
	}{
		{"start off", `{"showOnStart":false}`, []DownloadItem{{GID: "a", Status: "active"}, {GID: "n", Status: "active"}}, 0},
		{"complete off", `{"showOnComplete":false}`, []DownloadItem{{GID: "a", Status: "complete"}}, 0},
		{"error off", `{"showOnError":false}`, []DownloadItem{{GID: "a", Status: "error"}}, 0},
		{"complete still on", `{"showOnStart":false,"showOnError":false}`, []DownloadItem{{GID: "a", Status: "complete"}}, 1},
		{"added already failed is an error, not a start", `{"showOnError":false}`, []DownloadItem{{GID: "a", Status: "active"}, {GID: "x", Status: "error"}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, _, cfgMgr, cleanup := setupTestServer(t)
			defer cleanup()
			shown := 0
			srv.SetShowHandler(func() { shown++ })
			if _, err := cfgMgr.Patch([]byte(tc.patch)); err != nil {
				t.Fatal(err)
			}
			srv.observeDownloads([]DownloadItem{{GID: "a", Status: "active"}})
			srv.observeDownloads(tc.step)
			if shown != tc.want {
				t.Fatalf("shown = %d, want %d", shown, tc.want)
			}
		})
	}
}

func TestWindowEventsNeedAHandlerAndAnEnabledEvent(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()

	if srv.windowEventsWanted() {
		t.Fatal("a headless daemon has no window to show")
	}
	srv.SetShowHandler(func() {})
	if !srv.windowEventsWanted() {
		t.Fatal("events default to on once a window exists")
	}
	if _, err := cfgMgr.Patch([]byte(`{"showOnStart":false,"showOnComplete":false,"showOnError":false}`)); err != nil {
		t.Fatal(err)
	}
	if srv.windowEventsWanted() {
		t.Fatal("all events off means no polling")
	}
	srv.forgetObserved()
	if srv.seenStatus != nil {
		t.Fatal("baseline should be dropped")
	}
}

func TestConfigDefaultsShowEventsOnForOldFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	old := `{"downloadDir":"` + strings.ReplaceAll(t.TempDir(), `\`, `\\`) + `","maxConnections":8,"maxConcurrentDownloads":3}`
	if err := os.WriteFile(path, []byte(old), 0644); err != nil {
		t.Fatal(err)
	}
	mgr, err := config.NewManager(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c := mgr.Get(); !c.ShowOnStart || !c.ShowOnComplete || !c.ShowOnError {
		t.Fatalf("old file should default all three on: %+v", c)
	}

	// An explicit false survives a reload while the other keys stay on.
	if _, err := mgr.Patch([]byte(`{"showOnComplete":false}`)); err != nil {
		t.Fatal(err)
	}
	mgr2, err := config.NewManager(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if c := mgr2.Get(); c.ShowOnComplete || !c.ShowOnStart || !c.ShowOnError {
		t.Fatalf("explicit false was lost or others changed: %+v", c)
	}
}

// The window must pop up with no dashboard connected: a hidden tray window has
// no reason to hold a WebSocket open.
func TestBroadcastAllShowsWindowWithoutClients(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	shown := 0
	srv.SetShowHandler(func() { shown++ })
	ctx := context.Background()

	srv.wsHub.BroadcastAll(ctx) // baseline: empty list
	mockRPC.activeItems = []aria.DownloadStatus{{GID: "g1", Status: "active"}}
	srv.wsHub.BroadcastAll(ctx)
	if shown != 1 {
		t.Fatalf("new download: shown = %d, want 1", shown)
	}

	mockRPC.activeItems = nil
	mockRPC.stoppedItems = []aria.DownloadStatus{{GID: "g1", Status: "complete", Files: []aria.FileInfo{{Path: filepath.Join(t.TempDir(), "a.bin")}}}}
	srv.wsHub.BroadcastAll(ctx)
	if shown != 2 {
		t.Fatalf("finished download: shown = %d, want 2", shown)
	}
}

// The dashboard words its toast from these events, so they must say which
// download did what, and only for events that are switched on.
func TestObserveDownloadsReturnsTheEventsItActedOn(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()

	srv.SetShowHandler(func() {})
	if _, err := cfgMgr.Patch([]byte(`{"showOnStart":false,"showOnComplete":true,"showOnError":true}`)); err != nil {
		t.Fatal(err)
	}
	srv.observeDownloads([]DownloadItem{{GID: "run", Status: "active", Filename: "run.bin"}})

	events := srv.observeDownloads([]DownloadItem{
		{GID: "run", Status: "complete", Filename: "run.bin"},
		{GID: "bad", Status: "error", Filename: "bad.bin", ErrorMessage: "404 Not Found"},
		{GID: "new", Status: "active", Filename: "new.bin"}, // start is off
	})
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2: %+v", len(events), events)
	}
	if events[0] != (DownloadEvent{Event: "complete", GID: "run", Filename: "run.bin"}) {
		t.Fatalf("first event = %+v", events[0])
	}
	if events[1] != (DownloadEvent{Event: "error", GID: "bad", Filename: "bad.bin", ErrorMessage: "404 Not Found"}) {
		t.Fatalf("second event = %+v", events[1])
	}
	if got := srv.observeDownloads([]DownloadItem{{GID: "run", Status: "complete"}}); len(got) != 0 {
		t.Fatalf("unchanged list produced events: %+v", got)
	}
}
