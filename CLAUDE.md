# Immortal Barons — project guide

Immortal Barons is a from-scratch Go clone of the 1990s BBS door game
*Barren Realms Elite* (BRE). It is an independent reimplementation of the
game's rules and mechanics — no original code, text, or art is used. Do not
copy verbatim display text/art from the original into this repo, and never
put John Dailey's (or any third party's) private contact info into any
artifact here.

## Build, test, run

```
go build ./...
go test ./...
go run ./cmd/barons        # play locally in your terminal
gofmt -w .                 # always run before committing
```

Go 1.26. No external dependencies (stdlib only) — keep it that way unless
there's a strong reason.

## Architecture

The game logic never knows how it is shown to the player. It only reads
keypresses from, and writes ANSI bytes to, a `session.Session` (a byte
stream). Front-ends attach different streams; the engine is unchanged.

- `cmd/barons` — local terminal front-end (a person plays in their console)
- `internal/session` — the `Session` byte-stream abstraction + a console
  implementation + shared `ReadLine`
- `internal/ansi` — ANSI escape helpers (one rendering path for all front-ends)
- `internal/menu` — the generic menu engine (`menu.go`) plus the BRE menu
  tree and actions (`tree.go`, `actions.go`, `input.go`)
- `internal/game` — the world: empires, economy, turn engine, combat

`menu.go` is the framework; `tree.go` is content. That split is the seam
that lets the menu tree grow without touching the engine.

## Conventions

- Run `gofmt -w .` before every commit; keep `go vet` clean.
- Tests use a scripted fake `Session` (see `internal/menu/menu_test.go`) and
  a fixed RNG seed via `game.NewSeed` for determinism.
- Keep combat/economy numbers matching `docs/mechanics-reference.md`; when
  they diverge, update the doc in the same change.

## Mechanics fidelity

`docs/mechanics-reference.md` is the authoritative spec, built from the
original binary strings plus public strategy guides and the GameBanshee
manual overview. It covers units (offense/defense values, maintenance,
net-worth), covert ops, region types, economy, caps, and the new-player
start flow. Design specs live in `docs/superpowers/specs/`.

Combat uses a split offense/defense model (trooper 1/1, jet 2/0, turret
0/2, tank 4/4); jets need carriers to attack. Bank interest is ~1%/turn
with the 1.599-billion interest cap and 2-billion money cap.

## Status (v0.0.1)

Playable single-player game vs. three AI empires: economy, banking, combat,
turn engine, scoring, win/lose, startup realm naming. Stubbed for later:
nuclear/chemical/biological/Gooie Kablooie attacks, covert ops, diplomacy,
trading, messages, region types, and the land/food markets with moving
prices (currently land is a flat-priced single resource, which makes buying
land outperform fighting — a known gap pending the market slice).

## Primary goal: run as a BBS door

The main goal is to run as a native door game under modern BBS software
(Synchronet, Mystic) on Linux. Native means no DOSBox/DOSEMU. The stage
decomposes into:

1. **Dropfile + stdio front-end** (`cmd/barons-door`) — DONE. Parses
   `DOOR32.SYS`/`DOOR.SYS` (`internal/door`), runs the game over a stdio
   `Session` (`session.Stdio`, which adds `\r\n`), honors the ANSI flag and
   a hard time-left cutoff, and names the realm from the caller's handle.
   Socket I/O (Synchronet `COM0:SOCKETn` / Mystic telnet) is parsed but not
   yet used as a backend — stdio covers native Unix doors.
2. **Persistence / multi-user** — a persistent empire per caller in a shared
   world, keyed by BBS identity + node, with turns-per-day and daily
   maintenance. This is the heart of a door game.
3. **Sysop config** — game-rules settings file and dropfile path/type.

Dropfile field maps and the I/O contract are documented in
`docs/mechanics-reference.md` and cross-checked against the Synchronet
source (`~/src/sbbs/src/xpdoor/dropfiles.c`).

## Commits

One-line subject by default; body only for a non-obvious *why*. End with the
`Co-Authored-By: Claude ...` trailer. Do not add any `claude.ai` session
trailer. Commit only when asked.
