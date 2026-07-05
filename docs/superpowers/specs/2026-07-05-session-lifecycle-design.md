# Session lifecycle: idle timeout, time-left, connection logging (design)

Date: 2026-07-05
Status: design — awaiting review

## Why

The shared world is guarded by an **exclusive lock** (`flock`): one session
plays at a time; everyone else gets *"the game is busy."* But nothing bounds a
session's lifetime:

- `Session.ReadKey` blocks forever with no deadline.
- The web front-end's SSE stream dropping (tab closed) does **not** close the
  session, so `play.Run` stays blocked on `ReadKey`, **holding the lock until
  the server process restarts** — an abandoned tab locks everyone out.
- The door does not enforce the caller's BBS time-left in the play loop.

We want: an **idle timeout** that boots an inactive session (with a warning),
freeing the lock; the **door** additionally honoring the caller's remaining BBS
minutes; and **connection logging** in the web server's console so the operator
can see sessions come and go. All configurable.

Non-goals: multi-concurrent play (the single-lock model stays), account systems,
or reworking the front-ends beyond wiring the decorator and logging.

## Architecture

One new unit — a **session decorator** in `internal/session` — plus small
wiring in `internal/play`, the door, and the web front-end. No game-engine
changes: a boot simply makes `ReadKey` return `io.EOF`, which the menu loop and
`play.Run` already treat as a clean end (`defer lock.Release()` + world save).

### The deadline decorator (`internal/session`)

A `Session` wrapper (same pattern as the existing `MacroExpander`) that adds
deadlines to `ReadKey`:

- **Idle timeout** — resets on every keypress. If no key arrives within it, the
  session is booted.
- **Warning** — fires `warnLead = min(60s, idle/2)` before the nearer deadline;
  writes one line to the session (`⚠ You will be disconnected in 1 minute due
  to inactivity.`) and keeps waiting.
- **Idle-warning strikes** — a session-wide counter, incremented on each idle
  warning (a keypress resets the *timer* but not the *count*). When it reaches
  `maxWarnings`, the next warning point is an immediate boot
  (`Disconnected: idle too many times.`). This stops a player from parking on
  the lock and tapping a key each minute to dodge the timeout.
- **Hard deadline** (optional, absolute wall-clock) — for the door's BBS
  time-left. A single "your time is almost up" warning `warnLead` before it,
  then boot at the deadline. Independent of the idle strikes.

Mechanics of `ReadKey`: run the underlying read in a goroutine, `select` on its
result against the idle timer, the warning timer, and (if set) the hard-deadline
timer. On a boot it writes the final line, `Close()`s the underlying session (to
unblock the pending read), drains the goroutine, and returns `io.EOF`. It
records the **reason** (`idle` / `time`) so callers can log it; a raw `EOF` from
the underlying read (a real disconnect) is reported as `disconnect`, and a clean
menu Quit as `quit`.

Constructor sketch:

```go
func NewDeadline(s Session, idle time.Duration, maxWarnings int, hard time.Time) *Deadline
func (d *Deadline) Reason() string   // "", "idle", or "time" once it has booted
```

`idle == 0` disables the idle timeout; a zero `hard` disables the hard deadline.

### Config (per-board, not league-broadcast)

Two operational fields on `game.Config` (like `AICount` — a board setting, not a
league rule, so **not** in the `LeagueConfig` broadcast):

- `IdleTimeoutSecs int` — default **180** (3 min); `0` disables.
- `MaxIdleWarnings int` — default **3**.

Both appear in the Configuration Editor (unmarked, i.e. board-local fields), and
in `DefaultConfig`.

### Wiring

- **`play.Run`** wraps the raw session in the decorator (idle/maxWarnings from
  `cfg`, hard deadline from the caller) *before* `GameLoop` wraps it in the
  `MacroExpander`, so the decorator measures real input idle time. After
  `GameLoop` returns, `play.Run` determines the end reason (decorator's
  `Reason()`, else `disconnect` on `io.EOF`, else `quit`) and returns it
  alongside the error.
- **`play.Identity`** gains `TimeLeft time.Duration` (0 = unlimited). The **door**
  fills it from the dropfile's remaining-minutes field (surfaced by
  `internal/door` — parse it there if it isn't already) so the decorator's hard
  deadline is `now + TimeLeft`. Local terminal and web leave it 0.
- **`play.Run` return** becomes `(endReason string, err error)`; the three
  front-ends update to the new signature (barons-web logs the reason; the others
  ignore it).

### Connection logging (web console)

In `cmd/barons-web`, using the standard `log` package (timestamps on):

- **Session start** — when a new `WebSession` is created:
  `log.Printf("session connected from %s", r.RemoteAddr)`.
- **Session end** — when `runGame` returns:
  `log.Printf("session from %s ended (%s)", addr, endReason)` where `endReason`
  is `quit` / `idle` / `time` / `disconnect`.
- **Stream drop** — the stream handler's `r.Context().Done()`:
  `log.Printf("session from %s stream disconnected", addr)` (informational; the
  game keeps running until the idle timeout boots it).

The secret session id is never logged; the remote address is the correlation
key. `runGame` needs the address, so it is passed in (`runGame(id, ws, addr)`).

## The lock-leak, resolved

The idle timeout **is** the lock-leak fix: an abandoned tab is booted after
`IdleTimeoutSecs` → `io.EOF` → `play.Run` returns → lock released + world saved.
We deliberately do **not** close the session on SSE stream-drop, because the
browser's `EventSource` reconnects on transient network blips and that would
boot active players; the idle timeout is the robust, front-end-agnostic fix.

## Testing

- **Decorator unit tests** (fake session): (1) a keypress before the deadline
  returns normally and resets the idle timer; (2) no key past the idle deadline
  returns `io.EOF` with `Reason()=="idle"` and closes the underlying session;
  (3) the warning is written `warnLead` before the boot; (4) after `maxWarnings`
  idle warnings the next warning boots immediately; (5) a hard deadline boots
  with `Reason()=="time"`. Use small durations (e.g. 30–60 ms) for speed.
- **play.Run end-reason** unit test with a scripted session that quits vs one
  that idles, asserting the returned reason.
- **Config** defaults test (`IdleTimeoutSecs==180`, `MaxIdleWarnings==3`).
- Existing suites stay green (the change is additive; a `0` idle timeout in a
  test config disables the decorator).

## Implementation slices

1. **Decorator + tests** — `session.Deadline`, no wiring yet.
2. **Config fields** — `IdleTimeoutSecs`, `MaxIdleWarnings` in `Config`,
   `DefaultConfig`, and the Configuration Editor.
3. **Wire `play.Run`** — wrap the session, thread `Identity.TimeLeft`, return
   the end reason; update the three front-ends' call sites.
4. **Door time-left** — surface the dropfile's remaining minutes and pass it as
   `Identity.TimeLeft`.
5. **Web logging** — connect / end(reason) / stream-drop lines in `barons-web`.

## Open decisions

- **Local terminal idle timeout.** With the default config the local
  (`cmd/barons`) game also boots after 3 min idle. That is fine for a shared
  BBS-style world, but a solo local player might find it abrupt. Option if it
  annoys: `cmd/barons` could pass `idle=0` (disabled) since a local player holds
  no one else's lock in practice. Left enabled by default for now; easy to flip.
