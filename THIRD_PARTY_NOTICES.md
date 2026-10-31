# Third-Party Notices

Conduit is licensed under the **GNU General Public License, version 3 or later**
(GPL-3.0-or-later). See [LICENSE](LICENSE) and [NOTICE](NOTICE).

The distributed application also contains the third-party components listed
below. Full license texts are in [licenses/](licenses/). Source availability is
described in [SOURCE_OFFER.md](SOURCE_OFFER.md).

## aria2 (not bundled)

Conduit does not include aria2. Conduit runs `aria2c` as a separate program,
which is licensed under GPL-2.0-or-later by its authors:

- **Windows:** the installer downloads aria2 1.37.0 from
  <https://github.com/aria2/aria2/releases/tag/release-1.37.0> and checks
  `aria2c.exe` against its SHA-256 before installing it.
- **Linux:** the `.deb` package depends on the `aria2` package from your
  distribution.
- **Other installs:** you place an `aria2c` binary you trust where the README
  describes.

Upstream: <https://aria2.github.io/>. Source: <https://github.com/aria2/aria2>.

## IBM Plex Sans

The dashboard and setup pages embed `ibm-plex-sans-latin.woff2`:

- Upstream: <https://github.com/IBM/plex>
- License: SIL Open Font License, version 1.1
- License text: [licenses/OFL-1.1.txt](licenses/OFL-1.1.txt)

The font is embedded as a web asset and is not modified.

## Go and JavaScript dependencies

The Go modules linked into the desktop and daemon binaries are listed in
`desktop/go.sum` and `daemon/go.sum`. The exact linked versions and their
license texts are in [licenses/go/](licenses/go/). Linked dependencies include:

| Module | License |
| --- | --- |
| `fyne.io/systray` | Apache-2.0 |
| `github.com/godbus/dbus/v5` | BSD-2-Clause |
| `github.com/wailsapp/wails/v2` | MIT |
| `github.com/emersion/go-autostart` | MIT |
| `github.com/gorilla/websocket` | BSD-2-Clause |
| `golang.org/x/sys` | BSD-3-Clause |

The remaining linked modules and their license texts are in
[licenses/go/](licenses/go/). The browser extension has no third-party runtime
dependencies.

## Microsoft WebView2 Runtime

The Windows installer can bootstrap the Microsoft Edge WebView2 Runtime when it
is not already installed. The runtime is a Microsoft component governed by its
own license and is not bundled by Conduit.
