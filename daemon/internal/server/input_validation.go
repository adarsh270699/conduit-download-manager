package server

import (
	"errors"
	"path/filepath"
	"runtime"
	"strings"
)

// hasControlChars reports a character that could end a line. aria2 stores each
// option on its own line in aria2.session and joins headers with newlines, so a
// line break in a value would become a separate option or header.
func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// cleanFilename reduces a requested name to a base name that is safe to give
// aria2 on goos. An empty result means aria2 should choose the name.
func cleanFilename(raw, goos string) (string, error) {
	if hasControlChars(raw) {
		return "", errors.New("invalid filename: control characters not permitted")
	}
	base := filepath.Base(filepath.Clean(raw))
	if base == "." || base == "/" || base == "\\" || strings.Contains(raw, "..") {
		return "", errors.New("invalid filename: path traversal not permitted")
	}
	if goos == "windows" && !validWindowsName(base) {
		return "", errors.New("invalid filename: not a valid Windows file name")
	}
	if base == "download" {
		return "", nil
	}
	return base, nil
}

// validWindowsName rejects characters and device names that do not name an
// ordinary file. A colon would write an alternate data stream of another file.
func validWindowsName(name string) bool {
	if strings.ContainsAny(name, `<>:"|?*`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	stem, _, _ := strings.Cut(strings.ToUpper(name), ".")
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return false
	}
	return true
}

func validFilename(raw string) (string, error) {
	return cleanFilename(raw, runtime.GOOS)
}

// launchableExtensions run code when opened, so the dashboard asks first.
var launchableExtensions = map[string]bool{
	".appimage": true, ".appx": true, ".appxbundle": true, ".bat": true, ".cmd": true, ".com": true,
	".cpl": true, ".deb": true, ".desktop": true, ".exe": true, ".gadget": true, ".hta": true,
	".inf": true, ".jar": true, ".js": true, ".jse": true, ".lnk": true, ".msc": true, ".msi": true,
	".msix": true, ".msp": true, ".pif": true, ".ps1": true, ".py": true, ".reg": true, ".rpm": true,
	".run": true, ".scf": true, ".scr": true, ".sh": true, ".url": true, ".vb": true, ".vbe": true,
	".vbs": true, ".wsf": true, ".wsh": true,
}

func isLaunchable(path string) bool {
	return launchableExtensions[strings.ToLower(filepath.Ext(path))]
}
