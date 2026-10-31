package server

import (
	"errors"
	"io"
	"log"
	"net/http"

	"conduit/daemon/internal/config"
)

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	if s.configMgr == nil {
		writeError(w, http.StatusInternalServerError, "configuration manager not initialized")
		return
	}
	writeJSON(w, http.StatusOK, s.configMgr.Get())
}

// handleUpdateConfig merges the JSON body into the current settings. Clients
// send only the fields they change, so two open dashboards cannot overwrite
// each other's edits with stale values.
func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if s.configMgr == nil {
		writeError(w, http.StatusInternalServerError, "configuration manager not initialized")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 64<<10))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	updated, err := s.configMgr.Patch(body)
	if errors.Is(err, config.ErrInvalid) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update configuration: "+err.Error())
		return
	}

	if s.rpcClient != nil {
		if err := s.rpcClient.ChangeGlobalOption(r.Context(), updated.EngineOptions()); err != nil {
			// The setting is persisted and will be applied on the next engine
			// start, so this is a warning rather than a failed save.
			log.Printf("Warning: configuration saved but could not be applied to the running engine: %v", err)
		}
	}

	s.wsHub.BroadcastConfig(updated)
	writeJSON(w, http.StatusOK, updated)
}
