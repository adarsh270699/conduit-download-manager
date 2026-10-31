package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"runtime"
	"testing"

	"conduit/daemon/internal/release"
)

// newHostsService builds a service whose extra download hosts are env.
func newHostsService(t *testing.T, env string) *Service {
	t.Helper()
	svc, err := New(Options{
		ConfigDir:      t.TempDir(),
		CurrentVersion: "1.0.0",
		Channel:        func() Channel { return ChannelNone },
		DownloadHosts:  env,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func parseTestURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

// Without the override, downloads are GitHub-only and HTTPS-only.
func TestDownloadHostsDefaultToGitHubOnly(t *testing.T) {
	svc := newHostsService(t, "")
	cases := map[string]bool{
		"https://github.com/o/r/releases/download/v1/x.exe": true,
		"https://objects.githubusercontent.com/x":           true,
		"https://example.com/x":                             false,
		"https://127.0.0.1/x":                               false,
		"http://github.com/x":                               false,
		"http://127.0.0.1/x":                                false,
	}
	for raw, want := range cases {
		if got := svc.allowedURL(parseTestURL(t, raw)); got != want {
			t.Errorf("%s: allowed=%v, want %v", raw, got, want)
		}
	}
}

// The override adds the named hosts. Plain HTTP is allowed only for loopback
// hosts that are named, so the override cannot send a download over plain HTTP
// to a remote host.
func TestDownloadHostsOverrideAddsOnlyNamedHosts(t *testing.T) {
	svc := newHostsService(t, " 127.0.0.1 , Feed.Test ")
	cases := map[string]bool{
		"https://127.0.0.1:8443/x": true,
		"http://127.0.0.1:8080/x":  true,
		"https://feed.test/x":      true,
		"http://feed.test/x":       false,
		"https://example.com/x":    false,
		"https://github.com/x":     true,
	}
	for raw, want := range cases {
		if got := svc.allowedURL(parseTestURL(t, raw)); got != want {
			t.Errorf("%s: allowed=%v, want %v", raw, got, want)
		}
	}
}

// The end-to-end check the design asks for: a dev build, a feed on loopback,
// and an installer fetched over plain HTTP from the named loopback host.
func TestDownloadFromNamedLoopbackHostOverPlainHTTP(t *testing.T) {
	v, err := release.Parse("1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	name, err := installerName(runtime.GOOS, v)
	if err != nil {
		t.Skipf("no installer on %s: %v", runtime.GOOS, err)
	}
	body := []byte("installer bytes")
	mux := http.NewServeMux()
	mux.HandleFunc("/installer", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(body)
	})
	mux.HandleFunc("/SHA256SUMS.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "%s  %s\n", sha256Hex(body), name)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	svc := newHostsService(t, "127.0.0.1")
	rel := &githubRelease{
		TagName: "v1.3.0",
		Assets: []githubAsset{
			{Name: name, Size: int64(len(body)), BrowserDownloadURL: srv.URL + "/installer"},
			{Name: sumsName, Size: 1, BrowserDownloadURL: srv.URL + "/SHA256SUMS.txt"},
		},
	}
	if err := svc.download(context.Background(), rel, v); err != nil {
		t.Fatalf("download from a named loopback host: %v", err)
	}
	if st := svc.Status(); st.State != StateDownloaded {
		t.Fatalf("state %q, want downloaded (error %q)", st.State, st.Error)
	}
}
