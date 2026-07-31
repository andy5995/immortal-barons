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
no runtime dependency) for UI strings.

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
- **`docs/mechanics-reference.md` is the authoritative spec — refresh it in the
  same change that touches a mechanic.** This is where the numbers *and* the
  build-status notes live, not CLAUDE.md. When you implement or change a
  mechanic — especially when you **complete an issue** — update the doc: correct
  the numbers, and flip any "IB currently does X / not yet built (#N)" note to
  the new reality when #N lands. A mechanic's entry must never still say it's
  unbuilt after you build it (the #58 region picker is how this drifted). Don't
  restate these numbers in CLAUDE.md — point to the spec.
- **Single-source help details.** In the in-game help (`internal/help/content/`),
  each specific mechanic/unit fact (a ratio, a number, a unit property) belongs
  in ONE canonical file — the unit's or feature's own doc. Other help files that
  touch the concept in passing mention it generically without restating the
  figure (e.g. the 100-jets-per-carrier ratio lives in `jets.md`/`carriers.md`;
  the attack doc just says jets need carriers to fight). Duplicated numbers drift
  out of sync when tuned — this is the prose analogue of the balance.go rule.
- **Menu prompts are consistent.** A numbered selection list ends with a
  `0) Quit` line and the standard `Choice> Quit` prompt (use `choiceQuit`),
  matching the menu engine's `readChoice` — not a one-off `promptInt("… 0 to
  cancel?")`. Keep new lists on this convention unless a specific case calls for
  something else.

## Mechanics fidelity

`docs/mechanics-reference.md` is the authoritative spec — how every mechanic
works *and* its numbers: units (offense/defense, maintenance, net-worth),
combat, pirate raids, covert ops, region types, economy, caps, the new-player
start flow, and the news feeds. It is built from the original binary strings
plus public strategy guides and the GameBanshee manual overview; values
cross-referenced against the actual BRE files (`~/.dosemu/drive_c/games/bre-dos/`)
are noted there and in the `bre-binary-verified-math` memory, and a disassembly
of the original binary is authoritative for exact constants.

**Disassembling BRE is a routine, productive move — reach for it early.** The
binary is at `~/.dosemu/drive_c/games/bre-dos/` (`BRE.OVR` holds the gameplay
overlays, `BRE.EXE` the rest); `ndisasm`, `radare2` and `ghidra` are installed.
Several mechanics that resisted inference from play — the coastal support curve,
industrial gold and unit production, the crown tax, and the whole technology
system — were read straight out of it in minutes. Method, the empire/config
record layout mapped so far, and a Turbo Pascal real-constant decoder live in
`~/.claude/projects/<this project>/scripts/bre-tpreal.py`; the economy-capture
parser is `bre-econ.py` beside it. Prefer reading the code to fitting a curve:
a fit needs dozens of samples and can still be wrong, and two separate BRE
constants were mis-set this way before the disassembly corrected them.

**Mechanics live in that spec, not in this guide.** Don't describe how a
mechanic works, or restate any of its numbers, here — this file just points to
the spec, which is the single source (the same rule as balance.go for constants).

`docs/dev/` holds deeper reference material: `bre-screens.md` (BRE's literal
on-screen output, layout, and ANSI colors, captured live — the source of truth
for UI fidelity), `bre-save-format.md` (its binary `game.dat` layout, mapped by
differential diffing), and `ibbs-packet-format.md`.

## Status (v0.0.3 released 2026-07-31; v0.0.4 in development)

Persistent, multi-user door game. One shared JSON world; concurrent multi-node
door play (each action reloads/re-validates/mutates/saves under a brief
exclusive flock — a pluggable `Store`: file-per-action for the door, in-memory
for the web; #5); per-caller empires keyed by BBS handle; per-turn economy
(idle empires stagnate) split from a daily maintenance step; turns-per-day
and new-realm protection; an event log for asynchronous play. Front-ends:
`cmd/immortal-barons` (door + `-local` local play) and `cmd/immortal-barons-web`
(experimental browser).

**Network-facing front-ends: security posture.** Any mode that listens on a
socket and lets a stranger reach game code — today the web front-end, later the
SSH one (#84) — was written mostly by an LLM and has had no review by anyone who
works in server security. That fact belongs in the user docs for each such mode
(`docs/webserver.md` today, plus README and FAQ) and must stay there until such a
review happens. Closing the hardening issues (#82 and its children) does not
retire it: they fix specific known problems rather than substitute for a review,
and an LLM self-review — mine included — is not the missing review, so don't
present one as clearing the bar.

Using SSH narrows the question but does not remove it: `x/crypto/ssh` carries the
crypto, transport, and authentication, but session lifetime, resource limits, and
concurrent mutation of the shared world stay IB's own code and IB's own risk.

Phrasing: state the fact and the risk, and leave the decision to the operator.
Don't tell them whether to run it — including for a LAN, which is not
automatically safe either.

Implemented gameplay: conventional combat (offense/defense, turrets, carriers,
jets, bomber airfield strikes; a winning attacker chooses the captured region
types and both sides' casualties are reported by unit type), nuclear/chemical/
biological strikes, pirate raids (now rolled per turn, not once a day; the nine
factions carry IB-original names, not BRE's), covert operations (spy, stir revolts, set up,
support dissensions,
demoralize forces, bribery, expose enemy ops, and a Bomb Enemy Targets
submenu — incl. R5-Slappenheimer, the clone's rename of BRE's S3-Sabre),
diplomacy treaties, trading, region types + food market, SDI, Doomer Kaboomer
(BRE's Gooie Kablooie), player mail + a BRE-style multi-line message editor +
planetary bulletin, banking (deposit/withdraw/loan/invest), Set Industries +
Specialize, Write Macros, four named advisors (Civilian/Economic/Military/
Technology), an About screen, a first-run language picker, and a
rising land-market price (expansion self-limiting). The menu tree mirrors BRE:
the Diplomacy, Covert, and InterPlanetary Operations menus are matched
item-for-item where a mechanic exists (recorded-but-inert items are flagged in
`docs/mechanics-reference.md`). Menus share a uniform exit — `'0'` labeled
"Quit" with a `DefaultOnEnter` hook so Enter triggers the default (Play/Quit on
the opening menu; Quit on submenus). A **Play** turn opens with the
"since your last play" event log — shown when you start your turn, not before
the opening menu; Diplomacy and Change Production are no longer pre-turn stops
(they moved to the System menu, #70). A **daily news
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

**Screen fidelity**: menus, tables, prompts, combat/raid reports, and the four
advisor pages match BRE's captured layout and ANSI colors — figures are
highlighted (bright-white or yellow) against dimmer body text, per
`docs/dev/bre-screens.md`.

Key gameplay knobs are constants in `balance.go`, but they are no longer all
free to tune: a growing set is **binary-verified** and marked as such in that
file (region Rate/Base pairs, the coastal support curve, unit costs and the
specialization modifiers, industrial gold and the unit pool, the crown tax).
Those are the fidelity contract — changing one means the game stops matching the
original, so it needs new evidence, not taste. The rest (prices, `LandPriceStep`,
AI behaviour, IB's own additions) are the playtest knobs. Keep both matching
`docs/mechanics-reference.md`.

Stubbed / not built: leagues auto-reset (a daily Planetary Master leader is tracked, `CurrentMaster`, and
`endGame` crowns `LastMaster`, but timed auto-reset is not built), the
Quick-Strike / Extended-Battle attack variants,
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

## Issue titles: no category prefix

New issue and discussion titles are a plain statement of the problem — no `AI:`,
`InterBBS:`, `Web:` prefix. Labels carry the category (`AI`, `InterBBS`,
`ssh-frontend`, `windows`, …; `gh label list` for the current set), and GitHub
shows them beside the title anyway. Commit subjects keep their prefixes
(`balance:`, `menu:`, `door:`) — git has no labels. Existing titles are left as
they are; if the right label doesn't exist, say so rather than putting the
category back in the title.
