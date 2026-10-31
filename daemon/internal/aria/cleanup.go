package aria

import (
	"errors"
	"os"
	"path/filepath"
)

// StopManagedEngines stops the engine recorded for this installation and any
// aria2c running from a folder Conduit owns. It is safe to call when no engine
// is running.
func StopManagedEngines(configDir string) error {
	supervisor := &Supervisor{cfg: SupervisorConfig{ConfigDir: configDir}}
	return supervisor.stopOwnedEngines(managedEnginePaths(executableDir(), configDir))
}

// RemoveManagedEngineFiles deletes the engine ownership record and its temp
// copy. It leaves bin/ alone: the folder can hold an aria2c the user placed by
// hand, and purge removes it together with the config directory. It never
// deletes config.json or aria2.session.
func RemoveManagedEngineFiles(configDir string) error {
	var errs []error
	for _, name := range []string{"engine.json", "engine.json.tmp"} {
		path := filepath.Join(configDir, name)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
