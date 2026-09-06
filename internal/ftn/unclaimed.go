package ftn

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/andy5995/immortal-barons/internal/store"
)

// unclaimedAfter is how long a packet file may sit in the mailer's inbound
// directory before it is worth saying out loud. Age is measured from the change
// time, not the modification time: binkp carries the sender's timestamp and the
// mailer sets mtime from it, so a bundle queued for hours against an offline
// peer arrives already older than this and would be reported on the first look
// (see arrivedAt). A bundle and its envelope can
// arrive in either order, and an exchange runs on a schedule, so a file passed
// over once or twice is normal, so age is the signal (#236).
// Four fifteen-minute cycles is long enough that a healthy board never reports
// one.
// A var rather than a const only so a test can shorten it: ctime cannot be
// backdated, so a test file is always new.
var unclaimedAfter = time.Hour

// Unclaimed is one packet file in the mailer's inbound directory that no run
// has taken.
//
// It reports the wait and leaves the cause to the reader. Both causes met so
// far look identical from here -- an attach bundle whose envelope can never
// arrive because the board tosses netmail into its own message bases, and a
// mailer filing an unauthenticated session's files into a child directory the
// scan never enumerates (Mystic's echomail/in/unsecure) -- and so will whatever
// the next mailer does with a file it will not hand over.
type Unclaimed struct {
	Path string
	Age  time.Duration
	// Subdir is the child of InboundDir the file sits in, empty when it is in
	// InboundDir itself. Nothing scans a child, so a file there stays where it
	// is however many runs go by.
	Subdir string
}

// Where says which of the two shapes this is, in the wording both CLIs print.
func (u Unclaimed) Where() string {
	if u.Subdir != "" {
		return "in the " + u.Subdir + " subdirectory, which is not scanned"
	}
	return "not claimed by any run"
}

// scanUnclaimed reports packet files in the mailer's inbound directory, and in
// its immediate children, that nothing has claimed and that are old enough for
// the wait to mean something.
//
// One level down and no further. The point is a mailer's own quarantine or
// unsecure folder sitting beside the files it did hand over, not a general
// search: recursing into an arbitrary tree would be slow on a board whose
// inbound is also its file base, and would report things that were never meant
// for this board at all.
func scanUnclaimed(inboundDir string, claimed map[string]bool, now time.Time) []Unclaimed {
	if inboundDir == "" {
		return nil
	}
	entries, err := os.ReadDir(inboundDir)
	if err != nil {
		return nil
	}
	var out []Unclaimed
	add := func(dir, subdir string, entry os.DirEntry) {
		if !store.IsPacketFile(entry.Name()) {
			return
		}
		path := filepath.Join(dir, entry.Name())
		if claimed[cleanAbsolute(path)] {
			return
		}
		info, err := entry.Info()
		if err != nil {
			return
		}
		age := now.Sub(arrivedAt(info))
		if age < unclaimedAfter {
			return
		}
		out = append(out, Unclaimed{Path: path, Age: age, Subdir: subdir})
	}
	var subdirs []string
	for _, entry := range entries {
		if isDirectory(inboundDir, entry) {
			subdirs = append(subdirs, entry.Name())
			continue
		}
		add(inboundDir, "", entry)
	}
	for _, name := range subdirs {
		dir := filepath.Join(inboundDir, name)
		children, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range children {
			if isDirectory(dir, entry) {
				continue
			}
			add(dir, name, entry)
		}
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Age != out[b].Age {
			return out[a].Age > out[b].Age
		}
		return out[a].Path < out[b].Path
	})
	return out
}

// isDirectory follows a symlink, which ReadDir does not: a mailer's set-aside
// folder is often a link to storage elsewhere, and DirEntry.IsDir is false for
// one, so the files inside it would never be looked at.
func isDirectory(parent string, entry os.DirEntry) bool {
	if entry.IsDir() {
		return true
	}
	if entry.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(parent, entry.Name()))
	return err == nil && info.IsDir()
}

// eachInboundReceipt walks the inbound spool once and hands every readable
// receipt to fn. A directory whose journal will not parse is named in
// unreadable, which may be nil for a caller that has nothing to say about one.
func eachInboundReceipt(dataDir string, unreadable *[]string, fn func(receipt inboundReceipt, planPath string)) error {
	root := filepath.Join(dataDir, spoolDir, inSpoolDir)
	return eachSpoolDir(root, func(name, dir string) {
		planPath := filepath.Join(dir, inboundReceiptFile)
		receipt, err := loadInboundReceipt(planPath)
		if err != nil {
			if !os.IsNotExist(err) && unreadable != nil {
				*unreadable = append(*unreadable, filepath.Join(inSpoolDir, name))
			}
			return
		}
		fn(receipt, planPath)
	})
}

// addClaimed records the inbound files one receipt accounts for, so a bundle
// mid-flight through the spool is not also reported as abandoned. A receipt is
// kept until its cleanup succeeds, which means its source can still be sitting
// in the inbound directory while the transfer is perfectly healthy.
func addClaimed(claimed map[string]bool, receipt inboundReceipt) {
	for _, path := range []string{receipt.Source, receipt.Envelope} {
		if path != "" {
			claimed[cleanAbsolute(path)] = true
		}
	}
}

// claimedSources is the same answer for a caller that wants only it. The
// receipts are read from disk rather than carried out of the run that wrote
// them: they are the record of what was claimed, and a run that failed partway
// leaves the truth there rather than in memory.
func claimedSources(dataDir string) map[string]bool {
	claimed := map[string]bool{}
	_ = eachInboundReceipt(dataDir, nil, func(receipt inboundReceipt, _ string) {
		addClaimed(claimed, receipt)
	})
	return claimed
}

// isBundle reports whether the file opens with the ZIP local-header signature.
// Only a bundle carries a manifest, and a transport file may equally be a plain
// JSON packet or a truncated download, so the advice to read one has to be
// earned rather than offered to everything that failed to be claimed.
func isBundle(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	return magic == [4]byte{'P', 'K', 0x03, 0x04}
}
