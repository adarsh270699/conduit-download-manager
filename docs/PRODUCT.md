# Product

## Platform

Windows 10/11 (x64) and Ubuntu 24.04 (x64) desktop, with a shared Manifest V3 extension for Chrome, Chromium, Brave, Microsoft Edge, and Firefox.

## Users

People who download media, documents, installers, and large files from a browser and want faster, resumable transfers without a commercial download manager's ads, bundled software, cloud account, or telemetry. They expect the tool to stay out of the way until a download starts or fails.

## Product Purpose

Conduit accelerates browser and pasted-URL downloads using the `aria2c` engine. A local Go daemon owns the engine, exposes a loopback REST/WebSocket API, and serves an offline dashboard. A browser extension hands downloads to the daemon and holds them when it is unavailable. Success means the user can start a download from any browser and see it finish faster, from one quiet queue, without sending browsing data anywhere.

## Positioning

A single local agent that combines the `aria2c` engine, an embedded offline dashboard, and an offline-capable browser extension. Unlike commercial download managers, Conduit has no adware, no account, no telemetry, and no runtime dependency on Electron or a cloud service. Unlike a bare `aria2c` setup, it adds browser interception, a native window, OS autostart, and a queue the user can operate without a terminal.

## Operating Context

- **Operating systems:** Windows 10/11 x64 and Ubuntu 24.04 x64. Other Linux distributions may work but are not verified.
- **Browsers:** Chrome, Chromium, Brave, Microsoft Edge, and Firefox through one Manifest V3 codebase.
- **Protocols:** HTTP, HTTPS, and FTP. BitTorrent, magnet links, and metalinks are out of scope for v1.
- **Runtime:** The daemon binds to `127.0.0.1:18800` (REST and WebSocket) and controls an isolated `aria2c` process on `127.0.0.1:18801`. The browser extension expects the default HTTP port.
- **Storage:** Configuration, session state, and the instance lock live under the per-user config directory. Downloads go to a folder the user chooses during first-run setup.

## Capabilities and Constraints

- **Multi-connection acceleration:** Up to 16 connections per server, configurable concurrency, resumable transfers, and queue reordering.
- **Browser interception:** Downloads are resolved to their final URL, with cookies, referrer, user agent, and Content-Disposition filename forwarded to the daemon when possible.
- **Offline holding queue:** When the daemon is offline, intercepted downloads are paused and held in extension storage, then forwarded automatically when the daemon returns. A "Resume in browser" action remains available.
- **Per-site exclusions and a global interception toggle:** Users can keep selected domains or all downloads in the browser.
- **Native integration:** Per-user autostart on Windows and Linux, native folder reveal, a native window on both platforms, and opt-out window pop-ups when a download starts, finishes, or fails.
- **Local-only security model:** The API has no remote authentication because it is bound to loopback. It rejects unknown browser origins. It must not be exposed through a reverse proxy or port forward.
- **V1 constraints:** No BitTorrent, magnet links, metalinks, or multi-user/server mode. Custom daemon ports are not configurable from the extension UI.

## Brand Commitments

- **Name:** Conduit
- **Voice:** Direct, quiet, technical, and dependable.
- **Personality:** Industrial-grade reliability without clutter, nagging notifications, or telemetry.

## Evidence on Hand

- Go daemon source in `daemon/`.
- Wails v2 desktop shell in `desktop/`.
- Manifest V3 extension source in `extension/`, with an automated Node test suite.
- Packaging for a per-user NSIS installer and an Ubuntu `.deb` in `packaging/`.
- A local end-to-end verification harness in `scripts/`.

## Product Principles

1. **Silent, reliable acceleration:** Intercept transparently and accelerate aggressively. By default Conduit brings its window forward when a download starts, finishes, or fails, because users asked to see these moments. Each event is a Settings checkbox, and turning it off keeps Conduit in the tray.
2. **No cloud dependency:** Downloads, browsing context, settings, fonts, and the engine stay on the user's machine.
3. **Offline-first resilience:** The extension degrades to a held queue rather than losing a download when the daemon is not running.
4. **Lean footprint:** Native Go plus embedded web assets, not an Electron shell. A library may be added when it is vendored locally and served by the daemon, with no CDN load at runtime. Each one is recorded in DESIGN.md.
5. **User sovereignty:** The download folder, queue, connection limits, exclusions, and autostart state are the user's to change.

## Accessibility and Inclusion

- Primary text meets WCAG 2.1 AA contrast. Success green (4.43:1 on canvas) and subtle ink (3.37:1 on canvas) do not meet the 4.5:1 requirement for normal text. [REVIEW REQUIRED]
- Semantic table markup, explicit row labels, keyboard-navigable dialogs, focus trapping, and inert backgrounds are part of the dashboard shell.
- Progress and status animations respect `prefers-reduced-motion`.
- Setup and extension-guide pages keep a single visible path rather than presenting both browsers at once.
