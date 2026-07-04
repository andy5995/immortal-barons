# Localization & terminal-capability setup (design)

Date: 2026-07-03
Status: design — awaiting review

## Why

Two related gaps for a real multi-user door:

1. **Language.** All user-facing text is hardcoded English. Callers should be
   able to pick a language at first login and change it in Preferences. Doing
   this now (while the string surface is still growing) is cheaper than
   retrofitting later — once the seam exists, new features are born
   translatable.
2. **Terminal capabilities.** A caller's terminal may or may not support ANSI,
   and may be CP437 or UTF-8. BBSes classically *ask* ("ANSI? 1=CP437 2=UTF-8")
   because there is no reliable universal auto-detect. We currently assume
   ANSI + UTF-8 unconditionally.

Both surface in the same place — a first-time **setup flow** and the
**Preferences** menu — so they're designed together, but they are independent
build slices.

## Scope and non-goals

- **In scope:** the i18n infrastructure (catalog, lookup, per-caller language),
  the language picker + Preferences entry, storing/honoring ANSI and charset
  per caller, and reading the dropfile ANSI flag.
- **Deferred (later slices, noted inline):** full plain-ASCII rendering when
  ANSI is off; CP437 output translation; the runtime DSR/cursor-advance probes;
  translating the *entire* string corpus (English ships first, other languages
  land incrementally).
- **Non-goal:** machine-translating everything up front. The infrastructure
  ships with English; translations are added a language at a time.

## Part A — Internationalization (i18n)

### Dependency decision

Use **`golang.org/x/text`** (official Go module): `message`, `language`, and
`feature/plural`. It earns the dependency — a door game is wall-to-wall
"N troopers / N gold", and `x/text` gives locale-correct **plural forms** and
**number formatting** that a hand-rolled catalog would reimplement badly. This
supersedes the old "stdlib-only" note in `CLAUDE.md` (already updated).

Packaging (**confirmed with Andy**): commit `go.mod`/`go.sum` only; **no
committed `vendor/`** (Debian/Fedora build against their packaged `x/text`;
Arch/SBo fetch at build). The release/dist step may `go mod vendor` for an
offline tarball, but the tracked tree stays vendor-free.

### Mechanism

- A per-caller `*message.Printer` built from the caller's `language.Tag` carries
  the active language through the render path. `p.Sprintf("You have %d troopers",
  n)` looks up the translation and formats numbers/plurals for that locale.
- **Keys are the English source strings** (x/text's default). Wrapping an
  existing literal *is* the extraction — low-friction retrofit. Missing
  translations fall back to the key (English) automatically.
- Translations live in **`internal/i18n/locales/<tag>.json`** embedded via
  `embed.FS`, registered into the message catalog at init with
  `message.SetString(tag, key, translation)` (and `message.Set` +
  `plural.Selectf` for the handful of pluralized strings). One JSON file per
  language keeps translation contributions self-contained.
- New package **`internal/i18n`**: exposes `Printer(tag) *message.Printer`,
  `Available() []Lang` (tag + display name for the picker), and `Parse(string)
  language.Tag` with English fallback.

### Threading the Printer through the render path

The engine writes to a `session.Session`; text is produced in `internal/menu`
(and a few report strings in `internal/game`). Options considered:

- **(chosen)** Put the active `*message.Printer` on the per-caller context the
  render layer already has. `internal/menu` functions take `w *game.World`; add
  the Printer to the world's active-session state (`World.P` or a small
  `Session`-adjacent render context) so `p.Sprintf` is reachable where strings
  are built. Game-layer report strings (combat/pirate) either take a Printer or
  return keys that the menu layer translates.
- Rejected: a global Printer — breaks multi-caller correctness (each session
  has its own language).

**Open decision A1:** do game-layer messages (e.g. pirate-raid text, combat
report) get translated by passing the Printer into `internal/game`, or by
having the game emit **message keys + args** that the menu layer renders? The
latter keeps `internal/game` presentation-free (matches the architecture) but
means structured events, not prebuilt strings. Recommendation: **keys + args**
for the event log / reports, so the engine stays presentation-agnostic.

### Retrofit strategy

Mechanical but sweeping. Do it in slices by area so each is reviewable:
menu tree labels → prompts/actions → turn narration/reports → help database →
splash/setup. Each slice wraps its literals and adds their English catalog
entries. English is authored inline (the key); no separate English JSON needed.

## Part B — Terminal capabilities (ANSI + charset)

### ANSI

- **Doors:** the BBS already negotiated ANSI at login and passes it in the
  dropfile. We already parse it (`internal/door/dropfile.go`: DOOR32.SYS field
  10 `emulation`, DOOR.SYS field 20 `GR`). v1: **honor** the stored flag.
- **Local/stdio (no dropfile):** optional DSR probe — send `ESC[6n`, read with a
  short timeout; a reply means ANSI. *(Later slice.)*
- **ANSI-off rendering** (stripping color/box art to plain ASCII) is a real
  effort — every escape-emitting path needs a no-op color mode.
  **Open decision B1:** v1 = gate only the block-art splash (skip it, show a
  plain title) and leave menus as-is, or v1 = a full ansi-stripping `Session`
  wrapper. Recommendation: **splash-gating first**, full ASCII mode later.

### Charset (CP437 vs UTF-8)

- Not in the dropfile (DOS-era format, no UTF-8 concept) — hence the classic
  manual prompt. v1: **default UTF-8** (our art is Unicode block glyphs), stored
  per caller, changeable in setup/Preferences.
- **CP437 output** (translating Unicode block/box glyphs to CP437 bytes on
  write, for SyncTERM/NetRunner) = a later `Session`-wrapper slice.
- **Cursor-advance UTF-8 probe** (emit `█`, `ESC[6n`, column delta 1 = UTF-8,
  3 = CP437) = later optional auto-detect.

## Part C — Setup flow & storage

### Per-caller storage

Add to `Empire` (the persistent per-caller record, keyed by owner handle):

- `Language string` — BCP-47 tag ("en", "es", …); "" = unset → run setup.
- `Charset string` — "utf8" | "cp437"; "" = unset.
- `ANSIPref` — tri-state (unset / on / off) overriding the dropfile flag.

Migration: `EnsureLocale()` on load leaves unset fields empty so first play
triggers setup; existing empires get the setup prompt on next login.

**Open decision C1:** language/charset are arguably properties of the
*connection/client*, not the empire. Storing on the Empire is simplest (one
caller = one empire) and persists the choice. Alternative: a separate per-owner
profile keyed by handle+node. Recommendation: **on the Empire** for v1.

### The flow

1. **First login / unset prefs → setup wizard:** confirm ANSI (prefilled from
   dropfile), pick charset (default UTF-8), pick language (list from
   `i18n.Available()`). Store on the Empire.
2. **Preferences menu** gains **Language**, **Charset**, and an **ANSI** toggle,
   so any of them can be changed later. (Preferences currently lives in
   `internal/menu/tree.go`.)
3. The chosen language builds the caller's `*message.Printer` for the session;
   charset/ANSI select the render mode.

## Implementation slices

1. **i18n core** — `internal/i18n` package, `x/text` dep, embed.FS catalog,
   Printer plumbing to the render layer, one area (menu tree labels) wrapped as
   the proof. Ships English. Adds a Spanish stub `es.json` to prove a second
   language renders.
2. **Language picker + Preferences entry** — `Empire.Language`, `EnsureLocale`,
   the setup prompt on unset, and the Preferences "Language" item.
3. **String retrofit** — remaining areas wrapped slice by slice (prompts →
   reports → help → splash), each with catalog entries.
4. **Terminal capabilities** — honor the dropfile ANSI flag; `Charset`/`ANSIPref`
   fields + setup/Preferences entries; splash-gating on ANSI-off.
5. **(Later)** full ASCII mode, CP437 output wrapper, DSR/UTF-8 probes,
   additional translations.

## Open decisions to confirm

- **A1** — engine emits message keys+args vs. taking a Printer. (Rec: keys+args.)
- **B1** — ANSI-off v1 = splash-gating vs. full ASCII mode. (Rec: splash-gating.)
- **C1** — store locale on Empire vs. a separate per-caller profile. (Rec: Empire.)
- **Dep** — confirm `x/text` (vs. a zero-dep homegrown catalog). (Rec: `x/text`.)
