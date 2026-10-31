package platform

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// CommandRunner defines the function signature for launching system processes.
type CommandRunner func(name string, args ...string) error

func defaultRunner(name string, args ...string) error {
	return StartDetached(name, args)
}

// startAndReap starts a short-lived helper process and waits for it in the
// background. Without the Wait call the child becomes a zombie on Unix for the
// lifetime of the daemon.
func startAndReap(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

var runner = defaultRunner

// OpenFolder opens the native host OS file manager pointing to targetPath.
func OpenFolder(targetPath string) error {
	if targetPath == "" {
		return errors.New("target path cannot be empty")
	}

	cmdName, args, err := ResolveOpenCommand(runtime.GOOS, targetPath)
	if err != nil {
		return err
	}

	return runner(cmdName, args...)
}

// ResolveOpenCommand determines the appropriate system command and arguments per OS.
func ResolveOpenCommand(goos, targetPath string) (string, []string, error) {
	targetPath = normalizeOpenPath(goos, targetPath)
	fi, statErr := os.Stat(targetPath)

	if goos == "windows" {
		if statErr == nil && !fi.IsDir() {
			// The Windows runner must not pass this through os/exec.
			// explorer.exe drops an unquoted /select path and opens Home.
			return "explorer.exe", []string{"/select," + targetPath}, nil
		}
		dir := targetPath
		if statErr != nil {
			dir = filepath.Dir(targetPath)
		}
		return "explorer.exe", []string{dir}, nil
	}

	// Linux / Unix default (xdg-open)
	dir := targetPath
	if statErr == nil {
		if !fi.IsDir() {
			dir = filepath.Dir(targetPath)
		}
	} else {
		dir = filepath.Dir(targetPath)
	}

	if goos == "darwin" {
		if statErr == nil && !fi.IsDir() {
			return "open", []string{"-R", targetPath}, nil
		}
		return "open", []string{dir}, nil
	}

	return "xdg-open", []string{dir}, nil
}

// OpenFile opens a file with the application the OS associates with it.
func OpenFile(targetPath string) error {
	if targetPath == "" {
		return errors.New("target path cannot be empty")
	}

	cmdName, args := ResolveOpenFileCommand(runtime.GOOS, targetPath)
	return runner(cmdName, args...)
}

// ResolveOpenFileCommand determines the command that launches a file per OS.
func ResolveOpenFileCommand(goos, targetPath string) (string, []string) {
	switch goos {
	case "windows":
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", normalizeOpenPath(goos, targetPath)}
	case "darwin":
		return "open", []string{targetPath}
	}
	return "xdg-open", []string{targetPath}
}

// OpenURL opens the system default web browser pointing to targetURL.
func OpenURL(targetURL string) error {
	if targetURL == "" {
		return errors.New("target URL cannot be empty")
	}

	cmdName, args, err := ResolveOpenURLCommand(runtime.GOOS, targetURL)
	if err != nil {
		return err
	}

	return runner(cmdName, args...)
}

// ResolveOpenURLCommand determines the appropriate command to open a URL per OS.
func ResolveOpenURLCommand(goos, targetURL string) (string, []string, error) {
	if goos == "windows" {
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", targetURL}, nil
	}
	if goos == "darwin" {
		return "open", []string{targetURL}, nil
	}
	return "xdg-open", []string{targetURL}, nil
}
