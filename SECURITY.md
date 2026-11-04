# Security Policy

## Supported versions

Security fixes are applied to the current `1.x` release line.

## Reporting a vulnerability

Please do not open a public issue for a vulnerability. Use the repository's **Security** tab and choose **Report a vulnerability** to send a private GitHub Security Advisory. Include:

- the affected version and platform;
- whether the desktop app or headless daemon was running;
- reproduction steps and the expected/actual behavior;
- the impact you believe is possible;
- any suggested fix or mitigation.

Do not include real cookies, session tokens, private download URLs, or personal file paths in the report. Redact them.

## Scope and threat model

Conduit is a local-only application:

- The daemon listens on `127.0.0.1` only. The API has no remote authentication and must not be exposed through a reverse proxy, port forward, container port mapping, or public tunnel.
- Requests must name a loopback host, which blocks DNS rebinding. Browser requests are checked against the dashboard's own origin (this daemon's port only) and the Chrome extension ID derived from the pinned `key` in `extension/manifest.json`. Requests from any other origin, including pages from other local web servers, are rejected before handlers run.
- Firefox assigns each install a random UUID, so the daemon cannot tell Conduit from another add-on. `moz-extension://` origins can only do what the extension does: read status, list and add downloads, pause, resume, or remove them without deleting files, and change the theme. The Wails bootstrap page can only read `/api/health`. Another installed Firefox add-on can therefore see download URLs and add downloads; it cannot start a program, change settings, or delete files.
- The internal aria2 RPC port uses a per-launch random secret, passed through a private file rather than the command line, and verifies engine ownership before attaching or stopping a process.
- Download URLs are restricted to HTTP, HTTPS, and FTP at the API boundary. Filenames, cookies, referrers, user agents, and folders cannot hold line breaks, which aria2 would read as separate options or headers.
- A download sends only the cookies its final host would receive in the browser. If the daemon sees a redirect leave that host, or drop from HTTPS to HTTP, it starts from the redirect target with no cookies. aria2 fetches the URL itself, so a server that answers aria2 differently from that check is not covered.
- `enginePath` names a program to run, so it can only be set by editing `config.json`.
- On Windows, finished downloads get the Mark of the Web, and the dashboard asks before opening a file that runs code.
- Any local process running as the same user can call the local API, read the config directory, or control the user's downloads. This is outside Conduit's threat model.

Reports about the following are in scope:

- cross-site request forgery or origin-policy bypasses;
- path traversal, arbitrary file write/delete, or command injection;
- cookie/header leakage beyond the documented redirect behavior;
- privilege escalation, persistence, or autostart abuse;
- engine process hijacking or a second instance attaching to the wrong engine;
- XSS in the dashboard, setup, guide, or extension popup/offline pages;
- dependency vulnerabilities that are actually reachable from Conduit.

Reports about a user choosing an unsafe download folder, running the daemon on a shared machine, or exposing the loopback port are generally out of scope unless they bypass an explicit control.
