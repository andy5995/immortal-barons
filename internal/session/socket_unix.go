//go:build !windows

package session

import (
	"fmt"
	"net"
	"os"
)

// NewSocket attaches to the socket the BBS passed as an inherited file
// descriptor. net.FileConn duplicates the descriptor, so the returned Session
// owns its own copy and the descriptor the BBS handed us is left untouched.
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
	return newSocket(conn), nil
}
