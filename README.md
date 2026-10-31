# Conduit

Conduit is a cross-platform download accelerator for Windows and Linux. A local Go daemon drives the `aria2c` engine and serves a fully offline dashboard, while a Manifest V3 browser extension intercepts downloads from Chrome, Edge, and Firefox and hands them to the daemon.

[Architecture](docs/ARCHITECTURE.md) · [Product brief](docs/PRODUCT.md) · [Design system](docs/DESIGN.md) · [Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Third-party notices](THIRD_PARTY_NOTICES.md) | [Source offer](SOURCE_OFFER.md)

## Highlights

- **Multi-connection acceleration** through the `aria2c` engine, which Conduit runs as a separate program (up to 16 connections per server).
- **Browser interception** with final-URL resolution, cookies, referrer, user agent, and Content-Disposition filename forwarding.
- **Offline holding queue** in the extension. When the daemon is unavailable, downloads are paused and held locally, then forwarded automatically when it returns.
- **Native app on both platforms**: a Wails window with WebView2 on Windows and WebKitGTK on Ubuntu, plus system tray and per-user autostart.
- **One local API**: REST plus a WebSocket status stream on `127.0.0.1:18800`, documented below.
- **No cloud, accounts, telemetry, ads, or Electron.** All UI assets and fonts are embedded.
- **Pause, resume, retry, remove, reorder, open folder, change destination, delete file, speed limit, per-site exclusions.**

## Architecture

```text
Browser (Chrome / Edge / Firefox)
        │  download events + cookies
        ▼
Conduit extension ── REST + WebSocket ──► 127.0.0.1:18800
        │                                   │
        │ offline: hold queue               ▼
        └──────────────────────────► Go daemon (single binary)
                                            │
                              ┌─────────────┴──────────────┐
                              ▼                            ▼
                    Embedded dashboard             aria2c (separate program)
                    index / setup / guide          127.0.0.1:18801
```

The daemon owns configuration and process lifecycle. The desktop shell starts the daemon in-process and points a native window at the embedded dashboard. The extension is a thin client that survives daemon restarts.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for component responsibilities, storage layout, the API contract, the extension state machine, and security boundaries.

## Install

### Windows 10/11 (x64)

Run `dist/Conduit-windows-amd64-setup.exe` after building it with `make build-windows`.

- The per-user installer copies Conduit to `%LocalAppData%\Programs\Conduit` and does not require administrator rights.
- First launch asks for a download folder, then offers the browser extension guide.
- Uninstall from Windows Settings. Conduit stops its own engine and removes its runtime files; choose whether to keep settings and the download session. Downloaded files are never deleted. The shared WebView2 Runtime and the browser extension are left in place.

### Ubuntu 24.04 (x64)

```bash
sudo apt install ./dist/conduit_<version>_amd64.deb
```

`apt remove conduit` removes the packaged files but keeps `~/.config/conduit`. Turn off **Launch on System Startup** before removing the package, or delete `~/.config/autostart/conduit.desktop`.

### Headless daemon

`make build-daemon` produces `dist/conduit-linux-amd64` and `dist/conduit-windows-amd64.exe` without the native window.

```bash
./dist/conduit-linux-amd64
```

```cmd
dist\conduit-windows-amd64.exe
```

The dashboard remains available at <http://127.0.0.1:18800/>. On Linux the headless daemon exits on Ctrl+C/SIGTERM and stops its aria2 child process. The Windows headless build uses the GUI subsystem (no console window), so stop it with Task Manager or:

```cmd
curl -X POST http://127.0.0.1:18800/api/app/quit
```

| Flag | Default | Description |
| :--- | :--- | :--- |
| `--http-port` | `18800` | REST/WebSocket port on `127.0.0.1`. The extension expects the default. |
| `--rpc-port` | `18801` | Internal aria2 JSON-RPC port on `127.0.0.1`. |
| `--dir` | `~/Downloads` | Default download folder; persisted to the config file. |
| `--config-dir` | per-user config dir | Override the config, session, engine, and lock directory. |
| `--version` | — | Print the version and exit. |

## Browser extension

The app's **Install extension** action opens a local guide with the installed extension path. The guide covers both Chromium and Firefox.

### Chrome, Chromium, Brave, Edge

1. Open `chrome://extensions/` or `edge://extensions/`.
2. Enable **Developer mode**.
3. Choose **Load unpacked** and select the `extension` folder. The load persists across browser restarts.

### Firefox

1. Open `about:debugging#/runtime/this-firefox`.
2. Choose **Load Temporary Add-on…**.
3. Select `manifest.json` in the extension folder. Firefox removes temporary add-ons when it closes.

The extension uses `http://127.0.0.1:18800` and expects the daemon's default HTTP port.

## API

The daemon listens on `127.0.0.1:18800` only. Requests from browsers must carry an allowed `Origin`: a loopback dashboard origin, the Wails desktop asset origin, or a browser-extension origin (the Chrome ID derived from the pinned manifest key, or a Firefox `moz-extension://` origin). Unknown website origins are rejected with `403`; requests without an `Origin` header (curl, local tools) are allowed.

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/health` | `{app, status, version, engine}`; `engine` is `ready`, `starting`, or `offline`. |
| `POST` | `/api/app/show` | Loopback only. Shows the running desktop window. |
| `POST` | `/api/app/quit` | Loopback only. Requests shutdown. |
| `GET` | `/api/extension` | Loopback only. Returns `{path, exists}` for the extension folder. |
| `POST` | `/api/extension/open-guide` | Loopback only. Opens the extension guide in the system browser. |
| `GET` | `/api/downloads` | Lists active, waiting, paused, and stopped downloads. |
| `POST` | `/api/downloads` | Enqueues `{url, filename?, cookies?, referrer?, userAgent?}`. HTTP/HTTPS/FTP only. Returns `201 {gid}`. |
| `POST` | `/api/downloads/{gid}/action` | `pause`, `resume`, `retry`, `remove` (`deleteFile`), `open_folder`, `change_dir` (`newDir`), `move_up`, `move_down`. |
| `GET` | `/api/config` | Returns the current configuration. |
| `PUT` | `/api/config` | Persists and applies configuration to the live engine. |
| `GET` | `/api/ws` | WebSocket stream of the download list, sent on connect and every second. |
| `GET` | `/` | Embedded dashboard. `/setup` and `/extension` serve the wizard and guide. |

Example:

```bash
curl -s http://127.0.0.1:18800/api/health
curl -s -X POST http://127.0.0.1:18800/api/downloads \
  -H "Content-Type: application/json" \
  -d '{"url":"https://example.com/archive.zip","filename":"archive.zip"}'
```

### Configuration

| Field | Type | Notes |
| :--- | :--- | :--- |
| `downloadDir` | string | Default destination folder. |
| `maxConnections` | int | 1–16; default 16. |
| `maxConcurrentDownloads` | int | Default 5. |
| `autoStart` | bool | Mirrors the OS startup registration. |
| `maxDownloadLimit` | string | aria2-style limit; `"0"` disables it. |
| `setupComplete` | bool | `false` shows the first-run wizard. |

## Build from source

### Prerequisites

- **Go 1.27.2+**
- **Wails CLI v2.11.0** for the native window: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0`
- **Node.js 20+** for the extension tests
- **make**, **zip**, **unzip**, and **curl** for the Makefile targets (Git Bash or MSYS2 on Windows; Windows uses the PowerShell downloader); the quick build scripts do not need make or zip
- **Windows installer:** NSIS 3
- **Ubuntu 24.04 installer:** `gcc`, `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev`, and `nfpm`

The headless daemon cross-compiles with `CGO_ENABLED=0`. The native window requires the host toolchain for each platform.

### Engine binaries

Conduit does not ship aria2c. The Windows installer downloads the pinned aria2 1.37.0 release and checks its SHA-256. Linux packages depend on the `aria2` package. For a standalone build, place an `aria2c` binary where Conduit looks (see `docs/ARCHITECTURE.md`), or set `CONDUIT_ARIA2C` to its path.

The download URLs and SHA-256 values are pinned in `scripts/aria2_binaries.env`. To fetch the pinned build for tests, run `scripts/download_aria2_binaries.sh linux` (or `scripts/download_aria2_binaries.ps1` on Windows). It verifies the download and prints the path.

### Quick build scripts

These build the native app and extension zip in one step. Both scripts also create a standalone zip that includes the license bundle. The Windows script also builds the NSIS installer; Linux has no installer yet. The Windows script uses .NET to write Explorer-readable zip files (not `tar -a`); the Linux script uses `zip` or falls back to `python3`.

**Windows (PowerShell):**

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build_windows.ps1
```

Outputs:

- `dist\Conduit-windows-amd64-standalone.zip` (app + license bundle)
- `dist\Conduit-windows-amd64-standalone.exe`
- `dist\conduit-extension.zip`
- `dist\Conduit-windows-amd64-setup.exe`

**Linux:**

```bash
./scripts/build_linux.sh
# If the executable bit is missing: bash scripts/build_linux.sh
```

Outputs:

- `dist/Conduit-linux-amd64-standalone.zip` (app + license bundle)
- `dist/Conduit-linux-amd64-standalone`
- `dist/conduit-extension.zip`

No Linux installer is produced yet. Run the Linux script on Ubuntu 24.04 with the GTK 3, WebKitGTK 4.1, and `libayatana-appindicator3` development packages listed above. The Linux Wails build passes `-tags webkit2_41` because Wails defaults to WebKitGTK 4.0.

### Commands

```bash
# Headless daemons for Linux and Windows + extension zip
make all

# Windows native window and NSIS installer
make build-windows

# Ubuntu 24.04 native window and .deb
make build-linux

# Unit tests (Go + extension)
make test

# Unit and live integration tests
make test-all

# Format / static analysis / fast local gate
make fmt
make vet
make check

# Full Linux verification harness
make verify
```

Build outputs land in `dist/`:

| Artifact | Description |
| :--- | :--- |
| `Conduit-linux-amd64-standalone.zip` | Native Linux app plus the license bundle from `scripts/build_linux.sh`. |
| `Conduit-linux-amd64-standalone` | Native Linux app from `scripts/build_linux.sh`. |
| `Conduit-windows-amd64-standalone.zip` | Native Windows app plus the license bundle from `scripts/build_windows.ps1`. |
| `Conduit-windows-amd64-standalone.exe` | Native Windows app from `scripts/build_windows.ps1`. |
| `conduit-linux-amd64` | Headless Linux daemon from `make build-daemon`; license files and `licenses/` are copied to `dist/`. |
| `conduit-windows-amd64.exe` | Headless Windows daemon from `make build-daemon` (GUI subsystem, no console flash); license files and `licenses/` are copied to `dist/`. |
| `Conduit-windows-amd64-setup.exe` | Per-user NSIS installer from `scripts/build_windows.ps1` or `make build-windows`; includes the license bundle. |
| `conduit_<version>_amd64.deb` | Ubuntu 24.04 package from `make build-linux`, or from `scripts/build_linux.sh` when `nfpm` is installed; installs licenses under `/usr/share/doc/conduit`. |
| `conduit-extension.zip` | Manifest V3 extension package, including `LICENSE` and `NOTICE`. |

For release distribution, use the standalone zip or the installer; both include the license bundle. If you distribute a bare binary, include `LICENSE`, `NOTICE`, `THIRD_PARTY_NOTICES.md`, `SOURCE_OFFER.md`, and `licenses/` alongside it.

### Release

GitHub Actions publishes releases from a `vX.Y.Z` tag; nothing is uploaded by hand. The version lives only in `daemon/version/VERSION` and is changed with `cd daemon && go run ./tools/release bump X.Y.Z`. See [docs/RELEASING.md](docs/RELEASING.md) for versioning rules, release steps, release candidates, and hotfixes.

Each release carries the Windows installer and standalone zip, the Ubuntu `.deb` and standalone zip, the extension zip, `SHA256SUMS.txt`, and signed build provenance. Conduit's source is the GitHub source archive for the release tag; see `SOURCE_OFFER.md`.

To reproduce a release locally without publishing, run `scripts\prepare_release_windows.ps1` or `bash scripts/prepare_release_linux.sh`. Files land in `dist/release/`.


## Verification

Unit tests and builds need no engine. Integration tests need one: `make test-all` fetches a pinned test copy into `.cache/aria2/`, or set `CONDUIT_ARIA2C` to a binary you choose.

```bash
(cd daemon && go test ./...)
(cd daemon && go test -tags=integration ./...)
(cd desktop && go test ./...)
node extension/test/test_extension.js
```

On Ubuntu, `scripts/verify_cross_platform.sh` also builds both daemons, checks the Windows PE subsystem, verifies the extension archive, runs the Go tests with the race detector, and performs a live daemon smoke test. It requires `curl`, `unzip`, `fuser`, `zip`, and `python3`.

After uninstalling the Windows build, audit leftovers with:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File scripts\audit_windows_residue.ps1 -Expect keep
# or -Expect purge / -Expect clean
```

## Security

Conduit is a local application. The API has no remote authentication and is deliberately bound to loopback. Do not expose port 18800 through a firewall rule, reverse proxy, container port mapping, or public tunnel.

- Browser origins are checked before API handlers run, which blocks cross-site POSTs (CSRF), not just cross-origin response reads.
- Download URLs are validated server-side to HTTP, HTTPS, and FTP.
- The internal aria2 RPC port uses a per-launch random secret and verifies engine ownership before use.
- The extension merges cookies from the initiating and final URLs for redirect-based download gateways; cookies for the initiating host are therefore presented to the final host. Use per-site exclusions if that is not acceptable.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md#security-model) for the full threat model and [CONTRIBUTING.md](CONTRIBUTING.md) for development practices.

## Known limitations

- `retry` and `change_dir` re-enqueue using the URI list only; cookies, referrer, and user agent cannot be recovered from aria2 and are not restored.
- The extension is fixed to port 18800 and cannot follow a custom `--http-port`.
- BitTorrent, magnet links, and metalinks are out of scope for v1.
- A native Chromium "Save As" dialog can let the browser finish before Conduit pauses a download; in that case the browser copy is kept and not queued.
- Other Linux distributions and macOS are not supported.

## License

Conduit is free software: you can redistribute it and/or modify it under the terms of the GNU General Public License as published by the Free Software Foundation, either version 3 of the License, or (at your option) any later version. See [LICENSE](LICENSE) and [NOTICE](NOTICE).

`aria2c` is not part of Conduit. It is a separate program, licensed under GPL-2.0-or-later, that the installer or your system provides. The embedded IBM Plex Sans font is licensed under the SIL Open Font License 1.1. See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) and [SOURCE_OFFER.md](SOURCE_OFFER.md).
