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

// Config contains settings local to the FTN transport.
type Config struct {
	NetmailDir string
	Binkley    bool
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
	return cfg, nil
}
