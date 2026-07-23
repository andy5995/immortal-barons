//go:build windows

package session

import (
	"fmt"
	"io"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// NewSocket adopts the winsock handle the BBS passed in DOOR32.SYS line 2. The
// door does raw blocking recv/send on the inherited handle rather than
// os.NewFile+net.FileConn: that adopt assumes a Go-created overlapped socket
// registered with Go's I/O completion port, but a BBS's inherited socket is
// neither, and net.FileConn rejects it with WSAEINVAL (issue #37). Blocking
// recv/send work on any connected handle regardless of the runtime poller —
// which is what a native door needs.
func NewSocket(fd int) (*Socket, error) {
	if fd == 0 {
		return nil, fmt.Errorf("invalid BBS socket handle %d", fd)
	}
	if err := ensureWinsock(); err != nil {
		return nil, fmt.Errorf("winsock init for BBS socket %d: %w", fd, err)
	}
	h := windows.Handle(fd)
	// Some BBSes (MagicBBS) hand the door a socket already in non-blocking mode.
	// Prefer blocking (recv then just waits), but a socket the BBS registered
	// with WSAAsyncSelect/WSAEventSelect — the classic GUI-BBS pattern — can
	// NEVER be switched back: ioctlsocket(FIONBIO, 0) fails with WSAEINVAL
	// (issue #37, reproduced under Wine). So this is best-effort only; when the
	// socket stays non-blocking, winsockConn rides out WSAEWOULDBLOCK by
	// waiting for readiness (winsock select) and retrying.
	setBlocking(h)
	return newSocket(&winsockConn{h: h}), nil
}

// fionbio is ioctlsocket's command to toggle non-blocking mode: a nonzero argp
// enables it, zero restores blocking. x/sys/windows exposes neither the command
// nor ioctlsocket, so we call ws2_32 directly.
const fionbio = 0x8004667e

var (
	modws2_32       = windows.NewLazySystemDLL("ws2_32.dll")
	procIoctlsocket = modws2_32.NewProc("ioctlsocket")
	procSelect      = modws2_32.NewProc("select")
)

// setBlocking clears non-blocking mode on an inherited socket via
// ioctlsocket(FIONBIO, 0). ioctlsocket returns 0 on success, SOCKET_ERROR on
// failure; on an already-blocking handle it is a harmless no-op. Best-effort:
// an async-selected socket rejects this with WSAEINVAL (see NewSocket).
func setBlocking(h windows.Handle) error {
	var mode uint32 // 0 = blocking
	r, _, err := procIoctlsocket.Call(uintptr(h), fionbio, uintptr(unsafe.Pointer(&mode)))
	if int32(r) != 0 { // SOCKET_ERROR
		return err
	}
	return nil
}

// fdSet mirrors winsock fd_set: u_int fd_count; SOCKET fd_array[FD_SETSIZE].
// (fd_array is 8-byte aligned after the 4-byte count on win64, matching Go's
// own field alignment here.)
type fdSet struct {
	count uint32
	array [64]windows.Handle
}

// waitReady blocks until the socket is readable (or, forWrite, writable) via
// winsock select() with a nil timeout. select — not WSAPoll — on purpose:
// WSAPoll rejects an inherited handle with WSAENOTSOCK under Wine, while
// select works on any valid socket there and on real Windows alike (verified
// with the wine-37 harness). A ready-but-errored socket is left for the
// following recv/send to surface as its own error.
func waitReady(h windows.Handle, forWrite bool) error {
	fs := fdSet{count: 1}
	fs.array[0] = h
	var rp, wp uintptr
	if forWrite {
		wp = uintptr(unsafe.Pointer(&fs))
	} else {
		rp = uintptr(unsafe.Pointer(&fs))
	}
	r, _, err := procSelect.Call(0, rp, wp, 0, 0) // first arg ignored; nil timeout = block
	if int32(r) < 0 {                             // SOCKET_ERROR
		return err
	}
	return nil
}

// ensureWinsock initializes winsock once for the process. On Windows the door
// does not import net (only the Unix socket path does), so nothing else calls
// WSAStartup; WSACleanup is skipped — the OS reclaims it at process exit.
var (
	wsaOnce sync.Once
	wsaErr  error
)

func ensureWinsock() error {
	wsaOnce.Do(func() {
		var d windows.WSAData
		wsaErr = windows.WSAStartup(0x202, &d)
	})
	return wsaErr
}

// winsockConn is an io.ReadWriteCloser over an inherited winsock handle using
// blocking WSARecv/WSASend (nil OVERLAPPED). A zero-length recv means the peer
// closed the connection, reported as io.EOF to match net.Conn semantics.
type winsockConn struct{ h windows.Handle }

func (c *winsockConn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		buf := windows.WSABuf{Len: uint32(len(p)), Buf: &p[0]}
		var n, flags uint32
		err := windows.WSARecv(c.h, &buf, 1, &n, &flags, nil, nil)
		if err == windows.WSAEWOULDBLOCK {
			// Non-blocking socket (async-selected by the BBS, issue #37): wait
			// until readable, then retry — behaves like a blocking recv.
			if werr := waitReady(c.h, false); werr != nil {
				return 0, werr
			}
			continue
		}
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return int(n), nil
	}
}

func (c *winsockConn) Write(p []byte) (int, error) {
	// WSASend may accept fewer bytes than offered; loop until all are sent.
	total := 0
	for total < len(p) {
		chunk := p[total:]
		buf := windows.WSABuf{Len: uint32(len(chunk)), Buf: &chunk[0]}
		var n uint32
		err := windows.WSASend(c.h, &buf, 1, &n, 0, nil, nil)
		if err == windows.WSAEWOULDBLOCK {
			// Send buffer full on a non-blocking socket: wait for writability.
			if werr := waitReady(c.h, true); werr != nil {
				return total, werr
			}
			continue
		}
		if err != nil {
			return total, err
		}
		total += int(n)
	}
	return total, nil
}

func (c *winsockConn) Close() error { return windows.Closesocket(c.h) }
