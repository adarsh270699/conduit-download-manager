package aria

import (
	"os/exec"
	"testing"
)

func TestConfigureProcess(t *testing.T) {
	cmd := exec.Command("echo", "test")
	ConfigureProcess(cmd)

	if cmd.SysProcAttr == nil {
		t.Fatal("expected cmd.SysProcAttr to be configured")
	}
}
