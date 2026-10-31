package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/platform"
)

// openFolder is replaced in tests so a folder action does not open the host file manager.
var openFolder = platform.OpenFolder

// openFile is replaced in tests so an open action does not launch an application.
var openFile = platform.OpenFile

// CreateDownloadRequest represents the payload from browser extensions or web UI.
type CreateDownloadRequest struct {
	URL       string `json:"url"`
	Filename  string `json:"filename,omitempty"`
	Referrer  string `json:"referrer,omitempty"`
	UserAgent string `json:"userAgent,omitempty"`
	Cookies   string `json:"cookies,omitempty"`
}

// DownloadItem represents normalized download status returned to clients.
type DownloadItem struct {
	GID             string `json:"gid"`
	Filename        string `json:"filename"`
	Status          string `json:"status"`
	TotalLength     int64  `json:"totalLength"`
	CompletedLength int64  `json:"completedLength"`
	DownloadSpeed   int64  `json:"downloadSpeed"`
	Dir             string `json:"dir"`
	ErrorMessage    string `json:"errorMessage"`
	Connections     int64  `json:"connections"`
	URL             string `json:"url,omitempty"`
}

// DownloadActionRequest represents an action to execute on a download item.
type DownloadActionRequest struct {
	Action     string `json:"action"` // pause, resume, remove, open_folder, open_file, retry, change_dir, move_up, move_down
	DeleteFile bool   `json:"deleteFile,omitempty"`
	NewDir     string `json:"newDir,omitempty"`
}

// validateDownloadURL accepts the protocols supported by Conduit v1. Keeping
// this check on the server means a direct API call cannot smuggle file:// or
// another scheme into aria2 even though the dashboard and extension check it.
func validateDownloadURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("invalid url")
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "ftp":
	default:
		return errors.New("url must use http, https, or ftp")
	}
	if u.Host == "" {
		return errors.New("url must include a host")
	}
	return nil
}

func (s *Server) handleCreateDownload(w http.ResponseWriter, r *http.Request) {
	if !s.requireEngine(w) {
		return
	}

	var req CreateDownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.URL = strings.TrimSpace(req.URL)
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "url is required")
		return
	}
	if err := validateDownloadURL(req.URL); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	opts := &aria.DownloadOptions{}
	if req.Filename != "" {
		cleanName := filepath.Base(filepath.Clean(req.Filename))
		if cleanName == "." || cleanName == "/" || cleanName == "\\" || strings.Contains(req.Filename, "..") {
			writeError(w, http.StatusBadRequest, "invalid filename: path traversal not permitted")
			return
		}
		if cleanName != "" && cleanName != "download" {
			opts.Out = cleanName
		}
	}

	if s.configMgr != nil {
		opts.Dir = s.configMgr.Get().DownloadDir
	}

	var headers []string
	if req.Cookies != "" {
		headers = append(headers, "Cookie: "+req.Cookies)
	}
	if req.Referrer != "" {
		headers = append(headers, "Referer: "+req.Referrer)
	}
	if req.UserAgent != "" {
		headers = append(headers, "User-Agent: "+req.UserAgent)
	}
	opts.Headers = headers

	gid, err := s.rpcClient.AddURI(r.Context(), []string{req.URL}, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to enqueue download: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"gid": gid,
	})
	go s.wsHub.BroadcastAll(context.Background())
}

func (s *Server) handleListDownloads(w http.ResponseWriter, r *http.Request) {
	if !s.requireEngine(w) {
		return
	}
	items, err := s.getDownloadItems(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to query downloads: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// requireEngine answers 503 while aria2 is starting, restarting, missing, or offline.
func (s *Server) requireEngine(w http.ResponseWriter) bool {
	s.mu.Lock()
	statusFn := s.engineStatus
	s.mu.Unlock()
	state := ""
	if statusFn != nil {
		state = statusFn()
	}
	if s.rpcClient == nil || (statusFn != nil && state != "ready") {
		if state == "missing" {
			writeError(w, http.StatusServiceUnavailable, "download engine (aria2c) is not installed")
		} else {
			writeError(w, http.StatusServiceUnavailable, "download engine is not running")
		}
		return false
	}
	return true
}

// downloadListPageSize is the engine's page size for waiting and stopped
// lists. The engine has no "all" value, so these lists must be paged.
const downloadListPageSize = 100

func (s *Server) getDownloadItems(ctx context.Context) ([]DownloadItem, error) {
	if s.rpcClient == nil {
		return []DownloadItem{}, nil
	}

	active, err := s.rpcClient.TellActive(ctx)
	if err != nil {
		return nil, err
	}

	waiting, err := listAllDownloads(ctx, s.rpcClient.TellWaiting)
	if err != nil {
		return nil, err
	}

	stopped, err := listAllDownloads(ctx, s.rpcClient.TellStopped)
	if err != nil {
		return nil, err
	}

	var allItems []DownloadItem
	// A download can move between the active, waiting, and stopped lists
	// while the three calls run, so keep only the first copy of each GID.
	seen := make(map[string]bool, len(active)+len(waiting)+len(stopped))
	add := func(item aria.DownloadStatus) {
		if seen[item.GID] {
			return
		}
		seen[item.GID] = true
		allItems = append(allItems, s.mapToDownloadItem(item))
	}
	for _, item := range active {
		add(item)
	}
	for _, item := range waiting {
		add(item)
	}
	for _, item := range stopped {
		if item.Status == "removed" {
			// Auto-clean any lingering removed item from engine memory
			go func(gid string) {
				_ = s.rpcClient.RemoveDownloadResult(context.Background(), gid)
			}(item.GID)
			continue
		}
		add(item)
	}

	if allItems == nil {
		allItems = []DownloadItem{}
	}

	return allItems, nil
}

type downloadPageFunc func(ctx context.Context, offset, num int, keys ...string) ([]aria.DownloadStatus, error)

// listAllDownloads collects pages until the engine returns a short page.
// Without this, the 100-item page cap silently hides older waiting entries and
// any completed download past the first hundred.
func listAllDownloads(ctx context.Context, fetch downloadPageFunc) ([]aria.DownloadStatus, error) {
	var all []aria.DownloadStatus
	seen := make(map[string]bool)
	for offset := 0; ; {
		page, err := fetch(ctx, offset, downloadListPageSize)
		if err != nil {
			return nil, err
		}
		added := 0
		for _, item := range page {
			if seen[item.GID] {
				continue
			}
			seen[item.GID] = true
			all = append(all, item)
			added++
		}
		// Stop on a short page, or if a bad engine ignored offset and
		// returned the same page again.
		if len(page) < downloadListPageSize || added == 0 {
			return all, nil
		}
		offset += len(page)
	}
}

func (s *Server) handleDownloadAction(w http.ResponseWriter, r *http.Request) {
	if !s.requireEngine(w) {
		return
	}

	gid := r.PathValue("gid")
	if gid == "" {
		writeError(w, http.StatusBadRequest, "gid is required")
		return
	}

	var req DownloadActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	var actionErr error
	changesList := true // most actions alter the list; push it right away

	switch req.Action {
	case "pause":
		actionErr = s.rpcClient.Pause(ctx, gid)
	case "resume":
		actionErr = s.rpcClient.Unpause(ctx, gid)
	case "retry":
		s.htmlErrorCache.Delete(gid)
		actionErr = s.retryDownload(ctx, gid)
	case "remove":
		s.htmlErrorCache.Delete(gid)
		var targetFiles []string
		if req.DeleteFile {
			targetFiles = s.findDownloadFiles(ctx, gid)
		}
		actionErr = s.rpcClient.Remove(ctx, gid)
		// Delete files only after the engine has accepted the removal. If it
		// failed, an active transfer may still be writing them.
		if actionErr == nil {
			for _, p := range targetFiles {
				_ = removeFileWithRetry(p)
				_ = removeFileWithRetry(p + ".aria2")
			}
		}
	case "open_folder":
		changesList = false
		targetPath, err := s.findDownloadPath(ctx, gid)
		if err != nil {
			actionErr = err
			break
		}
		actionErr = openFolder(targetPath)
	case "open_file":
		changesList = false
		actionErr = s.openCompletedFile(ctx, gid)
		if errors.Is(actionErr, errFileUnavailable) {
			writeError(w, http.StatusBadRequest, actionErr.Error())
			return
		}
	case "move_up", "move_down":
		step := -1
		if req.Action == "move_down" {
			step = 1
		}
		_, actionErr = s.rpcClient.ChangePosition(ctx, gid, step, "POS_CUR")
		if actionErr != nil && strings.Contains(actionErr.Error(), "not found in the waiting queue") {
			writeError(w, http.StatusBadRequest, "download is not in the waiting queue")
			return
		}
	case "change_dir":
		if strings.TrimSpace(req.NewDir) == "" {
			writeError(w, http.StatusBadRequest, "newDir is required for change_dir")
			return
		}
		actionErr = s.moveDownloadFolder(ctx, gid, req.NewDir)
	default:
		writeError(w, http.StatusBadRequest, "unsupported action: "+req.Action)
		return
	}

	if changesList {
		go s.wsHub.BroadcastAll(context.Background())
	}
	if actionErr != nil {
		writeError(w, http.StatusInternalServerError, actionErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

var errFileUnavailable = errors.New("file is not available")

// openCompletedFile launches a finished download. The path comes from the
// engine, never from the request, and only a complete file that still exists
// on disk is opened.
func (s *Server) openCompletedFile(ctx context.Context, gid string) error {
	status, err := s.rpcClient.TellStatus(ctx, gid)
	if err != nil || status == nil || status.Status != "complete" ||
		len(status.Files) == 0 || status.Files[0].Path == "" {
		return errFileUnavailable
	}
	path := status.Files[0].Path
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		return errFileUnavailable
	}
	return openFile(path)
}

// retryDownload re-adds a stopped download from its URIs and drops the old
// entry. aria2 resumes from the partial file and its .aria2 control file, and
// the original cookies, referrer, and user agent are carried over. Without
// URIs the best we can do is unpause it.
func (s *Server) retryDownload(ctx context.Context, gid string) error {
	status, err := s.rpcClient.TellStatus(ctx, gid)
	if err != nil || status == nil {
		return s.rpcClient.Unpause(ctx, gid)
	}
	uris := downloadURIs(*status)
	if len(uris) == 0 {
		return s.rpcClient.Unpause(ctx, gid)
	}
	opts := &aria.DownloadOptions{Dir: status.Dir, Headers: s.downloadHeaders(ctx, gid)}
	if len(status.Files) > 0 && status.Files[0].Path != "" {
		opts.Out = filepath.Base(status.Files[0].Path)
	}
	if _, err := s.rpcClient.AddURI(ctx, uris, opts); err != nil {
		return err
	}
	_ = s.rpcClient.Remove(ctx, gid)
	return nil
}

// downloadHeaders returns the request headers (cookies, referrer, user agent)
// a download was added with, so a re-added copy can authenticate the same way.
func (s *Server) downloadHeaders(ctx context.Context, gid string) []string {
	opts, err := s.rpcClient.GetOption(ctx, gid)
	if err != nil {
		return nil
	}
	var headers []string
	for _, h := range strings.Split(opts["header"], "\n") {
		if h = strings.TrimSpace(h); h != "" {
			headers = append(headers, h)
		}
	}
	return headers
}

func downloadURIs(status aria.DownloadStatus) []string {
	if len(status.Files) == 0 {
		return nil
	}
	var uris []string
	for _, u := range status.Files[0].URIs {
		if u.URI != "" {
			uris = append(uris, u.URI)
		}
	}
	return uris
}

func (s *Server) findDownloadPath(ctx context.Context, gid string) (string, error) {
	if s.rpcClient == nil {
		return "", errors.New("download path is unknown")
	}
	status, err := s.rpcClient.TellStatus(ctx, gid)
	if err != nil || status == nil {
		return "", errors.New("download path is unknown")
	}
	if len(status.Files) > 0 && status.Files[0].Path != "" {
		return status.Files[0].Path, nil
	}
	if status.Dir != "" {
		return status.Dir, nil
	}
	return "", errors.New("download path is unknown")
}

func (s *Server) findDownloadFiles(ctx context.Context, gid string) []string {
	if s.rpcClient == nil {
		return nil
	}

	status, err := s.rpcClient.TellStatus(ctx, gid)
	if err != nil || status == nil {
		return nil
	}

	var files []string
	for _, f := range status.Files {
		if f.Path != "" {
			files = append(files, f.Path)
		}
	}

	if len(files) == 0 && status.Dir != "" {
		filename := determineFilename(*status)
		if filename != "" && filename != gid {
			files = append(files, filepath.Join(status.Dir, filename))
		}
	}

	return files
}

func removeFileWithRetry(path string) error {
	if path == "" {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if fi.IsDir() {
		return nil
	}

	for i := 0; i < 5; i++ {
		err = os.Remove(path)
		if err == nil || os.IsNotExist(err) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return err
}

func (s *Server) moveDownloadFolder(ctx context.Context, gid, newDir string) error {
	cleanNewDir := filepath.Clean(strings.TrimSpace(newDir))
	if cleanNewDir == "." || cleanNewDir == "" {
		return errors.New("invalid target directory")
	}

	status, err := s.rpcClient.TellStatus(ctx, gid)
	if err != nil {
		return fmt.Errorf("failed to retrieve download status: %w", err)
	}
	if status == nil {
		return errors.New("download not found")
	}

	cleanCurrentDir := filepath.Clean(status.Dir)
	if cleanCurrentDir == cleanNewDir {
		return nil
	}

	if err := os.MkdirAll(cleanNewDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	wasActive := status.Status == "active" || status.Status == "waiting"
	if status.Status == "active" {
		_ = s.rpcClient.Pause(ctx, gid)
		for i := 0; i < 20; i++ {
			time.Sleep(50 * time.Millisecond)
			st, err := s.rpcClient.TellStatus(ctx, gid)
			if err == nil && st != nil && (st.Status == "paused" || st.Status == "waiting") {
				status = st
				break
			}
		}
	}

	filesToMove := s.findDownloadFiles(ctx, gid)
	var allFiles []string
	for _, f := range filesToMove {
		allFiles = append(allFiles, f)
		if !strings.HasSuffix(f, ".aria2") {
			allFiles = append(allFiles, f+".aria2")
		}
	}

	uris := downloadURIs(*status)
	headers := s.downloadHeaders(ctx, gid)
	filename := determineFilename(*status)

	var moved [][2]string
	for _, src := range allFiles {
		fi, err := os.Stat(src)
		if err != nil || fi.IsDir() {
			continue
		}
		dst := filepath.Join(cleanNewDir, filepath.Base(src))
		if err := moveFile(src, dst); err != nil {
			for i := len(moved) - 1; i >= 0; i-- {
				_ = moveFile(moved[i][1], moved[i][0])
			}
			if wasActive {
				_ = s.rpcClient.Unpause(ctx, gid)
			}
			return fmt.Errorf("failed to move %s to %s: %w", src, dst, err)
		}
		moved = append(moved, [2]string{src, dst})
	}

	_ = s.rpcClient.Remove(ctx, gid)
	_ = s.rpcClient.RemoveDownloadResult(ctx, gid)

	if len(uris) > 0 {
		opts := &aria.DownloadOptions{
			Dir:     cleanNewDir,
			Headers: headers,
		}
		if filename != "" && filename != gid {
			opts.Out = filename
		}
		newGid, err := s.rpcClient.AddURI(ctx, uris, opts)
		if err != nil {
			return fmt.Errorf("failed to re-enqueue download in new directory: %w", err)
		}
		if !wasActive {
			_ = s.rpcClient.Pause(ctx, newGid)
		}
	}

	return nil
}

var (
	renameFile = os.Rename
	copyFile   = io.Copy
	moveFile   = moveFileWithFallback
)

func moveFileWithFallback(src, dst string) error {
	if src == dst {
		return nil
	}
	if err := renameFile(src, dst); err == nil {
		return nil
	}

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".conduit-move-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	if _, err := copyFile(tmp, srcFile); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := srcFile.Close(); err != nil {
		return err
	}
	if err := replaceWithTemp(tmpName, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func replaceWithTemp(tmpName, dst string) error {
	if err := renameFile(tmpName, dst); err == nil {
		return nil
	}

	backup := dst + ".conduit.bak"
	_ = os.Remove(backup)
	if _, err := os.Stat(dst); err == nil {
		if err := renameFile(dst, backup); err != nil {
			return err
		}
		if err := renameFile(tmpName, dst); err != nil {
			_ = renameFile(backup, dst)
			return err
		}
		_ = os.Remove(backup)
		return nil
	}

	return renameFile(tmpName, dst)
}

type htmlCheckResult struct {
	isError bool
	errMsg  string
}

func (s *Server) mapToDownloadItem(item aria.DownloadStatus) DownloadItem {
	filename := determineFilename(item)
	status := item.Status
	errMsg := item.ErrorMessage

	if status == "complete" {
		if isErr, htmlErr := s.checkHTMLContent(item.GID, item, filename); isErr {
			status = "error"
			errMsg = htmlErr
		}
	}

	return DownloadItem{
		GID:             item.GID,
		Filename:        filename,
		Status:          status,
		TotalLength:     item.TotalLength,
		CompletedLength: item.CompletedLength,
		DownloadSpeed:   item.DownloadSpeed,
		Dir:             item.Dir,
		ErrorMessage:    errMsg,
		Connections:     item.Connections,
		URL:             firstURI(item),
	}
}

func firstURI(status aria.DownloadStatus) string {
	if uris := downloadURIs(status); len(uris) > 0 {
		return uris[0]
	}
	return ""
}

func (s *Server) checkHTMLContent(gid string, item aria.DownloadStatus, filename string) (bool, string) {
	if s != nil {
		if val, ok := s.htmlErrorCache.Load(gid); ok {
			res := val.(htmlCheckResult)
			return res.isError, res.errMsg
		}
	}

	var filePath string
	if len(item.Files) > 0 && item.Files[0].Path != "" {
		filePath = item.Files[0].Path
	} else if item.Dir != "" && filename != "" {
		filePath = filepath.Join(item.Dir, filename)
	}

	isErr, errMsg := isUnexpectedHTMLContent(filePath, filename, item.TotalLength)
	if s != nil {
		s.htmlErrorCache.Store(gid, htmlCheckResult{isError: isErr, errMsg: errMsg})
	}
	return isErr, errMsg
}

// htmlHeadTag and htmlTitleTag match the tags themselves. A plain substring
// match would also flag <header> as <head>, which breaks XML feeds and other
// markup that a user downloads on purpose.
var (
	htmlHeadTag  = regexp.MustCompile(`<head[\s>]`)
	htmlTitleTag = regexp.MustCompile(`<title[\s>]`)
)

func isUnexpectedHTMLContent(filePath, filename string, totalBytes int64) (bool, string) {
	if filePath == "" || totalBytes > 2*1024*1024 || totalBytes <= 0 {
		return false, ""
	}

	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".html" || ext == ".htm" || ext == ".xhtml" {
		return false, ""
	}

	f, err := os.Open(filePath)
	if err != nil {
		return false, ""
	}
	defer f.Close()

	buf := make([]byte, 2048)
	n, err := f.Read(buf)
	if err != nil || n == 0 {
		return false, ""
	}

	content := string(buf[:n])
	trimmed := strings.ToLower(strings.TrimSpace(content))

	if strings.HasPrefix(trimmed, "<!doctype html") ||
		strings.HasPrefix(trimmed, "<html") ||
		(htmlHeadTag.MatchString(trimmed) && htmlTitleTag.MatchString(trimmed)) {
		title := extractHTMLTitle(content)
		if title != "" {
			return true, fmt.Sprintf("Server returned an HTML error/landing page (%q) instead of the requested file", title)
		}
		return true, "Server returned an HTML error/landing page instead of the requested file"
	}

	return false, ""
}

func extractHTMLTitle(content string) string {
	lower := strings.ToLower(content)
	start := strings.Index(lower, "<title>")
	if start == -1 {
		start = strings.Index(lower, "<title ")
		if start != -1 {
			closeBracket := strings.Index(lower[start:], ">")
			if closeBracket != -1 {
				start = start + closeBracket + 1
			} else {
				return ""
			}
		} else {
			return ""
		}
	} else {
		start += len("<title>")
	}
	end := strings.Index(lower[start:], "</title>")
	if end == -1 {
		return ""
	}
	rawTitle := strings.TrimSpace(content[start : start+end])
	fields := strings.Fields(rawTitle)
	clean := strings.Join(fields, " ")
	if len(clean) > 80 {
		clean = clean[:77] + "..."
	}
	return clean
}

func determineFilename(item aria.DownloadStatus) string {
	if len(item.Files) > 0 {
		if item.Files[0].Path != "" {
			base := filepath.Base(item.Files[0].Path)
			if base != "." && base != "/" && base != "\\" {
				return base
			}
		}
		if len(item.Files[0].URIs) > 0 {
			u, err := url.Parse(item.Files[0].URIs[0].URI)
			if err == nil {
				base := filepath.Base(u.Path)
				if base != "" && base != "." && base != "/" {
					return base
				}
			}
		}
	}
	return item.GID
}
