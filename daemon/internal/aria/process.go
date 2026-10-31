package aria

import "os/exec"

// ConfigureProcess applies platform-specific process attributes to cmd.
func ConfigureProcess(cmd *exec.Cmd) {
	configureProcess(cmd)
}
