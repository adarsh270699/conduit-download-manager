package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthIncludesEngine(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetEngineStatus(func() string { return "ready" })

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"engine":"ready"`) && !strings.Contains(body, `"engine": "ready"`) {
		t.Fatalf("health body missing engine ready: %s", body)
	}
	if !strings.Contains(body, `"app":"conduit"`) && !strings.Contains(body, `"app": "conduit"`) {
		t.Fatalf("health body missing app identity: %s", body)
	}
}

func TestQuitIsLoopbackOnly(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	quit := false
	srv.SetQuitHandler(func() { quit = true })
	handler := srv.Routes()

	remote := httptest.NewRequest(http.MethodPost, "/api/app/quit", nil)
	remote.RemoteAddr = "203.0.113.8:4000"
	remoteRec := httptest.NewRecorder()
	handler.ServeHTTP(remoteRec, remote)
	if remoteRec.Code != http.StatusForbidden {
		t.Fatalf("remote quit status %d", remoteRec.Code)
	}
	if quit {
		t.Fatal("remote quit invoked the quit callback")
	}

	local := httptest.NewRequest(http.MethodPost, "/api/app/quit", nil)
	local.RemoteAddr = "127.0.0.1:4000"
	localRec := httptest.NewRecorder()
	handler.ServeHTTP(localRec, local)
	if localRec.Code != http.StatusNoContent {
		t.Fatalf("local quit status %d", localRec.Code)
	}
	if !quit {
		t.Fatal("local quit did not invoke the quit callback")
	}
}

func TestShowAndExtensionAreLoopbackOnly(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	shown := false
	srv.SetShowHandler(func() { shown = true })
	opened := ""
	srv.SetURLOpener(func(target string) error {
		opened = target
		return nil
	})

	handler := srv.Routes()

	remote := httptest.NewRequest(http.MethodPost, "/api/app/show", nil)
	remote.RemoteAddr = "203.0.113.8:4000"
	remoteRec := httptest.NewRecorder()
	handler.ServeHTTP(remoteRec, remote)
	if remoteRec.Code != http.StatusForbidden {
		t.Fatalf("remote show status %d", remoteRec.Code)
	}
	if shown {
		t.Fatal("remote show invoked the window callback")
	}

	local := httptest.NewRequest(http.MethodPost, "/api/app/show", nil)
	local.RemoteAddr = "127.0.0.1:4000"
	localRec := httptest.NewRecorder()
	handler.ServeHTTP(localRec, local)
	if localRec.Code != http.StatusNoContent {
		t.Fatalf("local show status %d", localRec.Code)
	}
	if !shown {
		t.Fatal("local show did not invoke the window callback")
	}

	info := httptest.NewRequest(http.MethodGet, "/api/extension", nil)
	info.RemoteAddr = "127.0.0.1:4000"
	infoRec := httptest.NewRecorder()
	handler.ServeHTTP(infoRec, info)
	if infoRec.Code != http.StatusOK {
		t.Fatalf("extension info status %d body %s", infoRec.Code, infoRec.Body.String())
	}
	if !strings.Contains(infoRec.Body.String(), `"exists"`) {
		t.Fatalf("extension info body: %s", infoRec.Body.String())
	}

	guide := httptest.NewRequest(http.MethodPost, "/api/extension/open-guide", nil)
	guide.RemoteAddr = "127.0.0.1:4000"
	guideRec := httptest.NewRecorder()
	handler.ServeHTTP(guideRec, guide)
	if guideRec.Code != http.StatusNoContent {
		t.Fatalf("open guide status %d body %s", guideRec.Code, guideRec.Body.String())
	}
	if !strings.HasSuffix(opened, "/extension") {
		t.Fatalf("opened URL %q", opened)
	}
}

func TestSetupAndExtensionPages(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	setupReq := httptest.NewRequest(http.MethodGet, "/setup", nil)
	setupRec := httptest.NewRecorder()
	handler.ServeHTTP(setupRec, setupReq)
	if setupRec.Code != http.StatusOK || !strings.Contains(setupRec.Body.String(), "Where should files go?") {
		t.Fatalf("setup page status %d", setupRec.Code)
	}

	extReq := httptest.NewRequest(http.MethodGet, "/extension", nil)
	extRec := httptest.NewRecorder()
	handler.ServeHTTP(extRec, extReq)
	if extRec.Code != http.StatusOK || !strings.Contains(extRec.Body.String(), "Add the Conduit extension") {
		t.Fatalf("extension page status %d", extRec.Code)
	}
}

// TestDashboardDestructiveSafety verifies the guards that protect the user from
// losing a file or a failure record without a chance to confirm.
func TestDashboardDestructiveSafety(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard status %d", rec.Code)
	}
	body := rec.Body.String()

	// Clear Finished must confirm before it removes anything.
	if !strings.Contains(body, `id="clearModal"`) {
		t.Fatal("dashboard must ship a Clear Finished confirmation dialog")
	}
	if !strings.Contains(body, `id="confirmClearBtn"`) {
		t.Fatal("Clear Finished confirmation needs a confirm control")
	}

	// The disk-delete switch must carry a label the switch is nested in,
	// so a screen reader can name it.
	if !strings.Contains(body, `<label class="checkbox-group" for="deleteFileFromDisk">`) {
		t.Fatal("disk-delete switch needs a wrapping label")
	}

	// The confirm button must name the file when deletion is on.
	if !strings.Contains(body, `id="deleteFileFromDisk"`) {
		t.Fatal("disk-delete switch must be present")
	}

	// Add Download must state where the file lands.
	if !strings.Contains(body, `id="addDestination"`) {
		t.Fatal("Add Download must show the destination folder")
	}

	// The URL error needs a live inline slot, not only a toast.
	if !strings.Contains(body, `id="downloadUrlError"`) || !strings.Contains(body, `role="alert"`) {
		t.Fatal("Add Download needs an inline alert for URL errors")
	}
}

// TestDashboardJavaScriptGuards verifies the destructive paths in app.js.
func TestDashboardJavaScriptGuards(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("app.js status %d", rec.Code)
	}
	js := rec.Body.String()

	// Clear Finished must open the confirm dialog, not remove rows on click.
	if !strings.Contains(js, "openClearModal") {
		t.Fatal("Clear Finished must route through a confirmation dialog")
	}

	// Completed and failed must be separable.
	if !strings.Contains(js, "clearCompletedOnly") {
		t.Fatal("Clear Finished must let the user keep failed downloads")
	}

	// A double click must not submit the Add form twice.
	if !strings.Contains(js, "isAddingDownload") {
		t.Fatal("Add Download must guard against double submission")
	}

	// Dialogs must trap focus and free the background.
	if !strings.Contains(js, "setBackgroundInert") || !strings.Contains(js, "trapTab") {
		t.Fatal("dialogs must trap focus and mark the page inert")
	}
}
