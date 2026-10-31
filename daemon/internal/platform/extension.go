package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// ExtensionCandidates lists places the unpacked extension may be installed.
func ExtensionCandidates() []string {
	var out []string
	if exe, err := os.Executable(); err == nil {
		out = append(out, filepath.Join(filepath.Dir(exe), "extension"))
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			out = append(out, filepath.Join(local, "Programs", "Conduit", "extension"))
		}
	} else {
		out = append(out, "/usr/share/conduit/extension")
	}
	if cwd, err := os.Getwd(); err == nil {
		out = append(out,
			filepath.Join(cwd, "extension"),
			filepath.Join(cwd, "..", "extension"),
			filepath.Join(cwd, "..", "..", "extension"),
			filepath.Join(cwd, "..", "..", "..", "extension"),
		)
	}
	return out
}

// FindExtensionDir returns the first candidate that contains manifest.json.
// The path is empty when none of the candidates exist.
func FindExtensionDir(candidates []string) (string, bool) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		info, err := os.Stat(filepath.Join(candidate, "manifest.json"))
		if err != nil || info.IsDir() {
			continue
		}
		abs, err := filepath.Abs(candidate)
		if err != nil {
			return candidate, true
		}
		return abs, true
	}
	return "", false
}
