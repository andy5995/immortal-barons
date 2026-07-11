# Door Concurrency Foundation — Design Spec

**Issue:** #5 (door front-end is single-caller-at-a-time, not concurrent multi-node)

**Status:** design, pending Andy's review. No code written yet.

> Drafted by Claude (Opus 4.8) at Andy's direction. Every decision below was
> made by Andy in the brainstorming dialogue; prior art (LORD, TW2002) was read
> for mechanism only and is not copied into the repo.

## Goal

Let multiple separate door processes (one per BBS node) play the same JSON
world **concurrently**, instead of one caller holding the world flock for the
whole session and bouncing everyone else with "the game is busy." The web
front-end (a long-lived in-memory owner) must keep working unchanged.

This is the **foundation**. Live "who's online" presence and live inter-node
interaction (online attacks/messages) are explicitly **out of scope here** and
become follow-up issues layered on this foundation. They will be poll-based
against the shared file — **no background daemon** (the TW2002 EXTERN model was
considered and rejected in #5; Andy confirmed a daemon is not needed).

## Decisions (locked in the brainstorming dialogue)

1. **Full file-per-action refactor** (not "document single-caller", not a
   blocking-wait queue).
2. **Scope = concurrency foundation only**; presence + interaction are separate
   follow-up issues.
3. **No background process** — the file is the source of truth; future presence
   is poll-based.
4. **Architecture = a `Store` transaction abstraction behind `ctx.With`**; the
   door-vs-web difference lives in one `Store` implementation.
5. **Whole-file exclusive lock per transaction** (IB's world is one JSON blob;
   LORD's record-level locking is not adopted).

## 1. The `Store` transaction abstraction

A small interface (new, `internal/game` or `internal/store`):

```go
type Store interface {
	Transact(fn func()) error // run fn with exclusive access to the world
}
```

- **In-memory Store (web, and any single-owner process):** `w.mu.Lock()` →
  `fn()` → `w.mu.Unlock()`. No file I/O. The web path is behaviorally unchanged.
- **File Store (door / `-local`):** acquire the flock (blocking) →
  `json.Unmarshal` the world file **into the existing `*World`** (so the
  pointer the `ctx` holds stays valid; re-run the `Ensure*` migrations) →
  `fn()` → `store.Save` → release the flock.

`ctx.With(fn)` delegates to the store's `Transact`. This is the *only* place
the two front-ends diverge.

## 2. Active empire by handle

`ctx` replaces `active *game.Empire` with `handle string`. `w.Player()` becomes
`w.FindByOwner(handle)`, so it re-resolves the active empire from whatever world
is currently loaded. Called **inside** a transaction it returns the
freshly-reloaded empire; called for display outside a transaction it returns the
cached (possibly stale) copy — acceptable, and BBS-conventional.

## 3. The atomic-operation rule (core correctness pattern)

**Each game operation performs its gather + re-validate + mutate inside a single
`Transact` call.** Actions that today read state outside `With` and mutate
inside must move the precondition check into the write transaction. Example: Buy
Regions re-checks the player's gold and the per-turn region cap inside the same
`Transact` that deducts gold and adds regions — not before it.

If a precondition now fails because another node changed the world (e.g. the
gold was spent, the target empire died), the operation **aborts with a short
"the realm has changed — try again" notice and makes no partial mutation**.

This re-validation coverage — every mutating action, in one transaction each —
is the bulk of the work and the primary correctness risk. The implementation
plan enumerates every mutating action from the menu tree and gives each a
"concurrent conflict" test.

## 4. Maintenance & onboarding coordination

- **Date-rollover maintenance** on login runs inside a `Transact`, guarded on
  `LastMaintDate` (re-checked after the reload) so exactly one node runs the
  day's maintenance even if several log in at once.
- **New-player onboarding** creates the empire inside a `Transact`, re-checking
  the realm name and board-full state after the reload (extends the #40 TOCTOU
  fix), so no duplicate realm slips in between two simultaneous first-time
  callers.

## 5. Front-end wiring

- **Door and `-local`** use the File Store, so a local player and door nodes
  coexist. The per-session flock hold in `internal/play` (`Run`) is **removed** —
  the flock is held only *inside* each `Transact`, briefly, so nodes interleave
  between actions.
- **Web** uses the in-memory Store; unchanged.
- **`-maint` / `-planetary`** keep their existing whole-run blocking lock (batch,
  non-interactive); they are not per-action sessions.
- The session-end single `save()` in `play.Session` is removed — every action
  already persists through its transaction.

## 6. Locking granularity

Whole-file exclusive lock per transaction. Held only for the brief
reload → mutate → save, then released, so other nodes proceed between actions.
Finer (record-level) locking is a possible later optimization, not needed at
hobby node counts.

## 7. Testing

- **In-process:** two `ctx` instances with File Stores over one temp world file,
  interleaving operations, asserting no double-spend and correct re-validation
  (each atomic operation gets a "concurrent conflict" test — the second actor's
  operation aborts cleanly when the first changed the precondition).
- **Cross-process integration:** a Go test that `exec`s two `-local`-style runs
  against one data directory with scripted input, because Go's `-race` detector
  sees only in-process races, not cross-process file races.
- Existing web concurrency tests (from #36) must stay green (web path unchanged).

## Risks

1. **Re-validation must cover every mutating action** — a missed one is a silent
   race (double-spend, attack a dead empire). Mitigation: enumerate all mutating
   actions in the plan; one conflict test each.
2. **More I/O** — reload+save the whole JSON per transaction. Acceptable for a
   small world and a handful of nodes; batchable later if needed.
3. **Cross-process correctness is hard to fully test** — the integration test
   covers the common interleavings; careful per-action review covers the rest.

## Out of scope (follow-up issues)

- Live "who's online" presence (poll-based, no daemon).
- Live inter-node interaction (online attacks/messages surfacing mid-session).
- Record-level / finer-grained locking.
- Any background/resident process (explicitly rejected).
