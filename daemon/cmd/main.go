package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"conduit/daemon/internal/aria"
	"conduit/daemon/internal/boot"
	"conduit/daemon/internal/server"
	"conduit/daemon/version"
)

func main() {
	httpPort := flag.Int("http-port", server.DefaultServerPort, "HTTP REST API port (default 18800)")
	rpcPort := flag.Int("rpc-port", aria.DefaultRPCPort, "Internal RPC port for aria2 (default 18801)")
	downloadDir := flag.String("dir", "", "Default directory to store downloaded files")
	configDir := flag.String("config-dir", "", "Directory to store configuration and runtime state")
	showVersion := flag.Bool("version", false, "Print Conduit version and exit")
	// Accepted so a login entry written by the desktop app also starts the
	// headless daemon. The daemon never has a window, so it changes nothing.
	_ = flag.Bool("background", false, "Start without a window (always true for the headless daemon)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Conduit Daemon %s\n", version.String())
		os.Exit(0)
	}

	log.Printf("Starting Conduit Daemon %s...", version.String())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Check the HTTP port before acquiring the instance lock or sweeping a
	// previous engine. Otherwise a legacy build that does not use the lock
	// could lose its engine just because the new daemon could not bind HTTP.
	if httpPortInUse(*httpPort) {
		log.Printf("HTTP port %d is already in use; another Conduit instance may be running.", *httpPort)
		return
	}

	running, err := boot.Start(ctx, boot.Options{
		HTTPPort:    *httpPort,
		RPCPort:     *rpcPort,
		DownloadDir: *downloadDir,
		ConfigDir:   *configDir,
	})
	if errors.Is(err, boot.ErrAlreadyRunning) {
		log.Printf("Another Conduit instance is already running.")
		return
	}
	if err != nil {
		log.Fatalf("Failed to start Conduit: %v", err)
	}
	if running.EngineErr != nil {
		// Conduit keeps serving and retries the engine in the background, so a
		// missing aria2c shows up in /api/health instead of ending the process.
		log.Printf("Download engine is offline: %v", running.EngineErr)
	}

	running.Server.SetQuitHandler(cancel)

	log.Printf("Conduit engine fully initialized. Ready for connections. Press Ctrl+C to terminate.")

	<-ctx.Done()
	log.Printf("Shutdown signal received, shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = running.Shutdown(shutdownCtx)
	log.Printf("Conduit daemon stopped.")
}

func httpPortInUse(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
