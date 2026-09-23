#!/usr/bin/env sh
# Render the website's Screenshots page images.
#
# TestScreenshots (internal/menu/screenshots_test.go) plays each screen's key
# path and writes it as a CP437 .ans; ansilove draws each one in the IBM VGA
# font, as a DOS or SyncTERM caller sees it. The docs build runs this after
# barons-docs, so the images come from the current code on every build.
#
# Usage: scripts/render-screenshots.sh <out-dir>
#   e.g. scripts/render-screenshots.sh build/docs/site-src/en/screenshots
# Needs ansilove (Debian/Ubuntu package "ansilove").
set -eu

out="$1"
mkdir -p "$out"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

IB_SCREENSHOT_DIR="$tmp" go test ./internal/menu/ -run '^TestScreenshots$' -count=1
for f in "$tmp"/*.ans; do
	ansilove -q -o "$out/$(basename "$f" .ans).png" "$f" >/dev/null
done
