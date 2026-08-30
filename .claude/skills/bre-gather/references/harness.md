<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. -->

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
  **The stored date is not discoverable — probe for it.** It is not the mtime of
  `game.dat` (2026-08-09: mtime was a week in the past while the stored date was
  months in the future), and `strings game.dat` finds no date, because it is
  packed binary. Loop a few candidate dates, launching BRE after each and
  grepping the pane for `tampered`, stopping at the first that clears. Start
  close to today and step outward — every day skipped is a day of maintenance
  BRE runs on the next launch, and a jump of months can idle-remove the realms
  you were about to test with. That is exactly what happened on 2026-08-09.
  **When nothing in the game is worth keeping, `BRE RESET` at the real system
  date beats probing.** It re-stamps the stored date to today, so the trap stops
  firing for good and no maintenance days are burned. Probe forward only to reach
  an existing game you need intact.
- **Key pacing.** A burst (`send-keys "Andy" Enter`) crashes dosemu `-t` /
  overflows the 16-byte BIOS keyboard buffer. Send ONE key per send-keys call
  with ~0.25–0.3s sleeps between; Enter as its own call. Use **`send-keys -l`**
  for a literal character, so a key that shares a name with a tmux key (`0`,
  `y`, `n` are fine, but `Space`, `Enter`, `BSpace` are not) is never
  reinterpreted.
- **NEVER run two drivers against one tmux session at the same time.** They
  interleave keystrokes into the same keyboard and each one reads a screen the
  other is changing, so both misfire and the trace looks like BRE behaving
  randomly. Cost two failed runs on 2026-08-09: the first driver was still
  looping when the second launched BRE again, and the first driver's leftover
  Enters answered the second's prompts. Before starting a driver, `pgrep -f` the
  previous one and kill it; a driver that has "finished" from your point of view
  may still be mid-`sleep`.
- **A generic `Enter` fallback in a driver is dangerous, not a safe default.**
  At `Name your Realm:` an empty answer makes BRE exit immediately, so the
  catch-all branch silently ends the run. Match every prompt the flow can reach
  explicitly, and make the fallback *capture and stop* rather than press a key.
- **`BRE PLANETARY` is the InterBBS packet pass — daily maintenance is NOT.**
  Launching `BRE` on a new day runs "Daily Maintainence" (dead empires, news,
  covert ops, trade deals) and writes NOTHING to the outbound directory. The
  league traffic is a separate invocation:

  ```
  BRE PLANETARY
  ```

  which prints "Checking Daily Maintenance / Updating Local Recon Info /
  Processing Incoming Data / Updating Routing Data / Updating NodeList /
  Releasing Group Attacks / Packing Group Attacks / Packing IP Attack
  Information / Updating Outgoing Gooie Kablooie Status / Processing Outbound
  Data / Checking for Any Lost Attacks / Planetary Maintenance Complete".
  This is what IB's own `-planetary` flag mirrors. If an outbound directory
  stays empty while you are waiting for scores or an attack to ship, this is
  almost certainly why. Order per board: play a turn -> `BRE PLANETARY` -> move
  the file to the other board's inbound -> `BRE PLANETARY` there to ingest.
- **`ROUTE.CFG` must point at the OTHER node, and a copied board gets it wrong.**
  The stock file is a single line, `ROUTE * 2` — send everything to node 2. Copy
  a board to make node 2 and it now routes to ITSELF, so `BRE PLANETARY`
  completes normally and exports NOTHING, with no error. That silence is the
  symptom. On node 1 use `ROUTE * 2`; on node 2 use `ROUTE * 1`.
- **A working export looks like this**, in the outbound directory named by
  `bbs.cfg` line 4:

  ```
  999b0102.002     the data packet
  brnodes.999      the node list, redistributed with it
  ```

  `999b0102.002` = league 999, game `b` (BRE), from node 01, to node 02,
  sequence `.002`. This is the filename scheme the transport discussion (GH
  #117/#121) is about — league, game, source, dest, sequence.
- **BRE reads inbound packets from the SAME directory it writes outbound to.**
  `bbs.cfg` line 4 is the outbound netmail dir and line 5 the "inbound files"
  dir, but dropping a received packet in the line-5 directory alone does
  NOTHING — `BRE PLANETARY` completes and leaves it sitting there. Put it in the
  line-4 (outbound) directory and it is consumed on the next pass. The stock
  bbs.cfg hints at this: its line 5 points at a directory named `...\OUTBOUND`.
  A successful ingest produces the reply packet plus an FTN netmail attach:

  ```
  OUTBOUND/999b0201.003    the reply, node 02 -> 01
  INBOUND/2.msg            the .MSG netmail attach
  ```

  So a full round trip is: A `BRE PLANETARY` -> copy A/OUTBOUND/* into
  B/OUTBOUND -> B `BRE PLANETARY` -> copy B/OUTBOUND/* into A/OUTBOUND -> ...
  This also confirms BRE moves league traffic as ordinary FTN netmail with a
  file attach, which is what GH #117 was asking about.
- **Daily maintenance only fires when the DOS date has actually moved on**, and
  a fresh reset can leave it stamped such that +1 day is not enough; +4 worked.
  Verify with a bare `DATE` before concluding the advance took, and clear any
  stray character left on the command line by a quit-confirm first.
- **Configuration Editor: the "help" screen IS the edit screen.** Pressing Enter
  on a field opens a full-screen page whose top is that field's help text — and
  whose BOTTOM line is `Enter New Value: (current; max)`. Type the value and
  press Enter to return to the field list. It is NOT a dead-end popup, and it
  does not need dismissing. Full sequence:

  ```
  BRE RESET  ->  y  ->  Up/Down to the field  ->  Enter  ->  type value  ->  Enter
  PG-DN / PG-UP change page;  ESC saves and quits;  then answer the
  "Would you like this to be a league-wide reset?" prompt.
  ```

  **This cost two long detours before it was written down.** The trap is reading
  only the top of the pane: the help text fills the screen and looks modal, so a
  driver that captures `head` sees a popup and starts guessing dismiss keys.
  ALWAYS CAPTURE THE BOTTOM OF THE PANE before concluding a screen is stuck —
  `tmux capture-pane -p | tail -5`. Related trap: pressing a DIGIT on a field
  also opens the same screen, which made it look like digits were "help" and
  Enter was too.
- **`inuse.flg` lives in the BOARD'S ROOT, not `data/`.** Deleting
  `<board>/data/inuse.flg` clears nothing; the stale-lock error persists. It is
  `<board>/inuse.flg`.
- **The Trading Market exits on ESC, not `0` or Enter.** Its screen lists no
  Quit key and its `Your Choice?` prompt silently redraws on both, so a driver
  that presses `0` until it sees a known screen loops forever. `tmux send-keys
  -t bre -H 1b` leaves it (and pops the Trading submenu with it, landing on the
  System Menu). Found while identifying the market escrow fields, 2026-08.
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
- **Playing creates real state — so do not play in `bre-dos` at all.** Copy a
  snapshot to a NEW directory under `~/.dosemu/drive_c/games/` (e.g. `bre-cap`)
  and `CD` there instead. BRE reads its data relative to the current directory,
  and `bbs.cfg` only names outbound paths, so a copied board runs standalone
  with no edits. Andy's own install is then untouched by construction and there
  is nothing to restore afterwards — which beats snapshot-and-restore, because a
  restore you forget is a restore that did not happen. Say which directory you
  made so he can delete it.
- **`~/.dosemu/drive_c/dat-bak/` holds ready-made scenarios.** `saved4` is three
  built realms with a Full Defense Alliance already signed — the fastest way to
  a post-battle report, an Alliance Strength ally row, a `-*Relations*-` roster
  with a real treaty in it, and an incoming treaty-offer prompt. Read its
  `README.md` for the realm letters and the DOS date it needs before deciding to
  grind a game up from a reset.
- Don't run while Andy's own dosemu session is up (single-instance conflicts).
- **`doorfile.sr` lines must end `\r\n` — a bare `\n` merges lines.** Turbo
  Pascal's ReadLn only breaks on CR, so a name line written with a Unix newline
  swallows the line after it and a numeric read lands on text: BRE dies at boot
  with `Run-Time error #106 ... Invalid numeric format` before any screen.
  `bre-name.sh` writes CRLF now; any other tool that touches the drop file must
  too. (Cost a run, 2026-08-30.)
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
- **The Configuration Editor DOES accept edits in a headless pane** (settled
  2026-08-16; this note used to say it was read-only, which cost sessions of
  grinding around it). Enter on a field opens a full-screen page whose top is
  that field's help and whose bottom is `New Setting:`. From there:
  - a **preset** field (Maintenance Costs, Attack Damage, Sabre Handling, Buy
    Military) commits on **one key** — `H` sets High and returns to the list, no
    Enter;
  - a **numeric** field takes digits then Enter;
  - arrow keys move the highlight, PG-DN / PG-UP page, and the highlighted field
    is drawn bright-white on both label and value so you can see where you are;
  - `tmux send-keys -H 1b 1b` leaves the editor — a single `1b` was swallowed.

  The trap that produced the old note: the edit prompt is at the BOTTOM of the
  page while the help text fills the top, so a `head`-style capture shows a
  modal-looking wall of prose and no prompt. Always
  `tmux capture-pane -p | tail -5`.

  So **`BRE RESET` is the cheap A/B for "does the sysop's cost knob scale this
  price?"** — reset a scratch board with every cost preset at High, enrol a
  realm, and read the price off the screen that quotes it. That settled two
  questions in one run: local covert fees do not move, and Terrorist Ops does
  (`regions x 64 x 3` at High).

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

### Two-board InterBBS runs

Anything needing two BRE installs talking to each other — interplanetary attacks,
recon exchange, coordinator broadcasts, packet loss — is in
`references/interbbs.md`. Both boards are already installed and paired on this
machine, and the findings from the earlier league run are in GitHub issues #60
through #65 rather than in this skill.

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
