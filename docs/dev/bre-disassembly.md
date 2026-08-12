# BRE 0.988 static disassembly map

This repository contains a machine-readable map of the original BRE 0.988
executable and overlay, but no original program bytes. The map makes ordinary
analysis static and repeatable: an overlay call in `BRE.EXE` can be followed to
an exact byte in `BRE.OVR`, and reachable code can be decoded from known
instruction boundaries without first running BRE.

The companion tool is `scripts/bre-disasm.py`; the generated catalog is
`docs/dev/bre-v0988-disassembly.json`. Both are deliberately limited to the
main runtime (`BRE.EXE`, `BRE.OVR`, and the resident Turbo Pascal runtime).
`BREDATA.EXE` is outside this map.

## Quick start

Use a BRE 0.988 distribution you obtained lawfully. Do not add its binaries,
memory dumps, debugger logs, strings, or disassembly output to this repository.

```sh
# Verify an existing copy.
python3 scripts/bre-disasm.py verify --directory /path/to/bre

# Find named code without having the binaries present.
python3 scripts/bre-disasm.py list --filter pirate

# Resolve a linked far-call target or an OVR file offset.
python3 scripts/bre-disasm.py map --directory /path/to/bre --address 084d:0020
python3 scripts/bre-disasm.py map --directory /path/to/bre --ovr-offset 0x4ba48

# Disassemble one unit, synchronized at every proven exported root.
python3 scripts/bre-disasm.py disasm --directory /path/to/bre --unit ovr_04b9d0
```

Downloading is always explicit. `fetch` retrieves the official 0.988 archive,
checks its pinned SHA-256, extracts only `BRE.EXE` and `BRE.OVR`, and verifies
both files. It never runs the archive or either DOS program.

```sh
python3 scripts/bre-disasm.py fetch /path/to/private/bre-0.988
```

The supported files are exact. A different release is rejected instead of
being silently interpreted with 0.988 addresses:

| Artifact | Size | SHA-256 |
|---|---:|---|
| official `brev988.exe` archive | 337980 | `40c9d78066add460176a326ebe9f01f3b39df4f70a7bc060ec8ecf09d875b3d5` |
| `BRE.EXE` | 91712 | `ae1ce21a01b6b21840e603090e674286fd6848462298f479498d6c17ef31dde6` |
| `BRE.OVR` | 364835 | `c9d6a40261634f6b29c0b3bbf7e8fe8582106fb39ee3f49059c775434164d2c0` |

## The address model

`BRE.EXE` is an MZ executable with a `0x2940`-byte header. Addresses quoted as
logical `segment:offset` are relative to the beginning of its load module, not
to the PSP and not to a particular DOS run. Convert one to an EXE file offset
with:

```text
file_offset = 0x2940 + logical_segment * 16 + offset
```

`BRE.OVR` starts with `FBOV` and a four-byte payload length. Its first overlay
unit begins at file offset `0x000008`. The EXE contains 103 overlay descriptors
and 414 exported stubs. Their OVR records form an exact, gap-free chain through
the end of the file.

Each descriptor has this on-disk prefix:

| Offset | Size | Meaning |
|---:|---:|---|
| `+0x00` | 4 | `CD 3F 00 00` descriptor marker |
| `+0x04` | 4 | OVR code file offset |
| `+0x08` | 2 | code size in bytes |
| `+0x0a` | 2 | fixup-stream size in bytes |
| `+0x0c` | 2 | exported stub count |
| `+0x0e` | 2 | previous descriptor's logical segment |
| `+0x10` | 2 | runtime load segment (zero on disk) |
| `+0x14` | 2 | runtime loaded-list link |
| `+0x20` | 5 each | exported stubs |

An unloaded stub is exactly:

```text
CD 3F <entry:u16le> 00
```

The interrupt handler reads the entry word, loads the unit, and rewrites every
stub in that descriptor as:

```text
EA <entry:u16le> <runtime-unit-segment:u16le>
```

It then returns to the rewritten instruction. Consequently the static mapping
is exact and does not depend on the runtime unit segment:

```text
canonical OVR root = descriptor.ovr_code_offset + stub.entry
```

For example, logical stub `084d:0020` contains entry `0679` in the descriptor
whose unit starts at OVR offset `0x582d7`. It maps to `BRE.OVR 0x58950` and has
the stable name `ovr_0582d7_entry_0679`.

## Fixups and materialized units

Each unit's code is immediately followed by its fixup stream. The stream is a
sorted array of little-endian `uint16` offsets into that unit's code. For every
entry, the loader performs:

```text
word[unit + fixup_offset] += DOS_EXE_load_segment
```

These words are linked logical segments for resident routines and other overlay
descriptors. They are not adjusted by the dynamic segment where the overlay
unit itself happens to be stored. Therefore the untouched OVR is the preferred
canonical image for cross-references; use load base zero for normal static
analysis.

`materialize` reproduces the bytes from a particular DOS run when a runtime
load base is useful:

```sh
python3 scripts/bre-disasm.py materialize \
  --directory /path/to/bre --unit ovr_0582d7 --load-base 0x1a2 \
  --output /tmp/unit.bin
```

The model was checked against a debugger dump of unit `ovr_0582d7`: its 98
fixup words all differed by `0x01a2`, the resulting image matched byte for byte,
and no byte outside a fixup word changed. The reusable check is:

```sh
python3 scripts/bre-disasm.py compare-dump \
  --directory /path/to/bre --unit ovr_0582d7 --dump /tmp/MEMDUMP.BIN
```

## Roots, spans, and names

The catalog gives every exported stub root a deterministic fallback name:

```text
ovr_<six-digit-unit-file-offset>_entry_<four-digit-entry>
```

Direct near-call targets reachable from those exports are also proven
procedure roots and receive:

```text
ovr_<six-digit-unit-file-offset>_proc_<four-digit-entry>
```

Known routines get a semantic primary name such as `send_spy`; their address,
tags, confidence, and evidence remain in the same record. Interior addresses
already cited by the project's mechanics work are recorded separately as
landmarks. A landmark is not falsely promoted to a procedure merely because it
is useful.

Reachability uses Capstone in 16-bit x86 mode. Decoding begins independently at
every exported root and follows only fallthrough and typed direct-control-flow
operands. Newly encountered direct calls become procedure roots. The catalog
records:

- an entry basic-block span for each exported or direct-call procedure root;
- the union of root-reachable instruction ranges for each unit;
- the complementary unreached ranges, which may be data, padding, or code that
  is reachable only through an unresolved indirect transfer;
- grouped far-call/overlay-call edges and every unresolved indirect transfer;
- any target that conflicts with an already decoded instruction boundary.

Ranges use half-open bounds: `[start, end)`. They are intentionally not broad
"from this prologue to the next prologue" envelopes. The current catalog has
603 proven procedure roots, 353 disjoint reachable ranges, 11 unresolved
indirect transfers, and zero decode-boundary conflicts. Consumers must not
infer code behind an unresolved transfer without another static root or runtime
evidence.

Capstone 5's Python binding and native library are required to regenerate
reachable spans. On systems where the native library is in a nonstandard
location, set `LIBCAPSTONE_PATH`. `ndisasm` is optional and is used only by the
human-readable `disasm` command.

Regenerate the catalog only from the pinned files:

```sh
python3 scripts/bre-disasm.py analyze \
  --directory /path/to/bre --output docs/dev/bre-v0988-disassembly.json
```

## One-time debugger validation

Normal mapping and disassembly do not require DOSBox. The debugger remains
useful for validating a loader claim or materializing an unusual indirect
target. This DOSBox build has its heavy debugger enabled, but the SDL window
still needs a display. `debugger --run` starts DOSBox under a private Xvfb,
sets `TERM=xterm` (required for the function-key escape sequences), sends
Alt-Pause to the X window, and cleans up Xvfb when DOSBox exits:

```sh
python3 scripts/bre-disasm.py debugger --directory /path/to/bre --run
```

At the debugger prompt, press Enter to enter command mode before typing a
command. A minimal overlay trace is:

```text
BPINT 3F
F5
```

At the trap, `F10` enters the handler. `MEMDUMPBIN SEGMENT:0000 SIZE` writes a
unit image. `LOG count` writes an instruction trace. Both outputs contain
original program material and belong only in a private temporary directory.

The observed handler is at runtime `10fd:02e6` when the DOS load base is
`0x01a2`, hence logical `0f5b:02e6`. It loads through EMS (`INT 67h`), patches
the descriptor stubs, and jumps to the requested entry. Calls to Turbo Pascal
integer and Real48 helpers go to resident logical segments in `BRE.EXE` (notably
`0fd0`). No separate floating-point library file is opened or loaded; there is
no FP overlay whose segment must be guessed.

## Scope and legal hygiene

The map records mechanics of the file format, addresses, hashes, control-flow
boundaries, and analysis names. It does not contain original code bytes,
strings, display text, art, full disassembly, memory dumps, or debugger logs.
Keep those out of commits. The static analysis names are project terminology,
not claims about original compiler symbols.
