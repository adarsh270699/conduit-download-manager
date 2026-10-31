package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"conduit/daemon/internal/release"
)

const (
	// maxInstallerBytes caps one installer. Real installers are tens of
	// megabytes; the cap keeps a bad release from filling the disk.
	maxInstallerBytes = 1 << 30
	// maxSumsBytes caps SHA256SUMS.txt. A real one is a few hundred bytes.
	maxSumsBytes = 1 << 20
	// maxRedirects bounds the redirect chain GitHub's asset host uses.
	maxRedirects = 5
	sumsName     = "SHA256SUMS.txt"
)

var (
	// ErrNotAvailable means there is no newer release to download.
	ErrNotAvailable = errors.New("no newer release is available to download")
	// ErrBusy means another update step is already running.
	ErrBusy = errors.New("another update step is running")
)

// installerName returns the release asset this OS installs.
func installerName(goos string, v release.Version) (string, error) {
	switch goos {
	case "windows":
		return "Conduit-windows-amd64-setup.exe", nil
	case "linux":
		return fmt.Sprintf("conduit_%s_amd64.deb", v.String()), nil
	}
	return "", fmt.Errorf("no installer is published for %s", goos)
}

// checksumFor finds name in a sha256sum listing. It accepts "<hex>  <name>" and
// the binary-mode "<hex> *<name>", and tolerates CRLF line ends.
func checksumFor(listing, name string) (string, bool) {
	for _, line := range strings.Split(listing, "\n") {
		line = strings.TrimRight(line, "\r")
		sum, file, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		file = strings.TrimPrefix(strings.TrimLeft(file, " "), "*")
		if file != name {
			continue
		}
		sum = strings.ToLower(sum)
		if raw, err := hex.DecodeString(sum); err != nil || len(raw) != sha256.Size {
			continue
		}
		return sum, true
	}
	return "", false
}

// githubHostAllowed reports whether host is one GitHub serves release assets
// from. Downloads and their redirects must stay on these hosts.
func githubHostAllowed(host string) bool {
	return host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")
}

// downloadHostsEnv names extra hosts that downloads may use, as a comma-separated
// list. It lets a development build be tested against a feed on loopback. It is
// not a production setting.
const downloadHostsEnv = "CONDUIT_UPDATE_DOWNLOAD_HOSTS"

// parseDownloadHosts reads the override list. Names are lower-cased and empty
// entries are dropped.
func parseDownloadHosts(raw string) map[string]bool {
	hosts := map[string]bool{}
	for _, h := range strings.Split(raw, ",") {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			hosts[h] = true
		}
	}
	return hosts
}

// isLoopbackHost reports whether host is this machine.
func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// allowedURL reports whether u may be fetched. HTTPS is allowed to the hosts the
// service accepts. Plain HTTP is allowed only to a loopback host that the test
// override names.
func (s *Service) allowedURL(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	switch u.Scheme {
	case "https":
		return s.hostAllowed(host)
	case "http":
		return s.extraHosts[host] && isLoopbackHost(host)
	}
	return false
}

// checkRedirect follows a redirect only when it stays on HTTPS and on an
// allowed host.
func (s *Service) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("too many redirects")
	}
	if !s.allowedURL(req.URL) {
		return fmt.Errorf("refusing redirect to %s", req.URL.Redacted())
	}
	return nil
}

// get opens rawURL and returns a 200 response. The caller closes the body.
func (s *Service) get(ctx context.Context, rawURL string) (*http.Response, error) {
	u, err := url.Parse(rawURL)
	if err != nil || !s.allowedURL(u) {
		return nil, fmt.Errorf("refusing to download from %q: only HTTPS on GitHub is allowed", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	// The redirect rule lives on this client, not the shared one, so it covers
	// downloads without changing how the API is called.
	client := &http.Client{Transport: s.client.Transport, CheckRedirect: s.checkRedirect}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("contact the download server: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download server answered %s", resp.Status)
	}
	return resp, nil
}

// BeginDownload starts the installer download for the latest release. It
// checks the state and returns at once. The transfer runs in the background and
// reports through Status, because the server's write timeout is shorter than a
// download.
func (s *Service) BeginDownload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.busyLocked() {
		return ErrBusy
	}
	if s.status.State != StateAvailable || s.latest == nil {
		return ErrNotAvailable
	}
	rel, ver := s.latest, s.latestV
	s.status.State = StateDownloading
	s.status.Error = ""
	s.status.Progress = Progress{}
	go s.download(context.Background(), rel, ver)
	return nil
}

// download fetches, verifies, and records the installer for ver. On failure it
// removes what it wrote and sets error. The last good version stays in place.
func (s *Service) download(ctx context.Context, rel *githubRelease, ver release.Version) error {
	final, sum, err := s.fetchInstaller(ctx, rel, ver)

	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		s.status.State = StateError
		s.status.Error = err.Error()
		s.status.Progress = Progress{}
		return err
	}
	s.rec.DownloadedVersion = ver.String()
	s.rec.DownloadedPath = final
	s.rec.DownloadedSHA256 = sum
	s.status.State = StateDownloaded
	s.status.Error = ""
	s.saveLocked()
	return nil
}

// fetchInstaller downloads the installer for ver and checks it against
// SHA256SUMS.txt from the same release. It returns the final path and the
// verified SHA-256.
func (s *Service) fetchInstaller(ctx context.Context, rel *githubRelease, ver release.Version) (string, string, error) {
	name, err := installerName(runtime.GOOS, ver)
	if err != nil {
		return "", "", err
	}
	asset, ok := findAsset(rel.Assets, name)
	if !ok {
		return "", "", fmt.Errorf("release %s has no %s", rel.TagName, name)
	}
	sums, ok := findAsset(rel.Assets, sumsName)
	if !ok {
		return "", "", fmt.Errorf("release %s has no %s", rel.TagName, sumsName)
	}
	if asset.Size <= 0 {
		return "", "", fmt.Errorf("release %s reports no size for %s", rel.TagName, name)
	}
	if asset.Size > maxInstallerBytes {
		return "", "", fmt.Errorf("%s is %d bytes, over the 1 GiB limit", name, asset.Size)
	}

	want, err := s.fetchChecksum(ctx, sums, name)
	if err != nil {
		return "", "", err
	}

	dir := filepath.Join(s.configDir, updatesDirName, ver.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("create %s: %w", dir, err)
	}
	final := filepath.Join(dir, name)
	part := final + ".part"

	report := func(done, total int64) {
		s.mu.Lock()
		s.status.Progress = Progress{Done: done, Total: total}
		s.mu.Unlock()
	}
	got, err := s.streamAsset(ctx, asset, part, report)
	if err != nil {
		_ = os.Remove(part)
		return "", "", err
	}
	if got != want {
		_ = os.Remove(part)
		return "", "", fmt.Errorf("%s does not match its entry in %s", name, sumsName)
	}
	if err := os.Rename(part, final); err != nil {
		_ = os.Remove(part)
		return "", "", fmt.Errorf("move installer into place: %w", err)
	}
	return final, got, nil
}

// fetchChecksum reads SHA256SUMS.txt and returns the hash listed for name.
func (s *Service) fetchChecksum(ctx context.Context, asset githubAsset, name string) (string, error) {
	resp, err := s.get(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", sumsName, err)
	}
	defer resp.Body.Close()
	listing, err := io.ReadAll(io.LimitReader(resp.Body, maxSumsBytes))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", sumsName, err)
	}
	sum, ok := checksumFor(string(listing), name)
	if !ok {
		return "", fmt.Errorf("%s has no entry for %s", sumsName, name)
	}
	return sum, nil
}

// streamAsset writes the asset to dest, hashing as it goes. The body must match
// the size the release reports. The returned hash is the SHA-256 of what was
// written.
func (s *Service) streamAsset(ctx context.Context, asset githubAsset, dest string, report func(done, total int64)) (string, error) {
	resp, err := s.get(ctx, asset.BrowserDownloadURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.ContentLength >= 0 && resp.ContentLength != asset.Size {
		return "", fmt.Errorf("%s is %d bytes; the release says %d", asset.Name, resp.ContentLength, asset.Size)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	pw := &progressWriter{w: io.MultiWriter(f, hash), total: asset.Size, report: report}
	// Read one byte past the declared size, so a longer body is caught.
	n, copyErr := io.Copy(pw, io.LimitReader(resp.Body, asset.Size+1))
	closeErr := f.Close()
	if copyErr != nil {
		return "", fmt.Errorf("download %s: %w", asset.Name, copyErr)
	}
	if closeErr != nil {
		return "", closeErr
	}
	if n != asset.Size {
		return "", fmt.Errorf("%s is %d bytes; the release says %d", asset.Name, n, asset.Size)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// progressWriter counts bytes as they are written and reports them.
type progressWriter struct {
	w      io.Writer
	done   int64
	total  int64
	report func(done, total int64)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	p.report(p.done, p.total)
	return n, err
}
