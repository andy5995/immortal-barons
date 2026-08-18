package session

import (
	"bufio"
	"io"
)

// Socket is the Session used when the BBS hands the door an already-connected
// socket instead of piping the caller through stdio. Synchronet's
// COM0:SOCKETn and Mystic's telnet mode pass the socket in the dropfile
// (DOOR32.SYS line 2); NewSocket attaches to it. Like Stdio it does no tty
// changes and translates a lone "\n" to "\r\n". The BBS performs telnet
// negotiation before launching the door, so the stream is treated as raw
// bytes (no IAC handling here).
//
// The attach is platform-specific (socket_unix.go and socket_windows.go): a
// Unix door adopts an inherited file descriptor with net.FileConn, while a
// Windows door does raw blocking winsock recv/send on the inherited handle —
// net.FileConn cannot adopt a BBS's foreign winsock socket (issue #37).
type Socket struct {
	conn io.ReadWriteCloser
	r    *bufio.Reader
}

// newSocket wraps an already-attached transport (a net.Conn on Unix, a raw
// winsock handle on Windows) as a Session.
func newSocket(conn io.ReadWriteCloser) *Socket {
	return &Socket{conn: conn, r: bufio.NewReader(conn)}
}

func (s *Socket) ReadKey() (rune, error) {
	r, _, err := s.r.ReadRune()
	return r, err
}

// ReadKeyByte returns the next raw byte, undecoded (see KeyByteReader).
func (s *Socket) ReadKeyByte() (byte, error) { return s.r.ReadByte() }

// DrainInput drops a line terminator left buffered after a single-key answer
// (see InputDrainer).
func (s *Socket) DrainInput() { drainTerminators(s.r) }

func (s *Socket) Write(p []byte) (int, error) {
	if _, err := s.conn.Write(toCRLF(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close ends the caller's connection. The no-result signature matches the
// Session-closing interface the Deadline decorator uses to boot an
// idle/timed-out caller; the underlying Close error is best-effort at teardown.
func (s *Socket) Close() { s.conn.Close() }
