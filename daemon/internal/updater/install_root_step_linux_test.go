//go:build linux

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The root step installs a copy only when the copy matches the recorded hash.
// apt-get is replaced by a stub that logs its arguments, so no package changes.
func TestDebRootStepInstallsOnlyAVerifiedCopy(t *testing.T) {
	dir := t.TempDir()
	deb := filepath.Join(dir, "conduit_1.3.0_amd64.deb")
	payload := []byte("package bytes")
	if err := os.WriteFile(deb, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "apt.log")
	stub := "#!/bin/sh\necho \"$@\" >> " + logPath + "\n"
	if err := os.WriteFile(filepath.Join(bin, "apt-get"), []byte(stub), 0o755); err != nil {
		t.Fatal(err)
	}

	run := func(sum string) error {
		cmd := exec.Command("/bin/sh", "-c", debRootStep, "sh", deb, sum)
		cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"))
		return cmd.Run()
	}

	if err := run(sha256Hex(payload)); err != nil {
		t.Fatalf("matching hash: %v", err)
	}
	logged, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("apt-get was not called: %v", err)
	}
	if strings.Contains(string(logged), deb) {
		t.Fatalf("apt-get installed the user's file instead of a root-owned copy: %s", logged)
	}

	if err := os.Remove(logPath); err != nil {
		t.Fatal(err)
	}
	if err := run(strings.Repeat("0", 64)); err == nil {
		t.Fatal("a copy that fails the hash check was accepted")
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Fatal("apt-get ran after the hash check failed")
	}
}
