package server

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// The NumberFlow module is vendored, so it must be served from the binary as JavaScript and
// never fall back to the dashboard page.
func TestVendoredNumberFlowIsServedAsScript(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/vendor/number-flow/number-flow.js", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Fatalf("content type %q, want a JavaScript type", ct)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<!DOCTYPE") || strings.Contains(body, "<html") {
		t.Fatal("vendored script fell back to the dashboard page")
	}
	if !strings.Contains(body, "number-flow") {
		t.Fatal("vendored script does not define the number-flow element")
	}
}

// The dashboard loads nothing from a CDN or any other host.
func TestDashboardLoadsNoExternalScriptsOrStyles(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	srv.Routes().ServeHTTP(rec, req)

	external := regexp.MustCompile(`<(script|link)[^>]+(src|href)="(https?:)?//`)
	if m := external.FindString(rec.Body.String()); m != "" {
		t.Fatalf("dashboard loads from an external host: %s", m)
	}
}
