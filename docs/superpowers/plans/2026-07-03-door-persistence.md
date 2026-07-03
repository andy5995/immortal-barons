# Slice 2 — Persistence / Multi-user Door Game Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn Immortal Barons from a throwaway per-call game into a
persistent, shared, asynchronous door game where each caller has a lasting
empire in one world.

**Architecture:** One JSON world file guarded by an exclusive `flock`. A
session loads the world, runs any missed daily maintenance, finds or
onboards the caller's empire (keyed by BBS handle), plays, and saves. The
economy processes per *player turn* (idle empires stagnate); a separate
daily maintenance advances the world (refill turns, run AI, cull dead
empires, advance the day). Both front-ends share one orchestration package.

**Tech Stack:** Go 1.26, standard library only (`encoding/json`, `os`,
`syscall`, `time`).

## Global Constraints

- Go 1.26; **standard library only** — no new dependencies.
- Run `gofmt -w .` before every commit; keep `go vet ./...` clean.
- Module path: `github.com/andy5995/immortal-barons`.
- Dates are ISO strings `YYYY-MM-DD` (lexicographic order == chronological).
  Functions that need "today" take it as a `string` parameter so tests are
  deterministic; only `main` calls `time.Now()`.
- Tests use a fixed RNG seed (`game.NewWorldSeed`) and a scripted fake
  `session.Session`; no test calls `time.Now()`.
- Reference spec: `docs/superpowers/specs/2026-07-03-door-persistence-design.md`.

---

### Task 1: Game config and persistent world/empire model

**Files:**
- Create: `internal/game/config.go`
- Modify: `internal/game/game.go`
- Test: `internal/game/game_test.go` (replace obsolete tests)

> **Sequencing note:** Tasks 1 and 2 rewrite `game.go` and `turn.go`
> together — the `game` package does not compile in between (the old
> `turn.go` references fields/methods Task 1 removes). Implement both, then
> build, test, and commit once (at the end of Task 2).

**Interfaces:**
- Produces:
  - `game.Config{ TurnsPerDay, ProtectionTurns, AICount int; DataDir string }`
  - `game.DefaultConfig() Config`
  - `game.NewWorld(cfg Config) *World`, `game.NewWorldSeed(cfg Config, seed int64) *World`
  - `World` fields: `Config Config`, `GameDay int`, `LastMaintDate string`,
    `Active *Empire` (json:"-"), `Today string` (json:"-")
  - `Empire` fields: `Owner string`, `TurnsLeft int`, `Protection int`,
    `LastPlayed string`, `Events []string`
  - `(w *World) Player() *Empire` returns `w.Active`
  - `(w *World) AIEmpires() []*Empire` (alive, `Owner == ""`)
  - `(w *World) Targets(attacker *Empire) []*Empire` (alive, not attacker, `Protection == 0`)
  - `(w *World) FindByOwner(handle string) *Empire`
  - `(w *World) AddHuman(handle, realm string) *Empire`

- [ ] **Step 1: Write `internal/game/config.go`**

```go
package game

type Config struct {
	TurnsPerDay     int
	ProtectionTurns int
	AICount         int
	DataDir         string
}

func DefaultConfig() Config {
	return Config{
		TurnsPerDay:     10,
		ProtectionTurns: 20,
		AICount:         0,
		DataDir:         "./data",
	}
}
```

- [ ] **Step 2: Write failing tests in `internal/game/game_test.go`**

Replace the whole file (the old single-player tests no longer apply):

```go
package game

import "testing"

func testWorld() *World {
	cfg := DefaultConfig()
	cfg.AICount = 2
	return NewWorldSeed(cfg, 1)
}

func TestNewWorldSeedsAIOnly(t *testing.T) {
	w := testWorld()
	if len(w.Empires) != 2 {
		t.Fatalf("want 2 AI empires, got %d", len(w.Empires))
	}
	for _, e := range w.Empires {
		if e.Owner != "" {
			t.Errorf("AI empire should have empty Owner, got %q", e.Owner)
		}
	}
	if w.Player() != nil {
		t.Error("no active empire yet, Player() should be nil")
	}
}

func TestAddHumanAndFindByOwner(t *testing.T) {
	w := testWorld()
	e := w.AddHuman("Khan", "New Barony")
	if e.Owner != "khan" {
		t.Errorf("owner should be normalized to lowercase, got %q", e.Owner)
	}
	if e.Name != "New Barony" {
		t.Errorf("realm name: got %q", e.Name)
	}
	if w.FindByOwner("KHAN") != e {
		t.Error("FindByOwner should match case-insensitively")
	}
	if w.FindByOwner("nobody") != nil {
		t.Error("FindByOwner should return nil for unknown handle")
	}
}

func TestTargetsExcludeSelfAndProtected(t *testing.T) {
	w := testWorld()
	me := w.AddHuman("me", "Mine")
	me.Protection = 0
	w.Empires[0].Protection = 5 // protected AI
	w.Empires[1].Protection = 0
	got := w.Targets(me)
	for _, e := range got {
		if e == me {
			t.Error("Targets must not include the attacker")
		}
		if e.Protection > 0 {
			t.Error("Targets must exclude protected empires")
		}
	}
	if len(got) != 1 {
		t.Errorf("want 1 targetable empire, got %d", len(got))
	}
}
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/game/ -run 'TestNewWorld|TestAddHuman|TestTargets' -v`
Expected: FAIL (undefined `NewWorldSeed`, `AddHuman`, `Targets`, etc.)

- [ ] **Step 4: Rewrite `internal/game/game.go`**

Replace the file with the persistent model. Note the removed `MaxTurns`,
`GameOver`, `New`, and old `Rivals`/`NetWorth` signature stays but `Player`
now returns `Active`.

```go
// Package game holds the BRE world: empires, economy, turn engine, and
// combat. The world is persistent and shared; one session is active at a
// time (Active), and dates flow in as ISO strings for testability.
package game

import (
	"math/rand"
	"strings"
	"time"
)

type Empire struct {
	Name  string
	Owner string // normalized BBS handle; "" for AI
	Alive bool

	Gold int
	Bank int
	Debt int
	Food int
	Land int

	People   int
	Troopers int
	Jets     int
	Turrets  int
	Tanks    int
	Carriers int

	Tax int

	TurnsLeft  int
	Protection int
	LastPlayed string
	Events     []string
}

func (e *Empire) Army() int { return e.Troopers + e.Jets + e.Turrets + e.Tanks }

func (e *Empire) Offense() int {
	usableJets := min(e.Jets, e.Carriers*100)
	return e.Troopers + usableJets*2 + e.Tanks*4
}

func (e *Empire) Defense() int {
	return e.Troopers + e.Turrets*2 + e.Tanks*4
}

type Prices struct {
	Land, Food, Trooper, Jet, Turret, Tank, Carrier int
}

const (
	InterestCap = 1_599_999_999
	MoneyCap    = 2_000_000_000
)

type World struct {
	Empires       []*Empire
	Prices        Prices
	Config        Config
	GameDay       int
	LastMaintDate string

	Coordinator bool

	// Player preferences (kept on the world for now; per-empire is a later
	// refinement). Referenced by the Preferences menu.
	EnterExitsBuy  bool
	DepositEndTurn bool
	AutoPayMaint   bool
	AutoFeed       bool

	Active *Empire `json:"-"` // the empire playing this session
	Today  string  `json:"-"` // ISO date for this session

	rng *rand.Rand
}

func NewWorld(cfg Config) *World { return NewWorldSeed(cfg, time.Now().UnixNano()) }

func NewWorldSeed(cfg Config, seed int64) *World {
	w := &World{
		Prices: Prices{Land: 100, Food: 2, Trooper: 50, Jet: 60, Turret: 60, Tank: 350, Carrier: 40},
		Config: cfg,
		rng:    rand.New(rand.NewSource(seed)),
	}
	names := []string{"Crimson Horde", "Iron Dominion", "Ashfall Clan", "Storm Reavers", "Dust Kings"}
	for i := 0; i < cfg.AICount && i < len(names); i++ {
		w.Empires = append(w.Empires, newEmpire(names[i], "", cfg))
		w.Empires[len(w.Empires)-1].Jets = 5
		w.Empires[len(w.Empires)-1].Turrets = 40
	}
	return w
}

func newEmpire(name, owner string, cfg Config) *Empire {
	return &Empire{
		Name: name, Owner: owner, Alive: true,
		Gold: 10000, Food: 20000, Land: 100, People: 2000,
		Troopers: 150, Carriers: 1, Tax: 15,
		TurnsLeft: cfg.TurnsPerDay, Protection: cfg.ProtectionTurns,
	}
}

// AddHuman creates and registers a human empire keyed by handle.
func (w *World) AddHuman(handle, realm string) *Empire {
	e := newEmpire(realm, strings.ToLower(strings.TrimSpace(handle)), w.Config)
	w.Empires = append(w.Empires, e)
	return e
}

func (w *World) Player() *Empire { return w.Active }

func (w *World) FindByOwner(handle string) *Empire {
	h := strings.ToLower(strings.TrimSpace(handle))
	for _, e := range w.Empires {
		if e.Owner == h {
			return e
		}
	}
	return nil
}

func (w *World) AIEmpires() []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if e.Owner == "" && e.Alive {
			r = append(r, e)
		}
	}
	return r
}

func (w *World) Targets(attacker *Empire) []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if e != attacker && e.Alive && e.Protection == 0 {
			r = append(r, e)
		}
	}
	return r
}

func (w *World) NetWorth(e *Empire) int {
	return e.Gold + e.Bank - e.Debt +
		e.Land*12500 + e.Food*w.Prices.Food +
		e.Troopers*250 + e.Jets*325 + e.Turrets*425 + e.Tanks*1250 + e.Carriers*1000 +
		e.People*5
}
```

- [ ] **Step 5: Do not build/commit yet**

The `game` package will not compile until Task 2 rewrites `turn.go` (the old
`turn.go` still calls the removed `Rivals()`/`w.Turn`). Proceed directly to
Task 2, then build, test, and commit `config.go` + `game.go` + `turn.go` +
their tests together.

---

### Task 2: Turn split — PlayTurn and DailyMaintenance (compiles + commits with Task 1)

**Files:**
- Modify: `internal/game/turn.go`
- Test: `internal/game/turn_test.go` (create)

**Interfaces:**
- Consumes: `World`, `Empire`, `Config` (Task 1).
- Produces:
  - `(w *World) PlayTurn(e *Empire, today string)` — per-empire economy; decrements `TurnsLeft` and `Protection`; sets `e.LastPlayed = today`.
  - `(w *World) DailyMaintenance(today string)` — self-catching-up global advance.
  - `(w *World) nextDate(d string) string` — helper, returns `d`+1 day.

- [ ] **Step 1: Write failing tests in `internal/game/turn_test.go`**

```go
package game

import "testing"

func TestPlayTurnAffectsOnlyActingEmpire(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 2
	w := NewWorldSeed(cfg, 1)
	other := w.Empires[1]
	otherGold := other.Gold
	me := w.AddHuman("me", "Mine")
	turns := me.TurnsLeft
	prot := me.Protection

	w.PlayTurn(me, "2026-07-03")

	if me.TurnsLeft != turns-1 {
		t.Errorf("TurnsLeft: want %d, got %d", turns-1, me.TurnsLeft)
	}
	if me.Protection != prot-1 {
		t.Errorf("Protection: want %d, got %d", prot-1, me.Protection)
	}
	if me.LastPlayed != "2026-07-03" {
		t.Errorf("LastPlayed: got %q", me.LastPlayed)
	}
	if other.Gold != otherGold {
		t.Error("PlayTurn must not touch other empires")
	}
	if me.Gold <= 10000 {
		t.Errorf("acting empire should collect income, got %d", me.Gold)
	}
}

func TestDailyMaintenanceInitialisesDate(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.DailyMaintenance("2026-07-03")
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("first maintenance should just set the date, got %q", w.LastMaintDate)
	}
	if w.GameDay != 0 {
		t.Errorf("first maintenance should not advance the day, got %d", w.GameDay)
	}
}

func TestDailyMaintenanceCatchesUpAndRefills(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-01"
	me := w.AddHuman("me", "Mine")
	me.TurnsLeft = 0
	w.DailyMaintenance("2026-07-03") // two days missed
	if w.LastMaintDate != "2026-07-03" {
		t.Errorf("should catch up to today, got %q", w.LastMaintDate)
	}
	if w.GameDay != 2 {
		t.Errorf("two days should advance GameDay to 2, got %d", w.GameDay)
	}
	if me.TurnsLeft != cfg.TurnsPerDay {
		t.Errorf("turns should be refilled to %d, got %d", cfg.TurnsPerDay, me.TurnsLeft)
	}
}

func TestDailyMaintenanceIdempotent(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03"
	w.GameDay = 5
	w.DailyMaintenance("2026-07-03")
	if w.GameDay != 5 {
		t.Errorf("same-day maintenance should be a no-op, got %d", w.GameDay)
	}
}

func TestDailyMaintenanceCullsDead(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	w.LastMaintDate = "2026-07-02"
	dead := w.AddHuman("gone", "Gone")
	dead.Land = 0
	w.DailyMaintenance("2026-07-03")
	if w.FindByOwner("gone").Alive {
		t.Error("empire with 0 land should be marked dead")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/game/ -run 'TestPlayTurn|TestDailyMaintenance' -v`
Expected: FAIL (undefined `PlayTurn`, `DailyMaintenance`).

- [ ] **Step 3: Rewrite `internal/game/turn.go`**

```go
package game

import "time"

// PlayTurn processes one turn for a single empire: its economy runs, and
// its turn/protection counters tick down. Idle empires (whose owner never
// plays) never call this, so they stagnate.
func (w *World) PlayTurn(e *Empire, today string) {
	w.processEconomy(e)
	if e.TurnsLeft > 0 {
		e.TurnsLeft--
	}
	if e.Protection > 0 {
		e.Protection--
	}
	e.LastPlayed = today
}

// DailyMaintenance advances the world to `today`, running one pass per
// missed day. It is idempotent (no-op if already current) and self-catching
// up (loops over multiple missed days). The first call on a brand-new world
// just records the date.
func (w *World) DailyMaintenance(today string) {
	if w.LastMaintDate == "" {
		w.LastMaintDate = today
		return
	}
	for w.LastMaintDate < today {
		for _, e := range w.Empires {
			if e.Alive {
				e.TurnsLeft = w.Config.TurnsPerDay
			}
		}
		w.aiPlay(w.LastMaintDate)
		for _, e := range w.Empires {
			if e.Alive && (e.Land <= 0 || e.People <= 0) {
				e.Alive = false
			}
		}
		w.GameDay++
		w.LastMaintDate = w.nextDate(w.LastMaintDate)
	}
}

func (w *World) nextDate(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

// aiPlay runs each AI empire's turns for one day.
func (w *World) aiPlay(today string) {
	for _, e := range w.AIEmpires() {
		for e.TurnsLeft > 0 {
			if e.Gold > 5000 {
				n := (e.Gold / 4) / w.Prices.Trooper
				e.Troopers += n
				e.Gold -= n * w.Prices.Trooper
			}
			w.PlayTurn(e, today)
		}
	}
}

func (w *World) processEconomy(e *Empire) {
	e.Gold += e.People*e.Tax/100*8 + e.Land*20

	e.Bank += min(e.Bank, InterestCap) / 100
	if e.Debt > 0 {
		e.Debt += e.Debt * 10 / 100
	}

	e.Food += e.Land*100 - (e.People + e.Troopers + e.Jets*2 + e.Tanks*2)
	if e.Food < 0 {
		e.People -= (-e.Food)/10 + 1
		if e.People < 0 {
			e.People = 0
		}
		e.Food = 0
	}

	maint := e.Troopers*6 + e.Jets*12 + e.Turrets*9 + e.Tanks*6 + e.Carriers*1
	if e.Gold >= maint {
		e.Gold -= maint
	} else {
		e.Gold = 0
		e.Troopers -= e.Troopers / 10
	}

	if e.Food > 0 {
		if g := e.People * (10 - e.Tax/5) / 100; g > 0 {
			e.People += g
		}
	}

	if e.Gold > MoneyCap {
		e.Gold = MoneyCap
	}
	if e.Bank > MoneyCap {
		e.Bank = MoneyCap
	}
}
```

- [ ] **Step 4: Build and run all game tests (Tasks 1 + 2 together)**

Run: `go build ./internal/game/ && go test ./internal/game/ -v`
Expected: the package compiles now; all Task 1 and Task 2 tests PASS.

- [ ] **Step 5: Commit Tasks 1 + 2 together**

```bash
gofmt -w . && git add internal/game/config.go internal/game/game.go internal/game/turn.go internal/game/game_test.go internal/game/turn_test.go
git commit -m "feat(game): persistent world model + per-turn/daily split"
```

---

### Task 3: Combat — protection guard and victim event log

**Files:**
- Modify: `internal/game/combat.go`
- Test: `internal/game/combat_test.go` (create)

**Interfaces:**
- Consumes: `World.Attack`, `Empire.Events` (Tasks 1–2).
- Produces: `(w *World) Attack(a, d *Empire) string` now also appends a
  summary line to `d.Events`.

- [ ] **Step 1: Write failing test in `internal/game/combat_test.go`**

```go
package game

import (
	"strings"
	"testing"
)

func TestAttackRecordsVictimEvent(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("att", "Attacker")
	a.Troopers = 100000 // overwhelming, deterministic win
	d := w.Empires[0]
	d.Protection = 0

	before := len(d.Events)
	w.Attack(a, d)
	if len(d.Events) != before+1 {
		t.Fatalf("victim should get one event, got %d new", len(d.Events)-before)
	}
	if !strings.Contains(d.Events[len(d.Events)-1], a.Name) {
		t.Errorf("event should name the attacker: %q", d.Events[len(d.Events)-1])
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/game/ -run TestAttackRecordsVictimEvent -v`
Expected: FAIL (no event appended).

- [ ] **Step 3: Edit `internal/game/combat.go`**

At the very start of `Attack`, before building the report, capture the
attacker name; at the end (before `return`), append the summary to the
victim. Replace the function body's `return b.String()` tail with:

```go
	d.Events = append(d.Events, "While you were away: "+b.String())
	return b.String()
```

(Keep the rest of `Attack` unchanged: offense vs. defense resolution,
`loseForces`, plunder, capture, `jitter`.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/game/ -run TestAttackRecordsVictimEvent -v`
Expected: PASS.

- [ ] **Step 5: Run the whole game package**

Run: `go test ./internal/game/ -v`
Expected: PASS (all game tests).

- [ ] **Step 6: Commit**

```bash
gofmt -w . && git add internal/game/combat.go internal/game/combat_test.go
git commit -m "feat(game): attacks record an event on the victim"
```

---

### Task 4: Store — JSON save/load

**Files:**
- Create: `internal/store/store.go`
- Test: `internal/store/store_test.go`

**Interfaces:**
- Consumes: `game.World`, `game.Config` (Task 1).
- Produces:
  - `store.Load(cfg game.Config) (*game.World, error)` — reads
    `<DataDir>/world.json`; returns `game.NewWorld(cfg)` if absent.
  - `store.Save(w *game.World, cfg game.Config) error` — atomic write.

- [ ] **Step 1: Write failing tests in `internal/store/store_test.go`**

```go
package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func cfgIn(dir string) game.Config {
	c := game.DefaultConfig()
	c.DataDir = dir
	return c
}

func TestLoadMissingReturnsFreshWorld(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	cfg.AICount = 3
	w, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Empires) != 3 {
		t.Errorf("fresh world should have 3 AI, got %d", len(w.Empires))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Gold = 4242
	e.Events = []string{"hello"}
	w.GameDay = 7
	w.LastMaintDate = "2026-07-03"

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.GameDay != 7 || got.LastMaintDate != "2026-07-03" {
		t.Errorf("world scalars not preserved: day=%d date=%q", got.GameDay, got.LastMaintDate)
	}
	ge := got.FindByOwner("khan")
	if ge == nil || ge.Gold != 4242 || len(ge.Events) != 1 {
		t.Errorf("empire not preserved: %+v", ge)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	// no leftover temp file
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "world.json.tmp")); !os.IsNotExist(err) {
		t.Error("temp file should not remain after save")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/store/ -v`
Expected: FAIL (package/functions do not exist).

- [ ] **Step 3: Write `internal/store/store.go`**

```go
// Package store persists the game world to disk and guards it with an
// exclusive lock so only one session mutates it at a time.
package store

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/andy5995/immortal-barons/internal/game"
)

func worldPath(cfg game.Config) string { return filepath.Join(cfg.DataDir, "world.json") }

// Load reads the saved world, or returns a fresh one if none exists.
func Load(cfg game.Config) (*game.World, error) {
	data, err := os.ReadFile(worldPath(cfg))
	if os.IsNotExist(err) {
		return game.NewWorld(cfg), nil
	}
	if err != nil {
		return nil, err
	}
	w := game.NewWorld(cfg) // seeds rng; JSON overwrites exported fields
	if err := json.Unmarshal(data, w); err != nil {
		return nil, err
	}
	w.Config = cfg
	return w, nil
}

// Save writes the world atomically (temp file + rename).
func Save(w *game.World, cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := worldPath(cfg) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, worldPath(cfg))
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/store/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && git add internal/store/store.go internal/store/store_test.go
git commit -m "feat(store): atomic JSON save/load of the world"
```

---

### Task 5: Store — exclusive lock

**Files:**
- Create: `internal/store/lock.go`
- Test: `internal/store/lock_test.go`

**Interfaces:**
- Consumes: `game.Config` (Task 1).
- Produces:
  - `store.ErrBusy` (sentinel error)
  - `store.Lock(cfg game.Config, block bool) (*FileLock, error)` — `block=false`
    returns `ErrBusy` if held; `block=true` waits.
  - `(*FileLock) Release() error`

- [ ] **Step 1: Write failing test in `internal/store/lock_test.go`**

```go
package store

import (
	"errors"
	"testing"
)

func TestLockIsExclusive(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	l1, err := Lock(cfg, false)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}
	if _, err := Lock(cfg, false); !errors.Is(err, ErrBusy) {
		t.Errorf("second non-blocking lock should be ErrBusy, got %v", err)
	}
	if err := l1.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	l2, err := Lock(cfg, false)
	if err != nil {
		t.Fatalf("lock after release should succeed: %v", err)
	}
	l2.Release()
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/store/ -run TestLockIsExclusive -v`
Expected: FAIL (undefined `Lock`, `ErrBusy`).

- [ ] **Step 3: Write `internal/store/lock.go`**

```go
package store

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ErrBusy means another session holds the game lock.
var ErrBusy = errors.New("game is busy")

type FileLock struct{ f *os.File }

// Lock takes the exclusive game lock. With block=false it returns ErrBusy
// immediately if the lock is held; with block=true it waits.
func Lock(cfg game.Config, block bool) (*FileLock, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(cfg.DataDir, "game.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_EX
	if !block {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrBusy
		}
		return nil, err
	}
	return &FileLock{f: f}, nil
}

func (l *FileLock) Release() error {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/store/ -v`
Expected: PASS (both store tests).

- [ ] **Step 5: Commit**

```bash
gofmt -w . && git add internal/store/lock.go internal/store/lock_test.go
git commit -m "feat(store): exclusive flock-based game lock"
```

---

### Task 6: Menu wiring for the persistent model

**Files:**
- Modify: `internal/menu/tree.go` (statusBar), `internal/menu/actions.go`
  (nextTurn, regularAttack), `internal/menu/menu_test.go`

**Interfaces:**
- Consumes: `World.Player` (=Active), `World.Today`, `PlayTurn`, `Targets`
  (Tasks 1–3).
- Produces: menu behaves against a persistent world with turn limits.

- [ ] **Step 1: Update `internal/menu/menu_test.go`**

Every menu test now needs an **active empire** (the status bar calls
`w.Player()`, which would nil-deref otherwise). The simple tests
(`TestQuitFromMain`, `TestQuitIsCaseInsensitive`, `TestEnterAndLeaveSubmenu`,
`TestUnknownKeyIgnored`, `TestHiddenCoordinatorNotSelectable`) go through the
`run` helper — update it once:

```go
func run(t *testing.T, keys string) (*fakeSession, error) {
	t.Helper()
	f := &fakeSession{keys: []rune(keys)}
	cfg := game.DefaultConfig()
	cfg.AICount = 1
	w := game.NewWorldSeed(cfg, 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	return f, Run(f, w, Build())
}
```

Replace the three inline tests (`TestCoordinatorReachableWhenFlagged`,
`TestBuyLandThroughMenu`, `TestPreferenceToggle`) and swap
`TestNextTurnAdvances` for `TestNextTurnConsumesATurn`, all explicitly:

```go
func TestCoordinatorReachableWhenFlagged(t *testing.T) {
	f := &fakeSession{keys: []rune("YRQ")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	w.Coordinator = true
	if err := Run(f, w, Build()); err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Configuration Editor") {
		t.Error("coordinator menu should be reachable when flagged")
	}
}

func TestBuyLandThroughMenu(t *testing.T) {
	f := &fakeSession{keys: []rune("BL5\r ")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	before := w.Active.Land
	Run(f, w, Build())
	if w.Active.Land != before+5 {
		t.Errorf("expected land %d, got %d", before+5, w.Active.Land)
	}
}

func TestPreferenceToggle(t *testing.T) {
	f := &fakeSession{keys: []rune("PF")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	if err := Run(f, w, Build()); err != io.EOF {
		t.Fatalf("expected EOF after script, got %v", err)
	}
	if !w.AutoFeed {
		t.Error("Auto-feed should be ON after toggling")
	}
}

func TestNextTurnConsumesATurn(t *testing.T) {
	f := &fakeSession{keys: []rune("N ")}
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.Active = w.AddHuman("tester", "Testland")
	w.Today = "2026-07-03"
	left := w.Active.TurnsLeft
	Run(f, w, Build())
	if w.Active.TurnsLeft != left-1 {
		t.Errorf("Next Turn should consume a turn: want %d, got %d", left-1, w.Active.TurnsLeft)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/menu/ -v`
Expected: FAIL (statusBar/nextTurn still reference removed `MaxTurns`/`GameOver`; compile errors).

- [ ] **Step 3: Update `internal/menu/tree.go` statusBar**

```go
func statusBar(w *game.World) string {
	p := w.Player()
	return fmt.Sprintf("%s | Gold %d  Food %d  Land %d  Army %d | Turns left %d | Day %d",
		p.Name, p.Gold, p.Food, p.Land, p.Army(), p.TurnsLeft, w.GameDay)
}
```

- [ ] **Step 4: Update `internal/menu/actions.go`**

Replace `nextTurn` (remove the game-over path; enforce the turn limit):

```go
func nextTurn(s session.Session, w *game.World) Result {
	p := w.Player()
	if p.TurnsLeft <= 0 {
		ok(s, "You are out of turns for today. Come back after the next maintenance.")
		return Stay
	}
	w.PlayTurn(p, w.Today)
	ok(s, "Turn complete. Turns left: %d", p.TurnsLeft)
	return Stay
}
```

Delete `showFinal` and any reference to `w.GameOver()`. In `regularAttack`,
block a protected attacker and use `Targets` for the target list — replace
the opening of the function (through the `targets :=` line) with:

```go
func regularAttack(s session.Session, w *game.World) Result {
	if w.Player().Protection > 0 {
		ok(s, "You are under New Realm Protection and cannot attack yet.")
		return Stay
	}
	targets := w.Targets(w.Player())
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./internal/menu/ -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
gofmt -w . && git add internal/menu/
git commit -m "feat(menu): drive persistent world, per-turn limit, targeting"
```

---

### Task 7: Play — session orchestration

**Files:**
- Create: `internal/play/play.go`
- Test: `internal/play/play_test.go`

**Interfaces:**
- Consumes: `store.Lock/Load/Save/ErrBusy`, `game.*`, `menu.Run/Build`,
  `session.Session` (Tasks 1–6).
- Produces:
  - `play.Identity{ Handle string }`
  - `play.Run(s session.Session, id Identity, cfg game.Config, today string) error`

- [ ] **Step 1: Write failing tests in `internal/play/play_test.go`**

```go
package play

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

type fakeSession struct {
	keys []rune
	pos  int
	out  bytes.Buffer
}

func (f *fakeSession) ReadKey() (rune, error) {
	if f.pos >= len(f.keys) {
		return 0, io.EOF
	}
	r := f.keys[f.pos]
	f.pos++
	return r, nil
}
func (f *fakeSession) Write(p []byte) (int, error) { return f.out.Write(p) }

func cfgIn(dir string) game.Config {
	c := game.DefaultConfig()
	c.DataDir = dir
	return c
}

func TestOnboardsThenPersists(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// realm name "Khanate" then Quit
	f := &fakeSession{keys: []rune("Khanate\rQ")}
	if err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, _ := store.Load(cfg)
	e := w.FindByOwner("khan")
	if e == nil {
		t.Fatal("empire should have been created and saved")
	}
	if e.Name != "Khanate" {
		t.Errorf("realm name: got %q", e.Name)
	}
}

func TestReturningPlayerResumes(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune("Khanate\rQ")}
	Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	f2 := &fakeSession{keys: []rune("Q")} // no naming prompt second time
	Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if strings.Contains(f2.out.String(), "Name your Realm") {
		t.Error("returning player should not be asked to name a realm")
	}
}

func TestBusyLockIsReported(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	held, err := store.Lock(cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	f := &fakeSession{}
	if err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatalf("busy should be handled gracefully, got %v", err)
	}
	if !strings.Contains(f.out.String(), "busy") {
		t.Error("should tell the caller the game is busy")
	}
}

func TestEventsShownThenCleared(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	// create + save an empire with a pending event
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.Events = []string{"Enemy raided you!"}
	store.Save(w, cfg)

	f := &fakeSession{keys: []rune("Q")}
	Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if !strings.Contains(f.out.String(), "raided") {
		t.Error("pending event should be shown on login")
	}
	w2, _ := store.Load(cfg)
	if len(w2.FindByOwner("khan").Events) != 0 {
		t.Error("events should be cleared after being shown")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/play/ -v`
Expected: FAIL (package does not exist).

- [ ] **Step 3: Write `internal/play/play.go`**

```go
// Package play is the shared session orchestration used by every front-end:
// lock the world, load it, run missed maintenance, find or onboard the
// caller's empire, show pending events, play, and save.
package play

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

type Identity struct{ Handle string }

// Run plays one session for the given caller.
func Run(s session.Session, id Identity, cfg game.Config, today string) error {
	lock, err := store.Lock(cfg, false)
	if errors.Is(err, store.ErrBusy) {
		fmt.Fprintf(s, "\n%sThe game is busy — please try again shortly.%s\n", ansi.FgYellow, ansi.Reset)
		return nil
	}
	if err != nil {
		return err
	}
	defer lock.Release()

	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	w.Today = today
	w.DailyMaintenance(today)

	e := w.FindByOwner(id.Handle)
	if e == nil {
		realm := onboard(s, w, id.Handle)
		e = w.AddHuman(id.Handle, realm)
	}
	w.Active = e

	showEvents(s, e)

	if err := menu.Run(s, w, menu.Build()); err != nil {
		return err
	}
	return store.Save(w, cfg)
}

func onboard(s session.Session, w *game.World, handle string) string {
	taken := map[string]bool{}
	for _, e := range w.Empires {
		taken[strings.ToLower(e.Name)] = true
	}
	fmt.Fprintf(s, "%s%sName Your Empire%s\n", ansi.Clear, ansi.FgBrightCyan, ansi.Reset)
	for {
		fmt.Fprintf(s, "\n%sName your Realm:%s ", ansi.FgBrightWhite, ansi.Reset)
		name, err := session.ReadLine(s)
		if err != nil {
			return handle // stream ended; fall back to the handle
		}
		name = strings.TrimSpace(name)
		if alnum(name) < 3 || taken[strings.ToLower(name)] {
			fmt.Fprintf(s, "%s  Invalid: at least 3 letters/numbers, not matching another realm.%s\n", ansi.FgRed, ansi.Reset)
			continue
		}
		return name
	}
}

func showEvents(s session.Session, e *game.Empire) {
	if len(e.Events) == 0 {
		return
	}
	fmt.Fprintf(s, "%s%sWhile you were away:%s\n", ansi.Clear, ansi.FgBrightCyan, ansi.Reset)
	for _, ev := range e.Events {
		fmt.Fprintf(s, "  %s\n", ev)
	}
	e.Events = nil
	fmt.Fprintf(s, "\n%sPress any key...%s", ansi.FgWhite, ansi.Reset)
	s.ReadKey()
}

func alnum(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/play/ -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
gofmt -w . && git add internal/play/
git commit -m "feat(play): shared session orchestration with persistence"
```

---

### Task 8: Front-ends — barons-door (play + -maint) and local barons

**Files:**
- Modify: `cmd/barons-door/main.go`, `cmd/barons/main.go`

**Interfaces:**
- Consumes: `play.Run/Identity`, `store.Lock/Load/Save`, `game.*`,
  `door.ParseDropfile`, `session.NewStdio/NewConsole` (Tasks 1–7 + slice 1).

- [ ] **Step 1: Rewrite `cmd/barons-door/main.go`**

```go
// Command barons-door runs Immortal Barons as a native BBS door. Normal
// mode reads the caller's dropfile and plays over stdio. With -maint it runs
// daily maintenance non-interactively (for the sysop's nightly event).
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func main() {
	dropPath := flag.String("dropfile", "", "path to the BBS dropfile (DOOR32.SYS or DOOR.SYS)")
	dataDir := flag.String("data", "./data", "game data directory")
	maint := flag.Bool("maint", false, "run daily maintenance and exit")
	flag.Parse()

	cfg := game.DefaultConfig()
	cfg.DataDir = *dataDir
	today := time.Now().Format("2006-01-02")

	if *maint {
		if err := runMaint(cfg, today); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -maint:", err)
			os.Exit(1)
		}
		return
	}

	path := *dropPath
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	if path == "" {
		path = findDropfile()
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "barons-door: no dropfile found; pass -dropfile PATH")
		os.Exit(2)
	}
	caller, err := door.ParseDropfile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "barons-door:", err)
		os.Exit(1)
	}

	s := session.NewStdio()
	if caller.SecondsLeft > 0 {
		go func() {
			time.Sleep(time.Duration(caller.SecondsLeft) * time.Second)
			fmt.Fprint(s, "\r\n\r\nYour BBS time is up. Farewell, Baron!\r\n")
			os.Exit(0)
		}()
	}
	handle := caller.Handle
	if handle == "" {
		handle = fmt.Sprintf("node%d", caller.Node)
	}
	if err := play.Run(s, play.Identity{Handle: handle}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "barons-door:", err)
		os.Exit(1)
	}
}

// runMaint blocks on the lock (waits for any active player) then advances
// the world.
func runMaint(cfg game.Config, today string) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	w.DailyMaintenance(today)
	return store.Save(w, cfg)
}

func findDropfile() string {
	for _, n := range []string{"door32.sys", "DOOR32.SYS", "door.sys", "DOOR.SYS"} {
		if _, err := os.Stat(n); err == nil {
			return n
		}
	}
	return ""
}
```

- [ ] **Step 2: Rewrite `cmd/barons/main.go`**

```go
// Command barons plays Immortal Barons locally in your terminal against the
// shared persistent world.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
)

const version = "0.0.1"

func main() {
	name := flag.String("name", defaultName(), "your player handle")
	dataDir := flag.String("data", "./data", "game data directory")
	flag.Parse()

	cfg := game.DefaultConfig()
	cfg.DataDir = *dataDir
	today := time.Now().Format("2006-01-02")

	c := session.NewConsole()
	defer c.Close()

	fmt.Fprintf(c, "\n      IMMORTAL BARONS  v%s\n\n", version)
	if err := play.Run(c, play.Identity{Handle: *name}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "barons:", err)
	}
	fmt.Fprint(c, "\nUntil next turn, Baron.\n")
}

func defaultName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "sysop"
}
```

- [ ] **Step 3: Build everything and run the full suite**

Run: `gofmt -w . && go vet ./... && go build ./... && go test ./...`
Expected: all packages build; all tests PASS.

- [ ] **Step 4: Manual smoke — local persistence**

```bash
rm -rf /tmp/ib-data
printf 'Khanate\rDE Q' | go run ./cmd/barons -name Khan -data /tmp/ib-data
printf 'Q' | go run ./cmd/barons -name Khan -data /tmp/ib-data
```
Expected: first run onboards ("Name your Realm"), second run resumes without
the naming prompt. `cat /tmp/ib-data/world.json` shows the "khan" empire.

- [ ] **Step 5: Manual smoke — door + maintenance**

```bash
rm -rf /tmp/ib-data
printf '2\n0\n0\nBBS\n1\nReal Name\nKhan\n80\n30\n1\n1\n' > /tmp/DOOR32.SYS
printf 'Khanate\rQ' | go run ./cmd/barons-door -dropfile /tmp/DOOR32.SYS -data /tmp/ib-data
go run ./cmd/barons-door -maint -data /tmp/ib-data   # advances the day
```
Expected: door onboards Khan; `-maint` runs without error and (on a later
date) increments `GameDay` in `world.json`.

- [ ] **Step 6: Commit**

```bash
gofmt -w . && git add cmd/
git commit -m "feat(cmd): wire door and local front-ends to persistent play"
```

---

## Notes for the implementer

- **Slice 1 already exists:** `internal/door` (dropfile parser),
  `internal/session` (Console, Stdio, ReadLine), `internal/ansi`,
  `internal/menu`. Do not recreate them.
- The old `cmd/barons/main.go` had a splash + interactive `nameRealm`; that
  logic moves into `play.onboard`. Remove the old `nameRealm`/`alnumCount`
  from `cmd/barons`.
- After Task 1, packages other than `game` will not build until their tasks
  land — that is expected; run per-package tests as the steps say.
