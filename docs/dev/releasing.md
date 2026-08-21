# Releasing

The steps a release takes, in order. Written after v0.0.5, where the version
bump was missed — every commit for the rest of that day built claiming to *be*
the released version, and that version rides in inter-BBS packets and feeds the
League Coordinator's minimum-version gate.

## Before tagging

1. **Curate the ChangeLog.** The in-progress block accumulates in commit
   order, which is neither the reader's order nor a finished list. Three passes,
   in order:

   - **Reorder** so the most important, most user-facing changes come first and
     sysop, command-line and packaging entries come last.
   - **Fold** entries that describe one change, and delete any entry that fixes
     something added in the same cycle — nobody upgrading ever saw the bug.
   - **Shorten** to the one-line rule: no entry wraps, no second person, no
     explanation of how a feature plays, no clause saying what BRE does. If a
     change will not fit on one line, ask before writing a longer one.

   Expect this to be substantial: the v0.0.6 pass cut 71 wrapped entries to 69
   single lines.

2. **Regenerate the translations.** They are deliberately NOT regenerated on
   ordinary commits, so a release is where they catch up:

   ```
   python3 scripts/gen-ui-pot.py && scripts/merge-ui-po.sh
   scripts/gen-help-translations.sh
   ```

   **Check what the UI extractor is not seeing.** `gen-ui-pot.py` scans for a
   fixed list of call forms, so a print helper added since the last release
   drops out of the catalogs silently — its strings translate at runtime and no
   translator has ever been shown them. At v0.0.6 that was `okNoPause`,
   `askYesNoHere` and `promptSuggestedTight`, 28 strings between them. To find
   the next one, list the helpers that pass a literal through `i18n.T` and
   compare them against `CALL_PATTERNS`.

   **Never clear a `#, fuzzy` flag without reading the translation.** `msgmerge`
   fills a new entry from whichever old one has the most similar *English*, so a
   fuzzy `msgstr` is a guess about spelling, not about meaning. `de.po` currently
   renders `Specialization` as "Regionen" and `Relations` as "Regionen" — both
   inherited from `Regions` on string similarity alone, and `ru.po` has the same
   pair. Nobody sees them: `internal/i18n` skips fuzzy entries by design, so the
   reader gets English until a human validates one. Un-fuzzying in bulk is what
   would ship them.

3. **Stamp the ChangeLog.** Replace the `(in-progress:)` heading with
   `YYYY-MM-DD (vX.Y.Z)`, matching the existing entries.

4. **Update the status line** in `CLAUDE.md` — which version is released and
   which is in development.

5. **Verify.** All of these, not a subset:

   ```
   gofmt -l .            # silence
   go vet ./...
   go test ./...
   GOARCH=386 go test ./internal/game/    # money math on the 32-bit door builds
   go test ./internal/play/ -race
   ```

6. **Commit** as `release: vX.Y.Z`.

## Tagging

Push trunk, then draft the release on GitHub and let publishing create the tag.
Do not tag or push tags from the command line.

The **Release** workflow builds on publish and attaches the assets — Linux,
macOS and Windows archives, the vendored source tarball, and `SHA256SUMS`.
Watch it: a release with no assets is a release nobody can use.

## After publishing

7. **Bump `Version` in `internal/game/game.go`** to the next patch. This is the
   step that gets forgotten, because the release feels finished once the assets
   are up.

   It is not cosmetic. The constant is the in-development version, so leaving it
   at the released number means every later build identifies itself as that
   release — in `-version`, in the About screen, in the door log, and in the
   version stamped on every inter-BBS packet, where a Coordinator's minimum
   version is tested against it.

8. **Delete any renamed or removed asset** left behind on the snapshot
   prerelease by hand. `replacesArtifacts` only replaces an asset of the same
   name, so a rename leaves the old file sitting beside the new one.

9. **Bump the Homebrew formula** in `HomebrewFormula/immortal-barons.rb` — the
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
