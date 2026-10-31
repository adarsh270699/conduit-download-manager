#!/usr/bin/env bash
# Fail if a Conduit build output ships an aria2c engine. Conduit gets the engine
# at install time (the Windows installer) or from the system package manager
# (the .deb depends on aria2), so no artifact may contain one.
#
# usage: scripts/check_no_engine.sh PATH...   (files or directories to scan)
set -euo pipefail

# Text from the copyright banner that aria2c prints. Conduit does not contain it.
marker="Tatsuhiro Tsujikawa"
status=0

fail() {
    echo "FAIL: $1" >&2
    status=1
}

# Prints the member names of a zip. Uses unzip when present, otherwise Python
# (GitHub Windows runners have Python but no unzip).
zip_names() {
    if command -v unzip >/dev/null 2>&1; then
        unzip -Z1 "$1"
    else
        python -c 'import sys, zipfile; print("\n".join(zipfile.ZipFile(sys.argv[1]).namelist()))' "$1"
    fi
}

for path in "$@"; do
    if [[ ! -e "$path" ]]; then
        # A missing output directory must not pass the check.
        fail "$path does not exist"
        continue
    fi
    while IFS= read -r -d '' file; do
        case "$file" in
            *.deb)
                if ! listing="$(dpkg-deb -c "$file")"; then
                    fail "$file could not be listed"
                elif grep -Eiq '(^|/)aria2c(\.exe)?$' <<<"$listing"; then
                    fail "$file contains an aria2c binary"
                fi
                ;;
            *.zip)
                if ! listing="$(zip_names "$file")"; then
                    fail "$file could not be listed"
                elif grep -Eiq '(^|/)aria2c(\.exe)?$' <<<"$listing"; then
                    fail "$file contains an aria2c binary"
                fi
                ;;
            *)
                # Binaries and installers. A compressed payload hides the marker,
                # so this catches uncompressed builds; the archive name checks
                # above cover the rest.
                rc=0
                grep -aq -- "$marker" "$file" || rc=$?
                case "$rc" in
                    0) fail "$file contains aria2c code" ;;
                    1) ;;
                    *) fail "$file could not be read" ;;
                esac
                ;;
        esac
    done < <(find "$path" -type f -print0)
done

if [[ $status -eq 0 ]]; then
    echo "No aria2 engine in the build outputs."
fi
exit "$status"
