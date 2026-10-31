package updater

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"conduit/daemon/internal/release"
)

// A transfer that goes quiet is ended and reported. Without the stall timer, a
// connection that stops sending would hold the state at downloading for good,
// and Check and Install would refuse until Conduit restarts.
func TestStalledDownloadIsEndedAndReported(t *testing.T) {
	old := stallTimeout
	stallTimeout = 200 * time.Millisecond
	t.Cleanup(func() { stallTimeout = old })

	ver := release.Version{Major: 1, Minor: 3, Patch: 0}
	name, err := installerName(runtime.GOOS, ver)
	if err != nil {
		t.Skipf("no installer on %s: %v", runtime.GOOS, err)
	}
	body := []byte(strings.Repeat("x", 60))
	unblock := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, sumsName) {
			_, _ = fmt.Fprintf(w, "%s  %s\n", sha256Hex(body), name)
			return
		}
		w.Header().Set("Content-Length", "60")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body[:10])
		w.(http.Flusher).Flush()
		<-unblock
	}))
	// Cleanups run last-in first-out, so the handler is released before the server closes.
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(unblock) })

	svc := newHostsService(t, "127.0.0.1")
	rel := &githubRelease{TagName: "v1.3.0", Assets: []githubAsset{
		{Name: name, Size: 60, BrowserDownloadURL: srv.URL + "/installer"},
		{Name: sumsName, Size: 1, BrowserDownloadURL: srv.URL + "/" + sumsName},
	}}

	done := make(chan error, 1)
	go func() { done <- svc.download(context.Background(), rel, ver) }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "stalled") {
			t.Fatalf("download ended with %v, want a stall error", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("a stalled download was never ended")
	}

	if st := svc.Status(); st.State != StateError || !strings.Contains(st.Error, "stalled") {
		t.Fatalf("state %q error %q, want an error that says stalled", st.State, st.Error)
	}
	part := filepath.Join(svc.configDir, updatesDirName, ver.String(), name+".part")
	if _, err := os.Stat(part); !os.IsNotExist(err) {
		t.Fatalf("partial installer left behind: %v", err)
	}
}
