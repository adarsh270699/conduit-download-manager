# Changelog

All notable changes to Conduit are recorded here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Security

- A download no longer sends cookies to another site. The extension used to add the cookies of the current page or the first redirect, including HttpOnly session cookies, to the request for the final host. It now reads only the final host's cookies, and the daemon drops them if a redirect leaves that host or drops from HTTPS to HTTP. The exclusion list also covers a download's final address and right-click downloads.
- A page from another local web server can no longer use the API, so it cannot start a program. Only the dashboard on Conduit's own port and the Chrome extension have the whole API. Firefox add-ons can only do what the extension does, because the daemon cannot tell them apart.
- `enginePath` can only be changed by editing `config.json`.
- A crafted link can no longer add aria2 options or request headers through a line break in a filename or header.
- A download no longer overwrites an existing smaller file of the same name. It takes a new name.
- Requests that name another host are refused, which stops DNS rebinding.
- On Windows, finished downloads get the Mark of the Web, so SmartScreen checks them as it does browser downloads. The dashboard asks before it opens a file that runs code.
- The dashboard sends a Content Security Policy and other security headers, and other sites can no longer frame it. Request bodies are limited to 1 MiB.
- The aria2 RPC secret is no longer on the command line. On Linux the config directory, which holds download cookies in `aria2.session`, is private to the user.
- Builds use Go 1.27.3, for GO-2026-6809. GitHub Actions are pinned to commit SHAs.

## [1.2.2] - 2026-11-03

### Changed

- The desktop app icon is the yellow and black tile, with rounded corners. The tray, the Windows program and installer, and the Ubuntu package all use it.

### Fixed

- The Ubuntu package icon was a placeholder. It now shows the Conduit icon.

## [1.2.1] - 2026-11-02

### Changed

- State changes that used to land in one frame now move. The setup steps and the browser guide, a download line removed by a confirmed delete or clear, the connection banner, the first-time empty queue, the theme icon, and the page colour change, which now cross-fades. The close button and the extension popup's small buttons press like the other buttons. Reduced motion keeps the fades and drops the movement.
- The empty queue's loading, waiting and first-run text fades, and its box eases to the height of each state. The empty queue announces its state to screen readers.
- Setup moves keyboard focus to the next step's heading after Continue. A download line that is being removed cannot take focus or a second action, and the connection banner's Retry button cannot take focus while the banner closes.
- The extension popup and the offline page cross-fade their colour icon and page when the theme changes, as the dashboard does.

## [1.2.0] - 2026-11-01

### Added

- A colour scheme setting, `theme`, with `system`, `light` and `dark`. The dashboard, the extension popup and the offline page each have a button that cycles through the three, and every surface follows the saved choice. The daemon keeps the setting. The extension keeps a copy, so its pages paint correctly while the daemon is off, and a change made while the daemon is off is sent when it returns.
- Time left shows seconds: `2:14` under an hour and `1:02:14` from an hour. Speed and time left animate their changes with NumberFlow, a library served by the daemon itself, with no CDN.
- The dashboard arranges its panels as it opens: the chrome, then the metric cards, filters, next arrival and queue rows rise into place in turn. Reduced motion shows the fade only.
- An app icon in two tiles, a white tile with ink and yellow, and a black tile with off-white and yellow. The dashboard's tab icon follows the colour scheme.
- The in-page download notice follows the same colour scheme as the popup.

### Changed

- The dashboard uses the Departures Board look. IBM Plex Sans is bundled, corners are square, and signal yellow marks only the next arrival and the primary action. Status is a lamp with a word.
- The active count has an ink lamp, not a yellow one. The totals sit in a strip inside the queue panel.
- The extension popup and offline page use the same look. The offline page no longer shows steps for starting the daemon.
- The desktop splash uses the same colours as the dashboard.

### Fixed

- A resumed download's banner resizes with an animation instead of jumping to its new height.
- Refreshing a long queue no longer re-animates every row. Rows that are off screen update when they scroll into view.
- The active count no longer appears twice above the queue.

## [1.1.1] - 2026-11-01

### Security

- Only the headless development daemon reads `CONDUIT_UPDATE_API` and `CONDUIT_UPDATE_DOWNLOAD_HOSTS`. In 1.1.0 the desktop app read them too, so an environment variable could point an installed copy at another release feed.
- An install runs only the installer file that the download wrote. On Linux, the install checks a root-owned copy of the package against its SHA-256 before installing it, so the file cannot be swapped after the check.

### Fixed

- A download that sends no data for 60 seconds is ended, and a download is capped at six hours. A stalled download no longer blocks checks and installs until Conduit restarts.
- A download saved before a restart is ready to install after the restart, so "Install and restart" appears without a new check.
- The "Install updates automatically" switch can be turned off on a copy that cannot install.

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

[Unreleased]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.2.2...HEAD
[1.2.2]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.1.1...v1.2.0
[1.1.1]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.1.0...v1.1.1
[1.1.0]: https://github.com/adarsh270699/conduit-download-manager/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/adarsh270699/conduit-download-manager/releases/tag/v1.0.0
