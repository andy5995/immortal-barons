package store

import (
	"os"
	"path/filepath"
)

// BadDir is where an inbound packet file waits when it could not be parsed at
// all — corrupt JSON, a truncated transfer, or a foreign file dropped in the
// wrong directory. Unlike HeldDir, nothing here is expected to become
// readable later on its own: it stays for a sysop to look at, and
// releaseHeld never touches it.
const BadDir = "bad"

// quarantinePacket moves an unparseable inbound file aside so the rest of the
// batch is not blocked by it (#178). A failure to move it is reported to the
// caller for the same reason holdPacket's is: leaving the file in the inbound
// directory would fail the run the same way, in the same place, on every
// later run.
func quarantinePacket(dataDir, path string) error {
	dir := filepath.Join(dataDir, BadDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return moveFile(path, filepath.Join(dir, filepath.Base(path)))
}
