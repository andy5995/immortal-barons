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
  url "https://github.com/andy5995/immortal-barons/releases/download/v0.0.3/immortal-barons-v0.0.3-vendored-source.tar.gz"
  sha256 "6cd94c3d4956ccdfd8b4dceeb1ed3c693a6ed37232119fee79668db4d36ee29b"
  license "MIT"
  head "https://github.com/andy5995/immortal-barons.git", branch: "trunk"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args, "./cmd/immortal-barons"
    # This is the doc set scripts/build-archives.sh shipped at v0.0.3, the
    # release this formula pins -- not the current list on trunk, which has
    # since gained/lost files (e.g. docs/webserver.md was removed with the web
    # front-end; docs/download.md is newer than v0.0.3).
    doc.install "LICENSE", "README.md", "docs/faq.md", "docs/playing.md",
                "docs/command-reference.md", "docs/door-setup.md",
                "docs/charset.md", "docs/webserver.md", "docs/translating.md"
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
