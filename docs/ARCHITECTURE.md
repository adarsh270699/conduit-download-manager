# Architecture

This document describes how Conduit fits together at runtime and where to make changes. It replaces the historical phase plan. The user-facing overview lives in [README.md](../README.md); the visual language lives in [DESIGN.md](DESIGN.md).

## System at a Glance

Conduit has four cooperating pieces:

1. **Go daemon** (`daemon/`) — owns configuration, the `aria2c` engine it runs, the REST/WebSocket API, and the embedded dashboard.
2. **Wails desktop shell** (`desktop/`) — starts the daemon in-process, opens a native window pointed at the dashboard, and owns the tray icon.
3. **Manifest V3 extension** (`extension/`) — intercepts browser downloads, forwards session context to the daemon, and holds jobs while the daemon is unavailable.
4. **Packaging** (`packaging/`, `Makefile`) — builds the headless daemons, the native window, the NSIS installer, and the Ubuntu `.deb`.

```text
Browser (Chrome / Edge / Firefox)
        │  chrome.downloads events
        ▼
Conduit extension ── HTTP + WebSocket ──► 127.0.0.1:18800
        │                                   │
        │ offline: pause + queue            ▼
        └──────────────────────────► Go daemon (single binary)
                                            │
                              ┌─────────────┴──────────────┐
                              ▼                            ▼
                    Embedded dashboard             aria2c (separate program)
                    index/setup/extension          127.0.0.1:18801
```

## Repository Map

| Path | Responsibility |
| :--- | :--- |
| `daemon/cmd/main.go` | Headless daemon entry point, flags, signal handling, graceful shutdown. |
| `daemon/internal/boot/` | Startup orchestration: lock, config, supervisor, HTTP server, shutdown. |
| `daemon/internal/aria/` | Engine discovery, process supervision, JSON-RPC client, engine ownership state, and cleanup. Conduit ships no engine. |
| `daemon/internal/config/` | JSON settings, validation/clamping, autostart integration. |
| `daemon/internal/instance/` | Per-config-directory single-instance file lock. |
| `daemon/internal/server/` | REST handlers, CORS/origin policy, WebSocket hub, embedded web assets. |
| `daemon/internal/platform/` | Folder reveal, URL opening, extension directory discovery. |
| `daemon/internal/cleanup/` | Uninstall sweep with `keep` and `purge` modes. |
| `daemon/launch/` | Desktop-facing wrapper around `boot`, plus uninstall/shutdown helpers. |
| `desktop/` | Wails v2 shell, tray menu, native error dialogs, embedded initial frame. |
| `extension/` | MV3 service worker, popup, offline page, content-script toast, tests. |
| `packaging/` | `nfpm` Debian package, NSIS installer, desktop entry, WebView2 bootstrap. |
| `scripts/` | Quick platform build scripts, aria2 binary fetch, cross-platform verification, Windows residue audit, PE inspection. |
| `docs/` | Architecture, product brief, and design system. |

## Startup and Shutdown

### Headless daemon

`daemon/cmd/main.go` parses flags, checks whether the HTTP port is already owned, and calls `boot.Start`:

1. Resolve the config directory (`--config-dir`, otherwise `os.UserConfigDir()/conduit`).
2. Acquire `instance.lock`. A second process for the same directory receives `ErrAlreadyRunning`.
3. Remove stale `config.json.tmp` and `engine.json.tmp` files.
4. Load and clamp `config.json`; the first run creates defaults with `setupComplete: false`.
5. Create the supervisor and start it:
   - stop any engine recorded in `engine.json` or found running from the managed binary path;
   - find the `aria2c` binary, using the search order below (nothing is copied into the config directory);
   - create `aria2.session`;
   - spawn `aria2c` with RPC on `127.0.0.1:18801`, a fresh secret, the session file, and the configured connection limits;
   - wait until the RPC endpoint authenticates with that secret and reports the expected `dir`/`save-session`.
6. Start the HTTP server on `127.0.0.1:18800`.
7. Start the engine watcher. Every 2 seconds it checks that aria2 is running; if it exited (or never started) it restarts it with exponential backoff up to one minute and re-applies the saved settings. aria2's session file is saved every 30 seconds, so queued and paused work survives a restart.
8. On exit, stop the watcher, stop the HTTP server, ask aria2 to shut down, interrupt/kill it if needed, and release the lock.

The HTTP server stays up even when the engine fails to start. `/api/health` reports `engine: "starting"` until aria2's RPC answers and `"offline"` while it is down, and `"missing"` when no binary is found. Download requests return `503` in each of these states.

### Desktop app

`desktop/main.go` calls `launch.Start`, which delegates to `boot.Start`. It then opens a Wails window whose initial page embeds the local dashboard after the health endpoint responds. Closing the window hides it. Left-clicking the tray icon opens the window; right-clicking shows the Open / Install extension / Quit menu.

The login autostart entry runs `Conduit --background`. That starts the window hidden, so Conduit waits in the tray until the icon is clicked. If Conduit is already running, a `--background` launch exits quietly, while a normal launch brings the existing window forward. `--shutdown` and `--uninstall-cleanup=keep|purge` are hidden maintenance commands used by the installer.

## Storage Layout

| Data | Windows | Linux |
| :--- | :--- | :--- |
| Config, session, lock, engine state, extracted engine | `%AppData%\conduit\` | `~/.config/conduit/` |
| WebView2 cache | `%LocalAppData%\conduit\webview\` | n/a |
| Default download folder | `%UserProfile%\Downloads` | `~/Downloads` |
| Autostart entry | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` value `Conduit` = `"<exe>" --background` | `~/.config/autostart/conduit.desktop` (`Exec=<exe> --background`) |
| Extension state | `chrome.storage.local` | `chrome.storage.local` |

The extension stores `heldQueue` (paused jobs and their session context), `bypassedIds` (downloads the user resumed in the browser), `interceptionEnabled`, and `excludedDomains`.

Files inside the config directory:

| File | Purpose |
| :--- | :--- |
| `config.json` | User settings. Written atomically through a temporary file. |
| `aria2.session` | aria2's paused/waiting session, preserved across restarts. |
| `engine.json` | PID, process creation time, boot ID, binary path, RPC port/secret, start time. Used to stop an engine that survived a crash. Mode `0600`. |
| `instance.lock` | OS file lock that guarantees one daemon per config directory. |
| `update.json` | Update check state: last check time, latest version, and the downloaded installer's path and SHA-256. Written atomically through a temporary file. The updater never writes `config.json` or `aria2.session`. |
| `updates/<version>/` | Installers downloaded by the updater. Keep-mode uninstall removes them; purge removes everything. |
| `bin/aria2c` or `bin/aria2c.exe` | Optional. An engine placed by hand. Conduit never writes here. |

## Configuration

`config.Config` is persisted as JSON and clamped on every load/update:

| Field | Type | Notes |
| :--- | :--- | :--- |
| `downloadDir` | string | Default destination. Empty updates keep the previous value. |
| `maxConnections` | int | Default 16. Values outside 1–16 are set to 16. |
| `maxConcurrentDownloads` | int | Default 5; no upper clamp. |
| `autoStart` | bool | Mirrors the actual OS registration state on load. An existing entry is rewritten on load so it keeps the current executable path and `--background`. |
| `maxDownloadLimit` | string | aria2-style limit: digits with an optional `K` or `M` (`"0"` = unlimited). Other values are rejected with `400`. |
| `setupComplete` | bool, omitted | `false` runs the first-launch wizard. An existing file without the key is treated as already set up. |
| `showOnStart` | bool | Default `true`. Shows the window when a download starts. |
| `showOnComplete` | bool | Default `true`. Shows the window when a download completes. |
| `showOnError` | bool | Default `true`. Shows the window when a download fails. |
| `autoCheckUpdates` | bool | Default `false`. Checks GitHub for a newer release about once a day and tells open dashboards when one is found. A failed check is retried after an hour. |
| `autoInstallUpdates` | bool | Default `false`. Downloads and installs a found release without asking. Implies `autoCheckUpdates`. Installs only from a copy that the Windows setup or the `.deb` installed, and waits while downloads are running. |
| `enginePath` | string, omitted | Optional. Must be an absolute path to `aria2c`. The search order is in [Build and Packaging](#build-and-packaging). |

`PUT /api/config` merges the fields in the body into the current settings (absent fields keep their value), persists them, applies them to the live engine with `aria2.changeGlobalOption`, updates the OS autostart registration when that flag changes, and pushes the result to every WebSocket client. A failure to apply an option to a running engine is logged; the setting is still saved for the next start.

## REST and WebSocket API

The server binds to `127.0.0.1` only. All API requests from a browser must carry an allowed `Origin`:

- the dashboard at a loopback origin (`http://127.0.0.1:*` or `http://localhost:*`);
- the Wails asset origin used by the desktop bootstrap page (`http://wails.localhost` or `wails://…`);
- the Chrome extension ID derived from the pinned `key` in `extension/manifest.json`;
- any `moz-extension://` origin, because Firefox assigns a per-profile UUID rather than the Gecko add-on ID (host permissions still gate which extensions can reach the daemon).

Unknown browser origins receive `403` before a handler runs. Requests without an `Origin` header (curl, local tooling, the desktop shell) are allowed; the API has no remote authentication and must not be exposed beyond loopback.

| Method | Path | Description |
| :--- | :--- | :--- |
| `GET` | `/api/health` | Returns `app`, `status`, `version`, and `engine` (`ready`, `starting`, `offline`, or `missing`). |
| `POST` | `/api/app/show` | Loopback only. Shows the open desktop window. |
| `POST` | `/api/app/quit` | Loopback only. Requests application shutdown. |
| `GET` | `/api/extension` | Loopback only. Returns `{path, exists}` for the installed extension folder. |
| `POST` | `/api/extension/open-guide` | Loopback only. Opens `/extension` in the system browser. |
| `GET` | `/api/downloads` | Lists active, waiting, paused, and stopped downloads. |
| `POST` | `/api/downloads` | Enqueues `{url, filename?, cookies?, referrer?, userAgent?}`. URL schemes are limited to HTTP, HTTPS, and FTP. Returns `201 {gid}`. |
| `POST` | `/api/downloads/{gid}/action` | Actions: `pause`, `resume`, `retry`, `remove` (`deleteFile: true` also deletes the file), `open_folder`, `open_file`, `change_dir` (`newDir` required), `move_up`, `move_down`. |
| `GET` | `/api/config` | Returns the current configuration. |
| `PUT` | `/api/config` | Merges, validates, persists, applies, and broadcasts configuration. |
| `GET` | `/api/update` | Loopback only. Returns `state` (`idle`, `checking`, `upToDate`, `available`, `downloading`, `downloaded`, `installing`, or `error`), `currentVersion`, `latestVersion`, `progress` (`done` and `total` bytes), `error`, `lastCheckedAt`, and `installable` (whether this copy can install an update). Returns `503` if the update service did not start. |
| `POST` | `/api/update/check` | Loopback only. Asks GitHub for the latest stable release and returns the status. While another update step runs, it returns that step's status without sending a request. |
| `POST` | `/api/update/download` | Loopback only. Starts downloading the installer for this OS in the background and returns `202`. Returns `409` unless the state is `available`. The installer must match its entry in `SHA256SUMS.txt` from the same release. |
| `POST` | `/api/update/install` | Loopback only. Re-verifies the downloaded installer, queues it, returns `202`, and quits Conduit. The installer starts after Conduit has released its files and lock. Returns `409` when nothing is downloaded, the install channel is not supported, or a download is active. |
| `GET` | `/api/ws` | WebSocket stream of `{type, …}` frames: `{"type":"config","config":{…}}` on connect and whenever settings change, and `{"type":"downloads","downloads":[…]}` on connect and every second, and `{"type":"update","update":{…}}` on connect and whenever the update status changes. |
| `GET` | `/` | Embedded dashboard. `/setup` and `/extension` are the wizard and guide pages. |

The update routes read GitHub's release feed. Two environment variables change where the feed is read, and both are for testing. `CONDUIT_UPDATE_API` replaces the API base. `CONDUIT_UPDATE_DOWNLOAD_HOSTS` adds comma-separated hosts that installers may be downloaded from; plain HTTP is allowed only for loopback hosts in that list. Only the headless development daemon (`daemon/cmd`) reads them. The desktop app and the installers never do, so an installed Conduit cannot be pointed at another feed. The daemon logs a warning when the second is set.

With `autoCheckUpdates` or `autoInstallUpdates` on, the daemon runs its own schedule, so the checks happen even with no dashboard open. The first check comes about a minute after start if one is due, then one check a day, or one an hour after a failed check. Every minute the schedule reads the two settings, so a change applies without a restart.

Download objects contain `gid`, `filename`, `status`, `totalLength`, `completedLength`, `downloadSpeed`, `dir`, `errorMessage`, and `connections`. Completed files smaller than 2 MB are inspected for an HTML landing/error page; such items are reported as `error` instead of `complete`.

## Extension State Machine

1. `chrome.downloads.onCreated` receives the download.
2. In Chromium, `onDeterminingFilename` supplies the resolved filename and final URL. Conduit returns `true`, then either dispatches to the daemon or lets Chrome create the browser download.
3. Settings (`interceptionEnabled`, `excludedDomains`, `bypassedIds`) are read from `chrome.storage.local`.
4. Non-HTTP(S/FTP), blob/data/filesystem URLs, downloads below 1 MB when the size is known, completed items, and historical items older than two minutes are ignored.
5. The extension collects cookies for the final and original URL, the referrer, the user agent, and the Content-Disposition filename.
6. **Daemon online:** `POST /api/downloads`. On success the browser download is cancelled and erased, and the dashboard owns the transfer.
7. **Daemon offline:** the browser download is paused and a job is written to `heldQueue`. The action badge shows the count, a notification explains the hold, and a 30-second alarm retries. On Chromium, Conduit must call `suggest()` before it can pause the browser download; if the browser finishes or the user cancels before the pause lands, the browser copy wins and the job is not queued.
8. **Recovery:** when `/api/health` succeeds, `flushHeldQueue` forwards each job, cancels the browser copy, and clears the queue. Failures stay queued for the next attempt.
9. **Fallback:** the popup can resume a held download in the browser, which records its ID in `bypassedIds` so it is not intercepted again.

The extension assumes the daemon's default port `18800`. If the daemon is started with `--http-port`, the extension cannot discover the new port; use the default port or change the constants in `extension/background.js` and the popup/offline pages.

## Platform Integration

### Windows

- `aria2c.exe` is spawned with `CREATE_NO_WINDOW` and `HideWindow`.
- The child is assigned to a Job Object with kill-on-close, so the engine dies with the daemon or desktop process.
- `explorer.exe` is replaced with `SHOpenFolderAndSelectItems` for reliable file selection; directory opening uses a quoted `CmdLine`.
- Autostart uses `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`.
- The NSIS installer requires no elevation, installs to `%LocalAppData%\Programs\Conduit`, bootstraps WebView2 when missing, and offers keep-or-purge settings cleanup on uninstall.

### Linux

- `aria2c` receives `Pdeathsig: SIGTERM` and `--stop-with-process`, so it is tied to the daemon.
- Autostart is an XDG desktop entry at `~/.config/autostart/conduit.desktop`.
- Folder reveal uses `xdg-open` on the containing directory.
- The `.deb` installs `/usr/bin/conduit`, a desktop entry, an icon, and the extension under `/usr/share/conduit/extension`. Package removal leaves the user config directory in place.
- The native window links WebKitGTK 4.1 and GTK 3; the headless daemon is pure Go with `CGO_ENABLED=0`.

## Security Model

- Both servers bind to loopback. The RPC port is authenticated with a per-launch random secret and verified against the expected session path and download directory before Conduit trusts it.
- The instance lock prevents two daemons from managing the same engine. `engine.json` ties shutdown to a PID, process creation time, executable path, and boot ID.
- Browser origins are checked before API handlers run. This blocks cross-site POSTs from ordinary websites (CSRF), not just cross-origin response reads.
- Download URLs are validated server-side to HTTP, HTTPS, or FTP.
- Filenames are reduced to a base name and traversal attempts are rejected.
- The API has no user authentication. Any local process can call it. Do not expose port 18800 to a network, container host, or reverse proxy.
- The extension merges cookies from the initiating and final URLs into one `Cookie` header so redirect-based download gateways keep working. That means cookies for the initiating host are also presented to the final host. Use the per-site exclusion list for domains where this is not acceptable.

## Build and Packaging

Prerequisites and exact commands are in [README.md](../README.md) and [CONTRIBUTING.md](../CONTRIBUTING.md). At a high level:

- `scripts/build_windows.ps1` builds the native app, extension zip, and NSIS installer.
- `scripts/build_linux.sh` builds the native app and extension zip. It builds the `.deb` only when `nfpm` is installed.
- `make all` cleans `dist/`, packages the extension, and cross-compiles Linux and Windows headless daemons with `CGO_ENABLED=0`.

Conduit ships no aria2c. The daemon looks for `aria2c` in this order: the `CONDUIT_ARIA2C` environment variable; `enginePath` in `config.json`; `engine\aria2c.exe` beside the Windows executable (written by the installer); `bin/aria2c` in the config directory (placed by hand); then the system `PATH` (the Linux `.deb` depends on `aria2`). The first two are explicit settings: if they point at a missing file, the engine is reported missing rather than falling back to another binary. Only the installer and config-directory locations count as Conduit-managed, so the process sweep never stops an aria2c that Conduit did not start from one of them. `scripts/download_aria2_binaries.ps1` fills `.cache/aria2/windows/` with the pinned build for Windows tests. Linux tests use the aria2 package. When no engine is found, `/api/health` reports `"engine": "missing"` with an `engineHint`, and the daemon keeps checking, so an engine placed by hand is picked up without a restart.
- `make build-windows` runs `scripts/build_windows.ps1`: `wails build`, the zips, then NSIS with the pinned aria2 URL and hash.
- `make build-linux` runs `wails build` on Ubuntu and then `nfpm`.
- `make test` runs Go unit tests and the extension suite.
- `make test-all` adds the Go integration tests.
- `make check` runs `go vet` plus the test suite.
- `make verify` runs the Linux end-to-end harness in `scripts/verify_cross_platform.sh`.

The embedded UI is compiled into the daemon with `//go:embed web/*`, so the binary has no runtime asset dependency. The Wails shell embeds only a small bootstrap page; the dashboard itself is served by the daemon.

## Testing

| Layer | Command | What it covers |
| :--- | :--- | :--- |
| Go unit | `cd daemon && go test ./...` | RPC client, supervisor arguments/lifecycle, config, lock, cleanup, server routes, CORS/origin policy, WebSocket, platform helpers. |
| Go integration | `cd daemon && CONDUIT_ARIA2C=<aria2c> go test -tags=integration ./...` | Real aria2c process, local HTTP download, REST/WebSocket end to end, leftover-engine sweep. |
| Extension | `node extension/test/test_extension.js` | Filter rules, cookie serialization, filename extraction, online/offline branches, Chrome/Firefox paths, redirects, badge states, queue pruning. |
| Linux harness | `make verify` | Cross-compilation, PE subsystem, extension zip, Go tests with race detector, live daemon smoke test. |

Unit tests and builds need no engine. Integration tests skip unless `CONDUIT_ARIA2C` names an aria2c binary; `make test-all` uses `CONDUIT_ARIA2C` when it is set. Otherwise it fetches the pinned copy on Windows and uses the aria2 package on Linux.

CI runs formatting, vet, race-enabled unit and integration tests, extension tests, and both daemon cross-builds on Ubuntu.

## Known Limitations

- `retry` and `change_dir` re-enqueue through aria2 with the original URIs and request headers (read back with `aria2.getOption`). Cookies are the ones captured when the download was added, so a session that has since expired still fails.
- The extension is pinned to port 18800. Custom `--http-port` values are not discovered.
- The headless daemon has no tray icon. The tray belongs to the Wails desktop app.
- A native "Save As" dialog can delay Chromium's pause long enough for the browser to win the download race. Conduit does not enqueue a job in that case.
- The Windows uninstaller leaves the shared WebView2 Runtime installed and cannot remove the browser extension.
- The `.deb` has no maintainer script to stop a running instance or remove the per-user autostart entry; `README.md` documents the manual step.
- v1 does not support BitTorrent, magnet links, metalinks, or multiple simultaneous daemons for one config directory.
- Updates on Linux need one password prompt from `pkexec`, so they are not silent. With `autoInstallUpdates` on, that prompt can appear without a click. Portable Windows copies and development builds can check and download, but install returns `409`.
- Automatic install restarts Conduit as soon as a downloaded update is ready and no download is running. Automatic checks notify only through an open dashboard; there is no OS notification.
- The Windows installer is unsigned, and `SHA256SUMS.txt` comes from the same release. The check catches corruption and tampering in transit, not a compromised release. `gh attestation verify` is the stronger check and is not run yet.
- An interrupted update download starts over; it does not resume. A transfer that sends no data for 60 seconds, or runs longer than six hours, is ended and reported as an error.
- Update checks use GitHub's unauthenticated API, which allows 60 requests an hour per IP address. Conditional requests keep repeat checks inside that limit.
- `go test` targets Windows and Linux. Other operating systems are not supported.
