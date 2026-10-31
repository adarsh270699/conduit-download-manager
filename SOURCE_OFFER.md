# Source Code Availability

Conduit is licensed under the GNU General Public License, version 3 or later
(GPL-3.0-or-later). The complete corresponding source for Conduit is the source
tree in this repository and the source archive published with each release.

GPLv3 section 6(d) permits conveying object code by offering equivalent access
to the Corresponding Source from the same place. The release process must
publish a source archive next to the binaries. GitHub releases provide
automatic source archives for the tagged commit; if a release is distributed
outside GitHub, publish an equivalent source archive alongside it.

## Release checklist

Attach the GitHub source archive for the release tag to the release page. That archive meets the requirement above, so no other source archive is needed. The release
files do not contain aria2
or any other separately licensed engine.

## aria2 is not part of Conduit

Conduit does not contain aria2. Conduit runs `aria2c` as a separate program,
which is licensed under GPL-2.0-or-later by its authors. How it reaches your
machine depends on the platform:

- **Windows:** the installer downloads the official aria2 1.37.0 release,
  `aria2-1.37.0-win-64bit-build1.zip`, from
  <https://github.com/aria2/aria2/releases/tag/release-1.37.0>, checks the
  SHA-256 of `aria2c.exe` against the pin in `scripts/aria2_binaries.env`, and
  installs it beside Conduit.
- **Linux:** the `.deb` package depends on the `aria2` package from your
  distribution, so apt installs that build.
- **Other installs:** you place an `aria2c` binary yourself. Its source and
  license are those of the upstream aria2 project.

The source for aria2 1.37.0 is at
<https://github.com/aria2/aria2/releases/tag/release-1.37.0>.

## Third-party libraries

- Go modules linked into the desktop and daemon binaries: `licenses/go/`
- IBM Plex Sans: `licenses/OFL-1.1.txt`
- Apache-2.0 components: `licenses/Apache-2.0.txt`

If a source archive is missing or a link is unavailable, open an issue in the
Conduit issue tracker and we will provide the corresponding source. [REVIEW REQUIRED] This commitment needs review by the maintainers or legal counsel.
