//go:build !windows

package platform

import "os/exec"

func normalizeOpenPath(goos, targetPath string) string {
	return targetPath
}

// StartDetached starts name with args and does not wait for it to exit.
func StartDetached(name string, args []string) error {
	return startAndReap(exec.Command(name, args...))
}
