#!/usr/bin/env python3
"""Fail if a release zip is empty or uses paths Windows Explorer hides."""

import argparse
import sys
import zipfile


def verify_zip(path: str, required: list[str]) -> None:
    with zipfile.ZipFile(path) as archive:
        names = archive.namelist()

    if not names:
        raise SystemExit(f"ERROR: {path} has no members")

    for name in names:
        if name.startswith("./") or name.startswith(".\\"):
            raise SystemExit(
                f"ERROR: {path} stores '{name}' with a ./ prefix. "
                "Windows Explorer shows that archive as empty."
            )
        if "\\" in name:
            raise SystemExit(
                f"ERROR: {path} stores '{name}' with a backslash path. "
                "Use forward slashes in zip member names."
            )

    missing = [entry for entry in required if entry not in names]
    if missing:
        raise SystemExit(f"ERROR: {path} is missing {', '.join(missing)}")

    print(f"OK {path} ({len(names)} members)")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("zip_path")
    parser.add_argument("--require", nargs="*", default=[])
    args = parser.parse_args()
    verify_zip(args.zip_path, args.require)


if __name__ == "__main__":
    main()
    sys.exit(0)
