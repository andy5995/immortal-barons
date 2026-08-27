package game

import "runtime/debug"

// version.go — what this build calls itself, and the packet protocol number
// that decides whether another board can read what it sends.

// Version is the single source of truth for the game's version string,
// displayed in the status bar and reported by the front-ends. The
// in-development version; bumped after each release.
const Version = "0.0.8"

// Protocol is the packet format this build speaks. It moves ONLY when the wire
// format changes, which is what lets a board take a menu fix or a balance change
// without the whole league moving with it — the game version cannot say that,
// because it moves for everything.
//
// It says nothing about whether two boards agree on the RULES. A retuned
// formula leaves the wire identical, so this number stays put while the two
// compute different outcomes — and an interplanetary attack is resolved on the
// target board, so the same attack resolves differently depending on which end
// receives it. Config.MinBoardVersion gates on the game version and is what
// covers that; the two are not alternatives.
//
// v0.0.7 additionally accepted a packet that stated NO protocol, so that
// introducing the field was a soft change rather than a cutover that stranded
// every board on the release meant to fix them. That concession is gone as of
// v0.0.8: a build old enough to omit the field is exactly the kind this exists
// to hold, and letting it through left the mechanism with a permanent hole.
//
// "Old enough" is about the BUILD, not the version it reports. Version stayed
// "0.0.7" for that whole development cycle while the field landed late in it,
// so a snapshot taken during it calls itself v0.0.7 and still states no
// protocol. A board on the v0.0.7 RELEASE stamps this number and is unaffected;
// a board on a snapshot from before the field is held, whatever it calls
// itself. One more reason a league board should run a release.
const Protocol = 1

// SpeaksOurProtocol reports whether a packet's format is one this build can
// apply. A packet that states no protocol is NOT one of them — see Protocol.
func SpeaksOurProtocol(p int) bool { return p == Protocol }

// Revision is the short VCS revision (7 hex chars, git's default short hash) the
// binary was built from, with a "-dirty" suffix when the working tree had
// uncommitted changes, or "" when the build carries no VCS info (e.g. `go build`
// outside a repo, or an unversioned release tarball). Go embeds this automatically
// for a `go build` from a git checkout. Shared by -version and the About screen.
func Revision() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	var rev string
	var dirty bool
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return ""
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return rev
}

// VersionString is the game's full version identity: the release version plus the
// short VCS revision when the build has one — "0.0.1 (ffdec31)", or "0.0.1
// (ffdec31-dirty)" for a dirty build, else just "0.0.1". The single source both
// the -version output and the in-game About screen use, so they never diverge.
func VersionString() string {
	if rev := Revision(); rev != "" {
		return Version + " (" + rev + ")"
	}
	return Version
}

// NameVersion is how the game introduces itself on screen: the program name and
// its full version, "Immortal Barons  v0.0.8 (ffdec31)". The single definition
// the About screen and the pre-menu banner both render, so the two cannot drift.
// Not translated — a proper noun and a numeral.
func NameVersion() string { return "Immortal Barons  v" + VersionString() }
