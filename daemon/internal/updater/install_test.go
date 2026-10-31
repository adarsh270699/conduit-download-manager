package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/release"
)

// fakeDownloads answers the engine queue from fixed lists and counts calls.
type fakeDownloads struct {
	active  []aria.DownloadStatus
	waiting []aria.DownloadStatus
	err     error
	calls   atomic.Int32
}

func (f *fakeDownloads) TellActive(ctx context.Context, keys ...string) ([]aria.DownloadStatus, error) {
	f.calls.Add(1)
	return f.active, f.err
}

func (f *fakeDownloads) TellWaiting(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error) {
	f.calls.Add(1)
	if f.err != nil {
		return nil, f.err
	}
	if offset >= len(f.waiting) {
		return nil, nil
	}
	end := min(offset+num, len(f.waiting))
	return f.waiting[offset:end], nil
}

func withStatus(status string, n int) []aria.DownloadStatus {
	items := make([]aria.DownloadStatus, n)
	for i := range items {
		items[i] = aria.DownloadStatus{GID: status, Status: status}
	}
	return items
}

func busyService(t *testing.T, f *fakeDownloads, engineUp bool) *Service {
	t.Helper()
	svc, err := New(Options{
		ConfigDir:     t.TempDir(),
		Downloads:     f,
		EngineRunning: func() bool { return engineUp },
		Channel:       func() Channel { return ChannelNone },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

func TestBusyActiveDownloadBlocksInstall(t *testing.T) {
	f := &fakeDownloads{active: withStatus("active", 1)}
	busy, err := busyService(t, f, true).busy(context.Background())
	if err != nil || !busy {
		t.Fatalf("an active download must block: busy=%v err=%v", busy, err)
	}
}

func TestBusyQueuedDownloadBlocksInstall(t *testing.T) {
	f := &fakeDownloads{waiting: withStatus("waiting", 1)}
	busy, err := busyService(t, f, true).busy(context.Background())
	if err != nil || !busy {
		t.Fatalf("a queued download must block: busy=%v err=%v", busy, err)
	}
}

// aria2 restores paused downloads from aria2.session, so they must not block.
func TestBusyPausedDownloadsDoNotBlock(t *testing.T) {
	f := &fakeDownloads{waiting: withStatus("paused", 3)}
	busy, err := busyService(t, f, true).busy(context.Background())
	if err != nil || busy {
		t.Fatalf("paused downloads must not block: busy=%v err=%v", busy, err)
	}
}

func TestBusyEmptyQueueIsNotBusy(t *testing.T) {
	busy, err := busyService(t, &fakeDownloads{}, true).busy(context.Background())
	if err != nil || busy {
		t.Fatalf("an empty queue must not block: busy=%v err=%v", busy, err)
	}
}

// The only queued download sits on the third page of 100, so reading one
// page would miss it.
func TestBusyReadsEveryPageOfWaiting(t *testing.T) {
	waiting := append(withStatus("paused", 249), aria.DownloadStatus{GID: "last", Status: "waiting"})
	busy, err := busyService(t, &fakeDownloads{waiting: waiting}, true).busy(context.Background())
	if err != nil || !busy {
		t.Fatalf("a queued download on a later page must block: busy=%v err=%v", busy, err)
	}
}

// An RPC failure fails closed: install waits rather than risk an interrupt.
func TestBusyRPCErrorFailsClosed(t *testing.T) {
	f := &fakeDownloads{err: errors.New("connection refused")}
	busy, err := busyService(t, f, true).busy(context.Background())
	if err == nil {
		t.Fatal("the RPC error must be reported")
	}
	if !busy {
		t.Fatal("an RPC failure must report busy")
	}
}

// With the engine down nothing can be downloading, so the gate must not ask it.
func TestBusyWithEngineDownIsNotBusy(t *testing.T) {
	f := &fakeDownloads{active: withStatus("active", 1)}
	busy, err := busyService(t, f, false).busy(context.Background())
	if err != nil || busy {
		t.Fatalf("engine down means not busy: busy=%v err=%v", busy, err)
	}
	if f.calls.Load() != 0 {
		t.Fatalf("the engine queue was asked while the engine was down (%d calls)", f.calls.Load())
	}
}

// supportedChannel returns the channel that installs on this host, or skips.
func supportedChannel(t *testing.T) Channel {
	t.Helper()
	switch runtime.GOOS {
	case "windows":
		return ChannelWindowsInstaller
	case "linux":
		return ChannelDeb
	}
	t.Skipf("no installer channel on %s", runtime.GOOS)
	return ChannelNone
}

// installService builds a service on a supported channel with the given queue.
func installService(t *testing.T, ch Channel, f *fakeDownloads) *Service {
	t.Helper()
	svc, err := New(Options{
		ConfigDir:     t.TempDir(),
		Downloads:     f,
		EngineRunning: func() bool { return true },
		Channel:       func() Channel { return ch },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
}

// stageInstaller writes an installer and records it as downloaded.
func stageInstaller(t *testing.T, svc *Service, body []byte) string {
	t.Helper()
	v, _ := release.Parse("1.3.0")
	path, err := svc.installerPath(v)
	if err != nil {
		t.Skipf("no installer on %s: %v", runtime.GOOS, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	svc.mu.Lock()
	svc.rec.DownloadedVersion = "1.3.0"
	svc.rec.DownloadedPath = path
	svc.rec.DownloadedSHA256 = sha256Hex(body)
	svc.status.State = StateDownloaded
	svc.mu.Unlock()
	return path
}

func TestInstallRequiresDownloadedState(t *testing.T) {
	svc := installService(t, supportedChannel(t), &fakeDownloads{})
	svc.mu.Lock()
	svc.status.State = StateAvailable
	svc.mu.Unlock()

	if err := svc.Install(context.Background()); !errors.Is(err, ErrNotDownloaded) {
		t.Fatalf("install from available: %v, want ErrNotDownloaded", err)
	}
}

func TestInstallRefusesCheckOnlyBuild(t *testing.T) {
	svc := installService(t, ChannelNone, &fakeDownloads{})
	stageInstaller(t, svc, []byte("installer"))

	err := svc.Install(context.Background())
	if !errors.Is(err, ErrUnsupportedChannel) {
		t.Fatalf("install on a check-only build: %v, want ErrUnsupportedChannel", err)
	}
	if !strings.Contains(err.Error(), "not installed by a supported installer") {
		t.Fatalf("the message must say why: %q", err.Error())
	}
	if svc.Status().State != StateDownloaded {
		t.Fatalf("a refused install must leave the download usable, state %q", svc.Status().State)
	}
}

func TestInstallRefusesChangedInstaller(t *testing.T) {
	svc := installService(t, supportedChannel(t), &fakeDownloads{})
	path := stageInstaller(t, svc, []byte("installer"))
	if err := os.WriteFile(path, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := svc.Install(context.Background()); !errors.Is(err, ErrChecksum) {
		t.Fatalf("install of a changed file: %v, want ErrChecksum", err)
	}
	if svc.Status().State != StateError {
		t.Fatalf("a changed installer must set error, state %q", svc.Status().State)
	}
}

func TestInstallRefusesWhileDownloadsActiveAndRestoresState(t *testing.T) {
	f := &fakeDownloads{active: withStatus("active", 1)}
	svc := installService(t, supportedChannel(t), f)
	stageInstaller(t, svc, []byte("installer"))

	if err := svc.Install(context.Background()); !errors.Is(err, ErrDownloadsActive) {
		t.Fatalf("install during a download: %v, want ErrDownloadsActive", err)
	}
	if svc.Status().State != StateDownloaded {
		t.Fatalf("a busy refusal must restore downloaded, state %q", svc.Status().State)
	}
	if svc.TakePendingInstall() != nil {
		t.Fatal("a refused install must queue nothing")
	}
}

func TestInstallQueuesInstallerAndTakeReturnsItOnce(t *testing.T) {
	svc := installService(t, supportedChannel(t), &fakeDownloads{})
	path := stageInstaller(t, svc, []byte("installer"))

	if err := svc.Install(context.Background()); err != nil {
		t.Fatalf("install: %v", err)
	}
	if svc.Status().State != StateInstalling {
		t.Fatalf("state %q, want installing", svc.Status().State)
	}

	cmd := svc.TakePendingInstall()
	if cmd == nil {
		t.Fatal("the installer must be queued")
	}
	if !strings.Contains(strings.Join(append([]string{cmd.Name}, cmd.Args...), " "), path) {
		t.Fatalf("command %+v does not run %s", cmd, path)
	}
	if svc.TakePendingInstall() != nil {
		t.Fatal("TakePendingInstall must return the command once")
	}
}

func TestInstallCommandPerChannel(t *testing.T) {
	const installer = `C:\Users\u\AppData\Local\Temp\my setup\Conduit-windows-amd64-setup.exe`
	cmd, err := installCommand("windows", ChannelWindowsInstaller, installer, "")
	if err != nil {
		t.Fatalf("windows: %v", err)
	}
	if cmd.Name != installer {
		t.Fatalf("windows runs %q, want the installer", cmd.Name)
	}
	if strings.Join(cmd.Args, " ") != "/S /LAUNCH=background" {
		t.Fatalf("windows args %q, want /S /LAUNCH=background (no /NOARIA2)", cmd.Args)
	}

	deb := "/var/cache/conduit/updates/1.3.0/conduit_1.3.0_amd64.deb"
	const sum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cmd, err = installCommand("linux", ChannelDeb, deb, sum)
	if err != nil {
		t.Fatalf("linux: %v", err)
	}
	if cmd.Name != "/bin/sh" || len(cmd.Args) != 5 || cmd.Args[0] != "-c" || cmd.Args[2] != "sh" ||
		cmd.Args[3] != deb || cmd.Args[4] != sum {
		t.Fatalf("linux command %+v", cmd)
	}
	script := cmd.Args[1]
	if !strings.Contains(script, "pkexec /bin/sh -c") || !strings.Contains(script, "exec /usr/bin/conduit --background") {
		t.Fatalf("linux script must verify, install, and relaunch: %q", script)
	}
	if !strings.Contains(script, "sha256sum -c") || !strings.Contains(script, "apt-get install -y") {
		t.Fatalf("the root step must check the copy before installing it: %q", script)
	}
}

func TestInstallCommandRejectsMismatchedChannel(t *testing.T) {
	cases := []struct {
		goos string
		ch   Channel
	}{
		{"windows", ChannelNone},
		{"windows", ChannelDeb},
		{"linux", ChannelWindowsInstaller},
		{"linux", ChannelNone},
		{"darwin", ChannelDeb},
	}
	for _, c := range cases {
		if _, err := installCommand(c.goos, c.ch, "/tmp/x", ""); !errors.Is(err, ErrUnsupportedChannel) {
			t.Errorf("%s/%q: err %v, want ErrUnsupportedChannel", c.goos, c.ch, err)
		}
	}
}
