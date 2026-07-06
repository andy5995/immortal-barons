# Immortal Barons — project guide

Immortal Barons is a from-scratch Go clone of the 1990s BBS door game
*Barren Realms Elite* (BRE). It is an independent reimplementation of the
game's rules and mechanics — no original code, text, or art is used. Do not
copy verbatim display text/art from the original into this repo, and never
put John Dailey's (or any third party's) private contact info into any
artifact here.

Be careful with other people's names and handles in repo artifacts (code,
comments, docs, commit messages, ChangeLog). The line is public vs. private:

- **Fine to name** — an actual project contributor; or a person credited for a
  **public document, project, or repository they have published** that we cite
  or build on (proper attribution of public work). The original authors in the
  Heritage section (Mehul Patel, John Dailey) are named for creating BRE.
- **Do NOT name** — usernames/handles from **private or community channels**
  (forum/Discord/Slack handles, private tips, relayed disassembly notes). Credit
  the *source* generically instead: "a disassembly of the original binary",
  "reconstructed from play data", "community strategy guides".

When unsure whether a source is public enough to credit by name, keep it
generic.

## Build, test, run

```
go build ./...
go test ./...
go run ./cmd/barons        # play locally in your terminal
gofmt -w .                 # always run before committing
```

Go 1.26. Prefer the standard library, but a dependency is fine when it
clearly earns its place (current deps: `golang.org/x/term`, `golang.org/x/sys`).
Keep the set small and justified. Commit `go.mod`/`go.sum`; do NOT commit
`vendor/` (distros build against their own packaged deps or fetch at build); a
release tarball may `go mod vendor` for offline builds. i18n uses gettext/PO:
**po4a** for the help docs and a small **in-house PO reader** (`internal/i18n`,
no runtime dependency) for UI strings — see
`docs/superpowers/specs/2026-07-03-docs-help-localization-design.md`.

## Architecture

The game logic never knows how it is shown to the player. It only reads
keypresses from, and writes ANSI bytes to, a `session.Session` (a byte
stream). Front-ends attach different streams; the engine is unchanged.

- `cmd/barons` — local terminal front-end (a person plays in their console)
- `cmd/barons-door` — native BBS door front-end (stdio + dropfile; `-maint`,
  `-planetary`, `-league-config`, `-setup`, …)
- `cmd/barons-web` — experimental browser front-end (SSE + xterm.js)
- `internal/session` — the `Session` byte-stream abstraction + console/stdio/web
  implementations, shared `ReadLine`, and the Ctrl-key macro expander
- `internal/ansi` — ANSI escape helpers (one rendering path for all front-ends)
- `internal/menu` — the generic menu engine (`menu.go`) plus the BRE menu
  tree and actions (`tree.go`, `actions.go`, `input.go`)
- `internal/game` — the world: empires, economy, turn engine, combat, news
- `internal/store` — JSON persistence under an exclusive flock, config file,
  inter-BBS packet I/O, league roster
- `internal/play` — session bootstrap (load world → onboard/find empire → run)
- `internal/door` — dropfile parsing (`DOOR32.SYS`/`DOOR.SYS`)
- `internal/help` — embedded categorized Markdown help + Markdown→ANSI renderer,
  per-language content (`content/`, `content.de/`, `content.ru/`)
- `internal/i18n` — dependency-free gettext-PO reader for UI strings

`menu.go` is the framework; `tree.go` is content. That split is the seam
that lets the menu tree grow without touching the engine. Language is threaded
to output helpers via a per-session `langSession` wrapper set in `menu.Run`, so
`ok`/`fail`/`prompt`/`tr` translate by the caller's `Empire.Language`.

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
net-worth), covert ops, region types, economy, caps, the new-player start
flow, combat percentages, and the news feeds. Design specs live in
`docs/superpowers/specs/`. Values cross-referenced against the actual BRE
files (`~/.dosemu/drive_c/games/bre-dos/`) are noted there and in the
`bre-binary-verified-math` memory; a disassembly of the original binary is
authoritative for exact constants.

Combat uses a split offense/defense model (trooper 1/1, jet 2/0, turret
0/2, tank 4/4); jets need carriers to attack. A regular attack captures 20%
of the loser's regions and costs both sides 15% losses (per `attack.hlp`).
Bank interest is ~1%/turn with the 1.599-billion interest cap and 2-billion
money cap. Net-worth and maintenance use BRE's per-unit tables (net worth in
thousandths for exactness; bombers now carry upkeep). Pirate gold cap is
600,000 (BRE.EXE caps table).

## Status (v0.0.1)

Persistent, multi-user door game. One shared JSON world guarded by an
exclusive flock; per-caller empires keyed by BBS handle; per-turn economy
(idle empires stagnate) split from a daily maintenance step; turns-per-day
and new-realm protection; an event log for asynchronous play. Front-ends:
`cmd/barons` (local), `cmd/barons-door` (native BBS door), and `cmd/barons-web`
(experimental browser). Persistence design/plan:
`docs/superpowers/{specs,plans}/2026-07-03-door-*`.

Implemented gameplay: conventional combat (offense/defense, turrets, carriers,
jets, bomber airfield strikes), nuclear/chemical/biological strikes, pirate
raids, covert agents (spy + sabotage), diplomacy treaties, trading, region
types + food market, SDI, Gooie Kablooie, player mail + a BRE-style multi-line
message editor + planetary bulletin, banking (deposit/withdraw/loan/invest),
Set Industries + Specialize, Write Macros, and a rising land-market price
(expansion self-limiting). A **planetary news feed** broadcasts battles, WMD
strikes, pirate raids, and riots (original wording). The **sysop Configuration
Editor** edits the full BRE field set and those knobs are wired into gameplay
(tax/region caps, maintenance/attack Level presets, interest, Buy Military,
etc.); a league Coordinator broadcasts the whole ruleset over inter-BBS.

**Inter-BBS ("Option A")**: file-drop `.brp` JSON packets in Inbound/Outbound
dirs; the sysop's transport moves them; `-planetary` processes inbound, launches
group attacks, and exports scores/news. **Localization**: help docs (po4a) and
UI strings (`internal/i18n`) render in the caller's language; de/ru are seeded
and grow via the `.po` catalogs.

Key gameplay knobs (unit values, maintenance, prices, `LandPriceStep`) are
constants — tune freely; keep them matching `docs/mechanics-reference.md`.

Stubbed / not built: leagues auto-reset + Planetary Master crowning, the
Quick-Strike / Extended-Battle attack variants, civil-war collapse, and BRE's
finer interplanetary news subtypes. Some covert-menu items (Spy on Relations,
Spy Database, Bribery) route through the special-ops path pending fuller
diplomacy/database subsystems. Known scale gap: region income is ~100× smaller
than BRE's documented figures (an open scale decision).

## Primary goal: run as a BBS door

The main goal is to run as a native door game under modern BBS software
(Synchronet, Mystic). Native means no DOSBox/DOSEMU. Linux is the primary
target, but the game builds and runs on macOS, Windows, and the BSDs (per-OS
file lock, `x/term` console) — so a Windows Synchronet door is in scope too.
The stage decomposes into:

1. **Dropfile + stdio front-end** (`cmd/barons-door`) — DONE. Parses
   `DOOR32.SYS`/`DOOR.SYS` (`internal/door`), runs the game over a stdio
   `Session` (`session.Stdio`, which adds `\r\n`), honors the ANSI flag and
   a hard time-left cutoff, and names the realm from the caller's handle.
   Socket I/O (the `DOOR32.SYS` line-2 handle — a winsock handle on Windows,
   a plain fd socket on *nix) is parsed but not yet used as a backend. stdio
   covers native Unix doors today; a socket-handle backend (planned) is what a
   Windows door needs, where stdio redirection of the socket isn't the norm.
2. **Persistence / multi-user** — DONE. A persistent empire per caller in a
   shared JSON world under an exclusive flock, keyed by BBS handle, with
   turns-per-day and daily maintenance (`internal/store`, `internal/play`).
3. **Sysop config** — DONE. `config.json` with defaults + an in-game
   Configuration Editor (Coordinator menu); `-setup` seeds the file. The
   knobs are wired into gameplay and broadcast across a league.

Remaining toward the goal: socket-backed I/O (the DOOR32.SYS socket handle —
Synchronet `COM0:SOCKETn` / Mystic telnet / Windows winsock; Go's `net` stack
should make it one cross-platform backend) and validation under real BBS
software (needs Andy's env). Scheduled after the multiplayer web server lands.

Dropfile field maps and the I/O contract are documented in
`docs/mechanics-reference.md` and cross-checked against the Synchronet
source (`~/src/sbbs/src/xpdoor/dropfiles.c`).

## Commits

One-line subject by default; body only for a non-obvious *why*. End with the
`Co-Authored-By: Claude ...` trailer. Do not add any `claude.ai` session
trailer.
