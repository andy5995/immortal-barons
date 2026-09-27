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

Immortal Barons is a faithful clone. When a task is "make this match BRE", the
answer is in BRE's own files, not in memory: check the source, cite which one,
and state the confidence. A guess drifts the clone and costs a round-trip when
review catches it.

This guide covers *reading* ground truth from a local BRE under dosemu, where a
wrecked game costs nothing. For a **live game on a real BBS over syncterm**,
where every keypress is permanent, use the user-scoped `play-bre` skill
(`~/.claude/skills/play-bre/`) instead.

**This guide updates itself.** A run that teaches you something about *how to
gather* ends with an edit here, in the same pass (see "Keep this skill current"
at the end). If a run leaves the guide unchanged, be able to say why.

## Getting the original BRE

You need your own copy of the original BRE distribution; this project never
ships it (see the license section). The repository's fetcher downloads the
official 0.988 archive, verifies pinned hashes, and extracts `BRE.EXE` and
`BRE.OVR` without running anything:

```
python3 scripts/bre-disasm.py fetch ~/path/to/private/bre-dos
python3 scripts/bre-disasm.py verify --directory ~/path/to/private/bre-dos
```

`--include-docs` also unpacks the bundled help, docs, samples and art from
`BREDATA.EXE`, which is an ARJ self-extractor of 19 reference files, not game
code: never disassemble it. Do not commit anything downloaded. Every command in
this guide uses:

```
BRE=~/path/to/bre-dos     # e.g. a DOSEMU drive_c/games/bre-dos
```

- **`BRE.OVR`**: the overlay, holding most menu strings, prompts and news text.
- **`BRE.EXE`**: the main executable, its resident Turbo Pascal runtime, and the
  initialized data segment.
- **`docs/bre.doc`, `game/breins.txt`, `game/attack.hlp`, `game/reset.hlp`,
  `docs/whatsnew.doc`**: the prose. There are only TWO `.hlp` files, so do not
  hunt for per-screen help. **`reset.hlp` documents every config setting**, and
  settings are where BRE states mechanics outright.
- `game/*.dat`: news and report templates. `data/game.dat`, `data/planet.bre`:
  save data.

## Source priority (most authoritative first)

1. **A rendered screen from a live BRE session** — the only authority on
   **colors**, exact **hotkeys** and on-screen order. `cap/` holds many; you can
   also drive BRE headlessly (below), or ask someone who runs it.
2. **A disassembly of the original binary** — authoritative for exact constants
   and formulas.
3. **`BRE.OVR` / `BRE.EXE` strings** — authoritative for labels and declaration
   order.
4. **The shipped prose** — help text, the manual, tutorial wording.

**Grep the shipped prose FIRST anyway.** It is last in authority but first in
cost, and BRE documents far more of its mechanics, with numbers, than you would
expect. After a long failed emulator session, one `grep -i 'quick strike'`
turned up `game/attack.hlp` stating all nine attack-variant figures. Three
sweeps to start any hunt:

```
grep -rain '<mechanic>' "$BRE/game/" "$BRE/docs/" | head -40
python3 scripts/bre-disasm.py find-string --directory "$BRE" '<mechanic>'
strings -a -t x "$BRE/BRE.OVR" | grep -i '<mechanic>'
```

**But a shipped doc is a hypothesis: confirm every figure you implement.**
Nothing marks the wrong line. On one day (2026-08-14):
- `attack.hlp` puts the quick strike at 110 %; the resolver loads **1.2**, while
  the same paragraph's three loss figures are right.
- `bre.doc` says a Declaration of War breaks a pact "without causing internal
  troubles"; `break_diplomatic_treaty` takes a quarter of support and morale.
- `bre.doc` says Protective Trade makes deals cheaper "to send and maintain";
  there is no recurring cost at all.
- `whatsnew.doc` says tanks defend against chemical missiles; no WMD routine
  reads tanks. It is a changelog, and may describe a different build.

When prose and code disagree, the code wins, and the disagreement goes in the
constant's comment, or the next reader "fixes" it back.

**Open the screen that quotes a price before opening a disassembler.** The
Covert Operations menu prints all nine fees; issue #143 stood open on the theory
that they were an unreadable runtime table. Use the disassembly for what the
screen cannot say, such as whether the number is scaled.

**"Who may do this?" is answered by `bre.doc`'s command-line section**, one line
per switch: `PLAYERLIST` is "for League Coordinators only", `UPDATE` "Only can be
done by BBS #1". Grep it before reasoning from which menu a thing appears on.

**"IB's own" and "reconstructed" in our notes are DATED, not decided.** The same
label covers a deliberate divergence and a gap nobody could close at the time
(the Gooie Kablooie's siege was "not read from the binary", then read in twenty
minutes once the catalog existed). Re-read such a figure before building on it,
and when you write a marker, say which kind: "IB chooses this because …", or
"not read yet; <routine> is where it lives".

## Calibrate a capture before trusting its numbers

**Technology scales almost everything a session shows**: military strength up
to 1.4x, production 1.35x, gold and tax 1.5x, food 2.0x, maintenance down to
1/1.4, decay down to 1/5. A figure from a teched realm is inflated by an unknown
amount, and nothing on the status screen says so. **Zero Technology regions does
not mean factor 1.0**: research never decays and freezes when the regions are
sold. **Ask for the Technology advisor screen with every capture** (BRE.OVR
0x32ac2, "…functioning at N% strength"): it states the factors, and divides them
back out. For cross-checking:

```
factor = 1 + (cap - 1) x (1 - exp( -level / (totalRegions + 1) ))
```

so a large realm dilutes its own technology. When gathering fresh data, prefer a
realm that never bought Technology, and record which realm a figure came from.

**Read the same session's Game Setup screen before fitting a formula.** A per-game
setting can disable the mechanic you are fitting: a league running
`Protection Turns: 130` kept realms shielded, BRE waives the region surcharge
under protection, and 65 purchase screens "proved" the surcharge did not exist.
When a fit disagrees with the code, suspect the setting before the code.

**Before explaining a per-unit figure, check whether its denominator changed that
turn.** Purchases land between the report and the Regions display.

## What the strings give you — and what they don't

Menu items are length-prefixed ShortStrings stored consecutively in declaration
order, which is usually the rendered order.

- **Labels and order: yes.**
- **Colors and hotkeys: no.** The draw code sets them; get them from a capture
  or the disassembly.
- **Menu hierarchy and reachability: no.** `breins.txt`'s table of contents lists
  the menus. In the clone, reachability is runtime code (a turn-flow stage, a
  preference such as `VisitCovert`, an IBBS gate), so read the code path. Covert
  ops were twice judged InterPlanetary-only from the menu tree, when `runTurn`
  offers them every turn.
- **Rates, probabilities and formulas: no.** Strings give the trigger and the
  direction; the number is in the disassembly. With no disassembly value, say the
  rate is unverified and make it a tunable constant.

**The byte before a string is its length.** `0d "View IPScores"` is 13 characters.
It also makes a string look LONGER than it is: the Diplomacy Modification key set
was recorded as `WNPAU` for months, but the length byte is `0x04` and the `U` is
the next routine's `push bp`. Check a short quoted string against its length
byte the first time you rely on it.

**Any claim about what a SCREEN shows or omits is checked against `cap/` first.**
It costs one command:

```
LC_ALL=C grep -ao "<a distinctive line>" cap/*.cap | sort | uniq -c
LC_ALL=C tr '\r' '\n' < cap/<file>.cap | grep -a -B4 -A12 "<that line>" | sed 's/\x1b\[[0-9;]*m//g'
```

The treaty-proposal screen was reported as having no message field and IB's was
removed to match; one grep showed a boxed `Message attached:` block, and the next
proposal in the same capture showed it appears only when a message is attached.
Before reporting a screen as uncaptured, say which files and pattern you
searched, and search for a menu item with its value rather than a status line.
Traps in both directions are in `references/captures.md`.

**Never claim BRE LACKS a feature from the screen you looked at.** "The original
has no X" is a claim about the whole binary. "BRE gives no way to answer" an
interplanetary message went into a commit and the spec; one
`strings BRE.OVR | grep -i repl` found `Reply` twice, because **BRE duplicates
whole subsystems**: the local and interplanetary readers are separate routines,
and the one you skipped is usually the interplanetary twin. **`find-string
--function X` lists one routine's strings**, and a screen is drawn by several:
`Message attached:` belongs to `attach_message_to_diplomatic_proposal`, next to
the offer screen's routine. Search the feature's own words globally, and write
"not found in the sending menu; not searched further" if that is all you did.

**A negative strong enough to build a divergence on needs three independent
places to agree.** Requiring a treaty on the Trading Market rested on BRE having
no such rule: the prose describes "a general market" with no buyer rule,
`run_trading_market` references no relation string, and the one relation refusal
belongs to `create_trade_offer`. One of the three alone is a hunch.

**A player's account of the original is evidence.** Players said Sabre dial 4
hits military bases and 5 airbases while the spec said the dial did nothing; they
were right, and named the exact rows of the table. A report cannot settle a
constant, but it can reopen one: re-read the code before defending the note.

## Cross-reference the docs, the overlay and the disassembly

A mechanic's full scope is usually described in ONE place in the prose, while the
strings and the code show it piecemeal. Read the prose entry first for the
complete list of effects, then confirm each in the binary. The Technology
mechanic took two wrong answers before `breins.txt`'s entry settled what it
touches. One word can head two entries (`Technology` is a region type AND a
treaty), so read each entry to its `^END`.

**Plain `grep` reports nothing on these files.** `breins.txt`, the `.hlp` files
and the `.cap` captures contain CP437 and control bytes, so GNU grep classifies
them as binary and exits 1 with no output, in any locale. Use `grep -a` (with
`LC_ALL=C` on captures), `strings`, or `ack`. A silent empty grep on a BRE file is
not evidence of absence.

## Reading the disassembly — reach for it early

The coastal support curve, industrial gold, unit production, the crown tax, the
technology system and the terrorist-op price were each read straight out of the
binary in minutes. **Prefer reading the code to fitting a curve**: a fit needs
dozens of samples and can still be wrong, and two constants were mis-set that way.

Use the static catalog (`scripts/bre-disasm.py`); never guess a segment base.
The method, the command cookbook and the traps are in
`references/disassembly.md`. Before you open it:

- **Read `docs/dev/bre-save-format.md`'s entry for the mechanic**, every time,
  even when the task arrives as fresh evidence. A whole session went into
  recovering the Queen Royale refund formula already written down there.
- **Grep the shipped prose** (above).
- **A reading is not a finding until it reproduces captured figures.**

## Driving BRE and keeping captures

BRE can be driven from a script under **dosemu2** (not DOSBox, which is
graphical) and its screens scraped as text or with color. The harness, the
launch recipe, its landmines and the turn-by-turn flow are in
`references/harness.md`; read it BEFORE driving. Three rules decide whether to
start at all:

- **Never run two drivers against one tmux session**, and never kill it while BRE
  runs; quit through the menu so `inuse.flg` clears.
- **Playing creates real state.** Never drive a game Andy cares about; say what
  you enrolled.
- **Keep the raw capture in `cap/`** as `cap/<topic>-YYYYMMDD.cap`, never in a
  scratch directory you will delete. `cap/` is gitignored (it is verbatim BRE
  output) but it persists, and every claim in `docs/dev/bre-screens.md` must stay
  re-readable. On 2026-08-16 a pass corrected a dozen screens from captures that
  were then deleted with their directory, and two follow-up fixes stalled.

Record captured screens in `docs/dev/bre-screens.md`, citing the capture file.
Parsing captures, and the traps that produced wrong findings, is in
`references/captures.md`. Staging a realm or a setting in `game.dat` instead of
building one up in play is in `references/staging.md`; two-board league runs are
in `references/interbbs.md`.

## License boundaries — BRE is proprietary

Barren Realms Elite is copyrighted (owned by John Dailey Software; designed by
Mehul Patel). Immortal Barons reimplements its *rules*. Reading the binary is
fine; what crosses into the repo is limited.

**Fair to replicate (facts and function):**
- Mechanics, rules and formulas.
- Numeric constants (unit stats, prices, caps, rates), recorded in
  `docs/mechanics-reference.md`.
- Menu structure, item order and hotkey layout.

**Never copy into the repo:**
- BRE's code, or code reconstructed or decompiled from the binary. Learn the
  constant or formula, then write our own implementation.
- **Display prose**: news lines, help text, result reports, flavor. The
  functional furniture of a screen (a question prompt, a field label, a menu
  item, a short refusal) MAY match word for word. AGENTS.md carries the rule and
  its test: is the game asking a question, or telling a story?
- **ANSI art, logos, splash and end screens.**
- **Distinctive flavor names** are Andy's call, one at a time (#218). Ask rather
  than renaming or reverting one.
- **BRE's files themselves**: never commit or bundle `BRE.EXE`, `BRE.OVR`, its
  data, or anything extracted from them. `--details` output and debugger dumps
  count.

**The trap is SCREEN TITLES.** A captured screen is copied for layout and colors,
and the product name in its header looks like part of the design. It is
branding: replace it. IB's InterBBS Scores shipped as `Barren Realms Elite: Top
Planets by Score` for months. Naming the original in prose (the About screen,
the README's Heritage, a divergence note) is fine; the line is identity, not
mention.

BRE's own license and the case law behind reading a binary for its functional
elements are in `references/license.md`. When unsure whether something has
crossed from mechanic into expression, ask, and do not copy.

## Other guardrails

- **No third-party private contact info** (John Dailey's or anyone's) in any
  repo artifact.
- **CP437 vs UTF-8.** BRE strings are CP437; IB emits UTF-8. Map a high byte to
  its Unicode glyph, never paste it raw.

## After gathering

State the source and confidence in the reply, e.g. "from BRE.OVR string table
(labels and order authoritative; colors unknown, need a capture)". Update
`docs/mechanics-reference.md` when a verified value changes.

**Name every routine you identify, in the same pass**, in
`scripts/bre-semantic-names.json`: name, `confidence`, and an `evidence` list.
- **The callers pick the name's shape**: a routine with one parent takes
  `parent__child`; one shared by several takes a plain descriptive name.
- **Match the file's formatting**: `evidence` arrays sit on one line. Insert the
  text; a `json.dump(indent=2)` round-trip turns three entries into a 1,800-line
  diff.
- **The generated catalog is derived; do not hand-patch it.** Nothing
  regenerates it automatically. Rebuild it with
  `bre-disasm.py analyze --directory "$BRE" -o docs/dev/bre-v0988-disassembly.json`
  (needs Capstone), in the same commit as the names, and run `check-catalog`. If
  you cannot rebuild it, say so.

## Keep this skill current

When you learn something new about *how to gather from BRE* (a technique, a
harness landmine and its fix, a source that proved authoritative or not), fold
it in during the same pass. Verified mechanic *values* go to
`docs/mechanics-reference.md`, build-up technique to
`docs/dev/bre-buildup-strategy.md`; this guide holds the *method*.

**Put it in the right file, and merge rather than append.** This core loads on
every session that touches BRE, so it holds only what decides whether you start
correctly: the source ladder, the calibration rules, the license line. Detail you
need once the work is under way goes in `references/`, into the section of that
file it belongs to, merged with any entry already saying the same thing:

| File | What lives there |
| --- | --- |
| `references/disassembly.md` | the catalog, finding constants and fields, names and callers, control flow, Real48, porting a screen, the debugger |
| `references/captures.md` | parsing `.cap` files, deciding a screen is uncaptured, census and probe techniques |
| `references/harness.md` | driving BRE under tmux/dosemu2, its landmines, the turn-by-turn flow |
| `references/staging.md` | editing `game.dat` to stage a realm or a setting |
| `references/extraction.md` | string extraction, news templates, variant strings |
| `references/interbbs.md` | two-board league runs and the local InterBBS ring |
| `references/license.md` | BRE's license text and the case law behind the clean-room line |

The core was split once at 1,452 lines and had grown back to 1,178 by
2026-09-27, mostly by appending. Add a one-line hook here at most.
