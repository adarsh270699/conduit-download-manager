package main

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"conduit/daemon/launch"
	"conduit/daemon/version"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed frontend/dist
var assetEmbed embed.FS

//go:embed icon.ico
var iconICO []byte

//go:embed icon.png
var iconPNG []byte

func main() {
	httpPort := flag.Int("http-port", launch.DefaultHTTPPort, "HTTP REST API port (default 18800)")
	rpcPort := flag.Int("rpc-port", launch.DefaultRPCPort, "Internal RPC port for aria2 (default 18801)")
	downloadDir := flag.String("dir", "", "Default directory to store downloaded files")
	configDir := flag.String("config-dir", "", "Directory to store configuration and runtime state")
	showVersion := flag.Bool("version", false, "Print Conduit version and exit")
	shutdown := flag.Bool("shutdown", false, "Stop a running Conduit instance and exit")
	uninstallCleanup := flag.String("uninstall-cleanup", "", "Internal uninstall helper: keep or purge")
	// The login autostart entry passes this so Conduit waits in the tray.
	background := flag.Bool("background", false, "Start in the system tray without opening the window")
	flag.Parse()

	if *showVersion {
		fmt.Println("Conduit " + version.String())
		os.Exit(0)
	}

	// Hidden uninstaller/upgrade commands must not open a window or start the
	// engine. The uninstaller calls these before deleting Conduit.exe.
	if *uninstallCleanup != "" {
		if err := launch.UninstallCleanup(*uninstallCleanup, *configDir); err != nil {
			fmt.Fprintln(os.Stderr, "Conduit cleanup failed:", err)
			os.Exit(1)
		}
		return
	}
	if *shutdown {
		if err := shutdownRunning(*httpPort, *configDir); err != nil {
			fmt.Fprintln(os.Stderr, "Conduit shutdown failed:", err)
			os.Exit(1)
		}
		return
	}

	if running, conduit := checkHealth(*httpPort); running {
		if conduit {
			// A login launch must not pop up a window that is already running
			// in the tray. A manual launch brings it forward.
			if !*background {
				showExisting(*httpPort)
			}
			return
		}
		showError("Conduit", fmt.Sprintf(
			"Port %d is already in use by another program.\n\nClose that program or start Conduit with a different --http-port.",
			*httpPort,
		))
		return
	}

	running, err := launch.Start(context.Background(), launch.Options{
		HTTPPort:    *httpPort,
		RPCPort:     *rpcPort,
		DownloadDir: *downloadDir,
		ConfigDir:   *configDir,
	})
	if errors.Is(err, launch.ErrAlreadyRunning) {
		// The first instance may still be bringing up its HTTP listener.
		if !*background {
			showExisting(*httpPort)
		}
		return
	}
	if err != nil {
		showError("Conduit", fmt.Sprintf("Failed to start Conduit:\n\n%v", err))
		return
	}
	if err := running.EngineError(); err != nil {
		log.Printf("Warning: download engine is offline: %v", err)
	}

	app := &App{daemon: running}

	// Until the window starts, Wails cannot run its shutdown, so a quit request
	// (the installer's --shutdown, or POST /api/app/quit) would do nothing and
	// leave the process running. That happens when the window is slow to
	// start or never starts, for example when WebView2 is broken. Stop the
	// daemon and exit directly instead. startup() replaces this with the
	// normal window quit. The goroutine lets the quit request finish before
	// the HTTP server shuts down.
	running.SetQuitHandler(func() {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = running.Shutdown(ctx)
			// The daemon has released its lock, so a queued installer can run now.
			_ = running.LaunchPendingInstall()
			os.Exit(0)
		}()
	})

	assets, err := fs.Sub(assetEmbed, "frontend/dist")
	if err != nil {
		log.Fatalf("Failed to load window assets: %v", err)
	}

	cleanupLegacyWebViewProfile()

	winOptions := &windows.Options{}
	if webviewDir := prepareWebViewDir(); webviewDir != "" {
		winOptions.WebviewUserDataPath = webviewDir
	}

	err = wails.Run(&options.App{
		Title:            "Conduit",
		Width:            1120,
		Height:           720,
		MinWidth:         960,
		MinHeight:        640,
		BackgroundColour: &options.RGBA{R: 0xF5, G: 0xEB, B: 0xDD, A: 255},
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		StartHidden:   *background,
		Windows:       winOptions,
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.beforeClose,
	})
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = running.Shutdown(shutdownCtx)
		cancel()
		showError("Conduit", fmt.Sprintf("Failed to open the Conduit window:\n\n%v", err))
	}

	// Window mode has shut the daemon down by the time wails.Run returns, on
	// either branch above. Start a queued installer, if there is one.
	_ = running.LaunchPendingInstall()
}

type healthInfo struct {
	App string `json:"app"`
}

func checkHealth(port int) (running bool, isConduit bool) {
	client := http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/api/health", port))
	if err != nil {
		return false, false
	}
	defer resp.Body.Close()

	var info healthInfo
	_ = json.NewDecoder(resp.Body).Decode(&info)
	return true, resp.StatusCode == http.StatusOK && info.App == "conduit"
}

func showExisting(port int) {
	client := http.Client{Timeout: 300 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%d/api/app/show", port)
	for i := 0; i < 100; i++ {
		resp, err := client.Post(url, "application/json", nil)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusNoContent {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func shutdownRunning(port int, configDir string) error {
	running, isConduit := checkHealth(port)
	if !running {
		return nil
	}
	if !isConduit {
		return fmt.Errorf("port %d is in use by another program", port)
	}

	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Post(fmt.Sprintf("http://127.0.0.1:%d/api/app/quit", port), "application/json", nil)
	if err != nil {
		return fmt.Errorf("request quit: %w", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("quit endpoint returned status %d", resp.StatusCode)
	}

	// The HTTP listener closes before the engine and lock are released. Wait
	// for the instance lock so the uninstaller never deletes files underneath
	// a still-running process.
	if err := launch.WaitForInstanceStop(configDir, 15*time.Second); err != nil {
		return fmt.Errorf("wait for Conduit to stop: %w", err)
	}
	return nil
}

func prepareWebViewDir() string {
	if runtime.GOOS != "windows" {
		return ""
	}
	cacheRoot, err := os.UserCacheDir()
	if err != nil {
		log.Printf("Warning: could not locate the user cache directory: %v", err)
		return ""
	}
	dir := filepath.Join(cacheRoot, "conduit", "webview")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("Warning: could not create the WebView data directory: %v", err)
		return ""
	}
	return dir
}

func cleanupLegacyWebViewProfile() {
	if runtime.GOOS != "windows" {
		return
	}
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return
	}
	legacy := filepath.Join(configRoot, "Conduit.exe")
	if _, err := os.Stat(legacy); err != nil {
		return
	}

	// Rename before deleting. If an old Conduit process still owns the
	// profile, the rename fails and we leave the directory untouched instead
	// of partially deleting cache files out from under it.
	quarantine := legacy + ".cleanup"
	_ = os.RemoveAll(quarantine)
	if err := os.Rename(legacy, quarantine); err != nil {
		log.Printf("Skipping legacy WebView profile cleanup; it is in use: %v", err)
		return
	}
	if err := os.RemoveAll(quarantine); err != nil {
		log.Printf("Warning: could not remove the quarantined legacy WebView profile %s: %v", quarantine, err)
	}
}
