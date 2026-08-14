---
name: bre-gather
description: >-
  Use BEFORE reconstructing or verifying ANY Barren Realms Elite (BRE) detail
  for the Immortal Barons clone — menu items/order/hotkeys, mechanic constants,
  prices, combat math, news/bulletin text, colors, or screen layout. Check
  BRE's own files FIRST, do not reconstruct from memory. Triggers whenever the
  task is "match BRE", "check BRE", "how does BRE do X", BRE fidelity of a menu
  or mechanic, or anything touching the BRE binary/data/help files.
---

# Gathering ground truth from Barren Realms Elite

**Companion skill: `play-bre` (user-scoped, `~/.claude/skills/play-bre/`).** This
skill covers *reading* ground truth from a local BRE binary under dosemu, where a
wrecked game costs nothing. When the session is instead a **live game on a real
BBS over syncterm** — Andy attached to a shared tmux session, real opponents,
every keypress permanent — use `play-bre` instead. It carries the syncterm/tmux
harness, the take-over protocol, the traps specific to a live board (the Enter
cascade, Alt combos being swallowed in curses mode, no scrollback), and the
in-game strategy math. The turn flow is the one part the two share.

Immortal Barons is a faithful clone. When a task is "make this match BRE" —
a menu's items/order, a mechanic's numbers, a screen's layout or colors — the
authoritative answer is in BRE's own files, not memory. Check the source, cite
which source, state the confidence. Guessing drifts the clone away from the
original and costs a round-trip when it's caught in review.

**This skill self-updates — that is part of using it, not an optional extra.**
Every gathering run that teaches you something about *how to gather* ends with an
edit to this file, in the same pass, without being asked. The full rule is in
"Keep this skill current" at the bottom; it is repeated here because it was
buried at the end and got skipped twice — once for the HQ price hunt, once for
the reference-list lesson, both of which had to be prompted for. If you finish a
run and this file is unchanged, that is a decision you should be able to justify,
not a default.

## Getting the original BRE

You need your own copy of the **original BRE distribution** (the DOS door
release archive) — this skill never ships or bundles it (see the license
section). If you don't already have it, use the repository's explicit fetcher;
it downloads the official 0.988 archive, verifies pinned hashes, and extracts
`BRE.EXE` and `BRE.OVR`. It never executes the archive or the DOS programs:

```
python3 scripts/bre-disasm.py fetch ~/path/to/private/bre-dos
```

Add `--include-docs` when the bundled help, instructions, samples, artwork, and
template data are useful. The host extractor unpacks the nested `BREDATA.EXE`
payload into `~/path/to/private/bre-dos/reference`; it does not run that DOS
self-extractor.

An existing copy can be checked with `python3 scripts/bre-disasm.py verify
--directory ~/path/to/private/bre-dos`. Do not commit the downloaded files.
Point a shell variable at that directory; every command in this skill uses it:

```
BRE=~/path/to/bre-dos     # wherever you unpacked it, e.g. a DOSEMU drive_c/games/bre-dos
```

Key files inside:

- **`BRE.OVR`** — the overlay; holds the bulk of menu strings, prompts, news
  text. First place to look for labels and menu order.
- **`BRE.EXE`** — the main executable and its resident Turbo Pascal runtime;
  holds some strings and configuration code.
- **`game/*.hlp`, `game/breins.txt`, `game/bre.txt`, `docs/`** — help/prose.
- `data/planet.bre`, `data/game.dat` — runtime save data (not menu/mechanic
  definitions).

The distribution's **`BREDATA.EXE` is not a BRE runtime program**. It is an ARJ
self-extracting installer payload (`BREDATA.ARJ`) containing 19 documentation,
sample-configuration, help, ANSI-art, and initial/template data files. The
stock `UNPACK.BAT` merely runs `BREDATA -Y` to extract them. Do not disassemble
it as game code or look there for overlays, mechanics, or an FP library. This
is why ordinary fetches extract only `BRE.EXE` and `BRE.OVR`; use the explicit
`--include-docs` option when its non-runtime reference material is needed.

## A raw constant carries BRE's unit, not IB's — and the string beside it says which

BRE counts **population in millions**; IB counts people, twenty to BRE's one
(`PopBREUnitScale`). So a per-head price or weight lifted straight out of the
binary is per MILLION and has to be converted before IB applies it. Percentages
are unit-free and need no conversion — only absolute rates do. This has been got
wrong three times, most recently on the chemical missile's price.

**The cheapest way to settle a field's unit is the string printed beside it.**
The chemical and biological strikes subtract a share of record `+0x62` and then
print it with `" million civilians were killed!"`, which names the unit outright
in the same routine. Before scaling anything, find where the field reaches the
screen and read the label — one `find-string` — rather than inferring the unit
from the magnitude.

## Technology silently scales almost every number you capture

**Before deriving ANY constant from a capture, establish the realm's technology
factors.** Technology multiplies most of what a session shows — military
strength up to 1.4x, unit production 1.35x, gold income and population tax 1.5x,
food production 2.0x, maintenance down to 1/1.4, food decay down to 1/5. A
combat or economy figure read off a capture from a teched realm is inflated by
an unknown amount, and nothing on the status screen says so.

**Zero Technology regions does NOT mean the factors are 1.0.** The research level
**never decays and freezes permanently** when the regions are sold, so a realm
that once held Technology carries the boost for the rest of the game with no
Technology regions on screen. Region counts cannot tell you.

**Ask for the Technology advisor screen with every capture.** It is the one
place BRE states the factors outright, as percentages (BRE.OVR 0x32ac2 —
"Because of technology... military forces are functioning at N% strength", plus
lines for production, food, industry, expenses and decay). With that screen a
capture is correctable: divide the observed figure by its factor. Without it,
data from an unknown realm can only give ratios, never absolute constants.

The factor itself, for cross-checking:

```
factor = 1 + (cap - 1) x (1 - exp( -level / (totalRegions + 1) ))
```

so a LARGE realm dilutes its own technology — two realms at the same research
level do not share a factor. See docs/mechanics-reference.md, "Technology".

**When gathering fresh data, prefer a realm that has never bought Technology**,
and say in the notes which realm a figure came from and what its factors were.
Historical note: most of this project's early testing ran with no Technology
regions, so those constants are probably clean — but "probably" is doing work
there, and any figure that disagrees with a later capture should have technology
ruled out first.

### Local InterBBS: the documented ring, and the case-collision that breaks it

`docs/bre.doc` has a **"Local InterBBS Setup"** section for exactly the
several-games-on-one-machine case. Read it before improvising a transport:

- Each game's **inbound files directory** (bbs.cfg **line 4**, "Front End
  Incoming FILE Directory") points at the **previous game's OUTBOUND** — a ring.
  For two boards: A reads B's OUTBOUND, B reads A's OUTBOUND.
- BRE always **writes** to `<its own dir>\OUTBOUND\`; line 4 only says where to
  READ. Line 5 is the **netmail** dir, where the Binkley `.MSG` wrappers land —
  seeing outgoing `.msg` files there is line 5 working, NOT the dirs being
  swapped.
- `ROUTE.CFG`: game 1 `ROUTE * 2`, game 2 `ROUTE * 1` (last routes back to #1).
- **No file transfer step exists or is needed.** Run `BRE PLANETARY` on each
  board in order; the docs say running the cycle **twice** gives immediate
  results. `BRE INBOUND` runs only the inbound half, which is the fast way to
  test ingestion.

**The landmine that cost a whole session (2026-08-11):** the boards had BOTH an
`OUTBOUND` and an empty lowercase `outbound` directory. dosemu's case-insensitive
lookup resolved the DOS path `C:\GAMES\BRE-B\OUTBOUND` to the **empty
lowercase one**, so BRE scanned a directory that never held a packet and
reported nothing at all — no error, no "Unknown Node", just a silent no-op,
while `DIR` from the DOS prompt happily showed the file in the *other* directory.

**Never diagnose a "BRE ignores my file" problem by guessing at filename masks.
Watch what it actually opens:**

```
inotifywait -m -r -e open,access,create,delete <bre-dirs> > /tmp/io.log &
# ...run BRE INBOUND...
grep -i outbound /tmp/io.log
```

That named the wrong directory in one run, after several dead-end hours spent
base-solving the overlay to find the search mask. Reach for inotify the moment
BRE appears to ignore a file. Success looks like:

```
    Processing Incoming Data from Node 2
     Compressed: 334       Decompressed: 2488      %: 86.6%
```

and on the sending side:

```
     Outbound mail for Test Planet Two - Node 2 created.
```

**Also gating IP play:** most InterPlanetary Ops items silently do nothing until
the caller has played a turn this entry — the menu redraws with no message. The
explanation only appears on some items: *"You must play at least one turn per
entry in the game to access this option."* An inert IP menu item is far more
likely to be this than a broken mechanic.

### Disassembly: use the static map; never guess segment bases

`BRE.OVR` is a Turbo Pascal **overlay** file, so a unit's string constants are
addressed relative to a code-segment base that is not in the file. The habit of
*solving* for that base — scoring candidate offsets by how many `mov di,imm16`
land on plausible ShortString length bytes — works sometimes and fails silently
otherwise. On 2026-08-11 it found the attack unit (0x2b783, 36 hits) but dead-
ended completely on the IBBS inbound scanner and on the result-header strings,
burning hours for nothing.

The repository now does that mapping directly. `scripts/bre-disasm.py` parses
the overlay descriptors and relocation streams, follows typed 8086 control
flow, and ties each patched `INT 3Fh` stub back to its canonical `BRE.OVR`
offset. Start with its `list`, `lookup`, `map`, and `disasm` commands; they do
not need the unit's transient runtime segment and do not rely on a plausibility
score. The detailed workflow and proof are in "Reading the disassembly" below
and `docs/dev/bre-disassembly.md`. This map covers `BRE.EXE`, `BRE.OVR`, and the
resident runtime linked into that executable. It does not need to cover
`BREDATA.EXE`, which is only the installation self-extractor described above.

The pinned v0.988 catalog reaches a fixed point for calculated control flow:
all 23 reachable indirect-call sites belong to 13 proven closed target sets,
with no reachable indirect jumps, unresolved transfers, or decode-boundary
conflicts. When a call edge has `kind: calculated_call`, follow its
`dispatch_id`; the dispatch record is the complete target set. Do not guess a
target or launch an emulator merely because the source instruction is indirect.

Use the repository's **Xvfb-backed DOSBox debugger** only when the overlay
loader/materialized bytes need independent validation or new evidence falls
outside the pinned catalog. `python3 scripts/bre-disasm.py debugger --directory
"$BRE" --run` launches the debugger correctly under a private Xvfb. Memory
dumps and traces contain original program material; keep them private and out
of the repository.

This does **not** replace dosemu2 for everything — dosemu2 is still the only way
to scrape screens as text (see above), and that is most of what this skill does.
The two are complementary: **dosemu2 for behaviour and screens, the static map
for normal structure and constants, and a DOSBox debugger for dynamic
validation.** Static catalog lookup is the normal path; the debugger is an
exceptional validation tool, not a prerequisite for following overlays.

## Source priority (most authoritative first)

1. **A rendered screenshot from a live BRE session.** The ONLY authoritative
   source for **colors** and for the exact **hotkey characters + on-screen
   order**. Anyone with BRE running can grab one — select the menu/screen and
   paste it. Prefer this whenever colors or exact keys matter; if you can't run
   BRE, ask a maintainer or contributor who can. For **text content and layout**
   (not colors), you can also drive BRE yourself headlessly — see "Running BRE
   headless" below.
2. **A disassembly of the original binary** — authoritative for exact numeric
   constants. Disassembled values override a reconstruction or a guess when
   they conflict.
3. **`BRE.OVR` / `BRE.EXE` strings** — authoritative for menu **labels** and
   **declaration order** (which equals menu order). See the extraction cookbook.
4. **`game/*.hlp`, `docs/`, `breins.txt`** — prose, help text, tutorial wording.

**Grep the shipped help and docs FIRST, before driving the emulator or
disassembling.** They are last in *authority* but first in *cost*, and BRE
documents far more of its own mechanics than "prose" suggests — with numbers.
The individual-attack variants (2026-08-11) are the case that earned this note:
after a long stretch of failed emulator driving and a base-solving hunt through
`BRE.OVR`, one `grep -i 'quick strike' -r .` turned up `game/attack.hlp` stating
all nine figures outright (110%/50%/8%, 100%/100%/15%, 85%/125%/20%), and
`docs/bre.doc` + `breins.txt` supplied the individual-vs-group returns ratio.
The disassembly was still worth having — it gave the menu's item order, the
Normal-Attack-on-Enter default, and the wire codes the help does not mention —
but it should have been the *second* step, confirming and extending a cheap
answer rather than substituting for one.

**But a shipped doc is a HYPOTHESIS, not an answer — confirm every figure you
intend to implement.** The prose is cheap and usually right, which is what makes
the exceptions dangerous: nothing in the text marks the wrong line. Four cases
found on 2026-08-14 alone:

- `attack.hlp` puts the quick strike at **110%**; the resolver loads **1.2**
  (BRE.OVR 0x4055a). The same switch's retreat constants — 0.92/0.85/0.80, i.e.
  8/15/20% losses — match the help exactly, so the help is right about three
  numbers in that paragraph and wrong about the fourth.
- `bre.doc` says a Declaration Of War breaks a pact "without causing internal
  troubles" and is "not officially broken until the other realm is notified".
  `break_diplomatic_treaty` (0x01a838) takes a quarter of BOTH support and
  morale and clears both relation rows on the spot.
- `bre.doc` says Protective Trade makes deals cheaper "to send and maintain".
  The send discount is real (cost/3); there is no recurring cost in the binary
  at all, so the second half describes nothing.
- `whatsnew.doc` claims tanks defend against chemical missiles. No WMD routine
  reads tanks, turrets or SDI. Note `whatsnew.doc` is a CHANGELOG — it may
  faithfully describe a build that is not the one you have, which is a different
  failure from the manual being wrong about its own release.

So: sweep the prose first to learn **what to look for and roughly where**, then
read the code for the number you will actually type into `balance.go`. When the
two disagree, the code wins and the disagreement goes in the constant's comment
— otherwise the next reader "fixes" the constant back to the doc's value.

Three cheap sweeps to run at the start of any mechanic hunt:

```
grep -rain '<mechanic name>' "$BRE/game/" "$BRE/docs/" | head -40
python3 scripts/bre-disasm.py find-string --directory "$BRE" '<mechanic name>'
strings -a -t x "$BRE/BRE.OVR" | grep -i '<mechanic name>'
```

Prefer `find-string` for code discovery: it searches the private binary text
but returns the cataloged functions and blocks that reference each match. Use
raw `strings` only to inspect declaration order or text that has no indexed
code reference.

**`find-string` with an EMPTY query and a `--function` filter dumps every string
one routine touches** — which reconstructs a screen's whole structure in one
command, without disassembling anything:

```
python3 scripts/bre-disasm.py find-string --directory "$BRE" \
  --function resolve_returning_attack --details "" | jq -r '.matches[].text'
```

That is how the interplanetary returning-attack report was recovered (header,
four verdict words, per-unit lost/returned lines, enemy-destroyed line) in a
single call. Do this BEFORE reading any code: the string list tells you what the
routine's branches must be, so the disassembly is then confirming a shape rather
than discovering one. `--details` output is proprietary — never commit it.

**BRE's `game/*.dat` template files enumerate a feature's full category set for
free.** `ipnews.dat` and `ipreport.dat` are plain-text news/report templates
split by `^CATEGORY` headers, and the header names alone answer "how many
distinct cases does this mechanic have" — the interplanetary attack turned out
to have six news classes (individual / group-on-one-realm / group-on-whole-planet,
each way, on both the arrival and the return side) plus a total-conquest one.
`grep -a '^\^' "$BRE/game/"*.dat` lists every category in the game in one go.
Cheaper than any other source and no disassembly can give it to you as fast.

## Running BRE headless (tmux + dosemu2 harness)

BRE can be driven scriptably and its screens scraped as plain text
(`tmux capture-pane -p`) or WITH ANSI color (`-ep | cat -v`, or wrapping dosemu in
`script`). **The full harness — prerequisites, the launch recipe, and the dozen
landmines that each cost a session — is in `references/harness.md`, together with
the turn-by-turn flow for driving a game. Read it BEFORE driving the emulator;
skimming it afterwards is how the landmines get rediscovered.**

Three things belong here rather than in the reference, because they decide
whether you should open it at all:

- **dosemu2, not DOSBox.** The whole approach depends on dosemu2 rendering DOS
  text-mode video to a real terminal, so screens scrape as characters. DOSBox is
  graphical and would force screenshot + OCR. (DOSBox-X's text-scrapability is
  UNVERIFIED — say so rather than asserting it fails.)
- **Never run two drivers against one tmux session**, and never `tmux
  kill-session` while BRE is running — quit through the menu so `inuse.flg`
  clears and the turn is flushed.
- **Playing creates real state** in the sysop's actual game data. Never drive a
  game Andy cares about; say what you enrolled so he can reset.

**Record captured screens in `docs/dev/bre-screens.md`, not in this skill** — that
doc is the durable catalog of BRE's exact output, wording, layout and ANSI color.

## What the strings give you — and what they DON'T

BRE is Turbo Pascal. Menu items are **length-prefixed ShortStrings stored
consecutively in declaration order**, and that order is the rendered menu
order.

- **Labels + order → YES**, straight from the string table.
- **Colors + hotkey characters → NO.** They are set by the draw code (immediate
  operands in the code segment), not stored next to the string. Get them from a
  screenshot (source 1) or the disassembly (source 2).
- **Menu HIERARCHY / reachability → NO.** The string table gives one menu's
  item labels in declaration order — it does NOT tell you how menus nest, which
  item opens which sub-menu, or *when a menu is shown*. Do not infer "feature X
  lives only under menu Y" from string proximity or from finding a single entry
  point. For the structure, `breins.txt`'s **table of contents lists the menus**
  (a top-level menu there is a top-level menu, e.g. Covert Operations is its own
  menu, separate from Interplanetary Operations). For the clone, **reachability
  is runtime code** — a turn-flow sequence, a preference gate (e.g.
  `VisitCovert`), an IBBS gate — so read the actual code path, not just a
  `gotoMenu` grep. (Real miss: twice concluded covert was InterPlanetary-only
  from the menu tree, when `runTurn` presents it every turn behind a default-on
  preference.)
- **Rates, probabilities, thresholds, and formulas → NO.** The strings and help
  text give you the *trigger* and the qualitative behaviour ("Riots have broken
  out due to high tax rates!"), never the numbers behind them (the riot chance %,
  emigration rate, growth formula, damage math). `breins.txt` even says outright
  that "much information has been left out of the documentation." Those live in
  the **disassembly** (source 2). So when the task needs an exact rate/formula:
  extract the *trigger and direction* from strings/help, then get the number from
  the disassembly — and if no disassembly value is available, **say the rate is
  unverified and reconstruct it as a tunable constant** (as IB does for
  morale/support), rather than presenting a guess as fact. State the confidence.

**The length-prefix trap:** the byte immediately *before* each string is its
**length**, not a color or a hotkey. Example: `0d "View IPScores"` — `0x0d`=13 =
`len("View IPScores")`. A run of 16-bit little-endian values before a menu
cluster is usually a Pascal case/jump table, not data you want.

**Never claim BRE LACKS a feature from the screen you happened to look at.**
Saying "the original has no X" is a claim about the WHOLE binary, and it needs a
search of the whole binary — one `strings | grep -i` for the feature's verb costs
one command. Real miss (2026-08-06, caught by Andy): I built interplanetary
messages, saw no reply path in the IP Messages *sending* menu, and wrote into a
commit message and the spec that "BRE gives no way to answer one". One
`strings BRE.OVR | grep -i repl` found `Reply, / Delete, / Ignore, / Quit`
**twice** — because BRE has TWO message readers, and the second is the
interplanetary one. It even has a prompt the local reader does not:

- **local reader** — `DATA\MSGS.DAT` (BRE.OVR ~0x1DC0E): `Message From: ` /
  `Message To  : `, R/D/I/Q, then `Public Reply, ` / `Author only, or ` /
  `Select Destinations? `.
- **interplanetary reader** — `DATA\MSG.BRF` (~0x1F94C): `Message From: ` +
  ` on ` + `Unknown on ` (realm ON planet), `Message To  : ` + `Coordinator`,
  R/D/I/Q, then a two-way **`Public Reply?`** — the answer goes to the whole
  planet or to the author alone — plus `Quote Message?` with first/last line.

Two lessons, both cheap to apply. **BRE duplicates whole subsystems rather than
parameterising them**, so the local and interplanetary versions of a feature sit
in different overlay units with near-identical strings; finding one tells you
nothing about the other, and the interplanetary twin is usually the one you
skipped. And **absence of evidence in one menu is not evidence of absence** —
before writing "BRE does not do X" into a commit, a doc, or the spec, grep for
X's own words, and say "not found in the sending menu; not searched further" if
that is all you did.

**A prompt's TEXT is not its behaviour — read the caller.** `find-string` on a
prompt lands you in the routine that *prints* it, and that routine usually does
nothing else. The input loop, the key table and the semantics are one level up,
in `callers[]`. `(A-Y,Z=All,?=List) Send to:` had been cloned as a single
keypress for a year on the strength of how it reads; its caller
(`BRE.OVR` 0x1b65e) is a toggling multi-select closed by RETURN, and `Z` marks
every letter rather than sending. Always `lookup` the printer, then `lookup` its
caller, before describing what a prompt does.

**In a capture, count the characters echoed after a prompt.** One echoed key
means a single-key prompt; a run of them (`Send to: EFHIJKMNOP`) means
multi-select, and a run with embedded `\b`/space/`\b` means the keys *toggle*.
This is visible in a plain `cat -v` of the capture and it settles the question
before any disassembly — but only if the capture is read as bytes rather than
skimmed after ANSI stripping, which turns the erase sequences into
innocuous-looking spaces.

**A letter in a picker is usually an ARRAY INDEX, not a row number.** BRE
multiplies the ASCII letter straight into its empire-record stride, so the
letters carry gaps for dead realms and for the caller — never renumber a list to
close them, and check that every screen naming a realm by letter (roster,
relations, `Message To  :`) is using the same basis.

**The variant-string trap (a number in a string may be the WRONG variant's
number).** A literal value baked into a string is only authoritative for *that
call site*, and BRE often has several near-identical strings for variants of one
feature. `strings` flattens them with no context, so grabbing the first match
can hand you a number that belongs to a different mode. Real example: searching
the message editor turned up `You have 3 lines for your message.  /S=save
/A=abort /C=clear` — but a live screenshot showed the standalone message editor
allows **20** lines. The "3" was the *short attach-a-note-to-a-trade-deal*
editor; the "20" was the *Send Message* editor — two variants with almost
identical banners. Lesson: when a string carries a feature's number, don't
assume it's the mode you care about. Grep for sibling copies of the same banner
(`grep -a "lines for your message"` finds both), and confirm the number against
a live screenshot of the *specific* feature before treating it as fact — the
same source-1-beats-strings rule that already applies to colors.

See `references/extraction.md` for the catalog-first string-reference workflow
and the raw `strings` / `grep -abo` / `dd|xxd` fallback commands.

## Cross-reference the docs, the overlay, AND a disassembly — never one alone

A mechanic's full intended scope is usually described in ONE place in the help
docs (`game/breins.txt`, `game/*.hlp`), while the overlay strings and the
disassembly show it piecemeal — a display routine here, a constant there. So for
any mechanic: read the docs' entry FIRST for the complete list of effects, then
confirm the specifics in `BRE.OVR`/`BRE.EXE`, and (when a number is needed) the
disassembly — and reconcile all three before concluding. Reconstructing from only
the overlay + code, skipping the docs, is how a "finding" gets revised two or
three times.

Real miss (the Technology-region mechanic): `breins.txt`'s Technology entry lists
*every* effect (military efficiency, region output, maintenance on regions +
military + SDI, food spoilage, tax income). Skipping it and rebuilding the list
from the overlay "Because of technology…" report strings + the code produced two
wrong answers before the docs settled it.

**Plain grep silently misses these files — use `grep -a`, `strings`, or `ack`.**
`breins.txt` / `*.hlp` contain non-text bytes (ISO-8859/CP437 high bytes plus
color control codes like 0x04/0x07 in the `^\0BTechnology^\07 … ^END` entry
wrappers). GNU grep classifies such a file as *binary* and, by default, reports
NO match for text that is plainly there — no line, no "Binary file matches"
message, just exit 1. This is grep's binary classification of the file, **not**
the locale: reproduced here in BOTH a UTF-8 and a C locale, and `LC_ALL=C` does
not fix it. What works: `grep -a` (`--binary-files=text`), `strings <file> |
grep`, or **`ack`** (which treats these as text by default). **Never trust a
*silent* empty grep on a BRE file — confirm with `strings`/`ack` before
concluding the text isn't there.** (Reproduced: `grep 'longterm enhancements'
breins.txt` → exit 1; `grep -a` and `ack -i tech` → match.)

**One word heads several unrelated entries.** `Technology` is both the region-type
mechanic AND the "Technology Agreement" diplomacy treaty. Read the whole entry
(to its `^END`) and don't conflate two entries that share a keyword.

## Reading the disassembly — reach for it early

Several mechanics that resisted inference from play were read straight out of the
binary in minutes: the coastal support curve, industrial gold, unit production,
the crown tax, the technology system, the terrorist-op price. **Prefer reading the
code to fitting a curve** — a fit needs dozens of samples and can still be wrong,
and two BRE constants were mis-set that way before the disassembly corrected them.

**The method, the command cookbook, and the traps are in
`references/disassembly.md`.** Load it when a constant, formula or gate is
actually wanted.

What must be in front of you BEFORE you open it:

- **Read `docs/dev/bre-save-format.md`'s entry for the mechanic first**, every
  time, even when the task arrives as fresh evidence to analyse. A whole session
  went into recovering the Queen Royale refund formula that was already written
  down there. Grep the file for the mechanic's noun; it costs one command.
- **Grep the shipped help and docs before either** (`game/*.hlp`, `docs/bre.doc`,
  `breins.txt`) — they are last in authority but first in cost, and BRE documents
  far more of its own mechanics, with numbers, than "prose" suggests. Use
  `grep -a`; plain grep silently reports nothing on these files.
- **Never guess a segment base.** Use the static catalog (`scripts/bre-disasm.py`
  `list` / `lookup` / `find-string` / `map` / `disasm`), which needs no transient
  runtime segment. Base-solving by plausibility score dead-ends silently.
- **A candidate reading is not a finding until it reproduces captured figures.**

## Parsing a `.cap` capture — three traps that produced wrong findings

The economy parser is `scripts/bre-econ.py` in this project's Claude dir (per-turn
income, region counts, purchase markers, back-computed yields). Prefer it to
ad-hoc greps. Its `--shapes` mode censuses every distinct message form in a file
— **run that first**, so no relevant line escapes notice.

- **Count with `grep -oc`, never `grep -c`.** These captures are `\r`-separated,
  so a whole screen can be one "line": `grep -c` reported 1 fishing turn in a
  capture that contained 6.
- **Never pipe a survey through `head`.** A truncated survey once "proved" that
  rivers never produce food, when the line was simply below the cut.
- **The Regions display WRAPS onto a second line.** Parsing only the first loses
  Mountains, Coastal and Technology — that mistake hid 18 tourism samples.

**And before explaining any per-unit figure, check whether the count you divided
by changed that turn.** Purchases land between the report and the Regions
display, so a total can print next to a STALE count. Twice in one session a
changed denominator was mistaken for a changed mechanic.

### First ask whether the capture EXERCISED the feature at all

A capture that mentions a feature thousands of times may never have used it
once. BRE presents the Attack Menu and the InterPlanetary Ops menu **every
turn, automatically**, so their item labels accumulate once per turn whether or
not the player pressed anything. A 27 MB capture matched "Group Attack" 5,385
times and looked like a goldmine; it contained no interplanetary attack at all.

**The tell is equal counts across sibling items.** Count several items from the
same menu at once — when `Regular Attack`, `Nuclear Attack`, `Attack Pirates`
and `Alliance Strength` all land on 2,498, that is the menu being redrawn 2,498
times, not four features being used. A number that *breaks* from the cluster is
the one worth chasing (`Spy Database` at 13,155 against a 2,534 menu count is
real use; the 317 non-menu `Group Attack` hits were `BRE PLANETARY` step lines).

**Prove absence with the binary's own result strings, not with guessed wording.**
Harvest the Pascal ShortStrings from `BRE.OVR`, filter to the mechanic's
vocabulary, and test each against the capture — that answers "is this flow here"
without depending on how you remember the prompt being worded:

```
python3 - "$BRE/BRE.OVR" plain.txt <<'EOF'
import re,sys
ovr=open(sys.argv[1],'rb').read(); cap=open(sys.argv[2],'rb').read()
c=[m.group(2)[:m.group(1)[0]] for m in re.finditer(rb'([\x08-\x3c])([ -~]{8,60})',ovr)
   if len(m.group(2)[:m.group(1)[0]])==m.group(1)[0]]
kw=(b'attack',b'strike',b'battle')          # the mechanic's vocabulary
for s in sorted({x for x in c if any(k in x.lower() for k in kw)} , key=lambda x:-cap.count(x)):
    if cap.count(s): print(f"{cap.count(s):>6}  {s.decode('latin-1')}")
EOF
```

Run this **before** planning a mining session. It takes one command and it is
the difference between an afternoon of analysis and knowing in a minute that the
data is not there.

**`grep` calls these captures binary and silently reports nothing.** CP437 high
bytes in a UTF-8 locale make GNU grep exit 1 with no output and no "Binary file
matches" — the same trap the `breins.txt` section describes, in a new place. Use
`LC_ALL=C` **and** `grep -a`; a bare grep returning zero on a capture is not
evidence of absence until both are set.

### Auto-Pay turns are a stronger probe than the itemised prompts

With **Auto-Pay Maintenance ON**, BRE collapses the whole maintenance sequence
into a single `N Gold paid.` line. That is *more* informative than the separate
prompts, not less, because every component has to reconcile against one number at
once:

```
Gold paid = regionUpkeepPerRegion × regions      (constant for a given realm)
          + perUnitMaint × units held
          + trunc(turn income × PlanetaryTaxRate / 1000)
```

Guess one unknown, solve for another, then check whether the answer stays
constant across turns where the *first* quantity moved. **A constant that
survives a changing denominator is the signal; a "constant" that drifts with
income is a wrong assumption.** This settled whether industrial gold is inside
the crown-tax base in ten turns of already-captured data: assuming it is taxed
leaves region maintenance at exactly 913.000/region across three different region
counts, assuming it is not leaves a figure wandering 974–992.

It also yields per-unit maintenance for free — on turns holding manufactured
units the per-region figure sits slightly above the constant, and the gap is
`units × perUnitMaint`.

So when a capture is needed to settle an arithmetic question, **ask for Auto-Pay
ON**, not off. Auto-Pay OFF is for learning the prompt *sequence*, wording, and
colours (see the maintenance-flow section of `docs/mechanics-reference.md`).

## License boundaries — BRE is proprietary, not open source

Barren Realms Elite is copyrighted (John Dailey Software; original design by
Mehul Patel). Immortal Barons is a clean-room reimplementation of its *rules*.
Gathering from the binary is fine — but what crosses back into the repo is
strictly limited. The idea/expression line:

**Fair to replicate (facts / functional — not protected by copyright):**
- Game mechanics, rules, and formulas.
- Numeric balance constants — unit stats, prices, caps, rates. These are facts;
  verify against a disassembly and record them in `docs/mechanics-reference.md`.
- Menu structure, item order, and hotkey layout (functional organization).

**NEVER copy into the repo (copyrightable expression):**
- BRE's source code, or code reconstructed/**decompiled** from the binary.
  Read a disassembly to learn a *constant or formula*, then
  write our own implementation — transcribing its code is a derivative work and
  is infringing.
- **Display text verbatim**: menu descriptions, prompts, news/bulletin lines,
  help text, tutorial prose, end-of-turn messages. Reconstruct in our own words.
- **ANSI art, logos, splash/end screens** (`game/bre.ans`, `breend.ans`, etc.).
- **Distinctive flavor names.** Reconstruct BRE's distinctive labels in IB's own
  words rather than copy them verbatim; IB renamed the "Gooie Kablooie" weapon to
  "Clingy Annihilator".

**Trademark / branding:** do not present the project as "Barren Realms Elite" or
use its name/logo as our branding. IB is an independent tribute, "not affiliated
with, or endorsed by" the original authors (README "Heritage").

**Never ship BRE's files:** do not distribute, commit, or bundle `BRE.EXE` /
`BRE.OVR`, its data files, or any asset extracted from them. They stay in your
own local reference copy only.

**What BRE's own license says (scanned 2026-07, `docs/bre.doc` — the license
lives only there; `register.doc`/`whatsnew.doc`/`*.hlp`/`breins.txt` add no
terms).** The BRE "SOFTWARE LICENSE AGREEMENT" has **no anti-disassembly or
anti-reverse-engineering clause** — reading/disassembling the binary for study
is not addressed. What it *does* forbid: "You may not **alter** the
machine-readable object files or program documentation files" (esp. to defeat
the registration key or modify the copyright/text), removing/modifying the
copyright notice, and selling or bundling-for-fee. So disassembling BRE to learn
a constant/formula for OUR own implementation is not prohibited by its license;
altering `BRE.EXE`/`BRE.OVR` is. (Editing a local *save* file — `game.dat` — for
study is a different thing from altering the object files, but revert it and
never redistribute.) Not legal advice; absence of a clause ≠ affirmative
permission, and jurisdictions differ — the clean-room "no copying expression"
posture above is the safe line regardless.

**BRE is shareware** (60-day evaluation + registration key), and under US law
disassembling software to reach its *unprotected functional elements* (ideas,
mechanics, constants) is settled **fair use** — *Sega v. Accolade*, 977 F.2d
1510 (9th Cir. 1992) and *Sony v. Connectix*, 203 F.3d 596 (9th Cir. 2000): the
"intermediate copying" that disassembly requires is fair use when it's the only
way to get at the functional ideas and the result is an independent
implementation (exactly the clean-room clone here). The one live caveat is DMCA
§1201 anti-circumvention — but that bites only if you defeat the registration
"key system," which we never do (we read game math, not the key). Interpol is
irrelevant (a police-coordination body, not a lawmaker); cross-border norms come
from treaties (Berne/TRIPS) and the EU Software Directive 2009/24/EC, which
*expressly* permits decompilation for interoperability. Verified current 2026-07;
no ruling has disturbed Sega/Sony. Sources: EFF Coders' Rights Reverse
Engineering FAQ (`eff.org/issues/coders/reverse-engineering-faq`), *Sega v.
Accolade* (Wikipedia / BitLaw), *Sony v. Connectix* (digital-law-online.info).

Not legal advice. The idea/expression line above reflects general copyright
principles, not a ruling on any specific case. When unsure whether something has
crossed from *mechanic* into *expression*, ask a project maintainer or check the
copyright law that applies in your jurisdiction rather than assume — and when in
doubt, don't copy.

## Other guardrails

- **No third-party private contact info** (John Dailey's or anyone's) in any
  repo artifact — commits, comments, docs, anywhere.
- **CP437 vs UTF-8.** BRE strings are CP437; high bytes mojibake in a UTF-8
  terminal. Our clone emits UTF-8 — never paste raw CP437 bytes; map to the
  Unicode glyph.

## After gathering

State the source and confidence in the reply, e.g. "from BRE.OVR string table
(labels/order authoritative; colors unknown — need a screenshot)". Update
`docs/mechanics-reference.md` when a verified value changes, and record durable
findings in the relevant memory (`bre-binary-verified-math`,
`check-bre-strings-when-unsure`).

**Keep this skill current.** When you discover something new about *how to gather
from BRE* — a driving/scraping technique, a harness landmine and its fix, a
menu-input quirk, a build-up/strategy method for staging a scenario, a source
that turned out authoritative (or not) — fold it back into this skill so the next
run starts from it instead of re-learning. (Verified mechanic *values* still go
to `docs/mechanics-reference.md`; play/build-up technique goes to
`docs/dev/bre-buildup-strategy.md`; this skill captures the *gathering method*.)
Update the skill in the same pass that produced the discovery, not "later."

**Put it in the right file.** This SKILL.md is loaded on EVERY session that
touches BRE, so it earns its size only by holding what changes whether you start
correctly: the source ladder, the licence line, the "check the notes and the
shipped docs first" rules, and the calibration lessons. Detail that matters only
once you are already doing the work belongs in `references/`:

| File | What lives there |
| --- | --- |
| `references/harness.md` | tmux/dosemu2 driving, the launch recipe, its landmines, the turn-by-turn game flow |
| `references/disassembly.md` | the disassembly method, command cookbook, field/constant-hunting recipes |
| `references/extraction.md` | string extraction and the catalog-first reference workflow |
| `references/interbbs.md` | two-board league runs |

It was split for exactly this reason: the core had grown to 1,452 lines
(~21,500 tokens) of which two sections were over half. Add to a reference and
leave a one-line hook here; do not re-grow the core.
