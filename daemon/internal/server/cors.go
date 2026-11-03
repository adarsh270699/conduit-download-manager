package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

var allowedExtensionOrigins = loadAllowedExtensionOrigins()

// originKind says how much a request's browser origin may do.
type originKind int

const (
	// originDenied is rejected before any handler runs.
	originDenied originKind = iota
	// originTrusted has the whole API: this daemon's own dashboard, the Chrome
	// extension whose ID comes from the pinned key, and clients with no Origin.
	originTrusted
	// originFirefox is any Firefox extension. Firefox assigns each install a
	// random UUID, so the daemon cannot tell Conduit from another add-on. CORS
	// is the only gate on an extension page, so these origins get only what
	// firefoxRequestAllowed lists.
	originFirefox
	// originShell is the Wails bootstrap page. It only polls /api/health before
	// it moves to the daemon's own origin.
	originShell
)

// classifyOrigin sorts a request's Origin header. Only this daemon's own port
// counts as the dashboard: a page from any other local web server is a
// different origin and has no claim on this API.
func classifyOrigin(port int, origin string) originKind {
	if origin == "" {
		return originTrusted
	}
	if origin == fmt.Sprintf("http://127.0.0.1:%d", port) || origin == fmt.Sprintf("http://localhost:%d", port) {
		return originTrusted
	}
	if strings.HasPrefix(origin, "chrome-extension://") {
		if _, ok := allowedExtensionOrigins[origin]; ok {
			return originTrusted
		}
		return originDenied
	}
	if strings.HasPrefix(origin, "moz-extension://") {
		return originFirefox
	}
	if origin == "http://wails.localhost" || origin == "https://wails.localhost" || strings.HasPrefix(origin, "wails://") {
		return originShell
	}
	return originDenied
}

// firefoxRequestAllowed lists what the extension's own code calls. Nothing here
// can start a program, change where files go, delete a file, or install an
// update. It may replace r.Body after reading it.
func firefoxRequestAllowed(r *http.Request) bool {
	path := r.URL.Path
	switch r.Method {
	case http.MethodGet:
		return path == "/api/health" || path == "/api/downloads" || path == "/api/config"
	case http.MethodPost:
		if path == "/api/downloads" {
			return true
		}
		parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
		if len(parts) != 4 || parts[0] != "api" || parts[1] != "downloads" || parts[3] != "action" {
			return false
		}
		var req DownloadActionRequest
		if !peekJSON(r, &req) || req.DeleteFile {
			return false
		}
		return req.Action == "pause" || req.Action == "resume" || req.Action == "remove"
	case http.MethodPut:
		if path != "/api/config" {
			return false
		}
		var fields map[string]json.RawMessage
		if !peekJSON(r, &fields) {
			return false
		}
		_, hasTheme := fields["theme"]
		return hasTheme && len(fields) == 1
	}
	return false
}

// peekJSON decodes the body into v and leaves it readable for the handler.
func peekJSON(r *http.Request, v any) bool {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return false
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	return json.Unmarshal(data, v) == nil
}

// CORSMiddleware applies CORS headers to API responses and rejects browser
// requests from origins that may not make them.
//
// Rejecting the request (not merely omitting the response header) matters:
// CORS does not stop a cross-site "simple" POST from reaching the handler.
// Without this check a website could enqueue downloads or quit the app by
// posting to 127.0.0.1 from the user's browser.
func CORSMiddleware(port int, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		kind := classifyOrigin(port, origin)
		if kind == originDenied {
			writeError(w, http.StatusForbidden, "origin not allowed")
			return
		}

		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Add("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		allowed := true
		switch kind {
		case originFirefox:
			allowed = firefoxRequestAllowed(r)
		case originShell:
			allowed = r.Method == http.MethodGet && r.URL.Path == "/api/health"
		}
		if !allowed {
			writeError(w, http.StatusForbidden, "origin may not make this request")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// hostGuard rejects requests that name anything but this machine. A site that
// rebinds its DNS name to 127.0.0.1 is same-origin with itself, so its GETs
// carry no Origin header. The Host header still names the site.
func hostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if name, _, err := net.SplitHostPort(host); err == nil {
			host = name
		}
		switch strings.Trim(strings.ToLower(host), "[]") {
		case "127.0.0.1", "localhost", "::1":
			next.ServeHTTP(w, r)
		default:
			writeError(w, http.StatusForbidden, "host not allowed")
		}
	})
}

// maxRequestBody caps every request body. Cookie headers are the largest thing
// a client sends, and they stay well under this.
const maxRequestBody = 1 << 20

func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
		next.ServeHTTP(w, r)
	})
}

// securityHeaders hardens every response. The dashboard has no inline scripts,
// so scripts stay on 'self'. Its markup sets style attributes, so styles need
// 'unsafe-inline'. frame-ancestors keeps other sites from framing the dashboard
// for clickjacking while still allowing the Wails shell's fallback iframe; that
// iframe is also why there is no X-Frame-Options header.
func securityHeaders(port int, next http.Handler) http.Handler {
	csp := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data:",
		"font-src 'self'",
		fmt.Sprintf("connect-src 'self' ws://127.0.0.1:%d ws://localhost:%d", port, port),
		"object-src 'none'",
		"base-uri 'none'",
		"form-action 'self'",
		"frame-ancestors 'self' http://wails.localhost https://wails.localhost wails://wails",
	}, "; ")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", csp)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
