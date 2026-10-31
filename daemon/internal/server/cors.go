package server

import (
	"net/http"
	"strings"
)

var allowedExtensionOrigins = loadAllowedExtensionOrigins()

// isOriginAllowed checks if the origin is this extension or local loopback.
func isOriginAllowed(origin string) bool {
	if origin == "" {
		return true
	}
	if _, ok := allowedExtensionOrigins[origin]; ok {
		return true
	}
	// Firefox assigns an extension a per-profile UUID, so the Gecko add-on ID
	// from manifest.json is not the request origin. Host permissions already
	// gate which extensions may reach the daemon.
	if strings.HasPrefix(origin, "moz-extension://") {
		return true
	}
	// Allow loopback origins across any port (e.g. customized -http-port).
	if strings.HasPrefix(origin, "http://127.0.0.1:") || origin == "http://127.0.0.1" ||
		strings.HasPrefix(origin, "http://localhost:") || origin == "http://localhost" {
		return true
	}
	// The Wails shell serves its bootstrap page from a virtual host before it
	// navigates to the daemon origin.
	if origin == "http://wails.localhost" || origin == "https://wails.localhost" ||
		strings.HasPrefix(origin, "http://wails.localhost:") ||
		strings.HasPrefix(origin, "https://wails.localhost:") ||
		strings.HasPrefix(origin, "wails://") {
		return true
	}
	return false
}

// CORSMiddleware applies CORS headers to API responses and rejects browser
// requests from unknown origins.
//
// Rejecting the request (not merely omitting the response header) matters:
// CORS does not stop a cross-site "simple" POST from reaching the handler.
// Without this check a website could enqueue downloads or quit the app by
// posting to 127.0.0.1 from the user's browser.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !isOriginAllowed(origin) {
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

		next.ServeHTTP(w, r)
	})
}
