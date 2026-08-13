<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. -->

## Reading the disassembly — the method that works (2026-07-29, proven)

Several mechanics that resisted inference from play were read straight out of the
binary in minutes: the coastal support curve, industrial gold, unit production,
the crown tax, and the whole technology system. **Reach for this early rather
than fitting a curve** — a fit needs dozens of samples and can still be wrong,
and two BRE constants were mis-set that way before the disassembly corrected
them.

Start with the repository's static map, not a guessed OVR slice:

```
python3 scripts/bre-disasm.py check-catalog
python3 scripts/bre-disasm.py find-string --directory "$BRE" "technology"
python3 scripts/bre-disasm.py find-string --directory "$BRE" --function technology_report "technology"
python3 scripts/bre-disasm.py list --kind procedure --filter technology
python3 scripts/bre-disasm.py lookup technology_report
python3 scripts/bre-disasm.py xrefs technology_report
python3 scripts/bre-disasm.py list --kind dispatch
python3 scripts/bre-disasm.py map --directory "$BRE" --ovr-offset 0x32d1b
python3 scripts/bre-disasm.py disasm --directory "$BRE" --procedure technology_report
python3 scripts/bre-disasm.py disasm --directory "$BRE" --around 0x32d1b --instructions 40
```

`docs/dev/bre-disassembly.md` documents the file format and
`docs/dev/bre-v0988-disassembly.json` is the machine-readable catalog. The map
contains all 103 overlay units and names every proven procedure, direct-jump
basic block, conditional fallthrough, complementary chunk, fixup stream,
resident target, and overlay descriptor record. Half-open spans, source edges,
bidirectional caller/callee graphs, procedure body ranges, code-segment data
references, the Pascal-string reference index, resident RTL/Real48 names, and
closed calculated-transfer sets are attached to those records. Semantic names
have an auditable `identified` status; internal targets use `contextual`,
file-format records use `structural`, and address-only fallbacks are explicitly
`unclassified`.

Treat a record's `id` as its identity and `name` as its current human-friendly
label. Names can improve; IDs are derived from canonical EXE/OVR addresses and
keep call-graph and string-index links durable. `lookup NAME_OR_ID` accepts
either. On a procedure result, follow `callees[].to_id` downward and
`callers[].from_id` upward; `site_ids` identify the exact call instructions. If
an edge is `calculated_call`, look up its `dispatch_id` to get the complete
closed target list and assignment-tracing evidence. `list --kind
procedure|block|data|fixup|dispatch|all` searches record classes, and
`--status identified|contextual|structural|unclassified` filters by naming
state. Store durable IDs in working notes or generated indexes and show the
friendly name alongside them. Inspect a procedure's `data_references` and a
data record's inverse `referenced_by` list when a field or table matters.

`find-string --directory "$BRE" SUBSTRING` loads the committed index first,
verifies the exact binaries, then returns every named procedure and block that
directly references a matching Pascal string. Matching is case-insensitive by
default. Add repeatable `--function NAME_OR_ID` filters after a broad search to
limit uses to exact procedures. Add `--details` only when the original matching
text and exact use sites are necessary; that output is proprietary and must not
be committed.
The catalog deliberately stores hashes, lengths, and durable references rather
than original strings.

Start with identified names, then inspect unclassified roots only when the
desired behavior has no semantic match. `disasm --procedure NAME_OR_ID` prints
exact catalogued body ranges. `disasm --around ADDRESS` accepts an OVR file
offset, resident `SEGMENT:OFFSET`, durable site ID, or procedure, and safely
synchronizes to the containing block's nearest preceding catalog root. The
header names that anchor; an address in data is rejected rather than decoded
from an arbitrary byte. Whole-unit and `--start/--end` views remain available.
The tool skips catalogued non-code spans and verifies exact v0.988 hashes.
Capstone 5 supplies typed 16-bit operands for reachability; `ndisasm` is only
the optional text renderer.

The loop that keeps paying off:

1. **Find the message string's users.** Run `find-string` with a distinctive
   substring and begin at its returned procedure IDs. Search sibling wording
   too: duplicated subsystems and variant messages can lead to different code.
   Fall back to raw string offsets only when the index has no direct reference.
2. **Walk the graph by durable ID.** Use `xrefs NAME_OR_ID` or `lookup` on
   callers and callees until the desired calculation or gate is reached. With
   private binaries, add `--directory "$BRE" --show-sites --direction callers`
   (or `callees`/`both`) for synchronized windows at exact site IDs. Follow
   every calculated edge's `dispatch_id`; its targets are closed for this
   release. Prefer a call site near an indexed string when several callers need
   semantic identification.
3. **Map and disassemble at proven boundaries.** Prefer `disasm --procedure`
   for a named body or `disasm --around SITE_ID` for local context. The latter
   synchronizes from the nearest preceding root and refuses non-code addresses;
   never decode a guessed linear slice from an arbitrary byte.

   **Two things silently stop this working.** `disasm` needs the **capstone**
   Python module, which is not in the repo's dependencies — without it every
   `disasm` invocation exits 0 and prints NOTHING, which reads as "the tool found
   no code" rather than "the tool cannot run". Check with `python3 -c "import
   capstone"` before concluding anything about a procedure's contents, and put it
   in a venv (`~/.venvs/immortal-barons-disasm`) rather than on the host.

   And a **procedure record's `span` is its entry block, not its body** — the
   nuclear strike's span is 38 bytes while its callee sites run to +0x290 — so
   `--procedure` shows a fragment and `--around` can fail outright with "no
   instruction follows". When that happens, take the unit and offset from
   `lookup`, and read the bytes yourself at the canonical file offset:

   ```
   dd if="$BRE/BRE.OVR" bs=1 skip=$((0xe809+0x225e)) count=$((0x320)) of=/tmp/x.bin status=none
   ndisasm -b 16 -o 0x225e /tmp/x.bin
   ```

   `-o` makes ndisasm's addresses unit-relative, so they line up with the
   catalog's spans and site IDs and the call graph stays usable. Resident
   (`BRE.EXE`) code is the same trick with the MZ header added: file offset =
   `0x2940 + segment*16 + offset`. This is not "guessing a boundary" — the
   catalog supplied the start — and it was the only way through on the nuclear
   and decontamination routines.
4. **Find a constant** by searching `struct.pack("<H", value)` and checking the
   preceding byte for an imm16 opcode (`b8` mov ax, `05` add ax, `b9` mov cx …).
5. **Calculate Real48 exactly.** Reals are six-byte Turbo Pascal values loaded
   into a register triple before the resident helper call. Pass constants and
   intermediate values to the exact BRE-linked calculator, using an explicit
   `mem:` prefix for memory bytes:

   ```
   python3 scripts/bre-real48.py div 100 3
   python3 scripts/bre-real48.py mul mem:870000000048 0.25 --output all
   python3 scripts/bre-real48.py sqrt mem:820000000000
   python3 scripts/bre-real48.py decode mem:810000000000 mem:800000000000
   python3 scripts/bre-real48.py eval '100000 + (50 / 5)'
   ```

   Use `decode` for one or many stored values and `eval` for a compound
   expression rounded to Real48 after every operation. The calculator accepts
   decimal and `mem:hhhhhhhhhhhh` inputs, implements the complete arithmetic,
   conversion, comparison, standard-function, and random operation surface
   linked into BRE 0.988, and decorates every memory result with `mem:`. Never
   decode a Real48 by hand, use Python `float`/`math`, or borrow constants from
   another Turbo Pascal release. Read
   `docs/dev/bre-real48.md` for representation, errors, and operation names.
6. **Validate against play.** A candidate reading is not a finding until it
   reproduces captured figures exactly. Rounding vs truncation and the order of
   operations both change results by a unit — the two real→int helpers differ
   only in that, and picking the wrong one is easy.

Record layout and helper addresses live in `docs/dev/bre-save-format.md`; extend
that file rather than re-deriving.

**Read that file's entry for the mechanic BEFORE opening a disassembler — every
time, even when the task arrives as fresh evidence to analyse.** A whole session
went into recovering the Queen Royale refund formula from the binary when the
pool offset, both rates, the cap, the gating predicate and the crown-tax feed
were already written down there from an earlier pass. The trigger for the miss is
worth recognising: new data (a pile of captures) makes the question feel new, so
the notes never get checked. Grep the file for the mechanic's noun first; it
costs one command and tells you what is genuinely unanswered.

### Finding an UNKNOWN record field: scan the opcode, not the string

When you don't yet know a mechanic's field offset, don't hunt for its message
string — scan for the *shape of the code that touches it*. Empire-record fields
are reached as `es:`-prefixed ops on `[di+disp16]`, so a regex over the raw file
for `26 83 (85|bd) <disp16> <imm8>` (es: add/cmp word [di+d16], imm8) filtered to
plausible immediates finds every candidate in seconds. HeadQuarters fell out of
this immediately: `cmp word [es:di+0x26b],100` was the only `cmp …,100` at an
unmapped offset, and it sat inside the end-of-turn routine — which gave the
field, the build rate, and the clamp in one disassembly.

Once you have the offset, `re.finditer` for the packed disp16 (`6b 02`) lists
*every* site that reads or writes it, which separates the purchase path, the
per-turn advance, and the combat use without any further searching.

### Counting references is evidence — use it before hypothesising

Two cheap counts settle questions that otherwise invite a guess. Both come from
the Queen Royale tax-refund routine (2026-07-31), where they killed a hypothesis
I had already stated publicly.

- **Count a called function's call sites before assuming it is special-purpose.**
  A refund was gated by a predicate, and "this is the once-per-day check" was an
  attractive reading. Counting the far-call bytes found it invoked **27 times**
  across unrelated contexts — menu key handling among them — with several sites
  passing a byte from a local parameter rather than the global. That is a generic
  per-empire guard, not an event gate. One `re.finditer` overturned the guess.
- **A global read many times and never written is an index, not a tunable.**
  `[0x28dc]` had 104 `mov al,[0x28dc]` reads and zero writes anywhere in the
  overlay — the current empire index, set elsewhere and passed as an argument.
  Grep for the store opcodes (`a2`, `c6 06`, `88 26`) as well as the loads; the
  read:write ratio tells you what kind of thing it is.

### Check where a conditional jump actually lands

A `jz` inside a routine usually skips a *sub-block*, not the routine. In the
refund code the predicate's `jz` targeted a label past the cap computation but
before the crediting and output, so a false predicate meant **"pay uncapped"**,
not **"pay nothing"** — the opposite of what I reported. Resolve every branch
target against the surrounding structure before describing what a test controls.

### Absence of Random() is itself a finding

If a routine contains no `Random()` call (`0c03:0ed0`), it is deterministic: when
invoked it always acts. So an event that sometimes does not appear is not a dice
roll inside the routine — the decision lives in the **caller**. Combining that
with one live observation ("no second refund on re-entry the same day") proved
the routine was never called, without locating the caller at all. Cheap
deductions like this beat hunting for a gate you have not found yet.

### Name a field from ALL its sites, never from the two in front of you

An `inc` and a matching `dec` around a block look exactly like a re-entrancy
guard, and calling one that was the wrong reading behind a claim Andy caught.
Empire `+0x2b8` really is the **turn-stage counter**: the same byte is compared
against 1 and against 20 elsewhere, reset to 0 at turn commit, and dispatched on
by a loop head. Two of its eleven sites happened to be an inc/dec pair.

The rule the reference-list entry above already states, in the case it is easiest
to skip: **a field's meaning comes from the whole access list**, and the list is
one regex away. The tell that you are about to get this wrong is a field in the
PERSISTED record doing a job a local variable would do — BRE has no reason to
spend save-file bytes on a guard, so if it looks like one, you have misread it.
Grep the disp16 in both files, sort by opcode, and read every site.

### Finding an overlaid routine's CALLER — map the overlay stubs

"Who calls this?" looks unanswerable in an overlay, because callers far-call an
**INT 3Fh stub in `BRE.EXE`**, not the OVR file offset. The static catalog now
does the complete mapping and inverse call graph. First `lookup` the procedure
and read `callers[].from_id` plus `site_ids`; recurse by ID to find the owning
behavior. Given a raw far target, run `bre-disasm.py map --address SEG:OFF`;
given an OVR address, run it with `--ovr-offset`. The descriptor's five-byte
`cd 3f <entry> 00` stub maps exactly to `unit_file_offset + entry`.

For manual verification, the MZ header is `0x2940` bytes and a logical EXE
address maps to `0x2940 + segment*16 + offset`. At runtime the INT 3F handler
rewrites each stub to `ea <entry> <dynamic-unit-segment>`; the dynamic segment is
irrelevant to the canonical OVR address. The OVR fixup stream is an array of
word offsets, each relocated by adding the DOS EXE load segment. See the dev
guide rather than re-deriving or guessing this layout.

### Name a predicate from its OTHER call sites, not the one you are reading

A boolean helper reveals nothing where you found it. Look at the site where its
result gates a **message string** — that names it. `056d:19b5` was "some
per-empire predicate" for two sessions; one of its 27 call sites
(`BRE.OVR 0x1771a`) prints "Our empire is in protection, my lord." on a true
result, which settles it in one disassembly. Sort the call-site list by which one
is nearest a string table and read that one first.

### Calibrating "empire N's field" accesses

Fields of an *arbitrary* empire are reached as `mov al,<index>; mov dx,0x42d;
mul dx; les di,[0x28b0]; add di,ax`, then a large NEGATIVE `disp16` — a different
origin from the current-empire `les di,[0x28d8]` map. Convert with
`offset = disp + 3949`. Derive that constant rather than trusting it: grep the
displacement list for a run of **eighteen consecutive `cmp word` sites stepping
by 2** — the nine region counts — and anchor its start to the known `+0x96`.
Collecting every such access at once (regex the whole `ba 2d 04 f7 e2 c4 3e b0
28 03 f8 26` prefix) also gives the record's field census for free.

**Then OPEN every one of those sites before you state a mechanic's scope.** The
list is cheap to produce and cheap to skim, and skipping it is how a confident
wrong claim gets made. Real miss: "HQ affects only tanks" was asserted from the
two combat sites plus the manual, with four unexamined reads still on the list —
Andy caught it. Opening all nine confirmed the claim (the rest were an advisor
nag's zero-test, the status display, and the buy menu's `# Owned` pointer), but
that was luck, not method. A field's reference list IS the mechanic's scope;
until every entry is identified, "only X is affected" is a hypothesis.

Reading them all also finds sites you did not know to look for: the third combat
site (`0xF2E4`) computes the same strength expression over **locals** rather than
record fields — that is the *committed attack force*, and it is what confirmed
IB's three call sites map one-for-one onto BRE's.

### Prices: find the table write, not the buy handler

Item prices are NOT computed where they are spent. The buy routine reads a global
`int32` table at `DS:0x2216` indexed by the menu key (`price = [0x2216 + key*4]`,
so HeadQuarters `'5'` is `0x22EA`), and that table is filled once per turn from
per-empire record fields (`+0x113`, `+0x117`, `+0x11b`, …). So to find how a
price is *derived*, search for the WRITE to its table slot (`a3 <lo> <hi>` =
`mov [addr],ax`), not for the purchase path. The HQ price formula sat 20 bytes
above its slot write and nowhere near the buy handler.

That per-empire storage is itself worth knowing: BRE keeps unit prices per
empire, not planet-wide.

### Compare ALL siblings before theorising about one

When one quantity looks like it is drifting, extract the same series for every
sibling item in the same screen and put them side by side. One item's sequence in
isolation reads as noise; the contrast is the finding. Pulling all eight buy-menu
prices at once made it immediate that seven random-walk around a stable base
(up in one capture, down in another) while HeadQuarters rose in *every* capture —
which is what turned "the HQ price drifts" into "the HQ price is a ratchet, and
it is the only one."

### Census every instance in the captures BEFORE disassembling

An event's *distribution* across the captures tells you which branch of the code
you are about to read, and it costs one grep. The Queen Royale refund had been
filed from a single observation of 1,000,000, which reads as a fixed sum. The
captures held 47 payouts ranging from 354 to 14,000,000 — and fourteen of them
were exactly 1,000,000, which is the shape of a **cap**, not of a formula. Going
in knowing a cap existed made the routine's structure obvious on first read.

Two signatures worth recognising in a value list:

- **The same value many times, with others spread around it** — a clamp.
- **A value one below a round number** (999,999 beside many 1,000,000s) — a cap
  implemented by substituting `limit / x` for a rate and multiplying back by `x`.
  The round trip through a six-byte real loses the last unit and truncation keeps
  it. Do not chase it as a separate case.

Predictions from the recovered formula should then be tested against the whole
list, not the samples that suggested it: a two-rate mechanic with a threshold
implies a **gap** in the achievable payouts, and finding that no observation lands
in the gap is real evidence.

### Kill hypotheses with the captures, then disassemble

Captures are good at *falsifying* a driver cheaply, even when they cannot pin the
formula. For the HQ price, two plausible drivers died in one query each: **realm
size** (at a fixed 791 regions the price still climbed 5,697 → 7,682, and a
1,140-region realm was cheaper than a 791-region one) and **Score** (the ratio
ran 23.8 → 0.59 across the range). That is the signal to stop fitting and go to
the binary — which gave `5000 + 75×turnsPlayed + Random(300)`, capped at
`100000 − Random(1000)`, in minutes.

**Beware the proxy trap.** Score rises a flat 213 per turn, so it correlates with
anything driven by turns played and looks like a cause. BRE's actual driver was a
separate lifetime turn counter (`+0x281`). When two quantities advance in lockstep
every turn, a correlation cannot tell them apart — the binary can. (It mattered
for the clone too: IB's Score is *not* a usable stand-in, because combat adjusts
it, so IB needed its own counter.)

**Then re-validate the recovered formula against every capture at once**, not the
handful you derived it from: 163 distinct captured HQ prices, 161 inside the
300-wide jitter window and spread flat across it. Dedupe first — a repeated
screen inflates the sample and skews any distribution check (the raw count was
1,618 with a badly skewed mean; the 163 distinct values were flat).

### The docs entry is the cheapest way to identify a field

Before inferring what a record field holds from its arithmetic weight, read the
`breins.txt` entry for the mechanic — it usually names the unit outright. The HQ
entry says "gives tanks better coordination", which identified `+0x86` as Tanks
in one grep, after a net-worth-constant hunt failed to. This is the skill's own
"read the docs FIRST" rule; skipping it cost a detour.

**A doc figure may be a mid-curve value, not a base.** `breins.txt` calls a tank
"about the equivalent of four Troopers", but the binary weighs it `1.5 + HQ/100`
against a trooper's `0.5` — so 4 is what a tank is worth at HQ **50%**, and the
real range is 3 to 5. When the prose gives a flat number for something the same
prose says "scales", suspect a mid-curve sample and check the disassembly before
hard-coding it. This is the variant-string trap in a different costume.
