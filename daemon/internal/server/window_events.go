package server

// windowEventsWanted reports whether any download event can bring the window
// forward, so the list is only polled for them when someone could see it.
func (s *Server) windowEventsWanted() bool {
	if s.configMgr == nil || s.windowShower() == nil {
		return false
	}
	cfg := s.configMgr.Get()
	return cfg.ShowOnStart || cfg.ShowOnComplete || cfg.ShowOnError
}

func (s *Server) windowShower() func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.showWindow
}

// forgetObserved drops the baseline, so turning the pop-ups on later starts
// fresh instead of reporting what finished while they were off.
func (s *Server) forgetObserved() {
	s.seenMu.Lock()
	s.seenStatus = nil
	s.seenMu.Unlock()
}

// DownloadEvent describes one download that has just been added, finished, or
// failed. The dashboard turns it into a toast and highlights the row.
type DownloadEvent struct {
	Event        string `json:"event"` // start, complete, or error
	GID          string `json:"gid"`
	Filename     string `json:"filename"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// observeDownloads compares the list with the previous one and brings the
// window forward when a download has just been added, finished, or failed and
// that event is enabled. The first list after startup only records a baseline,
// so history never pops the window. Several events in one tick show it once.
// It returns the events that fired so the dashboard can say what happened.
func (s *Server) observeDownloads(items []DownloadItem) []DownloadEvent {
	s.seenMu.Lock()
	prev := s.seenStatus
	next := make(map[string]string, len(items))
	for _, item := range items {
		next[item.GID] = item.Status
	}
	s.seenStatus = next
	s.seenMu.Unlock()

	show := s.windowShower()
	if prev == nil || show == nil || s.configMgr == nil {
		return nil
	}
	cfg := s.configMgr.Get()
	var events []DownloadEvent
	for _, item := range items {
		before, known := prev[item.GID]
		if known && before == item.Status {
			continue
		}
		event := ""
		switch {
		case item.Status == "error" && cfg.ShowOnError:
			event = "error"
		case item.Status == "complete" && cfg.ShowOnComplete:
			event = "complete"
		case !known && cfg.ShowOnStart && item.Status != "error" && item.Status != "complete":
			event = "start"
		}
		if event != "" {
			events = append(events, DownloadEvent{Event: event, GID: item.GID, Filename: item.Filename, ErrorMessage: item.ErrorMessage})
		}
	}
	if len(events) > 0 {
		show()
	}
	return events
}
