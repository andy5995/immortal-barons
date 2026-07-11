# Door Concurrency Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Let multiple door processes play one JSON world concurrently by making every world mutation a file-per-action transaction, without changing the web (in-memory) path.

**Architecture:** Inject a `Store` into `*World`; `World.With(fn)` delegates to `store.Transact(fn)`. Web uses an in-memory Store (today's mutex, unchanged). Door/`-local` use a File Store: flock → unmarshal the file into the existing `*World` → run `fn` → save → release. The per-session active empire becomes a handle re-resolved inside each transaction, and every mutating action moves its precondition check inside the transaction that mutates.

**Tech Stack:** Go 1.26, stdlib. `internal/game` (World, Store, With), `internal/store` (File Store, flock), `internal/menu` (ctx, actions), `internal/play` (session wiring), `cmd/*` (front-end wiring).

## Global Constraints

- **Web path stays behaviorally identical.** The in-memory Store must reproduce today's `w.mu.Lock()/Unlock()` exactly; the #36 web concurrency tests must stay green.
- **Whole-file exclusive lock per transaction**, held only for reload→mutate→save (spec §6).
- **Every mutating action re-validates inside a single `Transact`** (spec §3). Abort-on-conflict: on a failed precondition after reload, show "the realm has changed — try again", mutate nothing.
- **Active empire is a handle** re-resolved via `w.Player()` = `FindByOwner(handle)` inside transactions (spec §2).
- Door/`-local` use the File Store; the per-session flock hold in `play.Run` and the session-end `save()` are removed. `-maint`/`-planetary` keep their whole-run lock.
- No background process; presence/interaction are out of scope (follow-up issues).
- `gofmt -w .`, `go vet`, `go test ./...` (incl. `-race`) green per task; also `GOARCH=386 go build ./...`.

---

### Task 1: `Store` interface + in-memory impl behind `World.With`

**Files:** Modify `internal/game/game.go` (`World` struct ~283, `With` ~390); Create `internal/game/store.go`; Test `internal/game/store_test.go`.

**Interfaces — Produces:** `type Store interface { Transact(fn func()) error }`; `func (w *World) SetStore(s Store)`; `World.With(fn func())` now calls `w.store.Transact(fn)` (falling back to the mutex if `store` is nil, so existing tests that build a bare World keep working).

- [ ] **Step 1: Failing test** — `store_test.go`: a `World` with the in-memory store; two goroutines each `With` incrementing a counter 1000×; assert no lost updates (same guarantee as the mutex today). Also assert `With` on a `World` with no store set still serializes (nil-store fallback).
- [ ] **Step 2:** Implement `MemStore{w *World}` with `Transact(fn){ w.mu.Lock(); defer w.mu.Unlock(); fn(); return nil }`. Add `store Store` to `World`; `SetStore`; `With` delegates to `store.Transact` or, if nil, the mutex directly. `NewWorldSeed` sets `w.store = &MemStore{w}` by default (so web/tests are unchanged).
- [ ] **Step 3:** `gofmt`; `go test ./internal/game/ -race`; the whole existing suite green (web path unchanged).
- [ ] **Step 4: Commit** `refactor(game): route World.With through a pluggable Store (in-memory default)`.

---

### Task 2: File Store (flock → reload-into-World → fn → save → release)

**Files:** Create `internal/store/filestore.go`; Test `internal/store/filestore_test.go`. Consumes Task 1's `game.Store`.

**Interfaces — Produces:** `func NewFileStore(w *game.World, cfg game.Config) *FileStore` implementing `game.Store`; `Transact` acquires the flock (blocking), unmarshals `<dataDir>/world.json` into `w` (re-running the `Ensure*` migrations and re-pointing `w.Config`), runs `fn`, `Save`s, releases.

- [ ] **Step 1: Failing test** — build a world, `NewFileStore`, save it; open a *second* `World` + `FileStore` over the same file; in store A `Transact(func(){ deduct gold })`; then in store B `Transact(func(){ read gold })` and assert B sees A's change (cross-store visibility through the file).
- [ ] **Step 2:** Implement. Reuse `store.Lock(cfg, true)` (blocking) and `store.Load`/`Save` internals, but `Load`-into-existing (`json.Unmarshal(data, w)` + the same `Ensure*`/`loadLeagueNodes` calls `Load` makes) rather than allocating a new `*World`, so the caller's pointer stays valid. On a missing file, seed (as `Load` does) then proceed.
- [ ] **Step 3:** `go test ./internal/store/ -race`; green.
- [ ] **Step 4: Commit** `feat(store): FileStore — per-transaction flock+reload+save for concurrent doors`.

---

### Task 3: Active empire by handle

**Files:** Modify `internal/menu/menu.go` (`ctx` ~22, `Player` ~33, `playerLang` ~37); Modify `internal/menu/gameflow.go` (`GameLoop` signature); Test `internal/menu/menu_test.go` helpers.

**Interfaces — Produces:** `ctx.handle string` (replacing `active *game.Empire`); `func (c *ctx) Player() *game.Empire { return c.World.FindByOwner(c.handle) }`; `GameLoop(s, w, handle string, utf8 bool)`.

- [ ] **Step 1: Failing test** — a `ctx` whose world is reloaded (swap the empire slice for equivalents) still returns the correct empire from `Player()` by handle, where a stored pointer would have gone stale.
- [ ] **Step 2:** Change `ctx.active *game.Empire` → `handle string`; `Player()` re-resolves via `FindByOwner`. Update `playerLang` (reads `c.Player().Language`), `langSession`, and the `newWorld()` test helper to set `handle`. Update `GameLoop`/`play.Session` callers to pass the handle (the onboarding result's `e.Owner`).
- [ ] **Step 3:** `gofmt`; `go test ./internal/menu/ -race`; green. (No behavior change yet — `Player()` still resolves against the same in-memory world.)
- [ ] **Step 4: Commit** `refactor(menu): resolve the active empire by handle, not a cached pointer`.

---

### Task 4: Wire door/`-local` to the File Store; drop the session-long flock

**Files:** Modify `internal/play/play.go` (`Run` ~34, `Session` end-save); Modify `cmd/immortal-barons/main.go` (`openSession`/`runLocal`).

- [ ] **Step 1: Failing test** — an integration-style test in `internal/play`: two `Session` runs over one temp data dir using File Stores, interleaved, both complete without one bouncing the other (today's `Run` would `ErrBusy` the second).
- [ ] **Step 2:** In `Run`: stop taking the session-long non-blocking flock. Instead build the world once (`store.Load`), `w.SetStore(store.NewFileStore(w, cfg))`, and run `Session`. Remove the `defer lock.Release()` and the `ErrBusy` early-return. Remove the session-end single `save()` (each action persists now). Daily maintenance on login moves inside a `Transact` (Task 12).
- [ ] **Step 3:** `go test ./... -race`; green. Manually: two `-local` runs against one dir don't bounce.
- [ ] **Step 4: Commit** `feat(play): door/-local use the FileStore; no session-long lock`.

---

### Tasks 5–13: Move re-validation inside a transaction, per action domain

**The pattern (apply to every mutating action):** today many actions do
`p := w.Player(); cost := …; if p.Gold < cost { fail }` **outside** `With`, then
`w.With(func(){ p.Gold -= cost; … })`. After Task 4 the world reloads inside
`With`, so the outside `p`/`cost` are stale. Rewrite each as:

```go
var failed bool
w.With(func() {
    p := w.Player()                 // fresh empire after reload
    cost := w.priceOf(p, n)         // recompute against fresh state
    if p.Gold < cost { failed = true; return }
    p.Gold -= cost; /* apply */
})
if failed { fail(s, "The realm has changed — try again.") ; return Stay }
```

Each task below covers one domain: convert its actions to the single-transaction
pattern and add a **concurrent-conflict test** (two stores over one file: actor A
spends the resource, then actor B's operation must abort cleanly, mutating
nothing).

- [ ] **Task 5 — Economy/regions** (`actions.go`, `economy.go`): Buy Regions, Sell Regions, Buy Food, Sell Food, Build HeadQuarters. Conflict test: A drains gold → B's Buy aborts, no partial regions; per-turn region cap re-checked inside.
- [ ] **Task 6 — Military purchase** (`actions.go`, Spending menu): Buy Troopers/Jets/Turrets/Tanks/Carriers/Bombers/Agents. Conflict test: A drains gold → B's buy aborts.
- [ ] **Task 7 — Bank** (`bank.go`, `actions.go`): Deposit, Withdraw, Loan, Repay, Invest. Conflict test: A withdraws → B's withdraw of the same funds aborts (no negative balance).
- [ ] **Task 8 — Attack** (`actions_attack.go`, `combat.go`, 7 `With` sites): regular/pirate attacks. Conflict test: target dies to A → B's attack aborts (no attacking a dead/absent empire); capture/loss computed on fresh state.
- [ ] **Task 9 — Covert + WMD/SDI** (`actions_covert.go`, `specials.go`): spy/ops, Bomb Targets, Slappenheimer, Nuke/Chem/Bio, Doomer, Fund SDI. Conflict test: A drains gold/agents → B's op aborts; SDI cap re-checked.
- [ ] **Task 10 — Trade + Diplomacy** (`actions_diplomacy.go` 9 sites, `trade.go`): send gold/trade deal, propose/accept/decline treaty. Conflict test: recipient dies → deal aborts; double-accept of a treaty is idempotent.
- [ ] **Task 11 — Messages + Industry + Prefs/Macros** (`actions_message.go`, `gameflow.go` setIndustries, `actions.go` macros/preferences): send mail (append under lock — already close; verify), Set Industries/Specialize, Write Macros, preference toggles. Conflict test: two concurrent mail sends to one inbox both land (no lost message).
- [ ] **Task 12 — Onboarding + login maintenance** (`play.go`, `game.go`): create empire inside a `Transact` re-checking name+board-full (extends #40); date-rollover `DailyMaintenance` inside a `Transact` guarded on `LastMaintDate` so one node runs it. Conflict test: two simultaneous first-time callers with the same realm name → one wins, the other is told the name is taken; maintenance runs once.
- [ ] **Task 13 — Interplanetary ops** (`actions_interbbs.go` 7 sites): group-attack join/add offense, IP special ops. Conflict test: A closes the group-attack window → B's join aborts.

Each task: convert the domain's actions, add the conflict test(s), `go test ./... -race` green, commit `fix(<domain>): single-transaction re-validation for concurrent doors`.

---

### Task 14: Cross-process integration test

**Files:** Create `internal/play/crossproc_test.go` (build-tagged or `testing.Short`-skippable).

- [ ] **Step 1:** A Go test that builds the binary (`go build -o tmp`), then `exec`s two `-local` runs against one temp data dir with scripted stdin (each buys regions), and asserts the final `world.json` reflects **both** callers' purchases and neither's spend was lost — the check `-race` cannot make across processes.
- [ ] **Step 2:** `go test ./internal/play/ -run CrossProcess -v`; green.
- [ ] **Step 3: Commit** `test(play): cross-process concurrency integration test`.

---

### Task 15: Docs + close-out

**Files:** `docs/door-setup.md` (door is now multi-node — remove the single-caller caveat, note changes visible at action boundaries), `docs/mechanics-reference.md` if it mentions single-session, `CLAUDE.md` "Primary goal" (single-caller line), the `bre-binary-verified-math`/concurrency memory as relevant.

- [ ] **Step 1:** Update docs. `go test ./...` full green (incl. `-race`), `GOARCH=386 go build ./...`.
- [ ] **Step 2: Commit** `docs(door): multi-node concurrent play; resolve #5`.

---

## Self-Review

- **Spec coverage:** §1 Store → Tasks 1–2. §2 handle → Task 3. §3 atomic rule → Tasks 5–13. §4 maintenance/onboarding → Task 12. §5 wiring → Task 4. §6 whole-file lock → Task 2. §7 testing → per-task conflict tests + Task 14. Covered.
- **Enumeration:** every mutating domain from the menu tree is a task (5–13); the 69 `w.With` sites are audited within these domains — none skipped (a final grep for un-migrated read-outside/mutate-inside patterns is part of Task 13's review).
- **Ambiguity resolved:** abort-on-conflict message + no partial mutation; nil-store fallback keeps bare-World tests working; reload-into-existing-World keeps pointers valid.
- **Risk:** the 69-site audit is where a missed re-validation hides — the per-domain conflict tests + the final grep are the net. The cross-process test (Task 14) covers what `-race` can't.
