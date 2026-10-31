package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"conduit/daemon/internal/release"
)

func TestInstallerNameForEachOS(t *testing.T) {
	v, err := release.Parse("1.3.0")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		goos, want string
		wantErr    bool
	}{
		{goos: "windows", want: "Conduit-windows-amd64-setup.exe"},
		{goos: "linux", want: "conduit_1.3.0_amd64.deb"},
		{goos: "darwin", wantErr: true},
	}
	for _, c := range cases {
		got, err := installerName(c.goos, v)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: want error, got %q", c.goos, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("%s: got %q, %v; want %q", c.goos, got, err, c.want)
		}
	}
}

func TestChecksumForReadsSha256sumLines(t *testing.T) {
	const good = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	listing := "" +
		good + "  Conduit-windows-amd64-setup.exe\n" +
		"AAAA" + "  other.zip\n" +
		good[:10] + "  short.exe\n" +
		"ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff *star.exe\r\n"

	if got, ok := checksumFor(listing, "Conduit-windows-amd64-setup.exe"); !ok || got != good {
		t.Errorf("two-space line: got %q, %v", got, ok)
	}
	ones := strings.Repeat("f", 64)
	if got, ok := checksumFor(listing, "star.exe"); !ok || got != ones {
		t.Errorf("star-prefixed line with CRLF: got %q, %v", got, ok)
	}
	if _, ok := checksumFor(listing, "missing.exe"); ok {
		t.Error("a name not in the listing must not match")
	}
	if _, ok := checksumFor(listing, "short.exe"); ok {
		t.Error("a hash that is not 64 hex digits must not match")
	}
	if _, ok := checksumFor(listing, "other.zip"); ok {
		t.Error("a non-hex hash must not match")
	}
}

// assetServer serves one installer and its SHA256SUMS.txt over HTTPS, the way
// github.com serves release assets.
type assetServer struct {
	srv      *httptest.Server
	body     []byte
	sums     string
	requests atomic.Int32
	gate     chan struct{} // when set, the installer waits until it is closed
	redirect string        // when set, the installer answers with a redirect here
}

func newAssetServer(t *testing.T, name string, body []byte, sumHex string) *assetServer {
	t.Helper()
	a := &assetServer{body: body, sums: fmt.Sprintf("%s  %s\n", sumHex, name)}
	mux := http.NewServeMux()
	mux.HandleFunc("/installer", func(w http.ResponseWriter, r *http.Request) {
		a.requests.Add(1)
		if a.gate != nil {
			<-a.gate
		}
		if a.redirect != "" {
			http.Redirect(w, r, a.redirect, http.StatusFound)
			return
		}
		_, _ = w.Write(a.body)
	})
	mux.HandleFunc("/SHA256SUMS.txt", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(a.sums))
	})
	a.srv = httptest.NewTLSServer(mux)
	t.Cleanup(a.srv.Close)
	return a
}

func sha256Hex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// downloadService builds a service that trusts the test server and treats the
// test server's host as the one GitHub host.
func downloadService(t *testing.T, a *assetServer) *Service {
	t.Helper()
	svc, err := New(Options{
		ConfigDir:      t.TempDir(),
		CurrentVersion: "1.0.0",
		Client:         a.srv.Client(),
		Channel:        func() Channel { return ChannelNone },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	svc.hostAllowed = func(host string) bool { return host == "127.0.0.1" }
	return svc
}

// offerRelease puts the service in the state a successful check leaves it in.
func offerRelease(t *testing.T, svc *Service, a *assetServer, ver string, size int64) (*githubRelease, string) {
	t.Helper()
	v, err := release.Parse(ver)
	if err != nil {
		t.Fatal(err)
	}
	name, err := installerName(runtime.GOOS, v)
	if err != nil {
		t.Skipf("no installer on %s: %v", runtime.GOOS, err)
	}
	rel := &githubRelease{
		TagName: "v" + ver,
		Assets: []githubAsset{
			{Name: name, Size: size, BrowserDownloadURL: a.srv.URL + "/installer"},
			{Name: "SHA256SUMS.txt", Size: 1, BrowserDownloadURL: a.srv.URL + "/SHA256SUMS.txt"},
		},
	}
	svc.mu.Lock()
	svc.latest, svc.latestV = rel, v
	svc.status.State = StateAvailable
	svc.status.LatestVersion = ver
	svc.mu.Unlock()
	return rel, name
}

func TestDownloadStoresVerifiedInstaller(t *testing.T) {
	body := []byte("installer bytes")
	a := newAssetServer(t, "", body, "")
	svc := downloadService(t, a)
	rel, name := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex(body), name)

	v, _ := release.Parse("1.3.0")
	if err := svc.download(context.Background(), rel, v); err != nil {
		t.Fatalf("download: %v", err)
	}

	st := svc.Status()
	if st.State != StateDownloaded {
		t.Fatalf("state %q, want downloaded (error %q)", st.State, st.Error)
	}
	final := filepath.Join(svc.configDir, updatesDirName, "1.3.0", name)
	got, err := os.ReadFile(final)
	if err != nil {
		t.Fatalf("installer not stored at %s: %v", final, err)
	}
	if string(got) != string(body) {
		t.Fatal("stored installer differs from the served bytes")
	}
	if _, err := os.Stat(final + ".part"); !os.IsNotExist(err) {
		t.Fatalf("a .part file was left behind: %v", err)
	}
	if svc.rec.DownloadedPath != final || svc.rec.DownloadedSHA256 != sha256Hex(body) || svc.rec.DownloadedVersion != "1.3.0" {
		t.Fatalf("record not updated: %+v", svc.rec)
	}
	saved, err := loadRecord(svc.configDir)
	if err != nil || saved.DownloadedPath != final {
		t.Fatalf("update.json not saved: %+v %v", saved, err)
	}
	if st.Progress.Done != int64(len(body)) || st.Progress.Total != int64(len(body)) {
		t.Fatalf("progress %+v, want done and total at %d", st.Progress, len(body))
	}
}

func TestDownloadRefusesPlainHTTPWithoutRequest(t *testing.T) {
	body := []byte("x")
	a := newAssetServer(t, "", body, sha256Hex(body))
	svc := downloadService(t, a)
	rel, _ := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	rel.Assets[0].BrowserDownloadURL = "http://127.0.0.1/installer"

	v, _ := release.Parse("1.3.0")
	if err := svc.download(context.Background(), rel, v); err == nil {
		t.Fatal("a plain http URL must be refused")
	}
	if a.requests.Load() != 0 {
		t.Fatalf("the refused URL was requested %d times", a.requests.Load())
	}
}

func TestDownloadRefusesForeignRedirect(t *testing.T) {
	body := []byte("x")
	a := newAssetServer(t, "", body, sha256Hex(body))
	// The redirect target counts requests. The check must refuse it before any
	// connection is made, so the count stays at zero.
	var hits atomic.Int32
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(body)
	}))
	t.Cleanup(foreign.Close)
	a.redirect = foreign.URL + "/evil.exe"
	svc := downloadService(t, a)
	rel, name := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex(body), name)

	v, _ := release.Parse("1.3.0")
	err := svc.download(context.Background(), rel, v)
	if err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Fatalf("a redirect off the allow-list must be refused, got %v", err)
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("the foreign host was contacted %d times", n)
	}
	dir := filepath.Join(svc.configDir, updatesDirName, "1.3.0")
	if _, err := os.Stat(filepath.Join(dir, name+".part")); !os.IsNotExist(err) {
		t.Fatalf("a refused download left a .part file: %v", err)
	}
	if svc.Status().State != StateError {
		t.Fatalf("state %q, want error", svc.Status().State)
	}
}

func TestDownloadRefusesSizeMismatch(t *testing.T) {
	body := []byte("installer bytes")
	a := newAssetServer(t, "", body, sha256Hex(body))
	svc := downloadService(t, a)
	rel, name := offerRelease(t, svc, a, "1.3.0", int64(len(body))+5)
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex(body), name)

	v, _ := release.Parse("1.3.0")
	if err := svc.download(context.Background(), rel, v); err == nil {
		t.Fatal("a body that differs from the API size must be refused")
	}
	dir := filepath.Join(svc.configDir, updatesDirName, "1.3.0")
	for _, p := range []string{name, name + ".part"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Fatalf("%s survived a size mismatch: %v", p, err)
		}
	}
}

func TestDownloadRefusesOversizedAssetWithoutRequest(t *testing.T) {
	body := []byte("x")
	a := newAssetServer(t, "", body, sha256Hex(body))
	svc := downloadService(t, a)
	rel, _ := offerRelease(t, svc, a, "1.3.0", maxInstallerBytes+1)

	v, _ := release.Parse("1.3.0")
	if err := svc.download(context.Background(), rel, v); err == nil {
		t.Fatal("an asset over 1 GiB must be refused")
	}
	if a.requests.Load() != 0 {
		t.Fatalf("an oversized asset was requested %d times", a.requests.Load())
	}
}

func TestDownloadHashMismatchDeletesFile(t *testing.T) {
	body := []byte("installer bytes")
	a := newAssetServer(t, "", body, sha256Hex([]byte("something else")))
	svc := downloadService(t, a)
	rel, name := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex([]byte("something else")), name)

	v, _ := release.Parse("1.3.0")
	if err := svc.download(context.Background(), rel, v); err == nil {
		t.Fatal("a hash that does not match SHA256SUMS.txt must be an error")
	}
	dir := filepath.Join(svc.configDir, updatesDirName, "1.3.0")
	for _, p := range []string{name, name + ".part"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Fatalf("%s survived a hash mismatch: %v", p, err)
		}
	}
	if svc.Status().State != StateError {
		t.Fatalf("state %q, want error", svc.Status().State)
	}
}

func TestBeginDownloadRequiresAvailable(t *testing.T) {
	body := []byte("x")
	a := newAssetServer(t, "", body, sha256Hex(body))
	svc := downloadService(t, a)

	if err := svc.BeginDownload(); err == nil {
		t.Fatal("an idle service has nothing to download")
	}
}

func TestBeginDownloadSecondCallIsBusy(t *testing.T) {
	body := []byte("installer bytes")
	a := newAssetServer(t, "", body, sha256Hex(body))
	a.gate = make(chan struct{})
	var once sync.Once
	openGate := func() { once.Do(func() { close(a.gate) }) }
	t.Cleanup(openGate)
	svc := downloadService(t, a)
	_, name := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex(body), name)

	if err := svc.BeginDownload(); err != nil {
		t.Fatalf("first download: %v", err)
	}
	if err := svc.BeginDownload(); err == nil {
		t.Fatal("a second download during one must be refused")
	}
	if st := svc.Status(); st.State != StateDownloading {
		t.Fatalf("state %q, want downloading", st.State)
	}
	openGate()
	waitForState(t, svc, StateDownloaded)
}

func TestBeginDownloadWorksOnCheckOnlyBuild(t *testing.T) {
	body := []byte("installer bytes")
	a := newAssetServer(t, "", body, sha256Hex(body))
	svc := downloadService(t, a)
	_, name := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex(body), name)

	if err := svc.BeginDownload(); err != nil {
		t.Fatalf("begin: %v", err)
	}
	waitForState(t, svc, StateDownloaded)
}

func TestFailedBackgroundDownloadKeepsLatestVersion(t *testing.T) {
	body := []byte("installer bytes")
	a := newAssetServer(t, "", body, sha256Hex([]byte("other")))
	svc := downloadService(t, a)
	_, name := offerRelease(t, svc, a, "1.3.0", int64(len(body)))
	a.sums = fmt.Sprintf("%s  %s\n", sha256Hex([]byte("other")), name)

	if err := svc.BeginDownload(); err != nil {
		t.Fatalf("begin: %v", err)
	}
	waitForState(t, svc, StateError)
	if got := svc.Status().LatestVersion; got != "1.3.0" {
		t.Fatalf("error must keep the latest version, got %q", got)
	}
}

// waitForState polls until the background download reaches want, so the test
// never removes its temp directory while the goroutine still writes to it.
func waitForState(t *testing.T, svc *Service, want State) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if svc.Status().State == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state %q, want %q", svc.Status().State, want)
}
