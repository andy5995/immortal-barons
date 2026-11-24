# Mod system: decisions and invariants

`docs/mods.md` is the user contract: every key, limit, file and command. The
code is how each part works. This page holds what neither states as a rule:
the decisions behind the mod system and the invariants a change must keep. The
design came out of discussion #202 and PR #350. Read this before changing how
mods are read, applied, checked or sent.

## Where a mod's values live

- **The base game is the rules in `Rules`, built from the `balance*.go`
  constants.** A mod changes a copy (`game.BaseRules`), never the constants.
  The binary-verified constants stay the fidelity contract for an unmodded
  game, and `-print-base-mod` shows them.
- **Rules belong to the `World`, not to package variables.** Tests of the mod
  loader change rules for one world without touching any other test running in
  parallel, and one world plays one mod set.
- **Every key is named in one table** (`unitFields`, `hqFields`,
  `game.RegionFields`). Reading, `-check-mod`, `-print-base-mod` and `Diff` all
  go through it, so a key cannot be readable in one place and missing in
  another.
- **Every figure has limits tighter than its type**, chosen so the engine's
  arithmetic stays safe on the 32-bit door builds. A figure times a region or
  unit count is worked in `int64` (or `big.Int`), and a total saturates rather
  than wraps (`fitInt`). Money stays `int64` all the way to `creditGold`, which
  records what its cap discards. A new key needs a limit chosen this way, and a
  `GOARCH=386` test when it multiplies a count.

## When a mod changes

- **Rules change only at a reset.** A game keeps the copy of each mod saved
  when it started, and warns when the folder has changed since.
- **`-reload-mods` is the one exception, and it is for testing.** It puts the
  folder's current values into a game in progress. It is refused on a league
  board.

## Versions

- **`version` is the author's**, for them to change as they like.
- **`mod_api` is the mod format's**, and every mod must give it. It covers the
  folder layout, the `values.toml` keys and what each one means, and what a
  script is given and returns. Raise `mod.ModAPI` for any change that could make
  a mod that worked before fail to load, or load and play differently, and say
  in the ChangeLog what changed. Adding a key or a script field does not raise
  it. The game refuses an older or newer mod by name before play.

## Scripts

- **Starlark, run in the game process, with a step cap and no memory cap.**
  `starlark-go` has no memory limit, so a script is as trusted as the folder it
  came in. A hard limit would need a child process with an OS limit.
- **A region script must answer the same question the same way.** It gets no
  `rand()` and no `day()` builtin, and `realm` carries no gold, because a screen
  asks for a projection and the turn asks again when it pays. What the view does
  carry (`turns_played`, `people`, `support`) changes between turns, so a
  projection holds for the realm as it stands, not for next turn.
- **A script's globals are frozen once its top level has run.** It keeps
  nothing between calls.
- **A failing script falls back to the type's row (regions) or the built-in raid
  (pirates)**, and each different failure is reported once per run, up to
  `ScriptWarnings`.

## Leagues

The League Coordinator chooses the mods; members install nothing.

- **The Coordinator's `-league-reset` order carries every file of every mod**,
  and the Coordinator's signature covers it. A member rebuilds its rules from
  those files with the same reader and limits a stand-alone reset uses
  (`mod.LoadForLeague`, installed as `game.LoadLeagueMods`).
- **A member that cannot play the order's mods does not reset**, and tells its
  sysop why. Its old game is then held apart by the ruleset check until the
  sysop acts. A reset under other rules would feed a different game into
  everyone else's.
- **Scripts run on a member only when its sysop allows it** (`AcceptLeagueScripts`
  in `bbs.cfg`, off by default). The check must come before anything compiles a
  script, since compiling runs its top level.
- **The ruleset fingerprint includes each mod's name and hash**, and every
  packet carries it. With no mods it is the settings' fingerprint unchanged. A
  member stamps the hash of the copy it actually plays, so a copy edited in
  `world.json` is held like any other difference in rules. This catches a
  misconfigured board, not a lying one: a modified binary can state any
  fingerprint.
- **Mod files are not scrubbed** in an arriving packet, because their tabs and
  newlines are part of what the hash covers. Every other field of a mod
  reference is scrubbed like the rest of the packet, and the mod reader refuses
  a control character in each manifest field it shows.
- **What only interplanetary play uses stays constant**: the remote tank curve,
  the share of a HeadQuarters a winning invasion takes, and a Sabotage HQ
  agent's points. Moving them into mods needs every board's resolver to agree.

## Not moddable yet

- **The AI.** It is lowest priority (Andy, 2026-11-24).
- **New units, and the number of pirate factions.** Both are fixed in the save
  format and the packets.
- **Further pirate figures** are #351.
