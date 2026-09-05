package session

import (
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// ErrSessionEnded marks a read that failed because the session is actually over
// — an idle/time boot or a dropped connection — as opposed to a test input
// stream simply running out. Input helpers unwind the whole turn only on this,
// not on a bare io.EOF. It wraps io.EOF so existing io.EOF checks still match.
var ErrSessionEnded = errors.New("session ended")

func ended(cause error) error {
	if cause == nil {
		cause = io.EOF
	}
	return fmt.Errorf("%w: %w", ErrSessionEnded, cause)
}

// Deadline wraps a Session and bounds how long it may sit idle (and, optionally,
// its total wall-clock time). It boots an inactive session by returning io.EOF
// from ReadKey — which the menu loop and play.Run already treat as a clean end,
// so the world is saved and the exclusive lock released.
//
//   - idle: reset on every keypress. No key within it -> a warning, then a boot.
//   - maxWarnings: idle warnings accumulate across the session (a keypress
//     resets the timer, not the count); once that many have shown, the next
//     idle warning boots immediately. Stops a player parking on the lock and
//     tapping a key each minute to dodge the timeout.
//   - hard: an absolute deadline (0 = none), e.g. a door caller's BBS time-left;
//     a single warning before it, then a boot. Independent of the idle strikes.
type Deadline struct {
	inner       Session
	idle        time.Duration
	warnLead    time.Duration
	maxWarnings int
	hard        time.Time

	warnings   int
	timeWarned bool
	reason     string

	// inputLine holds the text of the prompt line the caller is editing
	// (prompt prefix + typed value). ReadKey reprints it after an interrupting
	// warning so the cursor lands back at the end of the value. Stored
	// atomically: the reading goroutine sets it while ReadKey blocks in another.
	inputLine atomic.Value // string
}

// InputLineSetter is implemented by Deadline and forwarded by the session
// wrappers. A prompt registers the line it is editing so an interrupting
// idle/time warning can restore that line, and the cursor, afterwards.
type InputLineSetter interface {
	SetInputLine(line string)
}

// SetInputLine registers (or, with "", clears) the current prompt line to
// restore after a warning. Safe to call while ReadKey blocks concurrently.
func (d *Deadline) SetInputLine(line string) { d.inputLine.Store(line) }

func (d *Deadline) currentInputLine() string {
	if v := d.inputLine.Load(); v != nil {
		return v.(string)
	}
	return ""
}

// NewDeadline wraps inner. idle<=0 disables the idle timeout; a zero hard
// disables the hard deadline. The warning lead is min(60s, idle/2) so it stays
// sane for short timeouts.
func NewDeadline(inner Session, idle time.Duration, maxWarnings int, hard time.Time) *Deadline {
	lead := 60 * time.Second
	if idle > 0 && idle/2 < lead {
		lead = idle / 2
	}
	return &Deadline{inner: inner, idle: idle, warnLead: lead, maxWarnings: maxWarnings, hard: hard}
}

func (d *Deadline) Write(p []byte) (int, error) { return d.inner.Write(p) }

// UTF8, ASCII and ANSI forward the inner session's capability markers. Go
// promotes only the Session interface's own methods, so a wrapper that does not
// say these explicitly makes IsUTF8 report UTF-8 and IsASCII report "not ASCII"
// for every caller behind it — silently, since both have a defaulting answer.
// The charset writer sits INSIDE this wrapper, so without them nothing above can
// see the caller's real charset.
func (d *Deadline) UTF8() bool { return IsUTF8(d.inner) }

// Column forwards the cursor column from a tracker below (see ColumnTracker).
func (d *Deadline) Column() int { n, _ := Column(d.inner); return n }
func (d *Deadline) ASCII() bool { return IsASCII(d.inner) }
func (d *Deadline) ANSI() bool  { return HasANSI(d.inner) }

// DrainInput forwards to the inner session (see InputDrainer). Safe to call
// between reads — no ReadKey goroutine is in flight once ReadKey has returned.
func (d *Deadline) DrainInput() { Drain(d.inner) }

// Reason reports why the session ended: "idle", "time", or "" if it has not
// booted (a clean quit or a raw disconnect leaves it "").
func (d *Deadline) Reason() string { return d.reason }

type keyResult struct {
	r   rune
	err error
}

func (d *Deadline) ReadKey() (rune, error) {
	if d.idle <= 0 && d.hard.IsZero() {
		r, err := d.inner.ReadKey()
		if err != nil {
			d.reason = "disconnect"
			return r, ended(err)
		}
		return r, nil
	}

	ch := make(chan keyResult, 1)
	go func() { r, err := d.inner.ReadKey(); ch <- keyResult{r, err} }()

	var idleBoot time.Time
	if d.idle > 0 {
		idleBoot = time.Now().Add(d.idle)
	}
	bootAt, reason := earlier(idleBoot, "idle", d.hard, "time")

	warned := false
	for {
		target := bootAt
		if !warned {
			target = bootAt.Add(-d.warnLead)
		}
		wait := time.Until(target)
		if wait < 0 {
			wait = 0
		}
		timer := time.NewTimer(wait)
		select {
		case res := <-ch:
			timer.Stop()
			if res.err != nil {
				if d.reason == "" {
					d.reason = "disconnect"
				}
				return res.r, ended(res.err)
			}
			if !warned {
				// Responded before any warning fired: actively playing, not
				// dodging the timeout. Reset the strike count so only repeated
				// last-second dodges accumulate toward a no-warning boot.
				d.warnings = 0
			}
			return res.r, nil
		case <-timer.C:
			if warned {
				return d.boot(ch, reason)
			}
			// First hit: a warning (or an immediate boot if idle strikes are up).
			if reason == "idle" {
				if d.warnings >= d.maxWarnings {
					return d.boot(ch, "idle")
				}
				d.warnings++
				fmt.Fprintf(d.inner, "\r\n\x1b[93m** You will be disconnected in %s due to inactivity. **\x1b[0m\r\n", humanLead(d.warnLead))
			} else {
				if d.timeWarned {
					// Already warned about time; just wait out the deadline.
					warned = true
					continue
				}
				d.timeWarned = true
				fmt.Fprintf(d.inner, "\r\n\x1b[93m** Your time is almost up (%s left). **\x1b[0m\r\n", humanLead(d.warnLead))
			}
			// The warning was printed on its own lines, stranding the cursor
			// below the prompt. Reprint the line the caller was editing so the
			// cursor returns to the end of their typed value.
			if line := d.currentInputLine(); line != "" {
				fmt.Fprint(d.inner, line)
			}
			warned = true
		}
	}
}

// boot writes the final line, closes the underlying session (to unblock its
// pending read), records the reason, and reports EOF to end the game.
func (d *Deadline) boot(ch chan keyResult, reason string) (rune, error) {
	d.reason = reason
	if reason == "idle" {
		fmt.Fprint(d.inner, "\r\n\x1b[91mDisconnected due to inactivity.\x1b[0m\r\n")
	} else {
		fmt.Fprint(d.inner, "\r\n\x1b[91mYour time is up.\x1b[0m\r\n")
	}
	if c, ok := d.inner.(interface{ Close() }); ok {
		c.Close()
	}
	// Best-effort drain of the read goroutine (it returns once Close lands).
	select {
	case <-ch:
	case <-time.After(100 * time.Millisecond):
	}
	return 0, ended(nil)
}

// earlier returns the sooner of two deadlines (ignoring zero times) with its
// label. At least one is expected to be non-zero.
func earlier(a time.Time, aLabel string, b time.Time, bLabel string) (time.Time, string) {
	switch {
	case a.IsZero():
		return b, bLabel
	case b.IsZero():
		return a, aLabel
	case b.Before(a):
		return b, bLabel
	default:
		return a, aLabel
	}
}

// humanLead renders the warning lead as "1 minute" / "30 seconds".
func humanLead(d time.Duration) string {
	if d >= time.Minute {
		m := int(d / time.Minute)
		if m == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", m)
	}
	return fmt.Sprintf("%d seconds", int(d/time.Second))
}
