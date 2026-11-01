package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The theme is part of the settings API. The dashboard, the extension popup and the offline page
// all read and save it through /api/config.
func TestThemeSettingRoundTripsThroughAPI(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	put := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"theme":"dark"}`))
	put.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, put)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/config status %d: %s", rec.Code, rec.Body.String())
	}

	get := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, get)
	var cfg struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("GET /api/config body: %v", err)
	}
	if cfg.Theme != "dark" {
		t.Fatalf("theme after PUT = %q, want %q", cfg.Theme, "dark")
	}
}

// An unknown scheme is not stored; the setting falls back to following the system.
func TestThemeSettingRejectsUnknownSchemeInAPI(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	put := httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(`{"theme":"neon"}`))
	put.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, put)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status %d: %s", rec.Code, rec.Body.String())
	}
	var cfg struct {
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if cfg.Theme != "system" {
		t.Fatalf("unknown theme stored as %q, want %q", cfg.Theme, "system")
	}
}
