package session

import "io"

// WebSession is a Session driven over HTTP for the browser front-end:
// keystrokes arrive on `in` (fed by the web handler from the browser) and
// output bytes go to `out` (drained by the Server-Sent Events stream). It is
// the browser equivalent of Console (local tty) and Stdio (BBS door).
//
// The channels are buffered so short bursts don't block; a closed session
// unblocks a blocked ReadKey/Write via `done` so the game goroutine can exit
// cleanly when the browser goes away.
type WebSession struct {
	in   chan rune
	out  chan []byte
	done chan struct{}
}

// NewWebSession creates an idle web session.
func NewWebSession() *WebSession {
	return &WebSession{
		in:   make(chan rune, 64),
		out:  make(chan []byte, 1024),
		done: make(chan struct{}),
	}
}

// ReadKey blocks until a keystroke arrives or the session is closed (EOF).
func (w *WebSession) ReadKey() (rune, error) {
	select {
	case r := <-w.in:
		return r, nil
	case <-w.done:
		return 0, io.EOF
	}
}

// Write queues bytes for the browser, copying p (the engine reuses buffers). It
// blocks if the browser is far behind and errors once the session is closed, so
// the game loop stops rather than spinning.
func (w *WebSession) Write(p []byte) (int, error) {
	b := make([]byte, len(p))
	copy(b, p)
	select {
	case w.out <- b:
		return len(p), nil
	case <-w.done:
		return 0, io.ErrClosedPipe
	}
}

// Feed delivers a browser keystroke. It never blocks the HTTP handler: if the
// input buffer is full (player typing faster than the game reads), the key is
// dropped.
func (w *WebSession) Feed(r rune) {
	select {
	case w.in <- r:
	default:
	}
}

// Out is the channel the SSE handler drains to stream bytes to the browser.
func (w *WebSession) Out() <-chan []byte { return w.out }

// Done is closed when the session ends; the SSE handler watches it to stop.
func (w *WebSession) Done() <-chan struct{} { return w.done }

// Close ends the session, unblocking a pending ReadKey/Write. Idempotent.
func (w *WebSession) Close() {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
}
