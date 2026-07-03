# Slice 2 — Persistence / multi-user door game (design)

Date: 2026-07-03
Status: design (pending review)

## Background

Immortal Barons runs as a native BBS door (slice 1): it reads a dropfile
and plays over stdio. But each call is a fresh, throwaway game. Slice 2
turns it into a real door game: a **persistent, shared world** that many
callers dip into over time.

BRE is not real-time multiplayer. It is **asynchronous and
one-session-at-a-time**: while you play, you hold the world exclusively;
every other baron — human or AI — is a **stored empire record** that you
act upon. Victims learn what happened on their next login. Another caller
cannot enter until you quit.

## Decisions (from brainstorming)

1. **Maintenance trigger — hybrid.** A `barons-door -maint` command for the
   sysop's nightly event, plus lazy catch-up: on login, if the date rolled
   over since the last maintenance, run it (looping to catch up missed days).
2. **Concurrency — exclusive lock.** One active session at a time via a lock
   file. A second caller is told the game is busy and exits.
3. **World makeup — configurable AI, default 0.** Human-only out of the box;
   the sysop may add AI barons.
4. **Turn model — per-turn economy; idle empires stagnate.** An empire's
   economy processes only when its owner plays a turn. Daily maintenance is
   global bookkeeping only.
5. **Lifecycle — endless world.** No fixed game length in slice 2. Fixed-
   length leagues + reset + Planetary Master are a later slice.
6. **Async feedback — minimal event log.** A per-empire "while you were away"
   log. The full daily news bulletin and player-to-player messages come later.

## Goals / scope

Build the persistent multi-user foundation: save/load, exclusive locking,
the per-turn vs daily-maintenance split, per-player turns and protection,
new-player onboarding, the event log, and a shared session orchestration
used by both front-ends. Endless world; config from a JSON file with
sane defaults (the interactive sysop setup is slice 3).

## Data model (`internal/game`)

Replace the fixed-length single-player world with a persistent one.

**Config** (loaded from `<dataDir>/config.json`, else defaults):

```go
type Config struct {
    TurnsPerDay     int    // default 10
    ProtectionTurns int    // default 20
    AICount         int    // default 0
    DataDir         string // default "./data"
}
```

**World** — add `GameDay int` and `LastMaintDate string` (ISO `YYYY-MM-DD`);
carry `Config`. Remove `MaxTurns` and `GameOver()` (endless). Net-worth
ranking stays.

**Empire** — add:

```go
Owner      string   // normalized BBS handle; "" for AI
TurnsLeft  int      // turns remaining today
Protection int      // protection turns remaining
LastPlayed string   // ISO date
Events     []string // "while you were away" log
```

`Human` is implied by `Owner != ""`. `NewWorld(cfg)` creates a world with
`cfg.AICount` AI barons (from a name pool) and no human empires; humans are
added at onboarding.

## Turn / maintenance split (`turn.go`)

`EndTurn()` (which today processes every empire + AI + advances one turn)
splits in two:

- **`PlayTurn(e *Empire)`** — the current per-empire economy (taxes, bank
  interest, food, maintenance, growth, money caps) acting on **only `e`**.
  Decrements `e.TurnsLeft`; decrements `e.Protection` while it is > 0.
- **`DailyMaintenance()`** — global, idempotent, self-catching-up. Compute
  `today` (machine local date). While `LastMaintDate < today`, run one pass:
  1. Refill every empire's `TurnsLeft = Config.TurnsPerDay`.
  2. If `AICount > 0`, run each AI empire's turns (reuse today's `aiActions`
     over its turn allotment).
  3. Remove dead empires (`Land <= 0` or `People <= 0`).
  4. `GameDay++`; advance `LastMaintDate` by exactly one day.

  If `LastMaintDate == today` on entry it is a no-op. Running it once catches
  up any number of missed days, so callers just call it — no external loop.

Attack results are recorded on the victim: `Attack(a, d)` appends a short
summary line to `d.Events` (so both human and AI attacks notify the victim).

Targeting changes: replace `Rivals()` with `Targets(attacker)` returning
alive empires that are not the attacker and not under protection
(`Protection == 0`). An empire with `Protection > 0` cannot be attacked and
cannot initiate attacks.

## Persistence + locking (`internal/store`)

- **Save/Load** the `World` as JSON at `<dataDir>/world.json`. Save writes
  `world.json.tmp` then renames (atomic). Load returns a fresh
  `NewWorld(cfg)` if the file is absent.
- **Lock** on `<dataDir>/game.lock` via `syscall.Flock`:
  - Interactive play uses `LOCK_EX|LOCK_NB` (fail-fast): on `EWOULDBLOCK`,
    report the game is busy and exit.
  - `-maint` uses blocking `LOCK_EX`: the nightly event waits for any active
    player to finish, then runs, rather than being turned away.

  The lock is released (and fd closed) on exit.

Stdlib only (`encoding/json`, `os`, `syscall`).

## Session orchestration (`internal/play`)

A new package both front-ends call, so the flow lives in one place. It
imports `game`, `menu`, `store`, and `session` (no import cycle — it sits
above them).

```go
type Identity struct { Handle string } // from dropfile or -name

func Run(s session.Session, id Identity, cfg game.Config) error
```

`Run`:
1. Acquire the exclusive lock; if busy, tell the user and return.
2. Load the world (create fresh if none).
3. Lazy maintenance: call `DailyMaintenance()` (it self-catches-up any
   missed days).
4. Find the empire whose `Owner` matches `id.Handle` (normalized). If none,
   onboard: prompt for a realm name (min 3 alphanumerics, not matching an
   existing empire — the slice-1 rule), create the empire with starting
   resources, `Protection = cfg.ProtectionTurns`, `TurnsLeft =
   cfg.TurnsPerDay`, `Owner = id.Handle`.
5. If the empire has `Events`, display them, then clear the slice.
6. Run the menu. "End Turn" calls `PlayTurn`; when `TurnsLeft == 0`, further
   turn-ending is refused ("out of turns for today") and only view/quit
   remain.
7. Save the world; release the lock.

Menu changes: the status bar shows `GameDay` and `TurnsLeft` (not
`Turn X/MaxTurns`); `nextTurn` calls `PlayTurn` and enforces the turn
limit instead of calling the removed `GameOver()`.

## Commands

- `barons-door` — parse dropfile → `Identity{Handle: caller.Handle}` →
  `play.Run`.
- `barons-door -maint` — non-interactive: acquire lock, load, run
  `DailyMaintenance` (catch-up loop), save, unlock. For the sysop's nightly
  BBS event.
- `cmd/barons` — gains `-name` (identity, default `$USER` or `sysop`) and
  `-data` (data dir) so local play uses the same persistent store.

## Onboarding, turns, protection

- Starting resources: the current `New()` player values (gold, food, land,
  people, troopers, one carrier).
- Protection counts down one per turn played; while `> 0` the empire cannot
  be attacked or attack others. Reflects the original's "New Realm
  Protection".
- When `TurnsLeft` hits 0, the player can still view status/scores and read
  events, but not end another turn until the next daily maintenance.

## Testing

- **store**: save/load round-trip preserves the world; the temp-then-rename
  write leaves no partial file; a second `Flock` on the same file fails
  while the first is held.
- **game**: `PlayTurn` changes only the acting empire and decrements its
  turns/protection; `DailyMaintenance` is a no-op when already run today,
  refills turns, and catches up multiple missed days; a protected empire is
  absent from `Targets` and rejects attacks; an attack appends to the
  victim's `Events`.
- **play**: with a scripted `Session` and a temp data dir — a new handle
  onboards and persists; the same handle resumes its empire on a second
  `Run`; the event log is shown then cleared; a busy lock is reported.

## Out of scope (later slices)

Fixed-length leagues, reset, and Planetary Master; the full daily news
bulletin and player-to-player messages; the socket I/O backend (Synchronet
`COM0:SOCKETn` / Mystic telnet); the interactive sysop configuration editor
(slice 3 — slice 2 reads a JSON config with defaults).
