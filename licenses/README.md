# License texts and notices

Conduit's own code is licensed under **GPL-3.0-or-later**. The full license
text is in the repository root at [LICENSE](../LICENSE); the project copyright
notice is in [NOTICE](../NOTICE).

This directory contains the license texts for the third-party components that
Conduit releases include.

| Path | Component |
| --- | --- |
| `Apache-2.0.txt` | Apache-2.0 components, including `fyne.io/systray` |
| `OFL-1.1.txt` | IBM Plex Sans |
| `go/` | Go module licenses |

Conduit does not ship aria2. See [SOURCE_OFFER.md](../SOURCE_OFFER.md) and
[THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md) for the engine Conduit runs.

## Refreshing the Go licenses

`licenses/go/` is generated from the Go module cache for the modules linked
into the desktop and daemon builds. The exact versions are listed in
`licenses/go/MODULES.txt`.

From `desktop/`, list the modules actually linked into the build and copy the
license files from each module root:

```bash
go list -deps -f '{{if .Module}}{{.Module.Path}}|{{.Module.Version}}|{{.Module.Dir}}{{end}}' ./...
```

Repeat with `GOOS=linux` and `GOOS=windows` to cover both targets. Skip the
`conduit/*` modules and copy every root file whose name starts with `LICENSE`,
`LICENCE`, `COPYING`, `NOTICE`, `PATENTS`, or `UNLICENSE` into
`licenses/go/<module path>/`. Copy `$(go env GOROOT)/LICENSE` to
`licenses/go/Go-LICENSE`.
