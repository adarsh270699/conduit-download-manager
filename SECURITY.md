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
- Browser requests are checked against the dashboard's loopback origin, the Wails desktop asset origin, the Chrome extension ID derived from the pinned `key` in `extension/manifest.json`, and the `moz-extension://` scheme (Firefox assigns a per-profile UUID rather than the add-on ID). Requests from unknown website origins are rejected before handlers run.
- The internal aria2 RPC port uses a per-launch random secret and verifies engine ownership before attaching or stopping a process.
- Download URLs are restricted to HTTP, HTTPS, and FTP at the API boundary.
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
