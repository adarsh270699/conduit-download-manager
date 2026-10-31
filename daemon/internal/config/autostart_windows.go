//go:build windows

package config

import (
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// AppAutostart implements AutostartManager on Windows using the Registry (HKCU\...\Run).
type AppAutostart struct {
	name     string
	execPath string
}

// NewAppAutostart creates an autostart manager for the Conduit binary.
func NewAppAutostart(execPath string) (*AppAutostart, error) {
	if execPath == "" {
		var err error
		execPath, err = os.Executable()
		if err != nil {
			return nil, err
		}
	}
	return &AppAutostart{
		name:     "Conduit",
		execPath: execPath,
	}, nil
}

// IsEnabled checks if Conduit is in HKCU\...\Run.
func (a *AppAutostart) IsEnabled() bool {
	if a == nil {
		return false
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()

	val, _, err := k.GetStringValue(a.name)
	return err == nil && val != ""
}

// Enable adds Conduit to HKCU\...\Run.
func (a *AppAutostart) Enable() error {
	if a == nil {
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	return k.SetStringValue(a.name, `"`+a.execPath+`" `+BackgroundFlag)
}

// Disable removes Conduit from HKCU\...\Run.
func (a *AppAutostart) Disable() error {
	if a == nil {
		return nil
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	err = k.DeleteValue(a.name)
	if err == registry.ErrNotExist {
		return nil
	}
	return err
}
