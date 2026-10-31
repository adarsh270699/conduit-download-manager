//go:build integration

package aria

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestSupervisorStopsLeftoverManagedEngine(t *testing.T) {
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "config")
	downloadDir := filepath.Join(tmpDir, "downloads")
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatalf("create download dir: %v", err)
	}

	sup, err := NewSupervisor(SupervisorConfig{
		ConfigDir:   configDir,
		DownloadDir: downloadDir,
		RPCPort:     getFreePort(t),
	})
	if err != nil {
		t.Fatalf("create supervisor: %v", err)
	}

	// The leftover runs from the managed location, which is where the sweep looks.
	// Clear the override so the supervisor also picks that copy.
	engineBin := requireEngineBinary(t)
	t.Setenv(EngineEnvVar, "")
	binPath := filepath.Join(configDir, "bin", ExecutableName())
	copyFile(t, engineBin, binPath)

	// Simulate an engine left behind by an older Conduit version: it runs from
	// our managed path but has no --stop-with-process and is not our child.
	oldPort := getFreePort(t)
	oldSecret := "old-engine-secret"
	oldCmd := exec.Command(
		binPath,
		"--enable-rpc=true",
		"--rpc-listen-all=false",
		fmt.Sprintf("--rpc-listen-port=%d", oldPort),
		fmt.Sprintf("--rpc-secret=%s", oldSecret),
		fmt.Sprintf("--dir=%s", downloadDir),
		fmt.Sprintf("--save-session=%s", filepath.Join(configDir, "old.session")),
		"--file-allocation=none",
	)
	ConfigureProcess(oldCmd)
	if err := oldCmd.Start(); err != nil {
		t.Fatalf("start leftover engine: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() { waitCh <- oldCmd.Wait() }()
	defer func() {
		_ = oldCmd.Process.Kill()
		select {
		case <-waitCh:
		case <-time.After(2 * time.Second):
		}
	}()

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer waitCancel()
	oldClient := NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", oldPort), oldSecret)
	for {
		if _, err := oldClient.GetVersion(waitCtx); err == nil {
			break
		}
		if waitCtx.Err() != nil {
			t.Fatal("leftover engine did not become ready")
		}
		time.Sleep(100 * time.Millisecond)
	}

	created, exe, err := processIdentity(oldCmd.Process.Pid)
	if err != nil {
		t.Fatalf("read leftover identity: %v", err)
	}
	if err := writeEngineState(sup.statePath(), engineState{
		Version:      engineStateVersion,
		PID:          oldCmd.Process.Pid,
		CreationTime: created,
		BinaryPath:   exe,
		RPCPort:      oldPort,
		RPCSecret:    oldSecret,
		StartedAt:    time.Now().UTC(),
	}); err != nil {
		t.Fatalf("write leftover state: %v", err)
	}

	startCtx, startCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer startCancel()
	if err := sup.Start(startCtx); err != nil {
		t.Fatalf("start supervisor with leftover engine: %v", err)
	}
	defer func() { _ = sup.Stop() }()

	select {
	case <-waitCh:
	case <-time.After(5 * time.Second):
		t.Fatal("leftover engine was not stopped")
	}
	if processAlive(oldCmd.Process.Pid) {
		t.Fatal("leftover engine is still alive after supervisor startup")
	}
}
