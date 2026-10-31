package server

import (
	"context"
	"net/http"

	"conduit/daemon/internal/updater"
)

// SetUpdater registers the update service behind /api/update. A nil service
// makes the update routes answer 503.
func (s *Server) SetUpdater(u *updater.Service) {
	s.mu.Lock()
	s.updates = u
	s.mu.Unlock()
}

// updateSvc returns the registered update service, or nil.
func (s *Server) updateSvc() *updater.Service {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.updates
}

// updateService returns the registered service, or writes 503 and returns nil.
func (s *Server) updateService(w http.ResponseWriter) *updater.Service {
	u := s.updateSvc()
	if u == nil {
		writeError(w, http.StatusServiceUnavailable, "the update service did not start")
	}
	return u
}

func (s *Server) handleGetUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	u := s.updateService(w)
	if u == nil {
		return
	}
	writeJSON(w, http.StatusOK, u.Status())
}

func (s *Server) handleCheckUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	u := s.updateService(w)
	if u == nil {
		return
	}
	// A client that disconnects must not abandon the check half way and leave
	// the state stuck. The check has its own timeout.
	writeJSON(w, http.StatusOK, u.Check(context.WithoutCancel(r.Context())))
}

func (s *Server) handleDownloadUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	u := s.updateService(w)
	if u == nil {
		return
	}
	if err := u.BeginDownload(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	// The transfer continues in the background; the client follows it with GET /api/update.
	writeJSON(w, http.StatusAccepted, u.Status())
}

// handleInstallUpdate queues the installer and quits. The 202 is flushed before
// the quit starts, so the client always gets its answer. The installer itself
// starts after the process has released its files and lock.
func (s *Server) handleInstallUpdate(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	u := s.updateService(w)
	if u == nil {
		return
	}
	if err := u.Install(r.Context()); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, u.Status())
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
	s.quitNow()
}
