// Package version reports the Conduit release version.
//
// The VERSION file next to this source is the single source of truth. The
// release tool (daemon/tools/release) bumps it, the build scripts read it for
// the installer and the .deb, and the release workflow checks that the git tag
// matches it. Never edit a version number anywhere else by hand.
package version

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var raw string

// Version is the release version without a leading v, for example "1.2.0" or
// "1.3.0-rc.1".
var Version = strings.TrimSpace(raw)

// Commit is the git commit the binary was built from. Release builds set it
// with -ldflags "-X conduit/daemon/version.Commit=<sha>"; local builds keep
// the default.
var Commit = "dev"

// String returns the version with the commit, for --version output.
func String() string {
	if Commit == "" || Commit == "dev" {
		return Version
	}
	short := Commit
	if len(short) > 12 {
		short = short[:12]
	}
	return Version + " (" + short + ")"
}
