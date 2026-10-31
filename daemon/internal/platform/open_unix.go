//go:build !windows

package platform

import "os/exec"

func normalizeOpenPath(goos, targetPath string) string {
	return targetPath
}

func startDetached(name string, args []string) error {
	return startAndReap(exec.Command(name, args...))
}
