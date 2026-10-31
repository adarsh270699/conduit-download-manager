package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstallRouteIsLoopbackOnly(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))

	req := httptest.NewRequest(http.MethodPost, "/api/update/install", nil)
	req.RemoteAddr = "203.0.113.8:4000"
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("remote install status %d, want 403", rec.Code)
	}
}

// With nothing downloaded there is nothing to install, so the route refuses
// and does not quit the application.
func TestInstallRouteConflictsWhenNothingDownloaded(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))
	quit := false
	srv.SetQuitHandler(func() { quit = true })

	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, localRequest(http.MethodPost, "/api/update/install"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409 with nothing downloaded", rec.Code)
	}
	if quit {
		t.Fatal("a refused install must not quit Conduit")
	}
}
