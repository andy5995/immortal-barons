#!/usr/bin/env python3
"""Synthetic tests for scripts/bre-disasm.py (no BRE binaries required)."""

import importlib.util
import json
from pathlib import Path
import struct
import sys
import unittest


SCRIPT = Path(__file__).with_name("bre-disasm.py")
SPEC = importlib.util.spec_from_file_location("bre_disasm", SCRIPT)
bre = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
sys.modules[SPEC.name] = bre
SPEC.loader.exec_module(bre)


def synthetic_release():
    header_size = 0x40
    exe = bytearray(0xC0)
    struct.pack_into(
        "<14H",
        exe,
        0,
        0x5A4D,
        len(exe),
        1,
        0,
        header_size // 16,
        0,
        0,
        2,
        0xFFF0,
        0,
        0x1234,
        0,
        0,
        0,
    )
    ovr = bytearray(b"FBOV" + struct.pack("<I", 0x24))

    first_code = bytearray(b"\x9a\x34\x12\x10\x00\xcb\x90\xcb")
    first_fixups = struct.pack("<H", 3)
    first_offset = len(ovr)
    ovr += first_code + first_fixups
    first_descriptor = 0x40
    exe[first_descriptor : first_descriptor + 4] = b"\xcd\x3f\x00\x00"
    struct.pack_into("<IHHHH", exe, first_descriptor + 4, first_offset, 8, 2, 2, 0)
    exe[first_descriptor + 0x20 : first_descriptor + 0x25] = b"\xcd\x3f\x00\x00\x00"
    exe[first_descriptor + 0x25 : first_descriptor + 0x2A] = b"\xcd\x3f\x06\x00\x00"

    second_code = bytearray(b"\x55\x89\xe5\x5d\xcb\x90")
    second_fixups = b""
    second_offset = len(ovr)
    ovr += second_code + second_fixups
    second_descriptor = 0x80
    exe[second_descriptor : second_descriptor + 4] = b"\xcd\x3f\x00\x00"
    struct.pack_into(
        "<IHHHH",
        exe,
        second_descriptor + 4,
        second_offset,
        len(second_code),
        len(second_fixups),
        1,
        0,
    )
    # The previous descriptor segment is zero because the first descriptor is
    # at logical segment zero.
    exe[second_descriptor + 0x20 : second_descriptor + 0x25] = b"\xcd\x3f\x00\x00\x00"
    return bytes(exe), bytes(ovr)


class ParseTests(unittest.TestCase):
    def test_descriptor_chain_and_roots(self):
        exe, ovr = synthetic_release()
        mz = bre.parse_mz(exe)
        units = bre.parse_units(exe, ovr, mz)
        self.assertEqual(len(units), 2)
        self.assertEqual([stub.entry_offset for stub in units[0].stubs], [0, 6])
        self.assertEqual(units[0].fixups, (3,))
        self.assertEqual(units[1].ovr_offset, units[0].end_offset)
        self.assertEqual(units[-1].end_offset, len(ovr))

    def test_materialize_applies_only_fixup_words(self):
        exe, ovr = synthetic_release()
        mz = bre.parse_mz(exe)
        unit = bre.parse_units(exe, ovr, mz)[0]
        materialized = bre.materialized_code(ovr, unit, 0x20)
        self.assertEqual(materialized, b"\x9a\x34\x12\x30\x00\xcb\x90\xcb")

    def test_catalog_has_stable_fallback_names(self):
        exe, ovr = synthetic_release()
        mz = bre.parse_mz(exe)
        units = bre.parse_units(exe, ovr, mz)
        catalog = bre.build_catalog(exe, ovr, mz, units, cfg=False)
        self.assertEqual(catalog["summary"]["unit_count"], 2)
        self.assertEqual(catalog["summary"]["exported_root_count"], 3)
        self.assertEqual(catalog["units"][0]["id"], "ovr_000008")
        self.assertEqual(
            catalog["units"][0]["roots"][1]["name"],
            "ovr_000008_entry_0006",
        )

    def test_rejects_broken_overlay_chain(self):
        exe, ovr = synthetic_release()
        broken = bytearray(ovr)
        broken.insert(10, 0)
        mz = bre.parse_mz(exe)
        with self.assertRaises(bre.BREError):
            bre.parse_units(exe, bytes(broken), mz)

    def test_capstone_cfg_follows_typed_direct_call(self):
        try:
            bre.capstone_module()
        except bre.BREError as exc:
            self.skipTest(str(exc))
        code = bytes.fromhex("5589e5e80200cb905589e59a200010005dcb")
        flow = bre.analyze_cfg(
            code,
            {0},
            {(0x0010, 0x0020): ("ovr_000100", "ovr_000100_entry_0000")},
        )
        self.assertEqual(flow["_procedure_roots"], [0, 8])
        self.assertEqual(flow["decode_conflicts"], [])
        self.assertEqual(
            flow["external_edges"],
            [
                {
                    "kind": "overlay_call",
                    "to": "ovr_000100:ovr_000100_entry_0000",
                    "sites": ["0x000b"],
                }
            ],
        )

    def test_committed_catalog_invariants(self):
        catalog_path = SCRIPT.parent.parent / "docs" / "dev" / "bre-v0988-disassembly.json"
        if not catalog_path.exists():
            self.skipTest("generated catalog is not present")
        catalog = json.loads(catalog_path.read_text())
        self.assertEqual(catalog["summary"]["unit_count"], 103)
        self.assertEqual(catalog["summary"]["exported_root_count"], 414)
        self.assertEqual(catalog["summary"]["reachable_procedure_root_count"], 603)
        self.assertEqual(
            catalog["release"]["artifacts"]["ovr"]["sha256"],
            bre.EXPECTED["ovr"]["sha256"],
        )
        self.assertEqual(catalog["units"][-1]["ovr"]["end_offset"], "0x059123")
        self.assertFalse(
            any(
                unit["control_flow"]["decode_conflicts"]
                for unit in catalog["units"]
            )
        )


if __name__ == "__main__":
    unittest.main()
