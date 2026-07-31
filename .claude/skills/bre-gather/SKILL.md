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
as plain UTF-8 text with `tmux capture-pane -p` (menu text/layout/flow). For
**colors**, `tmux capture-pane -ep | cat -v` is PROVEN (2026-07): the `-e` flag
keeps the ANSI escapes, and `cat -v` renders them readably as `^[[NNm` SGR codes
— this is the headless way to read BRE's colors (no screenshot needed; the old
"use a screenshot for colors" note is superseded).

**Record literal captures in `docs/dev/bre-screens.md`, not here.** That doc is
the durable catalog of BRE's exact output — wording, borders/decorations,
numeric prompts, and the ANSI color of every element — so implementations don't
re-drive the binary each time. Add to it whenever you capture a new screen; keep
this skill about *how* to drive/scrape, and put the *captured data* in that doc.

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
- **ESC needs the raw byte, not `send-keys Escape`.** On S-Lang "ESC to Save &
  Quit" screens — BRE's Configuration Editor (from `BRE RESET`) is the one that
  bit us — `tmux send-keys Escape` is silently swallowed: S-Lang holds a lone
  ESC, waiting to disambiguate an escape sequence, and it never delivers. Send
  the raw byte instead: `tmux send-keys -t bre -H 1b` (double it, `-H 1b 1b`, if
  one doesn't take). Same fix for any menu that ignores ESC.
- **Headless (agent) driving = background script + scrape.** Running this from
  Claude Code, there's no live pane to watch and foreground `sleep` is blocked,
  so don't run the recipe inline. Put the whole drive sequence (with its
  `sleep`s) in a small script and run it BACKGROUNDED, ending in
  `tmux capture-pane -t bre -p > /tmp/bre.cap`; then read that file. Each
  interactive step (send a key → sleep → capture) is one such backgrounded call.
  A human instead just `tmux attach`es and types — this scaffolding is only for
  the blind/headless case.
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
- **`SRDOOR local` prompts `Name:` and it must be TYPED — never just Enter.**
  An empty answer writes an EMPTY caller name; BRE then treats you as a brand-new
  caller, asks "Name your Realm:", and exits on the empty answer, silently
  ending a scripted run. `bre-nextday.sh` reads the name back out of
  `doorfile.sr` line 1 and types it a key at a time for this reason.
- **A whole-pane grep for the main menu outranks a pending prompt.** BRE leaves
  the main menu on screen while the event log scrolls in beneath it, so a driver
  that checks `grep '(1) Play Game'` against the WHOLE pane will keep re-sending
  "1" and never answer the lottery / send-a-message prompt sitting on the active
  line. Answer active-line prompts BEFORE any whole-pane guard — this hung a
  grind until it was found. Fixed in `bre-drive.sh`.
- **Fresh test player = edit the name in DOORFILE.SR, then run BRE.** The caller
  name appears in **two** places in `doorfile.sr`: **line 1 and the last line**
  (both were `Andy`). Change BOTH to a new name and launch `BRE` — BRE enrolls a
  brand-new empire under that name. Use this to gather under a throwaway empire
  without touching Andy's own (his is under `Andy`); each distinct name is a
  separate realm in the shared game data.
- **ALWAYS quit BRE normally, the way a human would — never `tmux kill-session`
  while BRE is still running.** A normal quit is what clears `inuse.flg` AND
  flushes each empire's state to disk so realms persist; killing the pane mid-run
  leaves a stale lock and can lose the just-played turn. The clean path: `0` at
  the main menu (+ `y` confirm if prompted), let it return to `C:\>`, then
  `EXITEMU`, and only THEN `tmux kill-session` (the pane is already back at the
  DOS prompt, so nothing is lost).

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

### New-realm PROTECTION gates trading, not just combat (2026-07-30)

The config editor's own help for "Turns Of Protection" reads: a new empire is
"unable to attack, **trade**, and be attacked." So the Trading Market refuses a
protected realm with "Your dominion is still under protection." — which blocks
any market experiment until protection is burned off. Twenty turns at eight
turns/day is three game days PER EMPIRE, and a cross-empire sale needs two.

Two ways round it, one of which does NOT work:

- **Editing `data/game.dat` directly FAILS.** The config record sits at the top
  of the file and is easy to read — `+0x36` turns/day, `+0x38` turns of
  protection, `+0x3e` land created/day, `+0x40` interest, `+0x42` planetary tax
  (tenths of a percent, matching `bre-save-format.md`), `+0x6a` max purchasable
  regions. But BRE checksums it: patching protection to 0 produced
  `Error: Status File has been tampered with! Game will not run.` on the next
  launch. Restore from a backup copy and use the game instead. Those offsets are
  still worth having for READING a game's settings without driving the UI.
- **The Configuration Editor would not accept edits in a headless pane.**
  `BRE RESET` opens it, arrow keys move the highlight and PG-DN pages, but on a
  numeric field none of typing digits, Left/Right, `+`/`-`, `e`/`E`, Backspace,
  Space or Tab changed the value; Enter shows that field's help. Unresolved —
  if you find the edit key, record it here. Until then, assume the editor is
  read-only under `dosemu -t` and plan to burn protection with the driver.

So: **budget three game days per empire before any trade/attack experiment**,
and start the grind early rather than discovering the gate mid-run.

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

## Reading the disassembly — the method that works (2026-07-29, proven)

Several mechanics that resisted inference from play were read straight out of the
binary in minutes: the coastal support curve, industrial gold, unit production,
the crown tax, and the whole technology system. **Reach for this early rather
than fitting a curve** — a fit needs dozens of samples and can still be wrong,
and two BRE constants were mis-set that way before the disassembly corrected
them.

Tools: `ndisasm`, `radare2`, `ghidra` are installed. Copy `BRE.OVR` / `BRE.EXE`
somewhere writable first; never work on the originals.

The loop that keeps paying off:

1. **Find the message string.** Turbo Pascal strings are length-prefixed and sit
   in one contiguous table, and the routine that uses them very often begins
   IMMEDIATELY AFTER the table ends, at `55 89 E5` (push bp; mov bp,sp).
2. **Disassemble around it:**
   `dd if=BRE.OVR bs=1 skip=$((0xADDR)) count=$((0x100)) of=x.bin && ndisasm -b16 -o0xADDR x.bin`
3. **Find a constant** by searching `struct.pack("<H", value)` and checking the
   preceding byte for an imm16 opcode (`b8` mov ax, `05` add ax, `b9` mov cx …).
4. **Decode float constants.** Reals are 6-byte Turbo Pascal, loaded into a
   register triple right before the runtime call that consumes them. Use
   `scripts/bre-tpreal.py --scan FILE 0xSTART 0xLEN` (in this project's Claude
   dir) — its docstring carries the format, the empire/config record layout
   mapped so far, and the known runtime helper addresses.
5. **Validate against play.** A candidate reading is not a finding until it
   reproduces captured figures exactly. Rounding vs truncation and the order of
   operations both change results by a unit — the two real→int helpers differ
   only in that, and picking the wrong one is easy.

Record layout and helper addresses live in `docs/dev/bre-save-format.md`; extend
that file rather than re-deriving.

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
  "Doomer Kaboomer".

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

## Driving a BRE turn (v0.988, verified live)

Verified live against BRE v0.988 in local mode. A consolidated, step-by-step
account of launching the door, enrolling a caller, and driving one full turn.
Complements the tmux/dosemu harness notes above — this is the *game flow*, that
is the *emulator plumbing*.

### Launch sequence (from the `C:\` DOS prompt)

- `cd \GAMES\BRE-DOS`, then run **`SRDOOR local` FIRST**, THEN run **`BRE`**.
  `SRDOOR local` prompts `Name:` — type the caller/realm name one key at a time
  (same pacing as any BRE prompt), then Enter; it writes `doorfile.sr`. Running
  `BRE` *without* `SRDOOR local` first gives a Turbo-Pascal `Run-Time error #106
  Invalid numeric format while reading a file`. `local.bat` does exactly
  `SRDOOR local` then `BRE`.
- **Set the DOS `DATE` to >= the game's last-recorded date**, or BRE exits with
  "Computer Clock has been tampered with." This world's inception is 8-1-2026,
  so `07-27-2026` tripped the check and `12-01-2026` cleared it. Reaching `C:\>`
  never invokes the check — only running `BRE` does.
- `doorfile.sr` layout: the caller name is on **line 1 AND the last line**; lines
  2-7 are numeric (ANSI flag, IBM-chars flag, page length, baud, COM port,
  time-left). **Prefer generating it via `SRDOOR local`** over hand-editing —
  hand-editing risks mangling the DOS CRLF line endings.

### New-caller enrollment flow

hints screen `Continue? (Y/n)` (Enter) -> `Name your Realm:` (type name, Enter)
-> `Name Your Empire X? (Y/n)` (Enter = Yes) -> `Would you like Instructions?
(y/N)` (`n`) -> ANSI splash "Paused" (Enter) -> daily maintenance runs -> main
menu. A new empire is assigned the next free empire **LETTER** (existing empires
took the earlier letters).

### Main menu

(1) Play Game, (2) See Status, (3) See Scores, (4) Today's News,
(5) Yesterday's News, (6) Read Messages, (7) Send Messages, (8) Game Bulletins,
(9) InterPlanetary Ops, (A) Game Instructions, (B) Help Database,
(P) Preferences, (0) Quit.

### One turn's flow (after preferences are streamlined)

1. (1) Play Game.
2. **FIRST TURN ONLY:** event-log "Paused" (Enter) -> Diplomacy Menu (`0` to
   quit).
3. Industrial Production screen -> `Change Production? (y/N)` -> `n` (or `y` to
   set a production %, e.g. Tanks).
4. Income "Paused": lines for taxes / Ore Mines / Tourism / Solar Power / Food
   grown (Enter).
5. Status + a maintenance summary; prompt `Do you wish to visit the Bank?
   (y/N)` -> `n`. With Auto-Pay Maintenance + Auto-Feed ON, this shows
   "0 Gold paid / N units of Food consumed" and does NOT prompt for payment or
   food.
6. Crazy Gold Bank menu (`0` to quit).
7. **Spending Menu** — the buy hub: (*) System Menu, (1) Troopers, (2) Jets,
   (3) Turrets, (4) Bombers, (5) HeadQuarters, (6) Regions, (7) Covert Agents,
   (8) Tanks, (9) Carriers, (S) Sell, (V) Visit Bank, (0) Quit. Shows the current
   price and count owned per item, plus "You have X gold and N turns."
8. **Attack Menu** (`0` to quit): (R) Regular, (N) Nuclear, (C) Chemical,
   (B) Biological, (P) Attack Pirates, (A) Alliance Strength, (V) Visit Bank,
   (0) Quit.
9. **InterPlanetary Ops menu** (`0` to quit): View IPScores, Terrorist Ops, Send
   Trade Deal, Create/Join Group Attack, Indiv. Attack Force, Send Message,
   Special Operations, Gooie Kablooie Ops, SDI Program, Diplomacy List, Spy
   Database, Travel Times, Visit Bank, Quit.
10. End of Turn Statistics -> `Do you wish to continue? (Y/n)`. **`Y` starts the
    NEXT turn immediately (chains turns without returning to the main menu); `n`
    returns to the scores table then the main menu.**

**Turns:** 20 turns/day. When turns are exhausted, advance the DOS `DATE` by a
day and relaunch for 20 more.

### Setting preferences (streamline for scripted play)

From the Spending Menu press (*) System Menu, then (P) Preferences. Each numbered
line is a toggle — press the number to flip Yes/No:

- (1) Visit Covert Menu -> No
- (2) Visit Trading Menu -> No
- (3) Visit Message Menu -> No
- (4) Use Enter To Exit BUY Menu -> leave Yes
- (5) Deposit gold at End of Turn -> No (keeps Auto-Pay silent — depositing
  sweeps gold to the bank and makes Auto-Pay re-prompt)
- (6) Auto-Pay Maintenance -> Yes
- (7) Auto-Feed Empire -> Yes
- (0) to quit.

**Set tax rate:** System Menu -> (R) Set Tax Rate -> enter a number. 12% never
triggers tax riots while still earning tax income.

**System Menu also has:** (D) Diplomacy, (1) Set Industries, (3) Specialize
Industry, (E) Empire Status, (W) Write Macros, etc.

### Buying regions (Spending Menu -> 6)

Shows "There are N Regions available", warns that the region price shown is only
the price for the FIRST piece bought (prices change constantly), and "You can
afford N regions." Region types with hotkeys: (C) Coastal, (R) River,
(A) Agricultural, (D) Desert, (I) Industrial, (U) Urban, (M) Mountain,
(T) Technology, plus (*) Advisors. Select a type -> `Buy how many X regions?
(0; MAX)` -> press `>` for max, Enter to confirm. The marginal price RISES as you
buy within a single purchase AND ratchets up persistently across the turn
(observed 1412 -> 2402 after buying 30 in one go).

### Input model (recap)

Menu selections take ONE key with no Enter; numeric prompts need Enter; y/N
prompts take a single key; "Paused" screens continue on any key; at numeric
prompts `>` = max and `m`/`k` append 6/3 zeros. Enter triggers the default
everywhere.

### Headless driving (recap)

Capture via `script -q -c "dosemu -t" <file>.cap` to a `*.cap` in the repo root
(gitignored) — this preserves ANSI color (SGR codes); `script` block-buffers, so
the file flushes on a clean exit (quit BRE with `0`, then `EXITEMU`). `tmux
capture-pane -p` gives plain text (no color). Reusable driver scripts live in
`~/.claude/projects/-home-andy-src-andy5995-immortal-barons/scripts/`
(`bre-launch.sh`, `bre-key.sh`, `bre-type.sh`, `bre-name.sh`, `bre-play.sh`,
`bre-drive.sh`). A robust turn-driver keys off the active bottom line (last
non-empty line excluding the F2 status bar), not a fixed key count, so it
survives conditional prompts.

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
