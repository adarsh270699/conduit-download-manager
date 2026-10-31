package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
)

func TestCheckHealthRecognizesConduit(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		running   bool
		isConduit bool
	}{
		{
			name:      "current",
			body:      `{"app":"conduit","status":"ok","version":"1.0.0","engine":"ready"}`,
			running:   true,
			isConduit: true,
		},
		{
			name:      "no identity",
			body:      `{"engine":"ready","status":"ok","version":"1.0.0"}`,
			running:   true,
			isConduit: false,
		},
		{
			name:      "foreign",
			body:      `{"status":"ok"}`,
			running:   true,
			isConduit: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()

			running, isConduit := checkHealth(testServerPort(t, server.URL))
			if running != tc.running || isConduit != tc.isConduit {
				t.Fatalf("got running=%v isConduit=%v, want running=%v isConduit=%v", running, isConduit, tc.running, tc.isConduit)
			}
		})
	}
}

func TestCheckHealthReturnsNotRunningForClosedPort(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()

	running, isConduit := checkHealth(port)
	if running || isConduit {
		t.Fatalf("closed port reported running=%v isConduit=%v", running, isConduit)
	}
}

func testServerPort(t *testing.T, rawURL string) int {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse test server URL %q: %v", rawURL, err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("unexpected port in %q", rawURL)
	}
	return port
}
