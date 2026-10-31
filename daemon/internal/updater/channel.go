package updater

import (
	"os/exec"
	"path"
	"strings"
)

// Channel says how this copy of Conduit was installed. Install needs a
// supported channel; anything else is check-only.
type Channel string

const (
	// ChannelNone is check-only: install returns an error.
	ChannelNone Channel = ""
	// ChannelWindowsInstaller is the per-user NSIS install.
	ChannelWindowsInstaller Channel = "windows-installer"
	// ChannelDeb is the Debian package, which installs /usr/bin/conduit.
	ChannelDeb Channel = "deb"
)

// debBinary is where the .deb installs the daemon and window.
const debBinary = "/usr/bin/conduit"

// channelLookups are the OS queries detection needs. Tests replace them.
type channelLookups struct {
	windowsInstallLocation func() (string, error)
	debInstalled           func() bool
}

// osLookups returns the queries that ask the real system.
func osLookups() channelLookups {
	return channelLookups{
		windowsInstallLocation: windowsInstallLocation,
		debInstalled:           debInstalled,
	}
}

// detectChannel reports how the copy at exe was installed. A copy in any other
// place, such as a portable zip or a development build, is check-only.
func detectChannel(goos, exe string, l channelLookups) Channel {
	switch goos {
	case "windows":
		location, err := l.windowsInstallLocation()
		if err == nil && location != "" && sameDir(exe, location) {
			return ChannelWindowsInstaller
		}
	case "linux":
		if path.Clean(exe) == debBinary && l.debInstalled() {
			return ChannelDeb
		}
	}
	return ChannelNone
}

// sameDir reports whether the directory holding exe is dir. It accepts either
// separator and ignores case, so a Windows path compares the same on any host.
func sameDir(exe, dir string) bool {
	i := strings.LastIndexAny(exe, `\/`)
	if i < 0 {
		return false
	}
	return strings.EqualFold(strings.TrimRight(exe[:i], `\/`), strings.TrimRight(dir, `\/`))
}

// debInstalled reports whether dpkg has the conduit package installed.
func debInstalled() bool {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Status}", "conduit").Output()
	return err == nil && strings.Contains(string(out), "install ok installed")
}
