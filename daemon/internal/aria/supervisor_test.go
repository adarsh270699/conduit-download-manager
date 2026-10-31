package aria

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestNewSupervisorDefaults(t *testing.T) {
	sup, err := NewSupervisor(SupervisorConfig{})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	if sup.RPCPort() != DefaultRPCPort {
		t.Fatalf("expected port %d, got %d", DefaultRPCPort, sup.RPCPort())
	}

	if sup.RPCSecret() == "" {
		t.Fatal("expected non-empty rpc secret")
	}

	if sup.ConfigDir() == "" {
		t.Fatal("expected non-empty config dir")
	}
}

func TestSupervisorInitSession(t *testing.T) {
	tmpDir := t.TempDir()
	sup, err := NewSupervisor(SupervisorConfig{
		ConfigDir:   tmpDir,
		DownloadDir: filepath.Join(tmpDir, "downloads"),
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	sessionPath, err := sup.InitSession()
	if err != nil {
		t.Fatalf("failed to init session: %v", err)
	}
	if _, err := os.Stat(sessionPath); err != nil {
		t.Fatalf("session file not created at %s: %v", sessionPath, err)
	}
}

func TestSupervisorReportsMissingEngine(t *testing.T) {
	// No override, no installer engine, no copy in the config folder, and
	// nothing on PATH: Conduit has no engine to run.
	t.Setenv(EngineEnvVar, "")
	t.Setenv("PATH", t.TempDir())
	tmpDir := t.TempDir()

	sup, err := NewSupervisor(SupervisorConfig{
		ConfigDir:   tmpDir,
		DownloadDir: filepath.Join(tmpDir, "downloads"),
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	if err := sup.Start(context.Background()); !errors.Is(err, ErrEngineMissing) {
		t.Fatalf("Start() error = %v, want ErrEngineMissing", err)
	}
	if got := sup.EngineState(); got != "missing" {
		t.Fatalf("EngineState() = %q, want missing", got)
	}
	if sup.EngineHint() == "" {
		t.Fatal("expected a hint about where to place aria2c")
	}
	if sup.IsRunning() {
		t.Fatal("no engine should be running")
	}
}

func TestSupervisorLifecycle(t *testing.T) {
	requireEngineBinary(t)
	tmpDir, err := os.MkdirTemp("", "conduit-lifecycle-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	port := getFreePort(t)

	sup, err := NewSupervisor(SupervisorConfig{
		ConfigDir:   tmpDir,
		DownloadDir: filepath.Join(tmpDir, "downloads"),
		RPCPort:     port,
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}

	if !sup.IsRunning() {
		t.Fatal("expected supervisor to be running")
	}

	// Verify port is open
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 1*time.Second)
	if err != nil {
		t.Fatalf("failed to connect to aria2 rpc port %d: %v", port, err)
	}
	_ = conn.Close()

	// Stop supervisor
	if err := sup.Stop(); err != nil {
		t.Fatalf("failed to stop supervisor: %v", err)
	}

	// Give a moment to ensure process has fully exited
	time.Sleep(200 * time.Millisecond)

	if sup.IsRunning() {
		t.Fatal("expected supervisor to have stopped")
	}
}

func TestFileAllocationMethod(t *testing.T) {
	method := getFileAllocationMethod()
	// Must never be 'falloc' or 'prealloc' to avoid EOPNOTSUPP crashes on FUSE/NTFS
	// and disk-freezing zeros on multi-gigabyte downloads.
	if method != "trunc" && method != "none" {
		t.Fatalf("unexpected file allocation method '%s'; expected 'trunc' or 'none'", method)
	}
}

func TestSupervisorBuildArgs(t *testing.T) {
	sup, err := NewSupervisor(SupervisorConfig{})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}
	args := sup.buildArgs("/tmp/aria2.session")
	foundContentDisp := false
	foundStopWithProcess := false
	for _, arg := range args {
		if arg == "--content-disposition-default-utf8=true" {
			foundContentDisp = true
		}
		if len(arg) > len("--stop-with-process=") && arg[:len("--stop-with-process=")] == "--stop-with-process=" {
			foundStopWithProcess = true
		}
	}
	if !foundContentDisp {
		t.Fatal("expected --content-disposition-default-utf8=true in aria2 arguments")
	}
	if !foundStopWithProcess {
		t.Fatal("expected --stop-with-process in aria2 arguments")
	}
}
