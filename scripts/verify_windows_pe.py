#!/usr/bin/env python3
"""
Conduit Windows PE Binary Verifier
Verifies the compiled conduit-windows-amd64.exe binary against the architecture requirements:
1. Valid Portable Executable (PE32+) x64 architecture.
2. Optional Header Subsystem == 2 (IMAGE_SUBSYSTEM_WINDOWS_GUI).
   Ensures zero command prompt / console flashing upon launch.
"""

import sys
import os

IMAGE_FILE_MACHINE_AMD64 = 0x8664
PE32_PLUS_MAGIC = 0x20B
IMAGE_SUBSYSTEM_WINDOWS_GUI = 2
IMAGE_SUBSYSTEM_WINDOWS_CUI = 3

def verify_pe(filepath):
    print(f"=== Verifying Windows Binary: {filepath} ===")
    if not os.path.exists(filepath):
        print(f"ERROR: File not found: {filepath}")
        return False

    file_size = os.path.getsize(filepath)
    print(f"File size: {file_size / (1024 * 1024):.2f} MB ({file_size} bytes)")

    with open(filepath, "rb") as f:
        data = f.read()

    # 1. DOS Header
    if len(data) < 64 or data[:2] != b"MZ":
        print("ERROR: Invalid DOS header (expected 'MZ')")
        return False
    print("✓ DOS Header: Valid 'MZ' signature")

    # 2. PE Header Offset at 0x3C
    pe_offset = int.from_bytes(data[0x3C:0x40], "little")
    if pe_offset + 24 > len(data):
        print(f"ERROR: Invalid PE header offset: {pe_offset}")
        return False

    pe_sig = data[pe_offset:pe_offset+4]
    if pe_sig != b"PE\x00\x00":
        print(f"ERROR: Invalid PE signature: {pe_sig}")
        return False
    print(f"✓ PE Signature: Valid 'PE\\0\\0' at offset {hex(pe_offset)}")

    # 3. COFF File Header
    coff_offset = pe_offset + 4
    machine = int.from_bytes(data[coff_offset:coff_offset+2], "little")
    num_sections = int.from_bytes(data[coff_offset+2:coff_offset+4], "little")

    if machine != IMAGE_FILE_MACHINE_AMD64:
        print(f"ERROR: Unexpected machine architecture: {hex(machine)} (expected AMD64 {hex(IMAGE_FILE_MACHINE_AMD64)})")
        return False
    print(f"✓ Target Architecture: x86-64 / AMD64 ({hex(machine)}), Sections: {num_sections}")

    # 4. Optional Header (PE32+ 64-bit)
    opt_offset = coff_offset + 20
    opt_magic = int.from_bytes(data[opt_offset:opt_offset+2], "little")
    if opt_magic != PE32_PLUS_MAGIC:
        print(f"ERROR: Unexpected Optional Header Magic: {hex(opt_magic)} (expected PE32+ {hex(PE32_PLUS_MAGIC)})")
        return False
    print(f"✓ Optional Header Format: PE32+ 64-bit ({hex(opt_magic)})")

    # 5. Subsystem Check (Offset 68 in PE32+ Optional Header)
    subsystem_offset = opt_offset + 68
    subsystem = int.from_bytes(data[subsystem_offset:subsystem_offset+2], "little")

    subsystem_names = {
        IMAGE_SUBSYSTEM_WINDOWS_GUI: "IMAGE_SUBSYSTEM_WINDOWS_GUI (2)",
        IMAGE_SUBSYSTEM_WINDOWS_CUI: "IMAGE_SUBSYSTEM_WINDOWS_CUI (3 - Console)"
    }
    subsystem_str = subsystem_names.get(subsystem, f"Unknown ({subsystem})")

    print(f"✓ Executable Subsystem: {subsystem_str}")

    if subsystem != IMAGE_SUBSYSTEM_WINDOWS_GUI:
        print(f"ERROR: Binary subsystem is {subsystem_str}. Expected GUI (2) to prevent console window flashing!")
        return False

    print("\n---------------------------------------------------------")
    print("VERIFICATION SUCCESS: Binary is a native Windows GUI app.")
    print("Guaranteed zero command prompt flashing on startup/launch.")
    print("---------------------------------------------------------\n")
    return True

if __name__ == "__main__":
    target = sys.argv[1] if len(sys.argv) > 1 else "dist/conduit-windows-amd64.exe"
    success = verify_pe(target)
    sys.exit(0 if success else 1)
