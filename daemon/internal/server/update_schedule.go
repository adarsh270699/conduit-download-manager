package server

import (
	"context"
	"errors"
	"log"
	"time"

	"conduit/daemon/internal/updater"
)

const (
	// scheduleTick is how often the schedule reads the settings.
	scheduleTick = time.Minute
	// scheduleStartup delays the first automatic check after start, so a busy
	// start-up is not also a network request.
	scheduleStartup = 30 * time.Second
	// checkEvery is the wait after a successful automatic check.
	checkEvery = 24 * time.Hour
	// retryEvery is the wait after a failed automatic check.
	retryEvery = time.Hour
)

// runUpdateSchedule runs the automatic checks, downloads, and installs until
// ctx ends. It quits the app only to finish an install.
func (s *Server) runUpdateSchedule(ctx context.Context) {
	u := s.updateSvc()
	if u == nil {
		return
	}
	next := time.Now().Add(scheduleStartup)
	if last := u.Status().LastCheckedAt; last != nil && last.Add(checkEvery).After(next) {
		next = last.Add(checkEvery)
	}

	ticker := time.NewTicker(scheduleTick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			if s.updateTick(ctx, &next, now) {
				s.quitNow()
				return
			}
		}
	}
}

// updateTick runs one pass of the schedule at time now. It checks when next is
// due and moves next forward. Then, if auto-install is on and the copy can
// install, it downloads an available release and installs a downloaded one.
// It reports true when an install is queued and the app should quit.
func (s *Server) updateTick(ctx context.Context, next *time.Time, now time.Time) bool {
	u := s.updateSvc()
	if u == nil || s.configMgr == nil {
		return false
	}
	cfg := s.configMgr.Get()
	if !cfg.AutoCheckUpdates && !cfg.AutoInstallUpdates {
		return false
	}

	st := u.Status()
	if !now.Before(*next) {
		st = u.Check(ctx)
		*next = now.Add(checkEvery)
		if st.State == updater.StateError {
			*next = now.Add(retryEvery)
		}
	}
	if !cfg.AutoInstallUpdates || !st.Installable {
		return false
	}

	switch st.State {
	case updater.StateAvailable:
		// BeginDownload refuses while another step runs. The next tick tries again.
		_ = u.BeginDownload()
	case updater.StateDownloaded:
		err := u.Install(ctx)
		if err == nil {
			return true
		}
		// Waiting for downloads is expected and needs no log line each minute.
		if !errors.Is(err, updater.ErrDownloadsActive) {
			log.Printf("Warning: automatic update install did not start: %v", err)
		}
	}
	return false
}

// quitNow asks the app to quit, from its own goroutine so the caller can finish.
func (s *Server) quitNow() {
	s.mu.Lock()
	quit := s.quitApp
	s.mu.Unlock()
	if quit != nil {
		go quit()
	}
}
