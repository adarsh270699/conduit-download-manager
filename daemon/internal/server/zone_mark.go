package server

import (
	"context"
	"log"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/platform"
)

// markFromInternet is replaced in tests.
var markFromInternet = platform.MarkFromInternet

const zoneMarkInterval = 3 * time.Second

// markCompleted tags finished files the way a browser does, so Windows applies
// SmartScreen and its other checks for downloaded files. Each download is marked
// once, so a user who unblocks a file does not see the mark come back.
func (s *Server) markCompleted(item aria.DownloadStatus) {
	if item.Status != "complete" {
		return
	}
	if _, done := s.zoneMarked.LoadOrStore(item.GID, true); done {
		return
	}
	for _, file := range item.Files {
		if file.Path == "" {
			continue
		}
		if err := markFromInternet(file.Path); err != nil {
			log.Printf("Warning: could not mark a download as coming from the internet: %v", err)
		}
	}
}

// runZoneMarker marks finished downloads whether or not a dashboard is open.
func (s *Server) runZoneMarker(ctx context.Context) {
	if !platform.MarksDownloads {
		return
	}
	ticker := time.NewTicker(zoneMarkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		if s.rpcClient == nil {
			continue
		}
		s.mu.Lock()
		statusFn := s.engineStatus
		s.mu.Unlock()
		if statusFn != nil && statusFn() != "ready" {
			continue
		}
		stopped, err := listAllDownloads(ctx, s.rpcClient.TellStopped)
		if err != nil {
			continue
		}
		for _, item := range stopped {
			s.markCompleted(item)
		}
	}
}
