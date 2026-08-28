// Package ftn bridges Immortal Barons packet directories to stored-message
// file attach, direct obox, and BSO/FLO mailer handoffs.
package ftn

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ConfigFile is deliberately separate from bbs.cfg. The latter describes the
// board and the game's transport-neutral packet directories; this file belongs
// only to the optional FTN transport.
const ConfigFile = "ftn.cfg"

// SubjectMode selects how an attachment is spelled in the Type-2 Subject. The
// spelling is resolved by the mailer, so it is configured separately from the
// directory the file is actually written to: only the operator knows the
// mailer's working directory and attachment search path.
type SubjectMode int

const (
	// SubjectAbsolute writes the full pathname, as every release before this
	// setting existed did. It is what an unconfigured ftn.cfg still gets.
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
	Binkley    bool
	// AttachDir is where a claimed packet is written for an Attach or BSO
	// link. Empty defaults to attachmentDirectory's own default (dataDir's
	// att child, see transport.go) -- this field no longer has any notion
	// of a per-outbound-directory child; that was retired with the
	// bundled transport (#231).
	AttachDir     string
	SubjectMode   SubjectMode
	SubjectPrefix string
	// InboundDir is the mailer's receive directory. InboundNetmailDir is where
	// received stored-message envelopes are found; empty means InboundDir.
	InboundDir        string
	InboundNetmailDir string
	Links             map[int]Link
	OboxMeshFanout    bool
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
	Flavour   string
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

// LoadConfig reads <dataDir>/ftn.cfg. Unknown keys are ignored so a newer
// handler can extend the file without preventing an older one from running.
func LoadConfig(dataDir string) (Config, error) {
	path := filepath.Join(dataDir, ConfigFile)
	f, err := os.Open(path)
	if err != nil {
		return Config{}, err
	}
	defer f.Close()

	cfg := Config{OboxMeshFanout: true, Links: map[int]Link{}}
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
		switch {
		case strings.EqualFold(key, "NetmailDir"):
			cfg.NetmailDir = strings.TrimSpace(value)
		case strings.EqualFold(key, "AttachDir"):
			cfg.AttachDir = strings.TrimSpace(value)
		case strings.EqualFold(key, "InboundDir"):
			cfg.InboundDir = strings.TrimSpace(value)
		case strings.EqualFold(key, "InboundNetmailDir"):
			cfg.InboundNetmailDir = strings.TrimSpace(value)
		case strings.EqualFold(key, "Bundled"):
			b, err := parseYesNo(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s: Bundled: %w", path, err)
			}
			cfg.Bundled = b
		case strings.EqualFold(key, "OboxMeshFanout"):
			b, err := parseYesNo(value)
			if err != nil {
				return Config{}, fmt.Errorf("%s: OboxMeshFanout: %w", path, err)
			}
			cfg.OboxMeshFanout = b
		case strings.EqualFold(key, "Link"):
			node, link, err := parseLink(value, dataDir)
			if err != nil {
				return Config{}, fmt.Errorf("%s: Link: %w", path, err)
			}
			cfg.Links[node] = link
		case strings.EqualFold(key, "SubjectPath"):
			value = strings.TrimSpace(value)
			switch {
			case strings.EqualFold(value, "Absolute"):
				cfg.SubjectMode = SubjectAbsolute
			case strings.EqualFold(value, "Basename"):
				cfg.SubjectMode = SubjectBasename
			default:
				cfg.SubjectMode = SubjectPrefixed
				cfg.SubjectPrefix = value
			}
		case strings.EqualFold(key, "Binkley"):
			value = strings.TrimSpace(value)
			switch {
			case strings.EqualFold(value, "yes"), strings.EqualFold(value, "true"), value == "1":
				cfg.Binkley = true
			case strings.EqualFold(value, "no"), strings.EqualFold(value, "false"), value == "0":
				cfg.Binkley = false
			default:
				return Config{}, fmt.Errorf("%s: Binkley must be Yes or No", path)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return Config{}, err
	}
	if cfg.NetmailDir != "" && !filepath.IsAbs(cfg.NetmailDir) {
		cfg.NetmailDir = filepath.Join(dataDir, cfg.NetmailDir)
	}
	if cfg.NetmailDir != "" {
		info, err := os.Stat(cfg.NetmailDir)
		if err != nil {
			return Config{}, fmt.Errorf("NetmailDir %s: %w", cfg.NetmailDir, err)
		}
		if !info.IsDir() {
			return Config{}, fmt.Errorf("NetmailDir %s is not a directory", cfg.NetmailDir)
		}
	}
	if cfg.AttachDir != "" && !filepath.IsAbs(cfg.AttachDir) {
		cfg.AttachDir = filepath.Join(dataDir, cfg.AttachDir)
	}
	for _, field := range []*string{&cfg.InboundDir, &cfg.InboundNetmailDir} {
		if *field != "" && !filepath.IsAbs(*field) {
			*field = filepath.Join(dataDir, *field)
		}
	}
	if cfg.InboundNetmailDir == "" {
		cfg.InboundNetmailDir = cfg.InboundDir
	}
	return cfg, nil
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
	// mode parsers having to know about it, and so BSO's optional flavour keeps
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
		return 0, Link{}, fmt.Errorf("want <node> Attach, Obox <dir>, or BSO <dir> [flavour], each optionally followed by Raw or Bundled")
	}
	node, err := strconv.Atoi(fields[0])
	if err != nil || node < 1 || node > 999 {
		return 0, Link{}, fmt.Errorf("node %q is outside 1..999", fields[0])
	}
	link := Link{Flavour: "Normal"}
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
			return 0, Link{}, fmt.Errorf("BSO wants a directory and optional flavour")
		}
		link.Mode, link.Directory = LinkBSO, fields[2]
		if len(fields) == 4 {
			link.Flavour = normalFlavour(fields[3])
			if link.Flavour == "" {
				return 0, Link{}, fmt.Errorf("unknown BSO flavour %q", fields[3])
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
// handoff, and says what is missing when it cannot. It is asked by --out at the
// point of use rather than by LoadConfig, because --in never writes netmail: a
// file-box board that only RECEIVES has no netmail directory to name, and
// refusing to load its config told it to fix the one setting its runs never
// touch (found on a three-board rig, 2026-08-27).
func RequireNetmail(cfg Config, dataDir string) error {
	need := len(cfg.Links) == 0
	for _, link := range cfg.Links {
		need = need || link.Mode == LinkAttach
	}
	if need && cfg.NetmailDir == "" {
		return fmt.Errorf("%s: NetmailDir is not set, and this board has an attach handoff to make",
			filepath.Join(dataDir, ConfigFile))
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
