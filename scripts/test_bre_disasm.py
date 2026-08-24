#!/usr/bin/env python3
"""Synthetic tests for scripts/bre-disasm.py (no BRE binaries required)."""

import hashlib
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
    def test_around_anchors_to_containing_catalog_block(self):
        context = {
            "id": "synthetic",
            "storage": "ovr",
            "blocks": [
                {"name": "root_zero", "unit_span": ["0x0", "0x4"]},
                {"name": "root_four", "unit_span": ["0x4", "0x8"]},
            ],
            "data_chunks": [
                {"name": "data_eight", "unit_span": ["0x8", "0xa"]},
            ],
        }
        lines = [(offset, f"{offset:08X}  90 nop") for offset in range(8)]
        anchor, selected, _ranges = bre.around_range(context, lines, 5, 4)
        self.assertEqual(anchor, (4, 8, "root_four"))
        self.assertEqual([offset for offset, _line in selected], [4, 5, 6, 7])
        with self.assertRaisesRegex(bre.BREError, "refusing to guess"):
            bre.around_range(context, lines, 9, 4)

    def test_new_disassembly_parser_selectors(self):
        parser = bre.build_parser()
        args = parser.parse_args(
            [
                "disasm",
                "--directory",
                "/private/bre",
                "--around",
                "056d:01bf",
                "--instructions",
                "24",
            ]
        )
        self.assertEqual(args.around, "056d:01bf")
        self.assertEqual(args.instructions, 24)
        args = parser.parse_args(
            ["xrefs", "create_trade_offer", "--direction", "callers"]
        )
        self.assertEqual(args.name, "create_trade_offer")
        self.assertEqual(args.direction, "callers")
        self.assertIsNone(args.directory)

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
                    "logical_target": "0010:0020",
                    "sites": ["0x000b"],
                }
            ],
        )

    def test_cfg_records_every_direct_jump_target_and_fallthrough(self):
        try:
            bre.capstone_module()
        except bre.BREError as exc:
            self.skipTest(str(exc))
        # jne +2 has two successors: the fallthrough NOP/RET block at 2 and
        # the taken RET block at 4.
        flow = bre.analyze_cfg(bytes.fromhex("750290c3c3"), {0}, {})
        self.assertEqual(sorted(flow["_blocks"]), [0, 2, 4])
        self.assertEqual(
            flow["_blocks"][2]["sources"],
            [(0, "conditional_fallthrough")],
        )
        self.assertEqual(
            flow["_blocks"][4]["sources"],
            [(0, "conditional_jump")],
        )
        self.assertEqual(flow["decode_conflicts"], [])

    def test_string_index_uses_durable_ids_without_retaining_text(self):
        data = b"\x90\x90\x90\x90\x05Alpha"
        procedures = [
            {
                "id": "bre0988:ovr:procedure:000008",
                "body_ranges": [["0x0000", "0x0004"]],
            }
        ]
        blocks = [
            {
                "id": "bre0988:ovr:block:000008",
                "unit_span": ["0x0000", "0x0004"],
            }
        ]
        chunks = [{"unit_span": ["0x0004", "0x000a"]}]
        records = bre.string_records(
            "ovr",
            data,
            procedures,
            blocks,
            chunks,
            [(1, 4, "cs_offset_register_pair")],
            base_offset=8,
        )
        self.assertEqual(len(records), 1)
        record = records[0]
        self.assertEqual(record["id"], "bre0988:ovr:string:00000c")
        self.assertEqual(record["ovr_offset"], "0x00000c")
        self.assertEqual(record["length"], 5)
        self.assertEqual(record["sha256"], hashlib.sha256(b"Alpha").hexdigest())
        self.assertNotIn("text", record)
        self.assertEqual(
            record["used_by"],
            [
                {
                    "block_id": "bre0988:ovr:block:000008",
                    "procedure_ids": ["bre0988:ovr:procedure:000008"],
                    "kind": "cs_offset_register_pair",
                    "sites": ["0x000009"],
                }
            ],
        )

    def test_synthetic_catalog_exhaustively_names_code_and_data(self):
        try:
            bre.capstone_module()
        except bre.BREError as exc:
            self.skipTest(str(exc))
        exe, ovr = synthetic_release()
        mz = bre.parse_mz(exe)
        units = bre.parse_units(exe, ovr, mz)
        catalog = bre.build_catalog(exe, ovr, mz, units, cfg=True)
        result = bre.validate_catalog(catalog)
        self.assertEqual(result["overlay_blocks"], 3)
        self.assertEqual(result["overlay_data_chunks"], 1)
        self.assertEqual(
            catalog["units"][1]["data_chunks"][0]["name"],
            "ovr_000012_data_0005",
        )
        self.assertEqual(
            catalog["units"][1]["data_chunks"][0]["classification"],
            "nop_padding",
        )
        records = list(bre.catalog_records(catalog, "all"))
        self.assertEqual(len({record["name"] for record in records}), len(records))

    def test_committed_catalog_invariants(self):
        catalog_path = SCRIPT.parent.parent / "docs" / "dev" / "bre-v0988-disassembly.json"
        if not catalog_path.exists():
            self.skipTest("generated catalog is not present")
        catalog = json.loads(catalog_path.read_text())
        self.assertEqual(catalog["format_version"], 6)
        self.assertEqual(catalog["summary"]["unit_count"], 103)
        self.assertEqual(catalog["summary"]["exported_root_count"], 414)
        self.assertEqual(catalog["summary"]["reachable_procedure_root_count"], 603)
        self.assertEqual(catalog["summary"]["basic_block_count"], 8495)
        self.assertEqual(catalog["summary"]["data_chunk_count"], 319)
        self.assertEqual(catalog["summary"]["resident_procedure_root_count"], 389)
        self.assertEqual(catalog["summary"]["resident_basic_block_count"], 2921)
        self.assertEqual(catalog["summary"]["resident_data_chunk_count"], 231)
        self.assertEqual(catalog["summary"]["referenced_string_count"], 2350)
        self.assertEqual(catalog["summary"]["string_use_count"], 2571)
        self.assertEqual(catalog["summary"]["call_edge_count"], 6541)
        self.assertEqual(catalog["summary"]["call_site_count"], 20324)
        self.assertEqual(catalog["summary"]["calculated_transfer_group_count"], 13)
        self.assertEqual(catalog["summary"]["calculated_transfer_site_count"], 23)
        self.assertEqual(catalog["summary"]["calculated_target_count"], 29)
        self.assertEqual(
            catalog["summary"]["semantic_coverage"],
            {
                "procedures": {"identified": 407, "unclassified": 585},
                "blocks": {
                    "contextual": 6950,
                    "identified": 448,
                    "unclassified": 4018,
                },
                "data_chunks": {
                    "identified": 282,
                    "structural": 103,
                    "unclassified": 165,
                },
                "fixup_chunks": {"structural": 103},
            },
        )
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
        self.assertFalse(catalog["resident_image"]["control_flow"]["decode_conflicts"])
        self.assertEqual(
            sum(
                len(unit["control_flow"]["unresolved_transfers"])
                for unit in catalog["units"]
            ),
            0,
        )
        self.assertEqual(
            len(catalog["resident_image"]["control_flow"]["unresolved_transfers"]),
            0,
        )
        self.assertEqual(
            bre.validate_catalog(catalog),
            {
                "unique_names": 12675,
                "overlay_blocks": 8495,
                "overlay_data_chunks": 319,
                "resident_blocks": 2921,
                "resident_data_chunks": 231,
                "referenced_strings": 2350,
                "string_uses": 2571,
                "call_edges": 6541,
                "call_sites": 20324,
                "calculated_transfer_groups": 13,
                "calculated_transfer_sites": 23,
                "calculated_targets": 29,
            },
        )

        procedures = [
            root for unit in catalog["units"] for root in unit["roots"]
        ] + catalog["resident_image"]["roots"]
        procedure_ids = {root["id"] for root in procedures}
        by_name = {root["name"]: root for root in procedures}
        self.assertIn("run_bank", by_name)
        self.assertEqual(
            by_name["run_bank"]["id"],
            "bre0988:ovr:procedure:0389d6",
        )
        self.assertEqual(by_name["run_bank"]["naming"]["status"], "identified")
        self.assertTrue(by_name["run_bank"]["callers"])
        self.assertTrue(by_name["run_bank"]["callees"])
        self.assertTrue(
            all(
                caller["from_id"].startswith("bre0988:")
                and caller["site_ids"]
                for caller in by_name["run_bank"]["callers"]
            )
        )
        dispatches = catalog["calculated_transfers"]
        self.assertEqual(len(dispatches), 13)
        self.assertTrue(all(transfer["closed"] for transfer in dispatches))
        self.assertEqual(
            len(
                {
                    site_id
                    for transfer in dispatches
                    for site_id in transfer["site_ids"]
                }
            ),
            23,
        )
        self.assertTrue(
            all(
                target["id"] in procedure_ids
                for transfer in dispatches
                for target in transfer["targets"]
            )
        )
        dispatch_records = list(bre.catalog_records(catalog, "dispatch"))
        self.assertEqual(len(dispatch_records), 13)
        self.assertTrue(all(record["sources"] for record in dispatch_records))
        self.assertTrue(all(record["targets"] for record in dispatch_records))
        self.assertIn("scan_text_real48", by_name)
        self.assertTrue(
            all(
                callee["to_id"].startswith("bre0988:")
                and callee["site_ids"]
                for callee in by_name["run_bank"]["callees"]
            )
        )
        self.assertTrue(by_name["run_bank"]["data_references"])
        self.assertIn(
            "run_bank",
            {
                caller["from"]
                for root in procedures
                for caller in root["callers"]
            },
        )

        blocks = [
            block for unit in catalog["units"] for block in unit["blocks"]
        ] + catalog["resident_image"]["blocks"]
        blocks_by_name = {block["name"]: block for block in blocks}
        self.assertIn(
            "allocate_turn_budget__armed_forces_maintenance",
            blocks_by_name,
        )
        self.assertIn(
            "run_player_turn__dispatch_resume_stage",
            blocks_by_name,
        )
        self.assertEqual(
            blocks_by_name["allocate_turn_budget__armed_forces_maintenance"][
                "naming"
            ]["status"],
            "identified",
        )
        block_ids = {block["id"] for block in blocks}
        self.assertTrue(catalog["string_index"])
        self.assertTrue(
            all("text" not in record for record in catalog["string_index"])
        )
        self.assertTrue(
            all(
                use["block_id"] in block_ids
                and all(
                    identifier in procedure_ids
                    for identifier in use["procedure_ids"]
                )
                for record in catalog["string_index"]
                for use in record["used_by"]
            )
        )
        run_bank_record = next(
            record
            for record in bre.catalog_records(catalog, "procedure")
            if record["id"] == by_name["run_bank"]["id"]
        )
        self.assertEqual(run_bank_record["callers"], by_name["run_bank"]["callers"])
        self.assertEqual(run_bank_record["callees"], by_name["run_bank"]["callees"])
        known_call_edges = {
            (
                root["id"],
                callee["to_id"],
                callee["kind"],
                tuple(callee["site_ids"]),
            )
            for root in procedures
            for callee in root["callees"]
            if callee["to_id"] is not None
        }
        inverse_call_edges = {
            (
                caller["from_id"],
                root["id"],
                caller["kind"],
                tuple(caller["site_ids"]),
            )
            for root in procedures
            for caller in root["callers"]
        }
        self.assertEqual(known_call_edges, inverse_call_edges)
        self.assertTrue(
            all(
                callee["to_id"] is not None
                for root in procedures
                for callee in root["callees"]
                if callee["target_address"] is not None
            )
        )
        self.assertIn("calculate_crown_tax", by_name)
        self.assertIn("region_cost", by_name["calculate_crown_tax"]["aliases"])


if __name__ == "__main__":
    unittest.main()
