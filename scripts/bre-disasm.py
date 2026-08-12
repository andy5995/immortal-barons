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


VERSION = "0.2"
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

NON_RETURNING_FAR_TARGETS = {(0x0FD0, 0x0116)}


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
            if near_target < len(code):
                queue.append(near_target)
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
            if is_jump:
                block_targets.add(fallthrough)
                target_sources.setdefault(fallthrough, set()).add(
                    (offset, "conditional_fallthrough")
                )

    ranges = ranges_from_instructions(instructions, visited)
    procedure_roots = set(roots) | {target for target in call_targets if target in visited}
    block_starts = {target for target in block_targets if target in visited}
    block_spans = {}
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
    holes = []
    cursor = 0
    for start, end in ranges:
        if cursor < start:
            holes.append([cursor, start])
        cursor = max(cursor, end)
    if cursor < len(code):
        holes.append([cursor, len(code)])
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
        "_procedure_roots": sorted(procedure_roots),
        "_blocks": {
            start: {
                "end": block_spans[start],
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
            if 0 <= target_linear < len(image):
                block_targets.add(target_linear)
                target_sources.setdefault(target_linear, set()).add((linear, kind))
                logical_addresses.setdefault(target_linear, set()).add(
                    (target_segment, target_offset)
                )
                if is_call:
                    procedure_roots.add(target_linear)
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
                block_targets.add(target_linear)
                target_sources.setdefault(target_linear, set()).add((linear, kind))
                logical_addresses.setdefault(target_linear, set()).add(far_target)
                if is_call:
                    procedure_roots.add(target_linear)
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
    block_starts = {target for target in block_targets if target in visited}
    block_spans = {}
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
        aliases = sorted(
            {
                item[0]
                for item in metadata
                if item[0] != name
            }
        )
        root_names[linear] = name
        roots.append(
            {
                "name": name,
                "aliases": aliases,
                "logical_address": f"{segment:04x}:{offset:04x}",
                "logical_aliases": [f"{seg:04x}:{off:04x}" for seg, off in addresses],
                "load_offset": hx(linear, 5),
                "exe_offset": hx(mz.header_size + linear, 6),
                "entry_span": [hx(linear, 5), hx(block_spans[linear], 5)],
                "tags": sorted(set(["resident", "procedure", *tags])),
                "confidence": "proven",
                "evidence": evidence,
            }
        )

    blocks = []
    for start in sorted(block_starts):
        addresses = sorted(logical_addresses.get(start, set()))
        segment, offset = addresses[0]
        name = root_names.get(start, f"exe_{segment:04x}_loc_{offset:04x}")
        sources = [
            {"at": hx(source, 5) if source is not None else None, "kind": kind}
            for source, kind in sorted(
                target_sources.get(start, set()),
                key=lambda item: (-1 if item[0] is None else item[0], item[1]),
            )
        ]
        blocks.append(
            {
                "name": name,
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
            else:
                name = f"exe_data_{start:05x}"
                classification = classify_unreached(image[start:end])
            data_chunks.append(
                {
                    "name": name,
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

    grouped_edges = {}
    for source, kind, target, logical_target in edges:
        grouped_edges.setdefault((kind, target, logical_target), []).append(source)
    return {
        "load_span": [hx(0, 5), hx(len(image), 5)],
        "exe_span": [hx(mz.header_size, 6), hx(len(image) + mz.header_size, 6)],
        "roots": roots,
        "blocks": blocks,
        "data_chunks": data_chunks,
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
        },
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
            block_details = flow.pop("_blocks")
            exported = {stub.entry_offset for stub in unit.stubs}
            for entry in flow.pop("_procedure_roots"):
                if entry in exported:
                    matching = next(root for root in roots if int(root["entry_offset"], 0) == entry)
                    matching["entry_span"] = [hx(entry), hx(block_details[entry]["end"])]
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
                        "entry_span": [hx(entry), hx(block_details[entry]["end"])],
                        "ovr_offset": hx(absolute, 6),
                        "stub": None,
                        "aliases": [],
                        "tags": sorted(set(["overlay", "near-call-target", *semantic_tags])),
                        "confidence": "proven",
                        "evidence": "direct near call from exported-root-reachable code",
                    }
                )
            roots.sort(key=lambda root: int(root["entry_offset"], 0))
            roots_by_entry = {int(root["entry_offset"], 0): root for root in roots}
            blocks = []
            for start, details in block_details.items():
                end = details["end"]
                root = roots_by_entry.get(start)
                absolute = unit.ovr_offset + start
                landmark = LANDMARKS.get(absolute)
                if root:
                    name = root["name"]
                elif landmark:
                    name = landmark[0]
                else:
                    name = f"{unit.unit_id}_loc_{start:04x}"
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
                        "name": name,
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
            item["fixup_chunk"] = {
                "name": f"{unit.unit_id}_fixups",
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
        for address, (name, tags) in RESIDENT_NAMES.items():
            resident_seeds[address] = (
                name,
                tags,
                "repository-cited resident helper",
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
    else:
        resident = [
            {
                "name": name,
                "logical_address": f"{segment:04x}:{offset:04x}",
                "tags": tags,
                "confidence": "repository-cited",
            }
            for (segment, offset), (name, tags) in sorted(RESIDENT_NAMES.items())
        ]
    catalog = {
        "format": "immortal-barons-bre-disassembly-map",
        "format_version": 2,
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
            "ovr_payload_start": hx(8),
            "ovr_payload_end": hx(len(ovr), 6),
        },
        "resident_roots": resident,
        "landmarks": landmarks,
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
    if catalog.get("format_version") != 2:
        raise BREError("catalog format version is not 2")
    names = {}

    def record_name(name: str, location: str) -> None:
        previous = names.setdefault(name, location)
        if previous != location:
            raise BREError(f"name {name!r} maps to both {previous} and {location}")

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
            start = int(root["entry_offset"], 0)
            if start not in blocks_by_start or blocks_by_start[start]["name"] != root["name"]:
                raise BREError(f"{unit_id}: procedure root {root['name']} has no matching block")
            record_name(root["name"], root["ovr_offset"])
            for alias in root.get("aliases", []):
                record_name(alias, root["ovr_offset"])
            exported_roots += root.get("stub") is not None
        for block in unit["blocks"]:
            record_name(block["name"], block["ovr_span"][0])
            if not block["target_kinds"]:
                raise BREError(f"{unit_id}: block {block['name']} has no target evidence")
        for chunk in unit["data_chunks"]:
            record_name(chunk["name"], chunk["ovr_span"][0])
            for alias in chunk.get("aliases", []):
                record_name(alias, chunk["ovr_span"][0])
        record_name(unit["fixup_chunk"]["name"], unit["fixup_chunk"]["ovr_span"][0])
        if unit["control_flow"]["decode_conflicts"]:
            raise BREError(f"{unit_id}: decode-boundary conflicts remain")
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
        start = int(root["load_offset"], 0)
        if start not in resident_blocks_by_start or resident_blocks_by_start[start]["name"] != root["name"]:
            raise BREError(f"resident root {root['name']} has no matching block")
        record_name(root["name"], f"exe:{root['load_offset']}")
        for alias in root.get("aliases", []):
            record_name(alias, f"exe:{root['load_offset']}")
    for block in resident["blocks"]:
        record_name(block["name"], f"exe:{block['load_span'][0]}")
        if not block["target_kinds"]:
            raise BREError(f"resident block {block['name']} has no target evidence")
    for chunk in resident["data_chunks"]:
        record_name(chunk["name"], f"exe:{chunk['load_span'][0]}")
    if resident["control_flow"]["decode_conflicts"]:
        raise BREError("resident decode-boundary conflicts remain")

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
    }
    for key, value in expected_counts.items():
        if summary.get(key) != value:
            raise BREError(f"summary {key}={summary.get(key)}, actual={value}")
    return {
        "unique_names": len(names),
        "overlay_blocks": overlay_blocks,
        "overlay_data_chunks": overlay_data,
        "resident_blocks": len(resident["blocks"]),
        "resident_data_chunks": len(resident["data_chunks"]),
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
    include = {kind} if kind != "all" else {"block", "data", "fixup"}
    for unit in catalog["units"]:
        if "procedure" in include:
            for root in unit["roots"]:
                yield {
                    "kind": "procedure",
                    "name": root["name"],
                    "address": root["ovr_offset"],
                    "container": unit["id"],
                    "span": root.get("entry_span", [root["entry_offset"], root["entry_offset"]]),
                    "aliases": root.get("aliases", []),
                    "tags": root.get("tags", []),
                    "evidence": root["evidence"],
                }
        if "block" in include:
            for block in unit.get("blocks", []):
                yield {
                    "kind": "block",
                    "name": block["name"],
                    "address": block["ovr_span"][0],
                    "container": unit["id"],
                    "span": block["unit_span"],
                    "aliases": [],
                    "tags": block["tags"],
                    "evidence": ",".join(block["target_kinds"]),
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
                }
    resident = catalog.get("resident_image")
    if not resident:
        return
    if "procedure" in include:
        for root in resident["roots"]:
            yield {
                "kind": "procedure",
                "name": root["name"],
                "address": root["logical_address"],
                "container": "resident_exe",
                "span": root["entry_span"],
                "aliases": root.get("aliases", []),
                "tags": root["tags"],
                "evidence": root["evidence"],
            }
    if "block" in include:
        for block in resident["blocks"]:
            yield {
                "kind": "block",
                "name": block["name"],
                "address": block["logical_addresses"][0],
                "container": "resident_exe",
                "span": block["load_span"],
                "aliases": [],
                "tags": block["tags"],
                "evidence": ",".join(block["target_kinds"]),
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
            }


def list_rows(catalog: dict, pattern: str | None, kind: str):
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
        if not needle or needle in haystack:
            yield record


def command_list(args: argparse.Namespace) -> None:
    catalog = parse_catalog(args.catalog)
    rows = list(list_rows(catalog, args.filter, args.kind))
    if args.format == "json":
        print(json.dumps(rows, indent=2))
        return
    if args.format == "markdown":
        print("| Kind | Name | Address | Container | Span | Evidence |")
        print("|---|---|---:|---|---|---|")
        for row in rows:
            print(
                "| "
                + " | ".join(
                    [
                        row["kind"],
                        row["name"],
                        row["address"],
                        row["container"],
                        "-".join(row["span"]),
                        row["evidence"],
                    ]
                )
                + " |"
            )
        return
    print("kind\tname\taddress\tcontainer\tspan\tevidence")
    for row in rows:
        print(
            "\t".join(
                [
                    row["kind"],
                    row["name"],
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
        for kind in ("procedure", "block", "data", "fixup")
        for record in catalog_records(catalog, kind)
        if needle == record["name"].lower()
        or needle in {alias.lower() for alias in record["aliases"]}
    ]
    unique = {
        (record["name"], record["address"], record["kind"]): record
        for record in matches
    }
    if not unique:
        raise BREError(f"no catalog name or alias matches {args.name!r}")
    print(json.dumps(list(unique.values()), indent=2))


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
    catalog = parse_catalog(args.catalog)
    catalog_unit = next(
        (item for item in catalog["units"] if item["id"] == unit.unit_id),
        None,
    )
    if catalog_unit is None:
        raise BREError(f"catalog has no record for {unit.unit_id}")
    if not catalog_unit.get("blocks") or "data_chunks" not in catalog_unit:
        raise BREError("catalog does not contain exhaustive block and data boundaries")
    code = materialized_code(ovr, unit, args.load_base)
    with tempfile.NamedTemporaryFile(prefix="bre-disasm-", suffix=".bin") as stream:
        stream.write(code)
        stream.flush()
        command = [shutil.which("ndisasm") or "ndisasm", "-b", "16", "-a"]
        labels = {}
        for block in catalog_unit["blocks"]:
            start = int(block["unit_span"][0], 0)
            labels[start] = block["name"]
            command += ["-s", str(start)]
        data_labels = {}
        for chunk in catalog_unit["data_chunks"]:
            start, end = (int(value, 0) for value in chunk["unit_span"])
            data_labels[start] = chunk["name"]
            command += ["-k", f"{start},{end - start}"]
        command.append(stream.name)
        result = subprocess.run(command, check=True, text=True, capture_output=True)
    announced_data = set()
    for line in result.stdout.splitlines():
        match = NDISASM_LINE.match(line)
        if match:
            offset = int(match.group(1), 16)
            if offset in labels:
                print(f"\n{labels[offset]}:")
            if offset in data_labels:
                print(f"\n{data_labels[offset]}:")
                announced_data.add(offset)
        print(line)
    for offset, name in sorted(data_labels.items()):
        if offset not in announced_data:
            print(f"\n{name}: ; cataloged non-code span at {hx(offset)}")


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
        choices=("procedure", "block", "data", "fixup", "all"),
        default="procedure",
    )
    listing.add_argument("--format", choices=("tsv", "markdown", "json"), default="tsv")
    listing.set_defaults(func=command_list)

    lookup = subparsers.add_parser("lookup", help="look up an exact stable name or alias")
    lookup.add_argument("name")
    lookup.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
    lookup.set_defaults(func=command_lookup)

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
        "disasm", help="disassemble a unit at all named block boundaries"
    )
    add_binary_arguments(disasm)
    disasm.add_argument("--unit", required=True)
    disasm.add_argument("--load-base", type=parse_int, default=0)
    disasm.add_argument("--catalog", default="docs/dev/bre-v0988-disassembly.json")
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
