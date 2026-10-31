package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testUA = "Conduit/1.0.0"

func TestFetchLatestUsesLatestEndpointAndSendsHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/adarsh270699/conduit-download-manager/releases/latest" {
			t.Errorf("wrong path %q: the stable channel must use releases/latest", r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header %q", got)
		}
		if got := r.Header.Get("User-Agent"); got != testUA {
			t.Errorf("User-Agent %q", got)
		}
		w.Header().Set("ETag", `"abc"`)
		_, _ = w.Write([]byte(`{"tag_name":"v1.3.0","assets":[{"name":"Conduit-windows-amd64-setup.exe","size":42,"browser_download_url":"https://github.com/x"}]}`))
	}))
	defer srv.Close()

	got, err := fetchLatest(context.Background(), srv.Client(), srv.URL, testUA, "")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got.notModified || got.release == nil {
		t.Fatalf("expected a release, got %+v", got)
	}
	if got.release.TagName != "v1.3.0" || got.etag != `"abc"` {
		t.Fatalf("unexpected response: %+v", got)
	}
	if len(got.release.Assets) != 1 || got.release.Assets[0].Size != 42 {
		t.Fatalf("assets not read: %+v", got.release.Assets)
	}
}

func TestFetchLatestNotModifiedWhenETagMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != `"abc"` {
			t.Errorf("If-None-Match %q", r.Header.Get("If-None-Match"))
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	got, err := fetchLatest(context.Background(), srv.Client(), srv.URL, testUA, `"abc"`)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !got.notModified {
		t.Fatalf("a 304 must mean not modified: %+v", got)
	}
}

func TestFetchLatestServerErrorIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, err := fetchLatest(context.Background(), srv.Client(), srv.URL, testUA, ""); err == nil {
		t.Fatal("a 500 must be an error")
	}
}

func TestFetchLatestMalformedJSONIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":`))
	}))
	defer srv.Close()

	if _, err := fetchLatest(context.Background(), srv.Client(), srv.URL, testUA, ""); err == nil {
		t.Fatal("malformed JSON must be an error")
	}
}

func TestFetchLatestHonorsContextTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := fetchLatest(ctx, srv.Client(), srv.URL, testUA, ""); err == nil {
		t.Fatal("a cancelled context must end the request with an error")
	}
}

func TestParseTag(t *testing.T) {
	cases := []struct {
		tag     string
		want    string
		wantErr bool
	}{
		{tag: "v1.3.0", want: "1.3.0"},
		{tag: "1.3.0", want: "1.3.0"},
		{tag: "v1.3.0-rc.1", want: "1.3.0-rc.1"},
		{tag: "vnext", wantErr: true},
		{tag: "", wantErr: true},
	}
	for _, c := range cases {
		v, err := parseTag(c.tag)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseTag(%q): want error, got %s", c.tag, v)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTag(%q): %v", c.tag, err)
			continue
		}
		if v.String() != c.want {
			t.Errorf("parseTag(%q) = %s, want %s", c.tag, v, c.want)
		}
	}
}

// The release build reads no environment: a configured base wins, and an unset
// base is GitHub, even when CONDUIT_UPDATE_API is set.
func TestResolveAPIBaseIgnoresEnvironment(t *testing.T) {
	t.Setenv("CONDUIT_UPDATE_API", "http://env.example")
	if got := resolveAPIBase("http://configured.example"); got != "http://configured.example" {
		t.Errorf("configured base should win, got %s", got)
	}
	if got := resolveAPIBase(""); got != defaultAPIBase {
		t.Errorf("default base should be GitHub, got %s", got)
	}
}

func TestFindAssetMatchesExactName(t *testing.T) {
	assets := []githubAsset{
		{Name: "Conduit-windows-amd64-standalone.zip"},
		{Name: "Conduit-windows-amd64-setup.exe", Size: 7},
	}
	if a, ok := findAsset(assets, "Conduit-windows-amd64-setup.exe"); !ok || a.Size != 7 {
		t.Fatalf("exact match not found: %+v %v", a, ok)
	}
	if _, ok := findAsset(assets, "Conduit-windows-amd64-setup"); ok {
		t.Fatal("a partial name must not match")
	}
}
