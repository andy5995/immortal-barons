# Immortal Barons — project guide

Immortal Barons is a from-scratch Go clone of the 1990s BBS door game
*Barren Realms Elite* (BRE). It is an independent reimplementation of the
game's rules and mechanics — no original code or art is used, and its prose is
its own. Never put John Dailey's (or any third party's) private contact info
into any artifact here.

**Where the line on the original's TEXT falls.** Its *prose* is not reused:
news lines, result reports, help and instruction text, flavor and narration are
all written here. What may match the original word for word is the **functional
furniture of a screen** — a question prompt, a field label, a menu item, a short
refusal — because those are dictated by what is being asked rather than by how
anyone chose to say it, and because IB's own versions were already all but
identical. The covert and terror operation menus were aligned that way on
2026-09-11 (`Send how many?`, `This will cost you N gold.  Accept?`,
`N agents sent out.`). Their two separate can't-afford refusals were aligned
with them and then dropped again on 2026-09-12: every site short of gold now
says the same thing in IB's own words, because three near-identical refusals in
two registers read as three different systems talking.

Two things this does NOT license, and both have bitten before: **code or art**
(source, decompiled routines, ANSI screens, logos) stays out entirely, and the
original's **product name** never goes on an IB screen — see below. When in
doubt about a string, ask: is this the game asking a question, or the game
telling a story? The first may match; the second is ours.

**Never put the original's NAME on IB's own screens.** Cloning a captured screen
copies its layout and colors; the product name in its header is branding, not
design, and must be replaced with IB's. The InterBBS Scores view shipped as
"Barren Realms Elite: Top Planets by Score" for months because the title came
along with the layout (fixed 2026-08-23; `TestScreensDoNotWearTheOriginalsName`
guards it). Naming the original in prose is fine and sometimes required — the
About screen's attribution and disclaimer, the README's Heritage, a doc
explaining a divergence. The line is identity, not mention.

A distinctive item name inside the game is a separate question, and Andy's to
answer one at a time. The nine pirate factions carry IB-original names; the
Gooie Kablooie, the S3-Sabre and SpyGuy keep the original's (#218). Ask before
renaming one, and do not revert one either way.

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

## This file, and the guides beside it

These notes are for any agent or tool working in this repository, kept
tool-neutral so they are not specific to one assistant. The root `CLAUDE.md` is
a one-line pointer at this file, because Claude Code does not discover
`AGENTS.md` on its own.

Task-specific guides live in **`.claude/skills/`**, one directory per topic,
each a `SKILL.md` whose `description` says when it applies. Read that
description to decide whether a guide is worth opening. The directory is named
for the tool that discovers them automatically, but the guides are plain
Markdown and are meant for any agent working here.

| Guide | Read it when |
| --- | --- |
| `bre-gather` | Reconstructing or checking any detail of the original — a menu, a constant, a screen layout. Read it BEFORE working from memory |
| `ib-testing` | Running the game to answer a question: verifying a number, reproducing something seen in play, exercising inter-BBS features |
| `ftn` | Anything touching FidoNet transport — packets, file attaches, BSO outbound, binkp, a mailer or tosser that is not delivering |
| `ansi-artwork` | Drawing or fixing text-mode art: a splash screen, a banner, a box border, colors that render wrong over telnet |

A guide describing a flag, path or behavior that no longer exists is worse
than no guide, because it will be believed. Fix one in place when you find it
stale.

## Build, test, run

```
go build ./...
go test ./...
go test ./internal/play/ -race        # the concurrency tests — run these when touching the store
GOARCH=386 go vet ./...    # CI runs this over the WHOLE tree; an int constant past
                           # 2^31 fails here and nowhere else (it has, on a test)
go run ./cmd/immortal-barons -local   # play locally in your terminal
gofmt -w .                 # always run before committing
python3 scripts/gen-ui-pot.py && scripts/merge-ui-po.sh   # UI strings — see below
scripts/gen-help-translations.sh   # help topics — see below
```

`-data` defaults to `./data`, and the repo root holds a real `data/world.json`.
Point a test run at a temporary directory rather than that one; the
[`ib-testing`](.claude/skills/ib-testing/SKILL.md) guide covers the rest.

Go 1.26. Prefer the standard library, but a dependency is fine when it clearly
earns its place (current deps: `golang.org/x/term`, `golang.org/x/text`,
`golang.org/x/sys`, and `tcell/v2` + `tview` for the config-editor TUI).
Keep the set small and justified. Commit `go.mod`/`go.sum`; do NOT commit
`vendor/` (distros build against their own packaged deps or fetch at build); a
release tarball may `go mod vendor` for offline builds. i18n uses gettext/PO:
**po4a** for the help docs and a small **in-house PO reader** (`internal/i18n`,
no runtime dependency) for UI strings. They are two separate generation passes —
a help topic goes through `gen-help-translations.sh` (which rewrites
each `content.<lang>/` tree and `po/help/`), a `tr()` string through the
`gen-ui-pot.py` pair — and running one does not regenerate the other.

**Do not regenerate translations as a matter of course.** Run these passes only
when preparing a release (the whole sequence is in `docs/dev/releasing.md`),
when a new translation is added, or when the parity test forces it (below).
Reworded English with a stale translation still renders: the PO catalogs fall
back per string, so the reader sees English for what has moved on, which is the
intended behavior and not a defect to chase. Running the passes on every text
edit instead churns the `.po` files and the `content.<lang>/` trees on
unrelated commits.

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
- `internal/ftn` — optional bidirectional FTN transport, run by `-maint`,
  `-planetary` and `-full` around the planetary step (unwrap before, handoff
  after the save; `cmd/immortal-barons/transport.go`) and configured by its own
  lines in `bbs.cfg`: hands `.brp` packets on unchanged (plain by default, or
  grouped into 8.3-named ZIP bundles for a `Bundled` peer) by stored-message
  attach, obox, or BSO/FLO, safely coalesces
  advertised BSO bundles while holding the peer `.bsy`, then validates/unwraps
  and routes them on receive. Its lock is never taken while the world lock is
  held, and `-full` never waits for it
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
  per-language content (`content/`, then `content.de/`, `.nl/`, `.pt/`, `.ru/`)
- `internal/docsite` + `cmd/barons-docs` — assembles the documentation website
  from the committed Markdown, so the site and the in-game help share one
  source. `go run ./cmd/barons-docs -out build/docs` writes `site-src/` and a
  generated `mkdocs.yml`; **`build/` is gitignored, so edit the generator, never
  its output.** The site's topic section is titled "Game Instructions" to match
  the in-game menu — it renders the same `internal/help` topics.
- `internal/i18n` — dependency-free gettext-PO reader for UI strings
- `internal/numfmt` — renders large numbers for display (locale thousands
  separator; grouped digits in full at any size, no float). It sits below both
  `game` and `menu` because the engine writes player-visible event text and
  cannot import `menu`

`menu.go` is the framework; `tree.go` is content. That split is the seam
that lets the menu tree grow without touching the engine. Language is threaded
to output helpers via a per-session `langSession` wrapper set in `menu.Run`, so
`ok`/`fail`/`prompt`/`tr` translate by the caller's `Empire.Language`.

## Conventions

- Run `gofmt -w .` before every commit; keep `go vet` clean.
- **The German catalog must stay CP437-representable** — it is the one
  translation a CP437 door still renders, which `TestPlayerLangCP437Fallback`
  enforces by falling the whole language back to English when it is not. An em
  dash is the character that breaks it; use a hyphen in German UI strings.
- **American spelling, everywhere** — code, comments, docs, help topics, the
  ChangeLog and anything a player reads. The tree was swept on 2026-09-11
  (color, behavior, neighbor, defense, center, gray, recognize, …); a British
  spelling that creeps back in is a defect like any other inconsistency.
- **Money is `int64`, everywhere.** `Empire.Gold`/`Bank`/`Debt`, investments,
  loans, prices that can reach money scale. Plain `int` is 32 bits on the
  32-bit door builds this project supports, which is what once capped gold in
  hand at 2 billion and silently discarded the rest. A new money field or a
  function returning gold takes `int64`; unit counts stay `int`. `goldCost` and
  `unitsAffordable` convert between the two widths without wrapping. Run
  `GOARCH=386 go test ./...` when touching money math — it catches overflows
  the 64-bit build hides.
- **A body that only gathers goes through `World.Read`; only a body that changes
  something goes through `World.With`.** On a door `With` is flock → reload →
  fn → **save** → release, so a snapshot taken through it rewrites `world.json`
  under the exclusive lock every other node is queued on, once per screen drawn —
  and a failed save is session-fatal, so a pure read could end a caller's session
  over a write it never needed. `Read` keeps the lock and the reload, which is
  what makes what it gathers current, and drops only the write-back. Neither may
  contain player input; the lock is held for the duration. Under `MemStore` the
  two are otherwise identical, so a mutation wrongly routed through `Read` would
  pass every test and lose the change only on a door — `MemStore.Snapshot`
  fingerprints the world either side of the body under a test binary and panics
  on a change, which is what makes the split enforceable rather than a habit.

  **The reverse — a pure read routed through `With` — no test can catch**, since
  the body changes nothing and the fingerprints match either way; only a door
  pays for it. Every `With` site in the tree was read and audited on 2026-09-12
  (#238) and the seventeen that remain all mutate. A NEW `With` is therefore the
  thing to check at review: name the mutation it makes, and if you cannot, it is
  a `Read`.
- **Every path that pays gold in goes through `World.creditGold`.** It holds
  gold in hand at the configured cap (`World.MoneyCap`, the sysop's
  `MoneyCapBillions`) and files an event naming what was lost and where it came
  from. `Withdraw` is the one exception: it draws only what fits and leaves the
  remainder banked, so it destroys nothing. A credit site that assigns
  `e.Gold +=` directly reintroduces the silent loss this replaced.
- **Separate data from code: every tunable gameplay/economy number is a named
  constant in a dedicated data file, not a bare literal in the formula code.**
  Those files are `internal/game/balance*.go`, split by subject — `_regions`,
  `_crown`, `_start`, `_ai`, `_prices`, `_hq`, `_costs`, `_combat`, `_networth`,
  `_pirates`, and `balance.go` itself for the rest. Put a new number in the file for its
  mechanic; the rule is the separation from the formulas, not any one filename.
  This covers unit costs, prices, rates, caps, starting-setup values, score
  weights, food/industry figures, penalties — all of it. When you add a
  mechanic, put its numbers in the data file (with a one-line provenance
  comment) and reference them by name. Only structural literals (`0`, `1`, `100`
  for percent math) stay inline.
- **The unit set has ONE table: `internal/game/units.go` (#134).** A screen that
  needs the goods declares a slice of those rows — `[]*game.Good{Trooper, Jet,
  …}` in its own order — and reads names and fields through the row (`Plural`,
  `Count`, `Prod`, `Made`, `Basket`, `Price`). Never restate the set as string
  literals, and never pair two lists by index: that is what this replaced, and
  it had `prodTypeNames[i]` labelling `prodField(p, i)` with nothing checking
  them. Both naming forms are on the row because both are identity keys:
  `Empire.Specialized` persists the plural and `==`-compares it, a market
  listing persists the singular. They stay English; `tr()` translates at render
  time. Buy Military and Sell (`menu/tree.go`) are still hand-written menu
  items — their per-unit actions are not in the table.
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
  contract. Mirroring a `balance*.go` constant is fine only for a playtest knob.
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
  build-status notes live, not this file. When you implement or change a
  mechanic — especially when you **complete an issue** — update the doc: correct
  the numbers, and flip any "IB currently does X / not yet built (#N)" note to
  the new reality when #N lands. A mechanic's entry must never still say it's
  unbuilt after you build it (the #58 region picker is how this drifted). Don't
  restate these numbers here — point to the spec.
- **Name another project in the docs and link it, every time.** DeuceGate is
  https://deucegate.bbsdev.net/, SyncTERM https://syncterm.bbsdev.net/. A reader
  meeting the name for the first time
  should not have to search for it, and a bare name in one file while another
  links it reads as two different things.
- **Single-source help details.** In the in-game help (`internal/help/content/`),
  each specific mechanic/unit fact (a ratio, a number, a unit property) belongs
  in ONE canonical file — the unit's or feature's own doc. Other help files that
  touch the concept in passing mention it generically without restating the
  figure (e.g. the 100-jets-per-carrier ratio lives in `jets.md`/`carriers.md`;
  the attack doc just says jets need carriers to fight). Duplicated numbers drift
  out of sync when tuned — this is the prose analogue of the balance*.go rule.
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
- **A screen is rendered once and written wherever it is needed.** A `Session`
  is an `io.Writer` plus `ReadKey`, so `session.NewWriter` turns any writer into
  one and the bulletin files are the same function that draws the player's
  screen (`internal/menu/bulletinfiles.go`). Laying a screen out a second time
  for a second destination gives you two copies that drift, and the one that
  drifts first is the one nobody is looking at.
- **A prompt whose input has ended must end the session, not return empty.**
  Callers loop on input they cannot parse, so a swallowed `io.EOF` spins them
  against a stream that will never produce another byte — `-reset` with no
  terminal redrew its editor 300,616 times in five seconds before this was
  fixed. `session.End` on any read error, as `prompt`, `promptInt` and
  `AskRealmName` all now do.
- **Player-visible prose is wrapped at render time, never left to the terminal.**
  Print it through `ok`/`okNoPause`/`fail` or `menu.WrapIndented` (exported for
  `internal/play`, whose onboarding runs before the menu engine); a bare
  `Fprintf` of a sentence is the defect. A terminal breaks an over-long line
  mid-word at column 80, and **a translated string is longer than the English it
  came from** — the German invalid-name message ran 84 columns and split "Reich"
  into "Re" and "ich". Checking that the English fits is what makes this recur:
  check the longest catalog rendering, or just wrap unconditionally.
  `TestOnboardingOutputFitsTheScreen` holds the onboarding path to 80 columns.
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
the spec, which is the single source (the same rule as balance*.go for constants).

`docs/bulletins.md` is the sysop page for the bulletin files: what is written,
the four `bbs.cfg` lines, the six header tokens and the stylesheet. A new
`docs/*.md` page has FOUR registration points and a missing one breaks a build
rather than the page — `sitePages` and the linker map (`internal/docsite`), the
nav in `buildNav`, the fixture in `assemble_test.go` (its nav check fails on a
page it cannot find), and `scripts/build-archives.sh`, which no `--include=*.go`
grep will ever match.

`docs/dev/releasing.md` is the release checklist — the translation passes, the
ChangeLog stamp, what to verify, and the **version bump after publishing**,
which was missed on v0.0.5.

`docs/dev/` holds deeper reference material: `bre-screens.md` (BRE's literal
on-screen output, layout, and ANSI colors, captured live — the source of truth
for UI fidelity), `bre-save-format.md` (its binary `game.dat` layout, mapped by
differential diffing), `bre-resource-dat.md` (every setting BRE's per-install
config file reads, including the 21 its own manual never mentions),
`ibbs-packet-format.md`, and `ftn-standards.md`.

`ftn-standards.md` describes the FTN formats themselves — the stored-message
header, the attribute bits, the kludge lines — and which FTSC document defines
each. Put FTN facts there and IB's use of them in `ibbs-packet-format.md`. No
FTSC document is vendored: their licenses forbid distribution inside a work
that may be sold, which this project's license permits, and they carry their
authors' contact details. Facts are free to restate, so the file says them in
its own words and links the originals. Nothing in FidoNet is an RFC.

## Status (v0.1.2 released 2026-09-17; v0.2.0 in development)

Persistent, multi-user door game: one shared JSON world, concurrent multi-node
door play, per-caller empires keyed by BBS handle. Sole front-end:
`cmd/immortal-barons` (door + `-local`). What is built, and how each mechanic
works, is in `docs/mechanics-reference.md`. The player's view is the in-game
help (`internal/help/content/`); the sysop's is `docs/` (`door-setup.md`,
`inter-bbs.md`, `ftn-transport.md`, `bulletins.md`, `charset.md`,
`command-reference.md`). The wire format is `docs/dev/ibbs-packet-format.md`,
and BRE's screens, with IB's deliberate divergences beside each, are
`docs/dev/bre-screens.md`. This section keeps only the rules those pages do not
state as rules.

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

**An IB league is IB-only. IB and BRE games never intermix.** IB defines its own
JSON `.brp` packets; BRE's IBBS format is binary, and neither reads the other.
So no reasoning may rest on wire compatibility with the original — not "a packet
stays readable to BRE", not a BRE board joining an IB league, not a strike
arriving from one. Where IB reuses one of BRE's values (the attack-kind codes
Quick=0/Normal=1/Extended=2, say), the reason is that the encoding stays
checkable against the disassembly, never interoperability. This has been got
wrong more than once, including in two code comments that shipped.

**Translations.** All four catalogs (de, nl, pt, ru) are machine translation,
unreviewed by a native speaker, and are expected to be corrected by a PR rather
than defended. Two traps, both of which have already cost this project something:

A catalog using any character outside CP437 is not offered to a CP437 caller at
all — de and nl fit, pt and ru do not — so a stray em dash in a Latin-script
catalog silently costs that language its door audience. Dutch was excluded for
months over three of them.

`msgmerge` fills a new entry from whichever old one has the most similar
ENGLISH, so a `#, fuzzy` msgstr is a guess about spelling and not about meaning.
De and ru each carried 105 such entries ("Bomb AirBases" as the German for Bomb
Trade Routes) and several had a verb the English does not — `%s` against `%d` —
which would have failed the build the moment anyone cleared the markers in bulk.
Read a fuzzy entry before clearing it; never un-fuzzy a catalog wholesale.

**Bulletin files** (`docs/bulletins.md`). `header.html`, `footer.html` and
`bulletin.css` are written ONLY when absent and never rewritten: HTML generation
was built and removed once because the template was compiled into the game and
a sysop could not restyle it, so a run that overwrote an edited wrapper would
reintroduce exactly that. Colors reach the page as `ansi-fg-N` / `ansi-bg-N`
class names, never as inline styles — those names are the whole contract a
replacement stylesheet has to meet. The `.ans` and `.txt` forms are CP437, not
UTF-8.

**Screen fidelity.** Deliberate divergences from BRE's screens are recorded in
`docs/dev/bre-screens.md` beside the screen each belongs to, and the ones most
likely to read as bugs also under "Screen output that deliberately diverges" in
`docs/mechanics-reference.md`. They must not be "corrected" back.

Key gameplay knobs are constants in `balance*.go`, but they are no longer all
free to tune: a growing set is **binary-verified** and marked as such in that
file (region Rate/Base pairs, the coastal support curve, unit costs and the
specialization modifiers, industrial gold and the unit pool, the crown tax).
Those are the fidelity contract — changing one means the game stops matching the
original, so it needs new evidence, not taste. The rest (prices, `LandPriceStep`,
AI behavior, IB's own additions) are the playtest knobs. Keep both matching
`docs/mechanics-reference.md`.

Stubbed / not built: a TIMED league-wide reset (each board runs `endGame` on its
own once `GameLength` game days pass, crowning `LastMaster`, and the Coordinator
can reset every board on demand with `-league-reset`, but nothing schedules a
reset across the league) and BRE's finer interplanetary news subtypes. A few covert/diplomacy items are recorded-but-inert pending
fuller subsystems (flagged in `docs/mechanics-reference.md`).

## Primary goal: run as a BBS door

A native door under modern BBS software (Synchronet, Mystic): no DOSBox or
DOSEMU. Linux is the primary target, but the game builds and runs on macOS,
Windows and the BSDs, so a Windows Synchronet door is in scope too. The
dropfile + stdio/socket front-end, concurrent persistence (#5) and sysop config
are done. Dropfile field maps and the I/O contract are in
`docs/mechanics-reference.md`, cross-checked against the Synchronet source
(`~/src/sbbs/src/xpdoor/dropfiles.c`). On Unix, stdio is correct even when a
socket is reported, since Synchronet/Mystic (`EX_STDIO`) pipe the socket to
stdin/stdout and handle telnet themselves; `session.Socket` is for a Windows
door. Serial/FOSSIL doors are unsupported.

Remaining: more validation under real BBS software on Windows (one sysop has
reported the socket path working with a live caller), including the assumption
that the BBS performs telnet negotiation before launching the door (the socket
backend does no IAC handling).

## Commits

One-line subject by default; body only for a non-obvious *why*. End with the
`Co-Authored-By: Claude ...` trailer. Do not add any `claude.ai` session
trailer.

**Do not ask a contributor to add a ChangeLog entry when reviewing.** Every
branch inserts at the top of the same `(in-progress:)` block, so a ChangeLog
hunk is a guaranteed conflict between any two open PRs — it was the only
conflict across #215, #224, #225 and #226, and cost #215 two merge commits that
did nothing else. Andy writes the entry on trunk after the merge.

## Issue titles: no category prefix

New issue and discussion titles are a plain statement of the problem — no `AI:`,
`InterBBS:`, `Web:` prefix. Labels carry the category (`AI`, `ssh-frontend`,
`windows`, `wire-format`, …; `gh label list` for the current set), and GitHub
shows them beside the title anyway. There is no inter-BBS label: that work is
most of the tracker, so the label narrowed nothing and was deleted. Commit
subjects keep their prefixes (`balance:`, `menu:`, `door:`) — git has no labels.
Existing titles are left as they are; if the right label doesn't exist, say so
rather than putting the category back in the title.
