#!/usr/bin/env python3
"""Map Barren Realms Elite 0.988 overlay stubs without shipping its binaries.

The committed catalog contains addresses, names, and control-flow metadata only.
Users supply their own BRE.EXE and BRE.OVR, or explicitly fetch the official
release with the ``fetch`` subcommand.
"""

from __future__ import annotations

import argparse
import collections
import dataclasses
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import struct
import subprocess
import sys
import tempfile
import time
import urllib.request


VERSION = "0.6"
OFFICIAL_URL = (
    "https://www.johndaileysoftware.com/download/"
    "?fileName=brev988.exe&id=220BRE"
)
EXPECTED = {
    "archive": {
        "name": "brev988.exe",
        "size": 337_980,
        "sha256": "40c9d78066add460176a326ebe9f01f3b39df4f70a7bc060ec8ecf09d875b3d5",
    },
    "exe": {
        "name": "BRE.EXE",
        "size": 91_712,
        "sha256": "ae1ce21a01b6b21840e603090e674286fd6848462298f479498d6c17ef31dde6",
    },
    "ovr": {
        "name": "BRE.OVR",
        "size": 364_835,
        "sha256": "c9d6a40261634f6b29c0b3bbf7e8fe8582106fb39ee3f49059c775434164d2c0",
    },
}

SEMANTIC_NAMES_PATH = Path(__file__).with_name("bre-semantic-names.json")


def load_semantic_names() -> dict:
    try:
        semantics = json.loads(SEMANTIC_NAMES_PATH.read_text())
    except (OSError, json.JSONDecodeError) as exc:
        raise RuntimeError(f"cannot load {SEMANTIC_NAMES_PATH}: {exc}") from exc
    if semantics.get("format_version") != 1:
        raise RuntimeError("unsupported BRE semantic-name manifest version")
    return semantics


SEMANTIC_NAMES = load_semantic_names()


# These are stable public-analysis names, not symbols recovered from BRE.
RESIDENT_NAMES = {
    (0x0FD0, 0x028A): ("get_mem", ["rtl", "heap"]),
    (0x0FD0, 0x029F): ("free_mem", ["rtl", "heap"]),
    (0x0FD0, 0x04ED): ("get_io_result", ["rtl", "io"]),
    (0x0FD0, 0x04F4): ("check_io_result", ["rtl", "io", "error-check"]),
    (0x0FD0, 0x0530): ("check_stack_space", ["rtl", "stack", "error-check"]),
    (0x0FD0, 0x075A): ("text_write_padding", ["rtl", "text-io"]),
    (0x0FD0, 0x07A9): ("text_write_buffer", ["rtl", "text-io"]),
    (0x0FD0, 0x0800): ("text_read_line", ["rtl", "text-io"]),
    (0x0FD0, 0x0840): ("text_write_line", ["rtl", "text-io"]),
    (0x0FD0, 0x0861): ("text_write_end", ["rtl", "text-io"]),
    (0x0FD0, 0x087C): ("text_refill_input_buffer", ["rtl", "text-io", "internal"]),
    (0x0FD0, 0x088A): ("text_flush_output_buffer", ["rtl", "text-io", "internal"]),
    (0x0FD0, 0x0898): ("text_read_char", ["rtl", "text-io"]),
    (0x0FD0, 0x08DE): ("text_write_char", ["rtl", "text-io"]),
    (0x0FD0, 0x0929): ("text_read_shortstring", ["rtl", "text-io", "shortstring"]),
    (0x0FD0, 0x0964): ("text_write_shortstring", ["rtl", "text-io", "shortstring"]),
    (0x0FD0, 0x0990): ("text_read_i32", ["rtl", "text-io", "integer"]),
    (0x0FD0, 0x09EC): ("text_write_i32", ["rtl", "text-io", "integer"]),
    (0x0FD0, 0x0A20): ("text_read_real48", ["rtl", "text-io", "real48"]),
    (0x0FD0, 0x0B14): ("assign_file", ["rtl", "file-io"]),
    (0x0FD0, 0x0B4F): ("reset_file", ["rtl", "file-io"]),
    (0x0FD0, 0x0B58): ("rewrite_file", ["rtl", "file-io"]),
    (0x0FD0, 0x0BD0): ("close_file", ["rtl", "file-io"]),
    (0x0FD0, 0x0C04): ("read_file_record", ["rtl", "file-io"]),
    (0x0FD0, 0x0C3A): ("block_read", ["rtl", "file-io"]),
    (0x0FD0, 0x0C41): ("block_write", ["rtl", "file-io"]),
    (0x0FD0, 0x0CA2): ("seek_file", ["rtl", "file-io"]),
    (0x0FD0, 0x0CD2): ("erase_file", ["rtl", "file-io"]),
    (0x0FD0, 0x0E60): ("make_directory", ["rtl", "file-io"]),
    (0x0C03, 0x0ED0): ("random_u16", ["rng"]),
    (0x0C03, 0x0F10): ("add_i32_indirect", ["integer", "rtl"]),
    (0x0FD0, 0x0ECC): ("mul_i32", ["integer", "rtl"]),
    (0x0FD0, 0x0F09): ("div_i32", ["integer", "rtl"]),
    (0x0FD0, 0x0FAF): ("shift_right_i32", ["integer", "rtl"]),
    (0x0FD0, 0x0FD2): ("shift_left_i32", ["integer", "rtl"]),
    (0x0FD0, 0x0FF5): ("shortstring_load", ["shortstring", "rtl"]),
    (0x0FD0, 0x100F): ("shortstring_store", ["shortstring", "rtl"]),
    (0x0FD0, 0x1033): ("shortstring_copy", ["shortstring", "rtl"]),
    (0x0FD0, 0x1074): ("shortstring_concat", ["shortstring", "rtl"]),
    (0x0FD0, 0x10A0): ("shortstring_position", ["shortstring", "rtl"]),
    (0x0FD0, 0x10E6): ("shortstring_compare", ["shortstring", "rtl"]),
    (0x0FD0, 0x1111): ("shortstring_from_char", ["shortstring", "rtl"]),
    (0x0FD0, 0x113E): ("shortstring_insert", ["shortstring", "rtl"]),
    (0x0FD0, 0x119D): ("shortstring_delete", ["shortstring", "rtl"]),
    (0x0FD0, 0x1841): ("real_sqrt", ["real48", "rtl"]),
    (0x0FD0, 0x1C27): ("random_bounded_u16", ["rng", "rtl"]),
    (0x0FD0, 0x1C44): ("random_real", ["rng", "real48", "rtl"]),
    (0x0FD0, 0x20F3): ("format_i32_width", ["integer", "shortstring", "rtl"]),
    (0x0FD0, 0x2186): ("file_size_records", ["rtl", "file-io"]),
    (0x0FD0, 0x221F): ("fill_char", ["rtl", "memory"]),
    (0x0FD0, 0x0116): ("runtime_halt", ["rtl", "non-returning"]),
    (0x0FD0, 0x1768): ("real_add", ["real48", "rtl"]),
    (0x0FD0, 0x176E): ("real_subtract", ["real48", "rtl"]),
    (0x0FD0, 0x1774): ("real_square", ["real48", "rtl"]),
    (0x0FD0, 0x177A): ("real_multiply", ["real48", "rtl"]),
    (0x0FD0, 0x1780): ("real_divide", ["real48", "rtl"]),
    (0x0FD0, 0x178A): ("real_compare", ["real48", "rtl"]),
    (0x0FD0, 0x178E): ("integer_to_real", ["real48", "rtl"]),
    (0x0FD0, 0x1792): ("real_to_integer_truncate", ["real48", "rtl"]),
    (0x0FD0, 0x179A): ("real_to_integer_round", ["real48", "rtl"]),
    (0x0FD0, 0x193E): ("real_ln", ["real48", "rtl"]),
    (0x0FD0, 0x19E7): ("real_exp", ["real48", "rtl"]),
    (0x0851, 0x0288): ("format_i32_grouped", ["format"]),
    (0x056D, 0x0EC6): ("total_regions", ["empire"]),
    (0x056D, 0x0F43): ("net_worth", ["empire", "score"]),
    (0x056D, 0x1A07): ("technology_factor", ["technology"]),
    (0x056D, 0x19B5): ("is_under_protection", ["protection"]),
}

RTL_SEMANTIC_EVIDENCE = {
    address: (
        "high",
        "Turbo Pascal System unit runtime entry corroborated by instruction behavior",
    )
    for address in {
        (0x0FD0, 0x028A),
        (0x0FD0, 0x029F),
        (0x0FD0, 0x04ED),
        (0x0FD0, 0x04F4),
        (0x0FD0, 0x0530),
        (0x0FD0, 0x075A),
        (0x0FD0, 0x07A9),
        (0x0FD0, 0x0800),
        (0x0FD0, 0x0840),
        (0x0FD0, 0x0861),
        (0x0FD0, 0x087C),
        (0x0FD0, 0x088A),
        (0x0FD0, 0x0898),
        (0x0FD0, 0x08DE),
        (0x0FD0, 0x0929),
        (0x0FD0, 0x0964),
        (0x0FD0, 0x0990),
        (0x0FD0, 0x09EC),
        (0x0FD0, 0x0A20),
        (0x0FD0, 0x0B14),
        (0x0FD0, 0x0B4F),
        (0x0FD0, 0x0B58),
        (0x0FD0, 0x0BD0),
        (0x0FD0, 0x0C04),
        (0x0FD0, 0x0C3A),
        (0x0FD0, 0x0C41),
        (0x0FD0, 0x0CA2),
        (0x0FD0, 0x0CD2),
        (0x0FD0, 0x0E60),
        (0x0FD0, 0x0FAF),
        (0x0FD0, 0x0FD2),
        (0x0FD0, 0x0FF5),
        (0x0FD0, 0x100F),
        (0x0FD0, 0x1033),
        (0x0FD0, 0x1074),
        (0x0FD0, 0x10A0),
        (0x0FD0, 0x10E6),
        (0x0FD0, 0x1111),
        (0x0FD0, 0x113E),
        (0x0FD0, 0x119D),
        (0x0FD0, 0x1841),
        (0x0FD0, 0x1C27),
        (0x0FD0, 0x1C44),
        (0x0FD0, 0x20F3),
        (0x0FD0, 0x2186),
        (0x0FD0, 0x221F),
    }
}

NON_RETURNING_FAR_TARGETS = {(0x0FD0, 0x0116)}


# Closed target sets for every indirect control transfer reached in BRE 0.988.
# OVR sites and targets are canonical file offsets; EXE values are canonical
# load-module offsets. The evidence is intentionally structural rather than a
# transcription of private program text.
CALCULATED_RESIDENT_ROOTS = {
    (0x025D, 0x0CD8): ("resident_025d_0cd8", ["callback"]),
    (0x025D, 0x0D43): ("resident_025d_0d43", ["callback"]),
    (0x09BD, 0x026C): ("text_driver_close", ["rtl", "text-io", "callback"]),
    (0x09BD, 0x02A6): ("text_driver_read", ["rtl", "text-io", "callback"]),
    (0x09BD, 0x039F): ("text_driver_write", ["rtl", "text-io", "callback"]),
    (0x09BD, 0x0554): ("text_driver_flush", ["rtl", "text-io", "callback"]),
    (0x09BD, 0x0568): ("text_driver_open", ["rtl", "text-io", "callback"]),
    (0x0F5B, 0x0268): ("load_overlay_from_file", ["overlay-loader", "callback"]),
    (0x0F5B, 0x02E5): ("overlay_callback_noop", ["overlay-loader", "callback"]),
    (0x0F5B, 0x06E0): ("load_overlay_from_ems", ["overlay-loader", "callback"]),
    (0x0FD0, 0x00D6): ("default_heap_error_handler", ["rtl", "heap", "callback"]),
    (0x0FD0, 0x081E): ("scan_text_line_end", ["rtl", "text-io", "callback"]),
    (0x0FD0, 0x094C): ("scan_text_shortstring", ["rtl", "text-io", "callback"]),
    (0x0FD0, 0x09C7): ("scan_text_i32", ["rtl", "text-io", "callback"]),
    (0x0FD0, 0x0A59): ("scan_text_real48", ["rtl", "text-io", "callback"]),
    (0x0FD0, 0x0AEF): ("scan_text_character", ["rtl", "text-io", "callback"]),
}

CALCULATED_TRANSFERS = [
    {
        "key": "network_record_normalizer",
        "storage": "ovr",
        "sites": [0x0036EA, 0x003751],
        "targets": [("ovr", 0x003477)],
        "model": "far procedure parameter",
        "evidence": "all seven direct callers pass the same overlay procedure address",
    },
    {
        "key": "problem_report_handler",
        "storage": "ovr",
        "sites": [0x03E819, 0x03E83B, 0x0519C0],
        "targets": [("ovr", 0x048C81), ("ovr", 0x0519EE)],
        "model": "global far-pointer slot",
        "evidence": "the complete reachable write set contains two constant procedure addresses",
    },
    {
        "key": "typed_file_error_handler",
        "storage": "ovr",
        "sites": [0x055253, 0x0552F9, 0x0553DF, 0x055571, 0x05569D, 0x055782],
        "targets": [("ovr", 0x0558DE), ("ovr", 0x055A0E)],
        "model": "global far-pointer slot",
        "evidence": "the initializer and its sole setter call provide two constant assignments",
    },
    {
        "key": "idle_callback",
        "storage": "exe",
        "sites": [0x09DDD],
        "targets": [("exe", 0x032A8)],
        "model": "global far-pointer slot",
        "evidence": "the sole setter has one direct call site with a constant procedure argument",
    },
    {
        "key": "multitasker_callbacks",
        "storage": "exe",
        "sites": [0x09D31],
        "targets": [
            ("exe", 0x03313),
            ("ovr", 0x007584),
            ("ovr", 0x058911),
            ("ovr", 0x058924),
            ("ovr", 0x05893A),
        ],
        "model": "heap-linked callback records",
        "evidence": "all five calls to the sole record constructor pass constant procedure addresses",
    },
    {
        "key": "text_output_idle_callback",
        "storage": "exe",
        "sites": [0x09FA4],
        "targets": [("exe", 0x032A8)],
        "model": "global far-pointer slot",
        "evidence": "the sole setter has one direct call site with the same constant idle callback",
    },
    {
        "key": "overlay_notification",
        "storage": "exe",
        "sites": [0x0F8DA],
        "targets": [("exe", 0x0F895)],
        "model": "global far-pointer slot",
        "evidence": "zero-initialized storage receives one constant default callback",
    },
    {
        "key": "overlay_reader",
        "storage": "exe",
        "sites": [0x0F95A, 0x0FC24],
        "targets": [("exe", 0x0F818), ("exe", 0x0FC90)],
        "model": "global far-pointer slot",
        "evidence": "loader initialization selects exactly the file and EMS reader procedures",
    },
    {
        "key": "heap_error_handler",
        "storage": "exe",
        "sites": [0x10102, 0x1010E],
        "targets": [("exe", 0x0FDD6)],
        "model": "global far-pointer slot",
        "evidence": "runtime startup installs one constant handler and no other reachable write exists",
    },
    {
        "key": "text_driver_methods",
        "storage": "exe",
        "sites": [0x10361],
        "targets": [
            ("exe", 0x09E3C),
            ("exe", 0x09E76),
            ("exe", 0x09F6F),
            ("exe", 0x0A138),
        ],
        "model": "TextRec method fields selected by a bounded field offset",
        "evidence": "three callers select fields 0x10, 0x14, or 0x1c and initialization writes four constant methods",
    },
    {
        "key": "text_scanners",
        "storage": "exe",
        "sites": [0x10427],
        "targets": [
            ("exe", 0x1051E),
            ("exe", 0x1064C),
            ("exe", 0x106C7),
            ("exe", 0x10759),
            ("exe", 0x107EF),
        ],
        "model": "near procedure argument in AX",
        "evidence": "all five direct callers load AX with a constant scanner entry",
    },
    {
        "key": "text_input_method",
        "storage": "exe",
        "sites": [0x1057E],
        "targets": [("exe", 0x09E76), ("exe", 0x09F6F)],
        "model": "TextRec InOutFunc field",
        "evidence": "text-open initialization assigns exactly two constant input methods",
    },
    {
        "key": "text_flush_method",
        "storage": "exe",
        "sites": [0x1058C],
        "targets": [("exe", 0x09F6F), ("exe", 0x0A124)],
        "model": "TextRec FlushFunc field",
        "evidence": "text-open initialization assigns exactly two constant flush methods",
    },
]


# Named navigation landmarks already cited in this repository. A landmark may
# be data or an interior instruction, so it is deliberately not promoted to a
# procedure root unless it equals a proven control-flow target.
LANDMARKS = {
    0x0D08A: ("end_of_turn", ["turn", "economy"]),
    0x12633: ("market_price_walk", ["market", "price"]),
    0x12D30: ("region_purchase_allowance", ["regions"]),
    0x16AEB: ("sell_agent_price", ["agents", "market"]),
    0x17957: ("covert_agent_debit", ["covert", "agents"]),
    0x18280: ("queen_refund", ["queen", "tax"]),
    0x1C0E3: ("diplomacy_relation_dispatch", ["diplomacy"]),
    0x1DC0E: ("local_message_reader_strings", ["messages", "local"]),
    0x1F94C: ("interbbs_message_reader_strings", ["messages", "interbbs"]),
    0x23425: ("planetary_diplomacy", ["diplomacy", "interbbs"]),
    0x277A0: ("special_operation_funding", ["covert", "cost"]),
    0x2EA09: ("calculate_crown_tax", ["queen", "tax", "economy"]),
    0x2F4C4: ("popular_support_boost", ["support", "economy"]),
    0x2F740: ("popular_support_boost_scaling", ["support", "economy"]),
    0x2FAF1: ("crown_tax", ["queen", "tax"]),
    0x32D1B: ("technology_report", ["technology", "report"]),
    0x342C0: ("region_gold_income", ["regions", "economy"]),
    0x34F49: ("industrial_production", ["production", "economy"]),
    0x35DB5: ("pirate_raid_chance", ["pirates", "rng"]),
    0x35E30: ("pirate_spoil_selection", ["pirates", "rng"]),
    0x35F66: ("pirate_take", ["pirates"]),
    0x3671B: ("pirate_strength", ["pirates", "combat"]),
    0x445B0: ("interbbs_time_file", ["interbbs", "time"]),
    0x4A81C: ("combat_odds", ["combat"]),
    0x4BA48: ("send_spy", ["covert", "spy"]),
    0x4CAB7: ("covert_resolution", ["covert"]),
}


class BREError(RuntimeError):
    pass


def parse_int(value: str) -> int:
    try:
        return int(value, 0)
    except ValueError as exc:
        raise argparse.ArgumentTypeError(f"not an integer: {value}") from exc


def hx(value: int, width: int = 4) -> str:
    return f"0x{value:0{width}x}"


def naming_metadata(
    status: str,
    confidence: str,
    evidence: str,
) -> dict:
    return {
        "status": status,
        "confidence": confidence,
        "evidence": evidence,
    }


UNKNOWN_NAMING = naming_metadata(
    "unclassified",
    "none",
    "address-derived fallback; behavior has not yet been established",
)


def semantic_annotation(section: str, key: str) -> dict | None:
    return SEMANTIC_NAMES.get(section, {}).get(key)


def annotation_naming(annotation: dict) -> dict:
    return naming_metadata(
        "identified",
        annotation["confidence"],
        "; ".join(annotation["evidence"]),
    )


def annotation_aliases(annotation: dict | None) -> set[str]:
    if not annotation:
        return set()
    return set(annotation.get("aliases", []))


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def verify_one(path: Path, kind: str) -> None:
    want = EXPECTED[kind]
    if not path.is_file():
        raise BREError(f"missing {path}")
    size = path.stat().st_size
    digest = sha256(path)
    if size != want["size"] or digest != want["sha256"]:
        raise BREError(
            f"{path} is not the supported BRE 0.988 {want['name']}: "
            f"size={size}, sha256={digest}"
        )


def binary_paths(args: argparse.Namespace) -> tuple[Path, Path]:
    if getattr(args, "directory", None):
        base = Path(args.directory)
        return base / "BRE.EXE", base / "BRE.OVR"
    return Path(args.exe), Path(args.ovr)


@dataclasses.dataclass(frozen=True)
class MZHeader:
    header_size: int
    file_size: int
    relocation_count: int
    entry_segment: int
    entry_offset: int
    stack_segment: int
    stack_offset: int


@dataclasses.dataclass(frozen=True)
class Stub:
    index: int
    file_offset: int
    entry_offset: int


@dataclasses.dataclass(frozen=True)
class Unit:
    index: int
    descriptor_file_offset: int
    descriptor_segment: int
    ovr_offset: int
    code_size: int
    fixup_size: int
    previous_segment: int
    stubs: tuple[Stub, ...]
    fixups: tuple[int, ...]

    @property
    def fixup_offset(self) -> int:
        return self.ovr_offset + self.code_size

    @property
    def end_offset(self) -> int:
        return self.fixup_offset + self.fixup_size

    @property
    def unit_id(self) -> str:
        return f"ovr_{self.ovr_offset:06x}"


def parse_mz(exe: bytes) -> MZHeader:
    if len(exe) < 0x1C or exe[:2] != b"MZ":
        raise BREError("BRE.EXE is not an MZ executable")
    fields = struct.unpack_from("<14H", exe, 0)
    last_page, pages = fields[1], fields[2]
    file_size = (pages - 1) * 512 + (last_page or 512)
    return MZHeader(
        header_size=fields[4] * 16,
        file_size=file_size,
        relocation_count=fields[3],
        entry_segment=fields[11],
        entry_offset=fields[10],
        stack_segment=fields[7],
        stack_offset=fields[8],
    )


def parse_units(exe: bytes, ovr: bytes, mz: MZHeader) -> list[Unit]:
    candidates = []
    for pos in range(mz.header_size, len(exe) - 0x20, 16):
        if exe[pos : pos + 4] != b"\xcd\x3f\x00\x00":
            continue
        ovr_offset, code_size, fixup_size, count, previous = struct.unpack_from(
            "<IHHHH", exe, pos + 4
        )
        stub_end = pos + 0x20 + count * 5
        if not count or not code_size or fixup_size % 2:
            continue
        if ovr_offset < 8 or ovr_offset + code_size + fixup_size > len(ovr):
            continue
        if stub_end > len(exe):
            continue
        stubs = []
        for number in range(count):
            stub_pos = pos + 0x20 + number * 5
            if exe[stub_pos : stub_pos + 2] != b"\xcd\x3f" or exe[stub_pos + 4] != 0:
                stubs = []
                break
            stubs.append(
                Stub(number, stub_pos, struct.unpack_from("<H", exe, stub_pos + 2)[0])
            )
        if not stubs or any(stub.entry_offset >= code_size for stub in stubs):
            continue
        segment = (pos - mz.header_size) // 16
        candidates.append(
            (pos, segment, ovr_offset, code_size, fixup_size, previous, tuple(stubs))
        )

    candidates.sort(key=lambda item: item[2])
    if not candidates:
        raise BREError("no overlay descriptors found")
    units = []
    expected_offset = 8
    previous_segment = 0
    for index, candidate in enumerate(candidates):
        pos, segment, offset, code_size, fixup_size, previous, stubs = candidate
        if offset != expected_offset:
            raise BREError(
                f"overlay chain breaks before descriptor {hx(pos, 6)}: "
                f"expected {hx(expected_offset, 6)}, found {hx(offset, 6)}"
            )
        if previous != previous_segment:
            raise BREError(
                f"descriptor {hx(pos, 6)} links to {hx(previous)}, "
                f"expected {hx(previous_segment)}"
            )
        stream = ovr[offset + code_size : offset + code_size + fixup_size]
        fixups = struct.unpack("<" + "H" * (fixup_size // 2), stream)
        if any(fixup + 1 >= code_size for fixup in fixups):
            raise BREError(f"out-of-range fixup in {hx(offset, 6)}")
        if tuple(sorted(set(fixups))) != fixups:
            raise BREError(f"unordered or duplicate fixup in {hx(offset, 6)}")
        units.append(
            Unit(
                index,
                pos,
                segment,
                offset,
                code_size,
                fixup_size,
                previous,
                stubs,
                tuple(fixups),
            )
        )
        expected_offset = offset + code_size + fixup_size
        previous_segment = segment
    if expected_offset != len(ovr):
        raise BREError(
            f"overlay chain ends at {hx(expected_offset, 6)}, "
            f"file ends at {hx(len(ovr), 6)}"
        )
    return units


def load_release(args: argparse.Namespace, check_hashes: bool = True):
    exe_path, ovr_path = binary_paths(args)
    if check_hashes:
        verify_one(exe_path, "exe")
        verify_one(ovr_path, "ovr")
    exe = exe_path.read_bytes()
    ovr = ovr_path.read_bytes()
    mz = parse_mz(exe)
    units = parse_units(exe, ovr, mz)
    return exe_path, ovr_path, exe, ovr, mz, units


def select_unit(units: list[Unit], selector: str) -> Unit:
    lowered = selector.lower()
    if lowered.startswith("ovr_"):
        lowered = lowered[4:]
    try:
        value = int(lowered, 16 if not lowered.startswith("0x") else 0)
    except ValueError:
        value = -1
    for unit in units:
        if selector == unit.unit_id or value in {
            unit.index,
            unit.ovr_offset,
            unit.descriptor_segment,
        }:
            return unit
    raise BREError(f"no unit matches {selector!r}")


def root_name(unit: Unit, entry: int) -> str:
    absolute = unit.ovr_offset + entry
    annotation = semantic_annotation("overlay_procedures", hx(absolute, 6))
    if annotation:
        return annotation["name"]
    if absolute in LANDMARKS:
        return LANDMARKS[absolute][0]
    return f"{unit.unit_id}_entry_{entry:04x}"


def edge_target_name(target: str) -> str:
    """Return the stable location name from an optional unit-qualified edge."""
    if target.startswith("ovr_") and ":" in target:
        return target.split(":", 1)[1]
    return target


NDISASM_LINE = re.compile(r"^([0-9A-Fa-f]{8})\s+")


def capstone_module():
    """Load Capstone, including the normal FreeBSD /usr/local/lib location."""
    try:
        import capstone

        return capstone
    except ImportError as first_error:
        library = Path("/usr/local/lib/libcapstone.so")
        if library.exists() and not os.environ.get("LIBCAPSTONE_PATH"):
            os.environ["LIBCAPSTONE_PATH"] = str(library.parent)
            sys.modules.pop("capstone", None)
            try:
                import capstone

                return capstone
            except ImportError:
                pass
        raise BREError(
            "reachable-span analysis requires Capstone 5 Python bindings and "
            "its native library (set LIBCAPSTONE_PATH when it is nonstandard)"
        ) from first_error


def ranges_from_instructions(
    instructions: dict[int, object], visited: set[int]
) -> list[list[int]]:
    byte_ranges = sorted((off, off + instructions[off].size) for off in visited)
    merged: list[list[int]] = []
    for start, end in byte_ranges:
        if merged and start <= merged[-1][1]:
            merged[-1][1] = max(merged[-1][1], end)
        else:
            merged.append([start, end])
    return merged


def collect_procedure_bodies(
    roots: set[int], visited: set[int], successors: dict[int, set[int]]
) -> dict[int, set[int]]:
    """Collect intraprocedural instruction membership from each proven entry.

    Calls are deliberately not successors; conditional/unconditional branches
    and ordinary fallthrough are. Shared tails can therefore belong to more
    than one procedure without forcing a false single-owner decision.
    """
    bodies = {}
    for root in sorted(roots & visited):
        members = set()
        pending = [root]
        while pending:
            offset = pending.pop()
            if offset in members or offset not in visited:
                continue
            members.add(offset)
            pending.extend(successors.get(offset, ()))
        bodies[root] = members
    return bodies


def find_cs_pointer_references(
    instructions: dict[int, object], candidate_ranges: list[list[int]]
) -> list[tuple[int, int, str]]:
    """Find compiler-shaped far pointers into same-segment unreached spans.

    Turbo Pascal commonly emits ``mov reg, offset; push cs; push reg`` when
    passing a Pascal string or other code-segment datum. A direct
    ``push cs; push imm`` form is also accepted. Merely seeing an immediate
    that happens to resemble an address is intentionally not enough.
    """
    capstone = capstone_module()
    from capstone.x86 import X86_OP_IMM, X86_OP_REG, X86_REG_CS

    def is_candidate(target: int) -> bool:
        return any(start <= target < end for start, end in candidate_ranges)

    references = set()
    ordered = sorted(instructions)
    positions = {offset: index for index, offset in enumerate(ordered)}
    for source in ordered:
        instruction = instructions[source]
        operands = instruction.operands
        if (
            instruction.mnemonic.lower() == "mov"
            and len(operands) == 2
            and operands[0].type == X86_OP_REG
            and operands[1].type == X86_OP_IMM
        ):
            pointer_register = operands[0].reg
            target = operands[1].imm & 0xFFFF
            saw_cs = saw_pointer = False
            cursor = source + instruction.size
            for _step in range(3):
                following = instructions.get(cursor)
                if following is None:
                    break
                if following.mnemonic.lower() == "push" and len(following.operands) == 1:
                    operand = following.operands[0]
                    saw_cs |= operand.type == X86_OP_REG and operand.reg == X86_REG_CS
                    saw_pointer |= (
                        operand.type == X86_OP_REG and operand.reg == pointer_register
                    )
                if following.group(capstone.CS_GRP_JUMP) or following.group(
                    capstone.CS_GRP_RET
                ):
                    break
                cursor += following.size
            if saw_cs and saw_pointer and is_candidate(target):
                references.add((source, target, "cs_offset_register_pair"))

        if (
            instruction.mnemonic.lower() == "push"
            and len(operands) == 1
            and operands[0].type == X86_OP_REG
            and operands[0].reg == X86_REG_CS
        ):
            index = positions[source]
            if index + 1 >= len(ordered):
                continue
            following = instructions[ordered[index + 1]]
            if ordered[index + 1] != source + instruction.size:
                continue
            if (
                following.mnemonic.lower() == "push"
                and len(following.operands) == 1
                and following.operands[0].type == X86_OP_IMM
            ):
                target = following.operands[0].imm & 0xFFFF
                if is_candidate(target):
                    references.add((source, target, "cs_immediate_pair"))
    return sorted(references)


def analyze_cfg(
    code: bytes,
    roots: set[int],
    descriptor_roots: dict[tuple[int, int], tuple[str, str]],
) -> dict:
    capstone = capstone_module()
    from capstone.x86 import X86_OP_IMM

    decoder = capstone.Cs(capstone.CS_ARCH_X86, capstone.CS_MODE_16)
    decoder.detail = True
    instructions = {}
    visited: set[int] = set()
    byte_owners = {}
    queue = list(sorted(roots))
    edges = set()
    call_targets = set()
    block_targets = set(roots)
    target_sources = {root: set() for root in roots}
    direct_edges = set()
    successors = {}
    unresolved = set()
    conflicts = set()
    while queue:
        offset = queue.pop()
        if offset in visited or not (0 <= offset < len(code)):
            continue
        owner = byte_owners.get(offset)
        if owner is not None and owner != offset:
            conflicts.add((offset, owner))
            continue
        decoded = next(decoder.disasm(code[offset : offset + 15], offset, count=1), None)
        if decoded is None:
            unresolved.add((offset, "undecodable byte sequence"))
            continue
        overlapping = {
            byte_owners[byte]
            for byte in range(offset, offset + decoded.size)
            if byte in byte_owners and byte_owners[byte] != offset
        }
        if overlapping:
            conflicts.update((offset, existing) for existing in overlapping)
            continue
        instructions[offset] = decoded
        visited.add(offset)
        successors.setdefault(offset, set())
        for byte in range(offset, offset + decoded.size):
            byte_owners[byte] = offset

        mnemonic = decoded.mnemonic.lower()
        is_call = decoded.group(capstone.CS_GRP_CALL)
        is_jump = decoded.group(capstone.CS_GRP_JUMP)
        immediate_operands = [operand.imm for operand in decoded.operands if operand.type == X86_OP_IMM]
        far_target = None
        near_target = None
        if (is_call or is_jump) and mnemonic in {"lcall", "callf", "ljmp", "jmpf"} and len(immediate_operands) == 2:
            far_target = (immediate_operands[0] & 0xFFFF, immediate_operands[1] & 0xFFFF)
        elif (is_call or is_jump) and len(decoded.operands) == 1 and immediate_operands:
            near_target = immediate_operands[0] & 0xFFFF

        if near_target is not None:
            block_targets.add(near_target)
            if is_call:
                call_targets.add(near_target)
                target_kind = "near_call"
            elif mnemonic in {"jmp", "ljmp"}:
                target_kind = "unconditional_jump"
            else:
                target_kind = "conditional_jump"
            target_sources.setdefault(near_target, set()).add((offset, target_kind))
            direct_edges.add((offset, target_kind, near_target))
            if near_target < len(code):
                queue.append(near_target)
                if not is_call:
                    successors[offset].add(near_target)
            else:
                unresolved.add((offset, f"target {hx(near_target)} is outside unit"))
        elif far_target is not None:
            segment, destination = far_target
            resolved = descriptor_roots.get((segment, destination))
            edge_kind = "overlay_call" if is_call else "overlay_jump"
            if resolved:
                edges.add(
                    (
                        offset,
                        edge_kind,
                        f"{resolved[0]}:{resolved[1]}",
                        f"{segment:04x}:{destination:04x}",
                    )
                )
            else:
                known = RESIDENT_NAMES.get((segment, destination))
                label = known[0] if known else f"resident_{segment:04x}_{destination:04x}"
                edges.add(
                    (
                        offset,
                        "far_call" if is_call else "far_jump",
                        label,
                        f"{segment:04x}:{destination:04x}",
                    )
                )
        elif is_call or is_jump:
            unresolved.add((offset, f"{decoded.mnemonic} {decoded.op_str}".strip()))

        unconditional_jump = is_jump and mnemonic in {"jmp", "ljmp"}
        returns = decoded.group(capstone.CS_GRP_RET) or mnemonic == "iret"
        non_returning_call = is_call and far_target in NON_RETURNING_FAR_TARGETS
        falls_through = not (
            unconditional_jump
            or returns
            or non_returning_call
            or mnemonic in {"int3", "hlt"}
        )
        if falls_through and offset + decoded.size < len(code):
            fallthrough = offset + decoded.size
            queue.append(fallthrough)
            successors[offset].add(fallthrough)
            if is_jump:
                block_targets.add(fallthrough)
                target_sources.setdefault(fallthrough, set()).add(
                    (offset, "conditional_fallthrough")
                )

    ranges = ranges_from_instructions(instructions, visited)
    procedure_roots = set(roots) | {target for target in call_targets if target in visited}
    procedure_bodies = collect_procedure_bodies(
        procedure_roots, visited, successors
    )
    block_starts = {target for target in block_targets if target in visited}
    block_spans = {}
    block_terminators = {}
    for start in sorted(block_starts):
        cursor = start
        end = start
        seen = set()
        while cursor in visited and cursor not in seen:
            if cursor != start and cursor in block_starts:
                break
            seen.add(cursor)
            instruction = instructions[cursor]
            end = cursor + instruction.size
            mnemonic = instruction.mnemonic.lower()
            if (
                mnemonic == "jmp"
                or (mnemonic.startswith("j") and mnemonic != "jmp")
                or mnemonic.startswith("loop")
                or mnemonic.startswith("ret")
                or mnemonic in {"iret", "int3", "hlt"}
            ):
                break
            cursor = end
        block_spans[start] = end
        block_terminators[start] = instruction.mnemonic.lower()
    holes = []
    cursor = 0
    for start, end in ranges:
        if cursor < start:
            holes.append([cursor, start])
        cursor = max(cursor, end)
    if cursor < len(code):
        holes.append([cursor, len(code)])
    data_references = find_cs_pointer_references(instructions, holes)
    grouped_edges = {}
    for offset, kind, target, logical_target in edges:
        grouped_edges.setdefault((kind, target, logical_target), []).append(offset)
    return {
        "reachable_ranges": [[hx(a), hx(b)] for a, b in ranges],
        "unreached_ranges": [[hx(a), hx(b)] for a, b in holes],
        "external_edges": [
            {
                "kind": kind,
                "to": target,
                "logical_target": logical_target,
                "sites": [hx(site) for site in sorted(sites)],
            }
            for (kind, target, logical_target), sites in sorted(
                grouped_edges.items(), key=lambda item: (item[0][0], str(item[0][1]))
            )
        ],
        "unresolved_transfers": [
            {"at": hx(offset), "instruction": text}
            for offset, text in sorted(unresolved)
        ],
        "decode_conflicts": [
            {"target": hx(target), "inside_instruction_at": hx(owner)}
            for target, owner in sorted(conflicts)
        ],
        "direct_edges": [
            {"at": hx(source), "kind": kind, "to": hx(target)}
            for source, kind, target in sorted(direct_edges)
        ],
        "data_references": [
            {"at": hx(source), "to": hx(target), "kind": kind}
            for source, target, kind in data_references
        ],
        "_procedure_roots": sorted(procedure_roots),
        "_procedure_bodies": procedure_bodies,
        "_procedure_body_ranges": {
            root: ranges_from_instructions(instructions, members)
            for root, members in procedure_bodies.items()
        },
        "_blocks": {
            start: {
                "end": block_spans[start],
                "terminator": block_terminators[start],
                "sources": sorted(target_sources.get(start, set())),
            }
            for start in sorted(block_starts)
        },
    }


def classify_unreached(data: bytes) -> str:
    if data and not any(data):
        return "zero_fill"
    if data and all(byte == 0x90 for byte in data):
        return "nop_padding"
    if data and all(byte == 0xCC for byte in data):
        return "breakpoint_padding"
    return "unreached_data_or_indirect_code"


def looks_like_pascal_string(data: bytes, offset: int, end: int) -> bool:
    payload = pascal_string_payload(data, offset, end)
    if payload is None:
        return False
    display_bytes = sum(
        byte in {9, 10, 13, 27} or 32 <= byte < 127 for byte in payload
    )
    return display_bytes * 5 >= len(payload) * 4


def pascal_string_payload(data: bytes, offset: int, end: int) -> bytes | None:
    if not (0 <= offset < end <= len(data)):
        return None
    length = data[offset]
    if length == 0 or offset + 1 + length > end:
        return None
    payload = data[offset + 1 : offset + 1 + length]
    display_bytes = sum(
        byte in {9, 10, 13, 27} or byte >= 32 for byte in payload
    )
    return payload if display_bytes * 5 >= length * 4 else None


def durable_id(kind: str, storage: str, offset: int) -> str:
    width = 6 if storage == "ovr" else 5
    return f"bre0988:{storage}:{kind}:{offset:0{width}x}"


def string_records(
    storage: str,
    data: bytes,
    procedures: list[dict],
    blocks: list[dict],
    chunks: list[dict],
    references: list[tuple[int, int, str]],
    base_offset: int = 0,
) -> list[dict]:
    """Index directly referenced Pascal strings without retaining their text."""
    block_span_key = "unit_span" if storage == "ovr" else "load_span"
    chunk_span_key = "unit_span" if storage == "ovr" else "load_span"
    grouped: dict[int, dict[tuple[str, str, tuple[str, ...]], set[int]]] = {}
    payloads: dict[int, bytes] = {}
    for source, target, kind in references:
        block = next(
            (
                candidate
                for candidate in blocks
                if int(candidate[block_span_key][0], 0)
                <= source
                < int(candidate[block_span_key][1], 0)
            ),
            None,
        )
        chunk = next(
            (
                candidate
                for candidate in chunks
                if int(candidate[chunk_span_key][0], 0)
                <= target
                < int(candidate[chunk_span_key][1], 0)
            ),
            None,
        )
        if block is None or chunk is None:
            continue
        procedure_ids = tuple(
            procedure["id"]
            for procedure in procedures
            if any(
                int(span[0], 0) <= source < int(span[1], 0)
                for span in procedure["body_ranges"]
            )
        )
        payload = pascal_string_payload(
            data, target, int(chunk[chunk_span_key][1], 0)
        )
        if payload is None:
            continue
        payloads[target] = payload
        grouped.setdefault(target, {}).setdefault(
            (block["id"], kind, procedure_ids), set()
        ).add(source)
    records = []
    for target, uses in sorted(grouped.items()):
        payload = payloads[target]
        absolute = base_offset + target
        address_key = "ovr_offset" if storage == "ovr" else "load_offset"
        records.append(
            {
                "id": durable_id("string", storage, absolute),
                "storage": storage,
                address_key: hx(absolute, 6 if storage == "ovr" else 5),
                "length": len(payload),
                "sha256": hashlib.sha256(payload).hexdigest(),
                "encoding": "cp437",
                "used_by": [
                    {
                        "block_id": block_id,
                        "procedure_ids": list(procedure_ids),
                        "kind": kind,
                        "sites": [
                            hx(base_offset + site, 6 if storage == "ovr" else 5)
                            for site in sorted(sites)
                        ],
                    }
                    for (block_id, kind, procedure_ids), sites in sorted(
                        uses.items()
                    )
                ],
            }
        )
    return records


def data_chunks_for_unit(unit: Unit, code: bytes, ranges: list[list[str]]) -> list[dict]:
    chunks = []
    for encoded_start, encoded_end in ranges:
        range_start, range_end = int(encoded_start, 0), int(encoded_end, 0)
        split_points = {range_start, range_end}
        for absolute in LANDMARKS:
            relative = absolute - unit.ovr_offset
            if range_start < relative < range_end:
                split_points.add(relative)
        points = sorted(split_points)
        for start, end in zip(points, points[1:]):
            absolute_start = unit.ovr_offset + start
            absolute_end = unit.ovr_offset + end
            landmark = LANDMARKS.get(absolute_start)
            name = landmark[0] if landmark else f"{unit.unit_id}_data_{start:04x}"
            contained_landmarks = [
                landmark_name
                for offset, (landmark_name, _tags) in sorted(LANDMARKS.items())
                if absolute_start <= offset < absolute_end and landmark_name != name
            ]
            tags = ["overlay", "data-chunk"]
            if landmark:
                tags.extend(landmark[1])
            chunks.append(
                {
                    "name": name,
                    "naming": (
                        naming_metadata(
                            "identified",
                            "high",
                            "existing repository-cited semantic landmark",
                        )
                        if landmark
                        else dict(UNKNOWN_NAMING)
                    ),
                    "unit_span": [hx(start), hx(end)],
                    "ovr_span": [hx(absolute_start, 6), hx(absolute_end, 6)],
                    "size": end - start,
                    "classification": classify_unreached(code[start:end]),
                    "aliases": contained_landmarks,
                    "tags": sorted(set(tags)),
                    "confidence": "proven-boundary",
                    "evidence": (
                        "bytes not decoded from any proven root, split at known landmarks; "
                        "may contain code reachable only through an unresolved transfer"
                    ),
                }
            )
    return chunks


def analyze_resident_image(
    image: bytes,
    mz: MZHeader,
    units: list[Unit],
    seed_targets: dict[tuple[int, int], tuple[str, list[str], str]],
    overlay_targets: dict[tuple[int, int], tuple[str, str]],
) -> dict:
    capstone = capstone_module()
    from capstone.x86 import X86_OP_IMM

    decoder = capstone.Cs(capstone.CS_ARCH_X86, capstone.CS_MODE_16)
    decoder.detail = True
    excluded = []
    for unit in units:
        start = unit.descriptor_file_offset - mz.header_size
        size = (0x20 + len(unit.stubs) * 5 + 15) & ~15
        excluded.append((start, min(start + size, len(image)), unit))

    def excluded_owner(offset: int):
        return next((item for item in excluded if item[0] <= offset < item[1]), None)

    instructions = {}
    visited_states = set()
    byte_owners = {}
    procedure_roots = set()
    block_targets = set()
    target_sources = {}
    direct_edges = set()
    successors = {}
    logical_addresses = {}
    seed_metadata = {}
    queue = []
    edges = set()
    unresolved = set()
    conflicts = set()

    for (segment, offset), (name, tags, evidence) in sorted(seed_targets.items()):
        linear = segment * 16 + offset
        if not (0 <= linear < len(image)):
            continue
        logical_addresses.setdefault(linear, set()).add((segment, offset))
        seed_metadata.setdefault(linear, []).append((name, tags, evidence, segment, offset))
        procedure_roots.add(linear)
        block_targets.add(linear)
        target_sources.setdefault(linear, set()).add((None, "seed"))
        queue.append((linear, segment))

    while queue:
        linear, cs_segment = queue.pop()
        state = (linear, cs_segment)
        if state in visited_states or not (0 <= linear < len(image)):
            continue
        visited_states.add(state)
        blocked = excluded_owner(linear)
        if blocked:
            unresolved.add(
                (linear, f"target enters overlay descriptor {blocked[2].unit_id}")
            )
            continue
        owner = byte_owners.get(linear)
        if owner is not None and owner != linear:
            conflicts.add((linear, owner))
            continue
        logical_offset = (linear - cs_segment * 16) & 0xFFFF
        decoded = next(
            decoder.disasm(image[linear : linear + 15], logical_offset, count=1), None
        )
        if linear not in instructions:
            if decoded is None:
                unresolved.add((linear, "undecodable byte sequence"))
                continue
            overlap_excluded = next(
                (
                    item
                    for byte in range(linear, linear + decoded.size)
                    if (item := excluded_owner(byte))
                ),
                None,
            )
            if overlap_excluded:
                unresolved.add(
                    (linear, f"instruction overlaps overlay descriptor {overlap_excluded[2].unit_id}")
                )
                continue
            overlapping = {
                byte_owners[byte]
                for byte in range(linear, linear + decoded.size)
                if byte in byte_owners and byte_owners[byte] != linear
            }
            if overlapping:
                conflicts.update((linear, existing) for existing in overlapping)
                continue
            instructions[linear] = decoded
            successors.setdefault(linear, set())
            for byte in range(linear, linear + decoded.size):
                byte_owners[byte] = linear

        elif decoded is None:
            unresolved.add((linear, "undecodable byte sequence through segment alias"))
            continue
        logical_addresses.setdefault(linear, set()).add((cs_segment, logical_offset))
        mnemonic = decoded.mnemonic.lower()
        is_call = decoded.group(capstone.CS_GRP_CALL)
        is_jump = decoded.group(capstone.CS_GRP_JUMP)
        immediate_operands = [
            operand.imm for operand in decoded.operands if operand.type == X86_OP_IMM
        ]
        far_target = None
        near_target = None
        if (
            (is_call or is_jump)
            and mnemonic in {"lcall", "callf", "ljmp", "jmpf"}
            and len(immediate_operands) == 2
        ):
            far_target = (
                immediate_operands[0] & 0xFFFF,
                immediate_operands[1] & 0xFFFF,
            )
        elif (is_call or is_jump) and len(decoded.operands) == 1 and immediate_operands:
            target_offset = immediate_operands[0] & 0xFFFF
            near_target = (cs_segment * 16 + target_offset, cs_segment, target_offset)

        if near_target is not None:
            target_linear, target_segment, target_offset = near_target
            kind = (
                "near_call"
                if is_call
                else "unconditional_jump"
                if mnemonic in {"jmp", "ljmp"}
                else "conditional_jump"
            )
            direct_edges.add(
                (
                    linear,
                    kind,
                    target_linear,
                    f"{target_segment:04x}:{target_offset:04x}",
                )
            )
            if 0 <= target_linear < len(image):
                block_targets.add(target_linear)
                target_sources.setdefault(target_linear, set()).add((linear, kind))
                logical_addresses.setdefault(target_linear, set()).add(
                    (target_segment, target_offset)
                )
                if is_call:
                    procedure_roots.add(target_linear)
                else:
                    successors.setdefault(linear, set()).add(target_linear)
                queue.append((target_linear, target_segment))
            else:
                unresolved.add((linear, f"near target {target_segment:04x}:{target_offset:04x} outside image"))
        elif far_target is not None:
            target_segment, target_offset = far_target
            target_linear = target_segment * 16 + target_offset
            overlay = overlay_targets.get(far_target)
            if overlay:
                edges.add(
                    (
                        linear,
                        "overlay_call" if is_call else "overlay_jump",
                        f"{overlay[0]}:{overlay[1]}",
                        f"{target_segment:04x}:{target_offset:04x}",
                    )
                )
            elif 0 <= target_linear < len(image) and not excluded_owner(target_linear):
                kind = "far_call" if is_call else "far_jump"
                direct_edges.add(
                    (
                        linear,
                        kind,
                        target_linear,
                        f"{target_segment:04x}:{target_offset:04x}",
                    )
                )
                block_targets.add(target_linear)
                target_sources.setdefault(target_linear, set()).add((linear, kind))
                logical_addresses.setdefault(target_linear, set()).add(far_target)
                if is_call:
                    procedure_roots.add(target_linear)
                else:
                    successors.setdefault(linear, set()).add(target_linear)
                queue.append((target_linear, target_segment))
            else:
                name = f"external_{target_segment:04x}_{target_offset:04x}"
                edges.add(
                    (
                        linear,
                        "external_far_call" if is_call else "external_far_jump",
                        name,
                        f"{target_segment:04x}:{target_offset:04x}",
                    )
                )
        elif is_call or is_jump:
            unresolved.add((linear, f"{decoded.mnemonic} {decoded.op_str}".strip()))

        unconditional_jump = is_jump and mnemonic in {"jmp", "ljmp"}
        returns = decoded.group(capstone.CS_GRP_RET) or mnemonic == "iret"
        non_returning_call = is_call and far_target in NON_RETURNING_FAR_TARGETS
        falls_through = not (
            unconditional_jump
            or returns
            or non_returning_call
            or mnemonic in {"int3", "hlt"}
        )
        if falls_through:
            next_offset = (logical_offset + decoded.size) & 0xFFFF
            next_linear = cs_segment * 16 + next_offset
            if 0 <= next_linear < len(image):
                queue.append((next_linear, cs_segment))
                successors.setdefault(linear, set()).add(next_linear)
                if is_jump:
                    block_targets.add(next_linear)
                    target_sources.setdefault(next_linear, set()).add(
                        (linear, "conditional_fallthrough")
                    )
                    logical_addresses.setdefault(next_linear, set()).add(
                        (cs_segment, next_offset)
                    )

    visited = set(instructions)
    ranges = ranges_from_instructions(instructions, visited)
    procedure_bodies = collect_procedure_bodies(
        procedure_roots, visited, successors
    )
    block_starts = {target for target in block_targets if target in visited}
    block_spans = {}
    block_terminators = {}
    for start in sorted(block_starts):
        cursor, end, seen = start, start, set()
        while cursor in visited and cursor not in seen:
            if cursor != start and cursor in block_starts:
                break
            seen.add(cursor)
            instruction = instructions[cursor]
            end = cursor + instruction.size
            mnemonic = instruction.mnemonic.lower()
            if (
                instruction.group(capstone.CS_GRP_JUMP)
                or instruction.group(capstone.CS_GRP_RET)
                or mnemonic in {"iret", "int3", "hlt"}
            ):
                break
            cursor = end
        block_spans[start] = end
        block_terminators[start] = instruction.mnemonic.lower()

    roots = []
    root_names = {}
    for linear in sorted(procedure_roots & visited):
        metadata = seed_metadata.get(linear, [])
        preferred = next(
            (item for item in metadata if not item[0].startswith("resident_")),
            metadata[0] if metadata else None,
        )
        addresses = sorted(logical_addresses.get(linear, set()))
        if preferred:
            name, tags, evidence, segment, offset = preferred
        else:
            segment, offset = addresses[0]
            name = f"exe_{segment:04x}_proc_{offset:04x}"
            tags, evidence = ["resident", "procedure"], "direct call target"
        annotation = semantic_annotation(
            "resident_procedures", f"{segment:04x}:{offset:04x}"
        )
        fallback_name = name
        if annotation:
            name = annotation["name"]
        semantic_evidence = RTL_SEMANTIC_EVIDENCE.get((segment, offset))
        if annotation:
            naming = annotation_naming(annotation)
        elif name.startswith(("exe_", "resident_")):
            naming = dict(UNKNOWN_NAMING)
        elif semantic_evidence:
            naming = naming_metadata(
                "identified", semantic_evidence[0], semantic_evidence[1]
            )
        else:
            naming = naming_metadata("identified", "high", evidence)
        aliases = sorted(
            {
                item[0]
                for item in metadata
                if item[0] != name
            }
            | ({fallback_name} if fallback_name != name else set())
            | annotation_aliases(annotation)
        )
        root_names[linear] = name
        roots.append(
            {
                "id": durable_id("procedure", "exe", linear),
                "name": name,
                "naming": naming,
                "aliases": aliases,
                "logical_address": f"{segment:04x}:{offset:04x}",
                "logical_aliases": [f"{seg:04x}:{off:04x}" for seg, off in addresses],
                "load_offset": hx(linear, 5),
                "exe_offset": hx(mz.header_size + linear, 6),
                "entry_span": [hx(linear, 5), hx(block_spans[linear], 5)],
                "body_ranges": [
                    [hx(start, 5), hx(end, 5)]
                    for start, end in ranges_from_instructions(
                        instructions, procedure_bodies[linear]
                    )
                ],
                "tags": sorted(set(["resident", "procedure", *tags])),
                "confidence": "proven",
                "evidence": evidence,
            }
        )

    roots_by_linear = {
        int(root["load_offset"], 0): root for root in roots
    }
    for entry, root in roots_by_linear.items():
        members = procedure_bodies[entry]
        grouped = {}
        for source, kind, target, logical_target in direct_edges:
            if source not in members or kind not in {"near_call", "far_call"}:
                continue
            target_root = roots_by_linear.get(target)
            key = (
                kind,
                target_root["name"] if target_root else None,
                logical_target,
            )
            grouped.setdefault(key, []).append(source)
        for source, kind, target, logical_target in edges:
            if source not in members or "call" not in kind:
                continue
            grouped.setdefault(
                (kind, edge_target_name(target), logical_target), []
            ).append(source)
        for source, text in unresolved:
            if source in members and text.split(" ", 1)[0] in {"call", "lcall", "callf"}:
                grouped.setdefault(("indirect_call", None, None), []).append(source)
        root["callees"] = [
            {
                "kind": kind,
                "to": target,
                "target_address": logical_target,
                "sites": [hx(site, 5) for site in sorted(set(sites))],
            }
            for (kind, target, logical_target), sites in sorted(
                grouped.items(), key=lambda item: str(item[0])
            )
        ]
        root["callers"] = []

    blocks = []
    for start in sorted(block_starts):
        addresses = sorted(logical_addresses.get(start, set()))
        segment, offset = addresses[0]
        fallback_name = f"exe_{segment:04x}_loc_{offset:04x}"
        root = roots_by_linear.get(start)
        block_annotation = semantic_annotation(
            "blocks", f"{segment:04x}:{offset:04x}"
        )
        semantic_owners = [
            candidate
            for entry, candidate in roots_by_linear.items()
            if candidate["naming"]["status"] == "identified"
            and start in procedure_bodies[entry]
        ]
        if root:
            name = root["name"]
            block_naming = dict(root["naming"])
        elif block_annotation:
            name = block_annotation["name"]
            block_naming = annotation_naming(block_annotation)
        elif len(semantic_owners) == 1:
            owner = semantic_owners[0]
            is_loop_head = any(
                kind not in {"near_call", "far_call"}
                and target == start
                and source > start
                for source, kind, target, _logical in direct_edges
            )
            source_kinds = {
                kind for _source, kind in target_sources.get(start, set())
            }
            if is_loop_head:
                role = "loop_head"
            elif block_terminators[start].startswith("ret"):
                role = "return"
            elif source_kinds & {"conditional_jump", "conditional_fallthrough"}:
                role = "branch"
            elif source_kinds & {"unconditional_jump", "far_jump"}:
                role = "join"
            else:
                role = "block"
            name = f"{owner['name']}__{role}_{offset:04x}"
            block_naming = naming_metadata(
                "contextual",
                "structural",
                f"intraprocedural {role.replace('_', ' ')} in identified procedure "
                f"{owner['name']}",
            )
        else:
            name = fallback_name
            block_naming = dict(UNKNOWN_NAMING)
        sources = [
            {"at": hx(source, 5) if source is not None else None, "kind": kind}
            for source, kind in sorted(
                target_sources.get(start, set()),
                key=lambda item: (-1 if item[0] is None else item[0], item[1]),
            )
        ]
        blocks.append(
            {
                "id": durable_id("block", "exe", start),
                "name": name,
                "naming": block_naming,
                "aliases": (
                    list(root.get("aliases", []))
                    if root
                    else sorted(
                        ({fallback_name} if name != fallback_name else set())
                        | annotation_aliases(block_annotation)
                    )
                ),
                "load_span": [hx(start, 5), hx(block_spans[start], 5)],
                "exe_span": [
                    hx(mz.header_size + start, 6),
                    hx(mz.header_size + block_spans[start], 6),
                ],
                "logical_addresses": [f"{seg:04x}:{off:04x}" for seg, off in addresses],
                "target_kinds": sorted({source["kind"] for source in sources}),
                "sources": sources,
                "tags": ["resident", "basic-block"],
                "confidence": "proven",
                "evidence": "resident seed or direct control-flow target",
            }
        )

    holes = []
    cursor = 0
    for start, end in ranges:
        if cursor < start:
            holes.append([cursor, start])
        cursor = max(cursor, end)
    if cursor < len(image):
        holes.append([cursor, len(image)])
    resident_data_references = set()
    for source, target_offset, kind in find_cs_pointer_references(
        instructions, [[0, 0x10000]]
    ):
        for segment, _logical_offset in logical_addresses.get(source, set()):
            target = segment * 16 + target_offset
            if any(start <= target < end for start, end in holes):
                resident_data_references.add((source, target, kind))
    data_chunks = []
    for range_start, range_end in holes:
        split_points = {range_start, range_end}
        for start, end, _unit in excluded:
            if range_start < start < range_end:
                split_points.add(start)
            if range_start < end < range_end:
                split_points.add(end)
        for linear in seed_metadata:
            if range_start < linear < range_end:
                split_points.add(linear)
        points = sorted(split_points)
        for start, end in zip(points, points[1:]):
            descriptor = next(
                (unit for ex_start, ex_end, unit in excluded if ex_start == start and ex_end == end),
                None,
            )
            if descriptor:
                name = f"{descriptor.unit_id}_descriptor_record"
                classification = "overlay_descriptor_record"
                naming = naming_metadata(
                    "structural",
                    "proven",
                    "overlay descriptor parser supplies the exact record boundary",
                )
            else:
                name = f"exe_data_{start:05x}"
                classification = classify_unreached(image[start:end])
                naming = dict(UNKNOWN_NAMING)
            data_chunks.append(
                {
                    "name": name,
                    "naming": naming,
                    "load_span": [hx(start, 5), hx(end, 5)],
                    "exe_span": [
                        hx(mz.header_size + start, 6),
                        hx(mz.header_size + end, 6),
                    ],
                    "size": end - start,
                    "classification": classification,
                    "tags": ["resident", "data-chunk"],
                    "confidence": "proven-boundary",
                    "evidence": (
                        "overlay descriptor boundary"
                        if descriptor
                        else "bytes not decoded from any proven resident root"
                    ),
                }
            )

    for chunk in data_chunks:
        chunk["referenced_by"] = []
    for entry, root in roots_by_linear.items():
        members = procedure_bodies[entry]
        grouped_references = {}
        for source, target, kind in resident_data_references:
            if source not in members:
                continue
            chunk = next(
                (
                    candidate
                    for candidate in data_chunks
                    if int(candidate["load_span"][0], 0)
                    <= target
                    < int(candidate["load_span"][1], 0)
                ),
                None,
            )
            if chunk:
                grouped_references.setdefault((chunk["name"], target, kind), []).append(
                    source
                )
        root["data_references"] = [
            {
                "to": chunk_name,
                "target_address": hx(mz.header_size + target, 6),
                "kind": kind,
                "sites": [hx(site, 5) for site in sorted(set(sites))],
            }
            for (chunk_name, target, kind), sites in sorted(grouped_references.items())
        ]
        for reference in root["data_references"]:
            chunk = next(
                chunk for chunk in data_chunks if chunk["name"] == reference["to"]
            )
            chunk["referenced_by"].append(
                {
                    "from": root["name"],
                    "kind": reference["kind"],
                    "sites": reference["sites"],
                }
            )
    for chunk in data_chunks:
        chunk["referenced_by"].sort(
            key=lambda reference: (
                reference["from"], reference["kind"], reference["sites"]
            )
        )

    grouped_edges = {}
    for source, kind, target, logical_target in edges:
        grouped_edges.setdefault((kind, target, logical_target), []).append(source)
    return {
        "load_span": [hx(0, 5), hx(len(image), 5)],
        "exe_span": [hx(mz.header_size, 6), hx(len(image) + mz.header_size, 6)],
        "roots": roots,
        "blocks": blocks,
        "data_chunks": data_chunks,
        "strings": string_records(
            "exe",
            image,
            roots,
            blocks,
            data_chunks,
            sorted(resident_data_references),
        ),
        "control_flow": {
            "reachable_ranges": [[hx(start, 5), hx(end, 5)] for start, end in ranges],
            "unreached_ranges": [[hx(start, 5), hx(end, 5)] for start, end in holes],
            "external_edges": [
                {
                    "kind": kind,
                    "to": target,
                    "logical_target": logical_target,
                    "sites": [hx(site, 5) for site in sorted(sites)],
                }
                for (kind, target, logical_target), sites in sorted(
                    grouped_edges.items(), key=lambda item: (item[0][0], item[0][1])
                )
            ],
            "unresolved_transfers": [
                {"at": hx(offset, 5), "instruction": text}
                for offset, text in sorted(unresolved)
            ],
            "decode_conflicts": [
                {"target": hx(target, 5), "inside_instruction_at": hx(owner, 5)}
                for target, owner in sorted(conflicts)
            ],
            "direct_edges": [
                {
                    "at": hx(source, 5),
                    "kind": kind,
                    "to": hx(target, 5),
                    "logical_target": logical_target,
                }
                for source, kind, target, logical_target in sorted(direct_edges)
            ],
            "data_references": [
                {
                    "at": hx(source, 5),
                    "to": hx(target, 5),
                    "kind": kind,
                }
                for source, target, kind in sorted(resident_data_references)
            ],
        },
    }


def attach_calculated_callees(
    catalog_units: list[dict], resident_image: dict
) -> list[dict]:
    """Resolve the proven closed indirect-call sets into procedure edges."""
    units_by_id = {unit["id"]: unit for unit in catalog_units}
    roots_by_id = {
        root["id"]: root
        for unit in catalog_units
        for root in unit["roots"]
    }
    roots_by_id.update({root["id"]: root for root in resident_image["roots"]})
    result = []
    decoded_site_ids = {
        durable_id(
            "site",
            "ovr",
            int(unit["ovr"]["code_offset"], 0) + int(transfer["at"], 0),
        )
        for unit in catalog_units
        for transfer in unit["control_flow"]["unresolved_transfers"]
    }
    decoded_site_ids.update(
        durable_id("site", "exe", int(transfer["at"], 0))
        for transfer in resident_image["control_flow"]["unresolved_transfers"]
    )
    specified_site_ids = {
        durable_id("site", specification["storage"], site)
        for specification in CALCULATED_TRANSFERS
        for site in specification["sites"]
    }
    if decoded_site_ids != specified_site_ids:
        missing = sorted(decoded_site_ids - specified_site_ids)
        extra = sorted(specified_site_ids - decoded_site_ids)
        raise BREError(
            f"calculated-transfer site set is stale; missing={missing}, extra={extra}"
        )

    def source_records(storage: str, absolute_site: int) -> list[tuple[str, dict, int]]:
        if storage == "exe":
            return [
                ("resident_exe", root, absolute_site)
                for root in resident_image["roots"]
                if any(
                    int(start, 0) <= absolute_site < int(end, 0)
                    for start, end in root["body_ranges"]
                )
            ]
        unit = next(
            (
                candidate
                for candidate in catalog_units
                if int(candidate["ovr"]["code_offset"], 0)
                <= absolute_site
                < int(candidate["ovr"]["fixup_offset"], 0)
            ),
            None,
        )
        if unit is None:
            return []
        base = int(unit["ovr"]["code_offset"], 0)
        local_site = absolute_site - base
        return [
            (unit["id"], root, local_site)
            for root in unit["roots"]
            if any(
                int(start, 0) <= local_site < int(end, 0)
                for start, end in root["body_ranges"]
            )
        ]

    for specification in CALCULATED_TRANSFERS:
        dispatch_id = f"bre0988:dispatch:{specification['key']}"
        targets = []
        for storage, address in specification["targets"]:
            target_id = durable_id("procedure", storage, address)
            if target_id not in roots_by_id:
                raise BREError(
                    f"calculated transfer {dispatch_id}: target {target_id} is not a root"
                )
            target = roots_by_id[target_id]
            targets.append(
                {
                    "id": target_id,
                    "name": target["name"],
                    "storage": storage,
                    "address": (
                        target["ovr_offset"]
                        if storage == "ovr"
                        else target["logical_address"]
                    ),
                }
            )

        uses: dict[tuple[str, str], dict] = {}
        for absolute_site in specification["sites"]:
            sources = source_records(specification["storage"], absolute_site)
            if not sources:
                raise BREError(
                    f"calculated transfer {dispatch_id}: site {hx(absolute_site, 6)} has no owner"
                )
            for container, source, local_site in sources:
                key = (container, source["id"])
                use = uses.setdefault(
                    key,
                    {"container": container, "source": source, "sites": []},
                )
                use["sites"].append(local_site)

        for use in uses.values():
            sites = sorted(set(use["sites"]))
            for target in targets:
                use["source"]["callees"].append(
                    {
                        "kind": "calculated_call",
                        "to": target["name"],
                        "target_address": target["address"],
                        "sites": [
                            hx(site, 5) if use["container"] == "resident_exe" else hx(site)
                            for site in sites
                        ],
                        "dispatch_id": dispatch_id,
                    }
                )

        site_ids = [
            durable_id("site", specification["storage"], site)
            for site in specification["sites"]
        ]
        sources = []
        for use in sorted(uses.values(), key=lambda item: item["source"]["id"]):
            if use["container"] == "resident_exe":
                source_site_ids = [
                    durable_id("site", "exe", site)
                    for site in sorted(set(use["sites"]))
                ]
            else:
                base = int(units_by_id[use["container"]]["ovr"]["code_offset"], 0)
                source_site_ids = [
                    durable_id("site", "ovr", base + site)
                    for site in sorted(set(use["sites"]))
                ]
            sources.append(
                {
                    "id": use["source"]["id"],
                    "name": use["source"]["name"],
                    "container": use["container"],
                    "site_ids": source_site_ids,
                }
            )
        result.append(
            {
                "id": dispatch_id,
                "kind": "calculated_call",
                "closed": True,
                "model": specification["model"],
                "site_ids": site_ids,
                "sources": sources,
                "targets": targets,
                "evidence": specification["evidence"],
            }
        )

    resolved_sites = specified_site_ids
    for unit in catalog_units:
        base = int(unit["ovr"]["code_offset"], 0)
        unit["control_flow"]["unresolved_transfers"] = [
            transfer
            for transfer in unit["control_flow"]["unresolved_transfers"]
            if durable_id("site", "ovr", base + int(transfer["at"], 0))
            not in resolved_sites
        ]
    resident_image["control_flow"]["unresolved_transfers"] = [
        transfer
        for transfer in resident_image["control_flow"]["unresolved_transfers"]
        if durable_id("site", "exe", int(transfer["at"], 0))
        not in resolved_sites
    ]
    return result


def attach_procedure_callers(catalog_units: list[dict], resident_image: dict) -> None:
    """Attach durable IDs to both directions of the procedure call graph."""
    procedures = []
    for unit in catalog_units:
        procedures.extend((unit["id"], root) for root in unit["roots"])
    procedures.extend(
        ("resident_exe", root) for root in resident_image["roots"]
    )
    units_by_id = {unit["id"]: unit for unit in catalog_units}

    def call_site_ids(container: str, sites: list[str]) -> list[str]:
        if container == "resident_exe":
            return [
                durable_id("site", "exe", int(site, 0))
                for site in sites
            ]
        base = int(units_by_id[container]["ovr"]["code_offset"], 0)
        return [
            durable_id("site", "ovr", base + int(site, 0))
            for site in sites
        ]

    by_name = {
        name: (container, root)
        for container, root in procedures
        for name in [root["name"], *root.get("aliases", [])]
    }
    for _container, root in procedures:
        root["callers"] = []
    for caller_container, caller in procedures:
        for callee in caller["callees"]:
            target = callee.get("to")
            target_record = by_name.get(target)
            callee["to_id"] = (
                target_record[1]["id"] if target_record is not None else None
            )
            if target_record is not None:
                callee["to"] = target_record[1]["name"]
            callee["site_ids"] = call_site_ids(
                caller_container, callee["sites"]
            )
            if target_record is None:
                continue
            _target_container, target_root = target_record
            target_root["callers"].append(
                {
                    "from": caller["name"],
                    "from_id": caller["id"],
                    "container": caller_container,
                    "kind": callee["kind"],
                    "sites": callee["sites"],
                    "site_ids": callee["site_ids"],
                    **(
                        {"dispatch_id": callee["dispatch_id"]}
                        if "dispatch_id" in callee
                        else {}
                    ),
                }
            )
    for _container, root in procedures:
        root["callers"].sort(
            key=lambda caller: (
                caller["container"], caller["from"], caller["kind"], caller["sites"]
            )
        )


def semantic_coverage(catalog_units: list[dict], resident_image: dict) -> dict:
    groups = {
        "procedures": [
            root for unit in catalog_units for root in unit["roots"]
        ]
        + resident_image["roots"],
        "blocks": [block for unit in catalog_units for block in unit["blocks"]]
        + resident_image["blocks"],
        "data_chunks": [
            chunk for unit in catalog_units for chunk in unit["data_chunks"]
        ]
        + resident_image["data_chunks"],
        "fixup_chunks": [unit["fixup_chunk"] for unit in catalog_units],
    }
    return {
        group: dict(sorted(collections.Counter(
            record["naming"]["status"] for record in records
        ).items()))
        for group, records in groups.items()
    }


def build_catalog(exe: bytes, ovr: bytes, mz: MZHeader, units: list[Unit], cfg: bool):
    pinned_release = (
        hashlib.sha256(exe).hexdigest() == EXPECTED["exe"]["sha256"]
        and hashlib.sha256(ovr).hexdigest() == EXPECTED["ovr"]["sha256"]
    )
    descriptor_roots = {}
    for unit in units:
        for stub in unit.stubs:
            descriptor_roots[(unit.descriptor_segment, 0x20 + stub.index * 5)] = (
                unit.unit_id,
                root_name(unit, stub.entry_offset),
            )

    catalog_units = []
    for unit in units:
        roots = []
        for stub in unit.stubs:
            absolute = unit.ovr_offset + stub.entry_offset
            annotation = semantic_annotation(
                "overlay_procedures", hx(absolute, 6)
            )
            name = root_name(unit, stub.entry_offset)
            fallback_name = f"{unit.unit_id}_entry_{stub.entry_offset:04x}"
            aliases = sorted(
                ({fallback_name} if name != fallback_name else set())
                | annotation_aliases(annotation)
            )
            tags = ["overlay", "exported"]
            if absolute in LANDMARKS:
                tags.extend(LANDMARKS[absolute][1])
            roots.append(
                {
                    "id": durable_id("procedure", "ovr", absolute),
                    "name": name,
                    "naming": (
                        annotation_naming(annotation)
                        if annotation
                        else naming_metadata(
                            "identified",
                            "high",
                            "existing repository-cited semantic landmark",
                        )
                        if absolute in LANDMARKS
                        else dict(UNKNOWN_NAMING)
                    ),
                    "entry_offset": hx(stub.entry_offset),
                    "ovr_offset": hx(absolute, 6),
                    "stub": {
                        "exe_offset": hx(stub.file_offset, 6),
                        "logical_target": f"{unit.descriptor_segment:04x}:{0x20 + stub.index * 5:04x}",
                    },
                    "aliases": aliases,
                    "tags": sorted(set(tags)),
                    "confidence": "proven",
                    "evidence": "overlay descriptor stub",
                }
            )
        item = {
            "id": unit.unit_id,
            "descriptor": {
                "index": unit.index,
                "exe_offset": hx(unit.descriptor_file_offset, 6),
                "logical_segment": hx(unit.descriptor_segment),
                "previous_segment": hx(unit.previous_segment),
            },
            "ovr": {
                "code_offset": hx(unit.ovr_offset, 6),
                "code_size": hx(unit.code_size),
                "fixup_offset": hx(unit.fixup_offset, 6),
                "fixup_size": hx(unit.fixup_size),
                "end_offset": hx(unit.end_offset, 6),
            },
            "roots": roots,
            "fixups": {
                "encoding": "little-endian uint16 code offsets",
                "count": len(unit.fixups),
                "stream_sha256": hashlib.sha256(
                    ovr[unit.fixup_offset : unit.end_offset]
                ).hexdigest(),
            },
        }
        if cfg:
            code = ovr[unit.ovr_offset : unit.ovr_offset + unit.code_size]
            flow = analyze_cfg(
                code, {stub.entry_offset for stub in unit.stubs}, descriptor_roots
            )
            block_details = flow.pop("_blocks")
            procedure_bodies = flow.pop("_procedure_bodies")
            procedure_body_ranges = flow.pop("_procedure_body_ranges")
            exported = {stub.entry_offset for stub in unit.stubs}
            for entry in flow.pop("_procedure_roots"):
                if entry in exported:
                    matching = next(root for root in roots if int(root["entry_offset"], 0) == entry)
                    matching["entry_span"] = [hx(entry), hx(block_details[entry]["end"])]
                    continue
                absolute = unit.ovr_offset + entry
                annotation = semantic_annotation(
                    "overlay_procedures", hx(absolute, 6)
                )
                if annotation:
                    name, semantic_tags = annotation["name"], []
                elif absolute in LANDMARKS:
                    name, semantic_tags = LANDMARKS[absolute]
                else:
                    name, semantic_tags = f"{unit.unit_id}_proc_{entry:04x}", []
                fallback_name = f"{unit.unit_id}_proc_{entry:04x}"
                roots.append(
                    {
                        "id": durable_id("procedure", "ovr", absolute),
                        "name": name,
                        "naming": (
                            annotation_naming(annotation)
                            if annotation
                            else naming_metadata(
                                "identified",
                                "high",
                                "existing repository-cited semantic landmark",
                            )
                            if absolute in LANDMARKS
                            else dict(UNKNOWN_NAMING)
                        ),
                        "entry_offset": hx(entry),
                        "entry_span": [hx(entry), hx(block_details[entry]["end"])],
                        "ovr_offset": hx(absolute, 6),
                        "stub": None,
                        "aliases": sorted(
                            ({fallback_name} if name != fallback_name else set())
                            | annotation_aliases(annotation)
                        ),
                        "tags": sorted(set(["overlay", "near-call-target", *semantic_tags])),
                        "confidence": "proven",
                        "evidence": "direct near call from exported-root-reachable code",
                    }
                )
            roots.sort(key=lambda root: int(root["entry_offset"], 0))
            roots_by_entry = {int(root["entry_offset"], 0): root for root in roots}
            for entry, root in roots_by_entry.items():
                members = procedure_bodies[entry]
                root["body_ranges"] = [
                    [hx(start), hx(end)]
                    for start, end in procedure_body_ranges[entry]
                ]
                grouped = {}
                for edge in flow["direct_edges"]:
                    source = int(edge["at"], 0)
                    if source not in members or edge["kind"] != "near_call":
                        continue
                    target = int(edge["to"], 0)
                    target_root = roots_by_entry.get(target)
                    key = (
                        edge["kind"],
                        target_root["name"] if target_root else None,
                        hx(unit.ovr_offset + target, 6),
                    )
                    grouped.setdefault(key, []).append(source)
                for edge in flow["external_edges"]:
                    if "call" not in edge["kind"]:
                        continue
                    sites = [int(site, 0) for site in edge["sites"]]
                    owned_sites = [site for site in sites if site in members]
                    if owned_sites:
                        grouped.setdefault(
                            (
                                edge["kind"],
                                edge_target_name(edge["to"]),
                                edge["logical_target"],
                            ),
                            [],
                        ).extend(owned_sites)
                for transfer in flow["unresolved_transfers"]:
                    source = int(transfer["at"], 0)
                    mnemonic = transfer["instruction"].split(" ", 1)[0]
                    if source in members and mnemonic in {"call", "lcall", "callf"}:
                        grouped.setdefault(("indirect_call", None, None), []).append(source)
                root["callees"] = [
                    {
                        "kind": kind,
                        "to": target,
                        "target_address": target_address,
                        "sites": [hx(site) for site in sorted(set(sites))],
                    }
                    for (kind, target, target_address), sites in sorted(
                        grouped.items(), key=lambda item: str(item[0])
                    )
                ]
                root["callers"] = []
            blocks = []
            for start, details in block_details.items():
                end = details["end"]
                root = roots_by_entry.get(start)
                absolute = unit.ovr_offset + start
                landmark = LANDMARKS.get(absolute)
                block_annotation = semantic_annotation(
                    "blocks", hx(absolute, 6)
                )
                semantic_owners = [
                    candidate
                    for entry, candidate in roots_by_entry.items()
                    if candidate["naming"]["status"] == "identified"
                    and start in procedure_bodies[entry]
                ]
                if root:
                    name = root["name"]
                    block_naming = dict(root["naming"])
                elif block_annotation:
                    name = block_annotation["name"]
                    block_naming = annotation_naming(block_annotation)
                elif landmark:
                    name = landmark[0]
                    block_naming = naming_metadata(
                        "identified",
                        "high",
                        "existing repository-cited semantic landmark",
                    )
                elif len(semantic_owners) == 1:
                    owner = semantic_owners[0]
                    is_loop_head = any(
                        edge["kind"] != "near_call"
                        and int(edge["to"], 0) == start
                        and int(edge["at"], 0) > start
                        for edge in flow["direct_edges"]
                    )
                    source_kinds = {kind for _source, kind in details["sources"]}
                    if is_loop_head:
                        role = "loop_head"
                    elif details["terminator"].startswith("ret"):
                        role = "return"
                    elif source_kinds & {"conditional_jump", "conditional_fallthrough"}:
                        role = "branch"
                    elif "unconditional_jump" in source_kinds:
                        role = "join"
                    else:
                        role = "block"
                    name = f"{owner['name']}__{role}_{start:04x}"
                    block_naming = naming_metadata(
                        "contextual",
                        "structural",
                        f"intraprocedural {role.replace('_', ' ')} in identified procedure "
                        f"{owner['name']}",
                    )
                else:
                    name = f"{unit.unit_id}_loc_{start:04x}"
                    block_naming = dict(UNKNOWN_NAMING)
                block_fallback = f"{unit.unit_id}_loc_{start:04x}"
                sources = [
                    {"at": hx(source), "kind": kind}
                    for source, kind in details["sources"]
                ]
                if start in exported:
                    sources.insert(0, {"at": None, "kind": "overlay_stub"})
                tags = ["overlay", "basic-block"]
                if root:
                    tags.append("procedure-entry")
                if landmark:
                    tags.extend(landmark[1])
                blocks.append(
                    {
                        "id": durable_id(
                            "block", "ovr", unit.ovr_offset + start
                        ),
                        "name": name,
                        "aliases": (
                            list(root.get("aliases", []))
                            if root
                            else sorted(
                                ({block_fallback} if name != block_fallback else set())
                                | annotation_aliases(block_annotation)
                            )
                        ),
                        "naming": (
                            dict(root["naming"])
                            if root
                            else block_naming
                        ),
                        "unit_span": [hx(start), hx(end)],
                        "ovr_span": [
                            hx(unit.ovr_offset + start, 6),
                            hx(unit.ovr_offset + end, 6),
                        ],
                        "target_kinds": sorted({source["kind"] for source in sources}),
                        "sources": sources,
                        "tags": sorted(set(tags)),
                        "confidence": "proven",
                        "evidence": (
                            root["evidence"]
                            if root
                            else "direct jump target or conditional fallthrough"
                        ),
                    }
                )
            item["blocks"] = blocks
            item["data_chunks"] = data_chunks_for_unit(
                unit, code, flow["unreached_ranges"]
            )
            for chunk in item["data_chunks"]:
                chunk["referenced_by"] = []
            for entry, root in roots_by_entry.items():
                members = procedure_bodies[entry]
                grouped_references = {}
                for reference in flow["data_references"]:
                    source = int(reference["at"], 0)
                    target = int(reference["to"], 0)
                    if source not in members:
                        continue
                    chunk = next(
                        (
                            candidate
                            for candidate in item["data_chunks"]
                            if int(candidate["unit_span"][0], 0)
                            <= target
                            < int(candidate["unit_span"][1], 0)
                        ),
                        None,
                    )
                    if chunk is None:
                        continue
                    key = (chunk["name"], target, reference["kind"])
                    grouped_references.setdefault(key, []).append(source)
                root["data_references"] = [
                    {
                        "to": chunk_name,
                        "target_address": hx(unit.ovr_offset + target, 6),
                        "kind": kind,
                        "sites": [hx(site) for site in sorted(set(sites))],
                    }
                    for (chunk_name, target, kind), sites in sorted(
                        grouped_references.items()
                    )
                ]
                for reference in root["data_references"]:
                    chunk = next(
                        chunk
                        for chunk in item["data_chunks"]
                        if chunk["name"] == reference["to"]
                    )
                    chunk["referenced_by"].append(
                        {
                            "from": root["name"],
                            "kind": reference["kind"],
                            "sites": reference["sites"],
                        }
                    )
            for chunk in item["data_chunks"]:
                chunk["referenced_by"].sort(
                    key=lambda reference: (
                        reference["from"], reference["kind"], reference["sites"]
                    )
                )
            renamed_chunks = {}
            roots_by_name = {root["name"]: root for root in roots}
            for chunk in item["data_chunks"]:
                start, end = (
                    int(value, 0) for value in chunk["unit_span"]
                )
                targets = {
                    int(reference["target_address"], 0) - unit.ovr_offset
                    for root in roots
                    for reference in root["data_references"]
                    if reference["to"] == chunk["name"]
                }
                valid_strings = sum(
                    looks_like_pascal_string(code, target, end)
                    for target in targets
                )
                chunk["content_kind"] = (
                    "pascal_string_table"
                    if targets and valid_strings * 5 >= len(targets) * 4
                    else "code_segment_constants"
                )
                annotation = semantic_annotation(
                    "overlay_data", chunk["ovr_span"][0]
                )
                owners = {reference["from"] for reference in chunk["referenced_by"]}
                old_name = chunk["name"]
                if annotation:
                    new_name = annotation["name"]
                    chunk["naming"] = annotation_naming(annotation)
                elif (
                    chunk["naming"]["status"] == "unclassified"
                    and len(owners) == 1
                    and roots_by_name[next(iter(owners))]["naming"]["status"]
                    == "identified"
                ):
                    owner = next(iter(owners))
                    suffix = (
                        "strings"
                        if chunk["content_kind"] == "pascal_string_table"
                        else "constants"
                    )
                    new_name = f"{owner}_{suffix}"
                    chunk["naming"] = naming_metadata(
                        "identified",
                        "high" if suffix == "strings" else "medium",
                        f"exclusive code-segment references from identified procedure {owner}; "
                        f"content classified as {chunk['content_kind']}",
                    )
                else:
                    new_name = old_name
                if new_name != old_name:
                    chunk["name"] = new_name
                    chunk.setdefault("aliases", []).append(old_name)
                    renamed_chunks[old_name] = new_name
            for root in roots:
                for reference in root["data_references"]:
                    reference["to"] = renamed_chunks.get(
                        reference["to"], reference["to"]
                    )
            item["strings"] = string_records(
                "ovr",
                code,
                roots,
                blocks,
                item["data_chunks"],
                [
                    (
                        int(reference["at"], 0),
                        int(reference["to"], 0),
                        reference["kind"],
                    )
                    for reference in flow["data_references"]
                ],
                unit.ovr_offset,
            )
            item["fixup_chunk"] = {
                "name": f"{unit.unit_id}_fixups",
                "naming": naming_metadata(
                    "structural",
                    "proven",
                    "overlay descriptor supplies the exact relocation stream boundary",
                ),
                "ovr_span": [hx(unit.fixup_offset, 6), hx(unit.end_offset, 6)],
                "size": unit.fixup_size,
                "classification": "overlay_segment_relocation_offsets",
                "confidence": "proven",
            }
            item["control_flow"] = flow
        catalog_units.append(item)

    landmarks = []
    for absolute, (name, tags) in sorted(LANDMARKS.items()):
        owner = next(
            (unit for unit in units if unit.ovr_offset <= absolute < unit.ovr_offset + unit.code_size),
            None,
        )
        landmarks.append(
            {
                "name": name,
                "ovr_offset": hx(absolute, 6),
                "unit": owner.unit_id if owner else None,
                "unit_offset": hx(absolute - owner.ovr_offset) if owner else None,
                "tags": tags,
                "confidence": "repository-cited",
                "evidence": "existing binary-analysis citation in this repository",
            }
        )
    resident_image = None
    if cfg:
        resident_seeds = {
            (mz.entry_segment, mz.entry_offset): (
                "exe_entry",
                ["entry"],
                "MZ entry point",
            ),
            (0x0F5B, 0x02E6): (
                "overlay_interrupt_3f_handler",
                ["overlay-loader", "interrupt-handler"],
                "runtime trace normalized by the DOS load base",
            ),
        }
        if pinned_release:
            for address, (name, tags) in CALCULATED_RESIDENT_ROOTS.items():
                resident_seeds.setdefault(
                    address,
                    (
                        name,
                        tags,
                        "closed calculated-transfer target proven by complete static assignment tracing",
                    ),
                )
        for address, (name, tags) in RESIDENT_NAMES.items():
            semantic_evidence = RTL_SEMANTIC_EVIDENCE.get(address)
            resident_seeds[address] = (
                name,
                tags,
                semantic_evidence[1]
                if semantic_evidence
                else "repository-cited resident helper",
            )
        for unit in catalog_units:
            for edge in unit["control_flow"]["external_edges"]:
                if edge["kind"] not in {"far_call", "far_jump"}:
                    continue
                segment_text, offset_text = edge["logical_target"].split(":", 1)
                address = (int(segment_text, 16), int(offset_text, 16))
                resident_seeds.setdefault(
                    address,
                    (
                        edge["to"],
                        ["linked-from-overlay"],
                        "direct far target from exported-root-reachable overlay code",
                    ),
                )
        resident_image = analyze_resident_image(
            exe[mz.header_size :], mz, units, resident_seeds, descriptor_roots
        )
        resident = resident_image["roots"]
        calculated_transfers = (
            attach_calculated_callees(catalog_units, resident_image)
            if pinned_release
            else []
        )
        attach_procedure_callers(catalog_units, resident_image)
    else:
        calculated_transfers = []
        resident = [
            {
                "name": name,
                "logical_address": f"{segment:04x}:{offset:04x}",
                "tags": tags,
                "confidence": "repository-cited",
            }
            for (segment, offset), (name, tags) in sorted(RESIDENT_NAMES.items())
        ]
    string_index = [
        record for unit in catalog_units for record in unit.pop("strings", [])
    ]
    if resident_image:
        string_index.extend(resident_image.pop("strings", []))
    string_index.sort(key=lambda record: record["id"])
    catalog = {
        "format": "immortal-barons-bre-disassembly-map",
        "format_version": 6,
        "generator": f"scripts/bre-disasm.py {VERSION}",
        "release": {
            "name": "Barren Realms Elite 0.988",
            "official_url": OFFICIAL_URL,
            "artifacts": EXPECTED,
        },
        "mz": {
            "header_size": hx(mz.header_size),
            "load_module_size": hx(len(exe) - mz.header_size),
            "relocation_count": mz.relocation_count,
            "entry": f"{mz.entry_segment:04x}:{mz.entry_offset:04x}",
            "stack": f"{mz.stack_segment:04x}:{mz.stack_offset:04x}",
        },
        "loader": {
            "descriptor_magic": "cd 3f 00 00",
            "descriptor_size_before_stubs": hx(0x20),
            "stub_size": 5,
            "stub_disk_form": "cd 3f <entry:u16le> 00",
            "stub_loaded_form": "ea <entry:u16le> <runtime_unit_segment:u16le>",
            "root_mapping": "ovr code offset + stub entry offset",
            "fixup_action": "word[unit + fixup] += DOS EXE load segment",
            "runtime_unit_segment": "dynamic; not part of canonical addresses",
            "separate_fp_library": False,
            "fp_note": "Turbo Pascal Real48 helpers are resident in BRE.EXE (not a separate loaded file).",
        },
        "summary": {
            "unit_count": len(units),
            "exported_root_count": sum(len(unit.stubs) for unit in units),
            "reachable_procedure_root_count": sum(len(unit["roots"]) for unit in catalog_units),
            "basic_block_count": sum(len(unit.get("blocks", [])) for unit in catalog_units),
            "data_chunk_count": sum(len(unit.get("data_chunks", [])) for unit in catalog_units),
            "resident_procedure_root_count": len(resident_image["roots"]) if resident_image else len(resident),
            "resident_basic_block_count": len(resident_image["blocks"]) if resident_image else 0,
            "resident_data_chunk_count": len(resident_image["data_chunks"]) if resident_image else 0,
            "fixup_count": sum(len(unit.fixups) for unit in units),
            "referenced_string_count": len(string_index),
            "string_use_count": sum(
                len(record["used_by"]) for record in string_index
            ),
            "call_edge_count": sum(
                len(root.get("callees", []))
                for unit in catalog_units
                for root in unit["roots"]
            )
            + sum(len(root.get("callees", [])) for root in resident),
            "call_site_count": sum(
                len(callee["sites"])
                for unit in catalog_units
                for root in unit["roots"]
                for callee in root.get("callees", [])
            )
            + sum(
                len(callee["sites"])
                for root in resident
                for callee in root.get("callees", [])
            ),
            "calculated_transfer_group_count": len(calculated_transfers),
            "calculated_transfer_site_count": sum(
                len(transfer["site_ids"])
                for transfer in calculated_transfers
            ),
            "calculated_target_count": sum(
                len(transfer["targets"])
                for transfer in calculated_transfers
            ),
            "ovr_payload_start": hx(8),
            "ovr_payload_end": hx(len(ovr), 6),
            "semantic_coverage": (
                semantic_coverage(catalog_units, resident_image)
                if resident_image
                else {}
            ),
        },
        "resident_roots": resident,
        "landmarks": landmarks,
        "string_index": string_index,
        "calculated_transfers": calculated_transfers,
        "units": catalog_units,
    }
    if resident_image:
        catalog["resident_image"] = resident_image
    return catalog


def merged_spans(spans: list[tuple[int, int]], label: str) -> list[tuple[int, int]]:
    merged = []
    for start, end in sorted(spans):
        if end < start:
            raise BREError(f"{label}: reversed span {hx(start)}-{hx(end)}")
        if start == end:
            continue
        if merged and start < merged[-1][1]:
            raise BREError(f"{label}: overlapping spans at {hx(start)}")
        if merged and start == merged[-1][1]:
            merged[-1] = (merged[-1][0], end)
        else:
            merged.append((start, end))
    return merged


def validate_catalog(catalog: dict) -> dict:
    if catalog.get("format_version") != 6:
        raise BREError("catalog format version is not 6")
    names = {}
    ids = {}

    def record_name(name: str, location: str) -> None:
        previous = names.setdefault(name, location)
        if previous != location:
            raise BREError(f"name {name!r} maps to both {previous} and {location}")

    def record_id(identifier: str, location: str) -> None:
        if not identifier.startswith("bre0988:"):
            raise BREError(f"{location}: malformed durable ID {identifier!r}")
        previous = ids.setdefault(identifier, location)
        if previous != location:
            raise BREError(
                f"durable ID {identifier!r} maps to both {previous} and {location}"
            )

    def validate_naming(record: dict, location: str) -> None:
        naming = record.get("naming")
        if not naming or set(naming) != {"status", "confidence", "evidence"}:
            raise BREError(f"{location}: missing semantic naming record")
        if naming["status"] not in {
            "identified",
            "contextual",
            "structural",
            "unclassified",
        }:
            raise BREError(f"{location}: invalid naming status {naming['status']!r}")
        if not all(isinstance(naming[key], str) and naming[key] for key in naming):
            raise BREError(f"{location}: incomplete semantic naming evidence")

    overlay_blocks = overlay_data = 0
    overlay_roots = exported_roots = fixups = 0
    expected_ovr_offset = int(catalog["summary"]["ovr_payload_start"], 0)
    for unit in catalog["units"]:
        unit_id = unit["id"]
        code_offset = int(unit["ovr"]["code_offset"], 0)
        code_size = int(unit["ovr"]["code_size"], 0)
        fixup_offset = int(unit["ovr"]["fixup_offset"], 0)
        end_offset = int(unit["ovr"]["end_offset"], 0)
        fixup_span = tuple(
            int(value, 0) for value in unit["fixup_chunk"]["ovr_span"]
        )
        if code_offset != expected_ovr_offset:
            raise BREError(f"{unit_id}: OVR units are not contiguous")
        if fixup_offset != code_offset + code_size or fixup_span != (
            fixup_offset,
            end_offset,
        ):
            raise BREError(f"{unit_id}: code/fixup boundary is inconsistent")
        if unit["fixup_chunk"]["size"] != end_offset - fixup_offset:
            raise BREError(f"{unit_id}: fixup chunk size is inconsistent")
        expected_ovr_offset = end_offset
        reachable = [
            tuple(int(value, 0) for value in span)
            for span in unit["control_flow"]["reachable_ranges"]
        ]
        unreached = [
            tuple(int(value, 0) for value in span)
            for span in unit["control_flow"]["unreached_ranges"]
        ]
        if merged_spans(reachable + unreached, f"{unit_id} code partition") != [(0, code_size)]:
            raise BREError(f"{unit_id}: reachable and unreached bytes do not cover the code area")
        block_spans = [
            tuple(int(value, 0) for value in block["unit_span"])
            for block in unit["blocks"]
        ]
        if merged_spans(block_spans, f"{unit_id} blocks") != merged_spans(
            reachable, f"{unit_id} reachable ranges"
        ):
            raise BREError(f"{unit_id}: named blocks do not exactly cover reachable code")
        data_spans = [
            tuple(int(value, 0) for value in chunk["unit_span"])
            for chunk in unit["data_chunks"]
        ]
        if merged_spans(data_spans, f"{unit_id} data chunks") != merged_spans(
            unreached, f"{unit_id} unreached ranges"
        ):
            raise BREError(f"{unit_id}: named data chunks do not exactly cover unreached bytes")
        blocks_by_start = {
            int(block["unit_span"][0], 0): block for block in unit["blocks"]
        }
        for root in unit["roots"]:
            validate_naming(root, f"{unit_id}:{root['entry_offset']}")
            if not all(key in root for key in ("body_ranges", "callers", "callees", "data_references")):
                raise BREError(f"{unit_id}: procedure {root['name']} lacks analysis evidence")
            start = int(root["entry_offset"], 0)
            record_id(root.get("id", ""), root["ovr_offset"])
            if root["id"] != durable_id(
                "procedure", "ovr", int(root["ovr_offset"], 0)
            ):
                raise BREError(f"{unit_id}: procedure durable ID is stale")
            if start not in blocks_by_start or blocks_by_start[start]["name"] != root["name"]:
                raise BREError(f"{unit_id}: procedure root {root['name']} has no matching block")
            record_name(root["name"], root["ovr_offset"])
            for alias in root.get("aliases", []):
                record_name(alias, root["ovr_offset"])
            exported_roots += root.get("stub") is not None
        for block in unit["blocks"]:
            validate_naming(block, f"{unit_id}:{block['unit_span'][0]}")
            record_name(block["name"], block["ovr_span"][0])
            record_id(block.get("id", ""), block["ovr_span"][0])
            if block["id"] != durable_id(
                "block", "ovr", int(block["ovr_span"][0], 0)
            ):
                raise BREError(f"{unit_id}: block durable ID is stale")
            if not block["target_kinds"]:
                raise BREError(f"{unit_id}: block {block['name']} has no target evidence")
        for chunk in unit["data_chunks"]:
            validate_naming(chunk, f"{unit_id}:{chunk['unit_span'][0]}")
            if "referenced_by" not in chunk or "content_kind" not in chunk:
                raise BREError(f"{unit_id}: data chunk {chunk['name']} lacks analysis evidence")
            record_name(chunk["name"], chunk["ovr_span"][0])
            for alias in chunk.get("aliases", []):
                record_name(alias, chunk["ovr_span"][0])
        record_name(unit["fixup_chunk"]["name"], unit["fixup_chunk"]["ovr_span"][0])
        validate_naming(unit["fixup_chunk"], unit["fixup_chunk"]["ovr_span"][0])
        if unit["control_flow"]["decode_conflicts"]:
            raise BREError(f"{unit_id}: decode-boundary conflicts remain")
        if unit["control_flow"]["unresolved_transfers"]:
            raise BREError(f"{unit_id}: unresolved transfers remain")
        overlay_blocks += len(unit["blocks"])
        overlay_data += len(unit["data_chunks"])
        overlay_roots += len(unit["roots"])
        fixups += unit["fixups"]["count"]

    if expected_ovr_offset != int(catalog["summary"]["ovr_payload_end"], 0):
        raise BREError("OVR unit/fixup spans do not cover the payload")

    resident = catalog["resident_image"]
    resident_size = int(resident["load_span"][1], 0)
    resident_reachable = [
        tuple(int(value, 0) for value in span)
        for span in resident["control_flow"]["reachable_ranges"]
    ]
    resident_unreached = [
        tuple(int(value, 0) for value in span)
        for span in resident["control_flow"]["unreached_ranges"]
    ]
    if merged_spans(
        resident_reachable + resident_unreached, "resident image partition"
    ) != [(0, resident_size)]:
        raise BREError("resident reachable and data bytes do not cover the load module")
    resident_blocks = [
        tuple(int(value, 0) for value in block["load_span"])
        for block in resident["blocks"]
    ]
    if merged_spans(resident_blocks, "resident blocks") != merged_spans(
        resident_reachable, "resident reachable ranges"
    ):
        raise BREError("resident named blocks do not exactly cover reachable code")
    resident_data = [
        tuple(int(value, 0) for value in chunk["load_span"])
        for chunk in resident["data_chunks"]
    ]
    if merged_spans(resident_data, "resident data chunks") != merged_spans(
        resident_unreached, "resident unreached ranges"
    ):
        raise BREError("resident named data chunks do not exactly cover unreached bytes")
    resident_blocks_by_start = {
        int(block["load_span"][0], 0): block for block in resident["blocks"]
    }
    for root in resident["roots"]:
        validate_naming(root, f"resident:{root['logical_address']}")
        if not all(key in root for key in ("body_ranges", "callers", "callees", "data_references")):
            raise BREError(f"resident procedure {root['name']} lacks analysis evidence")
        start = int(root["load_offset"], 0)
        record_id(root.get("id", ""), f"exe:{root['load_offset']}")
        if root["id"] != durable_id("procedure", "exe", start):
            raise BREError("resident procedure durable ID is stale")
        if start not in resident_blocks_by_start or resident_blocks_by_start[start]["name"] != root["name"]:
            raise BREError(f"resident root {root['name']} has no matching block")
        record_name(root["name"], f"exe:{root['load_offset']}")
        for alias in root.get("aliases", []):
            record_name(alias, f"exe:{root['load_offset']}")
    for block in resident["blocks"]:
        validate_naming(block, f"resident:{block['load_span'][0]}")
        record_name(block["name"], f"exe:{block['load_span'][0]}")
        record_id(block.get("id", ""), f"exe:{block['load_span'][0]}")
        if block["id"] != durable_id(
            "block", "exe", int(block["load_span"][0], 0)
        ):
            raise BREError("resident block durable ID is stale")
        if not block["target_kinds"]:
            raise BREError(f"resident block {block['name']} has no target evidence")
    for chunk in resident["data_chunks"]:
        validate_naming(chunk, f"resident:{chunk['load_span'][0]}")
        if "referenced_by" not in chunk:
            raise BREError(f"resident data chunk {chunk['name']} lacks reference evidence")
        record_name(chunk["name"], f"exe:{chunk['load_span'][0]}")
    if resident["control_flow"]["decode_conflicts"]:
        raise BREError("resident decode-boundary conflicts remain")
    if resident["control_flow"]["unresolved_transfers"]:
        raise BREError("resident unresolved transfers remain")

    procedures_by_id = {
        root["id"]: root
        for unit in catalog["units"]
        for root in unit["roots"]
    }
    procedures_by_id.update({root["id"]: root for root in resident["roots"]})
    procedure_contexts = {
        root["id"]: ("ovr", int(unit["ovr"]["code_offset"], 0))
        for unit in catalog["units"]
        for root in unit["roots"]
    }
    procedure_contexts.update(
        {root["id"]: ("exe", 0) for root in resident["roots"]}
    )
    call_edge_count = call_site_count = 0
    callee_edges = set()
    caller_edges = set()
    for procedure_id, root in procedures_by_id.items():
        storage, base = procedure_contexts[procedure_id]
        for callee in root["callees"]:
            call_edge_count += 1
            call_site_count += len(callee["sites"])
            expected_site_ids = [
                durable_id("site", storage, base + int(site, 0))
                for site in callee["sites"]
            ]
            if callee.get("site_ids") != expected_site_ids:
                raise BREError(
                    f"procedure {procedure_id}: stale callee call-site IDs"
            )
            target_id = callee.get("to_id")
            if target_id is None:
                continue
            if target_id not in procedures_by_id:
                raise BREError(
                    f"procedure {procedure_id}: unknown callee {target_id}"
                )
            if callee.get("to") != procedures_by_id[target_id]["name"]:
                raise BREError(
                    f"procedure {procedure_id}: stale callee display name"
                )
            callee_edges.add(
                (
                    procedure_id,
                    target_id,
                    callee["kind"],
                    tuple(callee["site_ids"]),
                )
            )
        for caller in root["callers"]:
            source_id = caller.get("from_id")
            if source_id not in procedures_by_id:
                raise BREError(
                    f"procedure {procedure_id}: unknown caller {source_id}"
                )
            if caller.get("from") != procedures_by_id[source_id]["name"]:
                raise BREError(
                    f"procedure {procedure_id}: stale caller display name"
                )
            caller_edges.add(
                (
                    source_id,
                    procedure_id,
                    caller["kind"],
                    tuple(caller.get("site_ids", [])),
                )
            )
    if callee_edges != caller_edges:
        raise BREError("durable caller and callee edges are not reciprocal")

    calculated_transfers = catalog.get("calculated_transfers", [])
    calculated_site_ids = set()
    calculated_target_count = 0
    dispatch_ids = set()
    expected_calculated_edges = set()
    for transfer in calculated_transfers:
        dispatch_id = transfer.get("id", "")
        record_id(dispatch_id, dispatch_id)
        if dispatch_id in dispatch_ids:
            raise BREError(f"duplicate calculated transfer {dispatch_id}")
        dispatch_ids.add(dispatch_id)
        if transfer.get("kind") != "calculated_call" or not transfer.get("closed"):
            raise BREError(f"calculated transfer {dispatch_id} is not closed")
        if not transfer.get("site_ids") or not transfer.get("targets"):
            raise BREError(f"calculated transfer {dispatch_id} is empty")
        source_site_ids = set()
        for source in transfer.get("sources", []):
            source_id = source.get("id")
            if source_id not in procedures_by_id:
                raise BREError(
                    f"calculated transfer {dispatch_id}: unknown source {source_id}"
                )
            if source.get("name") != procedures_by_id[source_id]["name"]:
                raise BREError(
                    f"calculated transfer {dispatch_id}: stale source name"
                )
            source_site_ids.update(source.get("site_ids", []))
        for site_id in transfer["site_ids"]:
            if not re.fullmatch(r"bre0988:(?:exe|ovr):site:[0-9a-f]+", site_id):
                raise BREError(
                    f"calculated transfer {dispatch_id}: malformed site {site_id}"
                )
            if site_id in calculated_site_ids:
                raise BREError(f"calculated site {site_id} belongs to two groups")
            calculated_site_ids.add(site_id)
        for target in transfer["targets"]:
            target_id = target.get("id")
            if target_id not in procedures_by_id:
                raise BREError(
                    f"calculated transfer {dispatch_id}: unknown target {target_id}"
                )
            if target.get("name") != procedures_by_id[target_id]["name"]:
                raise BREError(
                    f"calculated transfer {dispatch_id}: stale target name"
                )
            calculated_target_count += 1
            for source in transfer["sources"]:
                expected_calculated_edges.add(
                    (
                        source["id"],
                        target_id,
                        dispatch_id,
                        tuple(source["site_ids"]),
                    )
                )
        if source_site_ids != set(transfer["site_ids"]):
            raise BREError(
                f"calculated transfer {dispatch_id}: source sites do not cover group"
            )
    graph_dispatch_ids = {
        callee["dispatch_id"]
        for root in procedures_by_id.values()
        for callee in root["callees"]
        if callee["kind"] == "calculated_call"
    }
    if graph_dispatch_ids != dispatch_ids:
        raise BREError("calculated-transfer table and call graph do not agree")
    actual_calculated_edges = {
        (
            root["id"],
            callee["to_id"],
            callee["dispatch_id"],
            tuple(callee["site_ids"]),
        )
        for root in procedures_by_id.values()
        for callee in root["callees"]
        if callee["kind"] == "calculated_call"
    }
    if actual_calculated_edges != expected_calculated_edges:
        raise BREError("calculated target memberships are absent or stale")

    blocks_by_id = {
        block["id"]: block
        for unit in catalog["units"]
        for block in unit["blocks"]
    }
    blocks_by_id.update({block["id"]: block for block in resident["blocks"]})
    string_use_count = 0
    for string in catalog.get("string_index", []):
        if string.get("storage") not in {"exe", "ovr"}:
            raise BREError(f"string {string.get('id', 'unknown')}: invalid storage")
        address_key = (
            "ovr_offset" if string["storage"] == "ovr" else "load_offset"
        )
        if address_key not in string:
            raise BREError(
                f"string {string.get('id', 'unknown')}: missing {address_key}"
            )
        location = string.get("ovr_offset", string.get("load_offset", "unknown"))
        record_id(string.get("id", ""), f"string:{location}")
        expected_id = durable_id(
            "string", string["storage"], int(location, 0)
        )
        if string["id"] != expected_id:
            raise BREError(f"string {string['id']}: durable ID is stale")
        if not re.fullmatch(r"[0-9a-f]{64}", string.get("sha256", "")):
            raise BREError(f"string {string['id']}: malformed content hash")
        if string.get("encoding") != "cp437" or string.get("length", 0) <= 0:
            raise BREError(f"string {string['id']}: invalid format metadata")
        for use in string["used_by"]:
            if use["block_id"] not in blocks_by_id:
                raise BREError(
                    f"string {string['id']}: unknown block {use['block_id']}"
                )
            if any(
                identifier not in procedures_by_id
                for identifier in use["procedure_ids"]
            ):
                raise BREError(
                    f"string {string['id']}: unknown procedure in use record"
                )
            if not use["sites"]:
                raise BREError(f"string {string['id']}: use has no instruction site")
            string_use_count += 1

    summary = catalog["summary"]
    expected_counts = {
        "unit_count": len(catalog["units"]),
        "exported_root_count": exported_roots,
        "reachable_procedure_root_count": overlay_roots,
        "basic_block_count": overlay_blocks,
        "data_chunk_count": overlay_data,
        "resident_procedure_root_count": len(resident["roots"]),
        "resident_basic_block_count": len(resident["blocks"]),
        "resident_data_chunk_count": len(resident["data_chunks"]),
        "fixup_count": fixups,
        "referenced_string_count": len(catalog.get("string_index", [])),
        "string_use_count": string_use_count,
        "call_edge_count": call_edge_count,
        "call_site_count": call_site_count,
        "calculated_transfer_group_count": len(calculated_transfers),
        "calculated_transfer_site_count": len(calculated_site_ids),
        "calculated_target_count": calculated_target_count,
    }
    for key, value in expected_counts.items():
        if summary.get(key) != value:
            raise BREError(f"summary {key}={summary.get(key)}, actual={value}")
    expected_semantic_coverage = semantic_coverage(catalog["units"], resident)
    if summary.get("semantic_coverage") != expected_semantic_coverage:
        raise BREError("summary semantic coverage is absent or stale")
    return {
        "unique_names": len(names),
        "overlay_blocks": overlay_blocks,
        "overlay_data_chunks": overlay_data,
        "resident_blocks": len(resident["blocks"]),
        "resident_data_chunks": len(resident["data_chunks"]),
        "referenced_strings": len(catalog.get("string_index", [])),
        "string_uses": string_use_count,
        "call_edges": call_edge_count,
        "call_sites": call_site_count,
        "calculated_transfer_groups": len(calculated_transfers),
        "calculated_transfer_sites": len(calculated_site_ids),
        "calculated_targets": calculated_target_count,
    }


def command_fetch(args: argparse.Namespace) -> None:
    destination = Path(args.destination).resolve()
    destination.mkdir(parents=True, exist_ok=True)
    extractor = shutil.which("7zz") or shutil.which("7z")
    if not extractor:
        raise BREError("fetch requires 7zz or 7z to unpack the official ARJ SFX")
    with tempfile.TemporaryDirectory(prefix="bre-fetch-") as temporary:
        staging = Path(temporary) / "release"
        staging.mkdir()
        if args.archive:
            archive = Path(args.archive).resolve()
        else:
            archive = Path(temporary) / EXPECTED["archive"]["name"]
            print("Downloading the official BRE 0.988 release to a temporary file...", file=sys.stderr)
            request = urllib.request.Request(
                OFFICIAL_URL, headers={"User-Agent": f"immortal-barons-bre-disasm/{VERSION}"}
            )
            with urllib.request.urlopen(request) as response, archive.open("wb") as output:
                shutil.copyfileobj(response, output)
        verify_one(archive, "archive")
        members = ["BRE.EXE", "BRE.OVR"]
        if args.include_docs:
            members.append("BREDATA.EXE")
        result = subprocess.run(
            [extractor, "e", "-y", f"-o{staging}", str(archive), *members],
            text=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )
        if result.returncode:
            raise BREError(f"extractor failed: {result.stderr.strip()}")
        shutil.copy2(staging / "BRE.EXE", destination / "BRE.EXE")
        shutil.copy2(staging / "BRE.OVR", destination / "BRE.OVR")
        if args.include_docs:
            documentation = destination / "reference"
            documentation.mkdir(parents=True, exist_ok=True)
            result = subprocess.run(
                [extractor, "e", "-y", f"-o{documentation}", str(staging / "BREDATA.EXE")],
                text=True,
                stdout=subprocess.DEVNULL,
                stderr=subprocess.PIPE,
            )
            if result.returncode:
                raise BREError(f"BREDATA extractor failed: {result.stderr.strip()}")
            required = ("BRE.DOC", "BREINS.TXT", "RESET.HLP")
            missing = [name for name in required if not (documentation / name).is_file()]
            if missing:
                raise BREError(
                    "BREDATA extraction omitted expected reference files: "
                    + ", ".join(missing)
                )
    verify_one(destination / "BRE.EXE", "exe")
    verify_one(destination / "BRE.OVR", "ovr")
    suffix = " and extracted bundled reference files" if args.include_docs else ""
    print(f"Verified BRE.EXE and BRE.OVR in {destination}{suffix}")


def command_verify(args: argparse.Namespace) -> None:
    exe_path, ovr_path, _exe, _ovr, mz, units = load_release(args)
    print(f"BRE.EXE {sha256(exe_path)}")
    print(f"BRE.OVR {sha256(ovr_path)}")
    print(
        f"MZ header={hx(mz.header_size)}, descriptors={len(units)}, "
        f"roots={sum(len(unit.stubs) for unit in units)}, "
        f"fixups={sum(len(unit.fixups) for unit in units)}"
    )


def command_analyze(args: argparse.Namespace) -> None:
    _ep, _op, exe, ovr, mz, units = load_release(args)
    catalog = build_catalog(exe, ovr, mz, units, not args.no_cfg)
    if not args.no_cfg:
        catalog["validation"] = validate_catalog(catalog)
    rendered = json.dumps(catalog, indent=2, sort_keys=False) + "\n"
    if args.output:
        Path(args.output).write_text(rendered)
    else:
        sys.stdout.write(rendered)


def parse_catalog(path: str) -> dict:
    return json.loads(Path(path).read_text())


def command_check_catalog(args: argparse.Namespace) -> None:
    catalog = parse_catalog(args.catalog)
    result = validate_catalog(catalog)
    if catalog.get("validation") != result:
        raise BREError("catalog validation record is absent or stale")
    print("VALID " + " ".join(f"{key}={value}" for key, value in result.items()))


def catalog_records(catalog: dict, kind: str):
    include = (
        {kind}
        if kind != "all"
        else {"block", "data", "fixup", "dispatch"}
    )
    for unit in catalog["units"]:
        if "procedure" in include:
            for root in unit["roots"]:
                yield {
                    "kind": "procedure",
                    "id": root["id"],
                    "name": root["name"],
                    "address": root["ovr_offset"],
                    "container": unit["id"],
                    "span": root.get("entry_span", [root["entry_offset"], root["entry_offset"]]),
                    "aliases": root.get("aliases", []),
                    "tags": root.get("tags", []),
                    "evidence": root["evidence"],
                    "naming": root["naming"],
                    "callers": root.get("callers", []),
                    "callees": root.get("callees", []),
                }
        if "block" in include:
            for block in unit.get("blocks", []):
                yield {
                    "kind": "block",
                    "id": block["id"],
                    "name": block["name"],
                    "address": block["ovr_span"][0],
                    "container": unit["id"],
                    "span": block["unit_span"],
                    "aliases": block.get("aliases", []),
                    "tags": block["tags"],
                    "evidence": ",".join(block["target_kinds"]),
                    "naming": block["naming"],
                }
        if "data" in include:
            for chunk in unit.get("data_chunks", []):
                yield {
                    "kind": "data",
                    "name": chunk["name"],
                    "address": chunk["ovr_span"][0],
                    "container": unit["id"],
                    "span": chunk["unit_span"],
                    "aliases": chunk.get("aliases", []),
                    "tags": chunk["tags"],
                    "evidence": chunk["classification"],
                    "naming": chunk["naming"],
                }
        if "fixup" in include:
            chunk = unit.get("fixup_chunk")
            if chunk:
                yield {
                    "kind": "fixup",
                    "name": chunk["name"],
                    "address": chunk["ovr_span"][0],
                    "container": unit["id"],
                    "span": chunk["ovr_span"],
                    "aliases": [],
                    "tags": ["overlay", "fixup"],
                    "evidence": chunk["classification"],
                    "naming": chunk["naming"],
                }
    resident = catalog.get("resident_image")
    if not resident:
        return
    if "procedure" in include:
        for root in resident["roots"]:
            yield {
                "kind": "procedure",
                "id": root["id"],
                "name": root["name"],
                "address": root["logical_address"],
                "container": "resident_exe",
                "span": root["entry_span"],
                "aliases": root.get("aliases", []),
                "tags": root["tags"],
                "evidence": root["evidence"],
                "naming": root["naming"],
                "callers": root.get("callers", []),
                "callees": root.get("callees", []),
            }
    if "block" in include:
        for block in resident["blocks"]:
            yield {
                "kind": "block",
                "id": block["id"],
                "name": block["name"],
                "address": block["logical_addresses"][0],
                "container": "resident_exe",
                "span": block["load_span"],
                "aliases": block.get("aliases", []),
                "tags": block["tags"],
                "evidence": ",".join(block["target_kinds"]),
                "naming": block["naming"],
            }
    if "data" in include:
        for chunk in resident["data_chunks"]:
            yield {
                "kind": "data",
                "name": chunk["name"],
                "address": chunk["exe_span"][0],
                "container": "resident_exe",
                "span": chunk["load_span"],
                "aliases": [],
                "tags": chunk["tags"],
                "evidence": chunk["classification"],
                "naming": chunk["naming"],
            }
    if "dispatch" in include:
        for transfer in catalog.get("calculated_transfers", []):
            yield {
                "kind": "dispatch",
                "id": transfer["id"],
                "name": transfer["id"].removeprefix("bre0988:dispatch:"),
                "address": transfer["site_ids"][0],
                "container": "calculated_transfers",
                "span": transfer["site_ids"],
                "aliases": [],
                "tags": ["calculated-transfer", "closed-target-set"],
                "evidence": transfer["evidence"],
                "naming": naming_metadata(
                    "structural", "proven", transfer["evidence"]
                ),
                "closed": transfer["closed"],
                "model": transfer["model"],
                "site_ids": transfer["site_ids"],
                "sources": transfer["sources"],
                "targets": transfer["targets"],
            }


def list_rows(catalog: dict, pattern: str | None, kind: str, status: str | None):
    needle = pattern.lower() if pattern else None
    for record in catalog_records(catalog, kind):
        haystack = " ".join(
            [
                record["name"],
                record["container"],
                record["evidence"],
                *record["aliases"],
                *record["tags"],
            ]
        ).lower()
        if (not needle or needle in haystack) and (
            status is None or record["naming"]["status"] == status
        ):
            yield record


def command_list(args: argparse.Namespace) -> None:
    catalog = parse_catalog(args.catalog)
    rows = list(list_rows(catalog, args.filter, args.kind, args.status))
    if args.format == "json":
        print(json.dumps(rows, indent=2))
        return
    if args.format == "markdown":
        print("| Kind | Name | Status | Address | Container | Span | Evidence |")
        print("|---|---|---|---:|---|---|---|")
        for row in rows:
            print(
                "| "
                + " | ".join(
                    [
                        row["kind"],
                        row["name"],
                        row["naming"]["status"],
                        row["address"],
                        row["container"],
                        "-".join(row["span"]),
                        row["evidence"],
                    ]
                )
                + " |"
            )
        return
    print("kind\tname\tstatus\taddress\tcontainer\tspan\tevidence")
    for row in rows:
        print(
            "\t".join(
                [
                    row["kind"],
                    row["name"],
                    row["naming"]["status"],
                    row["address"],
                    row["container"],
                    "-".join(row["span"]),
                    row["evidence"],
                ]
            )
        )


def command_lookup(args: argparse.Namespace) -> None:
    catalog = parse_catalog(args.catalog)
    needle = args.name.lower()
    matches = [
        record
        for kind in ("procedure", "block", "data", "fixup", "dispatch")
        for record in catalog_records(catalog, kind)
        if needle == record["name"].lower()
        or needle == record.get("id", "").lower()
        or needle in {alias.lower() for alias in record["aliases"]}
    ]
    unique = {
        (record["name"], record["address"], record["kind"]): record
        for record in matches
    }
    if not unique:
        raise BREError(f"no catalog name or alias matches {args.name!r}")
    print(json.dumps(list(unique.values()), indent=2))


def resolve_procedure(catalog: dict, selector: str) -> tuple[str, dict]:
    """Resolve one procedure without discarding its catalog-only span metadata."""
    needle = selector.lower()
    matches = []
    for unit in catalog["units"]:
        for root in unit["roots"]:
            if (
                needle == root["name"].lower()
                or needle == root["id"].lower()
                or needle in {alias.lower() for alias in root.get("aliases", [])}
            ):
                matches.append((unit["id"], root))
    for root in catalog["resident_image"]["roots"]:
        if (
            needle == root["name"].lower()
            or needle == root["id"].lower()
            or needle in {alias.lower() for alias in root.get("aliases", [])}
        ):
            matches.append(("resident_exe", root))
    unique = {(container, root["id"]): (container, root) for container, root in matches}
    if not unique:
        raise BREError(f"no procedure name, durable ID, or alias matches {selector!r}")
    if len(unique) != 1:
        raise BREError(f"procedure selector {selector!r} is ambiguous")
    return next(iter(unique.values()))


def command_find_string(args: argparse.Namespace) -> None:
    """Resolve a substring through the non-text catalog index and private binaries."""
    catalog = parse_catalog(args.catalog)
    if catalog.get("format_version") != 6 or "string_index" not in catalog:
        raise BREError("catalog does not contain the durable string index")
    validate_catalog(catalog)
    _ep, _op, exe, ovr, mz, _units = load_release(args)
    procedures = {
        root["id"]: {
            "id": root["id"],
            "name": root["name"],
            "address": root["ovr_offset"],
            "aliases": root.get("aliases", []),
        }
        for unit in catalog["units"]
        for root in unit["roots"]
    }
    procedures.update(
        {
            root["id"]: {
                "id": root["id"],
                "name": root["name"],
                "address": root["logical_address"],
                "aliases": root.get("aliases", []),
            }
            for root in catalog["resident_image"]["roots"]
        }
    )
    function_ids = None
    if args.function:
        function_ids = {
            resolve_procedure(catalog, selector)[1]["id"]
            for selector in args.function
        }
    blocks = {
        block["id"]: {
            "id": block["id"],
            "name": block["name"],
            "address": block["ovr_span"][0],
        }
        for unit in catalog["units"]
        for block in unit["blocks"]
    }
    blocks.update(
        {
            block["id"]: {
                "id": block["id"],
                "name": block["name"],
                "address": block["logical_addresses"][0],
            }
            for block in catalog["resident_image"]["blocks"]
        }
    )
    query = args.substring if args.case_sensitive else args.substring.casefold()
    matches = []
    matched_procedure_ids = set()
    matched_block_ids = set()
    for record in catalog["string_index"]:
        if record["storage"] == "ovr":
            address = int(record["ovr_offset"], 0)
            source = ovr
            display_address = record["ovr_offset"]
        else:
            load_offset = int(record["load_offset"], 0)
            address = mz.header_size + load_offset
            source = exe
            display_address = record["load_offset"]
        length = source[address]
        payload = source[address + 1 : address + 1 + length]
        content_hash = hashlib.sha256(payload).hexdigest()
        if length != record["length"] or content_hash != record["sha256"]:
            raise BREError(
                f"catalog string {record['id']} does not match the pinned binary"
            )
        text_value = payload.decode("cp437")
        haystack = text_value if args.case_sensitive else text_value.casefold()
        if query not in haystack:
            continue
        block_uses = {}
        procedure_ids = set()
        for use in record["used_by"]:
            use_procedure_ids = set(use["procedure_ids"])
            if function_ids is not None:
                use_procedure_ids &= function_ids
            if not use_procedure_ids:
                continue
            block_use = block_uses.setdefault(
                use["block_id"],
                {**blocks[use["block_id"]], "sites": set(), "kinds": set()},
            )
            block_use["sites"].update(use["sites"])
            block_use["kinds"].add(use["kind"])
            matched_block_ids.add(use["block_id"])
            procedure_ids.update(use_procedure_ids)
        if not procedure_ids:
            continue
        matched_procedure_ids.update(procedure_ids)
        matches.append(
            {
                "string_id": record["id"],
                "storage": record["storage"],
                "address": display_address,
                "text": text_value,
                "blocks": [
                    {
                        **{
                            key: value
                            for key, value in block.items()
                            if key not in {"sites", "kinds"}
                        },
                        "sites": sorted(block["sites"]),
                        "kinds": sorted(block["kinds"]),
                    }
                    for block in sorted(
                        block_uses.values(), key=lambda item: item["id"]
                    )
                ],
                "functions": [
                    procedures[identifier]
                    for identifier in sorted(procedure_ids)
                ],
            }
        )
    result = {
        "query": args.substring,
        "case_sensitive": args.case_sensitive,
        "function_filter": sorted(function_ids) if function_ids is not None else None,
        "matching_strings": len(matches),
        "functions": [
            procedures[identifier] for identifier in sorted(matched_procedure_ids)
        ],
        "blocks": [blocks[identifier] for identifier in sorted(matched_block_ids)],
    }
    if args.details:
        result["matches"] = matches
    print(json.dumps(result, indent=2, ensure_ascii=False))


def command_map(args: argparse.Namespace) -> None:
    _ep, _op, _exe, _ovr, mz, units = load_release(args)
    if args.address:
        try:
            segment_s, offset_s = args.address.split(":", 1)
            segment, offset = int(segment_s, 16), int(offset_s, 16)
        except ValueError as exc:
            raise BREError("address must be a hexadecimal segment:offset") from exc
        for unit in units:
            if segment == unit.descriptor_segment:
                for stub in unit.stubs:
                    stub_offset = 0x20 + stub.index * 5
                    if offset == stub_offset:
                        absolute = unit.ovr_offset + stub.entry_offset
                        print(
                            f"{segment:04x}:{offset:04x} -> {unit.unit_id}+{hx(stub.entry_offset)} "
                            f"= BRE.OVR {hx(absolute, 6)} ({root_name(unit, stub.entry_offset)})"
                        )
                        return
                print(f"{args.address} is in descriptor {unit.unit_id}, but is not a stub root")
                return
        known = RESIDENT_NAMES.get((segment, offset))
        if known:
            print(f"{args.address} = {known[0]}")
            return
        raise BREError(f"{args.address} is not a known overlay stub or resident name")
    if args.exe_offset is not None:
        value = args.exe_offset
        for unit in units:
            descriptor_end = unit.descriptor_file_offset + 0x20 + len(unit.stubs) * 5
            if unit.descriptor_file_offset <= value < descriptor_end:
                for stub in unit.stubs:
                    if value == stub.file_offset:
                        absolute = unit.ovr_offset + stub.entry_offset
                        print(
                            f"BRE.EXE {hx(value, 6)} -> "
                            f"{unit.descriptor_segment:04x}:{0x20 + stub.index * 5:04x} -> "
                            f"BRE.OVR {hx(absolute, 6)} ({root_name(unit, stub.entry_offset)})"
                        )
                        return
                print(
                    f"BRE.EXE {hx(value, 6)} is descriptor {unit.unit_id} "
                    f"+{hx(value - unit.descriptor_file_offset)}"
                )
                return
        if mz.header_size <= value < len(_exe):
            logical = value - mz.header_size
            print(
                f"BRE.EXE {hx(value, 6)} -> logical "
                f"{logical // 16:04x}:{logical % 16:04x} (not an overlay descriptor)"
            )
            return
        raise BREError(f"EXE offset {hx(value, 6)} is outside the load module")
    value = args.ovr_offset
    for unit in units:
        if unit.ovr_offset <= value < unit.end_offset:
            if value < unit.fixup_offset:
                relative = value - unit.ovr_offset
                matching = [stub for stub in unit.stubs if stub.entry_offset == relative]
                suffix = f" root={root_name(unit, relative)}" if matching else ""
                print(f"BRE.OVR {hx(value, 6)} -> {unit.unit_id}+{hx(relative)}{suffix}")
            else:
                print(
                    f"BRE.OVR {hx(value, 6)} -> {unit.unit_id} fixup stream "
                    f"+{hx(value - unit.fixup_offset)}"
                )
            return
    raise BREError(f"OVR offset {hx(value, 6)} is outside the overlay payload")


def materialized_code(ovr: bytes, unit: Unit, load_base: int) -> bytes:
    code = bytearray(ovr[unit.ovr_offset : unit.ovr_offset + unit.code_size])
    for offset in unit.fixups:
        value = struct.unpack_from("<H", code, offset)[0]
        struct.pack_into("<H", code, offset, (value + load_base) & 0xFFFF)
    return bytes(code)


def command_materialize(args: argparse.Namespace) -> None:
    _ep, _op, _exe, ovr, _mz, units = load_release(args)
    unit = select_unit(units, args.unit)
    Path(args.output).write_bytes(materialized_code(ovr, unit, args.load_base))
    print(
        f"Wrote {unit.code_size} bytes for {unit.unit_id}, applying "
        f"{len(unit.fixups)} fixups with load base {hx(args.load_base)}"
    )


def command_compare_dump(args: argparse.Namespace) -> None:
    _ep, _op, _exe, ovr, _mz, units = load_release(args)
    unit = select_unit(units, args.unit)
    dump = Path(args.dump).read_bytes()
    if len(dump) != unit.code_size:
        raise BREError(f"dump has {len(dump)} bytes; expected {unit.code_size}")
    raw = ovr[unit.ovr_offset : unit.ovr_offset + unit.code_size]
    bases = []
    permitted = set()
    for offset in unit.fixups:
        before = struct.unpack_from("<H", raw, offset)[0]
        after = struct.unpack_from("<H", dump, offset)[0]
        bases.append((after - before) & 0xFFFF)
        permitted.update((offset, offset + 1))
    unexplained = [i for i, (a, b) in enumerate(zip(raw, dump)) if a != b and i not in permitted]
    unique = sorted(set(bases))
    if len(unique) != 1 or unexplained:
        raise BREError(
            f"dump does not match the relocation model: bases={','.join(map(hx, unique))}, "
            f"unexplained_changed_bytes={len(unexplained)}"
        )
    expected = materialized_code(ovr, unit, unique[0])
    if expected != dump:
        raise BREError("dump has missing or partial fixup changes")
    print(
        f"MATCH {unit.unit_id}: {len(unit.fixups)} fixups, "
        f"DOS load base {hx(unique[0])}, no unexplained changes"
    )


def disassembly_contexts(
    exe: bytes,
    ovr: bytes,
    mz: MZHeader,
    units: list[Unit],
    catalog: dict,
    load_base: int,
) -> list[dict]:
    contexts = []
    for catalog_unit in catalog["units"]:
        unit = next((item for item in units if item.unit_id == catalog_unit["id"]), None)
        if unit is None:
            raise BREError(f"binary has no unit matching catalog {catalog_unit['id']}")
        contexts.append(
            {
                "id": unit.unit_id,
                "storage": "ovr",
                "code": materialized_code(ovr, unit, load_base),
                "absolute_base": unit.ovr_offset,
                "blocks": catalog_unit["blocks"],
                "data_chunks": catalog_unit["data_chunks"],
                "unit": unit,
                "catalog": catalog_unit,
            }
        )
    resident = catalog.get("resident_image")
    if resident:
        contexts.append(
            {
                "id": "resident_exe",
                "storage": "exe",
                "code": exe[mz.header_size :],
                "absolute_base": 0,
                "blocks": resident["blocks"],
                "data_chunks": resident["data_chunks"],
                "catalog": resident,
            }
        )
    return contexts


def context_boundaries(context: dict) -> tuple[dict[int, str], list[tuple[int, int, str]]]:
    if context["storage"] == "ovr":
        labels = {
            int(block["unit_span"][0], 0): block["name"]
            for block in context["blocks"]
        }
        data = [
            (int(chunk["unit_span"][0], 0), int(chunk["unit_span"][1], 0), chunk["name"])
            for chunk in context["data_chunks"]
        ]
    else:
        labels = {
            int(block["load_span"][0], 0): block["name"]
            for block in context["blocks"]
        }
        data = [
            (int(chunk["load_span"][0], 0), int(chunk["load_span"][1], 0), chunk["name"])
            for chunk in context["data_chunks"]
        ]
    return labels, data


def code_regions(size: int, data: list[tuple[int, int, str]]) -> list[tuple[int, int]]:
    """The parts of a context that are NOT catalogued data, in order."""
    regions = []
    cursor = 0
    for start, end, _name in sorted(data):
        start, end = max(0, start), min(size, end)
        if start > cursor:
            regions.append((cursor, start))
        cursor = max(cursor, end)
    if cursor < size:
        regions.append((cursor, size))
    return regions


def ndisasm_context(context: dict) -> list[tuple[int, str]]:
    labels, data = context_boundaries(context)
    code = context["code"]
    # The catalogued data chunks are cut out HERE rather than handed to ndisasm
    # as -k ranges. ndisasm 3.02's -k skips its range and then stops
    # disassembling altogether instead of resuming after it — verified with a
    # four-byte skip, which prints one "skipping" line and nothing else — so a
    # unit that opens with a string block (they routinely do) disassembled to a
    # single line, and asking for a procedure past that point produced no output
    # at all while still exiting 0.
    #
    # Each code region is disassembled on its own with -o set to its true start,
    # so offsets stay honest and no region can be thrown out of step by the data
    # in front of it.
    binary = shutil.which("ndisasm") or "ndisasm"
    lines = []
    for start, end in code_regions(len(code), data):
        with tempfile.NamedTemporaryFile(prefix="bre-disasm-", suffix=".bin") as stream:
            stream.write(code[start:end])
            stream.flush()
            command = [binary, "-b", "16", "-a", "-o", str(start)]
            for label in labels:
                if start <= label < end:
                    command += ["-s", str(label)]
            command.append(stream.name)
            result = subprocess.run(command, check=True, text=True, capture_output=True)
        for line in result.stdout.splitlines():
            match = NDISASM_LINE.match(line)
            if match:
                lines.append((int(match.group(1), 16), line))
    lines.sort(key=lambda item: item[0])
    return lines


def containing_code_block(context: dict, target: int) -> tuple[int, int, str]:
    span_key = "unit_span" if context["storage"] == "ovr" else "load_span"
    candidates = []
    for block in context["blocks"]:
        start, end = (int(value, 0) for value in block[span_key])
        if start <= target < end:
            candidates.append((start, end, block["name"]))
    if not candidates:
        raise BREError(
            f"{context['id']} offset {hx(target, 5)} is not inside catalogued code; "
            "refusing to guess a disassembly boundary"
        )
    return max(candidates, key=lambda item: item[0])


def print_disassembly_lines(
    context: dict,
    lines: list[tuple[int, str]],
    ranges: list[tuple[int, int]],
) -> None:
    labels, data = context_boundaries(context)
    data_labels = {start: name for start, _end, name in data}
    announced_data = set()
    for offset, line in lines:
        if not any(start <= offset < end for start, end in ranges):
            continue
        if offset in labels:
            print(f"\n{labels[offset]}:")
        if offset in data_labels:
            print(f"\n{data_labels[offset]}:")
            announced_data.add(offset)
        print(line)
    for offset, name in sorted(data_labels.items()):
        if offset not in announced_data and any(start <= offset < end for start, end in ranges):
            print(f"\n{name}: ; cataloged non-code span at {hx(offset)}")


def around_range(
    context: dict,
    lines: list[tuple[int, str]],
    target: int,
    instructions: int,
) -> tuple[tuple[int, int, str], list[tuple[int, str]], list[tuple[int, int]]]:
    anchor = containing_code_block(context, target)
    _labels, data = context_boundaries(context)
    instruction_lines = [
        item
        for item in lines
        if not any(start <= item[0] < end for start, end, _name in data)
    ]
    index = next(
        (number for number, (offset, _line) in enumerate(instruction_lines) if offset >= target),
        None,
    )
    if index is None:
        raise BREError(f"no instruction follows {context['id']} offset {hx(target, 5)}")
    before = instructions // 2
    anchor_index = next(
        number
        for number, (offset, _line) in enumerate(instruction_lines)
        if offset >= anchor[0]
    )
    first = max(anchor_index, index - before)
    selected = instruction_lines[first : first + instructions]
    if not selected:
        raise BREError("empty disassembly window")
    return anchor, selected, [(selected[0][0], selected[-1][0] + 16)]


def resolve_disassembly_target(contexts: list[dict], catalog: dict, selector: str) -> tuple[dict, int]:
    if selector.startswith("bre0988:ovr:site:"):
        value = int(selector.rsplit(":", 1)[1], 16)
    elif selector.startswith("bre0988:exe:site:"):
        value = int(selector.rsplit(":", 1)[1], 16)
        return next(item for item in contexts if item["storage"] == "exe"), value
    elif ":" in selector and not selector.startswith("bre0988:"):
        try:
            segment_text, offset_text = selector.split(":", 1)
            value = int(segment_text, 16) * 16 + int(offset_text, 16)
        except ValueError as exc:
            raise BREError("logical address must be hexadecimal SEGMENT:OFFSET") from exc
        return next(item for item in contexts if item["storage"] == "exe"), value
    else:
        try:
            value = int(selector, 0)
        except ValueError:
            container, root = resolve_procedure(catalog, selector)
            context = next(item for item in contexts if item["id"] == container)
            key = "entry_offset" if container != "resident_exe" else "load_offset"
            return context, int(root[key], 0)
    for context in contexts:
        if context["storage"] != "ovr":
            continue
        start = context["absolute_base"]
        if start <= value < start + len(context["code"]):
            return context, value - start
    raise BREError(f"OVR address {hx(value, 6)} is outside every code unit")


def command_disasm(args: argparse.Namespace) -> None:
    if args.instructions <= 0:
        raise BREError("--instructions must be positive")
    if not args.unit and (args.start is not None or args.end is not None):
        raise BREError("--start and --end require --unit")
    _ep, _op, exe, ovr, mz, units = load_release(args)
    catalog = parse_catalog(args.catalog)
    contexts = disassembly_contexts(exe, ovr, mz, units, catalog, args.load_base)
    if args.unit:
        unit = select_unit(units, args.unit)
        context = next(item for item in contexts if item["id"] == unit.unit_id)
        start = args.start if args.start is not None else 0
        end = args.end if args.end is not None else len(context["code"])
        if not 0 <= start < end <= len(context["code"]):
            raise BREError("disassembly range is outside the selected unit")
        ranges = [(start, end)]
        target = None
    elif args.procedure:
        container, root = resolve_procedure(catalog, args.procedure)
        context = next(item for item in contexts if item["id"] == container)
        ranges = [(int(start, 0), int(end, 0)) for start, end in root["body_ranges"]]
        target = None
    else:
        context, target = resolve_disassembly_target(contexts, catalog, args.around)
        ranges = []
    lines = ndisasm_context(context)
    if target is not None:
        anchor, selected, ranges = around_range(context, lines, target, args.instructions)
        print(
            f"; {context['id']} requested={hx(target, 5)} synchronized="
            f"{hx(anchor[0], 5)} ({anchor[2]})"
        )
        lines = selected
    print_disassembly_lines(context, lines, ranges)


def command_xrefs(args: argparse.Namespace) -> None:
    if args.context <= 0 or args.max_sites <= 0:
        raise BREError("--context and --max-sites must be positive")
    catalog = parse_catalog(args.catalog)
    container, root = resolve_procedure(catalog, args.name)
    record = next(
        item
        for item in catalog_records(catalog, "procedure")
        if item["id"] == root["id"]
    )
    if not args.show_sites:
        selected = dict(record)
        if args.direction == "callers":
            selected.pop("callees", None)
        elif args.direction == "callees":
            selected.pop("callers", None)
        print(json.dumps(selected, indent=2))
        return
    if not args.directory and not args.exe:
        raise BREError("--show-sites requires --directory or --exe/--ovr")
    _ep, _op, exe, ovr, mz, units = load_release(args)
    contexts = disassembly_contexts(exe, ovr, mz, units, catalog, 0)
    edges = []
    if args.direction in ("callers", "both"):
        for edge in root.get("callers", []):
            for site_id in edge["site_ids"]:
                edges.append((f"caller {edge['from']} -> {root['name']} ({edge['kind']})", site_id))
    if args.direction in ("callees", "both"):
        for edge in root.get("callees", []):
            for site_id in edge["site_ids"]:
                edges.append((f"callee {root['name']} -> {edge['to']} ({edge['kind']})", site_id))
    print(f"{root['name']} [{root['id']}] container={container}")
    if len(edges) > args.max_sites:
        print(f"; showing {args.max_sites} of {len(edges)} sites")
        edges = edges[: args.max_sites]
    cache = {}
    for heading, site_id in edges:
        context, target = resolve_disassembly_target(contexts, catalog, site_id)
        if context["id"] not in cache:
            cache[context["id"]] = ndisasm_context(context)
        lines = cache[context["id"]]
        anchor, selected, ranges = around_range(context, lines, target, args.context)
        print(
            f"\n## {heading} at {site_id}\n"
            f"; {context['id']} requested={hx(target, 5)} synchronized="
            f"{hx(anchor[0], 5)} ({anchor[2]})"
        )
        print_disassembly_lines(context, selected, ranges)


def command_debugger(args: argparse.Namespace) -> None:
    directory = Path(args.directory).resolve()
    verify_one(directory / "BRE.EXE", "exe")
    verify_one(directory / "BRE.OVR", "ovr")
    if not args.run:
        print("Verified debugger workflow:")
        print("  1. Start Xvfb, then DOSBox with TERM=xterm and DISPLAY set.")
        print("  2. Send Alt-Pause to the DOSBox X window to enter the heavy debugger.")
        print("  3. At the debugger, Enter enters command mode; use BPINT 3F, then F5.")
        print("  4. At an overlay trap, F10 steps through the loader; MEMDUMPBIN SEG:0 SIZE dumps a unit.")
        print("Run this command with --run to launch that Xvfb-backed debugger.")
        return
    xvfb = shutil.which("Xvfb")
    dosbox = shutil.which("dosbox")
    xdotool = shutil.which("xdotool")
    if not all((xvfb, dosbox, xdotool)):
        raise BREError("debugger --run requires Xvfb, dosbox, and xdotool")
    display = None
    server = None
    for number in range(170, 220):
        if not Path(f"/tmp/.X11-unix/X{number}").exists():
            display = f":{number}"
            server = subprocess.Popen(
                [xvfb, display, "-screen", "0", "1024x768x24"],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            break
    if server is None or display is None:
        raise BREError("could not allocate an Xvfb display")
    env = os.environ.copy()
    env["DISPLAY"] = display
    env["TERM"] = "xterm"
    process = None
    try:
        process = subprocess.Popen(
            [dosbox, "-conf", "/dev/null", "-c", f"mount c {directory}", "-c", "c:"],
            env=env,
        )
        time.sleep(2)
        subprocess.run(
            [xdotool, "search", "--sync", "--name", "DOSBox", "key", "alt+Pause"],
            env=env,
            check=True,
        )
        print("Debugger active. Press Enter before each debugger command; F5 continues.", file=sys.stderr)
        process.wait()
    finally:
        if process is not None and process.poll() is None:
            process.terminate()
        server.terminate()
        server.wait(timeout=5)


def add_binary_arguments(parser: argparse.ArgumentParser) -> None:
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument("--directory", help="directory containing BRE.EXE and BRE.OVR")
    group.add_argument("--exe", help="path to BRE.EXE (also pass --ovr)")
    parser.add_argument("--ovr", help="path to BRE.OVR")


def add_optional_binary_arguments(parser: argparse.ArgumentParser) -> None:
    group = parser.add_mutually_exclusive_group()
    group.add_argument("--directory", help="directory containing BRE.EXE and BRE.OVR")
    group.add_argument("--exe", help="path to BRE.EXE (also pass --ovr)")
    parser.add_argument("--ovr", help="path to BRE.OVR")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    fetch = subparsers.add_parser("fetch", help="explicitly fetch the official BRE 0.988 binaries")
    fetch.add_argument("destination")
    fetch.add_argument(
        "--archive",
        help="verify and extract an already-downloaded official archive instead of using the network",
    )
    fetch.add_argument(
        "--include-docs",
        action="store_true",
        help="also extract the bundled BREDATA reference files into DESTINATION/reference",
    )
    fetch.set_defaults(func=command_fetch)

    verify = subparsers.add_parser("verify", help="verify hashes and overlay structure")
    add_binary_arguments(verify)
    verify.set_defaults(func=command_verify)

    analyze = subparsers.add_parser("analyze", help="build a non-proprietary address catalog")
    add_binary_arguments(analyze)
    analyze.add_argument("--output", "-o")
    analyze.add_argument("--no-cfg", action="store_true", help="omit ndisasm reachability analysis")
    analyze.set_defaults(func=command_analyze)

    check = subparsers.add_parser(
        "check-catalog", help="validate exhaustive names, spans, and coverage"
    )
    check.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    check.set_defaults(func=command_check_catalog)

    listing = subparsers.add_parser("list", help="list named records from a generated catalog")
    listing.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    listing.add_argument("--filter")
    listing.add_argument(
        "--kind",
        choices=("procedure", "block", "data", "fixup", "dispatch", "all"),
        default="procedure",
    )
    listing.add_argument(
        "--status",
        choices=("identified", "contextual", "structural", "unclassified"),
        help="limit output to one semantic naming status",
    )
    listing.add_argument("--format", choices=("tsv", "markdown", "json"), default="tsv")
    listing.set_defaults(func=command_list)

    lookup = subparsers.add_parser(
        "lookup", help="look up an exact stable name, durable ID, or alias"
    )
    lookup.add_argument("name")
    lookup.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    lookup.set_defaults(func=command_lookup)

    find_string = subparsers.add_parser(
        "find-string",
        help="find private-binary Pascal strings by substring and list referencing functions",
    )
    add_binary_arguments(find_string)
    find_string.add_argument("substring")
    find_string.add_argument(
        "--catalog", default="docs/dev/bre-v0988-disassembly.json"
    )
    find_string.add_argument("--case-sensitive", action="store_true")
    find_string.add_argument(
        "--function",
        action="append",
        help="limit uses to an exact procedure name, durable ID, or alias (repeatable)",
    )
    find_string.add_argument(
        "--details",
        action="store_true",
        help="include matching private text and per-string use sites",
    )
    find_string.set_defaults(func=command_find_string)

    mapping = subparsers.add_parser("map", help="map an EXE stub or OVR file offset")
    add_binary_arguments(mapping)
    inputs = mapping.add_mutually_exclusive_group(required=True)
    inputs.add_argument("--address", help="logical EXE segment:offset")
    inputs.add_argument("--exe-offset", type=parse_int, help="BRE.EXE file offset")
    inputs.add_argument("--ovr-offset", type=parse_int)
    mapping.set_defaults(func=command_map)

    materialize = subparsers.add_parser("materialize", help="extract and relocate one overlay unit")
    add_binary_arguments(materialize)
    materialize.add_argument("--unit", required=True)
    materialize.add_argument("--load-base", type=parse_int, default=0)
    materialize.add_argument("--output", "-o", required=True)
    materialize.set_defaults(func=command_materialize)

    compare = subparsers.add_parser("compare-dump", help="validate a DOSBox MEMDUMPBIN image")
    add_binary_arguments(compare)
    compare.add_argument("--unit", required=True)
    compare.add_argument("--dump", required=True)
    compare.set_defaults(func=command_compare_dump)

    disasm = subparsers.add_parser(
        "disasm", help="disassemble a unit, procedure, or synchronized address window"
    )
    add_binary_arguments(disasm)
    disasm_selector = disasm.add_mutually_exclusive_group(required=True)
    disasm_selector.add_argument("--unit")
    disasm_selector.add_argument("--procedure", help="exact name, durable ID, or alias")
    disasm_selector.add_argument(
        "--around",
        help="OVR offset, resident SEG:OFF, site ID, or exact procedure selector",
    )
    disasm.add_argument("--start", type=parse_int, help="first unit-relative byte to print")
    disasm.add_argument("--end", type=parse_int, help="exclusive unit-relative byte to print")
    disasm.add_argument(
        "--instructions",
        type=int,
        default=40,
        help="total instructions in an --around window (default: 40)",
    )
    disasm.add_argument("--load-base", type=parse_int, default=0)
    disasm.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    disasm.set_defaults(func=command_disasm)

    xrefs = subparsers.add_parser(
        "xrefs", help="show a procedure's call graph and synchronized call-site windows"
    )
    add_optional_binary_arguments(xrefs)
    xrefs.add_argument("name", help="exact procedure name, durable ID, or alias")
    xrefs.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    xrefs.add_argument("--show-sites", action="store_true")
    xrefs.add_argument("--context", type=int, default=12, help="instructions per site")
    xrefs.add_argument(
        "--direction", choices=("callers", "callees", "both"), default="both"
    )
    xrefs.add_argument("--max-sites", type=int, default=40)
    xrefs.set_defaults(func=command_xrefs)

    debugger = subparsers.add_parser("debugger", help="document or launch the Xvfb DOSBox debugger")
    debugger.add_argument("--directory", required=True)
    debugger.add_argument("--run", action="store_true")
    debugger.set_defaults(func=command_debugger)
    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    if getattr(args, "exe", None) and not getattr(args, "ovr", None):
        parser.error("--exe requires --ovr")
    try:
        args.func(args)
    except KeyboardInterrupt:
        print("bre-disasm: interrupted", file=sys.stderr)
        return 130
    except (BREError, OSError, subprocess.CalledProcessError) as exc:
        print(f"bre-disasm: {exc}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
