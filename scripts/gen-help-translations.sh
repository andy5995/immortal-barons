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
    out="docs/help.$lang/$rel"
    mkdir -p "$(dirname "$out")"
    po4a-translate -f text -o markdown -o yfm_keys=title -k 0 \
      -m "$f" -p "$po" -l "$out" >/dev/null 2>&1
  done < <(find internal/help/content -name '*.md' | sort)
  echo "wrote docs/help.$lang/ ($(grep -c '^msgstr "[^"]' "$po" 2>/dev/null || echo 0) strings translated)"
done
