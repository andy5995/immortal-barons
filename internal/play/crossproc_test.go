package play

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// TestCrossProcessConcurrentPlay is the check `-race` cannot make: it drives
// TWO real, separate immortal-barons processes against ONE data directory at
// the same time. Each onboards a distinct realm and buys a distinct number of
// troopers through the menu, then the final world.json is asserted to hold
// BOTH callers' purchases — neither process's write clobbered the other's.
// The FileStore's per-action flock+reload+save is what makes that safe across
// process boundaries, where an in-process mutex (all -race can enforce) does
// nothing.
//
// Skipped in -short mode: it compiles the binary and spawns subprocesses.
func TestCrossProcessConcurrentPlay(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping cross-process integration test in -short mode")
	}

	// Build the binary once to a temp path. A build failure skips rather than
	// fails: this test exercises concurrency, not the build.
	bin := filepath.Join(t.TempDir(), "immortal-barons")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	buildCtx, cancelBuild := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancelBuild()
	build := exec.CommandContext(buildCtx, "go", "build", "-o", bin, "github.com/andy5995/immortal-barons/cmd/immortal-barons")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("could not build the binary (%v):\n%s", err, out)
	}

	// One shared data dir. Seed a world with the preferences that make the
	// turn pipeline deterministic to script: pay maintenance silently and skip
	// the Covert/Trading/Message stages, so each caller's turn is
	// Play -> (quit Diplomacy) -> (decline Change Production) -> three pauses ->
	// Spending (buy troopers, quit) -> (quit Attack) -> (decline continue) -> quit.
	dir := t.TempDir()
	cfg := cfgIn(dir)
	seed := game.NewWorld(cfg)
	seed.AutoPayMaint = true
	seed.VisitCovert = false
	seed.VisitTrading = false
	seed.VisitMessage = false
	if err := store.Save(seed, cfg); err != nil {
		t.Fatalf("seed world: %v", err)
	}

	// Splash dismiss, realm name + confirm, Play, three pauses (income, status,
	// military+maintenance), Spending: buy N troopers THREE separate times
	// (each buy is its own reload/mutate/save transaction, so each process
	// contends for the lock three times — one buy each made a lost update
	// nearly undetectable when the two processes' windows happened not to
	// interleave), quit Spending, quit Attack, decline "continue", quit the
	// Game menu. Every prompt gets its own explicit key — single-key prompts
	// drain a trailing line terminator (the Enter-leak fix), so a script may
	// not lean on a stray "\r" reaching the next prompt. -cp437 keeps the
	// session English (no language picker), so the script is
	// locale-independent.
	script := func(realm string, buy int) string {
		b := "1" + strconv.Itoa(buy) + "\r"
		return " " + realm + "\ry1   " + b + b + b + "00n0"
	}

	type player struct {
		handle, realm string
		buy           int
	}
	players := []player{
		{"alice", "Alicia", 10},
		{"bob", "Bobsland", 25},
	}

	var wg sync.WaitGroup
	for _, p := range players {
		wg.Add(1)
		go func(p player) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, bin, "-local", "-cp437", "-name", p.handle, "-data", dir)
			cmd.Stdin = strings.NewReader(script(p.realm, p.buy))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("%s process: %v\n%s", p.handle, err, out)
			}
		}(p)
	}
	wg.Wait()

	// Reload the shared world from disk and assert ALL purchases survived. A
	// new empire starts with 100 troopers and no Industrial regions, so nothing
	// but the buys changes the count — the final value is exactly 100 + 3×buy.
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("reload world: %v", err)
	}
	for _, p := range players {
		e := w.FindByOwner(p.handle)
		if e == nil {
			t.Fatalf("%s did not onboard — its process's write was lost", p.handle)
		}
		if e.Name != p.realm {
			t.Errorf("%s realm = %q, want %q", p.handle, e.Name, p.realm)
		}
		if want := 100 + 3*p.buy; e.Troopers != want {
			t.Errorf("%s troopers = %d, want %d (its purchase was lost or clobbered by the other node)", p.handle, e.Troopers, want)
		}
	}
}
