# Concurrent Web World Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let up to N (config, default 4) players share one live in-memory web world at once, with cross-player data always current, while the door and local front-ends keep their single-player exclusive-flock model unchanged.

**Architecture:** Move the "current empire" off the shared `game.World` (its `Active` field) into a per-session menu context (`ctx`) that embeds `*game.World` and carries the session's `active *Empire`. Guard the shared world with a single `sync.Mutex`, held only around short mutate-or-snapshot windows (never across player input). The web server becomes the sole owner of one long-lived world: it takes the flock once at startup, runs its own daily-maintenance tick, and persists on turn-commit and shutdown. The door/local path keeps the existing combined lock+load+maintenance+save in `play.Run`.

**Tech Stack:** Go 1.26, standard library (`sync`, `time`), existing `internal/{game,menu,session,play,store}` packages. Tests use the scripted fake `Session` and `go test -race`.

## Global Constraints

- Go 1.26; standard library preferred; run `gofmt -w .` before every commit and keep `go vet ./...` clean.
- The door (`cmd/barons-door`) and local (`cmd/barons`) front-ends MUST keep working exactly as today: single player, whole-session exclusive flock via `play.Run`. N=1 is just the degenerate case of the new code.
- **Never hold `World.mu` while blocked on player input** (`ReadKey`, `session.ReadLine`, `promptSuggested`, the message editor). Lock only around re-validation + mutation, or a brief read snapshot. A lock spanning a prompt re-serializes all players and defeats the project.
- A web-hosted world is web-only: the web server holds the exclusive flock for its whole lifetime; if the flock is busy at startup, fail fast.
- Do not port IBBS / inter-planetary / gooie co-funding to the web; those stay door/BBS-world concepts. No active mid-turn push — liveness is fresh-on-navigate only.
- Tests use `game.NewWorldSeed(cfg, seed)` for determinism (never wall-clock seeding in tests).

---

## File Structure

- `internal/game/game.go` — remove `World.Active` field and `World.Player()`; add `mu sync.Mutex`, `Lock()`, `Unlock()`, `With(fn func())`. Fix `RemoveEmpire` (drop the `Active` nil-out).
- `internal/game/league.go` — `resetForNewGame` drops `w.Active = nil`.
- `internal/menu/menu.go` — define per-session `ctx` (embeds `*game.World`, carries `active *game.Empire`, has `Player()`); change `Item` callback field types, `Action`, `draw`, `readChoice`, `Run`, `langSession`, `playerLang` from `*game.World` to `*ctx`.
- `internal/menu/tree.go` — convert the `owned`/`half`/price closures and any direct `func(*game.World)` menu callbacks to `*ctx`; unwrap `.World` where a `*game.World`-typed mutation method expression is called.
- `internal/menu/actions.go` — `buy2`/`sellUnit2` and every hand-written mutating `Do` gain the gather→lock→re-validate→mutate→unlock discipline; convert signatures to `*ctx`.
- `internal/menu/gameflow.go` — `GameLoop` and `Run` calls take the session's active empire; convert `*game.World` callback params to `*ctx` where they are menu callbacks.
- `internal/menu/helpbrowse.go` — convert `*game.World` menu-callback params to `*ctx`.
- `internal/game/config.go` — add `MaxConcurrentSessions int` (default 4) to `Config`/`DefaultConfig`.
- `internal/play/play.go` — split `Run` into `Run` (door/local: lock+load+maintenance+session+save) and an exported `Session` (plays one session against an already-loaded, caller-owned world; onboarding locks the world; no per-session maintenance/flock/save).
- `cmd/barons-web/main.go` — server owns the world: flock + load at startup, daily-maintenance ticker, `MaxConcurrentSessions` cap in the hub, per-session `play.Session` + save-on-commit.
- Tests: `internal/game/lock_test.go` (new), `internal/menu/ctx_test.go` (new), plus additions to existing menu/play tests, and `cmd/barons-web` or `internal/play` concurrent race test.

---

### Task 1: World mutex + `With` helper

Add the lock primitive used by every later task. No behavior change yet.

**Files:**
- Modify: `internal/game/game.go` (add fields/methods near the `Active`/`Today` block, ~line 232)
- Test: `internal/game/lock_test.go` (create)

**Interfaces:**
- Produces: `func (w *World) Lock()`, `func (w *World) Unlock()`, `func (w *World) With(fn func())` — `With` runs `fn` while holding the mutex.

- [ ] **Step 1: Write the failing test**

```go
package game

import (
	"sync"
	"testing"
)

func TestWorldWithIsMutuallyExclusive(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	const goroutines, incs = 8, 1000
	counter := 0
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < incs; j++ {
				w.With(func() { counter++ })
			}
		}()
	}
	wg.Wait()
	if counter != goroutines*incs {
		t.Fatalf("counter = %d, want %d (lost updates => not mutually exclusive)", counter, goroutines*incs)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game/ -run TestWorldWithIsMutuallyExclusive -race -v`
Expected: FAIL — `w.With` undefined (build error).

- [ ] **Step 3: Add the mutex and helpers**

In `internal/game/game.go`, add to the `World` struct near the `Active`/`Today` block (an unexported field is never marshaled, so no `json` tag is needed):

```go
	mu sync.Mutex // guards concurrent access when a server shares one World
```

Add `"sync"` to the game package imports if not already present. Add the methods (place them right after `NewWorldSeed` or near `Player`'s old location):

```go
// Lock/Unlock guard the shared World when a single process runs concurrent
// sessions (the web server). The door/local front-ends run one session and
// take it uncontended.
func (w *World) Lock()   { w.mu.Lock() }
func (w *World) Unlock() { w.mu.Unlock() }

// With runs fn while holding the world lock. Use it around a short
// mutate-or-snapshot window — never around player input.
func (w *World) With(fn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	fn()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/game/ -run TestWorldWithIsMutuallyExclusive -race -v`
Expected: PASS.

- [ ] **Step 5: Full game package test + commit**

Run: `gofmt -w . && go build ./... && go test ./internal/game/ -race`
Expected: PASS.

```bash
git add internal/game/game.go internal/game/lock_test.go
git commit -m "feat(game): add World mutex + With helper for concurrent sessions"
```

---

### Task 2: Move `Active` into a per-session `ctx`

The core refactor. The "current empire" stops being world state and becomes session state threaded through the menu engine. After this task the door/local/web still run one session at a time (web keeps its per-session flock until Task 5), so this is a pure structural change with no concurrency enabled yet — every existing test must still pass.

**Files:**
- Modify: `internal/game/game.go` (remove `Active` field ~line 232, remove `Player()` ~line 309, fix `RemoveEmpire` ~line 315), `internal/game/league.go` (~line 39)
- Modify: `internal/menu/menu.go`, `internal/menu/tree.go`, `internal/menu/actions.go`, `internal/menu/gameflow.go`, `internal/menu/helpbrowse.go`
- Modify: `internal/play/play.go` (~line 73, 79)
- Test: `internal/menu/ctx_test.go` (create); existing menu tests must pass unchanged in behavior.

**Interfaces:**
- Produces (menu package, unexported): `type ctx struct { *game.World; active *game.Empire }` with `func (c *ctx) Player() *game.Empire { return c.active }`.
- Produces: `Action = func(s session.Session, c *ctx) Result`; `Item` fields `LabelFn/Hidden func(*ctx)…`, `Price/Owned func(*ctx) int`.
- Produces: `func GameLoop(s session.Session, w *game.World, active *game.Empire) error`; `func Run(s session.Session, c *ctx, root *Menu) error`.
- Consumes: `game.World.With` / `Lock` / `Unlock` (Task 1) — not used yet here, but the types must exist.

- [ ] **Step 1: Write the failing test**

`internal/menu/ctx_test.go` — proves two sessions hold independent active empires (the bug `w.Active` caused):

```go
package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestCtxActiveIsPerSession(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	a := w.AddHuman("alice", "Alice")
	b := w.AddHuman("bob", "Bob")

	ca := &ctx{World: w, active: a}
	cb := &ctx{World: w, active: b}

	if ca.Player() != a {
		t.Fatalf("ca.Player() = %v, want alice", ca.Player())
	}
	if cb.Player() != b {
		t.Fatalf("cb.Player() = %v, want bob", cb.Player())
	}
	// They share the same world data (liveness) but not the active pointer.
	if ca.World != cb.World {
		t.Fatal("both sessions must share one World")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/menu/ -run TestCtxActiveIsPerSession -v`
Expected: FAIL — `ctx` undefined (build error).

- [ ] **Step 3: Define `ctx` and remove `Active` from `World`**

In `internal/menu/menu.go`, add near the top (after imports):

```go
// ctx is the per-session menu context: one shared *game.World plus the active
// empire for THIS session. Threading the active empire here (instead of a
// field on the shared World) is what lets the web server run concurrent
// sessions against one world. It embeds *game.World so callback bodies keep
// using w.Config, w.Prices, w.NetWorth(...), etc. unchanged.
type ctx struct {
	*game.World
	active *game.Empire
}

// Player is the active empire for this session (nil before onboarding / after
// abdication).
func (c *ctx) Player() *game.Empire { return c.active }
```

In `internal/game/game.go`:
- Remove the `Active *Empire ` json:"-"`` field from `World` (keep `Today`).
- Remove `func (w *World) Player() *Empire { return w.Active }`.
- In `RemoveEmpire`, delete the trailing block:

```go
	if w.Active == e {
		w.Active = nil
	}
```

In `internal/game/league.go` `resetForNewGame`, delete `w.Active = nil` and the comment line above it referencing Active.

- [ ] **Step 4: Convert menu callback types from `*game.World` to `*ctx`**

This is a mechanical type sweep. The rule: **every menu callback parameter that is currently `*game.World` becomes `*ctx`; keep the parameter name `w` so bodies are unchanged** (embedding promotes `w.Config`, `w.Prices`, etc., and `w.Player()` now resolves to `ctx.Player()`). Where a body passes `w` into a function that expects a real `*game.World` (a `game`/`store` function or a `(*game.World).Method` expression), pass `w.World` instead.

In `internal/menu/menu.go`:

```go
type Item struct {
	Key     rune
	Label   string
	LabelFn func(*ctx) string // dynamic label; wins over Label
	Do      Action            // nil => heading/separator, not selectable
	Hidden  func(*ctx) bool   // nil => always shown
	Price   func(*ctx) int
	Owned   func(*ctx) int
}

func (it *Item) hidden(c *ctx) bool { return it.Hidden != nil && it.Hidden(c) }

func (it *Item) label(c *ctx) string {
	if it.LabelFn != nil {
		return it.LabelFn(c)
	}
	return it.Label
}

func (it *Item) displayLabel(c *ctx, lang string) string {
	if it.LabelFn != nil {
		return it.LabelFn(c)
	}
	return i18n.T(lang, it.Label)
}

type Action func(s session.Session, c *ctx) Result
```

Change `playerLang` and `langSession` to read the active empire from the ctx rather than the world:

```go
// playerLang is the active caller's UI language ("" = English).
func playerLang(c *ctx) string {
	if c != nil && c.active != nil {
		return c.active.Language
	}
	return ""
}

// langSession wraps a Session so output helpers can learn the caller's language
// from the Session alone. Per-session state — safe for concurrent web sessions.
type langSession struct {
	session.Session
	c *ctx
}

func (l *langSession) Lang() string { return playerLang(l.c) }
```

Change `draw`, `readChoice`, `hasColumns`, `topBar`, `titleRule` callers, and `Run` to thread `*ctx` instead of `*game.World`. Signatures:

```go
func draw(s session.Session, c *ctx, m *Menu) { /* body: replace g with c; playerLang(c); it.hidden(c); it.Price(c); it.Owned(c); it.displayLabel(c, lang); m.Status(c); topBar(c) */ }

func (m *Menu) readChoice(s session.Session, c *ctx) (*Item, error) { /* body: it.selectable(c), it.hidden(c) */ }

func (it *Item) selectable(c *ctx) bool { return it.Do != nil && !it.hidden(c) }

func Run(s session.Session, c *ctx, root *Menu) error {
	s = &langSession{Session: s, c: c}
	stack := []*Menu{root}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		draw(s, c, cur)
		item, err := cur.readChoice(s, c)
		if err != nil {
			return err
		}
		if item == nil {
			continue
		}
		switch res := item.Do(s, c); res.kind {
		// ... unchanged ...
		}
	}
	return nil
}
```

Any menu-level helper that took `*game.World` and reads `Player()`/menu fields (`topBar`, `hasColumns`, `Status`, `titleRule` if it reads world) becomes `*ctx`. Grep to find them all:
`grep -n "\*game.World" internal/menu/menu.go`.

- [ ] **Step 5: Convert `tree.go` closures and `actions.go`/`gameflow.go` handlers**

In `internal/menu/tree.go`, convert the funnel helpers (bodies unchanged except types; `.World` unwrap only where a real `*game.World` is required):

```go
	owned := func(f func(*game.Empire) int) func(*ctx) int {
		return func(c *ctx) int { return f(c.Player()) }
	}
	priceTrooper := func(c *ctx) int { return c.Prices.Trooper }
	// ...priceJet/Turret/Bomber/Agent/Tank/Carrier identically: param c *ctx, body c.Prices.X...
	half := func(f func(*ctx) int) func(*ctx) int {
		return func(c *ctx) int { return f(c) / 2 }
	}
```

The `troopers`/`jets`/… (`func(p *game.Empire) int`) stay as-is (they take an empire, not a world). Method-expression `Do`s like `buy2("Troopers", true, priceTrooper, (*game.World).Recruit)` keep the `(*game.World).Recruit` expression — `buy2` (Task 3 signature) unwraps `.World` internally. Any remaining direct closures typed `func(w *game.World) …` in `tree.go` menu fields (`Price:`, `Hidden:`, `LabelFn:`) change to `func(w *ctx) …`. Find every one: `grep -n "func(w \*game.World)\|func(g \*game.World)" internal/menu/tree.go` — all 30 must become `*ctx`.

In `internal/menu/actions.go` and `internal/menu/gameflow.go`, every hand-written `Do` handler `func(s session.Session, w *game.World) Result` becomes `func(s session.Session, w *ctx) Result`. Bodies keep `w.Player()`, `w.Config`, etc. via embedding. Where a body calls a `store.`/`game.` function needing a real world (e.g. `store.Save(w, cfg)`), change to `w.World`. Menu-only helper funcs that took `*game.World` and are called from handlers (e.g. `showBulletin`, `incomeReport`, `endOfTurnStats`, `paymentStage`, `runTurn` in `gameflow.go`) — decide per function: if it reads `Player()` or menu state, take `*ctx`; if it is pure game logic on the world, keep `*game.World` and pass `w.World`.

In `internal/menu/helpbrowse.go`, convert its `*game.World` menu-callback params to `*ctx` (`browseCategory(s, cats[i-1], c.Player().Language)` etc.).

- [ ] **Step 6: Thread the active empire from `GameLoop`/`play.go`**

In `internal/menu/gameflow.go`, `GameLoop` takes the active empire and builds the ctx, and its internal `Run` calls pass the ctx:

```go
func GameLoop(s session.Session, w *game.World, active *game.Empire) error {
	c := &ctx{World: w, active: active}
	// ...body: replace bare w with c where a menu callback/Run/draw is involved;
	// keep w.World for pure game/store calls...
	// e.g.:
	if err := Run(s, c, menus.Spending); err != nil {
		return err
	}
	// ...
}
```

In `internal/play/play.go`:
- Delete `w.Active = e` (line ~73).
- Change `menu.GameLoop(s, w)` (line ~79) to `menu.GameLoop(s, w, e)`.

- [ ] **Step 7: Build, vet, run the whole suite (behavior unchanged)**

Run: `gofmt -w . && go build ./... && go vet ./... && go test ./... -race`
Expected: PASS — including the new `TestCtxActiveIsPerSession` and every pre-existing menu/play test (this task changed structure, not behavior).

- [ ] **Step 8: Commit**

```bash
git add internal/game/game.go internal/game/league.go internal/menu/ internal/play/play.go
git commit -m "refactor(menu): move active empire into per-session ctx; drop World.Active"
```

---

### Task 3: Lock discipline in mutating actions

Apply gather→lock→re-validate→mutate→unlock to the mutation funnels and the hand-written mutating handlers, so a turn's mutation is atomic and never blocks another player during input. The game mutation methods already re-check their own preconditions (e.g. gold), so the lock's job is to make that check-and-apply atomic against the shared world.

**Files:**
- Modify: `internal/menu/actions.go` (`buy2` ~line 17, `sellUnit2` ~line 47, `buildHQ`, `buyLand`, and every hand-written mutating `Do`), `internal/menu/gameflow.go` (`runTurn`, `paymentStage` — the turn-commit mutations)
- Test: `internal/menu/lockdiscipline_test.go` (create)

**Interfaces:**
- Consumes: `ctx` (embeds `*game.World`), `game.World.With` (Task 1).

- [ ] **Step 1: Write the failing test**

Prove the mutation is applied under the lock and re-validation refuses a stale purchase. Drive `buy2` through a fake session that requests more than the player can afford after a concurrent drain:

```go
package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestBuyRevalidatesUnderLock(t *testing.T) {
	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	p := w.AddHuman("alice", "Alice")
	p.Gold = 100
	c := &ctx{World: w, active: p}

	price := func(_ *ctx) int { return 10 }
	apply := func(gw *game.World, e *game.Empire, n int) error { return gw.Recruit(e, n) }
	action := buy2("Troopers", false, price, func(gw *game.World, e *game.Empire, n int) error {
		// Simulate another session draining gold between the prompt and the apply.
		e.Gold = 5
		return apply(gw, e, n)
	})

	// Ask to buy 10 (affordable at prompt time: 100/10). Under lock, gold is 5.
	f := &fakeSession{keys: []rune("10\r")}
	action(f, c)

	if p.Troopers != 0 {
		t.Fatalf("stale purchase applied: Troopers = %d, want 0", p.Troopers)
	}
	if !strings.Contains(strings.ToLower(f.out.String()), "gold") {
		t.Fatalf("expected an insufficient-gold refusal, got: %q", f.out.String())
	}
}
```

(If `fakeSession` is defined in `menu_test.go`, this file shares the package and reuses it. Confirm its field names with `grep -n "type fakeSession" internal/menu/*_test.go` and match them.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/menu/ -run TestBuyRevalidatesUnderLock -race -v`
Expected: FAIL — the purchase applies because `buy2` doesn't yet re-validate/lock (or the injected drain isn't respected).

- [ ] **Step 3: Add the discipline to `buy2` and `sellUnit2`**

`buy2` — gather `n` (unlocked), then lock around the affordability recompute + apply:

```go
func buy2(label string, military bool, unit func(*ctx) int, apply func(*game.World, *game.Empire, int) error) Action {
	return func(s session.Session, w *ctx) Result {
		p := w.Player()
		if military && w.Config.BuyMilitary == game.BuyNo {
			fail(s, fmt.Errorf("Buying military units is disabled in this league."))
			return Stay
		}
		price := unit(w)
		max := 0
		if price > 0 {
			max = p.Gold / price
		}
		n := promptSuggested(s, fmt.Sprintf("%s — %d gold each. How many?", label, price), 0, max)
		if n <= 0 {
			return Stay
		}
		var err error
		w.With(func() { err = apply(w.World, p, n) }) // apply re-checks gold atomically
		if err != nil {
			fail(s, err)
		} else {
			ok(s, "Done. Gold remaining: %d", p.Gold)
		}
		return Stay
	}
}
```

Apply the identical pattern to `sellUnit2` (wrap its `apply(w.World, p, n)` in `w.With`). Note the `unit`/`price` parameter type is now `*ctx` (Task 2).

- [ ] **Step 4: Add the discipline to hand-written mutating handlers**

For each hand-written `Do` that mutates the world (e.g. `buildHQ`, `buyLand`, attack handlers in `actions.go`, covert handlers, banking, gooie, messaging send, `runTurn`/`paymentStage` in `gameflow.go`): gather all input first, then wrap only the world-mutating call(s) in `w.With(func(){ ... })`. Read-only display handlers (scoreboards, status) that read multiple world fields wrap the reads in `w.With` to snapshot into locals, then render outside the lock. Enumerate them: `grep -n "func(s session.Session, w \*ctx) Result\|func .*w \*ctx.*Result" internal/menu/actions.go internal/menu/gameflow.go`. For each, confirm no `promptSuggested`/`ReadLine`/`ReadKey`/message-editor call sits inside a `w.With` block.

Example — `buyLand` shape (gather target amount unlocked, mutate under lock):

```go
func buyLand(s session.Session, w *ctx) Result {
	p := w.Player()
	// ... gather how many regions/units via promptSuggested (UNLOCKED) ...
	var err error
	w.With(func() { err = w.World.BuyLand(p, n) }) // or whatever the real mutation call is
	if err != nil {
		fail(s, err)
	} else {
		ok(s, "...")
	}
	return Stay
}
```

- [ ] **Step 5: Run the tests**

Run: `gofmt -w . && go build ./... && go test ./internal/menu/ -race`
Expected: PASS — `TestBuyRevalidatesUnderLock` and all existing menu tests.

- [ ] **Step 6: Commit**

```bash
git add internal/menu/actions.go internal/menu/gameflow.go internal/menu/lockdiscipline_test.go
git commit -m "feat(menu): lock world around mutations; re-validate after player input"
```

---

### Task 4: Web concurrent-session cap

Add the config knob and enforce it in the web hub, so only N sessions run at once.

**Files:**
- Modify: `internal/game/config.go` (add field + default)
- Modify: `cmd/barons-web/main.go` (`stream` handler, ~line 105)
- Test: `internal/game/config_test.go` (add a default-value assertion; match the existing style of `TestDefaultIdleConfig`)

**Interfaces:**
- Produces: `Config.MaxConcurrentSessions int` (default 4).
- Consumes: nothing new.

- [ ] **Step 1: Write the failing test**

Add to `internal/game/config_test.go` (or the file holding `TestDefaultIdleConfig`):

```go
func TestDefaultMaxConcurrentSessions(t *testing.T) {
	if c := DefaultConfig(); c.MaxConcurrentSessions != 4 {
		t.Errorf("MaxConcurrentSessions = %d, want 4", c.MaxConcurrentSessions)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/game/ -run TestDefaultMaxConcurrentSessions -v`
Expected: FAIL — unknown field `MaxConcurrentSessions`.

- [ ] **Step 3: Add the config field + default**

In `internal/game/config.go`, add to `Config` (near `MaxPlayers`): `MaxConcurrentSessions int `json:"max_concurrent_sessions"`` (match the tag style of the surrounding fields), and set it in `DefaultConfig`: `MaxConcurrentSessions: 4,`.

- [ ] **Step 4: Enforce it in the hub**

In `cmd/barons-web/main.go` `stream`, when creating a new session, refuse past the cap (the hub already holds `h.mu` around `h.sessions`):

```go
	h.mu.Lock()
	ws, existed := h.sessions[id]
	if !existed {
		if len(h.sessions) >= h.cfg.MaxConcurrentSessions {
			h.mu.Unlock()
			http.Error(w, "The realm is full — try again shortly.", http.StatusServiceUnavailable)
			return
		}
		ws = session.NewWebSession()
		h.sessions[id] = ws
		log.Printf("session connected from %s", r.RemoteAddr)
		go h.runGame(id, ws, r.RemoteAddr)
	}
	h.mu.Unlock()
```

- [ ] **Step 5: Build + test**

Run: `gofmt -w . && go build ./... && go test ./internal/game/ -run TestDefaultMaxConcurrentSessions -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/game/config.go internal/game/config_test.go cmd/barons-web/main.go
git commit -m "feat(web): cap concurrent sessions (MaxConcurrentSessions, default 4)"
```

---

### Task 5: Server-owned world lifecycle

Split `play.Run` so the web server owns the world (flock once, load once, its own maintenance tick, save on commit) and each browser session plays against the shared world via a new `play.Session`. The door/local path keeps the combined `play.Run`. This is the task that actually enables concurrent web play.

**Files:**
- Modify: `internal/play/play.go` (extract `Session` from `Run`)
- Modify: `cmd/barons-web/main.go` (hub owns `world`, lock, save; `runGame` calls `play.Session`; maintenance ticker in `main`)
- Test: `internal/play/session_test.go` (create) — a concurrent race test lives in Task 6; here, a unit test that `Session` plays and persists via the injected save.

**Interfaces:**
- Produces: `func Session(s session.Session, id Identity, w *game.World, cfg game.Config, save func() error) (reason string, err error)` — onboards (locking the world for `AddHuman`), wraps the session in the idle/time deadline, runs `GameLoop`, returns the end reason. Does NOT take the flock, load, run daily maintenance, or own persistence beyond calling `save` at session end.
- `Run` keeps its current signature `func Run(s session.Session, id Identity, cfg game.Config, today string) (reason string, err error)` and now delegates its inner play to `Session`.

- [ ] **Step 1: Write the failing test**

`internal/play/session_test.go`:

```go
package play

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestSessionOnboardsAndSaves(t *testing.T) {
	cfg := cfgIn(t.TempDir()) // reuse the helper from play_test.go
	w := game.NewWorldSeed(cfg, 1)
	saved := false
	f := &fakeSession{keys: []rune(" Khanate\r0")} // splash, realm name, quit
	if _, err := Session(f, Identity{Handle: "Khan"}, w, cfg, func() error { saved = true; return nil }); err != nil {
		t.Fatal(err)
	}
	if w.FindByOwner("khan") == nil {
		t.Fatal("empire should have been onboarded into the shared world")
	}
	if !saved {
		t.Fatal("Session should call save at end of session")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/play/ -run TestSessionOnboardsAndSaves -v`
Expected: FAIL — `Session` undefined.

- [ ] **Step 3: Extract `Session` from `Run`**

Refactor `internal/play/play.go`. Move the per-session body (deadline wrap, splash, onboard, showEvents, GameLoop, reason logic) into `Session`; `Run` keeps the flock+load+maintenance and delegates. Onboarding mutates the shared world, so lock it:

```go
// Session plays one session against an already-loaded world owned by the
// caller. It does not take the flock, load, or run daily maintenance — the
// caller owns those. It calls save once at session end.
func Session(s session.Session, id Identity, w *game.World, cfg game.Config, save func() error) (reason string, err error) {
	var hard time.Time
	if id.TimeLeft > 0 {
		hard = time.Now().Add(id.TimeLeft)
	}
	d := session.NewDeadline(s, time.Duration(cfg.IdleTimeoutSecs)*time.Second, cfg.MaxIdleWarnings, hard)
	s = d

	menu.Splash(s)

	e := w.FindByOwner(id.Handle)
	if e == nil {
		if !w.Config.JoinOpen(w.Today) {
			fmt.Fprintf(s, "\n%sThe game is closed to new barons (join cutoff %s has passed).%s\n", ansi.FgYellow, w.Config.JoinDate, ansi.Reset)
			return "closed", save()
		}
		if w.BoardFull() {
			fmt.Fprintf(s, "\n%sThis realm is full — no new barons may enroll.%s\n", ansi.FgYellow, ansi.Reset)
			return "closed", save()
		}
		realm := onboard(s, w, id.Handle)
		w.With(func() { e = w.AddHuman(id.Handle, realm) })
	}

	showEvents(s, e)

	gameErr := menu.GameLoop(s, w, e)
	reason = d.Reason()
	if reason == "" {
		if errors.Is(gameErr, io.EOF) {
			reason = "disconnect"
		} else {
			reason = "quit"
		}
	}
	if gameErr != nil && !errors.Is(gameErr, io.EOF) {
		return reason, gameErr
	}
	return reason, save()
}
```

`Run` becomes the door/local wrapper (unchanged external behavior):

```go
func Run(s session.Session, id Identity, cfg game.Config, today string) (reason string, err error) {
	lock, err := store.Lock(cfg, false)
	if errors.Is(err, store.ErrBusy) {
		fmt.Fprintf(s, "\n%sThe game is busy — please try again shortly.%s\n", ansi.FgYellow, ansi.Reset)
		return "busy", nil
	}
	if err != nil {
		return "", err
	}
	defer lock.Release()

	w, err := store.Load(cfg)
	if err != nil {
		return "", err
	}
	w.Today = today
	w.DailyMaintenance(today)

	return Session(s, id, w, cfg, func() error { return store.Save(w, cfg) })
}
```

(`w.Today` is now the source for `JoinOpen` inside `Session`; keep `Run` setting it. For the web server, set `w.Today` at load and on each maintenance tick.)

- [ ] **Step 4: Wire the web server to own the world**

In `cmd/barons-web/main.go`:
- Add `world *game.World` and `lock *store.FileLock` (and keep `cfg`) to the `hub`; add a `saveWorld()` method that saves under the world lock:

```go
func (h *hub) saveWorld() error {
	var err error
	h.world.With(func() { err = store.Save(h.world, h.cfg) })
	return err
}
```

- In `main`, before serving: take the flock (fail fast if busy), load the world, set `world.Today`, run one `DailyMaintenance`, and start a maintenance ticker:

```go
	lock, err := store.Lock(cfg, false)
	if err != nil {
		log.Fatalf("cannot start: another process holds this world (a web-hosted world can't be shared with a door): %v", err)
	}
	defer lock.Release()
	world, err := store.Load(cfg)
	if err != nil {
		log.Fatalf("load world: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	world.Today = today
	world.DailyMaintenance(today)
	h := &hub{sessions: map[string]*session.WebSession{}, cfg: cfg, world: world, lock: lock}

	// Daily maintenance on date rollover (guarded by the world lock).
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			d := time.Now().Format("2006-01-02")
			h.world.With(func() {
				if h.world.Today != d {
					h.world.Today = d
					h.world.DailyMaintenance(d)
				}
			})
		}
	}()
```

- Change `runGame` to play against the shared world and save on commit:

```go
func (h *hub) runGame(id string, ws *session.WebSession, addr string) {
	defer func() {
		ws.Close()
		h.mu.Lock()
		delete(h.sessions, id)
		h.mu.Unlock()
	}()
	reason, err := play.Session(ws, play.Identity{Handle: "web-" + id}, h.world, h.cfg, h.saveWorld)
	if err != nil {
		log.Printf("session from %s: %v", addr, err)
	}
	log.Printf("session from %s ended (%s)", addr, reason)
	fmt.Fprint(ws, "\r\n\x1b[97mUntil next turn, Baron. Refresh to play again.\x1b[0m\r\n")
}
```

Confirm `DailyMaintenance` is safe to call while holding the lock (it must not block on I/O or take the lock again). If `DailyMaintenance` internally persists, adjust so persistence happens outside the maintenance mutation or via the same `store.Save` path already used.

- [ ] **Step 5: Build, vet, test**

Run: `gofmt -w . && go build ./... && go vet ./... && go test ./internal/play/ ./... -race`
Expected: PASS — `TestSessionOnboardsAndSaves`, and the existing `play` tests (`Run` still onboards + persists via the delegated `Session`).

- [ ] **Step 6: Commit**

```bash
git add internal/play/play.go internal/play/session_test.go cmd/barons-web/main.go
git commit -m "feat(web): server owns one shared world (flock, maintenance tick, save-on-commit)"
```

---

### Task 6: Concurrent race/integration test

The headline verification: many goroutines drive scripted sessions against one shared world under `-race`, and the final state is internally consistent.

**Files:**
- Test: `internal/play/concurrent_test.go` (create)

**Interfaces:**
- Consumes: `play.Session`, `game.NewWorldSeed`, the fake `Session` (`fakeSession` from `play_test.go`).

- [ ] **Step 1: Write the test**

Drive N concurrent sessions that each onboard and take a few benign turns (buy land / recruit / quit), then assert no negative gold and a consistent empire count. Scripts are fixed strings (no randomness), and the world seed is fixed:

```go
package play

import (
	"fmt"
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestConcurrentSessionsShareWorld(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	w.Today = "2026-07-05"

	const n = 4
	var wg sync.WaitGroup
	var saveMu sync.Mutex
	save := func() error { saveMu.Lock(); defer saveMu.Unlock(); return nil }

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// splash, realm name, then quit (extend with buy/recruit keystrokes
			// once the exact menu hotkeys are confirmed from tree.go).
			f := &fakeSession{keys: []rune(fmt.Sprintf(" Realm%d\r0", i))}
			if _, err := Session(f, Identity{Handle: fmt.Sprintf("caller%d", i)}, w, cfg, save); err != nil {
				t.Errorf("session %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()

	humans := 0
	for _, e := range w.Empires {
		if e.Gold < 0 {
			t.Errorf("empire %q has negative gold: %d", e.Name, e.Gold)
		}
		if !e.AI {
			humans++
		}
	}
	if humans != n {
		t.Fatalf("onboarded humans = %d, want %d", humans, n)
	}
}
```

(Confirm the `Empire` AI flag field name with `grep -n "AI\b" internal/game/*.go`; adjust `!e.AI` to the real field. Extend the keystroke scripts with real spend/attack hotkeys from `tree.go` to exercise mutations under contention once the menu keys are confirmed.)

- [ ] **Step 2: Run under the race detector**

Run: `go test ./internal/play/ -run TestConcurrentSessionsShareWorld -race -v`
Expected: PASS with no `DATA RACE` report.

- [ ] **Step 3: Full suite under race**

Run: `go test ./... -race`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/play/concurrent_test.go
git commit -m "test(play): concurrent sessions share one world with no data race"
```

---

## Self-Review

**Spec coverage:**
- Shared in-memory world + mutex → Task 1 (mutex) + Task 5 (server owns one world). ✓
- Move `Active` into per-session context → Task 2. ✓
- gather→lock→re-validate→mutate→unlock discipline → Task 3 (Global Constraints restate it). ✓
- Web session cap → Task 4. ✓
- Server-owned lock/load/persistence/daily-maintenance → Task 5. ✓
- Race-detector tests → Task 1, 6 (and `-race` on every task's suite run). ✓
- Door/local keep single-player flock → Task 5 keeps `Run`; Global Constraints enforce it; Task 2 Step 7 runs the full suite to prove no regression. ✓
- Out-of-scope (IBBS/gooie/push) → not implemented; stated in Global Constraints. ✓

**Placeholder scan:** No "TBD"/"implement later". Two tests carry explicit "confirm the real field/hotkey name with grep" notes because the exact identifiers (`fakeSession` fields, `Empire` AI flag, menu hotkeys) must be read from the tree at implementation time — these are verification instructions, not placeholders for logic.

**Type consistency:** `ctx` embeds `*game.World` with field name `World` (used as `w.World`/`c.World`); `Player()` moves from `World` to `ctx`; `Action`/`Item` callbacks are `*ctx`; `GameLoop(s, w, active)`; `Run(s, c, root)`; `Session(s, id, w, cfg, save)` returns `(reason, err)` matching `Run`. Consistent across tasks.
