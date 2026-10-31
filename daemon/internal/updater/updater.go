package updater

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/release"
	"conduit/daemon/version"
)

// checkTimeout bounds one GET /releases/latest so a stalled GitHub cannot pin
// the state at checking. It sits under the server's 15-second write timeout.
const checkTimeout = 10 * time.Second

// Downloads reports the engine's queue. The install gate reads it to refuse
// while a download could be interrupted. aria.Client satisfies it.
type Downloads interface {
	TellActive(ctx context.Context, keys ...string) ([]aria.DownloadStatus, error)
	TellWaiting(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error)
}

// Options configures a Service. Zero values select the production behavior.
type Options struct {
	// ConfigDir holds update.json and the updates/ folder.
	ConfigDir string
	// APIBase overrides the GitHub API base. Empty uses api.github.com. Only the
	// development daemon sets it, from CONDUIT_UPDATE_API.
	APIBase string
	// DownloadHosts adds comma-separated hosts that installers may come from,
	// beyond GitHub. Plain HTTP is allowed only to loopback names in the list.
	// Only the development daemon sets it, from CONDUIT_UPDATE_DOWNLOAD_HOSTS.
	DownloadHosts string
	// Client sends the GitHub and download requests. Nil uses a plain client with
	// no timeout; each step is bounded by a context instead.
	Client *http.Client
	// Downloads is the engine queue. Nil means no downloads can block install.
	Downloads Downloads
	// EngineRunning reports whether the engine is up. While it is down nothing
	// can be downloading, so install is not blocked. Nil means always running.
	EngineRunning func() bool
	// Channel reports how this copy was installed. Nil detects it.
	Channel func() Channel
	// CurrentVersion is the running version. Empty uses the build's version.
	CurrentVersion string
}

// Service runs the update pipeline. Only one operation (check, download,
// install) runs at a time; the state field is the lock.
type Service struct {
	mu            sync.Mutex
	configDir     string
	apiBase       string
	client        *http.Client
	downloads     Downloads
	engineRunning func() bool
	channel       Channel
	current       release.Version
	currentText   string
	rec           record
	status        Status
	etag          string
	latest        *githubRelease
	latestV       release.Version
	// hostAllowed decides which hosts downloads may use over HTTPS. Tests replace
	// it so they can serve from a local TLS server.
	hostAllowed func(host string) bool
	// extraHosts are the development-only hosts from Options.DownloadHosts.
	extraHosts map[string]bool
	// pending is the installer command Install queued for launch after quit.
	pending *Command
}

// New builds the service and restores what the last run recorded. A corrupt
// update.json is logged and ignored, so the daemon still starts.
func New(opts Options) (*Service, error) {
	current := opts.CurrentVersion
	if current == "" {
		current = version.Version
	}
	currentV, err := release.Parse(current)
	if err != nil {
		return nil, fmt.Errorf("current version: %w", err)
	}
	rec, err := loadRecord(opts.ConfigDir)
	if err != nil {
		log.Printf("Warning: update state starts empty: %v", err)
	}
	client := opts.Client
	if client == nil {
		client = &http.Client{}
	}
	var channel Channel
	if opts.Channel != nil {
		channel = opts.Channel()
	} else {
		exe, _ := os.Executable()
		channel = detectChannel(runtime.GOOS, exe, osLookups())
	}

	extra := parseDownloadHosts(opts.DownloadHosts)
	if len(extra) > 0 {
		log.Printf("Warning: update downloads may come from more hosts (%q). This is for testing only.",
			opts.DownloadHosts)
	}

	s := &Service{
		configDir:     opts.ConfigDir,
		apiBase:       resolveAPIBase(opts.APIBase),
		client:        client,
		downloads:     opts.Downloads,
		engineRunning: opts.EngineRunning,
		channel:       channel,
		current:       currentV,
		currentText:   current,
		rec:           rec,
		hostAllowed: func(host string) bool {
			return githubHostAllowed(host) || extra[host]
		},
		extraHosts: extra,
		status: Status{
			State:          StateIdle,
			CurrentVersion: current,
			LatestVersion:  rec.LatestVersion,
			LastCheckedAt:  rec.LastCheckedAt,
		},
	}
	// A download saved by an earlier run is still ready to install. Restoring it
	// here means "Install and restart" and automatic install do not wait for the
	// next check.
	if dv, err := release.Parse(rec.DownloadedVersion); err == nil && dv.Compare(currentV) > 0 && s.holdsDownloadLocked(dv) {
		s.status.State = StateDownloaded
	}
	return s, nil
}

// Status returns the current state for GET /api/update.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

// Check asks GitHub for the latest stable release and records the result. If
// another operation is running, it returns that operation's status without
// sending a request.
func (s *Service) Check(ctx context.Context) Status {
	s.mu.Lock()
	if s.busyLocked() {
		st := s.snapshotLocked()
		s.mu.Unlock()
		return st
	}
	s.status.State = StateChecking
	s.status.Error = ""
	etag := ""
	if s.latest != nil {
		etag = s.etag
	}
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()
	resp, err := fetchLatest(ctx, s.client, s.apiBase, s.userAgent(), etag)
	if err != nil {
		return s.fail(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	ver, rel := s.latestV, s.latest
	if !resp.notModified {
		v, err := parseTag(resp.release.TagName)
		if err != nil {
			s.status.State = StateError
			s.status.Error = err.Error()
			return s.snapshotLocked()
		}
		ver, rel = v, resp.release
		s.etag = resp.etag
	}
	if rel == nil {
		s.status.State = StateError
		s.status.Error = "GitHub returned no release to compare"
		return s.snapshotLocked()
	}

	now := time.Now().UTC()
	s.latest, s.latestV = rel, ver
	s.rec.LastCheckedAt = &now
	s.rec.LatestVersion = ver.String()
	s.status.LatestVersion = ver.String()
	s.status.LastCheckedAt = &now
	switch {
	case ver.Compare(s.current) <= 0:
		s.status.State = StateUpToDate
	case s.holdsDownloadLocked(ver):
		s.status.State = StateDownloaded
	default:
		s.status.State = StateAvailable
	}
	s.saveLocked()
	return s.snapshotLocked()
}

// fail moves the service to error and keeps the last good version information.
func (s *Service) fail(err error) Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = StateError
	s.status.Error = err.Error()
	return s.snapshotLocked()
}

// busyLocked reports whether an operation holds the service. Caller holds mu.
func (s *Service) busyLocked() bool {
	switch s.status.State {
	case StateChecking, StateDownloading, StateInstalling:
		return true
	}
	return false
}

// holdsDownloadLocked reports whether the installer for v is already on disk.
// Caller holds mu.
func (s *Service) holdsDownloadLocked(v release.Version) bool {
	if s.rec.DownloadedVersion != v.String() {
		return false
	}
	path, ok := s.recordedInstaller()
	if !ok {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

// snapshotLocked copies the status for a caller. Caller holds mu.
func (s *Service) snapshotLocked() Status {
	st := s.status
	st.CurrentVersion = s.currentText
	st.Installable = s.channel != ChannelNone
	return st
}

// saveLocked persists update.json. A failed save is logged; the in-memory
// state stays correct for this run. Caller holds mu.
func (s *Service) saveLocked() {
	if err := saveRecord(s.configDir, s.rec); err != nil {
		log.Printf("Warning: could not save update state: %v", err)
	}
}

// userAgent identifies Conduit to GitHub, as the API asks.
func (s *Service) userAgent() string {
	return "Conduit/" + s.currentText
}
