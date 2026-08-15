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
  or build on (proper attribution of public work). The Heritage section names
  Mehul Patel, who created BRE, and John Dailey Software, which has owned it
  since 1998 — creating and owning are different roles, so do not credit either
  with the other's. Say **owned**, not maintained: no new BRE release is known
  of. Do NOT write the original off as abandoned either — the copyright is
  asserted for the current year, the games are still sold as registrations, and
  the site was posting news within days of this being written (checked
  2026-08-15).
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
go test ./internal/play/ -race        # the concurrency tests — run these when touching the store
go run ./cmd/immortal-barons -local   # play locally in your terminal
gofmt -w .                 # always run before committing
python3 scripts/gen-ui-pot.py && scripts/merge-ui-po.sh   # UI strings — see below
scripts/gen-help-translations.sh   # help topics — see below
```

Go 1.26. Prefer the standard library, but a dependency is fine when it clearly
earns its place (current deps: `golang.org/x/term`, `golang.org/x/text`,
`golang.org/x/sys`, and `tcell/v2` + `tview` for the config-editor TUI).
Keep the set small and justified. Commit `go.mod`/`go.sum`; do NOT commit
`vendor/` (distros build against their own packaged deps or fetch at build); a
release tarball may `go mod vendor` for offline builds. i18n uses gettext/PO:
**po4a** for the help docs and a small **in-house PO reader** (`internal/i18n`,
no runtime dependency) for UI strings. They are two separate generation passes —
a help topic goes through `gen-help-translations.sh` (which rewrites
`content.de/`, `content.ru/` and `po/help/`), a `tr()` string through the
`gen-ui-pot.py` pair — and running one does not regenerate the other.

**Do not regenerate translations as a matter of course.** Run these passes only
when preparing a release (the whole sequence is in `docs/dev/releasing.md`),
when a new translation is added, or when the parity test forces it (below).
Reworded English with a stale translation still renders: the PO catalogs fall
back per string, so the reader sees English for what has moved on, which is the
intended behaviour and not a defect to chase. Running the passes on every text
edit instead churns the `.po` files and the `content.de/` and `content.ru/`
trees on unrelated commits.

The one case that cannot wait: **adding, renaming, or removing a help topic
requires `gen-help-translations.sh` in the same change.**
`TestHelpTranslationParity` walks the language trees and fails when an English
topic has no counterpart or a translated file has no English source, so a topic
added without regenerating breaks `go test ./...`. That test checks structure
only, never freshness, which is what makes deferring the rest safe.

## Architecture

The game logic never knows how it is shown to the player. It only reads
keypresses from, and writes ANSI bytes to, a `session.Session` (a byte
stream). Front-ends attach different streams; the engine is unchanged.

- `cmd/immortal-barons` — the door + local terminal front-end (stdio + dropfile;
  `-local`, `-maint`, `-planetary`, `-league-config`, `-reset`, …)
- `internal/session` — the `Session` byte-stream abstraction + console/stdio/
  socket implementations, shared `ReadLine`, and the Ctrl-key macro expander
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
- `internal/docsite` + `cmd/barons-docs` — assembles the documentation website
  from the committed Markdown, so the site and the in-game help share one
  source. `go run ./cmd/barons-docs -out build/docs` writes `site-src/` and a
  generated `mkdocs.yml`; **`build/` is gitignored, so edit the generator, never
  its output.** The site's topic section is titled "Game Instructions" to match
  the in-game menu — it renders the same `internal/help` topics.
- `internal/i18n` — dependency-free gettext-PO reader for UI strings
- `internal/numfmt` — renders large numbers for display (locale thousands
  separator; a 4-decimal `1.8473B` form at a billion). It sits below both
  `game` and `menu` because the engine writes player-visible event text and
  cannot import `menu`

`menu.go` is the framework; `tree.go` is content. That split is the seam
that lets the menu tree grow without touching the engine. Language is threaded
to output helpers via a per-session `langSession` wrapper set in `menu.Run`, so
`ok`/`fail`/`prompt`/`tr` translate by the caller's `Empire.Language`.

## Conventions

- Run `gofmt -w .` before every commit; keep `go vet` clean.
- **Money is `int64`, everywhere.** `Empire.Gold`/`Bank`/`Debt`, investments,
  loans, prices that can reach money scale. Plain `int` is 32 bits on the
  32-bit door builds this project supports, which is what once capped gold in
  hand at 2 billion and silently discarded the rest. A new money field or a
  function returning gold takes `int64`; unit counts stay `int`. `goldCost` and
  `unitsAffordable` convert between the two widths without wrapping. Run
  `GOARCH=386 go test ./...` when touching money math — it catches overflows
  the 64-bit build hides.
- **Every path that pays gold in goes through `World.creditGold`.** It holds
  gold in hand at the configured cap (`World.MoneyCap`, the sysop's
  `MoneyCapBillions`) and files an event naming what was lost and where it came
  from. `Withdraw` is the one exception: it draws only what fits and leaves the
  remainder banked, so it destroys nothing. A credit site that assigns
  `e.Gold +=` directly reintroduces the silent loss this replaced.
- **Separate data from code: every tunable gameplay/economy number is a named
  constant in a dedicated data file, not a bare literal in the formula code.**
  Today that file is `internal/game/balance.go`; as the set grows it's fine to
  split into more focused files (e.g. a `food.go`/`combat_balance.go`) — the
  rule is the separation, not the single filename. This covers unit costs,
  prices, rates, caps, starting-setup values, score weights, food/industry
  figures, penalties — all of it. When you add a mechanic, put its numbers in
  the data file (with a one-line provenance comment) and reference them by
  name. Only structural literals (`0`, `1`, `100` for percent math) stay inline.
- **There is no canonical unit table — reuse an existing list, don't add another
  (#134).** The unit set is enumerated independently in a dozen places
  (`MarketGoods`, `prodTypeNames`, `pirateSpoilNames`, `slappenheimerResources`,
  the trade and Buy Military tables, …), in several orders and two naming forms:
  singular `"Trooper"` in the market code, plural `"Troopers"` everywhere else.
  Two pairs are coupled by position — `prodTypeNames[i]` with `prodField(p, i)`,
  and `pirateSpoilNames` with the `PirateSpoil` enum in another package — so
  reordering one half silently mislabels the other, and `industry_test.go`
  asserts through raw indices (`want[4]` is tanks). The names are identity keys
  as well as labels: `Empire.Specialized` is persisted and compared with `==`,
  and `marketField` switches on the singular form, so they stay English and
  translation happens at render time through `tr()`.
- Tests use a scripted fake `Session` (see `internal/menu/menu_test.go`) and
  a fixed RNG seed via `game.NewSeed` for determinism.
- **A scripted key sequence must assert it REACHED the screen it tests.** When
  the script runs dry the session ends *cleanly*, so any flow change upstream
  (a new prompt, a re-mapped hotkey) leaves the test green while it never gets
  to the code it covers. Two tests had rotted this way — one for weeks, after a
  first-run language picker ate one key and shifted every key after it. Assert a
  marker unique to the target screen plus a state effect (`TurnsPlayed`
  increased, the treaty formed), never just "produced some output".
- **Assert BRE-verified numbers as golden literals, not as the constant.**
  `want := 500*RegularAttackCapturePct/100` follows a retune silently; `want :=
  50` fails and forces new evidence, which is the point of the fidelity
  contract. Mirroring a `balance.go` constant is fine only for a playtest knob.
- **A fixed-seed test may only assert what holds on OTHER seeds.** A macro
  balance outcome ("nobody is eliminated", "no realm survives below N regions")
  is a property of the whole simulation, and one seed is one trajectory.
  `TestGroundDownRealmsGetFinished` asserted a second claim that was false about
  0.9 times per run across 24 seeds; it passed only because its chosen seed
  never hit it, and that surfaced only when an unrelated config change
  reshuffled the run (f783c13). Run several seeds and assert the property, or
  assert an exact computed figure — those stay deterministic.
- **When deleting a component, grep the whole tree with NO extension filter.**
  An `--include=*.go --include=*.md --include=*.yml` sweep cannot match a `.sh`
  file, which is how the web removal left `scripts/build-archives.sh` building a
  deleted command and broke every release build (37a897e). Shell scripts,
  Makefiles and CI YAML all reference paths.
- **Save-format back-compat needs a frozen fixture**, not a struct the test
  zeroes and re-saves: the same marshaller on both sides can't catch a renamed
  JSON key. See `internal/store/testdata/world-v0.0.3.json`, which pins the
  legacy `Bulletin` alias and string-form events.
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
- **A screen that draws its own box must match the menu engine's, and BRE's
  width.** Two things are easy to get wrong on a hand-drawn screen, and both
  make it look unlike the rest of the game. The **closing rule** is not
  optional and is drawn in `dim(accent)`, never bare — the engine closes every
  menu box that way, from a live capture (`menu.go`, `draw`). And **BRE sizes
  each box to its own content**: its captures run 23 to 76 columns, so there is
  no house width to reach for. Take the width from that screen's capture in
  `docs/dev/bre-screens.md` (Industrial Production 46, Spending 44, System 75,
  …), not from the 62-column `rule` constant, and keep every line — blurbs
  included — inside it. Before calling a hand-drawn screen done, render it and
  an engine-drawn menu side by side and compare; a checklist of fixes is not
  the same as looking at the output.
- **Menu prompts are consistent.** A numbered selection list ends with a
  `0) Quit` line and the standard `> Quit` prompt (use `ChoiceQuit`, which
  prints the bare `>` and echoes the translated `Quit` as the Enter default),
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
system — were read straight out of it in minutes. The method and mapped record
layout live in `docs/dev/bre-disassembly.md`; the exact BRE-linked Turbo Pascal
Real48 port and calculator are `scripts/bre_real48.py` and
`scripts/bre-real48.py`. To locate code through original strings, run
`python3 scripts/bre-disasm.py find-string --directory /path/to/bre
"substring"`; it loads the durable string-reference table and returns all
currently named functions and blocks that refer to a matching Pascal string.
Use `--details` only when the private string text and exact use sites are needed,
and never commit that output. To walk the call graph, `lookup NAME_OR_ID` returns
durable `callers[].from_id` and `callees[].to_id` links plus exact `site_ids`;
follow those IDs with another `lookup`. Calculated calls carry a `dispatch_id`;
look it up, or use `list --kind dispatch`, for the proven closed target set and
assignment evidence. Prefer reading the code to fitting a curve: a fit needs
dozens of samples and can still be wrong, and two separate BRE constants were
mis-set this way before the disassembly corrected them.

**Mechanics live in that spec, not in this guide.** Don't describe how a
mechanic works, or restate any of its numbers, here — this file just points to
the spec, which is the single source (the same rule as balance.go for constants).

`docs/dev/releasing.md` is the release checklist — the translation passes, the
ChangeLog stamp, what to verify, and the **version bump after publishing**,
which was missed on v0.0.5.

`docs/dev/` holds deeper reference material: `bre-screens.md` (BRE's literal
on-screen output, layout, and ANSI colors, captured live — the source of truth
for UI fidelity), `bre-save-format.md` (its binary `game.dat` layout, mapped by
differential diffing), and `ibbs-packet-format.md`.

## Status (v0.0.5 released 2026-08-15; v0.0.6 in development)

Persistent, multi-user door game. One shared JSON world; concurrent multi-node
door play (each action reloads/re-validates/mutates/saves under a brief
exclusive flock — a pluggable `Store`: file-per-action for the door, in-memory
by default; #5); per-caller empires keyed by BBS handle; per-turn economy
(idle empires stagnate) split from a daily maintenance step; turns-per-day
and new-realm protection; an event log for asynchronous play. Sole front-end:
`cmd/immortal-barons` (door + `-local` local play).

**Network-facing front-ends: security posture.** The experimental browser
front-end was REMOVED (2026-08-01); an SSH front-end (#84) is under
consideration to replace it. The posture below is dormant, not retired — it
applies again the moment any mode listens on a socket and lets a stranger reach
game code.

Such a mode is written mostly by an LLM and has had no review by anyone who
works in server security. That fact belongs in the user docs for each such mode
(README and FAQ, plus the mode's own guide) and must stay there until such a
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
diplomacy treaties, trading, region types + food market, SDI, Clingy Annihilator
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
(they moved to the System menu, #70). Each recap entry sits under its own
numbered, timestamped rule, as BRE draws it, and any pending treaty offer is
prompted right after the recap with the proposer's stats inline. Mail then
follows unasked — BRE has no "read them now?" gate — one message per box, and
Enter is inert at the `[R]/[D]/[I]/[Q]` prompt so a held key cannot skip an
unread message. **v0.0.4 diplomacy additions:** the proposer is told whether an
offer was accepted or rejected (BRE's wording, filed asynchronously on their
recap); View Treaties renders BRE's `-*Relations*-` roster of every living
realm; and IB additionally lists the offers YOU sent, which BRE shows nowhere
(#92) — a new proposal to the same realm replaces the pending one, and
proposals do not expire (#95 tracks whether BRE expires them). A **daily news
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
group attacks, and exports scores/news. Two ed25519 key pairs guard it: the
Coordinator's (`coord.key`, recorded once by hand) authorises league orders, and
each board's own (`board.key`, published on an optional seventh roster line)
proves which board a packet came from. A roster entry with no key is applied
unchecked — where every league starts — so "cannot check" and "failed the check"
are deliberately different outcomes (`docs/mechanics-reference.md`). IP Messages (planet-addressed mail, with
an IB-only reply path) and Travel Times (measured round trips, kept by a probe
that rides the packets) ride the same transport. An individual strike picks its
type — Normal Attack / Quick Strike / Extended Battle, BRE-verified from
`game/attack.hlp` and a disassembly; a group attack gets no choice and fights as
a Normal Attack. **Localization**: help docs (po4a) and
UI strings (`internal/i18n`) render in the caller's language; de/ru are seeded
and grow via the `.po` catalogs.

**Screen fidelity**: menus, tables, prompts, combat/raid reports, the four
advisor pages, and the diplomacy screens (incoming treaty offer, View Treaties'
`-*Relations*-` roster, Alliance Strength) match BRE's captured layout and ANSI
colors — figures are highlighted (bright-white or yellow) against dimmer body
text, per `docs/dev/bre-screens.md`. **Deliberate divergences** are recorded
there, each beside the screen it belongs to, and must not be "corrected" back —
IB comma-groups figures BRE prints bare, and the offer's stats line separates
fields with `│` where BRE uses `; `, among others. IB also lists the treaty
offers you have SENT, which BRE shows nowhere (#92).

Key gameplay knobs are constants in `balance.go`, but they are no longer all
free to tune: a growing set is **binary-verified** and marked as such in that
file (region Rate/Base pairs, the coastal support curve, unit costs and the
specialization modifiers, industrial gold and the unit pool, the crown tax).
Those are the fidelity contract — changing one means the game stops matching the
original, so it needs new evidence, not taste. The rest (prices, `LandPriceStep`,
AI behaviour, IB's own additions) are the playtest knobs. Keep both matching
`docs/mechanics-reference.md`.

Stubbed / not built: a TIMED league auto-reset (a daily Planetary Master leader
is tracked, `CurrentMaster`, and `endGame` crowns `LastMaster`; the Coordinator
can start a new season on demand with `-league-reset`, but nothing schedules
one), the Free Trade Agreement's morale/support contagion between partners
(`BRE.OVR 0x99BF`, a daily-maintenance step), and BRE's finer interplanetary
news subtypes. A few covert/diplomacy items are recorded-but-inert pending
fuller subsystems (flagged in `docs/mechanics-reference.md`).

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
before launching the door (the socket backend does no IAC handling).

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
