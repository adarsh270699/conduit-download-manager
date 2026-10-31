package config

// BackgroundFlag is passed to Conduit when the OS launches it at login, so the
// app starts in the tray instead of opening its window.
const BackgroundFlag = "--background"

// AutostartManager defines the interface for OS startup registration.
type AutostartManager interface {
	IsEnabled() bool
	Enable() error
	Disable() error
}
