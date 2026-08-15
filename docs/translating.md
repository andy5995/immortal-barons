# Translating Immortal Barons

Immortal Barons can show its interface and its help pages in more than one
language. This guide explains how to improve an existing translation and how to
add a new language.

You do not need to know Go. Translations live in standard **gettext PO files**
(`.po`), which you can edit in any text editor or in a PO editor such as Poedit.

## Two sets of text

There are two separate groups of text, kept in two places:

1. **The interface** — menus, prompts, on-screen messages, and reports.
   Catalogs live in `internal/i18n/locale/<lang>.po`.
2. **The help pages** — the topics shown in the in-game Guide and on the website
   Guide. Catalogs live in `po/help/<lang>.po`.

The website's own pages — this guide, the FAQ, and the setup pages — are kept in
English only.

Both groups use the same PO format, so the editing work feels the same. Only the
commands that refresh the files differ.

## What to install

- **gettext** — provides `msgmerge`, `msgfmt`, and `msginit`. Most systems
  already have it; otherwise install it with your package manager.
- **po4a** — used for the help pages only.
- Optional: **Poedit**, a graphical PO editor.

## The PO format in one minute

Each entry has an English source line and its translation:

```
msgid "Buy Regions"
msgstr "Regionen kaufen"
```

- `msgid` is the English text. Never change it.
- `msgstr` is your translation. Fill it in.
- An **empty** `msgstr` means "not translated yet". The game shows the English
  text instead, so you can translate a little at a time.
- A `#, fuzzy` line above an entry marks a guess that needs review. The game
  treats it as untranslated (it shows English) until you check the entry and
  remove the `#, fuzzy` line.
- Keep every placeholder — `%d`, `%s`, and so on — exactly as it appears in the
  `msgid`, in the same order. A wrong placeholder can break the display. A test
  checks this.
- Do not create two entries with the same `msgid`.

## Improve an existing translation

### Interface

1. Open `internal/i18n/locale/<lang>.po` (for example `de.po`).
2. Fill in or correct the `msgstr` lines.
3. Save. That is all — the game reads the `.po` file directly.

### Help pages

1. Open `po/help/<lang>.po`.
2. Fill in or correct the `msgstr` lines.
3. Rebuild the translated pages:

   ```
   scripts/gen-help-translations.sh
   ```

   This writes the translated Markdown under `internal/help/content.<lang>/`.

## Refresh the catalogs when they fall behind

The catalogs are refreshed in batches — before a release, when a language is
added, or when you are about to translate — rather than after every English
edit. A string that changed since the last refresh renders in English until
then. Run these from the project root.

### Interface

```
python scripts/gen-ui-pot.py     # collect the English interface strings
scripts/merge-ui-po.sh           # add new/changed ones to every locale .po
```

New strings appear with an empty `msgstr`, and changed ones become `#, fuzzy`,
for you to translate.

### Help pages

```
scripts/gen-help-translations.sh
```

This refreshes `po/help/*.po` from the English topics and rebuilds the
translated Markdown. Changed English is marked for review.

## Add a new language

Register the language in one place, then create and translate its two catalogs.
Use the two-letter language code (for example `es` for Spanish). Everything else
picks the language up on its own: the in-game menu, the website, and the refresh
scripts all read the language set from the list below and from the catalog files
on disk.

1. Add one line to `Languages` in `internal/i18n/languages.go`:

   ```go
   {Code: "<code>", Name: "<endonym>"},
   ```

   The endonym is the language's name in its own tongue (for example
   "Español"). This is what makes the game and the website offer the language.

2. Create the **interface** catalog from the template:

   ```
   msginit -i po/ui/immortal-barons.pot -l <code> -o internal/i18n/locale/<code>.po --no-translator
   ```

   (Run from `po/ui/` you can omit `-i` — msginit finds the template on its own.)

3. Create the **help** catalog: add `<code>:po/help/<code>.po` to the
   `[po4a_paths]` line in `po4a.cfg`, then run `scripts/gen-help-translations.sh`.
   It creates `po/help/<code>.po` and the translated pages.

Then translate the two new `.po` files as described above.

## Character set: some languages are UTF-8 only

Traditional BBS terminals use the CP437 character set, which has only a small set
of letters (see the [Character Set](charset.md) page). If your language uses any
character CP437 cannot show — Cyrillic, or Latin letters with less common accents,
for example — the game does not offer your language to callers running in CP437
mode. It is still offered to UTF-8 callers.

You do not need to do anything for this. The game detects it automatically from
your catalog: if any string uses a character outside CP437, the language is left
off the CP437 list so nothing shows as broken characters. It just means a CP437
caller may see only English, or the languages that fit CP437.

## Send your work

Open a pull request with the changed `.po` files. For help pages, also include
the regenerated files under `internal/help/content.<lang>/`.
