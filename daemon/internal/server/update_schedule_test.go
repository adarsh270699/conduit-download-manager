package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/updater"
	"github.com/gorilla/websocket"
)

// feedVersion is the release the test feed offers. It is newer than 1.0.0.
const feedVersion = "9.9.9"

// testFeed stands in for GitHub. It counts release requests, and it can be
// made to fail them.
type testFeed struct {
	*httptest.Server
	hits atomic.Int32
	fail atomic.Bool
}

// installerAssetName is the release asset this OS installs, or "" when no
// installer is published for it. Tests that download skip on those systems.
func installerAssetName() string {
	switch runtime.GOOS {
	case "windows":
		return "Conduit-windows-amd64-setup.exe"
	case "linux":
		return "conduit_" + feedVersion + "_amd64.deb"
	}
	return ""
}

// installedChannel is the install channel that can run its own installer.
func installedChannel() updater.Channel {
	switch runtime.GOOS {
	case "windows":
		return updater.ChannelWindowsInstaller
	case "linux":
		return updater.ChannelDeb
	}
	return updater.ChannelNone
}

// newTestFeed serves the latest release with the installer for this OS and its
// SHA256SUMS.txt, so a download can finish end to end.
func newTestFeed(t *testing.T) *testFeed {
	t.Helper()
	feed := &testFeed{}
	payload := []byte("installer for the schedule test\n")
	sum := sha256.Sum256(payload)
	name := installerAssetName()
	sums := fmt.Sprintf("%s  %s\n", hex.EncodeToString(sum[:]), name)

	feed.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		feed.hits.Add(1)
		base := "http://" + r.Host
		switch {
		case strings.HasSuffix(r.URL.Path, "/releases/latest"):
			if feed.fail.Load() {
				http.Error(w, "feed is down", http.StatusInternalServerError)
				return
			}
			assets := []map[string]any{}
			if name != "" {
				assets = append(assets,
					map[string]any{"name": name, "size": len(payload), "browser_download_url": base + "/dl/" + name},
					map[string]any{"name": "SHA256SUMS.txt", "size": len(sums), "browser_download_url": base + "/dl/SHA256SUMS.txt"},
				)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"tag_name": "v" + feedVersion, "assets": assets})
		case name != "" && r.URL.Path == "/dl/"+name:
			_, _ = w.Write(payload)
		case name != "" && r.URL.Path == "/dl/SHA256SUMS.txt":
			_, _ = w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(feed.Close)
	return feed
}

// newScheduleService builds an update service against feed. The downloads
// argument is the engine queue; nil means nothing can block an install.
func newScheduleService(t *testing.T, feed *testFeed, channel updater.Channel, downloads updater.Downloads) *updater.Service {
	t.Helper()
	svc, err := updater.New(updater.Options{
		ConfigDir:      t.TempDir(),
		APIBase:        feed.URL,
		CurrentVersion: "1.0.0",
		Channel:        func() updater.Channel { return channel },
		Downloads:      downloads,
		// Downloads come from the test feed on loopback, which the development override allows.
		DownloadHosts: "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("updater.New: %v", err)
	}
	return svc
}

// activeDownloads is an engine queue with one active transfer.
type activeDownloads struct{}

func (activeDownloads) TellActive(context.Context, ...string) ([]aria.DownloadStatus, error) {
	return []aria.DownloadStatus{{}}, nil
}

func (activeDownloads) TellWaiting(context.Context, int, int, ...string) ([]aria.DownloadStatus, error) {
	return nil, nil
}

// engineDownRPC is an engine whose queue cannot be read, as when aria2 is not running.
type engineDownRPC struct {
	*mockAriaClient
}

func (engineDownRPC) TellActive(context.Context, ...string) ([]aria.DownloadStatus, error) {
	return nil, errors.New("engine is not running")
}

// A dashboard must learn about an update even while the engine is down, so the
// update status cannot depend on the download list.
func TestUpdateStatusIsPushedWhenTheEngineIsDown(t *testing.T) {
	_, mock, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	srv := NewServer(18800, engineDownRPC{mock}, cfgMgr)
	srv.SetUpdater(newScheduleService(t, newTestFeed(t), updater.ChannelNone, nil))
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	header := http.Header{}
	header.Set("Origin", "http://127.0.0.1:18800")
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	if resp, err := http.Post(ts.URL+"/api/update/check", "application/json", nil); err != nil {
		t.Fatalf("POST check: %v", err)
	} else {
		resp.Body.Close()
	}
	srv.wsHub.BroadcastAll(context.Background())

	for {
		msg := readWS(t, conn)
		if msg.Type == "update" && msg.Update != nil && msg.Update.State == updater.StateAvailable {
			return
		}
	}
}

// waitForState polls the service until it reaches want, or fails the test.
func waitForState(t *testing.T, svc *updater.Service, want updater.State) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		st := svc.Status()
		if st.State == want {
			return
		}
		if st.State == updater.StateError || time.Now().After(deadline) {
			t.Fatalf("state %q (error %q), want %q", st.State, st.Error, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// With both switches off, the schedule never contacts GitHub.
func TestScheduleStaysOffWithBothSwitchesOff(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	feed := newTestFeed(t)
	svc := newScheduleService(t, feed, installedChannel(), nil)
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	if quit := srv.updateTick(context.Background(), &next, now); quit {
		t.Fatal("the schedule asked to quit with both switches off")
	}
	if n := feed.hits.Load(); n != 0 {
		t.Fatalf("%d requests to GitHub with automatic updates off", n)
	}
	if st := svc.Status(); st.State != updater.StateIdle {
		t.Fatalf("state %q, want idle", st.State)
	}
}

// Auto-check looks once a day and records the next check.
func TestScheduleChecksOncePerDayWhenAutoCheckIsOn(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	if _, err := cfgMgr.Patch([]byte(`{"autoCheckUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	feed := newTestFeed(t)
	svc := newScheduleService(t, feed, installedChannel(), nil)
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	srv.updateTick(context.Background(), &next, now)
	if st := svc.Status(); st.State != updater.StateAvailable || st.LatestVersion != feedVersion {
		t.Fatalf("after the check: state %q latest %q, want available %s", st.State, st.LatestVersion, feedVersion)
	}
	if want := now.Add(24 * time.Hour); !next.Equal(want) {
		t.Fatalf("next check at %v, want %v", next, want)
	}

	srv.updateTick(context.Background(), &next, now.Add(time.Hour))
	if n := feed.hits.Load(); n != 1 {
		t.Fatalf("%d checks within a day, want 1", n)
	}
}

// A failed automatic check is retried in an hour, not left for a day.
func TestScheduleRetriesAnHourAfterAFailedCheck(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	if _, err := cfgMgr.Patch([]byte(`{"autoCheckUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	feed := newTestFeed(t)
	feed.fail.Store(true)
	svc := newScheduleService(t, feed, installedChannel(), nil)
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	srv.updateTick(context.Background(), &next, now)
	if st := svc.Status(); st.State != updater.StateError {
		t.Fatalf("state %q after a failed check, want error", st.State)
	}
	if want := now.Add(time.Hour); !next.Equal(want) {
		t.Fatalf("retry at %v, want %v", next, want)
	}
}

// Notify only: auto-check finds the release and leaves the download to the user.
func TestAutoCheckAloneDoesNotDownload(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	if _, err := cfgMgr.Patch([]byte(`{"autoCheckUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	svc := newScheduleService(t, newTestFeed(t), installedChannel(), nil)
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	if quit := srv.updateTick(context.Background(), &next, now); quit {
		t.Fatal("the schedule asked to quit without an install")
	}
	if st := svc.Status(); st.State != updater.StateAvailable {
		t.Fatalf("state %q, want available with no download started", st.State)
	}
}

// A copy that is not installed by a supported installer cannot install, so
// automatic install must not download a file it could never run.
func TestAutoInstallNeedsAnInstalledCopy(t *testing.T) {
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	if _, err := cfgMgr.Patch([]byte(`{"autoInstallUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	svc := newScheduleService(t, newTestFeed(t), updater.ChannelNone, nil)
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	if quit := srv.updateTick(context.Background(), &next, now); quit {
		t.Fatal("the schedule asked to quit on a copy that cannot install")
	}
	if st := svc.Status(); st.State != updater.StateAvailable {
		t.Fatalf("state %q, want available; no download should start", st.State)
	}
}

// Auto-install checks, downloads, then installs on the next tick and asks the
// app to quit so the installer can run.
func TestAutoInstallDownloadsThenQuits(t *testing.T) {
	if installerAssetName() == "" {
		t.Skipf("no installer is published for %s", runtime.GOOS)
	}
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	if _, err := cfgMgr.Patch([]byte(`{"autoInstallUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	svc := newScheduleService(t, newTestFeed(t), installedChannel(), nil)
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	srv.updateTick(context.Background(), &next, now)
	waitForState(t, svc, updater.StateDownloaded)

	if quit := srv.updateTick(context.Background(), &next, now.Add(time.Minute)); !quit {
		t.Fatal("the schedule did not ask to quit after the installer was queued")
	}
	if svc.TakePendingInstall() == nil {
		t.Fatal("no installer was queued for launch after quit")
	}
}

// Auto-install waits while the engine has a download running, then installs.
func TestAutoInstallWaitsForActiveDownloads(t *testing.T) {
	if installerAssetName() == "" {
		t.Skipf("no installer is published for %s", runtime.GOOS)
	}
	srv, _, cfgMgr, cleanup := setupTestServer(t)
	defer cleanup()
	if _, err := cfgMgr.Patch([]byte(`{"autoInstallUpdates":true}`)); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	svc := newScheduleService(t, newTestFeed(t), installedChannel(), activeDownloads{})
	srv.SetUpdater(svc)

	now := time.Now()
	next := now
	srv.updateTick(context.Background(), &next, now)
	waitForState(t, svc, updater.StateDownloaded)

	if quit := srv.updateTick(context.Background(), &next, now.Add(time.Minute)); quit {
		t.Fatal("the schedule quit while a download was running")
	}
	if st := svc.Status(); st.State != updater.StateDownloaded {
		t.Fatalf("state %q, want downloaded until the queue is clear", st.State)
	}
}
