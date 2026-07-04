# Docs, help & localization (design)

Date: 2026-07-03
Status: design — awaiting review

Supersedes the **i18n** portion (Part A) of
`2026-07-03-i18n-and-terminal-setup-design.md`. That spec's terminal-capability
and setup-flow parts (ANSI/charset detection, the startup wizard, the language
picker) still stand — the picker now selects a PO/po4a language.

## Why

Three problems are really one:

1. **Single source of truth.** The same content wants to live in the in-game
   help, the user-facing docs, and (badly) the dev mechanics reference. They
   will drift.
2. **Categorized help.** BRE's help is a flat rows-and-columns dump. Ours
   should be browsable: categories → topics → text.
3. **Localization.** Both the docs and the UI strings need translating, in a
   way distro and community translators can actually contribute to.

## Architecture: categorized Markdown topics are the single source

```
docs/help/
  controls/     interface, number entry
  military/     units, combat values
  economy/      regions, food, banking
  warfare/      attacks, covert, pirates
  diplomacy/    treaties
  interbbs/     leagues, group attacks
```

- One **Markdown file per topic**, under a **category folder**.
- **YAML frontmatter** per file: `title`, `category`, `order`, and
  `in_game: true|false` (some topics are docs-only). po4a's `YamlFrontMatter`
  module keeps frontmatter out of the translated strings.
- Authored in a **plain-text-friendly Markdown subset** (headings, lists,
  short tables, emphasis) so it renders well to BOTH an 80-column terminal and
  HTML.

### The mechanics move here

The *mechanics themselves* (unit stats, costs, combat formulas, economy
numbers) are player-facing strategy content → they become topic files
(`military/units.md`, `warfare/combat.md`, `economy/*.md`, `warfare/pirates.md`,
…). What stays dev-only is the thin **provenance** layer ("reconstructed from
the Wennagel guide", "BRE.EXE constant 25000", "differs from BRE in X") →
a slim `docs/dev/reconstruction-notes.md`. `mechanics-reference.md` mostly
dissolves into the topics.

## Three consumers of the topic tree

1. **In-game help** — `embed.FS` over `docs/help/**`; a small Markdown→ANSI
   renderer; a **categorized help menu** (categories → topics → paged text).
   Replaces `internal/menu/help.go`'s hardcoded topics.
2. **User web guide** — the same tree rendered to HTML. GitHub renders Markdown
   for free on day one; a static generator can come later. `docs/playing.md`
   becomes the landing/index page.
3. **Dev reconstruction notes** — `docs/dev/reconstruction-notes.md`, the slim
   remainder of the mechanics reference.

## Markdown → terminal rendering

The in-game help needs plain ANSI text, 80 columns. Two options:

- **(recommended) A tiny in-house renderer** for the Markdown subset we
  actually use (headings → color, lists → bullets, emphasis → strip or ANSI,
  tables → aligned columns, wrap to 80). Zero runtime dependency, and we
  control the subset.
- A Go Markdown lib (`goldmark`) with a terminal renderer. More capable, but a
  runtime dep and more than the subset needs.

**Open decision R1:** in-house subset renderer vs. `goldmark`. Lean in-house.

## Localization: gettext PO, via po4a

One translation format — **gettext `.po`** — for everything, chosen because
distro localization teams and tools (Poedit, Weblate, Transifex) already speak
it, which is what makes community/distro translation realistic.

### Docs & help (po4a)

- `po4a.cfg` lists the topic files, the target languages, and the format
  (`[type: text] docs/help/... opt:"-o markdown"`, or the `CommonMark` module),
  with `YamlFrontMatter` handling frontmatter.
- First target languages: **Russian (ru)** and **German (de)** (Andy).
- po4a extracts English → `po/help/<lang>.po`; translators fill them in;
  po4a regenerates `docs/help.<lang>/**` per language, tracking fuzzy/outdated
  strings when English changes.
- **The in-game help translation comes for free:** the game embeds the
  per-language Markdown and picks the caller's language. No separate help
  catalog.

### UI strings (gettext Go lib)

- The short program strings that are NOT docs (menu labels, prompts, report
  fragments) go through PO too, via a Go gettext library (e.g. `gotext`):
  extract from Go source → `po/ui/<lang>.po` → embedded/compiled catalog.
- gettext plural-forms cover the "N troopers" problem.
- This **supersedes the earlier `x/text` recommendation** — one format and one
  translator workflow across docs + UI beats Go-native-but-siloed.

### Runtime vs translation-time

- **Runtime dep:** the Go gettext lib (`gotext`) for UI-string lookup. The
  binary embeds the generated per-language Markdown + UI catalogs.
- **Translation-time only (NOT build/runtime deps):** po4a (Perl) and the
  gettext tools. They run in a translation/CI step that produces committed,
  embeddable generated files. `go build` and the distro *runtime* package never
  see Perl.
- Packaging unchanged: `go.mod`/`go.sum` committed, no `vendor/` (confirmed).

## In-game categorized help menu

Replace the flat help database with: **Help → list categories → pick a category
→ list its topics → pick → render the topic (paged)**. Frontmatter `order` sorts
within a category; `in_game:false` hides docs-only topics.

## Implementation slices

1. **Scaffold + renderer + menu (English).** Category/topic layout, the
   Markdown→ANSI renderer, the categorized help menu; migrate one category
   (e.g. `controls/`) as proof. Retire the corresponding `help.go` entries.
2. **Content migration.** Move the remaining mechanics + help content into
   topics; create `docs/dev/reconstruction-notes.md`; make `playing.md` the
   index. Delete the migrated parts of `mechanics-reference.md`.
3. **UI-string i18n.** Extract UI strings to `po/ui/`, wire `gotext` at runtime,
   connect to the caller's language pref (from the terminal-setup spec).
4. **po4a pipeline.** `po4a.cfg`, and a sample second language (docs + UI) end
   to end, proving translation regenerates and the game renders it.
5. **Web assembly (later).** Static-site rendering of the topic tree; GitHub
   Markdown works from slice 1.

## Open decisions

- **R1** — Markdown→terminal renderer: in-house subset vs `goldmark`. (Lean
  in-house.)
- ~~Category set — finalize the folder list above.~~ **Decided (Andy):**
  `controls / military / economy / warfare / diplomacy / interbbs` is a good
  starting split; expect it to change with player feedback (cheap to move
  topics between folders).
- Frontmatter schema — exact fields.
- gettext Go lib — `gotext` vs alternatives.
- ~~Generated translated files — committed to the tree, or built in CI only?~~
  **Decided (Andy): committed.** Both the `.po` files (translator-editable
  source) AND the po4a-generated translated Markdown (`docs/help.<lang>/**`,
  which the game embeds) are committed. This keeps po4a out of `go build`:
  the Go build embeds ready files, and po4a runs only when a translation
  changes (edit `.po` → run po4a → commit `.po` + regenerated `.md`).
- po4a module — `text`+markdown opt vs the dedicated `CommonMark` module.

## Relationship to the terminal-setup spec

The language picker and per-caller language storage designed in
`2026-07-03-i18n-and-terminal-setup-design.md` (Parts B/C) are the front-end to
this: the caller's chosen language selects both the embedded Markdown language
and the `gotext` catalog. Build that picker as specced; only the *translation
backend* changes (PO/po4a instead of x/text).
