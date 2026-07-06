# News & Events System — Design

Date: 2026-07-06
Status: approved (design)

## Goal

Give Immortal Barons a full daily news system: a **Daily Bulletin** header of
planet-wide totals with day-over-day change, a **broadened planet news feed**
(economic, political, military, civil), a real **Today / Yesterday** split, and
**random per-empire events** that surface in each player's "while you were away"
log. Reconstructed from BRE's behavior; all wording is our own (no verbatim
copy of BRE's text).

## Background

BRE layers three distinct systems, gathered via the `bre-gather` method:

1. **Daily Bulletin** — a boxed header of planet aggregates (Total Population,
   Total Regions, Total Net Worth), each with a day-over-day Change.
2. **Daily news feed** — narrative lines below the header: bank-rate moves,
   Planetary-Master retention/change, attacks, WMD strikes, pirate raids, riots.
3. **Random per-empire events** — a sysop-editable file (`events.dat`) of
   "since your last play" flavor events (gain/lose units/food/people).

Immortal Barons today has only a slice of #2 (`World.Bulletin`, holding
combat/WMD/pirate/riot lines) and a per-empire event log (`Empire.Events`,
"while you were away"). It has no aggregate header, no day-over-day change, no
Today/Yesterday split, and no random events.

## Global Constraints

- Go 1.26, standard library preferred. Deterministic tests via `game.NewSeed`
  and the world RNG (`w.rng`); no wall-clock in game logic.
- Keep tunable numbers as Go constants (per the project convention). Random
  events are **hardcoded in Go**, not a data file (sysop-editability may come
  later as its own slice).
- Never copy BRE display text verbatim; all news/event wording is original.
- Concurrency: news/event generation happens inside `DailyMaintenance`, which
  runs under the world lock; renderers read under the caller's lock.
- The world JSON is pre-release; a light in-place migration is acceptable, a
  save-format version bump is not required.

## Data model (`internal/game/game.go`)

```go
// PlanetTotals is a snapshot of planet-wide aggregates.
type PlanetTotals struct {
    Population int // Σ Empire.People over living empires
    Regions    int // Σ Empire.Land
    NetWorth   int // Σ NetWorth(e)
}

// DailyBulletin is one day's frozen header: the totals at that day's
// maintenance and the change since the prior day.
type DailyBulletin struct {
    Totals PlanetTotals
    Change PlanetTotals // Totals minus the previous day's Totals
}
```

On `World`:

- `NewsToday []string \`json:"Bulletin"\`` — renames the existing `Bulletin`
  field but keeps the JSON key, so old saves load their lines into today's news.
- `NewsYesterday []string` — rolled from `NewsToday` at maintenance.
- `BulletinToday DailyBulletin` — the current day's frozen header.
- `BulletinYesterday DailyBulletin` — rolled from `BulletinToday`.

`postNews` appends to `NewsToday` (unchanged 20-line cap). An `EnsureNews()`
migration (mirroring `EnsureTreaties`/`EnsureMorale`) is a no-op beyond the JSON
rename — no data movement needed since the key is reused.

Per-empire random events reuse the existing `Empire.Events []string` log (shown
at turn start and cleared in `turn.go` when the player next plays). No new field.

## Component 1 — Rotation (daily maintenance hook)

In `DailyMaintenance` (`internal/game/turn.go`), after the per-empire economy
step and before returning, run one ordered `rollNews(w)` step:

1. Generate this day's **economic** and **political** news into `NewsToday`
   (Component 3), and each empire's **random events** into `Empire.Events`
   (Component 4) — these fire as their conditions are computed during
   maintenance.
2. Compute `newTotals := planetTotals(w)`.
3. `w.BulletinYesterday = w.BulletinToday`
   `w.BulletinToday = DailyBulletin{Totals: newTotals, Change: newTotals - w.BulletinToday.Totals}`
   (per-field subtraction; the first-ever day yields Change == Totals, acceptable).
4. `w.NewsYesterday = w.NewsToday; w.NewsToday = nil`.

`planetTotals(w)` sums `People`, `Land`, and `NetWorth(e)` over living empires.

## Component 2 — Daily Bulletin header (render, `internal/menu`)

`renderDailyBulletin(s, b game.DailyBulletin, title string)` draws a boxed panel
using the existing `titleBar`/table helpers in `actions.go`:

```
[ <title> — Daily Bulletin ]
  Total Population: 1,865,289    Change: -5,838
  Total Regions:    53,266       Change: +0
  Total Net Worth:  34,833k      Change: +1,373k
```

- Change is colored: green for `+`, red for `-`, neutral for `0`; `+`/`-` sign
  always shown. Net worth uses the existing `k`/`m` abbreviation helper.
- `title` = `Config.BoardID` if non-empty, else the panel shows just
  "Daily Bulletin".

`showBulletin` (`gameflow.go`) gains a "which day" argument. `Today's News`
renders `renderDailyBulletin(BulletinToday)` + `NewsToday`; `Yesterday's News`
renders `BulletinYesterday` + `NewsYesterday`. The `tree.go` items `Today's News`
(4) and `Yesterday's News` (5) call the respective variant (they currently both
call the same stub).

## Component 3 — Broadened planet news

Posted to `NewsToday` during maintenance, all original wording:

- **Economic** — when the daily investment/interest rate floats (existing
  `InvestRate` logic in `turn.go`), post a line stating the direction and size
  of the move.
- **Political** — when the Planetary Master changes vs `LastMaster`, post a
  "new master" line; when a master exists and is unchanged, post a
  "retains the title" line (matches BRE, which shows it daily).
- **Civil** — riots (high tax) and starvation already have effects in
  `turn.go`; ensure each posts a planet-news line if not already.

Military lines (combat/WMD/pirate) already post via `news.go` and need no
change beyond now targeting `NewsToday`.

## Component 4 — Random per-empire events

At maintenance, each living empire may fire one random event
(`maybeRandomEvent(w, e)`):

- 7 resources × {lose, gain} = **14 categories**: troopers, jets, turrets,
  tanks, agents, food, people.
- Each category is a Go slice of 2-4 original one-line variants with a `%d`
  count. Structure: `randomEvents map[eventKey][]string` plus a magnitude range
  per resource.
- Fire chance is a constant `RandomEventChancePct` (per empire per day). On
  fire: pick a category (respecting sensible weighting so a near-empty empire
  isn't told it lost units it doesn't have — a "lose" category is skipped if the
  resource is 0), pick a variant, roll a magnitude within range (clamped so a
  resource never goes below 0), apply the delta, and append the formatted line
  to `e.Events`.
- All deltas use `w.rng` for determinism.

## Files

- Modify: `internal/game/game.go` (types + World fields + `EnsureNews`,
  `planetTotals`), `internal/game/turn.go` (`rollNews`, economic/political
  posting, `maybeRandomEvent` call), `internal/game/news.go` (retarget
  `postNews`; economic/political/civil posters), new
  `internal/game/events_random.go` (the 14-category tables + `maybeRandomEvent`).
- Modify: `internal/menu/gameflow.go` (`showBulletin` day arg +
  `renderDailyBulletin`), `internal/menu/tree.go` (wire Today/Yesterday).
- Tests: `internal/game/*_test.go` (rotation, totals, random events),
  `internal/menu/*_test.go` (header render, day split).

## Testing

- **Rotation** — seed a world, run two maintenance cycles, assert
  `BulletinYesterday`/`NewsYesterday` hold the prior day and `Change` equals the
  per-field delta.
- **Totals** — `planetTotals` sums only living empires.
- **Header render** — fixed values produce the right lines, sign, and color
  (green `+`, red `-`).
- **Random events** — fixed seed → deterministic category/magnitude; a "lose"
  category never drives a resource below 0; a 0-resource empire is never told it
  lost that resource.
- **Economic/political** — a forced rate move and a master change each post one
  line; no line when nothing changed.

## Implementation slices (for the plan)

1. Data model + rotation + `planetTotals` + migration (Component 1) + tests.
2. Daily Bulletin header render + Today/Yesterday wiring (Component 2) + tests.
3. Broadened economic/political/civil news (Component 3) + tests.
4. Random per-empire events (Component 4) + tests.

Each slice is independently testable and reviewable.

## Out of scope

- Sysop-editable events file (random events are hardcoded this pass).
- A separate league-name field for the header (uses `BoardID` for now).
- Inter-BBS news packet changes (`NEWS_DATA` packet) — the planet feed is local.
- The full BRE weather/market-flavor news subtypes beyond economic/political.
