package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ftn"
	"github.com/andy5995/immortal-barons/internal/store"
)

// A key no reader knows is named with its line and, when one is close, the key
// it was probably meant to be. Known keys in any case, comments and the
// transport's own keys are not flagged.
func TestUnknownBoardKeysAreNamed(t *testing.T) {
	dir := t.TempDir()
	body := "# notes\n" +
		"boardid Alpha\n" +
		"GameInbond\tin\n" +
		"; more notes\n" +
		"IncomingFileDir /mail/in\n" +
		"Colour blue\n" +
		"MAILER Binkley\n"
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got := unknownBoardKeys(dir)
	if len(got) != 2 {
		t.Fatalf("got %d warnings, want 2: %q", len(got), got)
	}
	if !strings.Contains(got[0], "line 3") || !strings.Contains(got[0], `"GameInbond"`) ||
		!strings.Contains(got[0], "did you mean GameInbound?") {
		t.Errorf("misspelled key warning = %q", got[0])
	}
	if !strings.Contains(got[1], "line 6") || !strings.Contains(got[1], `"Colour"`) ||
		strings.Contains(got[1], "did you mean") {
		t.Errorf("unrelated key warning = %q; it should name the line and suggest nothing", got[1])
	}
}

// Every key either reader knows, in any case, raises no warning.
func TestKnownBoardKeysRaiseNoWarning(t *testing.T) {
	dir := t.TempDir()
	var body strings.Builder
	for i, k := range append(store.BoardKeys(), ftn.Keys()...) {
		if i%2 == 1 {
			k = strings.ToLower(k)
		}
		body.WriteString(k + " x\n")
	}
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := unknownBoardKeys(dir); len(got) != 0 {
		t.Errorf("known keys were warned about: %q", got)
	}
}
