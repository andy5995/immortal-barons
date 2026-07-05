package session

import (
	"bufio"
	"fmt"
	"net"
	"os"
)

// Socket is the Session used when the BBS hands the door an already-connected
// socket instead of piping the caller through stdio. Synchronet's
// COM0:SOCKETn and Mystic's telnet mode pass the socket's descriptor in the
// dropfile (DOOR32.SYS line 2); NewSocket attaches to it. Like Stdio it does
// no tty changes and translates a lone "\n" to "\r\n". The BBS performs telnet
// negotiation before launching the door, so the stream is treated as raw
// bytes (no IAC handling here).
type Socket struct {
	conn net.Conn
	r    *bufio.Reader
}

// NewSocket attaches to the socket the BBS passed as an inherited descriptor
// (a file descriptor on Unix; the winsock handle on Windows). net.FileConn
// duplicates the descriptor, so the returned Session owns its own copy and the
// descriptor the BBS handed us is left untouched.
func NewSocket(fd int) (*Socket, error) {
	f := os.NewFile(uintptr(fd), "bbs-socket")
	if f == nil {
		return nil, fmt.Errorf("invalid BBS socket descriptor %d", fd)
	}
	conn, err := net.FileConn(f)
	f.Close() // FileConn duplicated the descriptor; release our os.File copy
	if err != nil {
		return nil, fmt.Errorf("attach to BBS socket %d: %w", fd, err)
	}
	return &Socket{conn: conn, r: bufio.NewReader(conn)}, nil
}

func (s *Socket) ReadKey() (rune, error) {
	r, _, err := s.r.ReadRune()
	return r, err
}

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
