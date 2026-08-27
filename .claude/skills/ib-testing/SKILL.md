---
name: ib-testing
description: Use when testing Immortal Barons behaviour — verifying a mechanic's numbers, reproducing something seen in play, exercising inter-BBS features, or changing game state outside the menus. Covers which of the three test surfaces to reach for, how to set config without the editor, and the traps that cost hours. Triggers on "test this", "verify the balance", "reproduce what I saw", "set up a league", driving `-local` from a script, or any question answered by running the game rather than reading it.
---

# Testing Immortal Barons

**`-data` defaults to `./data`, and the repo root holds a real `data/world.json`.**
So `immortal-barons -local` run from the checkout plays a live world, not a
scratch one. Pass `-data` explicitly at every invocation, pointed somewhere
under `/tmp`.

**There is no seed control from the command line.** `store.Load` builds worlds
through `game.NewWorld` (time-seeded) and the RNG is unexported and never
serialized, so two `-reset-from-config` runs differ. Anything that must be
reproducible is a Go harness with `NewWorldSeed`, optionally unmarshalling a
saved `world.json` on top — do not burn time hunting for a `-seed` flag.

Three surfaces answer different questions, and picking the wrong one is where the
time goes.

| Surface | Answers | Costs |
|---|---|---|
| `internal/game` harness | "is this number right", "what does the engine do to a world" | milliseconds |
| `internal/menu` scripted session | "what does a PLAYER get" — every gate, prompt and refusal | milliseconds |
| Scripted two-board league | "does this inter-BBS feature move a packet" | a minute |
| Live BBS boards | "does a real sysop's setup work end to end" | tens of minutes, and a configured rig |

**Pick between the first two by where the rule lives, and the engine enforces
very little.** New-realm protection is the worked example: `World.Attack` has no
protection check at all and will happily fight a protected defender. The gate is
`game.Targets` plus `menu.blockedByProtection`
(`internal/menu/actions_attack.go:175`). A `game` harness therefore answers
"can a protected realm be attacked?" with a confident **yes**, which is wrong for
every question a player or sysop is actually asking. Ask "would a player ever
reach this?" — if yes, script the menu.

**Two facts the engine harness needs, both of which cost an hour when missed:**

- **`DefaultConfig().AICount` is 0** (`internal/game/config.go:429`), so
  `NewWorldSeed(DefaultConfig(), seed)` builds a world with **no empires**. Set
  `cfg.AICount` before the call, or add empires after.
- **`PlayTurn` does not collect income.** `CollectIncome`, `Manufacture` and
  `GrowFood` are separate turn-start calls the caller makes
  (`internal/game/turn.go`, `internal/menu/gameflow.go:57-59`). A naive
  `PlayTurn` loop leaves gold unchanged every day and reads as a broken
  economy. Drive days with `DailyMaintenance` instead.

Delete the throwaway file afterwards.

The recurring failure is not reaching for a harness at all: fidelity work
(reading the binary, parsing captures) puts you in a mode where the data source
feels fixed and external, and simulation stops feeling available. "I haven't verified
this" is a trigger to build a sandbox, not a disclaimer to ship.

**A question about game behaviour belongs in a throwaway world even when a live
rig is sitting there** — a fresh world is faster, isolated and repeatable, and it
cannot leave a test's leftovers behind to confuse the next session.

## Fixed seeds

A fixed-seed test may only assert what holds on OTHER seeds. A macro outcome
("nobody is eliminated") is a property of the whole simulation, and one seed is
one trajectory. Run several and assert the property, or assert an exact computed
figure — those stay deterministic. Run `GOARCH=386 go test ./...` when the change
touches money, to catch the 32-bit overflows the 64-bit build hides.

## Changing game state without the menus

`config.json` holds the game rules, and `store.repair` overwrites
`World.Config` from it on every load (`internal/store/store.go:55-58`), so
editing the world's copy achieves nothing.

**It is not the only file, and it does not win.** Per-board settings —
`BoardID`, `LeagueNumber`, `Inbound`, `Outbound`, `Link` — live in `bbs.cfg`,
and `LoadBoardConfig` runs *after* the JSON (`internal/store/config.go:70`), so
bbs.cfg overrides it. Editing `LeagueNumber` in `config.json` on a rig that has
a `bbs.cfg` silently does nothing. Dropfile settings are in `door.json` and
`MinBoardVersion` has its own file.

The flags that exist for testing, none of which need the editor:

| flag | does |
|---|---|
| `-dump` | print the normalized world as JSON after load-time migration — the inspection tool for "reproduce what I saw" |
| `-spectate N` | play N days of computer-baron turns and print standings, the built-in balance probe. **Advances and saves**, so never point it at a rig you care about |
| `-add-ai N` | add N computer barons to a running game (refused under IBBS) |
| `-reset-from-config` | rebuild the world from the current `config.json`, no editor |
| `-league-check` | report roster, board name, packet directories and keys at once — run this before blaming a league test |
| `-league-routes` | print which board each planet's packets are handed to |
| `-full`, `-detailed` | one full inbound/turn/outbound cycle, and per-packet tracing |
| `-dupe-check on\|off` | force the league lockout for one run; never written to `config.json` |

Prefer a CLI flag over the Configuration Editor when one exists. The editor is
driven by keystrokes, which means **the change leaves no searchable trace**: a
later session cannot tell how the state was reached, because the field name was
never typed anywhere. That is why `-dupe-check` exists.

    immortal-barons -local -dupe-check off     # this run only; `on` to force it on

**A testing override must not reach disk.** `-dupe-check` is a per-invocation
switch, not a setting: it changes behaviour for one run and leaves `config.json`
untouched, so a test session cannot leave a league rule changed behind it.

The obvious implementation is wrong, and the reason generalises. Overriding the
in-memory `game.Config` after `store.LoadConfig` *appears* correct — `Load` and
`FileStore` both take `cfg` as a parameter and `repair` does `w.Config = cfg`, so
the value propagates everywhere. But four call sites write that struct back out
(`config_editor.go`, `config_editor_tui.go`, `store/ibbs.go` applying a league
broadcast, and `main.go`), so opening the Configuration Editor mid-session would
silently persist the override.

**Override at the READ sites instead, behind one accessor**, so a later read site
cannot be added that ignores the switch. Before adding any override of this kind,
grep for every reader of the field AND every writer of the struct that holds it —
the writers are what turn a temporary switch into a permanent change.

A flag that genuinely *is* a persisted setting is a different shape: take the
world lock, re-read `config.json` under it, set the field, `store.SaveConfig`,
print what changed, exit. Re-reading under the lock matters because a
Coordinator's `-planetary` run rewrites the same file.

**A league-wide rule does not stay where you put it.** Anything carried in
`LeagueConfig` (`internal/game/ibbs.go`) is overwritten on member boards by the
Coordinator's next broadcast. A setting changed on a member board is temporary by
construction; one changed on the Coordinator propagates whether or not that was
the intent. When a test needs a rule to hold, set it on the Coordinator and
re-broadcast, then check it survived.

## Driving `-local` from a script

Two traps, both of which fail silently:

- **A line of input ends with `\r`, not `\n`.** The session reads CR; a `\n`
  never terminates the line and the run desyncs with no error.
- **A league board seeds no computer barons.** `AddAIEmpires` refuses while IBBS
  is on, so create a human realm on each side first, or a packet arrives with
  nobody to receive it.

**The language picker is still there**, and it is the third trap. A first run
needs a prelude before any game key:

    ' 1\r<RealmName>\ry\r'

— space to clear the splash pause, `1\r` to pick English, the realm name, then
`y\r` to confirm it. Omit it and the realm gets named from your first keystroke
and every key after that lands one screen early.

**A scripted key sequence must assert it REACHED the screen it tests.** When the
script runs dry the session ends *cleanly*, so any flow change upstream — a new
prompt, a re-mapped hotkey — leaves the test green while it never reaches the
code it covers. Two tests rotted this way, one for weeks, after that same
language picker ate a key. Assert a marker unique to the target screen plus a
state effect (`TurnsPlayed` rose, the treaty formed).

## A menu-level test, in full

The form the `internal/menu` tests use — this is what to copy when the answer
lives behind a gate rather than in the engine:

```go
menus := BuildMenus()
f, w, err := run(t, "g  0", menus.Game)   // helper in menu_test.go
// then assert on f.out.String() and on w
```

`run` builds the fake session and world and calls `Run(f, w, root)` for you.
The roots are **fields on `BuildMenus()`** — `.Game`, `.Attack`, `.System` —
not paths walked from `Game`, which is why grepping the menu tree for them
finds nothing. Hotkey dispatch is case-folded, so a lowercase letter is the
honest key to script.

## Gates that block a test before it starts

Check these first when a mechanic appears not to work:

- **New realm protection** gates trading and the market on both sides. End it
  from the System menu rather than playing turns to burn it off.
- **An interplanetary or WMD item that is simply absent is a config switch**,
  not a bug: `BombingOps`, `MissileOps`, `IPTrading`, `GooieKablooie` and
  `InterBBSEnabled` hide their items with no message
  (`internal/menu/tree.go:37-44,544`).
- **BRE gates most InterPlanetary options behind a turn played this entry, and
  IB does not — that is a known fidelity gap, not a settled difference.**
  BRE's `enforce_interbbs_turn_requirement` (`BRE.OVR 0x020a12`) prints "You
  must play at least one turn per entry in the game to access this option." and
  is called from **four** sites in `run_interbbs_menu` (`0x020caf`). Banking
  and IP messages are reachable at the entry menu without a turn. IB has no
  such check anywhere in `internal/game` or `internal/menu`. So when a test
  reaches an IP op without playing a turn, **that is the bug, not the
  baseline** — do not write the permissive behaviour into an expectation.
  Issue #162.
- **A `LeagueNumber` of 0 fails `-league-check` on a league board, and
  `barons-ftn` refuses to run** (#227). `ReadInbound` skips a packet just when
  reader and packet numbers are both set and differ, so a board left at 0 takes
  every league's packets as its own — which is why a test rig needs a real
  number on every board, not just the ones sharing an inbound directory.

---

# Running a local BBS rig

Everything below concerns a rig of real BBS software on one machine. It is the
slowest surface and the only one that answers "does a sysop's setup work".

**Paths here are written as placeholders.** Substitute your own:

| Placeholder | What it is |
|---|---|
| `$MYSTIC` | a Mystic BBS install |
| `$SBBS` | a Synchronet install |
| `$SBBSSRC` | a Synchronet source clone used only for building |

## Choosing board software by what the transport needs

**Mystic is reported not to test an FTN netmail path at all** — keeping its own
message bases with no `*.msg` netmail directory for the file-attach convention
to write into, where Synchronet has one through SBBSecho. Unverified against
Mystic's own source or docs; confirm before concluding a handoff is impossible
rather than misconfigured.
Check for the directory the transport actually watches before concluding a
handoff is broken.

A useful pairing is one board of each: the Mystic side exercises the plain
file-box path, the Synchronet side the FTN path, over one binkp link.

## Building Synchronet

- **Build from a clone, not from a reference checkout.** The install makefile
  builds in `REPODIR`, so pointing it at a tree you also grep for Synchronet
  source scatters `gcc.linux.x64.*` output through it. Clone to `$SBBSSRC` and
  install to `$SBBS`.
- **Unset `MAKEFLAGS` first** if your environment exports one. Parallel make
  breaks cryptlib's 3rdp extraction: the same archive is unzipped several times
  at once, directories vanish mid-unpack, and the build dies with
  `cryptlib.h: No such file or directory` — which reads like a missing dependency
  and is not one. `install-sbbs.mk` clears `MAKEFLAGS` for its sub-makes but
  cannot clear its own. Unset `CC`/`CXX` too if they carry a wrapper such as
  `ccache`:

      env -u CC -u CXX -u MAKEFLAGS make -f $SBBSSRC/install/install-sbbs.mk \
          SBBSDIR=$SBBS REPODIR=$SBBSSRC NOCAP=1 SYMLINK=1

- **`SBBSCTRL` decides where every binary looks**, defaulting to `/sbbs/ctrl`.
  Export it from your shell profile for interactive use. Profiles are not sourced
  for NON-interactive shells, so scripts, `cron` jobs, systemd units and tool-run
  commands must still set it themselves: `SBBSCTRL=$SBBS/ctrl <command>`.
- `SYMLINK=1` points `$SBBS/exec/*` back into the clone, so **the clone must not
  be cleaned away** afterwards.

## Configuring Synchronet

- **Nothing may bind below 1024** after a `NOCAP=1` build. Move the terminal
  ports (`sbbs.ini`: Telnet, SSH, RLogin), turn off the Mail/FTP/Web servers, and
  disable every `services.ini` entry with a privileged port — NNTP 119, Finger
  79, Gopher 70, MSP 18, ActiveUser 11. NNTP retrying port 119 five times is what
  delays the whole Services startup. `[Services]` itself must stay enabled:
  BinkIT lives there.
- **`scfg` and `echocfg` render under tmux with `-iA -k`** (ANSI mode, no mouse)
  and can be driven with `send-keys` + `capture-pane`, exactly like the BRE
  harness in the `bre-gather` skill. Without `-iA` the banner draws and nothing
  else. To confirm which row the cursor is on before typing into it,
  `capture-pane -pe | cat -v` and look for the `^[[47m` background — guessing the
  row count is how the wrong field gets set. `INS` adds a list entry; ESC backs
  out and offers to save.
- **The system's FTN address is not in any file until `scfg` writes it.** It
  lands in `msgs.ini` as `[FidoNet] addr_list=`. Linked nodes are `echocfg`, and
  land in `sbbsecho.ini` as `[node:ADDR@domain]` with `BinkpHost`, `BinkpPort`
  and `BinkpPoll`.

## Linking two boards over binkp

- **Poll by NODE ADDRESS, not by domain.** `mis poll 99:1/2` works; `mis poll
  <domain>` reports "Polled 0 systems" and looks like a connection failure when
  nothing was ever attempted.
- **Mystic offers plain-text binkp auth; Synchronet demands CRAM-MD5 by
  default.** The symptom is `Authorization failed` on the Mystic side and
  `CRAM-MD5 required (and not provided)` on the Synchronet side. Setting
  `BinkpAllowPlainAuth = true` on that node fixes it — acceptable on a loopback
  test rig, and not something to carry into advice for a real board.
- **A private FTN domain needs its own zone, or BinkIT ignores the host you
  configured.** Outbound, BinkIT maps zone to domain through the `[domain:*]`
  sections, so with the stock `[domain:fidonet] Zones = 1,…` an address like
  `1:1/1@mynet` resolves as `…@fidonet`, misses the `[node:1:1/1@mynet]` section
  entirely, and falls back to `f1.n1.z1.binkp.net` — a DNS failure that looks
  nothing like a config problem. Inbound keeps working throughout, because that
  matches on address, which is what makes it confusing. Retagging the node
  instead makes it authenticate as the wrong domain and the far side answers
  `Bad address or password`.
- **Give the private net a zone nobody else claims, rather than taking zone 1
  off `fidonet`.** Both make BinkIT resolve the domain correctly, and the rig
  here ran on the second one until 2026-08-21. It is the worse of the two: the
  moment that board carries a real FidoNet feed its zone 1 is gone, and a
  `DNSSuffix` left empty resolves to the literal `example.com`, which
  Synchronet substitutes as the default (`exec/load/fido_syscfg.js`). Stock
  `sbbsecho.ini` claims 1–6, 8–11, 18, 21, 24 and a long tail of higher
  numbers; 99 is free and is what the rig and `docs/inter-bbs.md` now use.

## The FTN handoff chain, end to end

Each step has its own failure mode, so check them in order rather than guessing
where a packet stalled:

    immortal-barons -planetary          # writes .brp into Outbound
    barons-ftn                          # claims it (Outbound/fido, or AttachDir), writes N.msg
    $SBBS/exec/sbbsecho                 # packs the .msg into the BSO .flo, deletes it
    $SBBS/exec/jsexec -c ctrl exec/binkit.js   # OUTBOUND session, actually sends

**Run the whole chain; do not wait to be polled.** binkp is bidirectional and
an inbound session *does* hand over what the BSO holds
(`~/src/sbbs/exec/binkit.js:1008`), so the reason is not that BinkIT withholds
the queue. It is that the first three steps have to run before there is
anything in the BSO to collect: until `barons-ftn` and `sbbsecho` have run, a
poll from the other board finds an empty outbound and both logs look healthy.
See the `ftn` skill's Myths table.

**The chain is safe to run with callers online.** `barons-ftn` takes its own
`barons-ftn.lock` rather than the game lock, so it never blocks a node mid-turn.
It reads only complete packets, because `WriteOutbox` publishes each one under a
temporary name and renames it into place.

The `.msg` numbering restarts from 1 each run. SBBSecho deletes the message once
it is packed (kill-sent), so seeing `1.msg` again is the previous one having been
carried, not the same one stuck.

**On Synchronet, keep the outbound path short.** The Type-2 subject holds 71
bytes for the whole attachment path, 70 with Binkley's `^`. `ftn.cfg` can spend
fewer — `SubjectPath Basename` writes the filename alone, a prefix is resolved
against the mailer's working directory (`internal/ftn/config.go:104-118`) — but
**SBBSecho does not search for a bare name**, so a Synchronet board keeps
`Absolute` and needs a short data directory (`docs/inter-bbs.md`). `AttachDir`
moves the file itself off the `fido/` child. The preflight refuses the run and
moves nothing, so this is loud rather than subtle.

**The packet name does not grow** — `packetFilename` zero-pads the sequence to a
fixed width (`internal/store/ibbs.go`), so a path that fits keeps fitting. Issue
#156 removed the opposite claim from the code and the docs; this file had it
too. What does consume a thin margin is a longer directory or a board joining on
a longer node number.

`barons-ftn` warns below 8 bytes spare (`internal/ftn/message.go:24`,
`handler.go:75-78`). Treat that warning as real headroom advice, and read a hard
failure — `attachment subject %q is %d bytes; FTN Type-2 permits at most …`
(`message.go:95-96`), which names the `SubjectPath` fix — as a path or config
change, not as drift.

## Scheduling the exchange

Nothing in the game moves a file between boards and no mailer knows about the
game, so a league only advances when something runs both halves on a schedule. A
timer per board, every 15 minutes, is enough for a test rig.

**EVERY board needs its own timer, including the one that gets polled.** This is
the same "BinkIT does not push its queue to a caller" fact seen from the
scheduling side, and it is easy to get wrong because the league does not look
dead: packets keep arriving at the polling board, its journal keeps reporting
`Applied N packets`, and only the far side's outbound quietly grows without
bound. Confirm both directions by looking at BOTH boards' inbound, never one.

**`-planetary` does not run daily maintenance, and some mechanics are keyed to
the GAME day rather than the wall clock.** Travel-time probes are the case that
bites: `PingTravelTimes` fires once per `LastMaintDate`, maintenance advances
that at most one day per REAL day (`internal/game/turn.go`), and nothing in an
exchange script runs `-maint`. A rig with no callers therefore exchanges packets
every fifteen minutes for days while queueing no probes at all — the boards here
had gone 7 and 12 days between pings with all three timers healthy. Before
concluding an inter-BBS mechanic is broken, check `LastMaintDate` against
today's date; if it is behind, run `-maint` on each board and try again. One
`-maint` buys one game day, so a board days behind needs one run per day to
catch up and cannot be fast-forwarded in a single pass.

**A board's timer must be retired with the board.** A timer left enabled for a
retired board keeps running `-planetary` on its data and keeps polling, so a
dead board goes on writing packets into a live one — which reads as ordinary
league traffic in the journal. Check the enabled timers against the boards that
actually exist whenever the rig changes shape.

The unit is a `oneshot` service with a templated instance per board, so one
script and one timer file cover the whole rig. Give the timer `Persistent=true`
(a workstation is not up at every scheduled time) and a `RandomizedDelaySec`, so
two boards polling each other do not open binkp sessions at the same instant.

Shape of the per-board script, which has to branch because the two board
softwares agree on nothing:

    for each game data dir on this board:
        immortal-barons -planetary -data <dir>
        barons-ftn -data <dir>            # only where the dir has an ftn.cfg
    then ONE mailer run for the whole board:
        Mystic:      cd $MYSTIC && ./mis poll <peer node address>
        Synchronet:  cd $SBBS && ./exec/sbbsecho && ./exec/jsexec -c ctrl exec/binkit.js

One mailer run per board rather than per game: several leagues share the link,
and running it last means a packet written this cycle leaves this cycle.

- **The data directory is not in the same place on both.** Mystic's is under the
  BBS `data/`; a Synchronet door's lives under `xtrn/<door>/data`. Do not derive
  it from the BBS root with one rule.
- **`SBBSCTRL` must be set inside the script.** systemd runs no profile, so
  `sbbsecho` and `jsexec` otherwise look in `/sbbs/ctrl` and do nothing useful.
- **Skip a missing data directory instead of failing.** Adding or retiring a
  league then needs no edit, and a half-set-up board does not spam the journal.
- `binkit.js` printing `We got an M_EOB, but there are still N files pending
  M_GOT` is normal on a rig like this — check the BSO outbound is empty and the
  file landed on the far side rather than reading it as a failure.

## Working on someone's rig without breaking it

A test rig is not production — nobody dials it, and experimenting on it is what
it is for, so do not stall to ask before playing a turn or running a step. What
it *is* is usually a working setup that took real effort to reach: two boards,
binkp both ways, keys, a roster. The care it needs is about not silently breaking
the parts that already work.

- **Check what a step does to the transport before running it.** If a board's
  `Outbound` directory IS its mailer's file box, anything that moves packets
  elsewhere — into a `fido/` subdirectory, say — stops delivery with no error.
  The league looks alive and quietly carries nothing.
- **Deliver before you tidy.** Files waiting in an outbound directory are usually
  pending mail, not litter. Poll first, then clean up what remains.
- **Say what you changed**, so a later session does not spend an hour on a
  symptom that was a leftover from a test.

## Machine-specific state is not in this file

This directory is tracked in git, so board paths, ports, FTN addresses, realm
names and league numbers do not belong here — they differ per machine and would
be wrong for everyone but their author. Keep them wherever you keep host notes.

What a rig needs recorded somewhere, for whoever sets one up: each board's
software and install path, its planet name and node number, its FTN address and
binkp port, its game data directory, its inbound and outbound directories, the
league number, and which board holds the Coordinator key.

## Keep this skill current

When a testing session teaches you something about *how to test* — a gate that
wasted an hour, a state change that left no trace, a harness worth reusing — add
it here in the same pass, without being asked. Prefer editing an existing section
over appending a near-duplicate. A lesson that exists only in a finished
conversation is lost, which is the exact failure that created this file.
