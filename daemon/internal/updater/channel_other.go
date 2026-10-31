//go:build !windows

package updater

import "errors"

// windowsInstallLocation has no meaning off Windows, so the lookup always fails.
func windowsInstallLocation() (string, error) {
	return "", errors.New("the Windows installer does not exist on this system")
}
