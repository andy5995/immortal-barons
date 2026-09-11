package game

import (
	"sync"
	"testing"
)

// TestConcurrentPerEmpirePricing drives two distinct empires buying at the same
// time and asserts each is charged its OWN per-empire price with no cross-
// contamination — the property the whole per-empire model rests on. The two
// empires' walks are advanced separately first so their prices differ; then they
// buy concurrently. Run under -race, it also proves the buy path adds no shared-
// write race (prices are read from each empire's own stored state).
func TestConcurrentPerEmpirePricing(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e1 := w.AddHuman("alice", "Alethia")
	e2 := w.AddHuman("bob", "Bobland")

	// Diverge their walks over a handful of turns so the two prices differ.
	for i := 0; i < 6; i++ {
		w.GameDay, e1.TurnsLeft, e2.TurnsLeft = i, i, i
		w.stepPrices(e1)
		w.stepPrices(e2)
	}
	if w.UnitPrice(e1, Trooper) == w.UnitPrice(e2, Trooper) {
		t.Fatalf("walks did not diverge: both %d", w.UnitPrice(e1, Trooper))
	}

	const n = 200
	p1, p2 := w.UnitPrice(e1, Trooper), w.UnitPrice(e2, Trooper)
	e1.Troopers, e2.Troopers = 0, 0
	e1.Gold, e2.Gold = int64(n*p1), int64(n*p2)

	var wg sync.WaitGroup
	wg.Add(2)
	for _, arg := range []struct {
		e     *Empire
		price int
	}{{e1, p1}, {e2, p2}} {
		go func(e *Empire) {
			defer wg.Done()
			for i := 0; i < n; i++ {
				if err := w.Buy(e, Trooper, 1); err != nil {
					t.Errorf("%s Recruit #%d: %v", e.Name, i, err)
					return
				}
			}
		}(arg.e)
	}
	wg.Wait()

	// Each empire bought exactly n at its own price and spent its own gold to
	// zero — if pricing had leaked across empires, one would have run short.
	if e1.Troopers != n || e1.Gold != 0 {
		t.Errorf("alice: troopers=%d gold=%d, want %d and 0 (price %d)", e1.Troopers, e1.Gold, n, p1)
	}
	if e2.Troopers != n || e2.Gold != 0 {
		t.Errorf("bob: troopers=%d gold=%d, want %d and 0 (price %d)", e2.Troopers, e2.Gold, n, p2)
	}
}

func TestUnitPriceWalk(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")
	base := w.Prices.Trooper

	// A fresh empire is unseeded → sits at the world base, and reads are stable
	// within a turn (no step between two reads).
	if got := w.UnitPrice(e, Trooper); got != base {
		t.Fatalf("fresh empire trooper price = %d, want base %d", got, base)
	}
	if a, b := w.UnitPrice(e, Trooper), w.UnitPrice(e, Trooper); a != b {
		t.Fatalf("price not stable within a turn: %d vs %d", a, b)
	}

	// Walk 60 turns: it moves off base but never leaves BRE's band.
	moved := false
	seen := map[int]bool{}
	for i := 0; i < 60; i++ {
		w.GameDay, e.TurnsLeft = i/16, i%16
		w.stepPrices(e)
		p := w.UnitPrice(e, Trooper)
		if p < PriceLoTrooper || p > PriceHiTrooper {
			t.Errorf("step %d: price %d out of band [%d,%d]", i, p, PriceLoTrooper, PriceHiTrooper)
		}
		if p != base {
			moved = true
		}
		seen[p] = true
	}
	if !moved {
		t.Error("price never left base — the walk is not moving")
	}
	if len(seen) < 3 {
		t.Errorf("walk barely moved: %v", seen)
	}

	// Per-empire: a second empire walked through the same turns lands on different
	// prices (its own market), so the walk is genuinely per-empire.
	f := w.AddHuman("bob", "Bobland")
	for i := 0; i < 60; i++ {
		w.GameDay, f.TurnsLeft = i/16, i%16
		w.stepPrices(f)
	}
	if w.UnitPrice(e, Trooper) == w.UnitPrice(f, Trooper) &&
		w.UnitPrice(e, Bomber) == w.UnitPrice(f, Bomber) &&
		w.AgentPrice(e) == w.AgentPrice(f) {
		t.Error("two empires walked to identical prices on every unit; walk is not per-empire")
	}

	// Shown == charged: a buy charges the stored price (stable, no step mid-buy).
	e.Gold, e.Troopers = 1<<30, 0
	price := w.UnitPrice(e, Trooper)
	before := e.Gold
	if err := w.Buy(e, Trooper, 3); err != nil {
		t.Fatalf("Recruit: %v", err)
	}
	if spent := before - e.Gold; spent != int64(3*price) {
		t.Errorf("charged %d, shown price implies %d", spent, 3*price)
	}
}

func TestLandPriceRisesWithHoldings(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Regions = RegionMix{}
	e.Land = 0
	base := w.LandPrice(e)
	// An empty realm still pays the half step: 900 + 33/2 rounds to 917.
	if want := RegionPriceBase + (LandPerRegion+1)/2; base != want {
		t.Errorf("LandPrice at Land=0: want %d, got %d", want, base)
	}

	e.Land = 50
	higher := w.LandPrice(e)
	if higher <= base {
		t.Errorf("LandPrice should rise with holdings: base=%d higher=%d", base, higher)
	}
}

func TestBuyLandIncremental(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	// regionCost climbs with holdings, so buying 5 regions from Land=0 costs the
	// sum of the first five rising prices.
	e.Regions = RegionMix{}
	e.Land = 0
	e.Gold = 1_000_000
	startGold := e.Gold

	const n = 5
	total := 0
	for i := 0; i < n; i++ {
		total += w.regionCost(e, e.Land+i)
	}

	if err := w.BuyLand(e, n); err != nil {
		t.Fatalf("BuyLand: %v", err)
	}
	if e.Land != n {
		t.Errorf("Land: want %d, got %d", n, e.Land)
	}
	if want := startGold - int64(total); e.Gold != want {
		t.Errorf("Gold: want %d, got %d", want, e.Gold)
	}
}

func TestBuyLandRejectsWhenBroke(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Regions = RegionMix{}
	e.Land = 0

	const n = 5
	total := 0
	for i := 0; i < n; i++ {
		total += w.regionCost(e, e.Land+i)
	}
	e.Gold = int64(total - 1)
	startGold := e.Gold

	if err := w.BuyLand(e, n); err != ErrCantAfford {
		t.Errorf("BuyLand: want ErrCantAfford, got %v", err)
	}
	if e.Land != 0 {
		t.Errorf("Land should be unchanged, got %d", e.Land)
	}
	if e.Gold != startGold {
		t.Errorf("Gold should be unchanged: want %d, got %d", startGold, e.Gold)
	}
}

func TestSellLandRefundsHalf(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")

	e.Regions = RegionMix{}
	e.Land = 0
	e.Gold = 1_000_000
	startGold := e.Gold

	if err := w.BuyLand(e, 5); err != nil {
		t.Fatalf("BuyLand: %v", err)
	}
	if e.Land != 5 {
		t.Fatalf("Land: want 5, got %d", e.Land)
	}

	if err := w.SellLand(e, 5); err != nil {
		t.Fatalf("SellLand: %v", err)
	}
	if e.Land != 0 {
		t.Errorf("Land: want 0, got %d", e.Land)
	}
	if e.Gold >= startGold {
		t.Errorf("buy-then-sell should lose money: start=%d end=%d", startGold, e.Gold)
	}
}

func TestBuyFoodMarket(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 100_000
	e.Food = 0

	if err := w.BuyFoodMarket(e, 10); err != nil {
		t.Fatalf("BuyFoodMarket: %v", err)
	}
	if e.Gold != int64(100_000-10*w.FoodBuyPrice()) {
		t.Errorf("Gold: want %d, got %d", 100_000-10*w.FoodBuyPrice(), e.Gold)
	}
	if e.Food != 10 {
		t.Errorf("Food: want 10, got %d", e.Food)
	}
}

func TestFoodMarketSupplyDepletesAndReplenishes(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Gold, e.Food = 1_000_000_000, 0
	start := w.FoodMarketSupply
	if err := w.BuyFoodMarket(e, 100); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if w.FoodMarketSupply != start-100 {
		t.Errorf("supply after buy: want %d, got %d", start-100, w.FoodMarketSupply)
	}
	if err := w.SellFood(e, 40); err != nil {
		t.Fatalf("sell: %v", err)
	}
	if w.FoodMarketSupply != start-60 {
		t.Errorf("supply after sell: want %d, got %d", start-60, w.FoodMarketSupply)
	}
}

func TestFoodMarketOutOfFood(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")
	e.Gold, e.Food = 1_000_000_000, 0
	w.FoodMarketSupply = 0
	if err := w.BuyFoodMarket(e, 10); err != ErrNoFoodSupply {
		t.Errorf("want ErrNoFoodSupply, got %v", err)
	}
	w.FoodMarketSupply = 5 // buying clamps to what's left today
	if err := w.BuyFoodMarket(e, 10); err != nil {
		t.Fatalf("buy: %v", err)
	}
	if e.Food != 5 || w.FoodMarketSupply != 0 {
		t.Errorf("should buy only the 5 available: food=%d supply=%d", e.Food, w.FoodMarketSupply)
	}
}

func TestFoodUnlimitedIgnoresSupply(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FoodUnlimited = true
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("t", "T")
	e.Gold, e.Food = 1_000_000_000, 0
	w.FoodMarketSupply = 0 // empty pool, but unlimited mode ignores it
	if err := w.BuyFoodMarket(e, 100); err != nil {
		t.Fatalf("unlimited buy: %v", err)
	}
	if e.Food != 100 || w.FoodMarketSupply != 0 {
		t.Errorf("unlimited: want 100 food and untouched pool, got food=%d supply=%d", e.Food, w.FoodMarketSupply)
	}
}

func TestFoodMarketRefillsDaily(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-03"
	w.FoodMarketSupply = 123
	w.DailyMaintenance("2026-07-04")
	if w.FoodMarketSupply != FoodMarketDailySupply {
		t.Errorf("supply should refill to %d, got %d", FoodMarketDailySupply, w.FoodMarketSupply)
	}
}

func TestBuyFoodMarketCantAfford(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 5
	e.Food = 3

	if err := w.BuyFoodMarket(e, 10); err != ErrCantAfford {
		t.Fatalf("BuyFoodMarket: want ErrCantAfford, got %v", err)
	}
	if e.Gold != 5 || e.Food != 3 {
		t.Errorf("state should not mutate on failed buy: gold=%d food=%d", e.Gold, e.Food)
	}
}

func TestSellFood(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 0
	e.Food = 100

	if err := w.SellFood(e, 30); err != nil {
		t.Fatalf("SellFood: %v", err)
	}
	if e.Gold != int64(30*w.FoodSellPrice()) {
		t.Errorf("Gold: want %d, got %d", 30*w.FoodSellPrice(), e.Gold)
	}
	if e.Food != 70 {
		t.Errorf("Food: want 70, got %d", e.Food)
	}
}

func TestSellFoodClampedToOwned(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = 0
	e.Food = 5

	if err := w.SellFood(e, 100); err != nil {
		t.Fatalf("SellFood: %v", err)
	}
	if e.Food != 0 {
		t.Errorf("Food: want 0, got %d", e.Food)
	}
	if e.Gold != int64(5*w.FoodSellPrice()) {
		t.Errorf("Gold: want %d, got %d", 5*w.FoodSellPrice(), e.Gold)
	}
}

func TestStartHQ(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = int64(w.HQPrice(e))

	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	if e.HQ != HQBuildStart {
		t.Errorf("HQ: want %d, got %d", HQBuildStart, e.HQ)
	}
	if e.Gold != 0 {
		t.Errorf("Gold: want 0, got %d", e.Gold)
	}
}

func TestStartHQAlreadyStarted(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Gold = int64(w.HQPrice(e) * 2)
	if err := w.StartHQ(e); err != nil {
		t.Fatalf("StartHQ: %v", err)
	}
	goldBefore := e.Gold
	if err := w.StartHQ(e); err != ErrHQExists {
		t.Errorf("second StartHQ: want ErrHQExists, got %v", err)
	}
	if e.Gold != goldBefore {
		t.Errorf("second StartHQ should not charge again: gold %d -> %d", goldBefore, e.Gold)
	}
}

func TestStartHQCantAfford(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	short := int64(w.HQPrice(e) - 1)
	e.Gold = short

	if err := w.StartHQ(e); err != ErrCantAfford {
		t.Errorf("StartHQ: want ErrCantAfford, got %v", err)
	}
	if e.HQ != 0 {
		t.Errorf("HQ should remain 0, got %d", e.HQ)
	}
	if e.Gold != short {
		t.Errorf("Gold should be unchanged, got %d", e.Gold)
	}
}

// A sale pays the QUOTED per-unit price, n times — the division falls per unit,
// not over the total (#204, binary-verified in sellUnit's comment). It divided
// the total until 2026-09-11, which paid a shade more than the menu said.
func TestSellUnitsThirdPrice(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.Troopers = 10
	e.Gold = 0

	// Clamped to owned: selling more than owned only sells what's owned.
	if err := w.Sell(e, Trooper, 15); err != nil {
		t.Fatalf("SellTroopers: %v", err)
	}
	if e.Troopers != 0 {
		t.Errorf("Troopers: want 0, got %d", e.Troopers)
	}
	wantGold := int64(10 * UnitSellPrice(w.UnitPrice(e, Trooper)))
	if e.Gold != wantGold {
		t.Errorf("Gold: want %d, got %d", wantGold, e.Gold)
	}

	// Selling a partial amount only removes n and pays n x the quoted price.
	e.Jets = 8
	e.Gold = 0
	if err := w.Sell(e, Jet, 3); err != nil {
		t.Fatalf("SellJets: %v", err)
	}
	if e.Jets != 5 {
		t.Errorf("Jets: want 5, got %d", e.Jets)
	}
	wantGold = int64(3 * UnitSellPrice(w.UnitPrice(e, Jet)))
	if e.Gold != wantGold {
		t.Errorf("Gold: want %d, got %d", wantGold, e.Gold)
	}
}

func TestTechBoostsIncomeAndCutsMaintenance(t *testing.T) {
	cfg := DefaultConfig()

	setup := func(regions RegionMix) (*World, *Empire) {
		w := NewWorldSeed(cfg, 1)
		e := w.AddHuman("me", "Mine")
		e.Regions = regions
		e.syncLand()
		e.Gold = 0
		e.Troopers, e.Jets, e.Turrets, e.Tanks, e.Carriers = 100, 20, 30, 10, 5
		return w, e
	}

	// Isolate the bonus: identical regions, one with the ramped tech factor and
	// one without. (Swapping income-regions for Technology regions would confound
	// this — Technology produces no direct gold, so the % boost applies to a
	// smaller base; tech's payoff is the multiplier/upkeep/combat, not more land.)
	wBase, base := setup(RegionMix{Coastal: 100})
	wTech, tech := setup(RegionMix{Coastal: 100})
	// Research both the income slot and the maintenance slot: this test asserts
	// higher income AND lower upkeep, which are separate slots in BRE.
	tech.TechSlots[TechSlotGold] = 200
	tech.TechSlots[TechSlotMaint] = 200

	wBase.CollectIncome(base)
	wTech.CollectIncome(tech)

	if tech.Gold <= base.Gold {
		t.Errorf("Technology empire should net more gold: base=%d tech=%d", base.Gold, tech.Gold)
	}
	if tech.ForcesUpkeep() >= base.ForcesUpkeep() {
		t.Errorf("Technology empire should have lower upkeep: base=%d tech=%d", base.ForcesUpkeep(), tech.ForcesUpkeep())
	}
}

func TestMaxAffordableRegionsIsTrulyAffordable(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 1_000_000
	max := w.MaxAffordableRegions(e)
	if max <= 0 {
		t.Fatalf("expected to afford some regions with 1,000,000 gold, got %d", max)
	}

	// Buying exactly the reported max must succeed against a fresh identical
	// empire; one more must fail — that was the bug (prompt offered a max the
	// rising price made unaffordable).
	ok := w.AddHuman("ok", "OK")
	ok.Gold = 1_000_000
	if err := w.BuyRegions(ok, &ok.Regions.Coastal, max); err != nil {
		t.Errorf("buying the affordable max (%d) should succeed, got %v", max, err)
	}
	over := w.AddHuman("over", "Over")
	over.Gold = 1_000_000
	if err := w.BuyRegions(over, &over.Regions.Coastal, max+1); err == nil {
		t.Errorf("buying one more than the max (%d+1) should have failed", max)
	}
}

func TestRegionPurchaseCapIsCumulativePerTurn(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.MaxRegions = 5
	e := w.AddHuman("tester", "Testland")
	e.Gold = 1_000_000
	startLand := e.Land

	// Buying up to the cap works.
	if err := w.BuyLand(e, 3); err != nil {
		t.Fatalf("BuyLand(3): %v", err)
	}
	if err := w.BuyLand(e, 2); err != nil {
		t.Fatalf("BuyLand(2): %v", err)
	}
	if e.RegionsBoughtThisTurn != 5 {
		t.Fatalf("RegionsBoughtThisTurn: want 5, got %d", e.RegionsBoughtThisTurn)
	}

	// A further purchase in the same turn — even a single region, even after
	// returning to the Spending menu — must be blocked at the cap, not
	// allowed to reset per action.
	if err := w.BuyLand(e, 1); err != ErrRegionCap {
		t.Errorf("BuyLand(1) over the per-turn cap: want ErrRegionCap, got %v", err)
	}
	if e.Land != startLand+5 {
		t.Errorf("Land should be unchanged by the rejected purchase, want %d, got %d", startLand+5, e.Land)
	}

	// Once the turn advances, the counter resets and the cap is available
	// again (mirrors the reset in runTurn, internal/menu/gameflow.go).
	e.RegionsBoughtThisTurn = 0
	if err := w.BuyLand(e, 5); err != nil {
		t.Fatalf("BuyLand(5) after turn reset: %v", err)
	}
	if e.Land != startLand+10 {
		t.Errorf("Land: want %d, got %d", startLand+10, e.Land)
	}
}

// The estimate the sell prompt shows must be the same figure the turn engine
// eats: population at 75/1000, troopers at 1/200, and jets/tanks eating NOTHING
// (they were once billed double rations — the golden below fails if that comes
// back).
func TestFoodDueCountsPeopleAndForces(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("tester", "Testland")
	e.People = 2000
	e.Troopers = 400
	e.Jets = 500
	e.Tanks = 300

	if got := w.FoodDue(e); got != 152 { // 2000×75/1000 + 400/200
		t.Errorf("FoodDue: want 152, got %d", got)
	}
}

func TestBuildAndSellBombers(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("me", "Mine")
	e.Gold = 1_000_000

	if err := w.Buy(e, Bomber, 10); err != nil {
		t.Fatal(err)
	}
	if e.Bombers != 10 {
		t.Errorf("expected 10 bombers, got %d", e.Bombers)
	}
	before := e.Gold
	if err := w.Sell(e, Bomber, 5); err != nil {
		t.Fatal(err)
	}
	if e.Bombers != 5 {
		t.Errorf("expected 5 bombers after selling, got %d", e.Bombers)
	}
	if e.Gold <= before {
		t.Error("selling bombers should add gold")
	}
}

// TestAgentPriceRatchet pins BRE's covert-agent price: a fixed base, plus 20 per
// lifetime turn played, plus a draw under 300, and no cap. The windows are golden
// figures rather than the constants, so a retune fails here and has to bring new
// evidence with it.
func TestAgentPriceRatchet(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("alice", "Alethia")

	for _, tc := range []struct{ turns, lo, hi int }{
		{0, 450, 749},
		{90, 2250, 2549}, // cap/eots-covert-agents.cap shows 2335 at this turn count
		{300, 6450, 6749},
	} {
		e.TurnsPlayed = tc.turns
		if got := w.AgentPrice(e); got < tc.lo || got > tc.hi {
			t.Errorf("TurnsPlayed %d: agent price %d, want in [%d,%d]", tc.turns, got, tc.lo, tc.hi)
		}
	}

	// It ratchets. Fifteen turns of climb outrun the widest possible draw, and no
	// cap flattens the curve later, so a veteran's price keeps pulling away from a
	// newcomer's.
	e.TurnsPlayed = 0
	fresh := w.AgentPrice(e)
	e.TurnsPlayed = 15
	if veteran := w.AgentPrice(e); veteran <= fresh {
		t.Errorf("price after 15 turns is %d, not above the fresh realm's %d", veteran, fresh)
	}
}

// TestPriceWalkRevertsToMid checks the damping that makes BRE's walk cluster
// prices near the center of a band they are free to roam. Several empires, since
// the claim is about the walk and one empire is one trajectory.
func TestPriceWalkRevertsToMid(t *testing.T) {
	mid := midPrice(PriceLoBomber, PriceHiBomber)
	const turns = 400
	for _, name := range []string{"Alethia", "Bobland", "Corvus", "Dynoland"} {
		w := NewWorldSeed(DefaultConfig(), 1)
		e := w.AddHuman(name, name)
		sum := 0
		for i := 0; i < turns; i++ {
			w.GameDay, e.TurnsLeft = i/16, i%16
			w.stepPrices(e)
			sum += w.UnitPrice(e, Bomber)
		}
		if avg := sum / turns; avg < mid*9/10 || avg > mid*11/10 {
			t.Errorf("%s: mean bomber price %d over %d turns, want within 10%% of mid %d", name, avg, turns, mid)
		}
	}
}

// Region Cost Change is a BIG-REALM SURCHARGE on the per-region climb, not a
// scale on the price: below 300 regions the knob does nothing at all, and at or
// above it the level's value is ADDED to the climb. Golden literals — IB used to
// multiply the whole price by a percentage, which taxed small realms the
// original never touches (#56).
func TestRegionCostChangeIsABigRealmSurcharge(t *testing.T) {
	price := func(l Level, owned int) int {
		cfg := DefaultConfig()
		cfg.RegionCosts = l
		w := NewWorldSeed(cfg, 1)
		// Protection waives the surcharge outright, so the realm priced here
		// must be past it or every level would agree at any size.
		return w.regionCost(&Empire{Protection: 0}, owned)
	}
	// Under the threshold every level agrees, because the knob has not engaged.
	const small = RegionCostSurchargeAt - 1
	base := RegionPriceBase + small*LandPerRegion + (LandPerRegion+1)/2
	for _, l := range []Level{None, Low, Medium, High} {
		if got := price(l, small); got != base {
			t.Errorf("%v at %d regions = %d, want %d — the knob must be inert below the threshold",
				l, small, got, base)
		}
	}
	// At the threshold the level's value is added to the CLIMB, so it multiplies
	// up by the region count.
	const big = RegionCostSurchargeAt
	for _, c := range []struct {
		level Level
		climb int
	}{
		{None, 33}, {Low, 33 + 15}, {Medium, 33 + 35}, {High, 33 + 55},
	} {
		want := RegionPriceBase + big*c.climb + (c.climb+1)/2
		if got := price(c.level, big); got != want {
			t.Errorf("%v at %d regions = %d, want %d (climb %d)", c.level, big, got, want, c.climb)
		}
	}
}

// New Realm Protection waives the region-cost surcharge outright, however large
// the realm. BINARY-VERIFIED: the guard (BRE.OVR 0x3019C) tests
// is_under_protection before it compares total_regions against the threshold,
// and IB applied the surcharge regardless until 2026-09-08.
//
// The unprotected figure is a golden literal from driving BRE itself with a
// staged realm: 1,000 regions, Region Cost Change Medium, Protection Turns 0 --
// its Spending Menu quoted 68,934 a region. Asserting the constant instead
// would follow a retune silently, which is the point of the fidelity contract.
func TestProtectionWaivesTheRegionSurcharge(t *testing.T) {
	cfg := DefaultConfig()
	cfg.RegionCosts = Medium
	w := NewWorldSeed(cfg, 1)

	const owned = 1000
	shielded := w.regionCost(&Empire{Protection: 5}, owned)
	exposed := w.regionCost(&Empire{Protection: 0}, owned)

	// Both figures are BRE's own, quoted by its Spending Menu for ONE staged
	// realm of 1,000 regions at Region Cost Change Medium, with only Protection
	// Turns changed between the two runs (0, then 100).
	if exposed != 68_934 {
		t.Errorf("unprotected at %d regions should pay 68,934, got %d", owned, exposed)
	}
	if shielded != 33_917 {
		t.Errorf("protected at %d regions should pay 33,917, got %d", owned, shielded)
	}
	if shielded >= exposed {
		t.Error("protection must make land cheaper, not dearer")
	}
}
