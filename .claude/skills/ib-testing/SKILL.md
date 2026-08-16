---
name: ib-testing
description: Use when testing Immortal Barons behaviour — verifying a mechanic's numbers, reproducing something seen in play, exercising inter-BBS features, or changing game state outside the menus. Covers which of the three test surfaces to reach for, how to set config without the editor, and the traps that cost hours. Triggers on "test this", "verify the balance", "reproduce what I saw", "set up a league", driving `-local` from a script, or any question answered by running the game rather than reading it.
---

# Testing Immortal Barons

Three surfaces answer different questions, and picking the wrong one is where the
time goes.

| Surface | Answers | Costs |
|---|---|---|
| Throwaway Go harness | "is this number right", "what does this code do to a world" | milliseconds |
| Scripted two-board league | "does this inter-BBS feature move a packet" | a minute |
| Live BBS boards | "does a real sysop's setup work end to end" | tens of minutes, and a configured rig |

**Default to the first.** `NewWorldSeed(DefaultConfig(), seed)` in a throwaway
`internal/game/zz_*_test.go` gives a complete world for nothing; delete the file
afterwards. The recurring failure is not reaching for it: fidelity work (reading
the binary, parsing captures) puts you in a mode where the data source feels
fixed and external, and simulation stops feeling available. "I haven't verified
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

`config.json` in the data directory is the single source of truth —
`store.repair` overwrites `World.Config` from it on every load, so editing the
world's copy achieves nothing.

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

**A scripted key sequence must assert it REACHED the screen it tests.** When the
script runs dry the session ends *cleanly*, so any flow change upstream — a new
prompt, a re-mapped hotkey — leaves the test green while it never reaches the
code it covers. Two tests rotted this way, one for weeks, after a first-run
language picker ate a key and shifted every key after it. Assert a marker unique
to the target screen plus a state effect (`TurnsPlayed` rose, the treaty formed).

## Gates that block a test before it starts

Check these first when a mechanic appears not to work:

- **New realm protection** gates trading and the market on both sides. End it
  from the System menu rather than playing turns to burn it off.
- **Most interplanetary ops need a turn played this entry.** The menu redraws
  with no message when it has not been.
- **Every league needs a non-zero `LeagueNumber`.** `ReadInbound` skips a packet
  only when reader and packet numbers are both set and differ, so a league left
  at 0 reads another league's packets and has its own read in turn.

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

**Mystic cannot test an FTN netmail path at all.** It keeps its own message bases
and has no `*.msg` netmail directory anywhere, so the Binkley/FrontDoor
file-attach convention has nothing to read it. Synchronet can, through SBBSecho.
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

- **Poll by NODE ADDRESS, not by domain.** `mis poll 1:1/2` works; `mis poll
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
  matches on address, which is what makes it confusing. On a rig that never
  touches real FidoNet, take zone 1 off `fidonet` and give it to the private
  domain. Retagging the node instead makes it authenticate as the wrong domain
  and the far side answers `Bad address or password`.

## The FTN handoff chain, end to end

Each step has its own failure mode, so check them in order rather than guessing
where a packet stalled:

    immortal-barons -planetary          # writes .brp into Outbound
    barons-ftn                          # claims it into Outbound/fido, writes N.msg
    $SBBS/exec/sbbsecho                 # packs the .msg into the BSO .flo, deletes it
    $SBBS/exec/jsexec -c ctrl exec/binkit.js   # OUTBOUND session, actually sends

**BinkIT does not push its queue to a caller.** An inbound session
authenticates, transfers nothing, and looks like success on both sides. Only an
outbound poll sends what the BSO holds — which is why the last step is a `jsexec`
run rather than "wait for the other board to poll".

**The chain is safe to run with callers online.** `barons-ftn` takes its own
`barons-ftn.lock` rather than the game lock, so it never blocks a node mid-turn.
It reads only complete packets, because `WriteOutbox` publishes each one under a
temporary name and renames it into place.

The `.msg` numbering restarts from 1 each run. SBBSecho deletes the message once
it is packed (kill-sent), so seeing `1.msg` again is the previous one having been
carried, not the same one stuck.

**Keep the outbound path short.** The Type-2 subject holds 70 bytes in Binkley
mode for the WHOLE absolute attachment path, `fido/` child and filename included.
`<data dir>/outbound` is already too deep on a normal Unix layout; a short path
near the BBS root is not fussiness, it is the only thing that fits. The preflight
refuses the run and moves nothing, so this is loud rather than subtle.

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
