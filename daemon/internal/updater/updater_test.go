package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// feed serves one GitHub release response and counts the requests it gets.
type feed struct {
	srv      *httptest.Server
	requests atomic.Int32
	status   atomic.Int32
	body     atomic.Value // string
	etag     atomic.Value // string
}

func newFeed(t *testing.T) *feed {
	t.Helper()
	f := &feed{}
	f.status.Store(http.StatusOK)
	f.body.Store(`{"tag_name":"v9.9.9","assets":[]}`)
	f.etag.Store(`"v1"`)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		if r.Header.Get("If-None-Match") == f.etag.Load().(string) && f.status.Load() == http.StatusOK {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", f.etag.Load().(string))
		w.WriteHeader(int(f.status.Load()))
		_, _ = w.Write([]byte(f.body.Load().(string)))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newTestService(t *testing.T, apiBase, current string) *Service {
	t.Helper()
	svc, err := New(Options{
		ConfigDir:      t.TempDir(),
		APIBase:        apiBase,
		CurrentVersion: current,
		Channel:        func() Channel { return ChannelNone },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func TestCheckNewerReleaseIsAvailable(t *testing.T) {
	f := newFeed(t)
	svc := newTestService(t, f.srv.URL, "1.0.0")

	st := svc.Check(context.Background())
	if st.State != StateAvailable {
		t.Fatalf("state %q, want available (error %q)", st.State, st.Error)
	}
	if st.LatestVersion != "9.9.9" || st.CurrentVersion != "1.0.0" {
		t.Fatalf("versions wrong: %+v", st)
	}
	if st.LastCheckedAt == nil {
		t.Fatal("a completed check must record lastCheckedAt")
	}

	rec, err := loadRecord(svc.configDir)
	if err != nil {
		t.Fatalf("load record: %v", err)
	}
	if rec.LatestVersion != "9.9.9" || rec.LastCheckedAt == nil {
		t.Fatalf("update.json not written: %+v", rec)
	}
}

func TestCheckSameOrOlderIsUpToDate(t *testing.T) {
	for _, tc := range []struct{ name, tag, current string }{
		{"same", "v1.0.0", "1.0.0"},
		{"older", "v0.9.0", "1.0.0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFeed(t)
			f.body.Store(`{"tag_name":"` + tc.tag + `","assets":[]}`)
			svc := newTestService(t, f.srv.URL, tc.current)

			st := svc.Check(context.Background())
			if st.State != StateUpToDate {
				t.Fatalf("state %q, want upToDate (error %q)", st.State, st.Error)
			}
		})
	}
}

func TestCheckBadTagIsErrorAndKeepsLastGoodVersion(t *testing.T) {
	f := newFeed(t)
	svc := newTestService(t, f.srv.URL, "1.0.0")
	if st := svc.Check(context.Background()); st.State != StateAvailable {
		t.Fatalf("setup check: %q", st.State)
	}

	f.body.Store(`{"tag_name":"nightly","assets":[]}`)
	f.etag.Store(`"v2"`)
	st := svc.Check(context.Background())
	if st.State != StateError || st.Error == "" {
		t.Fatalf("bad tag must set error, got %+v", st)
	}
	if st.LatestVersion != "9.9.9" {
		t.Fatalf("error must keep the last good version, got %q", st.LatestVersion)
	}
}

func TestCheckServerErrorKeepsLastGoodVersion(t *testing.T) {
	f := newFeed(t)
	svc := newTestService(t, f.srv.URL, "1.0.0")
	if st := svc.Check(context.Background()); st.State != StateAvailable {
		t.Fatalf("setup check: %q", st.State)
	}

	f.status.Store(http.StatusInternalServerError)
	f.etag.Store(`"v2"`)
	st := svc.Check(context.Background())
	if st.State != StateError {
		t.Fatalf("a 500 must set error, got %q", st.State)
	}
	if st.LatestVersion != "9.9.9" {
		t.Fatalf("error must keep the last good version, got %q", st.LatestVersion)
	}
}

func TestCheckReusesCachedReleaseOnNotModified(t *testing.T) {
	f := newFeed(t)
	svc := newTestService(t, f.srv.URL, "1.0.0")
	first := svc.Check(context.Background())

	second := svc.Check(context.Background())
	if f.requests.Load() != 2 {
		t.Fatalf("want 2 requests, got %d", f.requests.Load())
	}
	if second.State != StateAvailable || second.LatestVersion != "9.9.9" {
		t.Fatalf("304 must keep the cached result, got %+v", second)
	}
	if second.LastCheckedAt == nil || second.LastCheckedAt.Before(*first.LastCheckedAt) {
		t.Fatal("a 304 still counts as a check and must move lastCheckedAt forward")
	}
}

func TestCheckWhileCheckingReturnsStatusWithoutRequest(t *testing.T) {
	entered := make(chan struct{})
	unblock := make(chan struct{})
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			close(entered)
			<-unblock
		}
		_, _ = w.Write([]byte(`{"tag_name":"v9.9.9","assets":[]}`))
	}))
	defer srv.Close()
	svc := newTestService(t, srv.URL, "1.0.0")

	done := make(chan Status)
	go func() { done <- svc.Check(context.Background()) }()
	<-entered

	during := svc.Check(context.Background())
	if during.State != StateChecking {
		t.Fatalf("a second check must report checking, got %q", during.State)
	}
	close(unblock)
	if final := <-done; final.State != StateAvailable {
		t.Fatalf("first check should finish available, got %q", final.State)
	}
	if requests.Load() != 1 {
		t.Fatalf("the second check must not send a request; got %d", requests.Load())
	}
}

func TestLastCheckSurvivesRestart(t *testing.T) {
	f := newFeed(t)
	dir := t.TempDir()
	first, err := New(Options{ConfigDir: dir, APIBase: f.srv.URL, CurrentVersion: "1.0.0", Channel: func() Channel { return ChannelNone }})
	if err != nil {
		t.Fatal(err)
	}
	first.Check(context.Background())

	second, err := New(Options{ConfigDir: dir, APIBase: f.srv.URL, CurrentVersion: "1.0.0", Channel: func() Channel { return ChannelNone }})
	if err != nil {
		t.Fatal(err)
	}
	st := second.Status()
	if st.State != StateIdle {
		t.Fatalf("a restart starts idle, got %q", st.State)
	}
	if st.LatestVersion != "9.9.9" || st.LastCheckedAt == nil {
		t.Fatalf("restart must restore what the last check found: %+v", st)
	}
}

func TestNewDefaultsCurrentVersionToBuild(t *testing.T) {
	svc, err := New(Options{ConfigDir: filepath.Join(t.TempDir(), "conduit"), Channel: func() Channel { return ChannelNone }})
	if err != nil {
		t.Fatal(err)
	}
	if svc.Status().CurrentVersion == "" {
		t.Fatal("current version must come from the build")
	}
}
