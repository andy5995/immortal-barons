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
go run ./cmd/immortal-barons -local   # play locally in your terminal
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

- `cmd/immortal-barons` — the door + local terminal front-end (stdio + dropfile;
  `-local`, `-maint`, `-planetary`, `-league-config`, `-reset`, …)
- `cmd/immortal-barons-web` — experimental browser front-end (SSE + xterm.js)
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
- **Separate data from code: every tunable gameplay/economy number is a named
  constant in a dedicated data file, not a bare literal in the formula code.**
  Today that file is `internal/game/balance.go`; as the set grows it's fine to
  split into more focused files (e.g. a `food.go`/`combat_balance.go`) — the
  rule is the separation, not the single filename. This covers unit costs,
  prices, rates, caps, starting-setup values, score weights, food/industry
  figures, penalties — all of it. When you add a mechanic, put its numbers in
  the data file (with a one-line provenance comment) and reference them by
  name. Only structural literals (`0`, `1`, `100` for percent math) stay inline.
- Tests use a scripted fake `Session` (see `internal/menu/menu_test.go`) and
  a fixed RNG seed via `game.NewSeed` for determinism.
- Keep combat/economy numbers matching `docs/mechanics-reference.md`; when
  they diverge, update the doc in the same change.
- **Menu prompts are consistent.** A numbered selection list ends with a
  `0) Quit` line and the standard `Choice> Quit` prompt (use `choiceQuit`),
  matching the menu engine's `readChoice` — not a one-off `promptInt("… 0 to
  cancel?")`. Keep new lists on this convention unless a specific case calls for
  something else.

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

Persistent, multi-user door game. One shared JSON world; concurrent multi-node
door play (each action reloads/re-validates/mutates/saves under a brief
exclusive flock — a pluggable `Store`: file-per-action for the door, in-memory
for the web; #5); per-caller empires keyed by BBS handle; per-turn economy
(idle empires stagnate) split from a daily maintenance step; turns-per-day
and new-realm protection; an event log for asynchronous play. Front-ends:
`cmd/immortal-barons` (door + `-local` local play) and `cmd/immortal-barons-web`
(experimental browser). Persistence design/plan:
`docs/superpowers/{specs,plans}/2026-07-03-door-*`.

Implemented gameplay: conventional combat (offense/defense, turrets, carriers,
jets, bomber airfield strikes), nuclear/chemical/biological strikes, pirate
raids, covert operations (spy, stir revolts, set up, support dissensions,
demoralize forces, bribery, expose enemy ops, and a Bomb Enemy Targets
submenu — incl. R5-Slappenheimer, the clone's rename of BRE's S3-Sabre),
diplomacy treaties, trading, region types + food market, SDI, Doomer Kaboomer
(BRE's Gooie Kablooie), player mail + a BRE-style multi-line message editor +
planetary bulletin, banking (deposit/withdraw/loan/invest), Set Industries +
Specialize, Write Macros, an About screen, a first-run language picker, and a
rising land-market price (expansion self-limiting). The menu tree mirrors BRE:
the Diplomacy, Covert, and InterPlanetary Operations menus are matched
item-for-item where a mechanic exists (recorded-but-inert items are flagged in
`docs/mechanics-reference.md`). Menus share a uniform exit — `'0'` labeled
"Quit" with a `DefaultOnEnter` hook so Enter triggers the default (Play/Quit on
the opening menu; Quit on submenus). A **Play** turn opens once with BRE's
pre-turn flow (event log → Diplomacy → Change Production). A **daily news
system** renders a Daily Bulletin header (planet totals with day-over-day
change) and a Today/Yesterday split of planet news (battles, WMD strikes,
pirate raids, riots, bank-rate moves, Planetary Master changes — original
wording), plus random per-empire "while you were away" events. The **sysop
Configuration Editor** edits the full BRE field set and those knobs are wired
into gameplay
(tax/region caps, maintenance/attack Level presets, interest, Buy Military,
etc.); a league Coordinator broadcasts the whole ruleset over inter-BBS.

**Inter-BBS ("Option A")**: file-drop `.brp` JSON packets in Inbound/Outbound
dirs; the sysop's transport moves them; `-planetary` processes inbound, launches
group attacks, and exports scores/news. **Localization**: help docs (po4a) and
UI strings (`internal/i18n`) render in the caller's language; de/ru are seeded
and grow via the `.po` catalogs.

Key gameplay knobs (unit values, maintenance, prices, `LandPriceStep`) are
constants — tune freely; keep them matching `docs/mechanics-reference.md`.

Stubbed / not built: leagues auto-reset (a daily Planetary Master leader is
tracked, `CurrentMaster`, and `endGame` crowns `LastMaster`, but timed
auto-reset is not built), the Quick-Strike / Extended-Battle attack variants,
civil-war collapse, an interplanetary individual-attack force (`Indiv. Attack
Force` is a stub), and BRE's finer interplanetary news subtypes. A few
covert/diplomacy items are recorded-but-inert pending fuller subsystems (flagged
in `docs/mechanics-reference.md`).

## Primary goal: run as a BBS door

The main goal is to run as a native door game under modern BBS software
(Synchronet, Mystic). Native means no DOSBox/DOSEMU. Linux is the primary
target, but the game builds and runs on macOS, Windows, and the BSDs (per-OS
file lock, `x/term` console) — so a Windows Synchronet door is in scope too.
The stage decomposes into:

1. **Dropfile + stdio/socket front-end** (`cmd/immortal-barons`) — DONE. Parses
   `DOOR32.SYS`/`DOOR.SYS` (`internal/door`), runs the game over a stdio
   `Session` (`session.Stdio`, which adds `\r\n`), honors the ANSI flag and
   a hard time-left cutoff, and names the realm from the caller's handle.
   The socket backend is built too: `session.Socket` attaches to the
   `DOOR32.SYS` line-2 handle (a winsock handle on Windows, a plain fd socket on
   *nix) via `net.FileConn`, and `openSession` wires it for a Windows door that
   reports a socket. On Unix, stdio is correct even when a socket is reported —
   Synchronet/Mystic (`EX_STDIO`) pipe the socket to stdin/stdout and handle
   telnet themselves. Serial/FOSSIL doors are explicitly unsupported.
2. **Persistence / multi-user** — DONE, now concurrent multi-node (#5). A
   persistent empire per caller in a shared JSON world; each door action
   reloads/re-validates/mutates/saves under a brief exclusive flock (a pluggable
   `Store`: file-per-action for the door, in-memory for the web), so several BBS
   nodes play at once. Keyed by BBS handle, with turns-per-day and daily
   maintenance (`internal/store`, `internal/play`, `internal/game/store.go`).
3. **Sysop config** — DONE. `config.json` with defaults + an in-game
   Configuration Editor (Coordinator menu); `-reset` writes the file (and
   seeds/re-seeds the world). The knobs are wired into gameplay and broadcast
   across a league.

Remaining toward the goal: validation of the door under real BBS software
(Synchronet/Mystic on Windows for the socket backend; needs Andy's env),
including confirming the assumption that the BBS performs telnet negotiation
before launching the door (the socket backend does no IAC handling). Scheduled
after the multiplayer web server lands.

Dropfile field maps and the I/O contract are documented in
`docs/mechanics-reference.md` and cross-checked against the Synchronet
source (`~/src/sbbs/src/xpdoor/dropfiles.c`).

## Commits

One-line subject by default; body only for a non-obvious *why*. End with the
`Co-Authored-By: Claude ...` trailer. Do not add any `claude.ai` session
trailer.
