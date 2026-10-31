package aria

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const engineStateVersion = 1

// engineState is the persistent ownership record for the aria2 process.
// It lets a later launch identify an engine that survived a crash instead of
// starting a second one or attaching to somebody else's aria2.
type engineState struct {
	Version      int       `json:"version"`
	PID          int       `json:"pid"`
	CreationTime uint64    `json:"creationTime"`
	BootID       string    `json:"bootId,omitempty"`
	BinaryPath   string    `json:"binaryPath"`
	RPCPort      int       `json:"rpcPort"`
	RPCSecret    string    `json:"rpcSecret"`
	StartedAt    time.Time `json:"startedAt"`
}

func (s *Supervisor) statePath() string {
	return filepath.Join(s.cfg.ConfigDir, "engine.json")
}

func readEngineState(path string) (*engineState, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state engineState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse engine state: %w", err)
	}
	return &state, nil
}

func writeEngineState(path string, state engineState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode engine state: %w", err)
	}
	tempPath := path + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("write engine state: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace engine state: %w", err)
	}
	return nil
}

func removeEngineState(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("Warning: failed to remove engine state %s: %v", path, err)
	}
}

func (s *Supervisor) persistState(binPath string, pid int) {
	created, _, err := processIdentity(pid)
	if err != nil {
		log.Printf("Warning: could not read aria2 process identity: %v", err)
	}
	bootID, _ := currentBootID()
	state := engineState{
		Version:      engineStateVersion,
		PID:          pid,
		CreationTime: created,
		BootID:       bootID,
		BinaryPath:   filepath.Clean(binPath),
		RPCPort:      s.cfg.RPCPort,
		RPCSecret:    s.cfg.RPCSecret,
		StartedAt:    time.Now().UTC(),
	}
	if err := writeEngineState(s.statePath(), state); err != nil {
		log.Printf("Warning: failed to persist aria2 ownership state: %v", err)
	}
}

func (s *Supervisor) removeStateForPID(pid int) {
	if pid <= 0 {
		return
	}
	state, err := readEngineState(s.statePath())
	if err != nil {
		removeEngineState(s.statePath())
		return
	}
	if state != nil && state.PID == pid {
		removeEngineState(s.statePath())
	}
}

func stateMatchesProcess(state *engineState) bool {
	if state == nil || state.PID <= 0 || state.BinaryPath == "" {
		return false
	}
	created, exe, err := processIdentity(state.PID)
	if err != nil {
		return false
	}
	if !pathsEqual(exe, state.BinaryPath) {
		return false
	}
	if state.CreationTime != 0 && created != state.CreationTime {
		return false
	}
	if state.BootID != "" {
		bootID, err := currentBootID()
		if err != nil || bootID != state.BootID {
			return false
		}
	}
	return true
}

// stopOwnedEngines stops the process recorded in engine.json, and any process
// running from one of managedPaths. The caller must hold the instance lock,
// which guarantees that no other live Conduit process can own an engine from
// this directory. A binary outside managedPaths (for example the distribution's
// /usr/bin/aria2c) is never swept by name, because another program may be
// running it. Only the recorded process is stopped.
func (s *Supervisor) stopOwnedEngines(managedPaths []string) error {
	statePath := s.statePath()
	state, err := readEngineState(statePath)
	if err != nil {
		log.Printf("Warning: ignoring unreadable engine state: %v", err)
		removeEngineState(statePath)
		state = nil
	}
	if state != nil {
		if stateMatchesProcess(state) {
			stopEngineProcess(state)
		}
		removeEngineState(statePath)
	}

	var pids []int
	for _, binPath := range managedPaths {
		found, err := processesWithExe(binPath)
		if err != nil {
			return fmt.Errorf("enumerate managed engine processes: %w", err)
		}
		pids = append(pids, found...)
		// Older builds extracted the engine through a .tmp file; clear any left behind.
		if err := os.Remove(binPath + ".tmp"); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("Warning: failed to remove stale aria2 temp binary: %v", err)
		}
	}
	var lastErr error
	for _, pid := range pids {
		log.Printf("Stopping leftover Conduit engine process %d", pid)
		if err := terminateProcess(pid); err != nil && processAlive(pid) {
			lastErr = fmt.Errorf("stop leftover aria2 process %d: %w", pid, err)
		}
	}
	for _, pid := range pids {
		if err := waitForProcessExit(pid, 3*time.Second); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

func stopEngineProcess(state *engineState) {
	if state == nil || state.PID <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := NewClient(
		fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", state.RPCPort),
		state.RPCSecret,
	).Shutdown(ctx); err != nil {
		log.Printf("Warning: aria2 RPC shutdown failed for process %d: %v", state.PID, err)
	}
	cancel()

	if err := waitForProcessExit(state.PID, 5*time.Second); err == nil {
		return
	}
	// The PID may have exited and been reused while we waited. Re-check the
	// executable and creation time before sending a terminate signal.
	if !stateMatchesProcess(state) {
		return
	}
	log.Printf("Warning: leftover aria2 process %d did not exit gracefully; terminating it", state.PID)
	if err := terminateProcess(state.PID); err != nil {
		log.Printf("Warning: failed to terminate leftover aria2 process %d: %v", state.PID, err)
	}
	_ = waitForProcessExit(state.PID, 2*time.Second)
}

func waitForProcessExit(pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if !processAlive(pid) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("aria2 process %d did not exit", pid)
		}
		time.Sleep(100 * time.Millisecond)
	}
}
