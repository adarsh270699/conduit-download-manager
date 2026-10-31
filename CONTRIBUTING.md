# Contributing to Conduit

Thanks for helping improve Conduit. This guide covers the local workflow, the quality gate, and the conventions used in the repository.

## Before you start

- **Go 1.27.2+** for the daemon.
- **Node.js 20+** for the extension test suite.
- **make**, **zip**, **unzip**, and **curl** with a POSIX shell (Git Bash or MSYS2 on Windows); the quick build scripts do not need make or zip.
- Optional but needed for the native window:
  - Wails v2.11.0: `go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0`
  - Windows: NSIS 3.
  - Ubuntu 24.04: `gcc`, `libgtk-3-dev`, `libwebkit2gtk-4.1-dev`, `libayatana-appindicator3-dev`, and `nfpm`.
- Conduit does not ship aria2c. Integration tests need one: `make test-all` fetches a pinned test copy into `.cache/aria2/`, or set `CONDUIT_ARIA2C` to a binary you choose.

## Repository layout

- `daemon/` — Go daemon and embedded dashboard.
- `desktop/` — Wails shell.
- `extension/` — Manifest V3 extension and its Node tests.
- `packaging/` — NSIS and Debian packaging.
- `scripts/` - quick build scripts, engine fetch, and verification tooling.
- `licenses/` - third-party license texts and Go module notices.
- `docs/` - architecture, product, and design documentation. Keep user-facing setup in `README.md` and implementation detail in `docs/ARCHITECTURE.md`.

Generated or local-only directories (`dist/`, `graphify-out/`, `.cursor/`, `.gemini/`, `.impeccable/`, `desktop/build/bin/`) are ignored. Do not commit them.

## Local quality gate

```bash
make check              # go vet + Go unit tests (daemon and desktop) + extension tests
make version-check      # every version field agrees with daemon/version/VERSION
make test-all           # adds live aria2 integration tests (fetches a pinned test engine)
make fmt                # gofmt the Go tree
(cd desktop && go test ./...)   # Wails shell
```

Before opening a pull request:

1. `make fmt`
2. `make check`
3. `make test-all` when touching the engine, supervisor, RPC, or storage.
4. `node extension/test/test_extension.js` after any extension change.
5. For UI changes, load the embedded dashboard and exercise the affected states at desktop and narrow widths. The dashboard is at `http://127.0.0.1:18800/` after starting a daemon.
6. For extension changes, load the unpacked extension in Chrome and Firefox and test both the online and offline paths if you changed interception, queueing, or the popup.

CI (`.github/workflows/build.yml`) runs on every push and pull request: version, workflow, shell, format, and license checks; extension tests; race-enabled daemon unit and integration tests and desktop tests on Ubuntu 24.04; daemon and desktop tests on Windows; a full build on both; a `.deb` install test; and an installer install, launch, upgrade, and uninstall test on Windows. Releases run the same workflow; see [docs/RELEASING.md](docs/RELEASING.md).

For a quick local package, use the platform scripts:

```powershell
# Windows: native app + extension zip + NSIS installer
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\build_windows.ps1
```

```bash
# Linux: native app + extension zip, and the .deb when nfpm is installed
./scripts/build_linux.sh
```

## Go conventions

- Keep packages focused and dependencies flowing inward. `server` may use `aria`, `config`, and `platform`; infrastructure packages must not import the server.
- Wrap errors with context: `fmt.Errorf("operation: %w", err)`.
- Use `errors.Is`/`errors.As` rather than string matching when an error type exists.
- Pass `context.Context` through RPC and HTTP paths.
- Do not panic in an HTTP handler. Return a JSON error with the right status.
- Add a regression test for every bug fix. Prefer behavior tests over string-matching tests when practical.
- Run `gofmt`; the CI job fails on unformatted Go files.
- Keep files near or below ~500 lines when practical. `downloads.go` is the current exception; prefer extracting a helper rather than growing it.

## Extension conventions

- The background script has no build step. Keep browser compatibility guards (`typeof chrome !== 'undefined'`, optional APIs) because the same file runs in Chrome and Firefox and in Node tests.
- Escape every remote value before interpolating it into `innerHTML`. Filenames, URLs, and server error messages are untrusted.
- The queue is stored in `chrome.storage.local`. Wrap every read-modify-write in the `withQueueLock` helper so concurrent service-worker events cannot drop each other's jobs.
- Keep Chrome's `onDeterminingFilename` path and Firefox's direct path working. `extension/test/test_extension.js` covers both.
- Do not add a permission or host permission without a concrete feature that needs it.

## API and security

- The daemon binds to loopback. Do not add a listener on `0.0.0.0` or an unauthenticated remote endpoint.
- State-changing browser requests must pass the origin policy in `daemon/internal/server/cors.go`. If you add an API endpoint, add a test proving a foreign origin receives `403`.
- Validate URLs at the API boundary, not only in the UI.
- Keep the extension's deterministic Chrome ID (`key` in `manifest.json`) and Firefox ID intact. The daemon derives the allowed origins from them.
- Never log cookies, RPC secrets, or full download URLs that may contain tokens.

## Documentation

- `README.md` is the only install/usage entry point. Keep commands and paths accurate.
- `docs/ARCHITECTURE.md` is the design source of truth. Update it when behavior, storage, the API, or platform integration changes.
- `docs/PRODUCT.md` is the product brief; change it only when the product scope or user promise changes.
- `docs/DESIGN.md` is the visual language. Update tokens and component rules there, not in a new ad-hoc document.
- Remove planning notes and task lists when a phase ships; version control history is the archive.

## Licensing

Conduit is licensed under GPL-3.0-or-later. By submitting a contribution, you
agree that it may be distributed under those terms. Do not add code or assets
that cannot be licensed under GPL-3.0-or-later. Record every new third-party
component in `THIRD_PARTY_NOTICES.md` and add its license text under
`licenses/`. aria2 is not bundled; see
`SOURCE_OFFER.md`.

## Commit and pull request guidance

- Keep each change focused. Separate refactors from behavior changes.
- Use an imperative subject line: `fix(server): reject cross-site API requests`, not `fixed stuff`.
- Explain why in the body, including any trade-offs or follow-up work.
- Include the commands you ran and the result in the pull request description.
- Link the issue when one exists.

## Reporting bugs

Include the Conduit version, OS, browser, whether the daemon was running, the exact steps, and the relevant daemon log. Do not paste cookie values, session tokens, or private URLs. For security-sensitive issues, use the repository's private vulnerability reporting instead of a public issue.
