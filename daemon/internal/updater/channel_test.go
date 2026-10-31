package updater

import (
	"errors"
	"testing"
)

const (
	winInstallDir = `C:\Users\u\AppData\Local\Programs\Conduit`
	winInstalled  = winInstallDir + `\Conduit.exe`
)

// lookups returns detection inputs with a fixed registry answer and dpkg answer.
func lookups(location string, locErr error, deb bool) channelLookups {
	return channelLookups{
		windowsInstallLocation: func() (string, error) { return location, locErr },
		debInstalled:           func() bool { return deb },
	}
}

func TestDetectChannelWindows(t *testing.T) {
	cases := []struct {
		name string
		exe  string
		loc  string
		err  error
		want Channel
	}{
		{name: "installed copy", exe: winInstalled, loc: winInstallDir, want: ChannelWindowsInstaller},
		{name: "case differs", exe: winInstalled, loc: `c:\users\u\appdata\local\programs\conduit`, want: ChannelWindowsInstaller},
		{name: "trailing separator", exe: winInstalled, loc: winInstallDir + `\`, want: ChannelWindowsInstaller},
		{name: "portable zip elsewhere", exe: `D:\Portable\Conduit.exe`, loc: winInstallDir, want: ChannelNone},
		{name: "registry key missing", exe: winInstalled, err: errors.New("not found"), want: ChannelNone},
		{name: "registry key empty", exe: winInstalled, loc: "", want: ChannelNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := detectChannel("windows", c.exe, lookups(c.loc, c.err, false))
			if got != c.want {
				t.Fatalf("detectChannel = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetectChannelLinux(t *testing.T) {
	cases := []struct {
		name string
		exe  string
		deb  bool
		want Channel
	}{
		{name: "packaged copy", exe: "/usr/bin/conduit", deb: true, want: ChannelDeb},
		{name: "binary copied elsewhere", exe: "/home/u/conduit", deb: true, want: ChannelNone},
		{name: "package not installed", exe: "/usr/bin/conduit", deb: false, want: ChannelNone},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := detectChannel("linux", c.exe, lookups("", nil, c.deb))
			if got != c.want {
				t.Fatalf("detectChannel = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDetectChannelOtherSystemsAreCheckOnly(t *testing.T) {
	if got := detectChannel("darwin", "/Applications/Conduit", lookups("", nil, true)); got != ChannelNone {
		t.Fatalf("darwin must be check-only, got %q", got)
	}
}
