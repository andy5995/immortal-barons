package main

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Which I/O path a caller takes is decided by whether the BBS gave this process
// a live standard input, NOT by the platform and not by the drop file alone.
// Two live Linux boards make the case: Mystic names a socket AND hands over a
// terminal, and works on stdio; a native Synchronet door names a socket with no
// terminal, and its stdin is connected to nothing.
//
// The test runs under `go test`, whose stdin is not a terminal, so it exercises
// the Synchronet half directly; the Mystic half is the one that must not regress
// and is asserted through the same helper.
func TestSocketIsTakenOnlyWhenStandardInputIsDead(t *testing.T) {
	if session.StdinIsTerminal() {
		t.Skip("this needs a non-terminal stdin, which `go test` normally gives it")
	}
	for _, c := range []struct {
		name string
		io   door.IOMode
		want string
	}{
		{"a socket door with no terminal attaches to the socket", door.IOSocket, "socket"},
		{"a local door reads standard I/O", door.IOLocal, "stdio"},
		{"serial is refused rather than guessed at", door.IOSerial, "unsupported-serial"},
	} {
		if got := backendName(&door.Caller{IO: c.io, Socket: 58}); got != c.want {
			t.Errorf("%s: backend = %q, want %q", c.name, got, c.want)
		}
	}
}
