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

// idleTiming is the idle timeout for the two tests that must land a keypress in
// a SPECIFIC phase of the deadline — after the warning at idle/2, before the
// boot at idle. What makes those reliable is the margin around the target
// instant, not the absolute duration, so the timeout is deliberately generous
// and each sleep is written as a fraction of it rather than as its own literal.
//
// At the original 80ms the margin was 25ms, which a loaded runner eats: the
// FreeBSD job runs in a nested VM over SSH and is the jitteriest of the set, and
// it failed here twice (2026-08-19, 2026-08-25) with the boot beating the key.
// Scaling by ten costs about half a second and removes the race; asserting
// elapsed time instead is what made a sibling flaky, so don't reach for that.
const idleTiming = 800 * time.Millisecond

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
	_, err := d.ReadKey()
	if !errors.Is(err, ErrSessionEnded) || d.Reason() != "idle" {
		t.Fatalf("got (%v, reason=%q), want (ErrSessionEnded, idle)", err, d.Reason())
	}
	// With strikes maxed it boots AT the warning point instead of warning and
	// waiting out the rest of the idle timeout — so the tell is that no warning
	// was printed. Asserting the elapsed time instead is what made this flaky:
	// the boot is due at idle/2 and a loaded CI VM took 98.5ms of a 100ms
	// budget to get there (FreeBSD, 2026-08-19), which says nothing about the
	// code under test.
	if strings.Contains(f.out.String(), "You will be disconnected") {
		t.Errorf("a maxed-out striker should be booted without another warning:\n%s", f.out.String())
	}
}

func TestDeadlineActiveResponseResetsStrikes(t *testing.T) {
	f := newFakeConn()
	// Warning falls at idleTiming/2; respond well before that.
	d := NewDeadline(f, idleTiming, 3, time.Time{})
	d.warnings = 2 // two prior strikes
	go func() { time.Sleep(idleTiming / 8); f.in <- 'x' }()
	r, err := d.ReadKey()
	if err != nil || r != 'x' {
		t.Fatalf("got (%q, %v), want ('x', nil)", r, err)
	}
	if d.warnings != 0 {
		t.Errorf("strikes should reset when the player responds before a warning, got %d", d.warnings)
	}
}

func TestDeadlineRestoresInputLineAfterWarning(t *testing.T) {
	f := newFakeConn()
	// warnLead is idleTiming/2, so the warning lands at idleTiming/2 and the
	// boot at idleTiming.
	d := NewDeadline(f, idleTiming, 3, time.Time{})
	d.SetInputLine("Amount: 45")
	// Feed a key after the warning fires but before the boot, so ReadKey
	// returns cleanly and we can inspect what was written. 7/10 sits near the
	// middle of that window, leaving a fifth of the timeout of slack on each
	// side.
	go func() {
		time.Sleep(idleTiming * 7 / 10)
		f.in <- '6'
	}()
	r, err := d.ReadKey()
	if err != nil || r != '6' {
		t.Fatalf("got (%q, %v), want ('6', nil)", r, err)
	}
	out := f.out.String()
	warn := strings.Index(out, "inactivity")
	restore := strings.LastIndex(out, "Amount: 45")
	if warn == -1 {
		t.Fatalf("expected an inactivity warning:\n%s", out)
	}
	if restore == -1 || restore < warn {
		t.Errorf("input line should be reprinted after the warning:\n%s", out)
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
