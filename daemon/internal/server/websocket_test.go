package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"conduit/daemon/internal/aria"
	"github.com/gorilla/websocket"
)

func TestWebSocketHubLifecycle(t *testing.T) {
	srv, mockRPC, _, cleanup := setupTestServer(t)
	defer cleanup()

	mockRPC.activeItems = []aria.DownloadStatus{
		{
			GID:             "gid-ws-1",
			Status:          "active",
			TotalLength:     5000,
			CompletedLength: 2500,
			DownloadSpeed:   1000,
			Files: []aria.FileInfo{
				{Path: "/downloads/test.pkg"},
			},
		},
	}

	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	// Start hub with accelerated ticker for testing
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv.wsHub.tickDuration = 50 * time.Millisecond
	go srv.wsHub.Run(ctx)
	defer srv.wsHub.Stop()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/ws"

	// 1. Connect allowed origin
	header := http.Header{}
	header.Set("Origin", "http://127.0.0.1:18800")
	dialer := websocket.Dialer{
		HandshakeTimeout: 2 * time.Second,
	}

	conn, resp, err := dialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v (status %v)", err, resp.StatusCode)
	}
	defer conn.Close()

	// A new client receives the settings, then the list.
	if first := readWS(t, conn); first.Type != "config" || first.Config == nil {
		t.Fatalf("first message = %+v, want config", first)
	}
	if items := readWS(t, conn); items.Type != "downloads" || len(items.Downloads) != 1 || items.Downloads[0].GID != "gid-ws-1" {
		t.Fatalf("unexpected initial downloads: %+v", items)
	}

	// 2. Read next tick broadcast
	if tick := readWS(t, conn); tick.Type != "downloads" || len(tick.Downloads) != 1 {
		t.Fatalf("unexpected periodic broadcast: %+v", tick)
	}

	// 3. Reject disallowed origin
	badHeader := http.Header{}
	badHeader.Set("Origin", "https://unauthorized-domain.com")
	badConn, badResp, err := dialer.Dial(wsURL, badHeader)
	if err == nil {
		badConn.Close()
		t.Fatal("expected dial to fail for unauthorized origin, got success")
	}
	if badResp != nil && badResp.StatusCode != http.StatusForbidden {
		t.Logf("dial rejected with status %d", badResp.StatusCode)
	}
}

func readWS(t *testing.T, conn *websocket.Conn) WSMessage {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read ws message: %v", err)
	}
	var msg WSMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("unmarshal ws message %s: %v", data, err)
	}
	return msg
}

// Settings changed in one dashboard (the desktop window) must reach every
// other open dashboard (a browser tab) without a reload.
func TestConfigChangeIsPushedToOtherClients(t *testing.T) {
	srv, _, _, cleanup := setupTestServer(t)
	defer cleanup()
	ts := httptest.NewServer(srv.Routes())
	defer ts.Close()

	header := http.Header{}
	header.Set("Origin", "http://127.0.0.1:18800")
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(ts.URL, "http")+"/api/ws", header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	readWS(t, conn) // config
	readWS(t, conn) // downloads

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", strings.NewReader(`{"maxDownloadLimit":"2M"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/config: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT status = %d", resp.StatusCode)
	}

	for {
		msg := readWS(t, conn)
		if msg.Type != "config" {
			continue
		}
		if msg.Config.MaxDownloadLimit != "2M" {
			t.Fatalf("pushed limit = %q, want 2M", msg.Config.MaxDownloadLimit)
		}
		return
	}
}
