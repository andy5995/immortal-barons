package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// twoNodeWorld seeds one world.json holding a single human empire and returns
// two independent ctx, each over its OWN *game.World + FileStore against that
// SAME file — the in-process stand-in for two concurrent BBS door nodes. A
// mutation committed through one ctx reaches the other only through the file.
//
// The conflict these tests exercise is the "realm has changed" path: a node
// gathers its active empire (caching the pointer), another node then removes an
// empire from the file, and the first node commits. Because encoding/json reuses
// *Empire pointers by slice INDEX when it unmarshals the reloaded world, the
// removed empire's slot silently takes on a surviving empire's data — so an
// action that mutated its pre-gathered pointer instead of re-resolving by handle
// would spend the WRONG empire's gold. Re-resolving inside the transaction (and
// aborting when the empire is gone) is what these tests pin down.
//
// setup tunes the seed empire before the first save so both nodes load identical
// starting balances. cfgSetup (may be nil) tunes the league config each node's
// FileStore re-applies on every reload — needed for knobs like MaxRegions, which
// repair() resets from the store's config, not from world.json.
func twoNodeWorld(t *testing.T, handle, realm string, cfgSetup func(*game.Config), setup func(*game.Empire)) (a, b *ctx, cfg game.Config) {
	t.Helper()
	cfg = game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.AICount = 0
	if cfgSetup != nil {
		cfgSetup(&cfg)
	}

	wa := game.NewWorldSeed(cfg, 1)
	p := wa.AddHuman(handle, realm)
	if setup != nil {
		setup(p)
	}
	if err := store.Save(wa, cfg); err != nil {
		t.Fatalf("seed save: %v", err)
	}
	wa.SetStore(store.NewFileStore(wa, cfg))

	wb, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("load node B: %v", err)
	}
	wb.SetStore(store.NewFileStore(wb, cfg))

	h := strings.ToLower(handle)
	a = &ctx{World: wa, handle: h, UTF8: true}
	b = &ctx{World: wb, handle: h, UTF8: true}
	return a, b, cfg
}

// committedEmpire reloads the world file fresh and returns the named empire, so
// a test can assert the on-disk truth after both nodes have acted.
func committedEmpire(t *testing.T, cfg game.Config, handle string) *game.Empire {
	t.Helper()
	return committedWorld(t, cfg).FindByOwner(strings.ToLower(handle))
}

// committedWorld reloads the world file fresh so a test can assert on-disk
// state (treaties, group attacks, mail) that isn't keyed to a single empire.
func committedWorld(t *testing.T, cfg game.Config) *game.World {
	t.Helper()
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("reload committed world: %v", err)
	}
	return w
}

// commitOnFile stands in for another BBS node committing a transaction: it loads
// the world fresh, applies mutate, and saves — so the change is visible to every
// node's next reload.
func commitOnFile(t *testing.T, cfg game.Config, mutate func(*game.World)) {
	t.Helper()
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("commit-on-file load: %v", err)
	}
	mutate(w)
	if err := store.Save(w, cfg); err != nil {
		t.Fatalf("commit-on-file save: %v", err)
	}
}

// addDecoy commits a second empire to the file (another node's realm). It occupies
// a later slice slot so that, once the acting empire ahead of it is removed, the
// pointer-reuse-by-index reload maps the acting empire's cached pointer onto the
// decoy's data — the trap the re-resolve defends against.
func addDecoy(t *testing.T, cfg game.Config, gold, bank int) int {
	t.Helper()
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Gold = gold
		d.Bank = bank
	})
	return committedEmpire(t, cfg, "decoy").Regions.Coastal
}

// TestBuyRegionsVanishedEmpireConflict proves Buy Regions re-resolves the active
// empire inside its transaction. Node B gathers its realm, another node then
// removes it (abdication/elimination) leaving only a decoy realm, and node B
// commits its purchase. The reused pointer slot now holds the decoy's data, so
// mutating the pre-gathered pointer would buy regions with the decoy's gold; the
// re-resolve instead finds no realm and aborts with "the realm has changed".
func TestBuyRegionsVanishedEmpireConflict(t *testing.T) {
	a, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 100_000_000
	})
	_ = a
	startDecoy := addDecoy(t, cfg, 100_000_000, 0)

	_ = b.Player()                                                                       // gather node B's realm (caches the pointer)
	commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) }) // another node removes it

	fb := &fakeSession{keys: []rune("C1\r0")}
	buyLand(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Regions.Coastal != startDecoy {
		t.Fatalf("decoy coastal = %d, want %d — a stale pointer bought regions for the wrong empire", d.Regions.Coastal, startDecoy)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}

// TestPaymentStageVanishedEmpireConflict proves the turn pipeline re-resolves the
// active empire inside every transaction. paymentStage is a turn-pipeline stage
// (runTurn calls it each turn); it once took a *Empire captured before its first
// w.With. Node B gathers its realm, another node removes it leaving only a decoy,
// then node B runs paymentStage with Auto-Pay on. The pointer-reuse-by-index
// reload maps node B's cached pointer onto the decoy's data, so paying off the
// stale pointer would spend the DECOY's gold on maintenance. The re-resolve finds
// no realm and aborts the stage, leaving the decoy untouched.
func TestPaymentStageVanishedEmpireConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = 100_000_000
	})
	const decoyGold = 100_000_000
	addDecoy(t, cfg, decoyGold, 0)

	_ = b.Player() // gather node B's realm (caches the pointer)
	// Another node removes alice and turns Auto-Pay on for the whole world, so the
	// stale-pointer path would silently auto-pay the decoy's maintenance.
	commitOnFile(t, cfg, func(w *game.World) {
		w.RemoveEmpire(w.FindByOwner("alice"))
		w.AutoPayMaint = true
	})

	fb := &fakeSession{}
	paymentStage(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Gold != decoyGold {
		t.Fatalf("decoy gold = %d, want %d — a stale pointer paid the wrong empire's maintenance", d.Gold, decoyGold)
	}
	if out := fb.out.String(); strings.Contains(out, "Maintenance paid") {
		t.Fatalf("paymentStage should have aborted for the vanished realm, but paid maintenance: %q", out)
	}
}

// TestBuyFoodVanishedEmpireConflict is the same conflict for Buy Food.
func TestBuyFoodVanishedEmpireConflict(t *testing.T) {
	_, b, cfg := twoNodeWorld(t, "alice", "Alethia", nil, func(p *game.Empire) {
		p.Gold = game.FoodBuyPrice * 100
	})
	commitOnFile(t, cfg, func(w *game.World) {
		d := w.AddHuman("decoy", "Decoyland")
		d.Gold = game.FoodBuyPrice * 100
	})
	startFood := committedEmpire(t, cfg, "decoy").Food

	_ = b.Player()
	commitOnFile(t, cfg, func(w *game.World) { w.RemoveEmpire(w.FindByOwner("alice")) })

	fb := &fakeSession{keys: []rune("10\r")}
	buyFoodMarket(fb, b)

	if d := committedEmpire(t, cfg, "decoy"); d.Food != startFood {
		t.Fatalf("decoy food = %d, want %d — a stale pointer bought food for the wrong empire", d.Food, startFood)
	}
	if out := fb.out.String(); !strings.Contains(out, "realm has changed") {
		t.Fatalf("node B should have aborted with the realm-changed notice, got: %q", out)
	}
}
