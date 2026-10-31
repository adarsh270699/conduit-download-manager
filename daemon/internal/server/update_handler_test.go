package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"conduit/daemon/internal/updater"
)

// newFakeFeed starts a GitHub stand-in that always reports tag as the latest
// stable release.
func newFakeFeed(t *testing.T, tag string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"` + tag + `","assets":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newUpdateService(t *testing.T, apiBase string) *updater.Service {
	t.Helper()
	svc, err := updater.New(updater.Options{
		ConfigDir:      t.TempDir(),
		APIBase:        apiBase,
		CurrentVersion: "1.0.0",
		Channel:        func() updater.Channel { return updater.ChannelNone },
	})
	if err != nil {
		t.Fatalf("updater.New: %v", err)
	}
	return svc
}

func localRequest(method, path string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	req.RemoteAddr = "127.0.0.1:4000"
	return req
}

func TestUpdateRoutesAreLoopbackOnly(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))
	handler := srv.Routes()

	for _, route := range []struct{ method, path string }{
		{http.MethodGet, "/api/update"},
		{http.MethodPost, "/api/update/check"},
	} {
		req := httptest.NewRequest(route.method, route.path, nil)
		req.RemoteAddr = "203.0.113.8:4000"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s %s from a remote host: status %d, want 403", route.method, route.path, rec.Code)
		}
	}
}

func TestUpdateRoutesReportUnavailableWithoutService(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, localRequest(http.MethodGet, "/api/update"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503 when the update service did not start", rec.Code)
	}
}

func TestUpdateStatusAndCheckRoundTrip(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	srv.SetUpdater(newUpdateService(t, newFakeFeed(t, "v9.9.9").URL))
	handler := srv.Routes()

	idle := httptest.NewRecorder()
	handler.ServeHTTP(idle, localRequest(http.MethodGet, "/api/update"))
	if idle.Code != http.StatusOK {
		t.Fatalf("GET status %d", idle.Code)
	}
	var before map[string]any
	if err := json.Unmarshal(idle.Body.Bytes(), &before); err != nil {
		t.Fatalf("GET body is not JSON: %v", err)
	}
	if before["state"] != "idle" || before["currentVersion"] != "1.0.0" {
		t.Fatalf("GET body: %s", idle.Body.String())
	}

	checked := httptest.NewRecorder()
	handler.ServeHTTP(checked, localRequest(http.MethodPost, "/api/update/check"))
	if checked.Code != http.StatusOK {
		t.Fatalf("POST check status %d body %s", checked.Code, checked.Body.String())
	}
	var after map[string]any
	if err := json.Unmarshal(checked.Body.Bytes(), &after); err != nil {
		t.Fatalf("check body is not JSON: %v", err)
	}
	if after["state"] != "available" || after["latestVersion"] != "9.9.9" {
		t.Fatalf("check must return the result, got %s", checked.Body.String())
	}
	if !strings.Contains(checked.Body.String(), `"lastCheckedAt"`) {
		t.Fatalf("check body must include lastCheckedAt: %s", checked.Body.String())
	}
}
