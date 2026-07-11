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

Immortal Barons is a faithful clone. When a task is "make this match BRE" —
a menu's items/order, a mechanic's numbers, a screen's layout or colors — the
authoritative answer is in BRE's own files, not memory. Check the source, cite
which source, state the confidence. Guessing drifts the clone away from the
original and costs a round-trip when it's caught in review.

## Getting the original BRE

You need your own copy of the **original BRE distribution** (the DOS door
release archive) — this skill never ships or bundles it (see the license
section). If you don't already have it, obtain the archive from the John Dailey
Software release or the BRE community (the project README links a BRE Discord),
and unpack it somewhere local. Point a shell variable at that directory; every
command in this skill uses it:

```
BRE=~/path/to/bre-dos     # wherever you unpacked it, e.g. a DOSEMU drive_c/games/bre-dos
```

Key files inside:

- **`BRE.OVR`** — the overlay; holds the bulk of menu strings, prompts, news
  text. First place to look for labels and menu order.
- **`BRE.EXE`**, **`BREDATA.EXE`** — main binaries; some strings, the config
  screens.
- **`game/*.hlp`, `game/breins.txt`, `game/bre.txt`, `docs/`** — help/prose.
- `data/planet.bre`, `data/game.dat` — runtime save data (not menu/mechanic
  definitions).

## Source priority (most authoritative first)

1. **A rendered screenshot from a live BRE session.** The ONLY authoritative
   source for **colors** and for the exact **hotkey characters + on-screen
   order**. Anyone with BRE running can grab one — select the menu/screen and
   paste it. Prefer this whenever colors or exact keys matter; if you can't run
   BRE, ask a maintainer or contributor who can.
2. **A disassembly of the original binary** — authoritative for exact numeric
   constants. Disassembled values override a reconstruction or a guess when
   they conflict.
3. **`BRE.OVR` / `BRE.EXE` strings** — authoritative for menu **labels** and
   **declaration order** (which equals menu order). See the extraction cookbook.
4. **`game/*.hlp`, `docs/`, `breins.txt`** — prose, help text, tutorial wording.

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

See `references/extraction.md` for the exact `strings` / `grep -abo` / `dd|xxd`
commands and worked examples.

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
  "Doomer Kaboomer".

**Trademark / branding:** do not present the project as "Barren Realms Elite" or
use its name/logo as our branding. IB is an independent tribute, "not affiliated
with, or endorsed by" the original authors (README "Heritage").

**Never ship BRE's files:** do not distribute, commit, or bundle `BRE.EXE` /
`BRE.OVR`, its data files, or any asset extracted from them. They stay in your
own local reference copy only.

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
