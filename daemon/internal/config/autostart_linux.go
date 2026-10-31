//go:build !windows

package config

import (
	"os"

	"github.com/emersion/go-autostart"
)

// AppAutostart implements AutostartManager on Linux/Unix using XDG autostart desktop entries.
type AppAutostart struct {
	app *autostart.App
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

	app := &autostart.App{
		Name:        "conduit",
		DisplayName: "Conduit Download Accelerator",
		Exec:        []string{execPath, BackgroundFlag},
	}

	return &AppAutostart{app: app}, nil
}

// IsEnabled checks if autostart is registered.
func (a *AppAutostart) IsEnabled() bool {
	if a == nil || a.app == nil {
		return false
	}
	return a.app.IsEnabled()
}

// Enable registers Conduit to start on user login.
func (a *AppAutostart) Enable() error {
	if a == nil || a.app == nil {
		return nil
	}
	return a.app.Enable()
}

// Disable removes Conduit from user login startup.
func (a *AppAutostart) Disable() error {
	if a == nil || a.app == nil {
		return nil
	}
	return a.app.Disable()
}
