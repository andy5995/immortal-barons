package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BadDir is where an inbound packet file waits when it could not be parsed at
// all — corrupt JSON, a truncated transfer, or a foreign file dropped in the
// wrong directory. Unlike HeldDir, nothing here is expected to become
// readable later on its own: it stays for a sysop to look at, and
// releaseHeld never touches it. Nothing empties it automatically — see
// docs/inter-bbs-troubleshooting.md's "Quarantined packets" section.
const BadDir = "bad"

// quarantineGrace is how long a file that fails to parse is left alone
// before ReadInbound ever quarantines it. A mailer that writes straight to
// the final name instead of a temp-then-rename dance — scp, plain FTP, and
// several FTN mailers all do this — leaves a planetary run that lands
// mid-transfer reading truncated JSON. Quarantining that permanently loses
// a packet that would have applied cleanly on the very next run, which is a
// worse trade than the blocked batch quarantining exists to avoid: a file
// young enough to plausibly still be arriving is left in inbound instead,
// to be re-read once the write finishes.
const quarantineGrace = 5 * time.Minute

// maxQuarantineCopies bounds how many same-named copies uniqueName will
// step around before giving up. Without a cap, a neighbour whose transport
// keeps redelivering one broken file under the same name accumulates
// pkt.brp, pkt.2.brp, pkt.3.brp without limit, and every new arrival
// re-stats every copy already there — the cost of quarantining one more
// file grows with how many already have been. Past this many, the sender
// is malfunctioning badly enough that failing loudly is more useful than
// silently taking on unbounded disk and CPU.
const maxQuarantineCopies = 1000

// quarantinePacket moves an unparseable inbound file aside so the rest of the
// batch is not blocked by it (#178). A failure to move it is reported to the
// caller for the same reason holdPacket's is: leaving the file in the inbound
// directory would fail the run the same way, in the same place, on every
// later run.
//
// A second corrupt file arriving under the same name — a mailer retrying a
// bad transfer, say — must not collide with the first already sitting in
// BadDir: os.Rename silently overwrites an existing destination on Unix
// (destroying the sysop's first copy to inspect) and fails outright on
// Windows, which would fail this move and, propagated up, abort the whole
// batch — exactly the failure quarantining exists to avoid. uniqueName steps
// around the collision instead of hitting it.
func quarantinePacket(dataDir, path string) error {
	dir := filepath.Join(dataDir, BadDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	dst, err := uniqueName(filepath.Join(dir, filepath.Base(path)))
	if err != nil {
		return err
	}
	return moveFile(path, dst)
}

// uniqueName returns base if nothing is there yet, otherwise base with a
// counter spliced in before the extension, incremented until it names
// nothing that already exists — capped at maxQuarantineCopies so a
// malfunctioning sender cannot make one file's quarantine attempt scan an
// unbounded and ever-growing list of its own past copies.
func uniqueName(base string) (string, error) {
	if _, err := os.Stat(base); os.IsNotExist(err) {
		return base, nil
	} else if err != nil {
		return "", err
	}
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	for i := 2; i <= maxQuarantineCopies; i++ {
		candidate := fmt.Sprintf("%s.%d%s", stem, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("more than %d quarantined copies of %s already exist",
		maxQuarantineCopies, filepath.Base(base))
}
