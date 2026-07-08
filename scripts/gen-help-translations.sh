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

# Step 1: refresh the PO catalogs from the current English topics. --force
# bypasses po4a's mtime optimization, which otherwise silently skips extraction
# whenever the .pot happens to be newer than the source topics (e.g. after a
# checkout or a prior run) and leaves the catalogs stale.
po4a --force po4a.cfg

# Step 2: write the translated markdown from the catalogs.
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
done
