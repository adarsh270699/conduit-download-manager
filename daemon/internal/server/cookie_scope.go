package server

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	cookieScopeTimeout = 5 * time.Second
	cookieScopeMaxHops = 10
)

// resolveCookieScope is replaced in tests so a request does not reach the network.
var resolveCookieScope = followSameHostRedirects

// followSameHostRedirects decides where cookies may go. aria2 repeats a custom
// Cookie header on every redirect, whatever the host, so the redirects are
// walked here first. While they stay on the first host the cookies stay on. If
// one leaves that host, or drops from HTTPS to HTTP, the download starts from
// that redirect's URL with no cookies. A failed check also drops them: aria2
// would most likely fail to connect too, and a leak is the worse outcome.
//
// aria2 still fetches the URL itself, so this cannot cover a server that answers
// aria2 differently from this check.
func followSameHostRedirects(ctx context.Context, rawURL string, headers []string) (string, bool) {
	ctx, cancel := context.WithTimeout(ctx, cookieScopeTimeout)
	defer cancel()

	first, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, false
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	current := first
	for hop := 0; hop < cookieScopeMaxHops; hop++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, current.String(), nil)
		if err != nil {
			return rawURL, false
		}
		for _, header := range headers {
			if name, value, ok := strings.Cut(header, ": "); ok {
				req.Header.Set(name, value)
			}
		}
		// One byte is enough to see the status, and most servers treat a
		// ranged request as the start of a segmented download.
		req.Header.Set("Range", "bytes=0-0")
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Warning: dropped cookies for a download: could not check its redirects: %v", withoutURL(err))
			return rawURL, false
		}
		_ = resp.Body.Close()
		location := resp.Header.Get("Location")
		if resp.StatusCode < 300 || resp.StatusCode >= 400 || location == "" {
			return rawURL, true
		}
		next, err := current.Parse(location)
		if err != nil || validateDownloadURL(next.String()) != nil {
			return rawURL, false
		}
		leavesHost := !strings.EqualFold(next.Hostname(), first.Hostname())
		downgrades := strings.EqualFold(first.Scheme, "https") && !strings.EqualFold(next.Scheme, "https")
		if leavesHost || downgrades {
			return next.String(), false
		}
		current = next
	}
	return rawURL, false
}

// withoutURL strips the URL from a client error, because it can hold a signed token.
func withoutURL(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return urlErr.Err
	}
	return err
}
