<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. Organized by topic:
merge a new landmine into the entry it belongs to rather than appending. -->

# Driving BRE headless (tmux + dosemu2)

BRE can be driven from a script and its screens scraped as plain text with
`tmux capture-pane -p`, or with color through `capture-pane -ep | cat -v` or the
`script` log below. Record what you capture in `docs/dev/bre-screens.md`, and
keep the raw capture in `cap/` (SKILL.md, "Driving BRE and keeping captures").

## Prerequisites

- **`dosemu2`** (or `dosemu`) and **`tmux`**: `command -v dosemu tmux`. Where
  dosemu2 is not packaged, point the user at appman/AM
  (https://github.com/ivan-hc/AM), the dosemu2 AppImage
  (https://github.com/theimpossibleastronaut/dosemu2-appimage/releases) or the
  container (https://github.com/theimpossibleastronaut/dosemu2-container/). The
  native and AppImage installs are proven. The container is UNVERIFIED for this
  flow: run it as the pane's process on the host
  (`tmux new-session -d -s bre -x 80 -y 25 "docker run --rm -it -v ~/.dosemu:/home/dosuser/.dosemu ghcr.io/theimpossibleastronaut/dosemu2-container:release -t"`),
  not tmux inside it.
- **Your own BRE copy** (SKILL.md, "Getting the original BRE").

If one is missing, stop and name exactly what to install or download. Never
install it yourself.

**Why dosemu2, not DOSBox.** dosemu2's `-t` mode renders DOS text-mode video to a
real terminal, so the screens scrape as characters. DOSBox and its variants
render to an SDL window, which would mean screenshots and OCR. That DOSBox-X has
no text mode is reasoning, not a test: if DOSBox is all there is, try it before
saying it fails.

## The launch recipe

```
tmux new-session -d -s bre -x 80 -y 25 \
  'script -q -c "dosemu -t" ~/src/andy5995/immortal-barons/cap/<topic>-YYYYMMDD.cap'
sleep 9
tmux send-keys -t bre "C:" Enter; sleep 1
tmux send-keys -t bre "CD \\GAMES\\<SCRATCH-BOARD>" Enter; sleep 1
tmux send-keys -t bre "DATE 07-25-2026" Enter; sleep 1   # the clock check, below
tmux send-keys -t bre "SRDOOR local" Enter; sleep 2      # then type the name a key at a time
tmux send-keys -t bre "BRE" Enter; sleep 5
tmux capture-pane -t bre -p
```

`script` logs the raw pty stream, color escapes included, while you drive the
pane as usual. Two things about the log:

- **It block-buffers**, and flushes on a clean exit. Quit BRE, then `EXITEMU`,
  before reading it; killing the session can lose the tail.
- **It is noisy**: dosemu repaints whole frames. Grep the SGR runs
  (`grep -aoE $'\x1b\\[[0-9;]*m'`) near the text you want, and use
  `capture-pane -p` for text and layout.

Minicom's own capture (`Ctrl-A L`) strips escapes; for a real BBS session, wrap
minicom the same way.

**Running from Claude Code**: there is no pane to watch and a foreground `sleep`
is blocked, so put each drive sequence in a script, run it backgrounded, and end
it with `tmux capture-pane -t bre -p > <file>`. Two things that do not work: a
batch file (`dosemu -E FILE.BAT` exits the emulator when the batch ends, wiping
the screen), and piping stdin/stdout, which sees only DOS teletype output and
none of BRE's INT 10h screens.

The driver scripts live in this project's Claude scripts dir
(`~/.claude/projects/-home-andy-src-andy5995-immortal-barons/scripts/`):
`bre-launch.sh`, `bre-launch-dir.sh`, `bre-key.sh`, `bre-type.sh`, `bre-name.sh`,
`bre-play.sh`, `bre-drive.sh`, `bre-nextday.sh`, `bre-clone-slot.py`.
`bre-launch.sh` hardcodes `\GAMES\BRE-DOS`; use `bre-launch-dir.sh <BOARD-DIR>
<capfile> [date]` for a scratch copy.

## Work in a scratch copy

**Playing creates real state, so never play in `bre-dos`.** Copy it to a new
directory under `~/.dosemu/drive_c/games/` and `CD` there. BRE reads its data
relative to the current directory and `bbs.cfg` names only outbound paths, so a
copy runs standalone with no edits, and Andy's install is untouched without any
restore step to forget. Say which directory you made.

- **`~/.dosemu/drive_c/dat-bak/` holds ready-made scenarios.** `saved4` is three
  built realms with a Full Defense Alliance signed; its `README.md` gives the
  realm letters and the DOS date it needs.
- **Do not run while Andy's own dosemu session is up.**
- To stage a realm or a setting instead of playing one up, see `staging.md`.

## Landmines

**The clock check.** BRE exits with `ERROR: Computer Clock has been tampered
with` when the DOS date is EARLIER than the game's last-recorded date; if it dies
silently after "Probation/Reprieve Area Size", it is this. Only running `BRE`
triggers it. The stored date is packed binary and is not the file's mtime, so
probe: try dates from today outward, grepping the pane for `tampered`. Every day
skipped is a day of maintenance on the next launch, and a jump of months
idle-purges realms, including any you staged before the jump (probe first and
stage afterward, or clone a realm that survived with `bre-clone-slot.py`). When
nothing in the game matters, **`BRE RESET` at today's date** re-stamps it and
ends the problem.

**Daily maintenance runs only when the DOS date has moved on**, and after a fresh
reset +1 day may not be enough (+4 worked). Check with a bare `DATE`.

**Key pacing.** A burst overflows the 16-byte BIOS keyboard buffer and can crash
dosemu `-t`. Send ONE key per `send-keys` call with ~0.25–0.3 s between, Enter as
its own call, and `send-keys -l` for a literal character. **ESC needs the raw
byte**: S-Lang holds a lone `send-keys Escape` forever, so send `send-keys -t bre
-H 1b` (doubled, `-H 1b 1b`, if one does not take).

**One driver per session, ever.** Two drivers interleave keys and each reads a
screen the other is changing; the trace looks like BRE behaving randomly.
Before starting one, kill the last: a driver that "finished" may still be
mid-`sleep`. `pgrep -f drive.sh` matches the waiting loop's own command line, so
match the interpreter (`pgrep -af "bash .*drive.sh"`) or wait on a marker the
driver logs.

**A driver's fallback must capture and stop, never press a key.** A catch-all
Enter ends the run at `Name your Realm:`, where an empty answer exits BRE. A
catch-all `0`/Enter walks the menus, ends turns and answers money prompts: on
2026-09-12 one took a staged attacker from 1,000,038,170 gold to 34,482 over four
turns. Match every prompt by name.

**Key off the ACTIVE line, not the whole pane.** The screen is not cleared between
transitions, so income lines and the main menu linger above the live prompt. A
driver checking the whole pane for `(1) Play Game` kept re-sending `1` and never
answered the lottery prompt below it. Read the last non-empty line above the
status bar:
`capture-pane -p | grep -v 'F2=Extra Information' | awk 'NF{l=$0} END{print l}'`.
**And read the BOTTOM of a full-screen page**: the Configuration Editor's help
text fills the top and its edit prompt is the last line (`tail -5`).

**The drop file.** Run `SRDOOR local` before `BRE`; without it BRE dies with
`Run-Time error #106 Invalid numeric format`. `SRDOOR` prompts `Name:`: TYPE a
name, since an empty one enrolls a nameless caller who then exits at `Name your
Realm:`. The caller name is on line 1 AND the last line of `doorfile.sr`, and the
lines must end `\r\n`: Turbo Pascal breaks only on CR, so a bare `\n` merges two
lines and gives the same error #106. Prefer `SRDOOR local` to hand-editing.
**A new name enrolls a new realm**, which is how to get a second empire for
attacker/victim tests (BRE seeds no AI on a reset).

**Quit the way a human does.** `0` at the main menu (a `y` confirm sometimes,
depending on version and preference; read the active line), then `EXITEMU`, then
kill the session. That clears `inuse.flg` and flushes the realm. A stale lock
says someone "is currently playing… on another node"; the flag is
`<board>/inuse.flg` in the board ROOT, not `data/`.

**The Trading Market exits on ESC** (`-H 1b`), not `0` or Enter, which silently
redraw it; the ESC also leaves the Trading submenu.

**The Configuration Editor (`BRE RESET` → `y`) accepts edits.** Enter (or a
digit) on a field opens a page with that field's help on top and `Enter New
Value: (current; max)` at the bottom. A preset field (Maintenance Costs, Attack
Damage…) commits on one key (`H`); a numeric one takes digits and Enter. Arrows
move, PG-DN/PG-UP page, ESC (`-H 1b 1b`) saves and asks whether the reset is
league-wide. This makes `BRE RESET` a cheap A/B for "does this cost knob scale
this price?": every preset at High on a scratch board settled that covert fees do
not move and Terrorist Ops does (`regions x 64 x 3`). Editing the config bytes in
`game.dat` directly trips `Status File has been tampered with!`; see
`staging.md`.

## Driving a turn (v0.988, verified live)

**Input model**: a menu acts on ONE key with no Enter; a stray Enter is taken by
the NEXT screen as its default, which is how a menu "vanishes". Numeric prompts
need Enter (`>` is the max, `k`/`m` add three or six zeros). y/N prompts take one
key; Paused screens take any key. Enter is the default everywhere: Quit on a
submenu, No on y/N, pay-full on maintenance, Yes on "continue?", Play Game at the
main menu.

**Enrollment**: `Continue? (Y/n)` → `Name your Realm:` → confirm → `Would you
like Instructions? (y/N)` `n` → ANSI splash (~5 s) → daily maintenance → main
menu. A new empire takes the next free letter.

**Main menu**: (1) Play Game, (2) See Status, (3) See Scores, (4) Today's News,
(5) Yesterday's News, (6) Read Messages, (7) Send Messages, (8) Game Bulletins,
(9) InterPlanetary Ops, (A) Game Instructions, (B) Help Database, (P)
Preferences, (0) Quit.

**One turn**, preferences streamlined:

1. (1) Play Game. First turn only: event log, then the Diplomacy Menu (`0`).
2. Industrial Production → `Change Production? (y/N)`.
3. Income screen (taxes, Ore, Tourism, Solar, food).
4. `Do you wish to visit the Bank? (y/N)`. This screen shows income AND status
   together: the best single data point per turn.
5. Maintenance: Armed Forces, Regions, the boost-support prompt, Queen Royale
   taxes, food. Silent with Auto-Pay and Auto-Feed on and enough gold IN HAND.
6. The Crazy Gold Bank menu, again (`0`).
7. **Spending Menu**, the buy hub; `(*)` opens the System Menu (Set Tax Rate,
   Preferences, Set Industries, Specialize Industry, Empire Status, Write
   Macros, Diplomacy).
8. Attack Menu (`0`), then InterPlanetary Ops (`0`).
9. End of Turn Statistics → `Do you wish to continue? (Y/n)`: `Y` chains the
   next turn, `n` returns to scores and the main menu.

**Conditional prompts desync a fixed key count**: the boost-support prompt
(whenever support is under 100), a food shortage and its "reconsider?", the
lottery, "Change Production?". A `0` meant for a menu becomes 0 food and a
`DISASTEROUS` warning (BRE's own spelling). Drive prompt by prompt.

**Preferences** (System Menu → P; each number toggles): Visit Covert, Trading and
Message menus off; Use Enter To Exit BUY Menu on; Deposit gold at End of Turn
OFF (depositing sweeps gold to the bank and makes Auto-Pay prompt every turn);
Auto-Pay Maintenance and Auto-Feed on.

**Buying regions** (Spending → 6): the price shown is for the first piece only,
and it rises within a purchase and stays risen for the turn (1,412 → 2,402 after
30). "You can afford N regions" is capped by Max Purchasable Regions. Pick a type,
`>` for the max, Enter.

## Setting up a test run

- **Protection gates trading as well as combat** ("unable to attack, trade, and
  be attacked"), so the market refuses a protected realm. Twenty turns of
  protection at eight a day is three game days per empire; budget for it or ask
  for a reset with Turns of Protection 0.
- **Ask for testing config at the reset**: Maintenance Costs None (gold
  accumulates), Turns of Protection 0, Tax Rate 0 (support stays near 100), a high
  Max Purchasable Regions. Remind whoever runs the reset rather than grinding
  around the defaults. (Region Costs None is a maintenance toggle; regions still
  cost gold to buy.)
- **Build income before spending on the test.** Buying the maximum Coastal for
  2-3 turns compounds Tourism (about 12k → 147k → 403k gold a turn); keep the tax
  low, or Coastal output slumps.
- **Steer support for a sweep**: tax at or under about 60 barely erodes it; tax
  100 crashes it about 40 a turn; tax 0 plus paying the boost prompt recovers
  it, and it returns to 100 overnight. 12 % earns tax without riots.
- **Turns**: 20 a day on the test boards; when they run out, advance the DOS
  `DATE` a day.

Two-board InterBBS runs are in `interbbs.md`.
