package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/config"
	"conduit/daemon/internal/platform"
	"conduit/daemon/internal/updater"
	"conduit/daemon/version"
)

const DefaultServerPort = 18800

// ServerVersion is reported by /api/health.
var ServerVersion = version.Version

// AriaClient defines the RPC operations required by the REST server.
type AriaClient interface {
	AddURI(ctx context.Context, uris []string, opts *aria.DownloadOptions) (string, error)
	TellActive(ctx context.Context, keys ...string) ([]aria.DownloadStatus, error)
	TellWaiting(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error)
	TellStopped(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error)
	TellStatus(ctx context.Context, gid string, keys ...string) (*aria.DownloadStatus, error)
	GetOption(ctx context.Context, gid string) (map[string]string, error)
	Pause(ctx context.Context, gid string) error
	Unpause(ctx context.Context, gid string) error
	Remove(ctx context.Context, gid string) error
	ForceRemove(ctx context.Context, gid string) error
	RemoveDownloadResult(ctx context.Context, gid string) error
	ChangeGlobalOption(ctx context.Context, opts map[string]string) error
	ChangePosition(ctx context.Context, gid string, pos int, how string) (int, error)
	GetVersion(ctx context.Context) (*aria.VersionInfo, error)
	Shutdown(ctx context.Context) error
}

// Server provides the REST API for Conduit daemon.
type Server struct {
	port           int
	rpcClient      AriaClient
	configMgr      *config.Manager
	httpServer     *http.Server
	listener       net.Listener
	wsHub          *WSHub
	mu             sync.Mutex
	htmlErrorCache sync.Map
	showWindow     func()
	quitApp        func()
	openURL        func(string) error
	engineStatus   func() string
	engineHint     func() string
	updates        *updater.Service
	stopSchedule   context.CancelFunc
	seenMu         sync.Mutex
	seenStatus     map[string]string // last status per gid, for window pop-ups
}

// NewServer creates a new Server instance.
func NewServer(port int, rpcClient AriaClient, configMgr *config.Manager) *Server {
	if port <= 0 {
		port = DefaultServerPort
	}
	s := &Server{
		port:      port,
		rpcClient: rpcClient,
		configMgr: configMgr,
	}
	s.wsHub = NewWSHub(s)
	return s
}

// Routes constructs the HTTP mux and applies middleware.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("POST /api/app/show", s.handleShow)
	mux.HandleFunc("POST /api/app/quit", s.handleQuit)
	mux.HandleFunc("GET /api/extension", s.handleExtensionInfo)
	mux.HandleFunc("POST /api/extension/open-guide", s.handleOpenExtensionGuide)
	mux.HandleFunc("GET /api/ws", s.handleWebSocket)
	mux.HandleFunc("POST /api/downloads", s.handleCreateDownload)
	mux.HandleFunc("GET /api/downloads", s.handleListDownloads)
	mux.HandleFunc("POST /api/downloads/{gid}/action", s.handleDownloadAction)
	mux.HandleFunc("GET /api/config", s.handleGetConfig)
	mux.HandleFunc("PUT /api/config", s.handleUpdateConfig)
	mux.HandleFunc("GET /api/update", s.handleGetUpdate)
	mux.HandleFunc("POST /api/update/check", s.handleCheckUpdate)
	mux.HandleFunc("POST /api/update/download", s.handleDownloadUpdate)
	mux.HandleFunc("POST /api/update/install", s.handleInstallUpdate)

	// Mount embedded static dashboard
	webHandler, err := getWebHandler()
	if err == nil {
		mux.Handle("GET /", webHandler)
	}

	return CORSMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	engine := "offline"
	s.mu.Lock()
	statusFn := s.engineStatus
	hintFn := s.engineHint
	s.mu.Unlock()
	if statusFn != nil {
		if reported := statusFn(); reported != "" {
			engine = reported
		}
	}
	body := map[string]string{
		"app":     "conduit",
		"status":  "ok",
		"version": ServerVersion,
		"engine":  engine,
	}
	if engine == "missing" && hintFn != nil {
		body["engineHint"] = hintFn()
	}
	writeJSON(w, http.StatusOK, body)
}

// SetEngineStatus registers the download-engine state reported by /api/health.
func (s *Server) SetEngineStatus(fn func() string) {
	s.mu.Lock()
	s.engineStatus = fn
	s.mu.Unlock()
}

// SetEngineHint registers the text /api/health shows when the engine is missing.
func (s *Server) SetEngineHint(fn func() string) {
	s.mu.Lock()
	s.engineHint = fn
	s.mu.Unlock()
}

// SetShowHandler registers the callback for POST /api/app/show.
func (s *Server) SetShowHandler(fn func()) {
	s.mu.Lock()
	s.showWindow = fn
	s.mu.Unlock()
}

// SetQuitHandler registers the callback for POST /api/app/quit.
func (s *Server) SetQuitHandler(fn func()) {
	s.mu.Lock()
	s.quitApp = fn
	s.mu.Unlock()
}

// SetURLOpener replaces the browser opener used by the extension guide.
// Tests use this. Production leaves it nil and uses platform.OpenURL.
func (s *Server) SetURLOpener(fn func(string) error) {
	s.mu.Lock()
	s.openURL = fn
	s.mu.Unlock()
}

func (s *Server) handleShow(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	s.mu.Lock()
	fn := s.showWindow
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleQuit(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	s.mu.Lock()
	fn := s.quitApp
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExtensionInfo(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	path, exists := platform.FindExtensionDir(platform.ExtensionCandidates())
	writeJSON(w, http.StatusOK, map[string]any{
		"path":   path,
		"exists": exists,
	})
}

func (s *Server) handleOpenExtensionGuide(w http.ResponseWriter, r *http.Request) {
	if !requireLoopback(w, r) {
		return
	}
	guide := fmt.Sprintf("http://127.0.0.1:%d/extension", s.port)
	s.mu.Lock()
	opener := s.openURL
	s.mu.Unlock()
	var err error
	if opener != nil {
		err = opener(guide)
	} else {
		err = platform.OpenURL(guide)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open the extension guide")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func requireLoopback(w http.ResponseWriter, r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		writeError(w, http.StatusForbidden, "localhost only")
		return false
	}
	return true
}

// Start begins listening on 127.0.0.1:<port>.
func (s *Server) Start() error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	go s.wsHub.Run(context.Background())

	scheduleCtx, cancelSchedule := context.WithCancel(context.Background())
	go s.runUpdateSchedule(scheduleCtx)

	s.mu.Lock()
	s.stopSchedule = cancelSchedule
	s.listener = ln
	s.httpServer = &http.Server{
		Handler:      s.Routes(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}
	s.mu.Unlock()

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("HTTP server stopped unexpectedly: %v", err)
		}
	}()

	return nil
}

// Shutdown stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.wsHub.Stop()

	// Do not hold s.mu while Shutdown waits for active handlers: those
	// handlers take the same lock, which would deadlock until ctx expires.
	s.mu.Lock()
	httpServer := s.httpServer
	if s.stopSchedule != nil {
		s.stopSchedule()
	}
	s.mu.Unlock()
	if httpServer != nil {
		return httpServer.Shutdown(ctx)
	}
	return nil
}

// Port returns the bound listening port.
func (s *Server) Port() int {
	return s.port
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{
		"error": message,
	})
}
