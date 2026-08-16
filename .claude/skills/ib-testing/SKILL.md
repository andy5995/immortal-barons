---
name: ib-testing
description: Use when testing Immortal Barons behaviour — verifying a mechanic's numbers, reproducing something seen in play, exercising inter-BBS features, or changing game state outside the menus. Covers which of the three test surfaces to reach for, how to set config without the editor, and the traps that cost hours. Triggers on "test this", "verify the balance", "reproduce what I saw", "set up a league", driving `-local` from a script, or any question answered by running the game rather than reading it.
---

# Testing Immortal Barons

Three surfaces answer different questions, and picking the wrong one is where
the time goes. Read this section before building anything.

| Surface | Answers | Costs |
|---|---|---|
| Throwaway Go harness | "is this number right", "what does this code do to a world" | milliseconds |
| Scripted two-board league | "does this inter-BBS feature move a packet" | a minute |
| Live Mystic boards | "does a real sysop's setup work end to end" | tens of minutes, and Andy's machine |

**Default to the first.** `NewWorldSeed(DefaultConfig(), seed)` in a throwaway
`internal/game/zz_*_test.go` gives a complete world for nothing, and `rmw` the
file afterwards. The recurring failure is not reaching for it: fidelity work
(reading the binary, parsing captures) puts you in a mode where the data source
feels fixed and external, and simulation stops feeling available. "I haven't
verified this" is a trigger to build a sandbox, not a disclaimer to ship.

**Never run an experiment against Andy's live data directories.** His worlds
exist to reproduce what HE saw. A synthetic world answers balance and behaviour
questions, and cannot corrupt anything. Reproducing a specific report is the
only reason to touch a live dir, and then read it, do not mutate it.

## Changing game state without the menus

`config.json` in the data directory is the single source of truth — `store.repair`
overwrites `World.Config` from it on every load, so editing the world's copy
achieves nothing.

Prefer a CLI flag over the Configuration Editor when one exists. The editor is
driven by keystrokes, which means **the change leaves no searchable trace**: a
later session grepping the transcripts for the field name finds nothing and
cannot tell how the state was reached. This is not hypothetical — it happened on
2026-08-16 with Dupe Checking, and is why `-dupe-check` exists.

    immortal-barons -local -dupe-check off     # this run only; `on` to force it on

**A testing override must not reach disk.** `-dupe-check` is a per-invocation
switch, not a setting: it changes behaviour for one run and leaves `config.json`
untouched, so a test session cannot leave a league rule changed behind it.

The obvious implementation is wrong, and the reason generalises. Overriding the
in-memory `game.Config` after `store.LoadConfig` *appears* correct — `Load` and
`FileStore` both take `cfg` as a parameter and `repair` does `w.Config = cfg`,
so the value propagates everywhere. But four call sites write that struct back
out (`config_editor.go`, `config_editor_tui.go`, `store/ibbs.go` applying a
league broadcast, and `main.go`), so opening the Configuration Editor mid-session
would silently persist the override.

**Override at the READ sites instead, behind one accessor**, so a later read site
cannot be added that ignores the switch. Before adding any override of this kind,
grep for every reader of the field and every writer of the struct that holds it —
the writers are the ones that turn a temporary switch into a permanent change.

A flag that genuinely *is* a persisted setting is a different shape: take the
world lock, re-read `config.json` under it, set the field, `store.SaveConfig`,
print what changed, exit. Re-reading under the lock matters because a
Coordinator's `-planetary` run rewrites the same file.

**A league-wide rule does not stay where you put it.** Anything carried in
`LeagueConfig` (`internal/game/ibbs.go`) is overwritten on the member boards by
the Coordinator's next broadcast. So a setting changed on a member board is
temporary by construction, and a setting changed on the Coordinator propagates
whether or not that was the intent. When a test needs a rule to hold, set it on
the Coordinator and re-broadcast, or check afterwards that it survived.

## Driving `-local` from a script

Two traps, both of which fail silently:

- **A line of input ends with `\r`, not `\n`.** The session reads CR; a `\n`
  never terminates the line and the run desyncs with no error.
- **A league board seeds no computer barons.** `AddAIEmpires` refuses while IBBS
  is on, so create a human realm on each side first or a packet arrives with
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
  from the System menu rather than playing turns to burn it.
- **Most interplanetary ops need a turn played this entry.** The menu redraws
  with no message when it has not been.
- **Both leagues need a non-zero `LeagueNumber`.** `ReadInbound` skips a packet
  only when reader and packet numbers are both set and differ, so a league left
  at 0 reads another league's packets and has its own read in turn.

## Fixed seeds

A fixed-seed test may only assert what holds on OTHER seeds. A macro outcome
("nobody is eliminated") is a property of the whole simulation and one seed is
one trajectory. Run several and assert the property, or assert an exact computed
figure — those stay deterministic. `GOARCH=386 go test ./...` when the change
touches money, to catch the 32-bit overflows the 64-bit build hides.

## Machine-specific state is NOT in this file

This directory is tracked in git, so board paths, ports, realm names and league
numbers stay in Andy's memory store, not here. Look for the memories named
`two-board-league-live-setup` (the real Mystic pair and what is proven),
`two-board-league-harness` (the scripted pair, no BBS), and `bre-install-state`.
The reusable scripts live beside them under that project's `scripts/`.

## Keep this skill current

When a testing session teaches you something about *how to test* — a gate that
wasted an hour, a state change that left no trace, a harness worth reusing — add
it here in the same pass, without being asked. Prefer editing an existing
section over appending a near-duplicate. A lesson that exists only in a finished
conversation is lost, which is the exact failure that created this file.
