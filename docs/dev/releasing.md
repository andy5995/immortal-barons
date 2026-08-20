# Releasing

The steps a release takes, in order. Written after v0.0.5, where the version
bump was missed — every commit for the rest of that day built claiming to *be*
the released version, and that version rides in inter-BBS packets and feeds the
League Coordinator's minimum-version gate.

## Before tagging

1. **Regenerate the translations.** They are deliberately NOT regenerated on
   ordinary commits, so a release is where they catch up:

   ```
   python3 scripts/gen-ui-pot.py && scripts/merge-ui-po.sh
   scripts/gen-help-translations.sh
   ```

   **Never clear a `#, fuzzy` flag without reading the translation.** `msgmerge`
   fills a new entry from whichever old one has the most similar *English*, so a
   fuzzy `msgstr` is a guess about spelling, not about meaning. `de.po` currently
   renders `Specialization` as "Regionen" and `Relations` as "Regionen" — both
   inherited from `Regions` on string similarity alone, and `ru.po` has the same
   pair. Nobody sees them: `internal/i18n` skips fuzzy entries by design, so the
   reader gets English until a human validates one. Un-fuzzying in bulk is what
   would ship them.

2. **Stamp the ChangeLog.** Replace the `(in-progress:)` heading with
   `YYYY-MM-DD (vX.Y.Z)`, matching the existing entries.

3. **Update the status line** in `CLAUDE.md` — which version is released and
   which is in development.

4. **Verify.** All of these, not a subset:

   ```
   gofmt -l .            # silence
   go vet ./...
   go test ./...
   GOARCH=386 go test ./internal/game/    # money math on the 32-bit door builds
   go test ./internal/play/ -race
   ```

5. **Commit** as `release: vX.Y.Z`.

## Tagging

Push trunk, then draft the release on GitHub and let publishing create the tag.
Do not tag or push tags from the command line.

The **Release** workflow builds on publish and attaches the assets — Linux,
macOS and Windows archives, the vendored source tarball, and `SHA256SUMS`.
Watch it: a release with no assets is a release nobody can use.

## After publishing

6. **Bump `Version` in `internal/game/game.go`** to the next patch. This is the
   step that gets forgotten, because the release feels finished once the assets
   are up.

   It is not cosmetic. The constant is the in-development version, so leaving it
   at the released number means every later build identifies itself as that
   release — in `-version`, in the About screen, in the door log, and in the
   version stamped on every inter-BBS packet, where a Coordinator's minimum
   version is tested against it.

7. **Delete any renamed or removed asset** left behind on the snapshot
   prerelease by hand. `replacesArtifacts` only replaces an asset of the same
   name, so a rename leaves the old file sitting beside the new one.

8. **Bump the Homebrew formula** in `HomebrewFormula/immortal-barons.rb` — the
   `url`, the `sha256`, and the doc list:

   ```
   gh release download vX.Y.Z -p '*-vendored-source.tar.gz'
   sha256sum immortal-barons-vX.Y.Z-vendored-source.tar.gz
   tar tzf immortal-barons-vX.Y.Z-vendored-source.tar.gz | grep 'docs/.*\.md$'
   ```

   The formula installs from that tarball, so every path in `doc.install` has
   to exist inside it — the third command is the check. The vendored tarball
   carries the whole `docs/` tree, which is a wider set than the platform
   archives get from `scripts/build-archives.sh`.

   **A doc added on trunk does not go in the formula.** It is not in any
   released tarball yet, so the Homebrew CI job fails the install with
   `Errno::ENOENT` — which is how this step came to be written. The list
   changes only when the pin moves. `scripts/build-archives.sh` is the
   opposite: it runs from the tree at release time, so a new doc goes in it
   straight away.

   Missed at v0.0.5, which left the formula installing v0.0.4 until it was
   noticed.
