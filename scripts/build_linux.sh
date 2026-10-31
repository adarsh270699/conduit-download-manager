#!/usr/bin/env bash
# Linux build for Conduit: native app, license bundle, extension zip, and the
# .deb when nfpm is installed (set CONDUIT_REQUIRE_DEB=1 to make it mandatory).
set -euo pipefail

# `go install` puts wails and nfpm here. Non-login shells such as WSL often do
# not have it on PATH, so add it after the user's own entries.
for go_bin in "${GOBIN:-}" "${GOPATH:-$HOME/go}/bin"; do
    [[ -n "$go_bin" && -d "$go_bin" ]] && PATH="$PATH:$go_bin"
done
export PATH

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
DESKTOP_DIR="$ROOT_DIR/desktop"
EXTENSION_DIR="$ROOT_DIR/extension"
APP_BIN="$DESKTOP_DIR/build/bin/Conduit"
STANDALONE_OUT="$DIST_DIR/Conduit-linux-amd64-standalone"
STANDALONE_ZIP="$DIST_DIR/Conduit-linux-amd64-standalone.zip"
EXTENSION_OUT="$DIST_DIR/conduit-extension.zip"
STAGE_DIR=""

log() {
    printf '\n==> %s\n' "$*"
}

die() {
    printf 'ERROR: %s\n' "$*" >&2
    exit 1
}

[[ "$(uname -s)" == "Linux" ]] || die "This script builds the Linux app and must run on Linux."
[[ -f "$DESKTOP_DIR/wails.json" ]] || die "Run this script from a Conduit checkout."
command -v go >/dev/null 2>&1 || die "go was not found on PATH. Install Go 1.27.2+."
command -v wails >/dev/null 2>&1 || die "wails was not found. Run: go install github.com/wailsapp/wails/v2/cmd/wails@v2.11.0"
command -v install >/dev/null 2>&1 || die "install (coreutils) was not found."

# Wails links GTK and WebKitGTK through cgo, so check for them up front
# instead of letting `wails build` fail with a compiler error.
APT_DEPS="gcc pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev"
command -v gcc >/dev/null 2>&1 || die "gcc was not found. Run: sudo apt install ${APT_DEPS}"
command -v pkg-config >/dev/null 2>&1 || die "pkg-config was not found. Run: sudo apt install ${APT_DEPS}"
pkg-config --exists gtk+-3.0 webkit2gtk-4.1 || die "GTK3 or WebKitGTK 4.1 development files are missing. Run: sudo apt install ${APT_DEPS}"

if command -v zip >/dev/null 2>&1; then
    ZIP_METHOD=zip
elif command -v python3 >/dev/null 2>&1; then
    ZIP_METHOD=python
else
    die "zip or python3 is required to package the extension."
fi

cleanup() {
    if [[ -n "$STAGE_DIR" && -d "$STAGE_DIR" ]]; then
        rm -rf "$STAGE_DIR"
    fi
}
trap cleanup EXIT

mkdir -p "$DIST_DIR"
rm -f "$STANDALONE_OUT" "$STANDALONE_ZIP" "$EXTENSION_OUT"

# daemon/version/VERSION is the only place the version is written. The release
# tool fails here if wails.json or the extension manifest drifted from it.
log "Checking version fields"
(cd "$ROOT_DIR/daemon" && go run ./tools/release check)
VERSION="$(cd "$ROOT_DIR/daemon" && go run ./tools/release print)"
export VERSION
# CI sets GITHUB_SHA; a local checkout asks git. Builds outside git stay "dev".
COMMIT="${GITHUB_SHA:-$(git -C "$ROOT_DIR" rev-parse HEAD 2>/dev/null || echo dev)}"
LDFLAGS="-X conduit/daemon/version.Commit=${COMMIT}"
DEB_OUT="$DIST_DIR/conduit_${VERSION}_amd64.deb"
rm -f "$DEB_OUT"
printf '    Conduit %s (%s)\n' "$VERSION" "$COMMIT"

log "Building native Conduit app (linux/amd64)"
# Ubuntu 24.04 ships WebKitGTK 4.1. Wails defaults to 4.0 unless this tag is set.
if ! (cd "$DESKTOP_DIR" && wails build -skipbindings -skipembedcreate -platform linux/amd64 -tags webkit2_41 -ldflags "$LDFLAGS"); then
    die "wails build failed. Install the Linux prerequisites from README.md (GTK3, WebKitGTK 4.1, and libayatana-appindicator3)."
fi
[[ -f "$APP_BIN" ]] || die "Wails did not produce $APP_BIN"
install -m 0755 "$APP_BIN" "$STANDALONE_OUT"

log "Packaging standalone app with license bundle"
STAGE_DIR="$(mktemp -d)"
install -m 0755 "$APP_BIN" "$STAGE_DIR/Conduit"
cp "$ROOT_DIR/LICENSE" "$ROOT_DIR/NOTICE" "$ROOT_DIR/THIRD_PARTY_NOTICES.md" "$ROOT_DIR/SOURCE_OFFER.md" "$STAGE_DIR/"
cp -r "$ROOT_DIR/licenses" "$STAGE_DIR/licenses"
if [[ "$ZIP_METHOD" == "zip" ]]; then
    (cd "$STAGE_DIR" && zip -qrD "$STANDALONE_ZIP" .)
else
    python3 - "$STAGE_DIR" "$STANDALONE_ZIP" <<'PY'
import os
import sys
import zipfile

src, dst = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(dst, "w", zipfile.ZIP_DEFLATED) as archive:
    for root, dirs, files in os.walk(src):
        for name in files:
            path = os.path.join(root, name)
            archive.write(path, os.path.relpath(path, src))
PY
fi
[[ -s "$STANDALONE_ZIP" ]] || die "The standalone zip was not created."
rm -rf "$STAGE_DIR"
STAGE_DIR=""

log "Packaging browser extension"
if [[ "$ZIP_METHOD" == "zip" ]]; then
    (
        cd "$EXTENSION_DIR" || exit 1
        zip -qrD "$EXTENSION_OUT" . -x "test/*" -x ".DS_Store" -x "Thumbs.db"
    )
else
    python3 - "$EXTENSION_DIR" "$EXTENSION_OUT" <<'PY'
import os
import sys
import zipfile

src, dst = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(dst, "w", zipfile.ZIP_DEFLATED) as archive:
    for root, dirs, files in os.walk(src):
        dirs[:] = [d for d in dirs if d != "test"]
        for name in files:
            if name in {".DS_Store", "Thumbs.db"}:
                continue
            path = os.path.join(root, name)
            archive.write(path, os.path.relpath(path, src))
PY
fi
[[ -s "$EXTENSION_OUT" ]] || die "The extension zip was not created."

if command -v python3 >/dev/null 2>&1; then
    python3 "$ROOT_DIR/scripts/verify_zip.py" "$STANDALONE_ZIP" --require Conduit LICENSE
    python3 "$ROOT_DIR/scripts/verify_zip.py" "$EXTENSION_OUT" --require manifest.json background.js content.js
fi

built=("$STANDALONE_OUT" "$STANDALONE_ZIP" "$EXTENSION_OUT")
if command -v nfpm >/dev/null 2>&1; then
    log "Building Debian package"
    # nfpm reads VERSION from the environment (packaging/nfpm.yaml).
    (cd "$ROOT_DIR" && nfpm package -f packaging/nfpm.yaml -p deb -t "$DEB_OUT")
    [[ -s "$DEB_OUT" ]] || die "nfpm did not produce $DEB_OUT"
    built+=("$DEB_OUT")
elif [[ "${CONDUIT_REQUIRE_DEB:-0}" == "1" ]]; then
    die "nfpm was not found. Run: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.3"
else
    printf '\nSkipping the .deb: nfpm is not installed.\n'
fi

printf '\nBuild complete:\n'
for file in "${built[@]}"; do
    printf '  %-55s %s\n' "$file" "$(du -h "$file" | cut -f1)"
done
