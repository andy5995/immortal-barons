//go:build windows

package session

import (
	"net"
	"testing"

	"golang.org/x/sys/windows"
)

// TestWinsockSocketRoundTrip exercises the real NewSocket winsock path (issue
// #37). It hands NewSocket a FOREIGN connected handle — a raw x/sys/windows
// client socket that Go's net poller never registered, standing in for a BBS's
// inherited socket — then reads a keypress through the Session and writes a
// line back to a Go peer. This is exactly the case net.FileConn cannot adopt
// (WSAEINVAL). It only builds/runs on Windows; run it there or via
// `GOOS=windows go test -c ./internal/session` executed under Wine.
func TestWinsockSocketRoundTrip(t *testing.T) {
	if err := ensureWinsock(); err != nil {
		t.Fatalf("WSAStartup: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	peerCh := make(chan net.Conn, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		peerCh <- c
	}()

	// Foreign client handle: raw winsock, never seen by Go's poller.
	cfd, err := windows.Socket(windows.AF_INET, windows.SOCK_STREAM, windows.IPPROTO_TCP)
	if err != nil {
		t.Fatalf("socket: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := windows.Connect(cfd, &windows.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}, Port: port}); err != nil {
		windows.Closesocket(cfd)
		t.Fatalf("connect: %v", err)
	}
	peer := <-peerCh
	defer peer.Close()

	// The door adopts the foreign handle via the real production path.
	sock, err := NewSocket(int(cfd))
	if err != nil {
		windows.Closesocket(cfd)
		t.Fatalf("NewSocket: %v", err)
	}
	defer sock.Close() // owns cfd now

	// Peer -> door: a hotkey arrives as one ReadKey.
	if _, err := peer.Write([]byte("y")); err != nil {
		t.Fatalf("peer write: %v", err)
	}
	r, err := sock.ReadKey()
	if err != nil {
		t.Fatalf("ReadKey: %v", err)
	}
	if r != 'y' {
		t.Fatalf("ReadKey = %q, want 'y'", r)
	}

	// Door -> peer: Write translates a lone \n to \r\n on the wire.
	if _, err := sock.Write([]byte("ok\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	buf := make([]byte, 16)
	n, err := peer.Read(buf)
	if err != nil {
		t.Fatalf("peer read: %v", err)
	}
	if got := string(buf[:n]); got != "ok\r\n" {
		t.Fatalf("peer got %q, want %q", got, "ok\r\n")
	}
}
