package ftn

import (
	"path/filepath"
	"testing"
)

// A Synchronet install's data directory is fixed at xtrn/<door>/data -- the
// sysop cannot shorten it. #226's default attach spool (dataDir/ftn-spool/
// attach) spent enough of the fixed Type-2 Subject budget that a board which
// published before #226 stopped publishing entirely after it, with no config
// change of its own: captured live on a three-board rig (#231), the resulting
// subject was 73 bytes against a 70-byte Binkley limit.
func TestAttachSubjectFitsOnADeepSynchronetDataDir(t *testing.T) {
	const dataDir = "/home/andy/c-sbbs/xtrn/immortal-barons/data"
	const filename = "255U0000.BRP"

	transport := Config{Binkley: true}
	attached := filepath.Join(attachmentDirectory(dataDir, transport), filename)

	_, spare, err := fileAttachSubject(transport, attached)
	if err != nil {
		t.Fatalf("attach subject for the #231 rig no longer fits: %v", err)
	}
	if spare < subjectMarginBytes {
		t.Errorf("spare = %d bytes, want at least the %d-byte warning threshold -- "+
			"this rig would still trip the thin-margin warning on its first run",
			spare, subjectMarginBytes)
	}
}
