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
   BRE, ask a maintainer or contributor who can. For **text content and layout**
   (not colors), you can also drive BRE yourself headlessly — see "Running BRE
   headless" below.
2. **A disassembly of the original binary** — authoritative for exact numeric
   constants. Disassembled values override a reconstruction or a guess when
   they conflict.
3. **`BRE.OVR` / `BRE.EXE` strings** — authoritative for menu **labels** and
   **declaration order** (which equals menu order). See the extraction cookbook.
4. **`game/*.hlp`, `docs/`, `breins.txt`** — prose, help text, tutorial wording.

## Running BRE headless (tmux + dosemu2 harness)

Proven working (2026-07): BRE can be driven scriptably and its screens scraped
as plain UTF-8 text. Good for menu text/layout/flow (NOT colors — capture-pane
drops them; use a screenshot for colors, or `capture-pane -e` untested).

**Prerequisites — check first; do NOT assume or auto-install.** This harness
needs three things:

1. **`dosemu2`** (or `dosemu`) and **`tmux`** installed on the machine. Verify
   with `command -v dosemu tmux`. If `dosemu2` isn't packaged for the user's
   distro, point them at one of: **appman/AM** (`https://github.com/ivan-hc/AM`),
   a prebuilt **dosemu2 AppImage**
   (`https://github.com/theimpossibleastronaut/dosemu2-appimage/releases`), or a
   **Docker image** (`https://github.com/theimpossibleastronaut/dosemu2-container/`).
   `tmux` is in every mainstream distro's repos.
   - **Native install / AppImage is the proven path** — dosemu2 runs on the host
     and the recipe below works verbatim.
   - **The Docker image also works for this text-scraping harness** (its `-t`
     text mode is a first-class use case, not just X11 games). Don't run tmux
     *inside* the container — make the container the pane's process on the host:
     `tmux new-session -d -s bre -x 80 -y 25 "docker run --rm -it -v ~/.dosemu:/home/dosuser/.dosemu ghcr.io/theimpossibleastronaut/dosemu2-container:release -t"`,
     then `send-keys`/`capture-pane` the host pane as usual. The BRE files must
     live under the bind-mounted `~/.dosemu` (drive_c + mutating game state
     persist there). **UNVERIFIED end-to-end for the BRE flow** — reasoned from
     the container's README, not tested; the one spot to confirm is that
     `docker run -it` allocates a pty cleanly inside a *detached* tmux pane.
2. **The original BRE distribution files** — normally just the official BRE DOS
   door archive downloaded from the John Dailey Software release site (or the BRE
   community; the project README links a BRE Discord), unpacked to a local
   directory (see "Getting the original BRE" above for the key files). This skill
   never ships or bundles them (license section).

**If any of these is missing, STOP and tell the user exactly what to get** — name
the missing package(s) to install via their system package manager (dosemu2,
tmux), and/or that they need to download and unpack the original BRE archive from
the official site and point the skill at it. Let the user install/download it
themselves — never install software on the user's machine, and never present the
harness as simply "unavailable" without naming the specific missing dependency.
Only proceed once all three are present. (`xclip`/`wl-copy` for clipboard sharing
are Andy-specific conveniences, not harness requirements.)

**Why dosemu2 specifically — not DOSBox.** The whole approach depends on
dosemu2's ability to render DOS **text-mode / INT 10h video to a real terminal**
(the `-t` S-Lang mode), so `tmux capture-pane -p` scrapes the door screens as
**plain UTF-8 characters** — no images, cheap and reliable, drivable blind in a
headless pane. DOSBox / DOSBox-X / DOSBox-Staging are fundamentally **graphical**
emulators: they emulate VGA and render even text-mode screens to an SDL window
(pixels), with no supported path to pipe character cells to a Unix TTY. Using
DOSBox would force **screenshot + OCR** for every read. DOSBox's more-active
development is aimed at game graphics/sound compatibility — an axis this task
doesn't use; dosemu2 is purpose-built for the Linux-integration / BBS-door /
text-redirection case. (DOSBox is perfectly fine for a human who just wants to
*play* BRE — it's the *scriptable text scraping* that needs dosemu2.)

**NOT VERIFIED (don't state as certain):** the "DOSBox can't be scraped as text"
claim is reasoning from how these emulators render (graphical SDL surface), not
from an actual test. Nobody here has tried to coax a text-mode TTY out of
DOSBox-X (the most feature-rich variant), so an obscure mode can't be ruled out.
The dosemu2 side IS proven (it's what this harness runs on). If DOSBox is the
only emulator available, TEST whether its screens are text-scrapable before
concluding either way — say it's unverified, don't assert it fails.

The recipe — every step below dodges a landmine that otherwise kills the run:

```
# Wrap dosemu in `script` so the raw stream (incl. ANSI COLOR) is logged to a
# file — capture-pane -p only ever gives you plain text. See "Capturing color".
tmux new-session -d -s bre -x 80 -y 25 'script -q -c "dosemu -t" /tmp/bre-color.cap'
sleep 9
tmux send-keys -t bre "C:" Enter; sleep 1
tmux send-keys -t bre "CD \\GAMES\\BRE-DOS" Enter; sleep 1
tmux send-keys -t bre "DATE 07-25-2026" Enter; sleep 1  # SEE CLOCK CHECK BELOW
tmux send-keys -t bre "SRDOOR local" Enter; sleep 2     # writes DOORFILE.SR
# type at the Name: prompt ONE KEY AT A TIME (see pacing below), then Enter
tmux send-keys -t bre "BRE" Enter; sleep 5
tmux capture-pane -t bre -p                             # scrape the screen as text
```

### Capturing color (the `script` wrapper) — verified 2026-07-21

`tmux capture-pane -p` reconstructs the pane as **plain text and drops all
color** — useless when the task is the menu color scheme. Wrapping dosemu in
`script` (as the recipe's launch line now does) logs the **raw pty byte stream**
— exactly what dosemu writes, SGR color escapes included — to `/tmp/bre-color.cap`
while you still drive/scrape the tmux pane normally. Proven end-to-end here: the
tmux→script→dosemu pty nesting boots, keys register via `send-keys`, and the file
fills with real `\x1b[..m` codes. Two gotchas:

- **`script` block-buffers.** The file sits at one 4 KB block until it flushes.
  It flushes on the child's clean exit, so quit with **`EXITEMU`** (not
  `tmux kill-session`, which can lose the tail) before reading `/tmp/bre-color.cap`.
- **The file is noisy** — dosemu repaints the whole screen via S-Lang, so expect
  heavy cursor-positioning escapes and full-frame redraws between menus. The
  color is in there; grep the SGR runs (`grep -aoE $'\x1b\\[[0-9;]*m'`) near the
  menu text you want. For text/layout keep using `capture-pane -p` (cleaner); use
  the color file only when you actually need colors.
- Minicom's built-in capture (`Ctrl-A L`), by contrast, **strips** escape
  sequences — so for a real BBS door session, wrap minicom the same way:
  `script -q -c minicom /tmp/bre-color.cap`.

The landmines:

- **The clock check.** BRE compares the DOS date against its data files and
  exits instantly with `ERROR: Computer Clock has been tampered with` if the
  boot date is EARLIER than the game's last-recorded date. A fresh dosemu boot
  uses the host date, but the game data may have been written under a dosemu
  session whose clock ran ahead. Fix: `DATE <late-enough-date>` before `BRE`.
  If BRE dies silently right after "Probation/Reprieve Area Size", it's this.
- **Key pacing.** A burst (`send-keys "Andy" Enter`) crashes dosemu `-t` /
  overflows the 16-byte BIOS keyboard buffer. Send ONE key per send-keys call
  with ~0.25–0.3s sleeps between; Enter as its own call.
- **Interactive prompt, not `-E batch`.** `dosemu -E FILE.BAT` auto-exits the
  emulator the moment the batch ends, wiping the screen you wanted; error
  messages also scroll away. Boot to `C:\>` and type commands stepwise.
- **Pipes are blind.** Piping stdin/stdout (`-dumb` mode) captures DOS teletype
  output only — BRE's door screens render via INT 10h video and never reach the
  pipe. The tmux pane (a real TTY at 80×25) is what makes them scrapable.
- **New-caller flow** (fresh world): intro text `Continue? (Y/n)` → `n` →
  `Name your Realm:` → name + Enter → confirm `(Y/n)` → `y` →
  `Would you like Instructions? (y/N)` → `n` → ANSI splash (takes ~5s) →
  pause → main menu. An EMPTY realm name makes BRE exit immediately.
- **Playing creates real state.** The run enrolls an empire under the
  DOORFILE.SR caller name in the sysop's actual game data — tell Andy so he can
  re-`reset`, and never run this against data he cares about. Don't run while
  his own dosemu session is up (single-instance conflicts).
- **Fresh test player = edit the name in DOORFILE.SR, then run BRE.** The caller
  name appears in **two** places in `doorfile.sr`: **line 1 and the last line**
  (both were `Andy`). Change BOTH to a new name and launch `BRE` — BRE enrolls a
  brand-new empire under that name. Use this to gather under a throwaway empire
  without touching Andy's own (his is under `Andy`); each distinct name is a
  separate realm in the shared game data.
- Quit cleanly: `0` at the main menu (+ `y` confirm), then `EXITEMU` at `C:\>`,
  then `tmux kill-session`.

### Driving turns and reading the in-game economy (2026-07-14, proven)

Once inside a turn you can scrape income/status numbers per turn. Hard-won rules:

- **Input model: menus take ONE keypress, no Enter; numeric prompts need Enter.**
  A menu (`Choice> Quit`, Diplomacy, Spending, Attack…) acts on a single key —
  `tmux send-keys -t bre "0"` (NO `Enter`). If you send `"0" Enter`, the `0`
  picks the item and the stray Enter is consumed by the NEXT screen as its
  default (usually Quit) — silently skipping a menu (this is why the Spending
  Menu "vanishes"). Numeric "How much will you give?" prompts DO need Enter.
  y/N prompts take a single key (`n`), no Enter.
- **Enter = the default everywhere:** Quit on a submenu, No on y/N, pay-full on a
  maintenance prompt, Yes on "continue?", Play Game at the main menu. So a turn
  can be driven almost entirely by repeated Enter — but you must key off the
  ACTIVE prompt to stop at the right screen (next point).
- **The screen is NOT fully cleared between transitions.** Income lines linger in
  the upper pane while the active prompt is lower, so a screen-wide `grep` for
  "earned in Tourism" matches long after you've left the income screen. **Detect
  state from the active bottom line** (last non-empty line above the status bar):
  `cap | grep -v 'F2=Extra Information' | awk 'NF{l=$0} END{print l}'`.
- **The "Do you wish to visit the Bank? (y/N)" screen shows income AND status
  together** — one capture yields Tourism/Ore/Solar gold, Popular Support, Tax
  Rate, region counts, and Population. Best single per-turn data point.
- **System Menu (Set Tax Rate, Preferences, Write Macros, Empire Status, Set
  Industries) is reached via the Spending Menu's `(*) System Menu`** — the
  Spending Menu appears every turn AFTER the bank prompt and maintenance. Send
  `*` (single key) there.
- **Preferences (System Menu → P) to streamline a scripted run:** turn OFF Visit
  Covert/Trading/Message menus, turn ON Auto-Pay Maintenance + Auto-Feed Empire,
  and turn OFF "Deposit gold at End of Turn". Auto-pay only stays SILENT when you
  have enough gold ON HAND — depositing at end-of-turn sweeps it to the bank and
  makes auto-pay re-prompt every turn, so leave deposit off.
- **A "gold requested to boost popular support" prompt appears every turn when
  Support < 100** (no pref to disable). Enter `0` to decline (lets support erode);
  Enter the default to pay (raises support). Use it — plus the tax rate — to
  drive Support up or down for a sweep.
- **Conditional prompts a blind macro will desync on:** the boost-support prompt
  (support<100), a "People Need N food" prompt (only when short — gone with
  Auto-Feed), a "buy a lottery ticket?" event, and "Change Production? (y/N)".
  Handle each by matching the active line, not by a fixed key count.
- **Support DYNAMICS (for setting up a sweep):** tax ≤ ~60 barely erodes support
  (equilibrium 80-98); tax = 100 crashes it ~40/turn (faster at low pop/urban).
  To sweep the full range, crash with tax 100, then recover with tax 0 + paying
  the boost prompt. Support recovers to 100 overnight (daily maintenance).
- **A stale `inuse.flg`** left by an unclean exit makes BRE report "someone is
  currently playing… on another node." Delete it (`rmw .../inuse.flg`) before
  relaunching.

### Bank + turn-flow notes (2026-07-17, proven)

- **`SRDOOR local` prompts `Name:` interactively** — type the caller name one
  key at a time (not a burst) then Enter, same pacing as any BRE prompt. It
  rewrites `doorfile.sr` line1+last with that name.
- **The bank re-opens after the maintenance prompts inside a turn.** Turn order
  after the income screen is: `Do you wish to visit the Bank? (y/N)` → (bank if
  y) → maintenance prompts (Armed Forces, Regions, boost Popular Support, Queen
  Royale taxes, food) → the **Crazy Gold Bank menu appears AGAIN** → `0` to
  quit it → Spending/Buy-Military menu → Attack menu → InterPlanetary Ops menu →
  `Do you wish to continue? (Y/n)` (n = stop, back to scores+main menu). Key off
  the active line; don't assume one bank visit per turn.
- **`0` at the main menu quit straight to `C:\>` with NO `y` confirm** in this
  run (v0.988). The earlier "+ y confirm" note may be version/preference
  dependent — check the active line rather than blind-sending a `y`.

### Setting up an efficient test run (2026-07-21, proven)

Before a mechanics test that needs lots of gold/agents/military (covert costs,
attack outcomes), **don't blow your starting gold on the first turn.** Two
levers make testing far faster:

1. **Ask the maintainer to set reset config options for testing, up front.**
   The BRE Configuration Editor (sysop) has options that remove testing
   friction. Proven-useful settings: **Maintenance Costs: None** (income is not
   eaten each turn, so gold accumulates), **Turns of Protection: 0** (empires
   are attackable/covert-targetable immediately, no new-realm shield),
   **Tax Rate: 0** (popular support stays ~100, no erosion), high **Max
   Purchasable Regions**. Whoever runs the reset controls these — **remind them
   to set them** rather than grinding around the defaults. (Note: "Region Costs:
   None" and "Maintenance Costs: None" are *maintenance*-cost toggles, NOT the
   purchase price — regions still cost ~gold to BUY, and the buy quantity is
   gold-limited.)

2. **Play a few turns to build a saturated income BEFORE spending on the test.**
   The fastest income engine is **Coastal regions** (Tourism): buy the max
   affordable Coastal each turn for 2-3 turns and Tourism compounds hard —
   observed ~12k → 147k → 403k gold/turn over three buys. With Maintenance None
   you keep it all. Once income is saturated, stop buying land and let gold pile
   up for a turn or two; now you can afford repeated/expensive ops. Keep Tax
   Rate low (support high) or Coastal output "slumps drastically."

**Blind turn-advance macros desync on conditional prompts** — the boost-support
prompt (support < 100), a food-shortage "reconsider? (Y/n)" after underpaying
food, a lottery event, "Change Production?". A fixed key-count loop will feed a
key to the wrong prompt (e.g. a `0` meant for a menu becomes `0` food → a
DISASTEROUS-results warning). Drive prompt-by-prompt keying off the active
bottom line, not a blind burst, for any turn that might hit one of these.

**Two empires for attacker/victim tests.** BRE seeds no AI opponents on a fresh
reset — only your enrolled realm exists. To test attacks/covert against a
target and read the *victim* side, enroll a SECOND realm (edit both the first
and last line of `doorfile.sr` to a new caller name, launch, name the realm),
then switch back. Repeat the doorfile swap to alternate attacker/victim.

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
