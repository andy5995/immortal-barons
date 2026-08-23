#!/usr/bin/env sh
# Fetch the Windows 7-capable Go toolchain used to build the windows/386 asset.
#
# Official Go dropped Windows 7, 8, 8.1 and Server 2008 R2 through 2012 R2 in
# Go 1.21, and since Go 1.24 its runtime calls ProcessPrng, which Windows 7 does
# not export: a stock-Go binary faults at address 0 before main, so even
# -version cannot run (issue #181). go-legacy-win7 is a Go fork that reverts
# that call and the other Win7-breaking changes.
#
# The fork is a complete Go distribution, and the patches are in its own runtime
# sources, so cross-compiling from Linux with it produces a Win7-capable target
# binary. Only windows/386 is built this way; every other target uses whatever
# `go` is on PATH.
#
# Usage: WIN386_GO=$(scripts/fetch-win7-go.sh --path) scripts/build-archives.sh ...
#   in CI: scripts/fetch-win7-go.sh >> "$GITHUB_ENV"   (prints WIN386_GO=<path>)
#
# It downloads about 65 MB into $CACHE_DIR (default: a temp dir) and verifies it
# against the SHA-256 the fork publishes with the release. A mismatch is fatal:
# an unverified toolchain does not build a binary anyone downloads.
set -eu

# Pinned deliberately. Bump both lines together, taking the hash from the
# release page — never from the download.
VERSION="1.26.7-1"
ARCHIVE="go-legacy-win7-${VERSION}.linux_amd64.tar.gz"
SHA256="6e168bfe5a1d27e4282ca5a18aebbbb5c06962c2a699d1e9857f2f39b67df986"
URL="https://github.com/thongtech/go-legacy-win7/releases/download/v${VERSION}/${ARCHIVE}"

CACHE_DIR="${WIN386_GO_CACHE:-${TMPDIR:-/tmp}/go-legacy-win7-${VERSION}}"
GOBIN="${CACHE_DIR}/go-legacy-win7/bin/go"

# Bare path for a shell to assign, or a NAME=value line for $GITHUB_ENV, which
# takes no "export".
emit() {
	if [ "${1:-}" = "--path" ]; then
		printf '%s\n' "$GOBIN"
	else
		printf 'WIN386_GO=%s\n' "$GOBIN"
	fi
}

if [ -x "$GOBIN" ]; then
	emit "${1:-}"
	exit 0
fi

mkdir -p "$CACHE_DIR"
echo "==> fetching go-legacy-win7 ${VERSION}" >&2
curl -fsSL -o "${CACHE_DIR}/${ARCHIVE}" "$URL"
echo "${SHA256}  ${CACHE_DIR}/${ARCHIVE}" | sha256sum -c - >&2
tar -xzf "${CACHE_DIR}/${ARCHIVE}" -C "$CACHE_DIR"
rm -f "${CACHE_DIR}/${ARCHIVE}"
# The version string is upstream's, not the fork's, so this proves the toolchain
# is the Go release we pinned and not what the archive happened to hold.
"$GOBIN" version >&2
emit "${1:-}"
