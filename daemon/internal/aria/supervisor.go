package aria

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const (
	DefaultRPCPort      = 18801
	DefaultMaxConn      = 16
	DefaultSplit        = 16
	DefaultMinSplitSize = "1M"
)

// SupervisorConfig holds configuration parameters for the aria2 supervisor.
type SupervisorConfig struct {
	ConfigDir     string
	DownloadDir   string
	RPCPort       int
	RPCSecret     string
	MaxConnection int
	Split         int
	MinSplitSize  string
}

// Supervisor manages the aria2 lifecycle, engine discovery, and process monitoring.
type Supervisor struct {
	cfg        SupervisorConfig
	binaryPath string
	enginePath string
	cmd        *exec.Cmd
	jobClose   func()
	mu         sync.RWMutex
	running    bool
	phase      string
	exitErr    error
	stopOnce   sync.Once
}

// NewSupervisor creates a new Supervisor instance with default fallback values.
func NewSupervisor(cfg SupervisorConfig) (*Supervisor, error) {
	if cfg.ConfigDir == "" {
		userConfigDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user config dir: %w", err)
		}
		cfg.ConfigDir = filepath.Join(userConfigDir, "conduit")
	}

	if cfg.DownloadDir == "" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home dir: %w", err)
		}
		cfg.DownloadDir = filepath.Join(userHome, "Downloads")
	}

	if cfg.RPCPort <= 0 {
		cfg.RPCPort = DefaultRPCPort
	}

	if cfg.MaxConnection <= 0 {
		cfg.MaxConnection = DefaultMaxConn
	}

	if cfg.Split <= 0 {
		cfg.Split = DefaultSplit
	}

	if cfg.MinSplitSize == "" {
		cfg.MinSplitSize = DefaultMinSplitSize
	}

	if cfg.RPCSecret == "" {
		secret, err := generateSecret()
		if err != nil {
			return nil, fmt.Errorf("failed to generate rpc secret: %w", err)
		}
		cfg.RPCSecret = secret
	}

	return &Supervisor{
		cfg: cfg,
	}, nil
}

// resolveBinary chooses the aria2c binary to run. Nothing is copied or
// extracted: Conduit does not ship an engine, so the binary comes from the
// installer, the user, or the system. See Locate for the search order.
func (s *Supervisor) resolveBinary() (EngineLocation, error) {
	s.mu.RLock()
	enginePath := s.enginePath
	s.mu.RUnlock()

	loc, err := Locate(s.cfg.ConfigDir, enginePath)
	if err != nil {
		return EngineLocation{}, err
	}

	s.mu.Lock()
	s.binaryPath = loc.Path
	s.mu.Unlock()
	return loc, nil
}

// SetEnginePath sets the enginePath setting that the next Start honors. An
// empty path means the engine is found automatically.
func (s *Supervisor) SetEnginePath(path string) {
	s.mu.Lock()
	s.enginePath = path
	s.mu.Unlock()
}

// EngineHint tells the user how to provide aria2c when the engine is missing.
func (s *Supervisor) EngineHint() string {
	placement := filepath.Join(s.cfg.ConfigDir, "bin", ExecutableName())
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Reinstall Conduit while online, or place aria2c.exe at %s.", placement)
	}
	return fmt.Sprintf("Install aria2 (for example: sudo apt install aria2), or place aria2c at %s.", placement)
}

// InitSession ensures the aria2.session file exists in the config directory.
func (s *Supervisor) InitSession() (string, error) {
	if err := os.MkdirAll(s.cfg.ConfigDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create config dir: %w", err)
	}

	sessionPath := filepath.Join(s.cfg.ConfigDir, "aria2.session")
	if _, err := os.Stat(sessionPath); os.IsNotExist(err) {
		f, err := os.OpenFile(sessionPath, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			return "", fmt.Errorf("failed to initialize session file: %w", err)
		}
		_ = f.Close()
	}
	// Session files contain signed download URLs. Keep them owner-only even
	// when upgrading a session created by an older build.
	_ = os.Chmod(sessionPath, 0o600)

	return sessionPath, nil
}

func (s *Supervisor) setPhase(phase string) {
	s.mu.Lock()
	s.phase = phase
	s.mu.Unlock()
}

// EngineState reports ready, starting, missing, or offline for the download engine.
func (s *Supervisor) EngineState() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// A live process is only ready once its RPC answered; until then
	// /api/health must say "starting" so clients hold their requests.
	switch {
	case s.running && s.phase == "ready":
		return "ready"
	case s.phase == "missing":
		return "missing"
	case s.phase == "" || s.phase == "starting":
		return "starting"
	default:
		return "offline"
	}
}

// Start chooses the engine binary, prepares the session, launches aria2c, and waits for readiness.
func (s *Supervisor) Start(ctx context.Context) error {
	s.setPhase("starting")

	if err := s.stopOwnedEngines(managedEnginePaths(executableDir(), s.cfg.ConfigDir)); err != nil {
		s.setPhase("offline")
		return fmt.Errorf("failed to clear the previous Conduit engine: %w", err)
	}

	loc, err := s.resolveBinary()
	if err != nil {
		if errors.Is(err, ErrEngineMissing) {
			s.setPhase("missing")
		} else {
			s.setPhase("offline")
		}
		return err
	}
	binPath := loc.Path

	sessionPath, err := s.InitSession()
	if err != nil {
		s.setPhase("offline")
		return fmt.Errorf("session initialization failed: %w", err)
	}

	if err := os.MkdirAll(s.cfg.DownloadDir, 0o755); err != nil {
		s.setPhase("offline")
		return fmt.Errorf("failed to create download directory: %w", err)
	}

	args := s.buildArgs(sessionPath)

	cmd := exec.Command(binPath, args...)
	ConfigureProcess(cmd)

	if err := cmd.Start(); err != nil {
		s.setPhase("offline")
		return fmt.Errorf("failed to start aria2c: %w", err)
	}

	jobClose, jobErr := assignProcessToJob(cmd)
	if jobErr != nil {
		// --stop-with-process is still active, so this is a warning, not a
		// reason to refuse to run.
		log.Printf("Warning: failed to bind aria2 to a job object: %v", jobErr)
		jobClose = nil
	}

	s.mu.Lock()
	s.cmd = cmd
	s.jobClose = jobClose
	s.running = true
	s.stopOnce = sync.Once{}
	s.exitErr = nil
	s.mu.Unlock()

	s.persistState(binPath, cmd.Process.Pid)

	// Monitor child process exit in background.
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		// After a forced Stop and a restart, s.cmd is the new process. Only
		// the current process may mark the engine offline.
		if s.cmd == cmd || s.cmd == nil {
			s.running = false
			s.exitErr = err
			s.phase = "offline"
		}
		s.mu.Unlock()
		s.removeStateForPID(cmd.Process.Pid)
	}()

	if err := s.waitForRPC(ctx, s.cfg.RPCPort, s.cfg.RPCSecret, sessionPath, 5*time.Second); err != nil {
		_ = s.Stop()
		s.setPhase("offline")
		return err
	}

	s.setPhase("ready")
	return nil
}

func (s *Supervisor) buildArgs(sessionPath string) []string {
	return []string{
		"--enable-rpc=true",
		"--rpc-listen-all=false",
		fmt.Sprintf("--rpc-listen-port=%d", s.cfg.RPCPort),
		fmt.Sprintf("--rpc-secret=%s", s.cfg.RPCSecret),
		fmt.Sprintf("--stop-with-process=%d", os.Getpid()),
		fmt.Sprintf("--dir=%s", s.cfg.DownloadDir),
		fmt.Sprintf("--input-file=%s", sessionPath),
		fmt.Sprintf("--save-session=%s", sessionPath),
		"--save-session-interval=30",
		fmt.Sprintf("--max-connection-per-server=%d", s.cfg.MaxConnection),
		fmt.Sprintf("--split=%d", s.cfg.Split),
		fmt.Sprintf("--min-split-size=%s", s.cfg.MinSplitSize),
		"--continue=true",
		"--auto-file-renaming=true",
		"--allow-overwrite=false",
		fmt.Sprintf("--file-allocation=%s", getFileAllocationMethod()),
		"--content-disposition-default-utf8=true",
		"--enable-dht=false",
		"--bt-enable-lpd=false",
	}
}

// waitForRPC waits until the listener on port authenticates with our secret.
// A plain TCP dial is not enough: another program (or another aria2 instance)
// can hold the same port and must never be mistaken for our engine.
func (s *Supervisor) waitForRPC(ctx context.Context, port int, secret, sessionPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	client := NewClient(fmt.Sprintf("http://%s/jsonrpc", addr), secret)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()

			callCtx, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
			_, rpcErr := client.GetVersion(callCtx)
			if rpcErr == nil {
				if sessionPath != "" {
					opts, optErr := client.GetGlobalOption(callCtx)
					if optErr == nil {
						if !pathsEqual(opts["save-session"], sessionPath) || !pathsEqual(opts["dir"], s.cfg.DownloadDir) {
							cancel()
							return fmt.Errorf("aria2 RPC port %d is owned by another engine", port)
						}
					} else if isTransientRPCError(optErr) {
						cancel()
						time.Sleep(100 * time.Millisecond)
						continue
					} else {
						cancel()
						return fmt.Errorf("aria2 RPC port %d returned unexpected options: %w", port, optErr)
					}
				}
				cancel()
				return nil
			}
			cancel()

			if isTransientRPCError(rpcErr) {
				// The listener accepted the connection but aria2 may still be
				// initializing. Keep polling until the deadline.
			} else {
				var rpcError *RPCError
				if errors.As(rpcErr, &rpcError) {
					return fmt.Errorf("aria2 RPC port %d is owned by another engine: %w", port, rpcErr)
				}
				return fmt.Errorf("aria2 RPC port %d is in use by another program: %w", port, rpcErr)
			}
		}

		s.mu.RLock()
		running := s.running
		exitErr := s.exitErr
		s.mu.RUnlock()
		if !running {
			return fmt.Errorf("process terminated prematurely: %v", exitErr)
		}

		time.Sleep(100 * time.Millisecond)
	}

	return errors.New("timeout waiting for authenticated aria2 RPC")
}

func isTransientRPCError(err error) bool {
	var rpcError *RPCError
	if errors.As(err, &rpcError) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF) {
		return true
	}
	return false
}

// Stop gracefully shuts down the aria2 child process.
func (s *Supervisor) Stop() error {
	var stopErr error
	s.stopOnce.Do(func() {
		s.mu.Lock()
		cmd := s.cmd
		jobClose := s.jobClose
		running := s.running
		s.mu.Unlock()

		if cmd == nil || cmd.Process == nil {
			return
		}
		pid := cmd.Process.Pid

		if !running {
			if jobClose != nil {
				jobClose()
			}
			s.removeStateForPID(pid)
			return
		}

		// On non-Windows, send interrupt signal to allow session save.
		if runtime.GOOS != "windows" {
			_ = cmd.Process.Signal(os.Interrupt)
		}

		// Wait up to 3 seconds for exit (whether triggered by RPC shutdown or signal)
		done := make(chan struct{})
		go func() {
			for {
				s.mu.RLock()
				r := s.running
				s.mu.RUnlock()
				if !r {
					close(done)
					return
				}
				time.Sleep(50 * time.Millisecond)
			}
		}()

		select {
		case <-done:
			// Process exited cleanly
		case <-time.After(3 * time.Second):
			// Force kill if it didn't exit in time
			if err := cmd.Process.Kill(); err != nil {
				stopErr = err
			}
		}

		// Closing the job handle is the final lifecycle backstop on Windows.
		if jobClose != nil {
			jobClose()
		}

		s.mu.Lock()
		s.running = false
		s.jobClose = nil
		s.cmd = nil
		s.mu.Unlock()
		s.removeStateForPID(pid)
	})

	return stopErr
}

// RPCPort returns the configured RPC port.
func (s *Supervisor) RPCPort() int {
	return s.cfg.RPCPort
}

// RPCSecret returns the configured or generated RPC secret.
func (s *Supervisor) RPCSecret() string {
	return s.cfg.RPCSecret
}

// ConfigDir returns the directory where config and binaries are stored.
func (s *Supervisor) ConfigDir() string {
	return s.cfg.ConfigDir
}

// DownloadDir returns the target download folder.
func (s *Supervisor) DownloadDir() string {
	return s.cfg.DownloadDir
}

// BinaryPath returns the extracted aria2 binary path.
func (s *Supervisor) BinaryPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.binaryPath
}

// IsRunning reports whether aria2c is currently active.
func (s *Supervisor) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func generateSecret() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func getFileAllocationMethod() string {
	// On Linux, use 'trunc' (ftruncate) rather than 'falloc' (fallocate).
	// 'falloc' requires kernel fallocate(2) support, which fails fatally with
	// EOPNOTSUPP (errno 95: Not supported) on mounted Windows NTFS partitions (fuseblk/ntfs-3g),
	// FAT32, exFAT, and network file systems (NFS/CIFS/SMB).
	// 'trunc' extends sparse files instantly across ALL modern filesystems without freezing
	// or failing. On Windows, use 'none' to avoid MSVCRT zero-fill overhead.
	if runtime.GOOS == "windows" {
		return "none"
	}
	return "trunc"
}
