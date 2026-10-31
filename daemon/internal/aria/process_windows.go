//go:build windows

package aria

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// CREATE_NO_WINDOW prevents creation of a new console window.
	createNoWindow = 0x08000000

	// waitTimeout is WAIT_TIMEOUT from the Win32 API.
	waitTimeout = 0x00000102
)

// configureProcess sets Windows-specific process attributes.
// HideWindow and CREATE_NO_WINDOW prevent terminal/console window flashing.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// assignProcessToJob ties the child's lifetime to this process. When Conduit
// exits or crashes, the job handle closes and Windows terminates aria2c.
func assignProcessToJob(cmd *exec.Cmd) (func(), error) {
	if cmd == nil || cmd.Process == nil {
		return nil, errors.New("cannot assign a process that has not started")
	}

	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create job object: %w", err)
	}

	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	info.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("configure job object: %w", err)
	}

	process, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION,
		false,
		uint32(cmd.Process.Pid),
	)
	if err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("open child process: %w", err)
	}
	defer windows.CloseHandle(process)

	if err := windows.AssignProcessToJobObject(job, process); err != nil {
		_ = windows.CloseHandle(job)
		return nil, fmt.Errorf("assign child to job: %w", err)
	}

	return func() {
		_ = windows.CloseHandle(job)
	}, nil
}

func processIdentity(pid int) (uint64, string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, "", err
	}
	defer windows.CloseHandle(handle)

	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, "", err
	}

	buf := make([]uint16, 32768)
	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(handle, 0, &buf[0], &size); err != nil {
		return 0, "", err
	}
	exe := windows.UTF16ToString(buf[:size])
	created := uint64(creation.HighDateTime)<<32 | uint64(creation.LowDateTime)
	return created, filepath.Clean(exe), nil
}

func processesWithExe(target string) ([]int, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("create process snapshot: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	if err := windows.Process32First(snapshot, &entry); err != nil {
		if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
			return nil, nil
		}
		return nil, fmt.Errorf("read first process: %w", err)
	}

	self := windows.GetCurrentProcessId()
	var pids []int
	for {
		if entry.ProcessID != 0 && entry.ProcessID != self {
			if _, exe, err := processIdentity(int(entry.ProcessID)); err == nil && pathsEqual(exe, target) {
				pids = append(pids, int(entry.ProcessID))
			}
		}
		if err := windows.Process32Next(snapshot, &entry); err != nil {
			if errors.Is(err, windows.ERROR_NO_MORE_FILES) {
				break
			}
			return nil, fmt.Errorf("read next process: %w", err)
		}
	}
	return pids, nil
}

func terminateProcess(pid int) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	return windows.TerminateProcess(handle, 1)
}

func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	return event == waitTimeout
}

func pathsEqual(a, b string) bool {
	if abs, err := filepath.Abs(a); err == nil {
		a = abs
	}
	if abs, err := filepath.Abs(b); err == nil {
		b = abs
	}
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func currentBootID() (string, error) {
	// Windows process creation times are absolute, so a boot ID is unnecessary.
	return "", nil
}
