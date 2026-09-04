# frozen_string_literal: true

# The comment above is here because this file lives in HomebrewFormula/ rather
# than Formula/ -- Homebrew's formula rubocop rules only apply to the latter, so
# the generic Ruby cops run and brew audit --strict fails without it. No formula
# in homebrew-core carries it; do not "tidy" it away.

class ImmortalBarons < Formula
  desc "Persistent multiplayer BBS door game inspired by Barren Realms Elite"
  homepage "https://github.com/andy5995/immortal-barons"
  # This is the vendored-source tarball the release workflow publishes, not the
  # auto-generated tag archive: the repo does not commit vendor/, so only this
  # asset lets the build skip a module download.
  url "https://github.com/andy5995/immortal-barons/releases/download/v0.0.9/immortal-barons-v0.0.9-vendored-source.tar.gz"
  sha256 "3ad3f213e01fe96f6d78df73b908bd78b346356f97c80db5b662b9d4441c4632"
  license "MIT"
  head "https://github.com/andy5995/immortal-barons.git", branch: "trunk"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args, "./cmd/immortal-barons"
    # Every path here must exist in the v0.0.9 tarball this formula pins, NOT
    # in the current tree -- trunk drifts as docs are added, and naming a file
    # no release has shipped fails the install with ENOENT. The vendored
    # tarball carries the whole docs/ tree, so check a pin bump against
    # "tar tzf <tarball> | grep docs/", not against build-archives.sh.
    doc.install "LICENSE", "README.md", "docs/faq.md", "docs/playing.md",
                "docs/command-reference.md", "docs/door-setup.md",
                "docs/charset.md", "docs/download.md", "docs/translating.md",
                "docs/inter-bbs.md", "docs/inter-bbs-troubleshooting.md",
                "docs/ftn-transport.md", "docs/bulletins.md"
  end

  def caveats
    <<~EOS
      immortal-barons is a BBS door game; run it under a BBS to play with other
      callers. To play solo in your own terminal instead:

        immortal-barons -local
    EOS
  end

  test do
    assert_path_exists bin/"immortal-barons"

    output = shell_output("#{bin}/immortal-barons -version")
    assert_match "immortal-barons", output
    assert_match version.to_s, output unless build.head?
  end
end
