#!/usr/bin/env bash
# Merge the UI extraction template into the per-language UI catalogs, so
# translators see the current msgids. Two-step UI flow:
#
#   python scripts/gen-ui-pot.py   # step 1: extract Go UI strings -> po/ui/immortal-barons.pot
#   scripts/merge-ui-po.sh         # step 2: msgmerge the .pot into internal/i18n/locale/<lang>.po
#
# msgmerge adds new msgids (empty msgstr), marks changed ones fuzzy, and drops
# obsolete ones. Untranslated/fuzzy strings fall back to English at runtime.
set -euo pipefail

# Resolve paths from the repo root, not the caller's cwd, so this can't silently
# skip on the repo-relative paths when run from scripts/ (or anywhere).
cd "$(dirname "$(readlink -f "$0")")/.."

pot="po/ui/immortal-barons.pot"
# The languages are whichever catalogs exist under internal/i18n/locale/, so a
# new language (created with `msginit`) is picked up with no edit here.
for po in internal/i18n/locale/*.po; do
  msgmerge --update --backup=none "$po" "$pot"
  echo "merged $pot -> $po"
done
