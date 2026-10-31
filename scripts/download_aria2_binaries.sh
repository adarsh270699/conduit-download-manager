#!/usr/bin/env bash
# Fetch the pinned aria2c binary for development and tests.
#
# Conduit does not ship aria2c. The installer downloads it on Windows, and
# Linux installs get it from apt. Tests and local runs use a copy in
# .cache/aria2/<platform>/, which this script fills and verifies. It prints the
# path. Set CONDUIT_ARIA2C to that path to run the engine tests.
#
# usage: scripts/download_aria2_binaries.sh [linux|windows]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$ROOT_DIR/scripts/aria2_binaries.env"

platform="${1:-linux}"
case "$platform" in
    linux)
        url="$ARIA2_LINUX_URL"
        expected="$ARIA2_LINUX_SHA256"
        name="aria2c"
        ;;
    windows)
        url="$ARIA2_WIN_URL"
        expected="$ARIA2_WIN_SHA256"
        name="aria2c.exe"
        ;;
    *)
        echo "usage: $0 [linux|windows]" >&2
        exit 2
        ;;
esac

sha256_of() {
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum "$1" | cut -d ' ' -f 1
    else
        shasum -a 256 "$1" | cut -d ' ' -f 1
    fi
}

out_dir="$ROOT_DIR/.cache/aria2/$platform"
out="$out_dir/$name"
mkdir -p "$out_dir"

if [[ -f "$out" && "$(sha256_of "$out")" == "$expected" ]]; then
    echo "Already present and verified: $out" >&2
    echo "$out"
    exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "Downloading $url" >&2
curl -fsSL -o "$work/download.zip" "$url"
unzip -q "$work/download.zip" -d "$work/extracted"

# sed reads all of the output of find. head would exit early, and under pipefail
# that would turn the SIGPIPE from find into a failure.
found="$(find "$work/extracted" -type f -name "$name" | sed -n '1p')"
if [[ -z "$found" ]]; then
    echo "The downloaded archive does not contain $name" >&2
    exit 1
fi
if [[ "$(sha256_of "$found")" != "$expected" ]]; then
    echo "$name failed its SHA-256 check" >&2
    exit 1
fi

cp "$found" "$out"
chmod 755 "$out"
echo "$out"
