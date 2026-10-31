package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDownloadRouteIsLoopbackOnly(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))

	req := httptest.NewRequest(http.MethodPost, "/api/update/download", nil)
	req.RemoteAddr = "203.0.113.8:4000"
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("remote download status %d, want 403", rec.Code)
	}
}

// Nothing has been checked yet, so there is no release to download.
func TestDownloadRouteConflictsWhenNothingAvailable(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))

	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, localRequest(http.MethodPost, "/api/update/download"))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status %d, want 409 before any check found a release", rec.Code)
	}
	if rec.Body.Len() == 0 {
		t.Fatal("a 409 must say why")
	}
}

// After a check finds a newer release, the download is accepted and runs in
// the background. The fake feed lists no assets, so the transfer fails there;
// the test checks only the acceptance.
func TestDownloadRouteAcceptsWhenAvailable(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))
	handler := srv.Routes()

	checked := httptest.NewRecorder()
	handler.ServeHTTP(checked, localRequest(http.MethodPost, "/api/update/check"))
	if checked.Code != http.StatusOK {
		t.Fatalf("check status %d", checked.Code)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, localRequest(http.MethodPost, "/api/update/download"))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("download status %d body %s, want 202", rec.Code, rec.Body.String())
	}
}
