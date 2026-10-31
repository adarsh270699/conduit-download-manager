package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web/*
var webFS embed.FS

// getWebHandler returns an http.Handler that serves embedded dashboard assets with SPA fallback.
func getWebHandler() (http.Handler, error) {
	sub, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}

	fileServer := http.FileServer(http.FS(sub))

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		path := r.URL.Path
		switch path {
		case "/setup", "/setup/":
			r.URL.Path = "/setup.html"
			path = "/setup.html"
		case "/extension", "/extension/":
			r.URL.Path = "/extension.html"
			path = "/extension.html"
		}
		if path == "" || path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path, "/")
		if _, err := fs.Stat(sub, cleanPath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for non-API client routes
		if !strings.HasPrefix(path, "/api/") {
			r.URL.Path = "/"
			fileServer.ServeHTTP(w, r)
			return
		}

		http.NotFound(w, r)
	})

	return handler, nil
}
