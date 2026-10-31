//go:build windows

package platform

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

const (
	coinitApartmentThreaded = 0x2
	hrOK                    = 0
	hrFalse                 = 1
	rpcEChangedMode         = 0x80010106
)

var (
	ole32                          = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx             = ole32.NewProc("CoInitializeEx")
	procCoUninitialize             = ole32.NewProc("CoUninitialize")
	procCoTaskMemFree              = ole32.NewProc("CoTaskMemFree")
	shell32                        = syscall.NewLazyDLL("shell32.dll")
	procSHParseDisplayName         = shell32.NewProc("SHParseDisplayName")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")

	// Replaced in tests so a folder action does not open Explorer.
	selectItemFn = selectItemInFolder
	openDirFn    = openDirectory
)

func normalizeOpenPath(goos, targetPath string) string {
	if goos != "windows" {
		return targetPath
	}
	return nativePath(targetPath)
}

func nativePath(targetPath string) string {
	cleaned := filepath.Clean(targetPath)
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return cleaned
	}
	return abs
}

// StartDetached starts name with args and does not wait for it to exit.
func StartDetached(name string, args []string) error {
	if strings.EqualFold(name, "explorer.exe") && len(args) == 1 {
		if strings.HasPrefix(args[0], "/select,") {
			return selectItemFn(strings.TrimPrefix(args[0], "/select,"))
		}
		return openDirFn(args[0])
	}

	return startAndReap(exec.Command(name, args...))
}

func selectItemInFolder(targetPath string) error {
	targetPath = nativePath(targetPath)
	if err := shellSelect(targetPath); err != nil {
		return fmt.Errorf("select %s: %w", targetPath, err)
	}
	return nil
}

func openDirectory(targetPath string) error {
	targetPath = nativePath(targetPath)
	cmd := exec.Command("explorer.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: "explorer.exe " + quoteWindowsArg(targetPath),
	}
	return startAndReap(cmd)
}

func shellSelect(targetPath string) error {
	pathPtr, err := syscall.UTF16PtrFromString(targetPath)
	if err != nil {
		return err
	}

	// COM apartment state belongs to the OS thread, not the goroutine.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procCoInitializeEx.Call(0, coinitApartmentThreaded)
	switch hrCode(hr) {
	case hrOK, hrFalse:
		defer procCoUninitialize.Call()
	case rpcEChangedMode:
		// This thread already initialized COM in another mode.
	default:
		return fmt.Errorf("CoInitializeEx failed: 0x%08X", hrCode(hr))
	}

	var pidl uintptr
	hr, _, _ = procSHParseDisplayName.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(unsafe.Pointer(&pidl)),
		0,
		0,
	)
	if hrCode(hr) != hrOK || pidl == 0 {
		if pidl != 0 {
			procCoTaskMemFree.Call(pidl)
		}
		return fmt.Errorf("SHParseDisplayName failed: 0x%08X", hrCode(hr))
	}
	defer procCoTaskMemFree.Call(pidl)

	hr, _, _ = procSHOpenFolderAndSelectItems.Call(pidl, 0, 0, 0)
	if hrCode(hr) != hrOK {
		return fmt.Errorf("SHOpenFolderAndSelectItems failed: 0x%08X", hrCode(hr))
	}
	return nil
}

func quoteWindowsArg(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	slashes := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			slashes++
		case '"':
			for ; slashes > 0; slashes-- {
				b.WriteString(`\\`)
			}
			b.WriteString(`\"`)
		default:
			for ; slashes > 0; slashes-- {
				b.WriteByte('\\')
			}
			b.WriteByte(s[i])
		}
	}
	for ; slashes > 0; slashes-- {
		b.WriteString(`\\`)
	}
	b.WriteByte('"')
	return b.String()
}

func hrCode(hr uintptr) uint32 {
	return uint32(hr)
}
