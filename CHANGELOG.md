# Changelog

All notable changes to Conduit are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.1.0] - 2026-10-31

### Added

- The daemon can check GitHub for a newer stable release, download the installer for this OS, verify it against `SHA256SUMS.txt`, and install it with settings kept. Windows installs run silently and start Conduit again; Linux installs ask for a password once through `pkexec`. Driven by the `/api/update` routes.
- The Settings drawer has an Updates section: a button that checks for an update and downloads it when one is found, then installs it on the next press; and two switches, to check for updates automatically and to install them automatically.

## [1.0.0] - 2026-10-31

### Added

- Multi-connection acceleration through an aria2c engine that Conduit runs as a separate program, with up to 16 connections per server.
- Browser interception for Chrome, Chromium, Brave, Edge, and Firefox, forwarding the final URL, cookies, referrer, user agent, and Content-Disposition filename.
- An offline holding queue in the extension. Downloads are held locally while the daemon is unavailable and forwarded when it returns.
- A native app on Windows 10/11 (WebView2) and Ubuntu 24.04 (WebKitGTK), with a system tray and per-user autostart.
- A per-user Windows installer and an Ubuntu 24.04 `.deb` package.
- A headless daemon for Linux and Windows.
- A local REST API with a WebSocket status stream on `127.0.0.1:18800`.
- Pause, resume, retry, remove, reorder, open folder, change destination, delete file, speed limit, and per-site exclusions.

### Security

- Browser origins are checked before API handlers run, which blocks cross-site POST requests.
- Download URLs are limited to HTTP, HTTPS, and FTP.
- The internal aria2 RPC port uses a per-launch random secret and checks engine ownership before use.

[Unreleased]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.1.0...HEAD
[1.1.0]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/adarsh270699/conduit-download-manager/releases/tag/v1.0.0
