// Package ftn hands Immortal Barons inter-BBS packets to an FTN mailer by
// creating FTS-0001 file-attach netmail messages.
package ftn

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
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

	var cfg Config
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
	if cfg.NetmailDir == "" {
		return Config{}, fmt.Errorf("%s: NetmailDir is not set", path)
	}
	if !filepath.IsAbs(cfg.NetmailDir) {
		cfg.NetmailDir = filepath.Join(dataDir, cfg.NetmailDir)
	}
	info, err := os.Stat(cfg.NetmailDir)
	if err != nil {
		return Config{}, fmt.Errorf("NetmailDir %s: %w", cfg.NetmailDir, err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("NetmailDir %s is not a directory", cfg.NetmailDir)
	}
	if cfg.AttachDir != "" && !filepath.IsAbs(cfg.AttachDir) {
		cfg.AttachDir = filepath.Join(dataDir, cfg.AttachDir)
	}
	return cfg, nil
}
