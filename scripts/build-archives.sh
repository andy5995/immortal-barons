#!/usr/bin/env sh
# Build cross-platform release/snapshot archives for Immortal Barons.
#
# Each archive wraps a single versioned dir (both binaries + docs) so it unpacks
# as one tidy folder rather than scattering files into the cwd. Output lands in
# dist/*.{tar.gz,zip} plus dist/SHA256SUMS. Pure-Go / CGO-free, so every target
# cross-compiles on one host.
#
# Usage: scripts/build-archives.sh <archive-label> <dir-version>
#   archive-label  goes in the archive FILENAME. For snapshots keep it stable
#                  ("snapshot") so the rolling release assets clobber in place;
#                  for a tagged release pass the version (e.g. "v0.0.1").
#   dir-version    goes in the wrapped dir name so an unpacked build is
#                  identifiable: the short commit sha for snapshots, the version
#                  for tagged releases.
set -eu

label="$1"
ver="$2"

# windows/386 is built by a Windows 7-capable toolchain, because a stock-Go
# binary cannot start on Windows 7 at all (#181) and old Windows is most of what
# a 32-bit build is for — anyone on Windows 10 or later takes the amd64 asset.
# scripts/fetch-win7-go.sh downloads and verifies it and sets WIN386_GO; CI runs
# that before this script. Unset, the 386 archive is still built, with stock Go
# and the warning below, so a local test build needs no 65 MB download.
if [ -z "${WIN386_GO:-}" ]; then
	echo "!!! WIN386_GO is not set: the windows/386 build will NOT run on Windows 7." >&2
	echo "!!! Run: WIN386_GO=\$(scripts/fetch-win7-go.sh --path) \$0 $*" >&2
fi

mkdir -p dist
for target in windows/386 windows/amd64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
	os="${target%/*}"
	arch="${target#*/}"
	ext=""
	[ "$os" = windows ] && ext=".exe"
	name="immortal-barons-${label}-${os}-${arch}"
	dir="immortal-barons-${ver}-${os}-${arch}"
	gocmd="go"
	if [ "$target" = windows/386 ] && [ -n "${WIN386_GO:-}" ]; then
		gocmd="$WIN386_GO"
	fi
	echo "==> building $name ($dir) with $($gocmd version)"
	mkdir -p "$dir"
	GOOS="$os" GOARCH="$arch" "$gocmd" build -trimpath -ldflags "-s -w" \
		-o "$dir/immortal-barons${ext}" ./cmd/immortal-barons
	GOOS="$os" GOARCH="$arch" "$gocmd" build -trimpath -ldflags "-s -w" \
		-o "$dir/barons-ftn${ext}" ./cmd/barons-ftn
	cp LICENSE README.md ChangeLog install-xtrn.ini "$dir/"
	mkdir -p "$dir/docs"
	cp docs/faq.md docs/playing.md docs/command-reference.md docs/door-setup.md \
		docs/bulletins.md docs/inter-bbs.md docs/charset.md docs/download.md \
		docs/translating.md "$dir/docs/"
	if [ "$os" = windows ]; then
		zip -qr "dist/${name}.zip" "$dir"
	else
		tar -czf "dist/${name}.tar.gz" "$dir"
	fi
	rm -rf "$dir"
done
# Hash the archives explicitly (not "*") so SHA256SUMS never lists itself, and
# so a later step can add more assets and regenerate over the same glob.
( cd dist && sha256sum immortal-barons-* > SHA256SUMS )
ls -lR dist
