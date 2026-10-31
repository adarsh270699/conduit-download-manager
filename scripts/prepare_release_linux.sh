#!/usr/bin/env bash
# Stage the Linux release files in dist/release/.
#
# Usage:
#   bash scripts/prepare_release_linux.sh            # version from daemon/version/VERSION
#   bash scripts/prepare_release_linux.sh 1.3.0      # also fail unless VERSION is 1.3.0
#
# Builds the native app, extension zip, and .deb (scripts/build_linux.sh),
# and copies only the upload set into dist/release/
#
# The release workflow (.github/workflows/release.yml) runs this script on a
# tag, so a local run is a dry run of exactly what CI publishes. Publish from
# CI, not by uploading these files by hand; see docs/RELEASING.md.
set -euo pipefail

# `go install` puts nfpm and wails here. Non-login shells such as WSL often do
# not have it on PATH, so add it after the user's own entries. build_linux.sh
# does the same when run on its own.
for go_bin in "${GOBIN:-}" "${GOPATH:-$HOME/go}/bin"; do
    [[ -n "$go_bin" && -d "$go_bin" ]] && PATH="$PATH:$go_bin"
done
export PATH

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
DIST_DIR="$ROOT_DIR/dist"
RELEASE_DIR="$DIST_DIR/release"
REQUESTED="${1:-}"

log() {
    printf '\n==> %s\n' "$*"
}

die() {
    printf 'ERROR: %s\n' "$*" >&2
    exit 1
}

[[ "$(uname -s)" == "Linux" ]] || die "This script builds the Linux release and must run on Linux."

VERSION="$(tr -d '\r\n' < "$ROOT_DIR/daemon/version/VERSION")"
if [[ -n "$REQUESTED" && "$REQUESTED" != "$VERSION" ]]; then
    die "Requested version $REQUESTED does not match daemon/version/VERSION ($VERSION). Bump with: (cd daemon && go run ./tools/release bump $REQUESTED)"
fi

command -v nfpm >/dev/null 2>&1 || die "nfpm was not found. Run: go install github.com/goreleaser/nfpm/v2/cmd/nfpm@v2.41.3"

rm -rf "$RELEASE_DIR"
mkdir -p "$RELEASE_DIR"

log "Building Linux artifacts for ${VERSION}"
CONDUIT_REQUIRE_DEB=1 bash "${SCRIPT_DIR}/build_linux.sh"

log "Staging dist/release"
for name in \
    "Conduit-linux-amd64-standalone.zip" \
    "conduit_${VERSION}_amd64.deb" \
    "conduit-extension.zip"; do
    [[ -s "${DIST_DIR}/${name}" ]] || die "Missing release file: ${DIST_DIR}/${name}"
    cp "${DIST_DIR}/${name}" "${RELEASE_DIR}/${name}"
done

printf '\nRelease folder for %s:\n' "$VERSION"
for file in "$RELEASE_DIR"/*; do
    printf '  %-55s %s\n' "$(basename "$file")" "$(du -h "$file" | cut -f1)"
done
printf '\n  %s\n\n' "$RELEASE_DIR"
