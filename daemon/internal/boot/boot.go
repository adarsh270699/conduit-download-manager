package boot

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/config"
	"conduit/daemon/internal/instance"
	"conduit/daemon/internal/server"
)

// ErrAlreadyRunning is returned when another Conduit process owns the
// per-config-directory instance lock.
var ErrAlreadyRunning = instance.ErrAlreadyRunning

// Options selects ports and directories for one daemon process.
type Options struct {
	HTTPPort    int
	RPCPort     int
	DownloadDir string
	ConfigDir   string
}

// How often the engine watcher checks aria2, and the longest it waits between
// failed restarts. Variables so tests can shorten them.
var (
	engineCheckInterval = 2 * time.Second
	engineMaxBackoff    = time.Minute
)

// Running is a started daemon. EngineErr is set when aria2 did not start.
// The HTTP server is still up in that case so the window can report the engine,
// and the engine watcher keeps trying to bring aria2 up.
type Running struct {
	Config     *config.Manager
	Supervisor *aria.Supervisor
	Server     *server.Server
	RPC        *aria.Client
	EngineErr  error
	Lock       *instance.Lock

	stopWatch chan struct{}
	watchDone chan struct{}
	stopOnce  sync.Once
}

// Start loads config, starts aria2 when it can, and listens on the HTTP port.
func Start(ctx context.Context, opts Options) (*Running, error) {
	if opts.HTTPPort <= 0 {
		opts.HTTPPort = server.DefaultServerPort
	}
	if opts.RPCPort <= 0 {
		opts.RPCPort = aria.DefaultRPCPort
	}

	configDir, err := resolveConfigDir(opts.ConfigDir)
	if err != nil {
		return nil, err
	}

	// Acquire the lock before touching config or starting aria2. This closes
	// the launch race where two processes both observe "not running" and each
	// start their own engine.
	lock, err := instance.Acquire(configDir)
	if err != nil {
		return nil, err
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			_ = lock.Close()
		}
	}()

	removeStaleConfigTemps(configDir)

	autostartMgr, err := config.NewAppAutostart("")
	if err != nil {
		log.Printf("Warning: failed to initialize autostart manager: %v", err)
	}

	cfgMgr, err := config.NewManager(filepath.Join(configDir, "config.json"), autostartMgr)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize configuration manager: %w", err)
	}

	activeCfg := cfgMgr.Get()
	if opts.DownloadDir != "" {
		activeCfg.DownloadDir = opts.DownloadDir
		if updated, updateErr := cfgMgr.Update(activeCfg); updateErr == nil {
			activeCfg = updated
		} else {
			log.Printf("Warning: failed to persist the --dir setting: %v", updateErr)
		}
	}
	log.Printf("Configuration loaded [downloadDir: %s, maxConnections: %d, autostart: %v]",
		activeCfg.DownloadDir, activeCfg.MaxConnections, activeCfg.AutoStart)

	sup, err := aria.NewSupervisor(aria.SupervisorConfig{
		RPCPort:       opts.RPCPort,
		DownloadDir:   activeCfg.DownloadDir,
		ConfigDir:     configDir,
		MaxConnection: activeCfg.MaxConnections,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize aria2 supervisor: %w", err)
	}

	running := &Running{
		Config:     cfgMgr,
		Supervisor: sup,
		Lock:       lock,
		// The port and secret are fixed for the life of the supervisor, so one
		// client keeps working across engine restarts.
		RPC: aria.NewClient(
			fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", sup.RPCPort()),
			sup.RPCSecret(),
		),
		stopWatch: make(chan struct{}),
		watchDone: make(chan struct{}),
	}

	if err := running.startEngine(ctx); err != nil {
		running.EngineErr = err
		log.Printf("Warning: aria2 did not start: %v", err)
	} else if ver, err := running.RPC.GetVersion(ctx); err == nil {
		log.Printf("aria2 RPC online: version %s (features: %s)", ver.Version, strings.Join(ver.EnabledFeatures, ", "))
	}

	srv := server.NewServer(opts.HTTPPort, running.RPC, cfgMgr)
	srv.SetEngineStatus(sup.EngineState)
	srv.SetEngineHint(sup.EngineHint)
	if err := srv.Start(); err != nil {
		_ = sup.Stop()
		return nil, fmt.Errorf("failed to start HTTP server: %w", err)
	}
	running.Server = srv
	go running.watchEngine()
	lockHeld = false
	log.Printf("Conduit REST API online at http://127.0.0.1:%d/api/health", srv.Port())
	return running, nil
}

// startEngine launches aria2 and applies the saved settings to it.
func (r *Running) startEngine(ctx context.Context) error {
	startCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	// Read the setting on every start, so a changed enginePath applies on the next restart.
	r.Supervisor.SetEnginePath(r.Config.Get().EnginePath)
	if err := r.Supervisor.Start(startCtx); err != nil {
		return err
	}
	log.Printf("aria2 process active [binary: %s, rpc-port: %d]", r.Supervisor.BinaryPath(), r.Supervisor.RPCPort())
	if err := r.RPC.ChangeGlobalOption(ctx, r.Config.Get().EngineOptions()); err != nil {
		log.Printf("Warning: failed to apply settings to aria2: %v", err)
	}
	return nil
}

// watchEngine restarts aria2 when it exits or never started. aria2 saves its
// session every 30 seconds, so a restart picks queued and paused work back up.
// While aria2c is missing it keeps checking, so a binary placed by hand is
// picked up without restarting Conduit.
func (r *Running) watchEngine() {
	defer close(r.watchDone)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		<-r.stopWatch
		cancel()
	}()

	wait := engineCheckInterval
	missingLogged := false
	for {
		select {
		case <-r.stopWatch:
			return
		case <-time.After(wait):
		}
		if r.Supervisor.IsRunning() {
			wait = engineCheckInterval
			continue
		}
		if !missingLogged {
			log.Printf("Download engine is not running; restarting it.")
		}
		if err := r.startEngine(ctx); err != nil {
			wait = min(wait*2, engineMaxBackoff)
			if errors.Is(err, aria.ErrEngineMissing) {
				if !missingLogged {
					log.Printf("Download engine not found; Conduit will keep looking. %s", r.Supervisor.EngineHint())
					missingLogged = true
				}
				continue
			}
			log.Printf("Warning: aria2 restart failed: %v", err)
			continue
		}
		missingLogged = false
		wait = engineCheckInterval
	}
}

// Shutdown stops the HTTP server and aria2, then releases the instance lock.
func (r *Running) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	// Stop the watcher first so it cannot restart the engine we are stopping.
	if r.stopWatch != nil {
		r.stopOnce.Do(func() { close(r.stopWatch) })
		<-r.watchDone
	}
	var shutdownErr error
	if r.Server != nil {
		if err := r.Server.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down HTTP server: %v", err)
			shutdownErr = err
		} else {
			log.Printf("HTTP server stopped.")
		}
	}
	if r.RPC != nil {
		_ = r.RPC.Shutdown(ctx)
	}
	if r.Supervisor != nil {
		if err := r.Supervisor.Stop(); err != nil {
			log.Printf("Error during aria2 supervisor shutdown: %v", err)
			if shutdownErr == nil {
				shutdownErr = err
			}
		} else {
			log.Printf("aria2 process exited cleanly.")
		}
	}
	if r.Lock != nil {
		if err := r.Lock.Close(); err != nil {
			log.Printf("Error releasing instance lock: %v", err)
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
		r.Lock = nil
	}
	return shutdownErr
}

// DefaultConfigDir returns the per-user directory that holds config, session,
// engine state, and the instance lock.
func DefaultConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user config directory: %w", err)
	}
	return filepath.Join(base, "conduit"), nil
}

func resolveConfigDir(dir string) (string, error) {
	if dir == "" {
		return DefaultConfigDir()
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return abs, nil
}

func removeStaleConfigTemps(configDir string) {
	for _, name := range []string{"config.json.tmp", "engine.json.tmp"} {
		path := filepath.Join(configDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: failed to remove stale temp file %s: %v", path, err)
		}
	}
}
