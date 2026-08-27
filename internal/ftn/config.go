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
	// AttachDir overrides the fido child of each outbound directory as the
	// place claimed packets are moved to. Empty keeps the per-outbound child.
	AttachDir     string
	SubjectMode   SubjectMode
	SubjectPrefix string
	// InboundDir is the mailer's receive directory. InboundNetmailDir is where
	// received stored-message envelopes are found; empty means InboundDir.
	InboundDir        string
	InboundNetmailDir string
	Links             map[int]Link
	OboxMeshFanout    bool
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
}

// fidoSubdir is the child of an outbound directory that claimed packets are
// moved into when AttachDir is unset.
const fidoSubdir = "fido"

// attachPath is where a claimed packet is written on this filesystem.
func (c Config) attachPath(outboundDir, name string) string {
	if c.AttachDir != "" {
		return filepath.Join(c.AttachDir, name)
	}
	return filepath.Join(outboundDir, fidoSubdir, name)
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
	if len(fields) < 2 {
		return 0, Link{}, fmt.Errorf("want <node> Attach, Obox <dir>, or BSO <dir> [flavour]")
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
