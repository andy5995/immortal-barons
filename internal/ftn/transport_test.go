package ftn

import (
	"path/filepath"
	"strings"
	"testing"
)

// A Synchronet door conventionally sits at <sbbs>/xtrn/<door>/data, which the
// sysop can place anywhere but which the usual layout nests deep -- and either
// way it is not ours to shorten. #226's default attach spool (dataDir/ftn-spool/
// attach) spent enough of the fixed Type-2 Subject budget that a board which
// published before #226 stopped publishing entirely after it, with no config
// change of its own: captured live on a three-board rig (#231), the resulting
// subject was 73 bytes against a 70-byte Binkley limit.
func TestAttachSubjectFitsOnADeepSynchronetDataDir(t *testing.T) {
	const dataDir = "/home/sysop/c-sbbs/xtrn/immortal-barons/data"
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

// #232 review: checkSubjectMargin repeats its warning on every run for as
// long as a board stays within subjectMarginBytes, unlike fileAttachSubject's
// error, which fires once. Reusing subjectAdvice's full ~550-byte explanation
// for that recurring warning was correct but trains a sysop to stop reading
// it. Proves the warning stays short across all three SubjectMode values,
// reached through a real checkSubjectMargin call rather than the pointer
// function directly -- the same discipline as
// TestSubjectPrefixedIgnoresAttachDirsOwnLength, since a sysop only ever
// sees what the real call path produces.
func TestSubjectMarginWarningStaysShort(t *testing.T) {
	cases := []struct {
		name      string
		cfg       Config
		attached  string
		wantWords string // must appear
		wantNot   string // must not appear -- only the full subjectAdvice text says this
		wantDocs  bool   // must cite docs/ftn-transport.md
		shorter   bool   // must be strictly shorter than subjectAdvice's full text
	}{
		{"absolute", Config{Binkley: true}, strings.Repeat("x", 65),
			"shorten AttachDir", "this is what actually shortens it", true, true},
		{"prefixed", Config{Binkley: true, SubjectMode: SubjectPrefixed, SubjectPrefix: strings.Repeat("p", 60) + "/"},
			"/dir/f.brp", "shorten the SubjectPath prefix", "point AttachDir at", true, true},
		// Basename's full advice is already one short sentence, so the
		// warning is that same text verbatim -- no doc pointer to add, and
		// nothing shorter to say.
		{"basename", Config{Binkley: true, SubjectMode: SubjectBasename},
			"/dir/" + strings.Repeat("f", 62) + ".brp", "no SubjectPath setting can shorten it", "", false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var result Result
			if err := checkSubjectMargin(c.cfg, c.attached, &result); err != nil {
				t.Fatalf("checkSubjectMargin: %v", err)
			}
			if len(result.Warnings) != 1 {
				t.Fatalf("Warnings = %v, want exactly one thin-margin warning", result.Warnings)
			}
			warning := result.Warnings[0]
			if !strings.Contains(warning, c.wantWords) {
				t.Errorf("warning does not name the fix: %q", warning)
			}
			if c.wantNot != "" && strings.Contains(warning, c.wantNot) {
				t.Errorf("warning contains %q, which belongs only to the full hard-error explanation: %q",
					c.wantNot, warning)
			}
			if c.wantDocs && !strings.Contains(warning, "docs/ftn-transport.md") {
				t.Errorf("warning drops the pointer to the full reasoning: %q", warning)
			}
			// checkSubjectMargin wraps whichever text this returns in its own
			// "N byte(s) to spare" sentence, so the size comparison is against
			// subjectMarginPointer's own output, not the full formatted
			// warning -- the wrapping prefix would make every case "longer"
			// regardless of what this function contributes.
			pointer := subjectMarginPointer(c.cfg.SubjectMode)
			full := subjectAdvice(c.cfg.SubjectMode)
			switch {
			case c.shorter && len(pointer) >= len(full):
				t.Errorf("pointer text (%d bytes) is not shorter than the full advice (%d bytes) it replaces",
					len(pointer), len(full))
			case !c.shorter && pointer != full:
				t.Errorf("pointer text = %q, want the full advice unchanged: %q", pointer, full)
			}
			if !strings.Contains(warning, pointer) {
				t.Errorf("warning does not contain subjectMarginPointer's own text: %q vs %q", warning, pointer)
			}
		})
	}
}
