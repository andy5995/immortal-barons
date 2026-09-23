package ftn

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/store"
)

// legacyKeys maps each ftn.cfg keyword to its bbs.cfg name. Binkley is not
// here: its Yes/No became a Mailer line.
var legacyKeys = map[string]string{
	"inboundnetmaildir": keyIncomingNetmailDir,
	"inbounddir":        keyIncomingFileDir,
	"netmaildir":        keyOutgoingNetmailDir,
	"attachdir":         keyAttachDir,
	"subjectpath":       keySubjectPath,
	"link":              keyLink,
	"bundled":           keyBundled,
	"oboxmeshfanout":    keyOboxMeshFanout,
}

// LegacyRefusal reports a <dataDir>/ftn.cfg left from when the transport was a
// separate program, with its settings rewritten as bbs.cfg lines ready to
// paste. Nil when there is no such file.
//
// The file is refused rather than read: two files that can both name the same
// setting leave a sysop editing the one that no longer counts. The values are
// copied byte for byte, so a Windows path keeps its backslashes.
func LegacyRefusal(dataDir string) error {
	path := filepath.Join(dataDir, LegacyConfigFile)
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		key, value, _ := store.SplitKey(line)
		if strings.EqualFold(key, "Binkley") {
			if on, err := parseYesNo(value); err == nil && on {
				lines = append(lines, keyMailer+" Binkley")
			}
			continue
		}
		if name, ok := legacyKeys[strings.ToLower(key)]; ok && value != "" {
			lines = append(lines, name+" "+value)
		}
	}
	msg := fmt.Sprintf("%s is no longer read; its settings now live in %s.", path,
		filepath.Join(dataDir, store.BoardConfigFile))
	if len(lines) > 0 {
		msg += " Add these lines to bbs.cfg:\n  " + strings.Join(lines, "\n  ") + "\nthen delete ftn.cfg."
	} else {
		msg += " It holds no settings; delete it."
	}
	return fmt.Errorf("%s\nRemove barons-ftn from your scheduler and mailer hooks as well: -maint and -planetary now do its work.", msg)
}
