#!/usr/bin/env bash
# Regenerate the translated help docs from the English topics, in two steps:
#
#   1. po4a po4a.cfg  — refresh po/help/*.po (+ .pot) from the English topics
#   2. po4a-translate — write internal/help/content.<lang>/** from the catalogs
#
# Both run here so one invocation always produces correct, up-to-date docs.
# Config-mode po4a (step 1) updates the catalogs but does not reliably write the
# translated files in this setup, so po4a-translate (step 2) does the writing.
# Untranslated strings fall back to English (-k 0 = always write).
set -euo pipefail

# Resolve paths from the repo root, not the caller's cwd. Every path below is
# repo-relative, so running this from scripts/ (or anywhere) would otherwise
# skip both languages at the `[ -f "$po" ]` guard and silently do nothing.
cd "$(dirname "$(readlink -f "$0")")/.."

# Step 0: keep po4a.cfg's per-file [type: text] entries in sync with the content
# tree, so adding or removing a help topic needs no manual config edit. Only the
# generated block is rewritten — the header (comments, [po4a_paths] languages)
# and the trailing comment are preserved by splitting on the first/last entry.
{
  awk '/^\[type: text\]/{exit} {print}' po4a.cfg
  find internal/help/content -name '*.md' | sort | while read -r f; do
    rel="${f#internal/help/content/}"
    printf '[type: text] internal/help/content/%s $lang:internal/help/content.$lang/%s opt:"-o markdown -o yfm_keys=title"\n' "$rel" "$rel"
  done
  tac po4a.cfg | awk '/^\[type: text\]/{exit} {print}' | tac
} > po4a.cfg.tmp && mv po4a.cfg.tmp po4a.cfg

# Step 1: refresh the PO catalogs from the current English topics. --force
# bypasses po4a's mtime optimization, which otherwise silently skips extraction
# whenever the .pot happens to be newer than the source topics (e.g. after a
# checkout or a prior run) and leaves the catalogs stale.
po4a --force po4a.cfg

# Step 2: write the translated markdown from the catalogs. The languages are
# whichever catalogs po4a just refreshed under po/help/ (driven by po4a.cfg), so
# a new language needs no edit here.
for po in po/help/*.po; do
  lang="$(basename "$po" .po)"
  while IFS= read -r f; do
    rel="${f#internal/help/content/}"
    out="internal/help/content.$lang/$rel"
    mkdir -p "$(dirname "$out")"
    po4a-translate -f text -o markdown -o yfm_keys=title -k 0 \
      -m "$f" -p "$po" -l "$out" >/dev/null 2>&1
  done < <(find internal/help/content -name '*.md' | sort)
  echo "wrote internal/help/content.$lang/ ($(grep -c '^msgstr "[^"]' "$po" 2>/dev/null || echo 0) strings translated)"
done
