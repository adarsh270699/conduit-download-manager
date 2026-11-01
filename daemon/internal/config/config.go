package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

const (
	DefaultMaxConnections         = 16
	DefaultMaxConcurrentDownloads = 5
)

// Colour schemes. "system" follows the operating system; the other two pin the
// dashboard, the extension popup and the offline page to one scheme.
const (
	ThemeSystem = "system"
	ThemeLight  = "light"
	ThemeDark   = "dark"
)

// Config represents persistent application settings.
// SetupComplete is nil when the key is absent. An existing file without the
// key is treated as already set up. A new file stores false so the first
// launch runs the wizard. True means the wizard finished.
type Config struct {
	DownloadDir            string `json:"downloadDir"`
	MaxConnections         int    `json:"maxConnections"`
	MaxConcurrentDownloads int    `json:"maxConcurrentDownloads"`
	AutoStart              bool   `json:"autoStart"`
	MaxDownloadLimit       string `json:"maxDownloadLimit,omitempty"`
	SetupComplete          *bool  `json:"setupComplete,omitempty"`
	ShowOnStart            bool   `json:"showOnStart"`
	ShowOnComplete         bool   `json:"showOnComplete"`
	ShowOnError            bool   `json:"showOnError"`
	// AutoCheckUpdates checks GitHub for a newer release about once a day and
	// tells open dashboards when one is found. Off by default: it is the only
	// setting that makes Conduit contact GitHub without a click.
	AutoCheckUpdates bool `json:"autoCheckUpdates"`
	// AutoInstallUpdates downloads and installs a newer release without asking.
	// It implies AutoCheckUpdates.
	AutoInstallUpdates bool `json:"autoInstallUpdates"`
	// EnginePath optionally names the aria2c binary to run. Empty finds it automatically.
	EnginePath string `json:"enginePath,omitempty"`
	// Theme is the colour scheme every surface uses: "system", "light" or "dark".
	// The dashboard, the extension popup and the offline page all read it.
	Theme string `json:"theme"`
}

// NeedsSetup reports whether the first-launch wizard must run.
func (c Config) NeedsSetup() bool {
	return c.SetupComplete != nil && !*c.SetupComplete
}

func boolPtr(v bool) *bool {
	return &v
}

// ErrInvalid marks a rejected settings change. The API answers 400 for it.
var ErrInvalid = errors.New("invalid configuration")

// limitPattern matches the aria2 speed syntax Conduit accepts: bytes, or a
// K/M suffix ("0" means unlimited).
var limitPattern = regexp.MustCompile(`^[0-9]+[KkMm]?$`)

// ValidateAndClamp ensures config values are within safe, allowed boundaries.
func (c *Config) ValidateAndClamp() {
	if c.MaxConnections <= 0 {
		c.MaxConnections = DefaultMaxConnections
	} else if c.MaxConnections > 16 {
		c.MaxConnections = 16
	}

	if c.MaxConcurrentDownloads <= 0 {
		c.MaxConcurrentDownloads = DefaultMaxConcurrentDownloads
	}

	c.MaxDownloadLimit = strings.TrimSpace(c.MaxDownloadLimit)
	if !limitPattern.MatchString(c.MaxDownloadLimit) {
		// aria2 rejects the whole option batch when one value is invalid, so a
		// bad limit must never reach it.
		c.MaxDownloadLimit = "0"
	}

	switch c.Theme {
	case ThemeLight, ThemeDark:
	default:
		c.Theme = ThemeSystem
	}
}

// DefaultConfig generates a default configuration object.
func DefaultConfig() (Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Config{}, fmt.Errorf("failed to get user home directory: %w", err)
	}

	return Config{
		DownloadDir:            filepath.Join(home, "Downloads"),
		MaxConnections:         DefaultMaxConnections,
		MaxConcurrentDownloads: DefaultMaxConcurrentDownloads,
		AutoStart:              false,
		MaxDownloadLimit:       "0",
		SetupComplete:          boolPtr(false),
		Theme:                  ThemeSystem,
		ShowOnStart:            true,
		ShowOnComplete:         true,
		ShowOnError:            true,
	}, nil
}

// Manager handles thread-safe configuration reading, updating, and disk persistence.
type Manager struct {
	mu        sync.RWMutex
	cfg       Config
	filePath  string
	autostart AutostartManager
}

// NewManager creates a Manager, loading from filePath or initializing with defaults.
func NewManager(filePath string, autostart AutostartManager) (*Manager, error) {
	if filePath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user config directory: %w", err)
		}
		filePath = filepath.Join(configDir, "conduit", "config.json")
	}

	m := &Manager{
		filePath:  filePath,
		autostart: autostart,
	}

	if err := m.loadOrInit(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) loadOrInit() error {
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := os.ReadFile(m.filePath)
	if os.IsNotExist(err) {
		defaultCfg, err := DefaultConfig()
		if err != nil {
			return err
		}
		m.cfg = defaultCfg
		return m.saveLocked()
	} else if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", m.filePath, err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}
	// A file written before the window pop-ups existed has no such keys. They
	// are opt-out, so an absent key means on.
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err == nil {
		for key, flag := range map[string]*bool{
			"showOnStart":    &loaded.ShowOnStart,
			"showOnComplete": &loaded.ShowOnComplete,
			"showOnError":    &loaded.ShowOnError,
		} {
			if _, ok := keys[key]; !ok {
				*flag = true
			}
		}
	}

	loaded.ValidateAndClamp()

	// The OS registration is the source of truth. Re-registering an existing
	// entry keeps its command current: it follows the app if it was moved and
	// upgrades entries written before Conduit started in the background.
	if m.autostart != nil {
		loaded.AutoStart = m.autostart.IsEnabled()
		if loaded.AutoStart {
			if err := m.autostart.Enable(); err != nil {
				log.Printf("Warning: could not refresh the autostart entry: %v", err)
			}
		}
	}

	m.cfg = loaded
	return nil
}

// Get returns a copy of the current configuration.
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := m.cfg
	if m.cfg.SetupComplete != nil {
		v := *m.cfg.SetupComplete
		cfg.SetupComplete = &v
	}
	return cfg
}

// Update replaces the settings. An empty DownloadDir or a nil SetupComplete
// keeps the current value.
func (m *Manager) Update(newCfg Config) (Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applyLocked(newCfg)
}

// Patch applies a JSON object on top of the current settings. Fields that are
// absent keep their value, so a client that only changes the speed limit
// cannot reset other settings it did not send.
func (m *Manager) Patch(data []byte) (Config, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	next := m.cfg
	if m.cfg.SetupComplete != nil {
		// Unmarshal writes through pointers. Give it its own copy.
		next.SetupComplete = boolPtr(*m.cfg.SetupComplete)
	}
	if err := json.Unmarshal(data, &next); err != nil {
		return m.cfg, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	return m.applyLocked(next)
}

func (m *Manager) applyLocked(newCfg Config) (Config, error) {
	if limit := strings.TrimSpace(newCfg.MaxDownloadLimit); limit != "" && !limitPattern.MatchString(limit) {
		return m.cfg, fmt.Errorf("%w: download limit %q must be a number of bytes, optionally followed by K or M", ErrInvalid, newCfg.MaxDownloadLimit)
	}
	// A save that does not name a theme keeps the current one.
	if newCfg.Theme == "" {
		newCfg.Theme = m.cfg.Theme
	}
	newCfg.EnginePath = strings.TrimSpace(newCfg.EnginePath)
	if newCfg.EnginePath != "" && !filepath.IsAbs(newCfg.EnginePath) {
		return m.cfg, fmt.Errorf("%w: enginePath %q must be an absolute path to aria2c", ErrInvalid, newCfg.EnginePath)
	}
	newCfg.ValidateAndClamp()
	if newCfg.DownloadDir == "" {
		newCfg.DownloadDir = m.cfg.DownloadDir
	}
	if newCfg.SetupComplete == nil {
		newCfg.SetupComplete = m.cfg.SetupComplete
	}

	if m.autostart != nil && newCfg.AutoStart != m.cfg.AutoStart {
		var err error
		if newCfg.AutoStart {
			err = m.autostart.Enable()
		} else {
			err = m.autostart.Disable()
		}
		if err != nil {
			return m.cfg, fmt.Errorf("failed to change autostart: %w", err)
		}
	}

	m.cfg = newCfg
	if err := m.saveLocked(); err != nil {
		return m.cfg, err
	}
	return m.cfg, nil
}

func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config JSON: %w", err)
	}

	tempPath := m.filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp config file: %w", err)
	}

	if err := os.Rename(tempPath, m.filePath); err != nil {
		return fmt.Errorf("failed to rename temp config file: %w", err)
	}

	return nil
}

// FilePath returns the file path where configuration is persisted.
func (m *Manager) FilePath() string {
	return m.filePath
}

// EngineOptions returns the aria2 global options that mirror these settings.
func (c Config) EngineOptions() map[string]string {
	opts := map[string]string{
		"dir":                       c.DownloadDir,
		"max-connection-per-server": strconv.Itoa(c.MaxConnections),
		"max-concurrent-downloads":  strconv.Itoa(c.MaxConcurrentDownloads),
	}
	if c.MaxDownloadLimit != "" {
		opts["max-overall-download-limit"] = c.MaxDownloadLimit
	}
	return opts
}
