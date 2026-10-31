package aria

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRPCClientMock(t *testing.T) {
	const testSecret = "secret-12345"

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req rpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.JSONRPC != "2.0" {
			t.Errorf("expected jsonrpc '2.0', got '%s'", req.JSONRPC)
		}

		if len(req.Params) == 0 || req.Params[0] != "token:"+testSecret {
			t.Errorf("expected first param to be 'token:%s', got %v", testSecret, req.Params)
		}

		resp := rpcResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
		}

		switch req.Method {
		case "aria2.getVersion":
			v := VersionInfo{
				Version:         "1.37.0",
				EnabledFeatures: []string{"Async DNS", "BitTorrent", "GZip", "HTTPS"},
			}
			raw, _ := json.Marshal(v)
			resp.Result = raw

		case "aria2.addUri":
			resp.Result = json.RawMessage(`"2089b05ecca3d829"`)

		case "aria2.tellActive":
			downloads := []DownloadStatus{
				{
					GID:             "2089b05ecca3d829",
					Status:          "active",
					TotalLength:     1048576,
					CompletedLength: 524288,
					DownloadSpeed:   262144,
					Dir:             "/tmp/downloads",
				},
			}
			raw, _ := json.Marshal(downloads)
			resp.Result = raw

		case "aria2.pause", "aria2.unpause", "aria2.remove", "aria2.forceRemove", "aria2.removeDownloadResult", "aria2.shutdown":
			resp.Result = json.RawMessage(`"OK"`)

		case "aria2.errorMethod":
			resp.Error = &RPCError{
				Code:    1,
				Message: "Gid not found",
			}

		default:
			http.Error(w, "unknown method", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewClient(ts.URL, testSecret)
	ctx := context.Background()

	// 1. GetVersion
	ver, err := client.GetVersion(ctx)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}
	if ver.Version != "1.37.0" {
		t.Fatalf("expected version 1.37.0, got %s", ver.Version)
	}

	// 2. AddURI with options
	gid, err := client.AddURI(ctx, []string{"https://example.com/test.zip"}, &DownloadOptions{
		Out:     "test.zip",
		Headers: []string{"User-Agent: Conduit/1.0", "Cookie: sid=abc"},
	})
	if err != nil {
		t.Fatalf("AddURI failed: %v", err)
	}
	if gid != "2089b05ecca3d829" {
		t.Fatalf("expected gid '2089b05ecca3d829', got %s", gid)
	}

	// 3. TellActive
	active, err := client.TellActive(ctx)
	if err != nil {
		t.Fatalf("TellActive failed: %v", err)
	}
	if len(active) != 1 || active[0].GID != "2089b05ecca3d829" {
		t.Fatalf("unexpected active downloads: %v", active)
	}
	if active[0].TotalLength != 1048576 || active[0].CompletedLength != 524288 {
		t.Fatalf("unexpected length parsing: total=%d completed=%d", active[0].TotalLength, active[0].CompletedLength)
	}

	// 4. Pause / Unpause / ForceRemove / Remove
	if err := client.Pause(ctx, gid); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if err := client.Unpause(ctx, gid); err != nil {
		t.Fatalf("Unpause failed: %v", err)
	}
	if err := client.ForceRemove(ctx, gid); err != nil {
		t.Fatalf("ForceRemove failed: %v", err)
	}
	if err := client.Remove(ctx, gid); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// 5. Shutdown
	if err := client.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown failed: %v", err)
	}

	// 6. Error handling
	var dummy string
	err = client.Call(ctx, "aria2.errorMethod", nil, &dummy)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) {
		t.Fatalf("expected RPCError, got: %v", err)
	}
	if rpcErr.Code != 1 {
		t.Fatalf("expected error code 1, got %d", rpcErr.Code)
	}
}
