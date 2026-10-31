//go:build linux

package aria

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// configureProcess sets Linux-specific process attributes.
// Pdeathsig ensures the child process is terminated when the parent dies.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGTERM,
	}
}

// assignProcessToJob has no Linux equivalent. Pdeathsig and aria2's
// --stop-with-process provide the lifecycle binding on this platform.
func assignProcessToJob(*exec.Cmd) (func(), error) {
	return func() {}, nil
}

func processIdentity(pid int) (uint64, string, error) {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	data, err := os.ReadFile(statPath)
	if err != nil {
		return 0, "", err
	}

	// The comm field is parenthesized and may itself contain spaces or ')'.
	closeParen := strings.LastIndex(string(data), ")")
	if closeParen < 0 {
		return 0, "", errors.New("malformed process stat")
	}
	fields := strings.Fields(string(data)[closeParen+1:])
	if len(fields) < 20 {
		return 0, "", errors.New("process stat is missing start time")
	}
	startTime, err := strconv.ParseUint(fields[19], 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("parse process start time: %w", err)
	}

	exe, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "exe"))
	if err != nil {
		return 0, "", err
	}
	exe = strings.TrimSuffix(exe, " (deleted)")
	return startTime, filepath.Clean(exe), nil
}

func processesWithExe(target string) ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}

	self := os.Getpid()
	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid == self {
			continue
		}
		if _, exe, err := processIdentity(pid); err == nil && pathsEqual(exe, target) {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func terminateProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Kill()
}

func processAlive(pid int) bool {
	_, _, err := processIdentity(pid)
	return err == nil
}

func pathsEqual(a, b string) bool {
	if abs, err := filepath.Abs(a); err == nil {
		a = abs
	}
	if abs, err := filepath.Abs(b); err == nil {
		b = abs
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

func currentBootID() (string, error) {
	data, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}
