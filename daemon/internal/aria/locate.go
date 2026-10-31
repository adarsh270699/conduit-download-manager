package aria

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// EngineEnvVar names an aria2c binary to run instead of the one Conduit would
// find on its own. Developers and tests use it; users should not need it.
const EngineEnvVar = "CONDUIT_ARIA2C"

// ErrEngineMissing means no usable aria2c binary is available. Conduit does not
// ship one, so the installer, the system package manager, or the user provides it.
var ErrEngineMissing = errors.New("aria2c was not found")

// EngineLocation is the aria2c binary chosen for this run.
type EngineLocation struct {
	Path string
	// Managed reports that the binary sits in a folder Conduit owns: the
	// installer's engine folder or the config bin folder. Only managed engines
	// are swept by process name. A binary found elsewhere, such as the
	// distribution's /usr/bin/aria2c, can be running for another program too.
	Managed bool
}

// ExecutableName returns the aria2c file name for this platform.
func ExecutableName() string {
	if runtime.GOOS == "windows" {
		return "aria2c.exe"
	}
	return "aria2c"
}

// Locate chooses the aria2c binary to run. The search order is:
//
//  1. the CONDUIT_ARIA2C environment variable
//  2. enginePath from config.json
//  3. engine/ beside the Conduit executable (written by the Windows installer)
//  4. bin/ in the config directory (manual placement)
//  5. the system PATH (for example /usr/bin/aria2c from apt)
//
// An explicit path from steps 1 or 2 that is not usable is an error rather than
// a silent fallback, so a typo can never start a different binary.
func Locate(configDir, enginePath string) (EngineLocation, error) {
	return locateEngine(engineSearch{
		envPath:    os.Getenv(EngineEnvVar),
		configPath: enginePath,
		exeDir:     executableDir(),
		configDir:  configDir,
	})
}

// engineSearch holds the inputs to Locate so tests can supply their own.
type engineSearch struct {
	envPath    string
	configPath string
	exeDir     string
	configDir  string
}

func locateEngine(in engineSearch) (EngineLocation, error) {
	if in.envPath != "" {
		if !isUsableEngine(in.envPath) {
			return EngineLocation{}, fmt.Errorf("%w: %s points to %q, which is not an executable file", ErrEngineMissing, EngineEnvVar, in.envPath)
		}
		return EngineLocation{Path: in.envPath}, nil
	}
	if in.configPath != "" {
		if !isUsableEngine(in.configPath) {
			return EngineLocation{}, fmt.Errorf("%w: enginePath in config.json points to %q, which is not an executable file", ErrEngineMissing, in.configPath)
		}
		return EngineLocation{Path: in.configPath}, nil
	}

	managed := managedEnginePaths(in.exeDir, in.configDir)
	for _, candidate := range managed {
		if isUsableEngine(candidate) {
			return EngineLocation{Path: candidate, Managed: true}, nil
		}
	}
	if found, err := exec.LookPath(ExecutableName()); err == nil && isUsableEngine(found) {
		return EngineLocation{Path: found}, nil
	}
	return EngineLocation{}, fmt.Errorf("%w: looked in %s and on PATH", ErrEngineMissing, strings.Join(managed, ", "))
}

// managedEnginePaths lists the engine locations Conduit owns, in search order.
// The process sweep only ever considers these paths.
func managedEnginePaths(exeDir, configDir string) []string {
	name := ExecutableName()
	var paths []string
	if exeDir != "" {
		paths = append(paths, filepath.Join(exeDir, "engine", name))
	}
	if configDir != "" {
		paths = append(paths, filepath.Join(configDir, "bin", name))
	}
	return paths
}

// executableDir returns the folder that holds the running Conduit executable,
// or "" when it cannot be determined.
func executableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Dir(exe)
}

// isUsableEngine reports whether path is a regular file that can be run.
func isUsableEngine(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return runtime.GOOS == "windows" || info.Mode().Perm()&0o111 != 0
}
