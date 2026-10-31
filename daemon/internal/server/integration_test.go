//go:build integration

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/config"
	"github.com/gorilla/websocket"
)

func getFreePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to find free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// startLiveREST starts the REST server on a free port and returns it with that
// port. getFreePort only shows the port was free a moment ago, and aria2 or a
// parallel test package can take it before Start binds, so retry on a new port.
func startLiveREST(t *testing.T, rpcClient AriaClient, cfgMgr *config.Manager) (*Server, int) {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		port := getFreePort(t)
		srv := NewServer(port, rpcClient, cfgMgr)
		if err := srv.Start(); err != nil {
			lastErr = err
			continue
		}
		return srv, port
	}
	t.Fatalf("failed to start REST server: %v", lastErr)
	return nil, 0
}

func TestLiveServerRESTIntegration(t *testing.T) {
	needsEngine(t)
	tmpDir, err := os.MkdirTemp("", "conduit-int-srv-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	downloadDir := filepath.Join(tmpDir, "downloads")
	_ = os.MkdirAll(downloadDir, 0755)

	rpcPort := getFreePort(t)

	// 1. Initialize Config Manager
	configPath := filepath.Join(configDir, "config.json")
	cfgMgr, err := config.NewManager(configPath, nil)
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}
	_, _ = cfgMgr.Update(config.Config{
		DownloadDir:            downloadDir,
		MaxConnections:         16,
		MaxConcurrentDownloads: 5,
	})

	// 2. Start Live aria2 Supervisor
	sup, err := aria.NewSupervisor(aria.SupervisorConfig{
		ConfigDir:   configDir,
		DownloadDir: downloadDir,
		RPCPort:     rpcPort,
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer func() {
		_ = sup.Stop()
	}()

	// 3. Start Live REST Server
	rpcClient := aria.NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", rpcPort), sup.RPCSecret())
	srv, httpPort := startLiveREST(t, rpcClient, cfgMgr)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	// 4. Test GET /api/health
	resp, err := http.Get(baseURL + "/api/health")
	if err != nil {
		t.Fatalf("health check failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	// 4b. Test GET / (Embedded dashboard)
	webResp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("failed to fetch embedded dashboard: %v", err)
	}
	if webResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for dashboard root, got %d", webResp.StatusCode)
	}
	var webBuf bytes.Buffer
	_, _ = webBuf.ReadFrom(webResp.Body)
	_ = webResp.Body.Close()
	if !bytes.Contains(webBuf.Bytes(), []byte("Conduit")) {
		t.Fatalf("expected embedded dashboard HTML to contain 'Conduit'")
	}

	// 4c. Connect live WebSocket client
	wsURL := fmt.Sprintf("ws://127.0.0.1:%d/api/ws", httpPort)
	wsHeader := http.Header{}
	wsHeader.Set("Origin", "http://127.0.0.1:18800")
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, wsHeader)
	if err != nil {
		t.Fatalf("live WebSocket connection failed: %v", err)
	}
	defer wsConn.Close()

	// Read initial broadcast
	_ = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, initMsg, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("failed to read initial live ws message: %v", err)
	}
	var initMsgParsed WSMessage
	if err := json.Unmarshal(initMsg, &initMsgParsed); err != nil || initMsgParsed.Type == "" {
		t.Fatalf("failed to parse initial live ws message %s: %v", initMsg, err)
	}

	// 5. Start mock download target server
	targetData := "Live REST server integration test content payload 123456789"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(targetData)))
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(targetData))
	}))
	defer ts.Close()

	// 6. Test POST /api/downloads
	createBody, _ := json.Marshal(CreateDownloadRequest{
		URL:       ts.URL + "/rest_test.txt",
		Filename:  "rest_test.txt",
		Referrer:  ts.URL,
		UserAgent: "ConduitIntegration/1.0",
		Cookies:   "token=abc123xyz",
	})
	postResp, err := http.Post(baseURL+"/api/downloads", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("POST /api/downloads failed: %v", err)
	}
	if postResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", postResp.StatusCode)
	}

	var createResult map[string]string
	_ = json.NewDecoder(postResp.Body).Decode(&createResult)
	_ = postResp.Body.Close()

	gid := createResult["gid"]
	if gid == "" {
		t.Fatal("expected non-empty GID in response")
	}

	// 7. Poll GET /api/downloads until completed
	var completed bool
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		getResp, err := http.Get(baseURL + "/api/downloads")
		if err == nil && getResp.StatusCode == http.StatusOK {
			var list []DownloadItem
			_ = json.NewDecoder(getResp.Body).Decode(&list)
			_ = getResp.Body.Close()

			for _, item := range list {
				if item.GID == gid && item.Status == "complete" {
					completed = true
					break
				}
			}
		}
		if completed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !completed {
		t.Fatalf("download with gid %s did not reach 'complete' status within deadline", gid)
	}

	// 8. Verify file on disk
	downloadedFile := filepath.Join(downloadDir, "rest_test.txt")
	content, err := os.ReadFile(downloadedFile)
	if err != nil {
		t.Fatalf("failed to read downloaded file: %v", err)
	}
	if string(content) != targetData {
		t.Fatalf("content mismatch: expected '%s', got '%s'", targetData, string(content))
	}

	// 9. Test POST /api/downloads/{gid}/action (remove completed download)
	actionBody := bytes.NewReader([]byte(`{"action":"remove"}`))
	actionResp, err := http.Post(fmt.Sprintf("%s/api/downloads/%s/action", baseURL, gid), "application/json", actionBody)
	if err != nil {
		t.Fatalf("action failed: %v", err)
	}
	if actionResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for action, got %d", actionResp.StatusCode)
	}
	_ = actionResp.Body.Close()

	// 10. Test PUT /api/config
	configPayload, _ := json.Marshal(map[string]any{
		"downloadDir":            downloadDir,
		"maxConnections":         8,
		"maxConcurrentDownloads": 3,
	})
	putBody := bytes.NewReader(configPayload)
	req, _ := http.NewRequest(http.MethodPut, baseURL+"/api/config", putBody)
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /api/config failed: %v", err)
	}
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT /api/config, got %d", putResp.StatusCode)
	}
	var updated config.Config
	_ = json.NewDecoder(putResp.Body).Decode(&updated)
	_ = putResp.Body.Close()

	if updated.MaxConnections != 8 || updated.MaxConcurrentDownloads != 3 {
		t.Fatalf("unexpected updated config: %+v", updated)
	}
}

func TestLiveServerActiveDownloadRemoveWithFiles(t *testing.T) {
	needsEngine(t)
	tmpDir, err := os.MkdirTemp("", "conduit-int-remove-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	downloadDir := filepath.Join(tmpDir, "downloads")
	_ = os.MkdirAll(downloadDir, 0755)

	rpcPort := getFreePort(t)

	// Slow chunk server (50MB). It serves on a listener bound here, so its port
	// cannot be taken between choosing it and serving.
	slowLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for slow chunk server: %v", err)
	}
	slowPort := slowLn.Addr().(*net.TCPAddr).Port
	chunkData := make([]byte, 1024*1024)
	slowSrv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", 50*1024*1024))
			for i := 0; i < 50; i++ {
				_, _ = w.Write(chunkData)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(100 * time.Millisecond)
			}
		}),
	}
	go func() {
		_ = slowSrv.Serve(slowLn)
	}()
	defer slowSrv.Close()

	cfgMgr, err := config.NewManager(filepath.Join(configDir, "config.json"), nil)
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}
	_, _ = cfgMgr.Update(config.Config{
		DownloadDir:            downloadDir,
		MaxConnections:         16,
		MaxConcurrentDownloads: 5,
	})

	sup, err := aria.NewSupervisor(aria.SupervisorConfig{
		ConfigDir:   configDir,
		DownloadDir: downloadDir,
		RPCPort:     rpcPort,
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer func() {
		_ = sup.Stop()
	}()

	rpcClient := aria.NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", rpcPort), sup.RPCSecret())
	srv, httpPort := startLiveREST(t, rpcClient, cfgMgr)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	// Create download via REST
	createPayload := []byte(fmt.Sprintf(`{"url":"http://127.0.0.1:%d/largefile.bin","filename":"largefile.bin"}`, slowPort))
	resp, err := http.Post(baseURL+"/api/downloads", "application/json", bytes.NewReader(createPayload))
	if err != nil {
		t.Fatalf("failed to create download: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}
	var created map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&created)
	_ = resp.Body.Close()

	gid := created["gid"]
	if gid == "" {
		t.Fatal("expected non-empty gid")
	}

	// Wait for download to start and files to be created
	targetFile := filepath.Join(downloadDir, "largefile.bin")
	ariaFile := targetFile + ".aria2"

	var filesCreated bool
	for i := 0; i < 20; i++ {
		if _, err := os.Stat(ariaFile); err == nil {
			filesCreated = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !filesCreated {
		t.Fatal("expected .aria2 file to be created during download")
	}

	// Now call POST /api/downloads/{gid}/action with remove and deleteFile: true
	actionPayload := []byte(`{"action":"remove","deleteFile":true}`)
	actionResp, err := http.Post(fmt.Sprintf("%s/api/downloads/%s/action", baseURL, gid), "application/json", bytes.NewReader(actionPayload))
	if err != nil {
		t.Fatalf("failed to post remove action: %v", err)
	}
	if actionResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for remove action, got %d", actionResp.StatusCode)
	}
	_ = actionResp.Body.Close()

	// Verify both target file and .aria2 are removed
	if _, err := os.Stat(targetFile); !os.IsNotExist(err) {
		t.Fatalf("expected target file to be removed, got err: %v", err)
	}
	if _, err := os.Stat(ariaFile); !os.IsNotExist(err) {
		t.Fatalf("expected .aria2 file to be removed, got err: %v", err)
	}

	// Wait 2 seconds to ensure aria2 does NOT recreate the .aria2 file
	time.Sleep(2 * time.Second)

	files, err := os.ReadDir(downloadDir)
	if err != nil {
		t.Fatalf("failed to read download dir: %v", err)
	}
	if len(files) > 0 {
		var names []string
		for _, f := range files {
			names = append(names, f.Name())
		}
		t.Fatalf("expected download dir to be empty, but found lingering files: %v", names)
	}

	// Verify GET /api/downloads has 0 items
	listResp, err := http.Get(baseURL + "/api/downloads")
	if err != nil {
		t.Fatalf("failed to get downloads: %v", err)
	}
	var items []DownloadItem
	_ = json.NewDecoder(listResp.Body).Decode(&items)
	_ = listResp.Body.Close()

	if len(items) != 0 {
		t.Fatalf("expected 0 download items in list, but got %d: %+v", len(items), items)
	}

	// Verify TellStopped has 0 items
	stopped, err := rpcClient.TellStopped(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("failed to check TellStopped: %v", err)
	}
	if len(stopped) != 0 {
		t.Fatalf("expected 0 stopped items in aria2 memory, got %d: %+v", len(stopped), stopped)
	}
}

func TestLiveServerQoLFeatures(t *testing.T) {
	needsEngine(t)
	tmpDir, err := os.MkdirTemp("", "conduit-int-qol-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configDir := filepath.Join(tmpDir, "config")
	downloadDir1 := filepath.Join(tmpDir, "downloads1")
	downloadDir2 := filepath.Join(tmpDir, "downloads2")
	_ = os.MkdirAll(downloadDir1, 0755)
	_ = os.MkdirAll(downloadDir2, 0755)

	rpcPort := getFreePort(t)

	cfgMgr, err := config.NewManager(filepath.Join(configDir, "config.json"), nil)
	if err != nil {
		t.Fatalf("failed to create config manager: %v", err)
	}
	_, _ = cfgMgr.Update(config.Config{
		DownloadDir:            downloadDir1,
		MaxConnections:         16,
		MaxConcurrentDownloads: 5,
		MaxDownloadLimit:       "0",
	})

	sup, err := aria.NewSupervisor(aria.SupervisorConfig{
		ConfigDir:   configDir,
		DownloadDir: downloadDir1,
		RPCPort:     rpcPort,
	})
	if err != nil {
		t.Fatalf("failed to create supervisor: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := sup.Start(ctx); err != nil {
		t.Fatalf("failed to start supervisor: %v", err)
	}
	defer func() {
		_ = sup.Stop()
	}()

	rpcClient := aria.NewClient(fmt.Sprintf("http://127.0.0.1:%d/jsonrpc", rpcPort), sup.RPCSecret())
	srv, httpPort := startLiveREST(t, rpcClient, cfgMgr)
	defer func() {
		_ = srv.Shutdown(context.Background())
	}()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", httpPort)

	// --- 1. Test Dynamic Speed Limiter (PUT /api/config -> aria2 max-overall-download-limit) ---
	configPayload, _ := json.Marshal(map[string]any{
		"downloadDir":            downloadDir1,
		"maxConnections":         16,
		"maxConcurrentDownloads": 1,
		"maxDownloadLimit":       "1M",
	})
	putBody := bytes.NewReader(configPayload)
	putReq, _ := http.NewRequest(http.MethodPut, baseURL+"/api/config", putBody)
	putReq.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(putReq)
	if err != nil {
		t.Fatalf("PUT /api/config failed: %v", err)
	}
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for PUT /api/config, got %d", putResp.StatusCode)
	}
	var updatedCfg config.Config
	_ = json.NewDecoder(putResp.Body).Decode(&updatedCfg)
	_ = putResp.Body.Close()

	if updatedCfg.MaxDownloadLimit != "1M" {
		t.Fatalf("expected MaxDownloadLimit '1M', got '%s'", updatedCfg.MaxDownloadLimit)
	}

	opts, err := rpcClient.GetGlobalOption(context.Background())
	if err != nil {
		t.Fatalf("failed to get global options: %v", err)
	}
	if opts["max-overall-download-limit"] != "1M" && opts["max-overall-download-limit"] != "1048576" {
		t.Fatalf("expected aria2 option max-overall-download-limit to be '1M' or '1048576', got '%s'", opts["max-overall-download-limit"])
	}

	// --- 2. Test Queue Reordering (move_up / move_down) ---
	// Start a mock chunk server
	chunkPayload := make([]byte, 2*1024*1024) // 2MB
	for i := range chunkPayload {
		chunkPayload[i] = byte(i % 251)
	}

	slowLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen for slow chunk server: %v", err)
	}
	slowPort := slowLn.Addr().(*net.TCPAddr).Port
	slowSrv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(chunkPayload)))
			w.Header().Set("Content-Type", "application/octet-stream")
			// Serve in chunks with small delay
			step := 64 * 1024
			for offset := 0; offset < len(chunkPayload); offset += step {
				end := offset + step
				if end > len(chunkPayload) {
					end = len(chunkPayload)
				}
				_, _ = w.Write(chunkPayload[offset:end])
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				time.Sleep(30 * time.Millisecond)
			}
		}),
	}
	go func() {
		_ = slowSrv.Serve(slowLn)
	}()
	defer slowSrv.Close()

	// Enqueue first download (becomes active because maxConcurrentDownloads = 1)
	d1Payload, _ := json.Marshal(CreateDownloadRequest{
		URL:      fmt.Sprintf("http://127.0.0.1:%d/payload1.bin", slowPort),
		Filename: "payload1.bin",
	})
	r1, err := http.Post(baseURL+"/api/downloads", "application/json", bytes.NewReader(d1Payload))
	if err != nil {
		t.Fatalf("failed to create download 1: %v", err)
	}
	var res1 map[string]string
	_ = json.NewDecoder(r1.Body).Decode(&res1)
	_ = r1.Body.Close()
	gid1 := res1["gid"]

	// Enqueue second download (queued in waiting queue)
	d2Payload, _ := json.Marshal(CreateDownloadRequest{
		URL:      fmt.Sprintf("http://127.0.0.1:%d/payload2.bin", slowPort),
		Filename: "payload2.bin",
	})
	r2, err := http.Post(baseURL+"/api/downloads", "application/json", bytes.NewReader(d2Payload))
	if err != nil {
		t.Fatalf("failed to create download 2: %v", err)
	}
	var res2 map[string]string
	_ = json.NewDecoder(r2.Body).Decode(&res2)
	_ = r2.Body.Close()
	_ = res2["gid"]

	// Enqueue third download (queued behind download 2 in waiting queue)
	d3Payload, _ := json.Marshal(CreateDownloadRequest{
		URL:      fmt.Sprintf("http://127.0.0.1:%d/payload3.bin", slowPort),
		Filename: "payload3.bin",
	})
	r3, err := http.Post(baseURL+"/api/downloads", "application/json", bytes.NewReader(d3Payload))
	if err != nil {
		t.Fatalf("failed to create download 3: %v", err)
	}
	var res3 map[string]string
	_ = json.NewDecoder(r3.Body).Decode(&res3)
	_ = r3.Body.Close()
	gid3 := res3["gid"]

	// Verify move_up on active download returns 400 Bad Request (not in waiting queue)
	activeMoveUpBody := bytes.NewReader([]byte(`{"action":"move_up"}`))
	actResp, err := http.Post(fmt.Sprintf("%s/api/downloads/%s/action", baseURL, gid1), "application/json", activeMoveUpBody)
	if err != nil {
		t.Fatalf("failed to post move_up on active: %v", err)
	}
	if actResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for move_up on active download, got %d", actResp.StatusCode)
	}
	_ = actResp.Body.Close()

	// Reorder waiting queue: move gid3 up
	moveUpBody := bytes.NewReader([]byte(`{"action":"move_up"}`))
	moveResp, err := http.Post(fmt.Sprintf("%s/api/downloads/%s/action", baseURL, gid3), "application/json", moveUpBody)
	if err != nil {
		t.Fatalf("failed to post move_up: %v", err)
	}
	if moveResp.StatusCode != http.StatusOK {
		var errBuf bytes.Buffer
		_, _ = errBuf.ReadFrom(moveResp.Body)
		t.Fatalf("expected 200 OK for move_up on waiting download, got %d: %s", moveResp.StatusCode, errBuf.String())
	}
	_ = moveResp.Body.Close()

	// Reorder waiting queue: move gid3 down
	moveDownBody := bytes.NewReader([]byte(`{"action":"move_down"}`))
	moveDownResp, err := http.Post(fmt.Sprintf("%s/api/downloads/%s/action", baseURL, gid3), "application/json", moveDownBody)
	if err != nil {
		t.Fatalf("failed to post move_down: %v", err)
	}
	if moveDownResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for move_down on waiting download, got %d", moveDownResp.StatusCode)
	}
	_ = moveDownResp.Body.Close()

	// --- 3. Test Changing Download Folder Mid-Download (change_dir) ---
	// Wait until payload1.bin or payload1.bin.aria2 is created in downloadDir1
	file1InDir1 := filepath.Join(downloadDir1, "payload1.bin")
	aria1InDir1 := file1InDir1 + ".aria2"

	var started bool
	for i := 0; i < 40; i++ {
		if _, err := os.Stat(aria1InDir1); err == nil {
			started = true
			break
		}
		if _, err := os.Stat(file1InDir1); err == nil {
			started = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !started {
		t.Fatal("expected download 1 to start and create files in downloadDir1")
	}

	// Change directory to downloadDir2 mid-download
	changeDirPayload, _ := json.Marshal(map[string]string{
		"action": "change_dir",
		"newDir": downloadDir2,
	})
	changeDirBody := bytes.NewReader(changeDirPayload)
	cdResp, err := http.Post(fmt.Sprintf("%s/api/downloads/%s/action", baseURL, gid1), "application/json", changeDirBody)
	if err != nil {
		t.Fatalf("failed to request change_dir: %v", err)
	}
	if cdResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for change_dir, got %d", cdResp.StatusCode)
	}
	_ = cdResp.Body.Close()

	// Verify old files no longer exist in downloadDir1
	if _, err := os.Stat(file1InDir1); !os.IsNotExist(err) {
		t.Fatalf("expected payload1.bin to be moved out of downloadDir1, but it still exists")
	}
	if _, err := os.Stat(aria1InDir1); !os.IsNotExist(err) {
		t.Fatalf("expected payload1.bin.aria2 to be moved out of downloadDir1, but it still exists")
	}

	// Wait for downloads to complete
	deadline := time.Now().Add(15 * time.Second)
	file1InDir2 := filepath.Join(downloadDir2, "payload1.bin")
	for time.Now().Before(deadline) {
		// When complete, .aria2 is removed and payload1.bin is fully written
		if fi, err := os.Stat(file1InDir2); err == nil && fi.Size() == int64(len(chunkPayload)) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Verify file in downloadDir2 exists and matches expected content
	finalData, err := os.ReadFile(file1InDir2)
	if err != nil {
		t.Fatalf("failed to read moved and completed file in downloadDir2: %v", err)
	}
	if !bytes.Equal(finalData, chunkPayload) {
		t.Fatalf("content of payload1.bin in downloadDir2 does not match source: got %d bytes, want %d bytes", len(finalData), len(chunkPayload))
	}
}
