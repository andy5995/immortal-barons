# BRE 0.988 static disassembly map

This repository contains a machine-readable map of the original BRE 0.988
executable and overlay, but no original program bytes. The map makes ordinary
analysis static and repeatable: an overlay call in `BRE.EXE` can be followed to
an exact byte in `BRE.OVR`, and reachable code can be decoded from known
instruction boundaries without first running BRE.

The companion tool is `scripts/bre-disasm.py`; the generated catalog is
`docs/dev/bre-v0988-disassembly.json`. Both are deliberately limited to the
main runtime (`BRE.EXE`, `BRE.OVR`, and the resident Turbo Pascal runtime).
`BREDATA.EXE` is not another game executable: it is an ARJ self-extracting
installation payload containing documentation, help, artwork, samples, and
initial/template data. It has no role in runtime overlay or floating-point
loading and therefore needs no disassembly map.

## Quick start

Use a BRE 0.988 distribution you obtained lawfully. Do not add its binaries,
memory dumps, debugger logs, strings, or disassembly output to this repository.

```sh
# Verify an existing copy.
python3 scripts/bre-disasm.py verify --directory /path/to/bre

# Find procedures, basic blocks, or data chunks without having the binaries.
python3 scripts/bre-disasm.py list --kind procedure --filter pirate
python3 scripts/bre-disasm.py list --kind block --filter pirate
python3 scripts/bre-disasm.py list --kind data --filter message
python3 scripts/bre-disasm.py list --kind dispatch
python3 scripts/bre-disasm.py lookup crown_tax

# Find all functions and blocks referring to a Pascal string containing text.
python3 scripts/bre-disasm.py find-string --directory /path/to/bre "bank"
python3 scripts/bre-disasm.py find-string --directory /path/to/bre \
  --function create_trade_offer "Trade Deal"

# Resolve a linked far-call target or an OVR file offset.
python3 scripts/bre-disasm.py map --directory /path/to/bre --address 084d:0020
python3 scripts/bre-disasm.py map --directory /path/to/bre --ovr-offset 0x4ba48

# Disassemble a named procedure, or a whole/ranged overlay unit.
python3 scripts/bre-disasm.py disasm --directory /path/to/bre \
  --procedure create_trade_offer
python3 scripts/bre-disasm.py disasm --directory /path/to/bre --unit ovr_04b9d0
python3 scripts/bre-disasm.py disasm --directory /path/to/bre \
  --unit ovr_04b9d0 --start 0x1d50 --end 0x21a0

# Show synchronized context around an OVR file offset, resident address, or site ID.
python3 scripts/bre-disasm.py disasm --directory /path/to/bre \
  --around 0x0268a1 --instructions 40
python3 scripts/bre-disasm.py disasm --directory /path/to/bre \
  --around 056d:01bf --instructions 24

# Inspect a procedure's graph without binaries, then show its exact call sites.
python3 scripts/bre-disasm.py xrefs create_trade_offer
python3 scripts/bre-disasm.py xrefs create_trade_offer --directory /path/to/bre \
  --show-sites --direction callers

# Verify that the committed names and spans exhaustively partition both files.
python3 scripts/bre-disasm.py check-catalog
```

Downloading is always explicit. `fetch` retrieves the official 0.988 archive,
checks its pinned SHA-256, extracts `BRE.EXE` and `BRE.OVR`, and verifies both
files. It never runs the archive or any DOS program. Add `--include-docs` to
unpack the nested `BREDATA.EXE` archive into `DESTINATION/reference`; this uses
the host extractor and does not execute the self-extracting DOS payload.

```sh
python3 scripts/bre-disasm.py fetch /path/to/private/bre-0.988
python3 scripts/bre-disasm.py fetch /path/to/private/bre-0.988 --include-docs
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

## Procedures, blocks, chunks, and names

The catalog gives every exported stub root a deterministic fallback name:

```text
ovr_<six-digit-unit-file-offset>_entry_<four-digit-entry>
```

Direct near-call targets reachable from those exports are also proven
procedure roots and receive:

```text
ovr_<six-digit-unit-file-offset>_proc_<four-digit-entry>
```

Known routines and internal jump targets get semantic primary names such as
`run_bank`, `calculate_crown_tax`,
`allocate_turn_budget__armed_forces_maintenance`, or
`text_read_shortstring`; their old address name remains an alias. Curated names
live in `scripts/bre-semantic-names.json`, separate from the reachability
engine. Each record carries one of four honest naming states:

- `identified`: behavior is supported by call-graph, owned-data, instruction,
  public RTL, or existing repository evidence;
- `contextual`: an internal loop, branch, join, or return is named within one
  identified procedure without claiming more behavior than the CFG proves;
- `structural`: an overlay fixup stream or descriptor boundary is proven by
  the file format;
- `unclassified`: only the stable address-derived fallback is known, and the
  catalog says so explicitly.

Interior addresses already cited by the project's mechanics work are recorded
separately as landmarks. A landmark is not falsely promoted to a procedure
merely because it is useful. The semantic manifest records paraphrased topics,
not strings copied from the proprietary binary.

Reachability uses Capstone in 16-bit x86 mode. Decoding begins independently at
every exported root and follows only fallthrough and typed direct-control-flow
operands. Newly encountered direct calls become procedure roots. Every direct
jump destination and every conditional fallthrough becomes a named basic-block
boundary. Stable fallback names are:

```text
ovr_<unit-file-offset>_loc_<unit-offset>
exe_<logical-segment>_loc_<logical-offset>
```

The resident pass uses segmented 8086 addresses rather than pretending that
the MZ load module is one flat code segment. It begins at the MZ entry point,
the traced overlay loader, the known runtime helpers, and every direct resident
far target found in reachable overlay code. Direct resident near and far calls
and jumps then extend that graph. Overlay descriptor records are excluded from
resident decoding and get their own names.

Every byte not reached from those proven roots belongs to a named, contiguous
complementary chunk:

```text
ovr_<unit-file-offset>_data_<unit-offset>
exe_data_<load-offset>
ovr_<unit-file-offset>_fixups
ovr_<unit-file-offset>_descriptor_record
```

Pure zero, NOP, and breakpoint-fill chunks are classified as such. Other
chunks are conservatively classified as `unreached_data_or_indirect_code`:
they are safe boundaries for a rooted disassembly, but may contain code that
can only be reached through one of the explicitly recorded indirect calls.
Existing semantic landmarks split a chunk and supply its primary name.

The catalog therefore records:

- a stable name, source edge, and half-open span for every procedure and basic
  block target;
- the union of root-reachable instruction ranges for each unit;
- a named partition of every complementary unreached range and every overlay
  fixup stream;
- grouped direct and calculated call edges, including the complete closed target
  set and assignment evidence for every reachable indirect transfer;
- caller and callee lists, exact body ranges, and code-segment data references
  for every procedure, plus the inverse `referenced_by` relation on data;
- a bidirectional procedure call graph whose caller, callee, and instruction
  sites use durable address IDs rather than friendly names as their identity;
- an index of directly referenced Pascal strings, keyed by durable address IDs
  and retaining only lengths, hashes, and durable code references;
- any target that conflicts with an already decoded instruction boundary.

Ranges use half-open bounds: `[start, end)`. They are intentionally not broad
"from this prologue to the next prologue" envelopes. The current catalog has
603 overlay procedure roots, 8,495 overlay basic blocks, 319 overlay data/code
chunks, 389 resident procedure roots, 2,921 resident basic blocks, 231 resident
data/code chunks, and 103 named fixup streams containing 16,672 fixups. Its
12,668 stable location names are unique. It indexes 2,350 directly referenced
Pascal strings and 2,571 block-use records without retaining their text. The
naming pass identifies 400 of the
992 proven procedures and ties 282 of 550 complementary chunks to identified
behavior. Of the basic blocks, 6,862 targets have procedure-context names and
441 entries have behavior-specific names. The remaining 592 procedures and 165
non-structural chunks are explicitly unclassified rather than being given
speculative names. The procedure graph contains 6,541 grouped outgoing edges at
20,324 call sites. All edges resolve bidirectionally by durable ID. Recursive
target discovery reaches a fixed point with 13 closed calculated-transfer
groups, 23 indirect call sites, 29 group-to-target memberships, zero unresolved
transfers, and zero decode-boundary conflicts.

`check-catalog` independently verifies that named block spans exactly cover all
reachable bytes, named chunks exactly cover all remaining bytes, together they
cover each overlay code area and the complete resident load module, every
procedure has a same-name entry block, names are unique, and the recorded
summary counts agree. It also verifies that every durable ID matches its file
address and that every indexed string use resolves to known block and procedure
IDs. Known call edges must appear identically in the caller and callee directions,
and call-site IDs are recomputed from their containing binary addresses.
`list --kind procedure|block|data|fixup|dispatch|all` emits TSV, Markdown, or JSON,
and `--status identified|contextual|structural|unclassified` selects a naming
state. `lookup NAME_OR_ID` returns the matching records, evidence, call graph,
and data references for a stable name, semantic alias, or durable ID (a
procedure entry is also its first block).

Every procedure record is directly walkable as a graph node. Follow
`callees[].to_id` to descend into things it calls and `callers[].from_id` to
walk back to its callers; pass either ID to `lookup` to load the next node.
Each grouped edge retains its kind and local `sites`, while `site_ids` identify
the exact instructions as canonical EXE load offsets or OVR file offsets.
An indirect edge has kind `calculated_call` and a `dispatch_id` linking it to
the closed-set proof described below.

`xrefs NAME_OR_ID` is the compact graph lookup intended for this traversal. It
needs only the committed catalog unless `--show-sites` is requested. With
private binaries available, `--show-sites --direction callers|callees|both`
renders bounded instruction windows at the recorded site IDs; `--context` and
`--max-sites` control their size and count.

## Closed calculated transfers

`calculated_transfers` records why every reachable indirect call is finite.
Each durable dispatch record contains its exact instruction `site_ids`, source
model, complete procedure target list, and concise assignment-tracing evidence.
The models found in this linked release are far procedure parameters, fixed
global procedure slots, a heap-linked callback list with a sole constructor,
Turbo Pascal `TextRec` method fields, and a near scanner callback passed in AX.
There are no reachable indirect jumps and no open-ended indirect calls.

Target discovery is recursive. The original 22 indirect sites supplied roots
for callback-only code; decoding those roots exposed one further indirect call
inside the text driver. Resolving that site exposed no more, establishing the
23-site fixed point. `check-catalog` requires every calculated target to be a
procedure root, every dispatch to appear in the bidirectional call graph, every
site to belong to exactly one group, and both unresolved-transfer lists to be
empty.

`find-string SUBSTRING` is the bridge from private binary text to the static
map. It loads the committed table first, verifies the exact BRE 0.988 binaries,
materializes the indexed Pascal strings, and performs a case-insensitive search.
The default JSON lists every currently named function and basic block that uses
a match. Address-derived IDs such as `bre0988:ovr:block:02ef0f` are stored in
the table, so improving a friendly name does not break the relation. Add
`--case-sensitive` for an exact-case search or `--details` to include the
matching private text and instruction sites. Repeat `--function NAME_OR_ID` to
restrict uses to one or more exact procedures after resolving their durable
IDs. Detailed output contains original program text and must not be committed.

`disasm` accepts three boundary-safe selectors. `--procedure` prints the exact
catalogued body ranges for an overlay or resident procedure. `--unit` prints an
overlay unit and optionally accepts a half-open, unit-relative `--start/--end`
range. `--around` accepts a canonical OVR file offset, resident logical
`SEGMENT:OFFSET`, durable site ID, or exact procedure selector.

An `--around` address is never assumed to be an instruction boundary. The tool
finds the containing catalogued code block, supplies all catalogued roots to
the decoder, and anchors the window at the nearest containing/preceding root.
It prints both the requested offset and synchronization anchor in the header,
never prints instructions before that anchor, and rejects an address in a
catalogued data span instead of guessing. This same synchronization is used by
`xrefs --show-sites`.

Useful audit queries are:

```sh
python3 scripts/bre-disasm.py list --kind procedure --status identified
python3 scripts/bre-disasm.py list --kind procedure --status unclassified
python3 scripts/bre-disasm.py lookup resolve_received_trade_offer
python3 scripts/bre-disasm.py lookup bre0988:ovr:procedure:0389d6
python3 scripts/bre-disasm.py lookup bre0988:dispatch:text_scanners
python3 scripts/bre-disasm.py find-string --directory /path/to/bre "trade"
python3 scripts/bre-disasm.py find-string --directory /path/to/bre \
  --function resolve_received_trade_offer "trade"
python3 scripts/bre-disasm.py xrefs resolve_received_trade_offer
python3 scripts/bre-disasm.py disasm --directory /path/to/bre \
  --around bre0988:ovr:site:03b7b0
```

Capstone 5's Python binding and native library are required to regenerate
reachable spans. On systems where the native library is in a nonstandard
location, set `LIBCAPSTONE_PATH`. `ndisasm` is optional and is used only by the
human-readable `disasm` command.

Regenerate the catalog only from the pinned files:

```sh
python3 scripts/bre-disasm.py analyze \
  --directory /path/to/bre --output docs/dev/bre-v0988-disassembly.json
python3 scripts/bre-disasm.py check-catalog
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

The exact operation surface, six-byte representation, BRE-linked constants,
Python port, and calculator are documented in [bre-real48.md](bre-real48.md).

Two integer helpers in `0c03` the catalog does not name yet, identified from two
independent call sites each while reading the nuclear strike and the waste
decontamination routines:

| Address     | Behaviour                                                        |
|-------------|------------------------------------------------------------------|
| `0c03:129b` | `max` of two int32 arguments                                     |
| `0c03:12e1` | `min` of two int32 arguments                                     |

The nuclear strike caps its price with `12e1` against 50,000,000, and the
decontamination allowance is `min(max(waste / 5, 10), waste)` — one use of each,
in the only arrangement that makes both routines sensible.

## Empire record fields identified from their whole access list

Both were found by scanning each binary for every `es:`-prefixed `[di+disp16]`
access and every `add di,disp16` at the field's displacement, in the
current-empire form (`disp`) and the arbitrary-empire form (`disp + 0xf093`),
then naming the containing block for each hit. The method is the one the
`bre-gather` skill describes; what makes it conclusive is reading *all* the
sites, not the two nearest the question.

**`+0x286` — Score** (int32). Fourteen sites. Written by `resolve_regular_attack`
(two branches, gated on config record `+0x3d8`), `launch_nuclear_attack`,
`launch_chemical_attack`, `launch_biological_attack`, `launch_pirate_raid`,
`resolve_returning_attack` and `send_spy`; read by `show_empire_status`,
`show_scores`, `build_recon_record`, and `format_player_ranking_line`. That last
one settles it: the rankings line prints `total_regions`, then this field, then
the net-worth function — Territory, Score, Net Worth, in the scores table's own
column order.

**`+0xba` — the unallocated-region pool** (int32). Credited by a won regular
attack, a pirate raid, a returning interplanetary attack, and waste
decontamination; drained by the picker that prompts `[N Regions left]` and
`How many <Type> regions?`, which adds the named count to the chosen type's field
and subtracts it here, looping until the pool is empty.

`total_regions` (`056d:0ec6`) sums exactly the nine region counts at `+0x96`
through `+0xb6+2` and does **not** include this pool, so unallocated land is not
territory until it is placed.

### The trade-offer record and the routines around it

The offers a realm has not answered live in ONE list shared by trade deals and
diplomatic proposals — the head pointer is a resident global, each node carries a
record type (1 = trade deal, 2 = treaty proposal) and a pointer to its payload.
A trade-deal payload is 151 bytes with a CRC over the first 0x93 of them stored
at `+0x93`, which `process_trade_offer` recomputes before trusting the record and
which is what its "data tampered with" line reports.

Fields read while settling this session's questions:

- **`+0x5a` — when the deal lapses** (Real48). `create_trade_offer` adds the
  chosen day count to the clock global at `0x8606` and stores the sum; the
  turn-start sweep compares the two and deletes anything older.
- **`+0x8` / `+0x9` — the two realms**, sender and target as slot bytes: `+0x8`
  is who the outcome is reported to, `+0x9` indexes the target's record for the
  identity check that follows.
- **`+0x4` / `+0x6` — the target's identity**, matched against the record `+0x9`
  points at and then against the player taking the turn, so a slot reused by a
  new realm does not inherit the offer.

Four routines in the trade unit were named from this pass:
`calculate_trade_offer_cost` (the weighted sum of the nine goods that gives the
per-day figure), `empire_trade_good_pointer` (slot + good index -> the count to
adjust, used by both the escrow loop and every accept-branch transfer),
`store_trade_offer_record`, and `pack_trade_offer_packets` (the daily-maintenance
step that exports deals to other boards).

### A catalog name corrected

`allocate_unassigned_regions` (`BRE.OVR 0x030ebb`) was catalogued as
`select_regions_to_lose`, which the field access list contradicts: it returns
immediately when the pool is empty, and its body adds the chosen count to a
region type while subtracting the same from the pool. Its four callers are the
three that credit the pool — a won regular attack, a pirate raid, waste
decontamination — plus `run_player_turn`, which re-offers whatever is left. The
key handling is the region-picker's: a type letter, `?` to redisplay the list,
`*` for the advisors, Enter to leave.

Renamed in the catalog with `select_regions_to_lose` retired to `aliases`, so
the old name still resolves — the convention `calculate_crown_tax` already
follows. Every record derived from the name (its 23 branch/loop/join blocks)
was renamed with it, and the `"to"`/`"from"` call-graph references were rewritten
to match.

**Editing the catalog by hand:** it round-trips exactly through
`json.dumps(catalog, indent=2)` plus a trailing newline, so a structural edit
need not reformat the file — dumping with any other setting produces a
million-line diff. The `validation` block at the end is a set of counters over
the catalog's own contents and goes stale on any edit; recompute it with
`validate_catalog` from `scripts/bre-disasm.py` and confirm with
`bre-disasm.py check-catalog`. Retiring a name into `aliases` raises
`unique_names` by one, because the old name stays resolvable.
