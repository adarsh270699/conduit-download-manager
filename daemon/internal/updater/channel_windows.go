//go:build windows

package updater

import "golang.org/x/sys/windows/registry"

// windowsInstallLocation reads the InstallLocation that the NSIS installer
// writes under the per-user uninstall key.
func windowsInstallLocation() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\Conduit`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer key.Close()
	location, _, err := key.GetStringValue("InstallLocation")
	return location, err
}
