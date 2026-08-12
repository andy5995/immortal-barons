#!/usr/bin/env python3
"""Map Barren Realms Elite 0.988 overlay stubs without shipping its binaries.

The committed catalog contains addresses, names, and control-flow metadata only.
Users supply their own BRE.EXE and BRE.OVR, or explicitly fetch the official
release with the ``fetch`` subcommand.
"""

from __future__ import annotations

import argparse
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


VERSION = "0.1"
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


# These are stable public-analysis names, not symbols recovered from BRE.
RESIDENT_NAMES = {
    (0x0C03, 0x0ED0): ("random_u16", ["rng"]),
    (0x0C03, 0x0F10): ("add_i32_indirect", ["integer", "rtl"]),
    (0x0FD0, 0x0ECC): ("mul_i32", ["integer", "rtl"]),
    (0x0FD0, 0x0F09): ("div_i32", ["integer", "rtl"]),
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
    0x2EA09: ("region_cost", ["regions", "economy"]),
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
    if absolute in LANDMARKS:
        return LANDMARKS[absolute][0]
    return f"{unit.unit_id}_entry_{entry:04x}"


NDISASM_LINE = re.compile(r"^([0-9A-Fa-f]{8})\s+([0-9A-Fa-f]+)\s+(.+?)\s*$")


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
        for byte in range(offset, offset + decoded.size):
            byte_owners[byte] = offset

        mnemonic = decoded.mnemonic.lower()
        is_call = decoded.group(capstone.CS_GRP_CALL)
        is_jump = decoded.group(capstone.CS_GRP_JUMP)
        immediate_operands = [operand.imm for operand in decoded.operands if operand.type == X86_OP_IMM]
        far_target = None
        near_target = None
        if is_call and mnemonic in {"lcall", "callf"} and len(immediate_operands) == 2:
            far_target = (immediate_operands[0] & 0xFFFF, immediate_operands[1] & 0xFFFF)
        elif (is_call or is_jump) and len(decoded.operands) == 1 and immediate_operands:
            near_target = immediate_operands[0] & 0xFFFF

        if near_target is not None:
            block_targets.add(near_target)
            if is_call:
                call_targets.add(near_target)
            if near_target < len(code):
                queue.append(near_target)
        elif far_target is not None and is_call:
            segment, destination = far_target
            resolved = descriptor_roots.get((segment, destination))
            if resolved:
                edges.add((offset, "overlay_call", f"{resolved[0]}:{resolved[1]}"))
            else:
                known = RESIDENT_NAMES.get((segment, destination))
                label = known[0] if known else f"resident_{segment:04x}_{destination:04x}"
                edges.add((offset, "far_call", label))
        elif is_call or is_jump:
            unresolved.add((offset, f"{decoded.mnemonic} {decoded.op_str}".strip()))

        unconditional_jump = is_jump and mnemonic in {"jmp", "ljmp"}
        returns = decoded.group(capstone.CS_GRP_RET) or mnemonic == "iret"
        falls_through = not (unconditional_jump or returns or mnemonic in {"int3", "hlt"})
        if falls_through and offset + decoded.size < len(code):
            queue.append(offset + decoded.size)

    ranges = ranges_from_instructions(instructions, visited)
    procedure_roots = set(roots) | {target for target in call_targets if target in visited}
    entry_spans = {}
    for root in sorted(procedure_roots):
        cursor = root
        end = root
        seen = set()
        while cursor in visited and cursor not in seen:
            if cursor != root and cursor in block_targets:
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
        entry_spans[root] = end
    holes = []
    cursor = 0
    for start, end in ranges:
        if cursor < start:
            holes.append([cursor, start])
        cursor = max(cursor, end)
    if cursor < len(code):
        holes.append([cursor, len(code)])
    grouped_edges = {}
    for offset, kind, target in edges:
        grouped_edges.setdefault((kind, target), []).append(offset)
    return {
        "reachable_ranges": [[hx(a), hx(b)] for a, b in ranges],
        "unreached_ranges": [[hx(a), hx(b)] for a, b in holes],
        "external_edges": [
            {
                "kind": kind,
                "to": target,
                "sites": [hx(site) for site in sorted(sites)],
            }
            for (kind, target), sites in sorted(
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
        "_procedure_roots": sorted(procedure_roots),
        "_block_roots": sorted(target for target in block_targets if target in visited),
        "_entry_spans": entry_spans,
    }


def build_catalog(exe: bytes, ovr: bytes, mz: MZHeader, units: list[Unit], cfg: bool):
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
            name = root_name(unit, stub.entry_offset)
            aliases = []
            tags = ["overlay", "exported"]
            if absolute in LANDMARKS:
                tags.extend(LANDMARKS[absolute][1])
            roots.append(
                {
                    "name": name,
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
            exported = {stub.entry_offset for stub in unit.stubs}
            for entry in flow.pop("_procedure_roots"):
                if entry in exported:
                    matching = next(root for root in roots if int(root["entry_offset"], 0) == entry)
                    matching["entry_span"] = [hx(entry), hx(flow["_entry_spans"][entry])]
                    continue
                absolute = unit.ovr_offset + entry
                if absolute in LANDMARKS:
                    name, semantic_tags = LANDMARKS[absolute]
                else:
                    name, semantic_tags = f"{unit.unit_id}_proc_{entry:04x}", []
                roots.append(
                    {
                        "name": name,
                        "entry_offset": hx(entry),
                        "entry_span": [hx(entry), hx(flow["_entry_spans"][entry])],
                        "ovr_offset": hx(absolute, 6),
                        "stub": None,
                        "aliases": [],
                        "tags": sorted(set(["overlay", "near-call-target", *semantic_tags])),
                        "confidence": "proven",
                        "evidence": "direct near call from exported-root-reachable code",
                    }
                )
            flow.pop("_block_roots")
            flow.pop("_entry_spans")
            roots.sort(key=lambda root: int(root["entry_offset"], 0))
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
    resident = [
        {
            "name": name,
            "logical_address": f"{segment:04x}:{offset:04x}",
            "tags": tags,
            "confidence": "repository-cited",
        }
        for (segment, offset), (name, tags) in sorted(RESIDENT_NAMES.items())
    ]
    return {
        "format": "immortal-barons-bre-disassembly-map",
        "format_version": 1,
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
            "fixup_count": sum(len(unit.fixups) for unit in units),
            "ovr_payload_start": hx(8),
            "ovr_payload_end": hx(len(ovr), 6),
        },
        "resident_roots": resident,
        "landmarks": landmarks,
        "units": catalog_units,
    }


def command_fetch(args: argparse.Namespace) -> None:
    destination = Path(args.destination).resolve()
    destination.mkdir(parents=True, exist_ok=True)
    extractor = shutil.which("7zz") or shutil.which("7z")
    if not extractor:
        raise BREError("fetch requires 7zz or 7z to unpack the official ARJ SFX")
    with tempfile.TemporaryDirectory(prefix="bre-fetch-") as temporary:
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
        result = subprocess.run(
            [extractor, "e", "-y", f"-o{destination}", str(archive), "BRE.EXE", "BRE.OVR"],
            text=True,
            stdout=subprocess.DEVNULL,
            stderr=subprocess.PIPE,
        )
        if result.returncode:
            raise BREError(f"extractor failed: {result.stderr.strip()}")
    verify_one(destination / "BRE.EXE", "exe")
    verify_one(destination / "BRE.OVR", "ovr")
    print(f"Verified BRE.EXE and BRE.OVR in {destination}")


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
    rendered = json.dumps(catalog, indent=2, sort_keys=False) + "\n"
    if args.output:
        Path(args.output).write_text(rendered)
    else:
        sys.stdout.write(rendered)


def parse_catalog(path: str) -> dict:
    return json.loads(Path(path).read_text())


def list_rows(catalog: dict, pattern: str | None):
    needle = pattern.lower() if pattern else None
    for unit in catalog["units"]:
        ranges = unit.get("control_flow", {}).get("reachable_ranges", [])
        span = ",".join(f"{start}-{end}" for start, end in ranges) or "not analyzed"
        for root in unit["roots"]:
            haystack = " ".join(
                [root["name"], unit["id"], *root.get("aliases", []), *root.get("tags", [])]
            ).lower()
            if needle and needle not in haystack:
                continue
            yield (
                root["name"],
                root["ovr_offset"],
                unit["id"],
                root["entry_offset"],
                root["stub"]["logical_target"] if root.get("stub") else "direct call",
                "-".join(root.get("entry_span", [root["entry_offset"], root["entry_offset"]])),
                span,
            )


def command_list(args: argparse.Namespace) -> None:
    catalog = parse_catalog(args.catalog)
    rows = list(list_rows(catalog, args.filter))
    if args.format == "json":
        print(json.dumps(rows, indent=2))
        return
    if args.format == "markdown":
        print("| Name | OVR offset | Unit | Entry | Evidence | Entry span | Reachable unit ranges |")
        print("|---|---:|---|---:|---|---|---|")
        for row in rows:
            print("| " + " | ".join(row) + " |")
        return
    print("name\tovr_offset\tunit\tentry\tevidence\tentry_span\treachable_unit_ranges")
    for row in rows:
        print("\t".join(row))


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


def command_disasm(args: argparse.Namespace) -> None:
    _ep, _op, _exe, ovr, _mz, units = load_release(args)
    unit = select_unit(units, args.unit)
    code = materialized_code(ovr, unit, args.load_base)
    with tempfile.NamedTemporaryFile(prefix="bre-disasm-", suffix=".bin") as stream:
        stream.write(code)
        stream.flush()
        command = [shutil.which("ndisasm") or "ndisasm", "-b", "16", "-a"]
        for stub in unit.stubs:
            command += ["-s", str(stub.entry_offset)]
        command.append(stream.name)
        result = subprocess.run(command, check=True, text=True, capture_output=True)
    labels = {stub.entry_offset: root_name(unit, stub.entry_offset) for stub in unit.stubs}
    for line in result.stdout.splitlines():
        match = NDISASM_LINE.match(line)
        if match and int(match.group(1), 16) in labels:
            print(f"\n{labels[int(match.group(1), 16)]}:")
        print(line)


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


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    fetch = subparsers.add_parser("fetch", help="explicitly fetch the official BRE 0.988 binaries")
    fetch.add_argument("destination")
    fetch.add_argument(
        "--archive",
        help="verify and extract an already-downloaded official archive instead of using the network",
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

    listing = subparsers.add_parser("list", help="list roots from a generated catalog")
    listing.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    listing.add_argument("--filter")
    listing.add_argument("--format", choices=("tsv", "markdown", "json"), default="tsv")
    listing.set_defaults(func=command_list)

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

    disasm = subparsers.add_parser("disasm", help="disassemble a unit at proven roots")
    add_binary_arguments(disasm)
    disasm.add_argument("--unit", required=True)
    disasm.add_argument("--load-base", type=parse_int, default=0)
    disasm.set_defaults(func=command_disasm)

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
