package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"conduit/daemon/internal/config"
	"conduit/daemon/internal/updater"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return isOriginAllowed(r.Header.Get("Origin"))
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// WSMessage is one frame on /api/ws. Type says which field is set:
// "downloads" carries the full list, "config" carries the current settings,
// "event" lists what just made the window pop up, and "update" carries the
// update status.
type WSMessage struct {
	Type      string          `json:"type"`
	Downloads []DownloadItem  `json:"downloads,omitempty"`
	Config    *config.Config  `json:"config,omitempty"`
	Events    []DownloadEvent `json:"events,omitempty"`
	Update    *updater.Status `json:"update,omitempty"`
}

// WSHub pushes the download list every tick and settings whenever they change,
// so every open dashboard (desktop window and browser tabs) stays in step.
type WSHub struct {
	server       *Server
	mu           sync.Mutex // guards clients, lastUpdate, and serialises writes
	clients      map[*websocket.Conn]struct{}
	lastUpdate   []byte // the last update status sent, so unchanged status is not resent
	stopChan     chan struct{}
	stopOnce     sync.Once
	tickDuration time.Duration
}

// NewWSHub creates a new WebSocket hub instance.
func NewWSHub(s *Server) *WSHub {
	return &WSHub{
		server:       s,
		clients:      make(map[*websocket.Conn]struct{}),
		stopChan:     make(chan struct{}),
		tickDuration: time.Second,
	}
}

// Run broadcasts the download list on every tick until ctx ends or Stop is called.
func (h *WSHub) Run(ctx context.Context) {
	ticker := time.NewTicker(h.tickDuration)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			h.closeAll()
			return
		case <-h.stopChan:
			h.closeAll()
			return
		case <-ticker.C:
			h.BroadcastAll(ctx)
		}
	}
}

// Stop terminates the hub event loop.
func (h *WSHub) Stop() {
	h.stopOnce.Do(func() { close(h.stopChan) })
}

// ClientCount returns the number of connected WebSocket clients.
func (h *WSHub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}

// BroadcastAll sends the current download list to every client and feeds it to
// the window pop-up watcher.
func (h *WSHub) BroadcastAll(ctx context.Context) {
	hasClients := h.ClientCount() > 0
	// The update status does not need the engine, so it goes out even when the
	// download list cannot be read.
	if hasClients {
		h.broadcastUpdate()
	}
	if !hasClients && !h.server.windowEventsWanted() {
		h.server.forgetObserved()
		return
	}
	items, err := h.server.getDownloadItems(ctx)
	if err != nil {
		return
	}
	events := h.server.observeDownloads(items)
	if !hasClients {
		return
	}
	h.send(WSMessage{Type: "downloads", Downloads: items})
	// After the list, so the row an event points at is already on screen.
	if len(events) > 0 {
		h.send(WSMessage{Type: "event", Events: events})
	}
}

// broadcastUpdate sends the update status when it has changed since the last
// send. A dashboard that connects later gets the status when it connects.
func (h *WSHub) broadcastUpdate() {
	u := h.server.updateSvc()
	if u == nil {
		return
	}
	st := u.Status()
	data, err := json.Marshal(st)
	if err != nil {
		return
	}
	h.mu.Lock()
	same := bytes.Equal(data, h.lastUpdate)
	h.lastUpdate = data
	h.mu.Unlock()
	if !same {
		h.send(WSMessage{Type: "update", Update: &st})
	}
}

// BroadcastConfig sends changed settings to every client.
func (h *WSHub) BroadcastConfig(cfg config.Config) {
	h.send(WSMessage{Type: "config", Config: &cfg})
}

// send writes msg to the given clients, or to all clients when none are given.
func (h *WSHub) send(msg WSMessage, only ...*websocket.Conn) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	targets := only
	if len(targets) == 0 {
		for conn := range h.clients {
			targets = append(targets, conn)
		}
	}
	for _, conn := range targets {
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			_ = conn.Close()
			delete(h.clients, conn)
		}
	}
}

func (h *WSHub) add(conn *websocket.Conn) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	select {
	case <-h.stopChan:
		return false
	default:
	}
	h.clients[conn] = struct{}{}
	return true
}

func (h *WSHub) remove(conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, conn)
	_ = conn.Close()
}

func (h *WSHub) closeAll() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		_ = conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutting down"),
			time.Now().Add(time.Second),
		)
		_ = conn.Close()
		delete(h.clients, conn)
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	if !s.wsHub.add(conn) {
		_ = conn.Close()
		return
	}

	// A new client gets the settings and the list immediately instead of
	// waiting for the next tick.
	if s.configMgr != nil {
		cfg := s.configMgr.Get()
		s.wsHub.send(WSMessage{Type: "config", Config: &cfg}, conn)
	}
	if items, err := s.getDownloadItems(r.Context()); err == nil {
		s.wsHub.send(WSMessage{Type: "downloads", Downloads: items}, conn)
	}

	if u := s.updateSvc(); u != nil {
		st := u.Status()
		s.wsHub.send(WSMessage{Type: "update", Update: &st}, conn)
	}

	// The read loop only detects disconnects; clients never send data.
	go func() {
		defer s.wsHub.remove(conn)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}
