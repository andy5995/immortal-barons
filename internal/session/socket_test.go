//go:build unix

package session

import (
	"syscall"
	"testing"
)

// TestSocketAttachReadWrite exercises the exact path a BBS door takes: the BBS
// hands the door one end of a connected socket as an inherited file
// descriptor, the door attaches to it, reads the caller's keypresses, and
// writes ANSI back. A socketpair is a faithful local stand-in for that handoff.
func TestSocketAttachReadWrite(t *testing.T) {
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatalf("socketpair: %v", err)
	}
	doorFD, callerFD := fds[0], fds[1]
	defer syscall.Close(callerFD)

	sock, err := NewSocket(doorFD)
	if err != nil {
		t.Fatalf("NewSocket: %v", err)
	}
	defer sock.Close()
	// NewSocket dups doorFD via net.FileConn, so the original is ours to close.
	syscall.Close(doorFD)

	// The caller types a hotkey; ReadKey returns it with no Enter needed.
	if _, err := syscall.Write(callerFD, []byte("A")); err != nil {
		t.Fatalf("caller write: %v", err)
	}
	r, err := sock.ReadKey()
	if err != nil {
		t.Fatalf("ReadKey: %v", err)
	}
	if r != 'A' {
		t.Fatalf("ReadKey = %q, want 'A'", r)
	}

	// The door writes with a bare LF; the caller must receive CRLF (no
	// terminal is doing output post-processing over a socket).
	if _, err := sock.Write([]byte("Hi\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf := make([]byte, 16)
	n, err := syscall.Read(callerFD, buf)
	if err != nil {
		t.Fatalf("caller read: %v", err)
	}
	if got := string(buf[:n]); got != "Hi\r\n" {
		t.Fatalf("caller received %q, want %q", got, "Hi\r\n")
	}
}

func TestNewSocketInvalidFD(t *testing.T) {
	if _, err := NewSocket(-1); err == nil {
		t.Fatal("NewSocket(-1) should fail, got nil error")
	}
}
