package platform

import (
	"runtime"
	"testing"
)

// StartDetached is what the updater uses to launch the installer after Conduit
// quits, so it must start a real program without waiting for it.
func TestStartDetachedStartsProgram(t *testing.T) {
	name, args := "true", []string{}
	if runtime.GOOS == "windows" {
		name, args = "cmd.exe", []string{"/c", "exit", "0"}
	}
	if err := StartDetached(name, args); err != nil {
		t.Fatalf("StartDetached(%s): %v", name, err)
	}
}

func TestStartDetachedReportsMissingProgram(t *testing.T) {
	if err := StartDetached("conduit-no-such-program-for-test", nil); err == nil {
		t.Fatal("a program that does not exist must be an error")
	}
}
