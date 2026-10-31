package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"conduit/daemon/internal/release"
)

const (
	repoSlug       = "adarsh270699/conduit-download-manager"
	defaultAPIBase = "https://api.github.com"
	maxAPIBody     = 1 << 20
)

// githubRelease is the part of GET /releases/latest the updater reads.
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset is one file attached to a release.
type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// latestResponse is the outcome of one GET /releases/latest request.
type latestResponse struct {
	release     *githubRelease
	etag        string
	notModified bool
}

// resolveAPIBase returns the configured base, or GitHub when none is set. The
// release build never reads the environment, so a feed can only be replaced
// through Options.
func resolveAPIBase(configured string) string {
	if configured != "" {
		return configured
	}
	return defaultAPIBase
}

// fetchLatest asks GitHub for the latest stable release. The endpoint skips
// prereleases and drafts on its own. A non-empty etag is sent as If-None-Match;
// a 304 means the release the caller cached is still the latest.
func fetchLatest(ctx context.Context, client *http.Client, apiBase, userAgent, etag string) (latestResponse, error) {
	url := strings.TrimRight(apiBase, "/") + "/repos/" + repoSlug + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return latestResponse{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}

	resp, err := client.Do(req)
	if err != nil {
		return latestResponse{}, fmt.Errorf("contact GitHub: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusNotModified:
		return latestResponse{notModified: true, etag: etag}, nil
	case http.StatusOK:
	default:
		return latestResponse{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}

	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxAPIBody)).Decode(&rel); err != nil {
		return latestResponse{}, fmt.Errorf("read the latest release: %w", err)
	}
	return latestResponse{release: &rel, etag: resp.Header.Get("ETag")}, nil
}

// parseTag reads a release tag such as v1.3.0 as a version.
func parseTag(tag string) (release.Version, error) {
	v, err := release.Parse(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return release.Version{}, fmt.Errorf("release tag %q: %w", tag, err)
	}
	return v, nil
}

// findAsset returns the asset with exactly this name.
func findAsset(assets []githubAsset, name string) (githubAsset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return githubAsset{}, false
}
