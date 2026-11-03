package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"conduit/daemon/internal/aria"
)

// TestMain keeps the handler tests off the network. Cookie scope has its own
// tests against local servers.
func TestMain(m *testing.M) {
	resolveCookieScope = func(_ context.Context, rawURL string, _ []string) (string, bool) {
		return rawURL, true
	}
	os.Exit(m.Run())
}

func request(handler http.Handler, method, path, origin, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func TestClassifyOriginTrustsOnlyThisDaemonsPort(t *testing.T) {
	for origin, want := range map[string]originKind{
		"":                                 originTrusted,
		"http://127.0.0.1:18800":           originTrusted,
		"http://localhost:18800":           originTrusted,
		"http://127.0.0.1:3000":            originDenied,
		"http://localhost:5173":            originDenied,
		"http://127.0.0.1":                 originDenied,
		"http://wails.localhost:3000":      originDenied,
		"http://127.0.0.1:18800.evil.test": originDenied,
		"chrome-extension://aaaaaaaaaaaa":  originDenied,
		"null":                             originDenied,
		"moz-extension://1111-2222":        originFirefox,
		"http://wails.localhost":           originShell,
		"wails://wails":                    originShell,
	} {
		if got := classifyOrigin(18800, origin); got != want {
			t.Errorf("classifyOrigin(%q) = %d, want %d", origin, got, want)
		}
	}
}

func TestLocalWebPageCannotDriveTheAPI(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/config", `{"theme":"dark"}`},
		{http.MethodPost, "/api/downloads", `{"url":"http://x.test/a.exe"}`},
		{http.MethodPost, "/api/downloads/g1/action", `{"action":"open_file","confirmed":true}`},
		{http.MethodGet, "/api/downloads", ""},
	} {
		if rec := request(handler, tc.method, tc.path, "http://localhost:3000", tc.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s from another local port: got %d, want 403", tc.method, tc.path, rec.Code)
		}
	}
}

func TestFirefoxOriginGetsOnlyWhatTheExtensionUses(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()
	mockRPC.stoppedItems = []aria.DownloadStatus{{GID: "g1", Status: "paused"}}
	const firefox = "moz-extension://11111111-2222-3333-4444-555555555555"

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/health", ""},
		{http.MethodGet, "/api/config", ""},
		{http.MethodPut, "/api/config", `{"theme":"dark"}`},
		{http.MethodGet, "/api/downloads", ""},
		{http.MethodPost, "/api/downloads/g1/action", `{"action":"pause"}`},
	} {
		if rec := request(handler, tc.method, tc.path, firefox, tc.body); rec.Code == http.StatusForbidden {
			t.Errorf("%s %s %s was refused", tc.method, tc.path, tc.body)
		}
	}

	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPut, "/api/config", `{"theme":"dark","downloadDir":"C:\\Windows"}`},
		{http.MethodPut, "/api/config", `{"autoStart":true}`},
		{http.MethodPost, "/api/downloads/g1/action", `{"action":"open_file","confirmed":true}`},
		{http.MethodPost, "/api/downloads/g1/action", `{"action":"open_folder"}`},
		{http.MethodPost, "/api/downloads/g1/action", `{"action":"change_dir","newDir":"C:\\x"}`},
		{http.MethodPost, "/api/downloads/g1/action", `{"action":"remove","deleteFile":true}`},
		{http.MethodPost, "/api/update/install", ""},
		{http.MethodPost, "/api/app/quit", ""},
		{http.MethodGet, "/api/ws", ""},
	} {
		if rec := request(handler, tc.method, tc.path, firefox, tc.body); rec.Code != http.StatusForbidden {
			t.Errorf("%s %s %s: got %d, want 403", tc.method, tc.path, tc.body, rec.Code)
		}
	}
}

func TestWailsBootstrapMayOnlyPollHealth(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	if rec := request(handler, http.MethodGet, "/api/health", "http://wails.localhost", ""); rec.Code != http.StatusOK {
		t.Fatalf("health: got %d", rec.Code)
	}
	if rec := request(handler, http.MethodGet, "/api/config", "http://wails.localhost", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("config: got %d, want 403", rec.Code)
	}
}

func TestEnginePathCannotBeSetThroughTheAPI(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	rec := request(handler, http.MethodPut, "/api/config", "http://127.0.0.1:18800", `{"enginePath":"C:\\evil\\aria2c.exe"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400: %s", rec.Code, rec.Body.String())
	}
	if got := srv.configMgr.Get().EnginePath; got != "" {
		t.Fatalf("EnginePath = %q", got)
	}
}

func TestHostGuardRejectsRebindingNames(t *testing.T) {
	handler := hostGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	for host, want := range map[string]int{
		"127.0.0.1:18800":        http.StatusOK,
		"localhost:18800":        http.StatusOK,
		"LOCALHOST:18800":        http.StatusOK,
		"[::1]:18800":            http.StatusOK,
		"127.0.0.1":              http.StatusOK,
		"evil.example:18800":     http.StatusForbidden,
		"evil.example":           http.StatusForbidden,
		"127.0.0.1.evil.example": http.StatusForbidden,
		"":                       http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Errorf("Host %q: got %d, want %d", host, rec.Code, want)
		}
	}
}

func TestResponsesCarrySecurityHeaders(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	rec := request(srv.Routes(), http.MethodGet, "/", "", "")
	csp := rec.Header().Get("Content-Security-Policy")
	for _, want := range []string{"script-src 'self'", "frame-ancestors 'self' http://wails.localhost", "ws://127.0.0.1:"} {
		if !strings.Contains(csp, want) {
			t.Errorf("CSP %q lacks %q", csp, want)
		}
	}
	if strings.Contains(csp, "script-src 'self' 'unsafe") {
		t.Errorf("CSP allows inline script: %q", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing X-Content-Type-Options")
	}
	if strings.Contains(rec.Body.String(), "<script>") {
		t.Error("the dashboard has an inline script, which its CSP blocks")
	}
}

func TestRequestBodiesAreCapped(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	body := `{"url":"http://x.test/a","cookies":"` + strings.Repeat("a", maxRequestBody) + `"}`
	if rec := request(srv.Routes(), http.MethodPost, "/api/downloads", "", body); rec.Code != http.StatusBadRequest {
		t.Fatalf("got %d, want 400", rec.Code)
	}
}

func TestLineBreaksCannotReachTheEngine(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()
	handler := srv.Routes()

	for _, tc := range []CreateDownloadRequest{
		{URL: "http://x.test/a", Filename: "a.bin\n all-proxy=evil.test:8080"},
		{URL: "http://x.test/a", Cookies: "a=b\nX-Injected: yes"},
		{URL: "http://x.test/a", Referrer: "http://x.test/\r\nX-Injected: yes"},
		{URL: "http://x.test/a", UserAgent: "x\nX-Injected: yes"},
		{URL: "http://x.test/a\n all-proxy=evil.test:8080"},
	} {
		mockRPC.addedOpts = nil
		body, _ := json.Marshal(tc)
		rec := request(handler, http.MethodPost, "/api/downloads", "", string(body))
		if rec.Code != http.StatusBadRequest || mockRPC.addedOpts != nil {
			t.Errorf("%+v: got %d, engine options %+v", tc, rec.Code, mockRPC.addedOpts)
		}
	}

	if rec := request(handler, http.MethodPut, "/api/config", "", `{"downloadDir":"C:\\x\n all-proxy=evil.test:8080"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("downloadDir with a line break: got %d", rec.Code)
	}
	rec := request(handler, http.MethodPost, "/api/downloads/g1/action", "", `{"action":"change_dir","newDir":"C:\\x\n all-proxy=evil.test:8080"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("newDir with a line break: got %d", rec.Code)
	}
}

func TestCleanFilename(t *testing.T) {
	for _, tc := range []struct {
		raw, goos, want string
		bad             bool
	}{
		{raw: "report.pdf", goos: "linux", want: "report.pdf"},
		{raw: "dir/report.pdf", goos: "linux", want: "report.pdf"},
		{raw: "download", goos: "linux", want: ""},
		{raw: "../x", goos: "linux", bad: true},
		{raw: "a\nb", goos: "linux", bad: true},
		{raw: "a\tb", goos: "linux", bad: true},
		{raw: "a.txt:evil.exe", goos: "windows", bad: true},
		{raw: "a?.txt", goos: "windows", bad: true},
		{raw: "con.txt", goos: "windows", bad: true},
		{raw: "NUL", goos: "windows", bad: true},
		{raw: "com1.log", goos: "windows", bad: true},
		{raw: "name.", goos: "windows", bad: true},
		{raw: "console.txt", goos: "windows", want: "console.txt"},
		{raw: "com.txt", goos: "windows", want: "com.txt"},
	} {
		got, err := cleanFilename(tc.raw, tc.goos)
		if tc.bad != (err != nil) || got != tc.want {
			t.Errorf("cleanFilename(%q, %s) = %q, %v", tc.raw, tc.goos, got, err)
		}
	}
}

func TestCookiesStayOnTheirHost(t *testing.T) {
	var otherCookie string
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		otherCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusPartialContent)
	}))
	defer other.Close()
	// httptest listens on 127.0.0.1, so "localhost" is a different host name.
	otherURL := strings.Replace(other.URL, "127.0.0.1", "localhost", 1)

	var sawCookie string
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawCookie = r.Header.Get("Cookie")
		switch r.URL.Path {
		case "/same":
			http.Redirect(w, r, "/file", http.StatusFound)
		case "/cross":
			http.Redirect(w, r, otherURL+"/file", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		default:
			w.WriteHeader(http.StatusPartialContent)
		}
	}))
	defer first.Close()

	headers := []string{"Cookie: SESSION=secret"}
	ctx := context.Background()

	if target, keep := followSameHostRedirects(ctx, first.URL+"/file", headers); !keep || target != first.URL+"/file" {
		t.Errorf("no redirect: %q, %v", target, keep)
	}
	if sawCookie != "SESSION=secret" {
		t.Errorf("first host did not get its cookie: %q", sawCookie)
	}
	if target, keep := followSameHostRedirects(ctx, first.URL+"/same", headers); !keep || target != first.URL+"/same" {
		t.Errorf("same-host redirect: %q, %v", target, keep)
	}
	target, keep := followSameHostRedirects(ctx, first.URL+"/cross", headers)
	if keep || target != otherURL+"/file" {
		t.Errorf("cross-host redirect: %q, %v", target, keep)
	}
	if otherCookie != "" {
		t.Errorf("the other host received %q", otherCookie)
	}
	if _, keep := followSameHostRedirects(ctx, first.URL+"/loop", headers); keep {
		t.Error("a redirect loop kept its cookies")
	}
	other.Close()
	if _, keep := followSameHostRedirects(ctx, other.URL+"/file", headers); keep {
		t.Error("an unreachable host kept its cookies")
	}
}

func TestCreateDownloadDropsCookiesWhenTheScopeSaysSo(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()
	old := resolveCookieScope
	t.Cleanup(func() { resolveCookieScope = old })
	resolveCookieScope = func(context.Context, string, []string) (string, bool) {
		return "https://cdn.test/signed", false
	}

	body := `{"url":"https://site.test/get","filename":"a.bin","cookies":"SESSION=secret","referrer":"https://site.test/"}`
	if rec := request(srv.Routes(), http.MethodPost, "/api/downloads", "", body); rec.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", rec.Code, rec.Body.String())
	}
	if len(mockRPC.addedURIs) != 1 || mockRPC.addedURIs[0] != "https://cdn.test/signed" {
		t.Fatalf("URIs = %v", mockRPC.addedURIs)
	}
	for _, h := range mockRPC.addedOpts.Headers {
		if strings.HasPrefix(h, "Cookie:") {
			t.Fatalf("cookie reached the engine: %v", mockRPC.addedOpts.Headers)
		}
	}
}

func TestOpeningAProgramNeedsConfirmation(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()
	file := filepath.Join(t.TempDir(), "setup.EXE")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mockRPC.stoppedItems = []aria.DownloadStatus{{GID: "g", Status: "complete", Files: []aria.FileInfo{{Path: file}}}}

	var opened, marked string
	oldOpen, oldMark := openFile, markFromInternet
	t.Cleanup(func() { openFile, markFromInternet = oldOpen, oldMark })
	openFile = func(path string) error { opened = path; return nil }
	markFromInternet = func(path string) error { marked = path; return nil }

	rec := postAction(t, srv, "g", `{"action":"open_file"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "confirm_launch") {
		t.Fatalf("unconfirmed: got %d: %s", rec.Code, rec.Body.String())
	}
	if opened != "" {
		t.Fatal("opened without confirmation")
	}
	if rec := postAction(t, srv, "g", `{"action":"open_file","confirmed":true}`); rec.Code != http.StatusOK {
		t.Fatalf("confirmed: got %d: %s", rec.Code, rec.Body.String())
	}
	if opened != file || marked != file {
		t.Fatalf("opened %q, marked %q", opened, marked)
	}
}

func TestCompletedFilesAreMarkedOnce(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	var calls int
	old := markFromInternet
	t.Cleanup(func() { markFromInternet = old })
	markFromInternet = func(string) error { calls++; return nil }

	item := aria.DownloadStatus{GID: "g", Status: "complete", Files: []aria.FileInfo{{Path: "a"}}}
	srv.markCompleted(item)
	srv.markCompleted(item)
	srv.markCompleted(aria.DownloadStatus{GID: "h", Status: "active", Files: []aria.FileInfo{{Path: "b"}}})
	if calls != 1 {
		t.Fatalf("marked %d times, want 1", calls)
	}
}
