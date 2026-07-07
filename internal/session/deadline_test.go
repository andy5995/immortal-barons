package session

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// fakeConn is a controllable Session: ReadKey blocks until a rune is fed or the
// connection is closed. Only the test goroutine writes to out (via the
// Deadline's warning prints), so no locking is needed.
type fakeConn struct {
	in     chan rune
	out    bytes.Buffer
	closed chan struct{}
}

func newFakeConn() *fakeConn {
	return &fakeConn{in: make(chan rune), closed: make(chan struct{})}
}

func (f *fakeConn) ReadKey() (rune, error) {
	select {
	case r := <-f.in:
		return r, nil
	case <-f.closed:
		return 0, io.EOF
	}
}
func (f *fakeConn) Write(p []byte) (int, error) { return f.out.Write(p) }
func (f *fakeConn) Close()                      { close(f.closed) }

func TestDeadlineKeypressReturns(t *testing.T) {
	f := newFakeConn()
	d := NewDeadline(f, 200*time.Millisecond, 3, time.Time{})
	go func() { f.in <- 'x' }()
	r, err := d.ReadKey()
	if r != 'x' || err != nil {
		t.Fatalf("got (%q, %v), want ('x', nil)", r, err)
	}
	if d.Reason() != "" {
		t.Errorf("reason should be empty after a normal read, got %q", d.Reason())
	}
}

func TestDeadlineIdleBoots(t *testing.T) {
	f := newFakeConn()
	d := NewDeadline(f, 40*time.Millisecond, 3, time.Time{})
	r, err := d.ReadKey() // never feed a key
	if !errors.Is(err, ErrSessionEnded) || r != 0 {
		t.Fatalf("got (%q, %v), want (0, ErrSessionEnded)", r, err)
	}
	if d.Reason() != "idle" {
		t.Errorf("reason = %q, want idle", d.Reason())
	}
	if !strings.Contains(f.out.String(), "disconnected") && !strings.Contains(f.out.String(), "inactivity") {
		t.Errorf("expected an inactivity warning in output:\n%s", f.out.String())
	}
}

func TestDeadlineStrikesBootEarly(t *testing.T) {
	f := newFakeConn()
	d := NewDeadline(f, 100*time.Millisecond, 3, time.Time{})
	d.warnings = 3 // simulate three prior idle strikes
	start := time.Now()
	_, err := d.ReadKey()
	elapsed := time.Since(start)
	if !errors.Is(err, ErrSessionEnded) || d.Reason() != "idle" {
		t.Fatalf("got (%v, reason=%q), want (ErrSessionEnded, idle)", err, d.Reason())
	}
	// With strikes maxed it boots at the warning point (idle-warnLead), not the
	// full idle deadline.
	if elapsed >= 90*time.Millisecond {
		t.Errorf("strike boot took %v, expected earlier than the full idle timeout", elapsed)
	}
}

func TestDeadlineHardDeadline(t *testing.T) {
	f := newFakeConn()
	d := NewDeadline(f, 0, 3, time.Now().Add(40*time.Millisecond)) // idle off, hard in 40ms
	_, err := d.ReadKey()
	if !errors.Is(err, ErrSessionEnded) || d.Reason() != "time" {
		t.Fatalf("got (%v, reason=%q), want (ErrSessionEnded, time)", err, d.Reason())
	}
	if !strings.Contains(f.out.String(), "time is") {
		t.Errorf("expected a time warning in output:\n%s", f.out.String())
	}
}

func TestDeadlineDisabledDelegates(t *testing.T) {
	f := newFakeConn()
	d := NewDeadline(f, 0, 3, time.Time{}) // no idle, no hard
	go func() { f.in <- 'q' }()
	if r, err := d.ReadKey(); r != 'q' || err != nil {
		t.Fatalf("disabled deadline should delegate: got (%q, %v)", r, err)
	}
}
