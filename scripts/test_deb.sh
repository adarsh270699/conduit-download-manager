#!/usr/bin/env bash
# Smoke-test the Ubuntu package: metadata, contents, install, --version, and
# removal. CI runs this on ubuntu-24.04 after scripts/build_linux.sh.
#
# Usage: sudo is used for apt; run it in CI or a throwaway VM.
#   bash scripts/test_deb.sh [path/to/conduit_X_amd64.deb]
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
VERSION="$(tr -d '\r\n' < "$ROOT_DIR/daemon/version/VERSION")"
DEB="${1:-$ROOT_DIR/dist/conduit_${VERSION}_amd64.deb}"
# Debian orders prereleases with ~ (1.3.0~rc.1 < 1.3.0); nfpm writes it so.
DEB_VERSION="${VERSION/-/\~}"

failures=0
pass() { printf '  ok   %s\n' "$*"; }
fail() { printf '  FAIL %s\n' "$*"; failures=$((failures + 1)); }
check() { local msg="$1"; shift; if "$@"; then pass "$msg"; else fail "$msg"; fi; }

[[ -s "$DEB" ]] || { echo "No package at $DEB" >&2; exit 1; }
echo "Testing $DEB (expected version $DEB_VERSION)"

printf '\n==> Metadata\n'
check "Package is conduit" [ "$(dpkg-deb -f "$DEB" Package)" = conduit ]
check "Version is $DEB_VERSION" [ "$(dpkg-deb -f "$DEB" Version)" = "$DEB_VERSION" ]
check "Architecture is amd64" [ "$(dpkg-deb -f "$DEB" Architecture)" = amd64 ]
depends="$(dpkg-deb -f "$DEB" Depends)"
for dep in aria2 libgtk-3-0 libwebkit2gtk-4.1-0 libayatana-appindicator3-1; do
    check "Depends on $dep" grep -q "$dep" <<<"$depends"
done

printf '\n==> Contents\n'
contents="$(dpkg-deb -c "$DEB" | awk '{print $NF}')"
for path in ./usr/bin/conduit ./usr/share/applications/conduit.desktop \
    ./usr/share/doc/conduit/LICENSE ./usr/share/conduit/extension/manifest.json; do
    check "contains $path" grep -qx "$path" <<<"$contents"
done
check "does not ship extension tests" bash -c '! grep -q "/extension/test/" <<<"$1"' _ "$contents"

printf '\n==> Install\n'
sudo apt-get install -y --no-install-recommends "$DEB" >/dev/null
check "dpkg reports installed $DEB_VERSION" [ "$(dpkg-query -W -f='${Version}' conduit)" = "$DEB_VERSION" ]
out="$(/usr/bin/conduit --version)"
check "conduit --version prints 'Conduit $VERSION' (got '$out')" bash -c '[[ "$1" == "Conduit $2"* ]]' _ "$out" "$VERSION"
check "aria2c is installed for downloads" command -v aria2c

printf '\n==> Remove\n'
sudo apt-get remove -y conduit >/dev/null
check "/usr/bin/conduit removed" [ ! -e /usr/bin/conduit ]

echo
if (( failures > 0 )); then
    echo "$failures package check(s) failed."
    exit 1
fi
echo "All package checks passed."
