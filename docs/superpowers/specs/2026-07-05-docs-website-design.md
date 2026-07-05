# Documentation Website (design)

Date: 2026-07-05
Status: design — awaiting review

## Why

The player help, sysop guide, and README already live as Markdown in the repo
and are embedded into the game (help topics) or rendered by GitHub (docs). We
want a **real documentation website** — clean, modern, searchable, multilingual
— built from those *same* files, so there is one source of truth shared by the
in-game help and the public site, with no drift.

Constraints and decisions already made:

- **Look:** clean modern docs (sidebar nav, instant search, dark/light toggle,
  language switcher).
- **Generator:** MkDocs Material + the `mkdocs-static-i18n` plugin.
- **Single source:** the site is a *second consumer* of the committed Markdown;
  the game keeps `go:embed`-ing `internal/help/content{,.de,.ru}` unchanged.
- **Branch-free publish:** GitHub Pages via the Actions artifact deploy — no
  `gh-pages` branch, no committed HTML.
- **Languages:** English (default), German, Russian, matching the game.

Non-goals: versioned docs, a blog, comments, analytics, search-as-a-service
(Algolia), or authoring any new prose. This spec covers only assembling and
publishing existing content.

## Content model

The site is assembled from existing sources. Nothing new is authored.

| Site section        | Source                                             | Languages       |
|---------------------|----------------------------------------------------|-----------------|
| **Home**            | `README.md`                                        | en / de / ru    |
| **Player Guide**    | `docs/playing.md` (intro) + `internal/help/content{,.de,.ru}/**` | en / de / ru |
| **Running a Board** | `docs/sysop-guide.md`                              | en / de / ru    |
| **Developers**      | `docs/dev/**`                                      | en only         |

Explicitly **off** the public site (stay in-repo): `docs/mechanics-reference.md`
and `docs/superpowers/**`.

- The **Player Guide** groups and orders topics from the help frontmatter:
  `category` → sidebar group (Controls, Military, Economy, Warfare, Diplomacy,
  Inter-BBS), `order` → position, `title` → label. `in_game: false` topics (none
  today) would still appear on the site.
- **Developer docs** are English-only; under a non-English language they render
  via the plugin's English fallback.

## Architecture

Three units, each with one job:

1. **`cmd/barons-docs`** (Go) — the *assembler*. Reads the source Markdown and
   writes a throwaway MkDocs source tree (`site-src/`) plus a generated
   `mkdocs.yml`. Reuses `internal/help`'s frontmatter parser so "what a topic
   is" has one definition. Pure and deterministic: given the source dirs it
   always produces the same `site-src/` + `mkdocs.yml`.
2. **MkDocs Material** (Python, CI only) — renders `site-src/` → `site/` HTML.
3. **`.github/workflows/docs.yml`** — runs the assembler, builds with MkDocs,
   and deploys to GitHub Pages.

```
README.md, docs/playing.md, docs/sysop-guide.md   ┐
internal/help/content{,.de,.ru}/**                │→ cmd/barons-docs → site-src/ + mkdocs.yml
docs/dev/**                                        ┘        │
                                                            └→ mkdocs build → site/ → Pages
```

The game is untouched: it still embeds `internal/help/content{,.de,.ru}`. The
assembler is a read-only consumer of the same files.

## The assembler (`cmd/barons-docs`)

Responsibilities:

- **Layout for i18n.** Emit the `mkdocs-static-i18n` **folder** structure:
  `site-src/en/…`, `site-src/de/…`, `site-src/ru/…`. English is the default and
  the fallback for any page missing a translation.
- **Copy pages** from each source group into the right section/language:
  - `README.{md,de.md,ru.md}` → `<lang>/index.md` (Home).
  - `docs/playing.{md,de.md,ru.md}` → `<lang>/guide/index.md` (Guide intro).
  - `internal/help/content{,.de,.ru}/<cat>/<topic>.md` → `<lang>/guide/<cat>/<topic>.md`.
  - `docs/sysop-guide.{md,de.md,ru.md}` → `<lang>/running-a-board/index.md`.
  - `docs/dev/**` → `en/developers/**` (English only).
- **Generate `mkdocs.yml`.** Theme `material`; features: `navigation.instant`,
  `search.suggest`, `content.code.copy`, palette toggle (light/dark). Plugins:
  `search`, `i18n` (languages en/de/ru, default en, fallback en). `site_url` and
  `use_directory_urls: true` set for the project subpath
  (`https://andy5995.github.io/immortal-barons/`). The **nav** is generated from
  the help frontmatter: Home, then Guide (intro + categories in the fixed
  category order, topics sorted by `order` then `title`), then Running a Board,
  then Developers.
- **Rewrite intra-repo links.** Markdown links between the published sources
  (e.g. README → sysop guide) are rewritten to site paths. Links to *off-site*
  targets (e.g. `mechanics-reference.md`, `docs/superpowers/...`) are rewritten
  to their GitHub blob URL so they still resolve. Anchor links (`#heritage`) are
  left as-is. Image/asset links, if any, are copied alongside and repathed.

The assembler is the one place that knows the site's shape; MkDocs only sees the
generated tree.

## Translatability (po4a)

Extend the existing po4a pipeline (`po4a.cfg` + `po/`) to cover the three new
translatable sources, joining the help topics already handled:

- `README.md` → `README.de.md`, `README.ru.md`
- `docs/playing.md` → `docs/playing.de.md`, `docs/playing.ru.md`
- `docs/sysop-guide.md` → `docs/sysop-guide.de.md`, `docs/sysop-guide.ru.md`

Generated translated Markdown is **committed** (as the help `content.de/ru`
already is). po4a stays a translation-time step run via the existing scripts
when English text changes; **the docs build never runs po4a** — it reads the
committed output. Untranslated pages/strings fall back to English through the
i18n plugin, so partial de/ru coverage renders cleanly and the language switcher
always works. Dev docs are not added to po4a (English-only).

## CI & deploy

New workflow `.github/workflows/docs.yml`, matching repo conventions (the
concurrency block; pinned action SHAs/tags as in `ci.yml`):

- **Triggers:** `push` to `trunk`/`main` → build **and** deploy; `pull_request`
  → build only (validation, no deploy).
- **Build job:** `actions/checkout` → `actions/setup-go` (from `go.mod`) → run
  `go run ./cmd/barons-docs` (emits `site-src/` + `mkdocs.yml`) →
  `actions/setup-python` → `pip install -r requirements-docs.txt` (pinned
  `mkdocs-material`, `mkdocs-static-i18n`) → `mkdocs build --strict` (fail on
  broken internal links / warnings) → `actions/upload-pages-artifact` (the
  `site/` dir).
- **Deploy job** (push only): `actions/deploy-pages` with `pages: write` and
  `id-token: write` permissions. No `gh-pages` branch; no HTML in git.
- **URL:** `https://andy5995.github.io/immortal-barons/`.

`requirements-docs.txt` pins the exact MkDocs/plugin versions so local
(`mkdocs serve`) and CI builds match. Local dev needs only those two Python
packages (Go and po4a are already present); a venv keeps them off system Python.

## Testing

- **Assembler unit tests** (`cmd/barons-docs`): given a small fixture source
  tree, assert the produced `site-src/` layout (paths per language/section), the
  generated `mkdocs.yml` (nav order derived from frontmatter, languages, theme
  keys), and link rewriting (repo-internal → site path; off-site → GitHub URL;
  anchors untouched). Deterministic output → golden-file comparison.
- **CI build gate:** `mkdocs build --strict` fails the workflow on broken links
  or warnings, so a bad move/rename is caught before deploy.
- **No game behavior changes**, so the existing Go test suite is unaffected;
  `cmd/barons-docs` is additive.

## Implementation slices

1. **Assembler skeleton + English-only site.** `cmd/barons-docs` reads the
   English sources, writes `site-src/en/**` + a generated `mkdocs.yml` (nav from
   frontmatter), no i18n yet. Unit tests on layout + nav. `mkdocs build --strict`
   passes locally.
2. **Link rewriting.** Repo-internal → site paths; off-site → GitHub URLs;
   anchors untouched. Tests.
3. **i18n.** Add the `de`/`ru` trees (help `content.de/ru` already exist) and
   the `mkdocs-static-i18n` config with English fallback. Extend `po4a.cfg` for
   `README`, `playing.md`, `sysop-guide.md`; regenerate + commit their `.de/.ru`.
4. **CI + Pages.** `docs.yml` (build on PR, build+deploy on push),
   `requirements-docs.txt` pinned, Pages enabled (artifact deploy). Verify the
   live site + language switcher.

## Open decisions

- **Home vs. Guide intro overlap.** `README.md` (Home) and `docs/playing.md`
  (Guide intro) both introduce the game. Acceptable for now (Home = project
  overview incl. build/heritage/license; Guide intro = how-to-play). Revisit if
  they feel redundant on the site.
- **Exact MkDocs Material feature set** (e.g. whether to enable `navigation.tabs`
  vs a single sidebar) is a rendering nicety to settle during implementation;
  it does not affect the architecture.
