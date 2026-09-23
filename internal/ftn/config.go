// Package ftn bridges Immortal Barons packet directories to stored-message
// file attach, direct obox, and BSO/FLO mailer handoffs. It runs inside the
// game's own inter-BBS modes; its settings are lines in bbs.cfg.
package ftn

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/store"
)

// LegacyConfigFile is the file these settings lived in while the transport was
// a separate program. It is no longer read: a board that still has one is
// refused, with the lines to move, by LegacyRefusal.
const LegacyConfigFile = "ftn.cfg"

// The bbs.cfg keywords this package reads. The game's own reader ignores them,
// as it ignores every keyword it does not know, so one file serves both.
// IncomingFileDir, NetmailDir and Mailer are the original's BBS.CFG lines 4, 5
// and 7, named after the labels its manual gives them.
const (
	keyIncomingFileDir    = "IncomingFileDir"
	keyNetmailDir         = "NetmailDir"
	keyIncomingNetmailDir = "IncomingNetmailDir"
	keyAttachDir          = "AttachDir"
	keyMailer             = "Mailer"
	keyLink               = "Link"
	keyBundled            = "Bundled"
	keyOboxMeshFanout     = "OboxMeshFanout"
	keySubjectPath        = "SubjectPath"
)

// mailers is the original's list for BBS.CFG line 7, in the casing IB writes
// it. Only Binkley changes what the game does (the ^ on the attach Subject),
// and None writes no netmail at all; the rest are accepted so a sysop can copy
// the line from an original install unchanged.
var mailers = []string{"FrontDoor", "Binkley", "DBridge", "InterMail", "DBridgeOld", "Other", "None"}

// CanonicalMailer returns name in IB's casing, and false for a name that is not
// on the original's list.
func CanonicalMailer(name string) (string, bool) {
	for _, m := range mailers {
		if strings.EqualFold(m, strings.TrimSpace(name)) {
			return m, true
		}
	}
	return "", false
}

// SubjectMode selects how an attachment is spelled in the Type-2 Subject. The
// spelling is resolved by the mailer, so it is configured separately from the
// directory the file is actually written to: only the operator knows the
// mailer's working directory and attachment search path.
type SubjectMode int

const (
	// SubjectAbsolute writes the full pathname. It is the default.
	SubjectAbsolute SubjectMode = iota
	// SubjectBasename writes the filename alone, for a mailer that searches
	// its own attachment directory.
	SubjectBasename
	// SubjectPrefixed writes SubjectPrefix + filename, resolved by the mailer
	// against its working directory.
	SubjectPrefixed
)

// Config contains settings local to the FTN transport.
type Config struct {
	NetmailDir string
	// Mailer is the Mailer line, canonical; empty when the line is absent,
	// which behaves as Other. Binkley and NoNetmail are what it decides.
	Mailer    string
	Binkley   bool
	NoNetmail bool
	// AttachDir is where a claimed packet is written for an Attach or BSO
	// link. Empty defaults to attachmentDirectory's own default (dataDir's
	// att child, see transport.go) -- this field no longer has any notion
	// of a per-outbound-directory child; that was retired with the
	// bundled transport (#231).
	AttachDir     string
	SubjectMode   SubjectMode
	SubjectPrefix string
	// IncomingFileDir is the mailer's receive directory. IncomingNetmailDir is
	// where received stored-message envelopes are found; unset, every
	// IncomingFileDir is searched (netmailDirs).
	//
	// IncomingFileDirs holds every directory named by an IncomingFileDir line,
	// in order; IncomingFileDir is the first. A mailer can deliver into more
	// than one -- ENiGMA files an authenticated session into secInbound and an
	// unauthenticated one into inbound -- and a board that names only one
	// silently never reads the other (#236).
	IncomingFileDir    string
	IncomingFileDirs   []string
	IncomingNetmailDir string
	Links              map[int]Link
	OboxMeshFanout     bool
	// Bundled is the board-wide posture: false (the default) sends every peer
	// plain packets, true sends bundles. A Link line saying Raw or Bundled
	// overrides it for that peer. It is a posture rather than a per-link chore
	// because it changes once in a league's life -- off while boards are still
	// upgrading, on when they are all past it -- and a Coordinator should not
	// have to edit every link to make that switch (#230).
	Bundled bool
}

// LinkMode is the handoff exposed by one directly connected peer.
type LinkMode int

const (
	LinkAttach LinkMode = iota
	LinkObox
	LinkBSO
)

// Link describes one local mailer handoff. Directory is an obox for LinkObox
// and the exact BSO directory for the destination's zone for LinkBSO.
type Link struct {
	Mode      LinkMode
	Directory string
	Flavor    string
	// Raw sends this peer one unbundled game packet per file, the shape every
	// board understood before the bundled transport. It is a modifier on the
	// handoff rather than a mode of its own, because a peer that cannot read a
	// bundle may still be reached by attach, obox or BSO — the envelope is what
	// it cannot parse, not the way the file travels (#230).
	//
	// It is the DEFAULT, and `Bundled` is what turns it off. A sysop who
	// upgrades and configures nothing keeps sending what every board can
	// already read, so the unsafe state has to be asked for rather than
	// arrived at. The cost is real and is paid until a link is switched: a raw
	// file carries no routing manifest, so a receiver rebuilds Route from the
	// packet's own FromNode and learns nothing about which peers a broadcast
	// already covered. Bounded by MaxPacketHops and by replay detection rather
	// than by the manifest, exactly as it was before the bundled transport.
	//
	// THE DEFAULT IS MEANT TO FLIP. It is raw for the release that introduces
	// bundles, so no league can be broken by a sysop who upgrades without
	// reading anything. Once every board in the wild can unwrap a bundle, the
	// default becomes Bundled and Raw goes back to being the opt-in it was
	// written as -- otherwise the routing manifest stays switched off for
	// everyone who never touched their configuration.
	Raw bool
	// RawSet distinguishes "this link says raw" from "this link says nothing",
	// so the board-wide posture below can supply the answer for links that do
	// not state one.
	RawSet bool
}

// subjectPath is how that file is spelled for the mailer.
func (c Config) subjectPath(attached string) string {
	switch c.SubjectMode {
	case SubjectBasename:
		return filepath.Base(attached)
	case SubjectPrefixed:
		return joinSubject(c.SubjectPrefix, filepath.Base(attached))
	}
	return attached
}

// joinSubject separates with the prefix's own convention: the path is resolved
// by the mailer, which may not run on the same kind of system as the game.
func joinSubject(prefix, base string) string {
	switch {
	case prefix == "":
		return base
	case strings.HasSuffix(prefix, "/"), strings.HasSuffix(prefix, `\`):
		return prefix + base
	case strings.Contains(prefix, `\`):
		return prefix + `\` + base
	}
	return prefix + "/" + base
}

// LoadConfig reads the transport's lines out of <dataDir>/bbs.cfg. A missing
// file, or one with none of them, is a board with no FTN transport: Receives
// and Sends both answer false, and nothing runs.
func LoadConfig(dataDir string) (Config, error) {
	path := filepath.Join(dataDir, store.BoardConfigFile)
	cfg := Config{OboxMeshFanout: true, Links: map[int]Link{}}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		key, value, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		switch {
		case strings.EqualFold(key, keyNetmailDir):
			cfg.NetmailDir = value
		case strings.EqualFold(key, keyAttachDir):
			cfg.AttachDir = value
		case strings.EqualFold(key, keyIncomingFileDir):
			if value != "" {
				cfg.IncomingFileDirs = append(cfg.IncomingFileDirs, value)
			}
		case strings.EqualFold(key, keyIncomingNetmailDir):
			cfg.IncomingNetmailDir = value
		case strings.EqualFold(key, keyBundled):
			b, err := parseYesNo(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s: %s: %w", path, keyBundled, err)
			}
			cfg.Bundled = b
		case strings.EqualFold(key, keyOboxMeshFanout):
			b, err := parseYesNo(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s: %s: %w", path, keyOboxMeshFanout, err)
			}
			cfg.OboxMeshFanout = b
		case strings.EqualFold(key, keyLink):
			node, link, err := parseLink(value, dataDir)
			if err != nil {
				return Config{}, fmt.Errorf("%s: %s %s: %w", path, keyLink, value, err)
			}
			cfg.Links[node] = link
		case strings.EqualFold(key, keySubjectPath):
			switch {
			case strings.EqualFold(value, "Absolute"):
				cfg.SubjectMode = SubjectAbsolute
			case strings.EqualFold(value, "Basename"):
				cfg.SubjectMode = SubjectBasename
			default:
				cfg.SubjectMode = SubjectPrefixed
				cfg.SubjectPrefix = value
			}
		case strings.EqualFold(key, keyMailer):
			// Refused rather than defaulted: a misspelled None would go on
			// writing netmail, and a misspelled Binkley would silently drop
			// the ^ its tosser relies on.
			m, ok := CanonicalMailer(value)
			if !ok {
				return Config{}, fmt.Errorf("%s: %s %q is not one of %s",
					path, keyMailer, value, strings.Join(mailers, ", "))
			}
			cfg.Mailer = m
			cfg.Binkley = m == "Binkley"
			cfg.NoNetmail = m == "None"
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, err
	}
	if cfg.NoNetmail {
		for node, link := range cfg.Links {
			if link.Mode == LinkAttach {
				return Config{}, fmt.Errorf("%s: %s %d Attach needs netmail, and %s None writes none",
					path, keyLink, node, keyMailer)
			}
		}
	}
	if cfg.NetmailDir != "" && !filepath.IsAbs(cfg.NetmailDir) {
		cfg.NetmailDir = filepath.Join(dataDir, cfg.NetmailDir)
	}
	if cfg.AttachDir != "" && !filepath.IsAbs(cfg.AttachDir) {
		cfg.AttachDir = filepath.Join(dataDir, cfg.AttachDir)
	}
	// Absolute and de-duplicated: the same directory named twice would have
	// every file in it enumerated twice, and the second pass then warns about a
	// source the first has already consumed. A NESTED directory is kept, not
	// dropped: RunIn reads one level and does not descend, so a mailer's
	// unsecure child needs its own line to be read at all.
	seen := map[string]bool{}
	dirs := cfg.IncomingFileDirs[:0]
	for _, dir := range cfg.IncomingFileDirs {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(dataDir, dir)
		}
		dir = filepath.Clean(dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true
		dirs = append(dirs, dir)
	}
	cfg.IncomingFileDirs = dirs
	if len(cfg.IncomingFileDirs) > 0 {
		cfg.IncomingFileDir = cfg.IncomingFileDirs[0]
	}
	if cfg.IncomingNetmailDir != "" && !filepath.IsAbs(cfg.IncomingNetmailDir) {
		cfg.IncomingNetmailDir = filepath.Join(dataDir, cfg.IncomingNetmailDir)
	}
	return cfg, nil
}

// Receives reports whether this board has a mailer inbound to unwrap.
func (c Config) Receives() bool { return len(c.IncomingFileDirs) > 0 }

// Sends reports whether this board hands packets to FTN at all: a Link line, or
// a netmail directory for the attach every unlinked peer gets.
func (c Config) Sends() bool {
	return len(c.Links) > 0 || (c.NetmailDir != "" && !c.NoNetmail)
}

// netmailDirs names where received .msg envelopes are looked for: the
// configured directory when there is one, otherwise EVERY inbound directory.
// Defaulting to the first alone made the order of the lines decide whether
// envelopes were seen, which is not something a sysop would think to check.
func (c Config) netmailDirs() []string {
	if c.IncomingNetmailDir != "" {
		return []string{c.IncomingNetmailDir}
	}
	return c.IncomingFileDirs
}

func parseYesNo(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "yes", "true", "on", "1":
		return true, nil
	case "no", "false", "off", "0":
		return false, nil
	}
	return false, fmt.Errorf("want Yes or No, got %q", strings.TrimSpace(value))
}

func parseLink(value, dataDir string) (int, Link, error) {
	fields := strings.Fields(value)
	// Raw is read off the end first so it composes with every mode without the
	// mode parsers having to know about it, and so BSO's optional flavor keeps
	// its own position.
	raw, rawSet := false, false
	if n := len(fields); n > 0 {
		switch {
		case strings.EqualFold(fields[n-1], "raw"):
			raw, rawSet, fields = true, true, fields[:n-1]
		case strings.EqualFold(fields[n-1], "bundled"):
			raw, rawSet, fields = false, true, fields[:n-1]
		}
	}
	if len(fields) < 2 {
		return 0, Link{}, fmt.Errorf("want <node> Attach, Obox <dir>, or BSO <dir> [flavor], each optionally followed by Raw or Bundled")
	}
	node, err := strconv.Atoi(fields[0])
	if err != nil || node < 1 || node > 999 {
		return 0, Link{}, fmt.Errorf("node %q is outside 1..999", fields[0])
	}
	link := Link{Flavor: "Normal"}
	switch strings.ToLower(fields[1]) {
	case "attach":
		if len(fields) != 2 {
			return 0, Link{}, fmt.Errorf("Attach takes no directory")
		}
		link.Mode = LinkAttach
	case "obox":
		if len(fields) != 3 {
			return 0, Link{}, fmt.Errorf("Obox wants exactly one directory")
		}
		link.Mode, link.Directory = LinkObox, fields[2]
	case "bso":
		if len(fields) < 3 || len(fields) > 4 {
			return 0, Link{}, fmt.Errorf("BSO wants a directory and optional flavor")
		}
		link.Mode, link.Directory = LinkBSO, fields[2]
		if len(fields) == 4 {
			link.Flavor = normalFlavour(fields[3])
			if link.Flavor == "" {
				return 0, Link{}, fmt.Errorf("unknown BSO flavor %q", fields[3])
			}
		}
	default:
		return 0, Link{}, fmt.Errorf("unknown mode %q", fields[1])
	}
	link.Raw, link.RawSet = raw, rawSet
	if link.Directory != "" && !filepath.IsAbs(link.Directory) {
		link.Directory = filepath.Join(dataDir, link.Directory)
	}
	return node, link, nil
}

func normalFlavour(s string) string {
	switch strings.ToLower(s) {
	case "immediate":
		return "Immediate"
	case "continuous", "crash":
		return "Continuous"
	case "direct":
		return "Direct"
	case "normal":
		return "Normal"
	case "hold":
		return "Hold"
	}
	return ""
}

// RequireNetmail reports whether this configuration can publish an attach
// handoff, and says what is missing when it cannot. It is asked by the handoff
// at the point of use rather than by LoadConfig, because the unwrap step never
// writes netmail: a
// file-box board that only RECEIVES has no netmail directory to name, and
// refusing to load its config told it to fix the one setting its runs never
// touch (found on a three-board rig, 2026-08-27).
func RequireNetmail(cfg Config, dataDir string) error {
	need := len(cfg.Links) == 0
	for _, link := range cfg.Links {
		need = need || link.Mode == LinkAttach
	}
	return netmailProblem(cfg, dataDir, need, "this board")
}

// netmailProblem names what stops an attach handoff for who, or returns nil
// when there is nothing to stop.
func netmailProblem(cfg Config, dataDir string, attach bool, who string) error {
	if !attach {
		return nil
	}
	path := filepath.Join(dataDir, store.BoardConfigFile)
	if cfg.NoNetmail {
		return fmt.Errorf("%s: %s takes an attach handoff, and %s None writes no netmail; give it a %s line with Obox or BSO",
			path, who, keyMailer, keyLink)
	}
	if cfg.NetmailDir == "" {
		return fmt.Errorf("%s: %s is not set, and %s takes an attach handoff", path, keyNetmailDir, who)
	}
	// Checked here, where netmail is written, and not at load: the unwrap step
	// never writes any, and a netmail directory that has gone missing must not
	// also stop this board receiving.
	info, err := os.Stat(cfg.NetmailDir)
	if err != nil {
		return fmt.Errorf("%s %s: %w", keyNetmailDir, cfg.NetmailDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s %s is not a directory", keyNetmailDir, cfg.NetmailDir)
	}
	return nil
}

// rawFor answers whether this peer gets plain packets: what the link says if it
// says anything, otherwise the board-wide posture. Kept in one place so a new
// call site cannot read Link.Raw directly and miss the posture (#230).
func rawFor(cfg Config, link Link) bool {
	if link.RawSet {
		return link.Raw
	}
	return !cfg.Bundled
}
