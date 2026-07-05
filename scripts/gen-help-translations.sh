#!/usr/bin/env bash
# Generate translated help docs from the PO catalogs.
#
#   po4a po4a.cfg              # step 1: update po/help/*.po from the English topics
#   scripts/gen-help-translations.sh   # step 2: write docs/help.<lang>/**
#
# Kept separate because config-mode po4a updates the catalogs but does not
# reliably write the translated files in this setup; po4a-translate does.
# Untranslated strings fall back to English (-k 0 = always write).
set -euo pipefail
langs=(de ru)
for lang in "${langs[@]}"; do
  po="po/help/$lang.po"
  [ -f "$po" ] || continue
  while IFS= read -r f; do
    rel="${f#internal/help/content/}"
    out="internal/help/content.$lang/$rel"
    mkdir -p "$(dirname "$out")"
    po4a-translate -f text -o markdown -o yfm_keys=title -k 0 \
      -m "$f" -p "$po" -l "$out" >/dev/null 2>&1
  done < <(find internal/help/content -name '*.md' | sort)
  echo "wrote internal/help/content.$lang/ ($(grep -c '^msgstr "[^"]' "$po" 2>/dev/null || echo 0) strings translated)"

  # Site docs (README, playing, sysop guide): -k 1 so an untranslated doc is
  # not written (the website falls back to English) instead of committing an
  # all-English "translation".
  for src in README.md docs/playing.md docs/sysop-guide.md; do
    out="${src%.md}.$lang.md"
    po4a-translate -f text -o markdown -k 1 -m "$src" -p "$po" -l "$out" >/dev/null 2>&1 || true
  done
done
