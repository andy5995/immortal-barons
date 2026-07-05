# Concurrent Web World — Design

**Status:** approved design, pending implementation plan
**Date:** 2026-07-05
**Author:** drafted by Claude (Opus 4.8) at Andy's direction

## Goal

Let up to N players (config knob, default 4) share **one live web world** at the
same time. Because every web session runs inside a single server process
touching one in-memory `World`, cross-player data is always current: a
scoreboard, an empire's stats, or the land market reflect other players'
actions the moment you look at them. No packets, no polling, no staleness.

A web-hosted world is **web-only**. When the server runs it owns that world for
its whole lifetime; the door, `-maint`, and the local front-end do not touch
the same `world.json` concurrently. This is the simplifying decision that makes
the project small: there is no cross-process coordination to design, only
in-process concurrency.

## Non-goals (explicitly out of scope)

- **True multi-node door.** The door keeps its current model: one OS process
  per caller, a whole-session exclusive flock, one player at a time. Unchanged.
- **IBBS / inter-planetary / gooie co-funding on the web.** These solve a
  network-partition problem — separate BBSes with no shared real-time state —
  that a single web server does not have. On one shared world they are
  redundant (a "group attack" is just an attack; "cross-board scores" is just
  the scoreboard). They remain door/BBS-world concepts. Gooie co-funding, which
  was an IBBS-era idea, leaves scope entirely.
- **Active mid-turn push / interrupts.** No "** New mail **" shoved onto a
  player's screen mid-keystroke, no live-ticking counters. Liveness is
  *fresh-on-navigate*: current data appears whenever a player returns to a menu
  or opens a screen. Notifications (mail, events) continue to surface at their
  existing points (session start via `Empire.Events`), reading current state.
- **Inter-Web federation** (multiple web servers gossiping). Low value; if ever
  wanted it would use a live HTTP API, not the packet system. Not now.

## Current state (what we are changing)

- `play.Run` acquires a **whole-session** exclusive flock (`store.Lock(cfg,
  false)`) held via `defer` for the entire session. Consequence: exactly one
  player total, across all front-ends, can be in a session at once; a second
  gets "the game is busy." This serialized single-writer model is what makes
  the JSON world impossible to corrupt — and it is exactly what we relax, for
  the web front-end only.
- `game.World` has an `Active *Empire` field (`json:"-"`) — the empire playing
  "this session." It is the one piece of shared state that assumes a single
  player. It is read through `w.Player()` and, for language, `playerLang(w)` →
  `w.Player().Language`.
- Game logic (combat, covert ops, economy, `GooieKablooie`) already takes
  empires as **explicit arguments**; it is not entangled with "whose turn it
  is." Only the menu layer's notion of the current empire depends on `Active`.
- The web front-end (`cmd/barons-web`) is already a single process with a
  `hub` holding a `sessions` map; each session runs `play.Run` in its own
  goroutine. Today they cannot overlap only because `play.Run` takes the flock.
- Daily maintenance runs **per session** inside `play.Run` (`w.DailyMaintenance(today)`
  right after load).

## Architecture

The web server becomes the **sole owner** of one long-lived `*World`. Sessions
are goroutines that share it. Concurrency is guarded by a single mutex on the
`World`, with a strict discipline (below). Per-session state moves out of the
`World` and into the per-session menu context.

### Component changes

1. **`game.World` — mutex + remove `Active`.**
   - Add an unexported `mu sync.Mutex` (with `json:"-"` semantics — unexported
     fields are not marshaled, so no tag needed).
   - Remove the `Active *Empire` field and `w.Player()`. The "current empire"
     is no longer world state; it is session state.
   - Expose lock helpers used by the menu layer, e.g. `func (w *World) Lock()`,
     `func (w *World) Unlock()` (or a `func (w *World) With(fn func())` wrapper).
   - Adjust the two internal users of `Active`:
     - `league.go` `Reset()` no longer sets `w.Active = nil`.
     - `game.go` `RemoveEmpire` no longer nils `Active`; the caller's session
       context handles a removed empire.

2. **Per-session context — carry the active empire.**
   - The menu engine already wraps the session per-player in `langSession`
     (`menu.go`, set in `menu.Run`). Extend that wrapper (or a small sibling
     `sessionCtx`) to hold `active *Empire`.
   - Replace `w.Player()` call sites and `playerLang(w)` with reads from the
     session context. Grep surface today: `menu/helpbrowse.go`,
     `menu/menu.go`, `menu/tree.go`, and `play.go:73` (which sets `w.Active`).
   - The menu action signatures stay `(s session.Session, w *game.World)`; the
     active empire is reachable from `s` (the `langSession`/context wrapper),
     the same way language already is.

3. **Locking discipline — the crux.**
   **Never hold `World.mu` while blocked on player input.** A menu action often
   prompts mid-way (`ReadLine`, the multi-line message editor), where a player
   may sit for minutes; a lock spanning that prompt re-serializes everyone and
   defeats the whole project. The pattern for every mutating action:

   ```
   gather all input (UNLOCKED)
   → Lock()
   → re-validate preconditions (gold still sufficient? target still alive?)
   → mutate
   → Unlock()
   ```

   Re-validation is required because the world can change while a player types
   (their gold was spent by another action of theirs elsewhere; the empire they
   are attacking just died). On a failed re-check the action refuses cleanly
   ("You no longer have enough gold.") and returns to the menu — no panic, no
   partial mutation.

   Reads (scoreboards, status, an empire's stats) take the lock briefly,
   **snapshot** the values they need into locals, `Unlock()`, then render from
   the snapshot. Rendering (which can be slow / block on the writer) happens
   unlocked.

   This discipline touches each mutating handler in `menu/actions.go`. That is
   the real cost of the project and where implementation effort concentrates.

4. **Web hub — concurrent-session cap.**
   - Add a config knob `MaxConcurrentSessions` (default 4). Distinct from
     `MaxPlayers` (total empires that may ever enroll) — this caps *live*
     sessions.
   - When a new SSE connection would exceed the cap, the hub declines it with
     "The realm is full — try again shortly." and does not start a game
     goroutine.

5. **World lifecycle owned by the server.**
   - **Startup:** acquire the exclusive flock once (`store.Lock(cfg, false)`);
     if busy, fail fast with a clear message ("another process holds this world
     — a web-hosted world cannot be shared with a door"). Load the world into
     memory. Hold the flock for the server's lifetime.
   - **Sessions:** `play.Run` (or a web-specific variant) no longer takes the
     flock or loads/saves per session. It receives the shared `*World` and the
     lock helpers, resolves/onboards the caller's empire into the session
     context, and runs the menu loop.
   - **Persistence:** save to disk on each **turn-commit** (a natural, already
     bounded point) and on server **shutdown**. Saving takes the world lock
     briefly to snapshot/marshal.
   - **Daily maintenance:** run `DailyMaintenance` on **date rollover**, driven
     by a background ticker in the server (guarded by the world lock), not
     per-session as today. Sessions no longer call `DailyMaintenance`
     themselves against the shared world.

### What the door / local front-ends keep

`cmd/barons-door` and `cmd/barons` are unchanged: they still call the existing
`play.Run` with its whole-session flock. The refactor of `Active` into session
context must keep those paths working (a single-player session is just N=1).
The mutex adds negligible cost to a one-player session.

## Data flow (concurrent turn)

```
Player A goroutine                     Player B goroutine
------------------                     ------------------
render scoreboard:                     mid-turn: buy 10 regions
  Lock(); snapshot scores; Unlock()      gather count from prompt (UNLOCKED)
  write table to A's session             Lock()
                                          re-check gold & region cap
Player A: attack empire "Zorg"            apply purchase; adjust market land
  choose target, forces (UNLOCKED)        Unlock()
  Lock()                                  render result to B
  re-check: is Zorg still alive?
  apply combat losses to both            (A's next scoreboard render now
  Unlock()                                shows B's new land — fresh, free)
  render battle report to A
```

The shared in-memory world makes B's purchase visible to A's next read with no
extra machinery. The lock only ever protects a short mutate-or-snapshot window.

## Error handling

- **Failed re-validation under lock:** the action reports a clear message and
  returns to the menu. No partial state; the lock is released in all paths
  (defer `Unlock()` immediately after `Lock()` where the mutation is
  self-contained, or explicit unlock before rendering).
- **Session cap exceeded:** SSE connection declined with a message; no goroutine
  started; no lock touched.
- **Startup flock busy:** server exits with a clear message rather than starting
  a half-owned world.
- **Save failure:** logged to the server console; the in-memory world remains
  authoritative and the next commit retries. A save failure does not crash live
  sessions.

## Testing

- **Race detector is the headline.** `go test -race` with a test that spawns N
  goroutines driving scripted fake sessions against one shared `World`, then
  asserts: no data race, and a consistent final state — e.g. two players buying
  land, total gold spent reconciles, region counts and market land are
  internally consistent, no gold goes negative.
- **Re-validate-under-lock path:** a player gathers input, another action spends
  the gold, the first player confirms → action refused cleanly, world unchanged.
- **Session cap:** the (cap+1)-th connection is declined; existing sessions
  continue.
- **Single-player regression:** existing `play.Run` / menu tests still pass with
  `Active` moved to session context (N=1 is just the degenerate case).
- **Maintenance on rollover:** advancing the date triggers exactly one
  `DailyMaintenance`, guarded by the lock, not once per active session.

## Risks & open points

- **The locking-across-input trap** is the single biggest correctness risk; the
  gather→lock→re-validate→mutate→unlock discipline is mandatory, not advisory.
  Reviewers should check every mutating handler for a lock that spans a prompt.
- **Refactor breadth of `actions.go`:** each mutating handler is touched. The
  change is mechanical but broad; it is the bulk of the work.
- **`play.Run`'s dual role:** it currently does lock + load + maintenance +
  session. The web path needs those responsibilities split out (server owns
  lock/load/maintenance; a session-run function owns onboarding + menu loop),
  while the door/local path keeps the combined behavior. Cleanly separating
  these without duplicating logic is the main structural judgment call for the
  implementation plan.
