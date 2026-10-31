#!/usr/bin/env bash
# Fail if a Conduit build output ships an aria2c engine. Conduit gets the engine
# at install time (the Windows installer) or from the system package manager
# (the .deb depends on aria2), so no artifact may contain one.
#
# usage: scripts/check_no_engine.sh PATH...   (files or directories to scan)
set -euo pipefail

if [[ $# -eq 0 ]]; then
    echo "usage: $0 PATH..." >&2
    exit 2
fi

# Text from the copyright banner that aria2c prints. Conduit does not contain it.
marker="Tatsuhiro Tsujikawa"
# A file or archive member named aria2c or aria2c.exe, at any depth.
aria2_name='(^|[/\\ ])aria2c(\.exe)?$'
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT
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

# Prints the path of 7-Zip, which reads NSIS installers. Prints nothing when it
# is not installed.
seven_zip() {
    if command -v 7z >/dev/null 2>&1; then
        command -v 7z
    elif [[ -x "/c/Program Files/7-Zip/7z.exe" ]]; then
        echo "/c/Program Files/7-Zip/7z.exe"
    fi
}

# Fails when one file holds aria2c's copyright banner.
check_marker() {
    local label="$1" file="$2" rc=0
    grep -aq -- "$marker" "$file" || rc=$?
    case "$rc" in
        0) fail "$label contains aria2c code" ;;
        1) ;;
        *) fail "$label could not be read" ;;
    esac
}

# NSIS compresses its payload, so the raw bytes do not show the banner. 7-Zip
# lists the payload names and unpacks it, and both are checked.
check_installer() {
    local file="$1" zip listing dir native
    zip="$(seven_zip)"
    if [[ -z "$zip" ]]; then
        fail "$file cannot be inspected: install 7-Zip"
        return
    fi
    if ! listing="$("$zip" l -ba -- "$file")"; then
        fail "$file could not be listed"
        return
    fi
    if grep -Eiq "$aria2_name" <<<"$listing"; then
        fail "$file contains an aria2c binary"
        return
    fi
    dir="$scratch/unpacked"
    rm -rf "$dir"
    mkdir "$dir"
    native="$(cygpath -w "$dir")"
    if ! "$zip" x -y "-o$native" -- "$file" >/dev/null; then
        fail "$file could not be unpacked"
        return
    fi
    while IFS= read -r -d '' inner; do
        check_marker "$file:${inner#"$dir"/}" "$inner"
    done < <(find "$dir" -type f -print0)
}

for path in "$@"; do
    if [[ ! -e "$path" ]]; then
        # A missing output directory must not pass the check.
        fail "$path does not exist"
        continue
    fi
    while IFS= read -r -d '' file; do
        case "$file" in
            *-setup.exe)
                check_installer "$file"
                ;;
            *.deb)
                if ! listing="$(dpkg-deb -c "$file")"; then
                    fail "$file could not be listed"
                elif grep -Eiq "$aria2_name" <<<"$listing"; then
                    fail "$file contains an aria2c binary"
                fi
                ;;
            *.zip)
                if ! listing="$(zip_names "$file")"; then
                    fail "$file could not be listed"
                elif grep -Eiq "$aria2_name" <<<"$listing"; then
                    fail "$file contains an aria2c binary"
                fi
                ;;
            *)
                # Standalone binaries are not compressed, so the banner shows.
                check_marker "$file" "$file"
                ;;
        esac
    done < <(find "$path" -type f -print0)
done

if [[ $status -eq 0 ]]; then
    echo "No aria2 engine in the build outputs."
fi
exit "$status"
