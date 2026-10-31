package aria

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// DefaultRPCEndpoint defines the default loopback JSON-RPC URL for Conduit.
const DefaultRPCEndpoint = "http://127.0.0.1:18801/jsonrpc"

// RPCError represents a JSON-RPC 2.0 error returned by aria2.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("aria2 rpc error (%d): %s", e.Code, e.Message)
}

type rpcRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// DownloadStatus describes an active, paused, or completed download item.
type DownloadStatus struct {
	GID             string     `json:"gid"`
	Status          string     `json:"status"` // active, waiting, paused, error, complete, removed
	TotalLength     int64      `json:"totalLength,string"`
	CompletedLength int64      `json:"completedLength,string"`
	DownloadSpeed   int64      `json:"downloadSpeed,string"`
	UploadSpeed     int64      `json:"uploadSpeed,string"`
	Dir             string     `json:"dir"`
	Files           []FileInfo `json:"files"`
	Connections     int64      `json:"connections,string"`
	ErrorCode       string     `json:"errorCode"`
	ErrorMessage    string     `json:"errorMessage"`
}

// FileInfo represents file metadata in an aria2 download.
type FileInfo struct {
	Index           string    `json:"index"`
	Path            string    `json:"path"`
	Length          int64     `json:"length,string"`
	CompletedLength int64     `json:"completedLength,string"`
	Selected        string    `json:"selected"`
	URIs            []FileURI `json:"uris"`
}

// FileURI represents an individual source URI and its status.
type FileURI struct {
	URI    string `json:"uri"`
	Status string `json:"status"`
}

// VersionInfo contains version and feature flags of the running aria2 daemon.
type VersionInfo struct {
	Version         string   `json:"version"`
	EnabledFeatures []string `json:"enabledFeatures"`
}

// DownloadOptions defines customizable parameters for adding downloads.
type DownloadOptions struct {
	Dir              string            `json:"dir,omitempty"`
	Out              string            `json:"out,omitempty"`
	Headers          []string          `json:"headers,omitempty"`
	Split            int               `json:"split,omitempty"`
	MaxConnPerServer int               `json:"maxConnPerServer,omitempty"`
	ExtraOptions     map[string]string `json:"extraOptions,omitempty"`
}

// ToMap converts DownloadOptions into the map format expected by aria2.
func (o *DownloadOptions) ToMap() map[string]interface{} {
	m := make(map[string]interface{})
	if o == nil {
		return m
	}

	if o.Dir != "" {
		m["dir"] = o.Dir
	}
	if o.Out != "" {
		m["out"] = o.Out
	}
	if len(o.Headers) > 0 {
		m["header"] = o.Headers
	}
	if o.Split > 0 {
		m["split"] = strconv.Itoa(o.Split)
	}
	if o.MaxConnPerServer > 0 {
		m["max-connection-per-server"] = strconv.Itoa(o.MaxConnPerServer)
	}
	for k, v := range o.ExtraOptions {
		m[k] = v
	}
	return m
}

// Client interacts with the aria2 JSON-RPC 2.0 endpoint.
type Client struct {
	endpoint   string
	secret     string
	httpClient *http.Client
	requestID  uint64
}

// NewClient creates a new JSON-RPC client. If endpoint is empty, DefaultRPCEndpoint is used.
func NewClient(endpoint, secret string) *Client {
	if endpoint == "" {
		endpoint = DefaultRPCEndpoint
	}
	return &Client{
		endpoint: endpoint,
		secret:   secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Call sends a JSON-RPC 2.0 request to aria2 and decodes the result.
func (c *Client) Call(ctx context.Context, method string, params []interface{}, out interface{}) error {
	var finalParams []interface{}
	if c.secret != "" {
		finalParams = append(finalParams, fmt.Sprintf("token:%s", c.secret))
	}
	finalParams = append(finalParams, params...)

	reqID := atomic.AddUint64(&c.requestID, 1)
	reqBody := rpcRequest{
		JSONRPC: "2.0",
		ID:      strconv.FormatUint(reqID, 10),
		Method:  method,
		Params:  finalParams,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal rpc request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respData, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected http status %d: %s", resp.StatusCode, string(respData))
	}

	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return fmt.Errorf("failed to decode rpc response: %w", err)
	}

	if rpcResp.Error != nil {
		return rpcResp.Error
	}

	if out != nil && len(rpcResp.Result) > 0 {
		if err := json.Unmarshal(rpcResp.Result, out); err != nil {
			return fmt.Errorf("failed to unmarshal rpc result into target struct: %w", err)
		}
	}

	return nil
}

// AddURI adds a new download given one or more source URIs and optional download options. Returns the GID.
func (c *Client) AddURI(ctx context.Context, uris []string, opts *DownloadOptions) (string, error) {
	if len(uris) == 0 {
		return "", errors.New("at least one uri is required")
	}

	params := []interface{}{uris}
	if opts != nil {
		params = append(params, opts.ToMap())
	}

	var gid string
	if err := c.Call(ctx, "aria2.addUri", params, &gid); err != nil {
		return "", err
	}
	return gid, nil
}

// TellActive returns the list of active downloads.
func (c *Client) TellActive(ctx context.Context, keys ...string) ([]DownloadStatus, error) {
	var params []interface{}
	if len(keys) > 0 {
		params = append(params, keys)
	}

	var results []DownloadStatus
	if err := c.Call(ctx, "aria2.tellActive", params, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// TellWaiting returns the list of waiting or paused downloads.
func (c *Client) TellWaiting(ctx context.Context, offset, num int, keys ...string) ([]DownloadStatus, error) {
	params := []interface{}{offset, num}
	if len(keys) > 0 {
		params = append(params, keys)
	}

	var results []DownloadStatus
	if err := c.Call(ctx, "aria2.tellWaiting", params, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// TellStopped returns the list of completed, error, or removed downloads.
func (c *Client) TellStopped(ctx context.Context, offset, num int, keys ...string) ([]DownloadStatus, error) {
	params := []interface{}{offset, num}
	if len(keys) > 0 {
		params = append(params, keys)
	}

	var results []DownloadStatus
	if err := c.Call(ctx, "aria2.tellStopped", params, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// TellStatus returns the status of a specific download by its GID.
func (c *Client) TellStatus(ctx context.Context, gid string, keys ...string) (*DownloadStatus, error) {
	params := []interface{}{gid}
	if len(keys) > 0 {
		params = append(params, keys)
	}

	var status DownloadStatus
	if err := c.Call(ctx, "aria2.tellStatus", params, &status); err != nil {
		return nil, err
	}
	return &status, nil
}

// Pause pauses an active download by its GID.
func (c *Client) Pause(ctx context.Context, gid string) error {
	var result string
	return c.Call(ctx, "aria2.pause", []interface{}{gid}, &result)
}

// Unpause resumes a paused download by its GID.
func (c *Client) Unpause(ctx context.Context, gid string) error {
	var result string
	return c.Call(ctx, "aria2.unpause", []interface{}{gid}, &result)
}

// ForceRemove forcefully halts and removes a download by GID without waiting for slow operations.
func (c *Client) ForceRemove(ctx context.Context, gid string) error {
	var result string
	return c.Call(ctx, "aria2.forceRemove", []interface{}{gid}, &result)
}

// Remove completely stops and purges a download by its GID.
// For active/waiting/paused downloads, it halts them and removes the result from memory.
// For completed/stopped/error downloads, it directly removes the download result from memory.
func (c *Client) Remove(ctx context.Context, gid string) error {
	var result string
	err := c.Call(ctx, "aria2.forceRemove", []interface{}{gid}, &result)
	if err != nil {
		err = c.Call(ctx, "aria2.remove", []interface{}{gid}, &result)
	}

	if err != nil {
		// If download is already stopped, completed, or error, purge it from memory
		return c.RemoveDownloadResult(ctx, gid)
	}

	// Download was active/waiting; wait briefly for status to become stopped/removed and purge from memory
	for i := 0; i < 20; i++ {
		rErr := c.RemoveDownloadResult(ctx, gid)
		if rErr == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return nil
}

// RemoveDownloadResult explicitly removes a completed, stopped, or error download from memory.
func (c *Client) RemoveDownloadResult(ctx context.Context, gid string) error {
	var result string
	return c.Call(ctx, "aria2.removeDownloadResult", []interface{}{gid}, &result)
}

// ChangePosition changes the position of a waiting download in the queue.
// pos is an integer, how must be "POS_SET", "POS_CUR", or "POS_END".
func (c *Client) ChangePosition(ctx context.Context, gid string, pos int, how string) (int, error) {
	var result int
	err := c.Call(ctx, "aria2.changePosition", []interface{}{gid, pos, how}, &result)
	return result, err
}

// ChangeGlobalOption dynamically modifies global options.
func (c *Client) ChangeGlobalOption(ctx context.Context, opts map[string]string) error {
	var result string
	return c.Call(ctx, "aria2.changeGlobalOption", []interface{}{opts}, &result)
}

// GetOption returns the options of one download. aria2 keeps them for
// stopped and failed downloads too.
func (c *Client) GetOption(ctx context.Context, gid string) (map[string]string, error) {
	var result map[string]string
	if err := c.Call(ctx, "aria2.getOption", []interface{}{gid}, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetGlobalOption retrieves currently set global options.
func (c *Client) GetGlobalOption(ctx context.Context) (map[string]string, error) {
	var result map[string]string
	if err := c.Call(ctx, "aria2.getGlobalOption", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetVersion returns the aria2 version and compiled features.
func (c *Client) GetVersion(ctx context.Context) (*VersionInfo, error) {
	var version VersionInfo
	if err := c.Call(ctx, "aria2.getVersion", nil, &version); err != nil {
		return nil, err
	}
	return &version, nil
}

// Shutdown gracefully instructs aria2 to flush sessions and terminate.
func (c *Client) Shutdown(ctx context.Context) error {
	var result string
	return c.Call(ctx, "aria2.shutdown", nil, &result)
}
