package game

import (
	"math/rand"
	"testing"
)

// specialOpWorlds builds two boards, an attacker on one and a target on the
// other, ready to exchange one Special Operation.
func specialOpWorlds(t *testing.T) (from, to *World, attacker, target *Empire) {
	t.Helper()
	cfgA := DefaultConfig()
	cfgA.IBBS, cfgA.BoardID = true, "Alpha BBS"
	cfgA.BombingOps, cfgA.MissileOps = true, true
	from = NewWorldSeed(cfgA, 1)
	attacker = from.AddHuman("andy", "Alpha Baron")
	attacker.Bombers = BombingBombersRequired
	attacker.Gold = 10_000_000_000
	attacker.Protection = 0

	cfgB := DefaultConfig()
	cfgB.IBBS, cfgB.BoardID = true, "Bravo BBS"
	to = NewWorldSeed(cfgB, 2)
	target = to.AddHuman("ecl", "Bravo Hold")
	target.Protection = 0
	target.Regions = RegionMix{Desert: 500, Mountain: 500}
	target.syncLand()

	// The sending board knows the far realm only through the scores a packet
	// brought it, and an interplanetary missile is priced off exactly that. A
	// fixture without it is a board that has never heard of the target, which
	// the engine now refuses rather than pricing at zero.
	from.ImportBoard(RemoteBoard{
		BoardID: "Bravo BBS",
		Scores:  []RemoteScore{{Empire: target.Name, Land: target.Land}},
	})
	return from, to, attacker, target
}

// landNextBombingRun reseeds w so the next arriving bombing run passes its
// landing roll, for a test about what a run does once it lands. Two runs in
// three are driven off (BombingLandOdds), so without this a fixed seed decides
// whether such a test reaches the code it covers at all.
func landNextBombingRun(w *World) {
	for seed := int64(1); ; seed++ {
		if rand.New(rand.NewSource(seed)).Intn(BombingLandOdds) == 0 {
			w.rng = rand.New(rand.NewSource(seed))
			return
		}
	}
}

// The whole round trip: the op leaves with the gold, lands on the other board's
// realm, and the answer files a report with the baron who sent it.
func TestSpecialOpCrossesAndReportsBack(t *testing.T) {
	from, to, attacker, target := specialOpWorlds(t)
	to.FoodMarketSupply = 4000
	_ = target

	goldBefore := attacker.Gold
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "", OpBombFood, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	cost := from.SpecialOpGoldCost(attacker, OpBombFood, 0) // planet-wide: no target size
	if attacker.Gold != goldBefore-cost {
		t.Errorf("gold %d, want %d", attacker.Gold, goldBefore-cost)
	}
	if len(from.InFlight) != 1 || from.InFlight[0].Kind != "special" {
		t.Fatalf("op was not booked in flight: %+v", from.InFlight)
	}

	// The packet crosses.
	if len(from.Outbox) != 1 || len(from.Outbox[0].SpecialOps) != 1 {
		t.Fatalf("no special op queued: %+v", from.Outbox)
	}
	if !from.Outbox[0].HasPayload() {
		t.Fatal("a packet carrying only a special op reads as empty, so it would never be written")
	}
	landNextBombingRun(to)
	answer := to.ApplyPacket(from.Outbox[0])

	// A landed run burns 20-99% of the supply (TestBombingDamageMatchesTheOriginal).
	if to.FoodMarketSupply < 40 || to.FoodMarketSupply > 3200 {
		t.Errorf("the planet's food market holds %d, want 1-80%% of 4000 left", to.FoodMarketSupply)
	}
	if len(answer.Results) != 1 {
		t.Fatalf("target board sent no answer: %+v", answer.Results)
	}
	if got := answer.Results[0].Report; got == "" {
		t.Error("the answer carries no report, so the sender learns nothing")
	}

	// And the answer comes home.
	from.applyAttackResult(answer.Results[0])
	if len(from.InFlight) != 0 {
		t.Errorf("the op is still in flight after its answer arrived: %+v", from.InFlight)
	}
	if len(attacker.Events) == 0 {
		t.Fatal("the sender was told nothing")
	}
	last := attacker.Events[len(attacker.Events)-1]
	if !contains(last.Text, "Bomb Food Market") {
		t.Errorf("the report does not name the operation: %q", last.Text)
	}
}

// New Realm Protection stops a Special Operation exactly as it stops an attack
// or a terror op. A weapon that ignored the only shield a new player has would
// be the strongest thing in the game.
func TestSpecialOpBreaksOnNewRealmProtection(t *testing.T) {
	from, to, attacker, target := specialOpWorlds(t)
	target.Land, target.People = 500, 100_000
	target.Protection = 5

	// A MISSILE is the case protection covers: it is aimed at one realm. The
	// bombing ops are aimed at the planet, where a new realm's shield has
	// nothing to refuse.
	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpNuclear, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	answer := to.ApplyPacket(from.Outbox[0])
	if target.Land != 500 {
		t.Errorf("a protected realm lost land: %d", target.Land)
	}
	if got := answer.Results[0].outcome(); got != OutcomeProtected {
		t.Errorf("outcome %q, want %q", got, OutcomeProtected)
	}
}

// The bomber floor is the original's own delivery requirement and applies to
// every op on the menu, missiles included.
func TestSpecialOpNeedsBombers(t *testing.T) {
	from, _, attacker, _ := specialOpWorlds(t)
	attacker.Bombers = BombingBombersRequired - 1
	for _, op := range []SpecialOp{OpBombFood, OpNuclear} {
		if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", op, 0); err != ErrNeedBombers {
			t.Errorf("%s with too few bombers: %v, want ErrNeedBombers", op, err)
		}
	}
	if len(from.Outbox) != 0 {
		t.Error("a refused op still queued a packet")
	}
}

// The sysop's two switches govern this menu the same way they govern the local
// one: neither can be disabled on one menu and left live on the other.
func TestSpecialOpHonoursTheSysopSwitches(t *testing.T) {
	from, _, attacker, _ := specialOpWorlds(t)
	from.Config.BombingOps = false
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", OpBombFood, 0); err != ErrBombingOpsDisabled {
		t.Errorf("bombing op with Bombing Ops off: %v", err)
	}
	from.Config.BombingOps = true
	from.Config.MissileOps = false
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", OpNuclear, 0); err != ErrMissileOpsDisabled {
		t.Errorf("missile op with Missile Ops off: %v", err)
	}
}

// A packet that never comes back must not leave the op in flight for good.
func TestLostSpecialOpIsGivenUp(t *testing.T) {
	from, _, attacker, _ := specialOpWorlds(t)
	from.Config.LostForcesDays = 3
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", OpBombFood, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	from.GameDay += 4
	if n := from.ReturnLostForces(); n != 1 {
		t.Fatalf("swept %d in-flight ops, want 1", n)
	}
	if len(from.InFlight) != 0 {
		t.Errorf("the op is still in flight: %+v", from.InFlight)
	}
}

// What a landed bombing run destroys is the original's arithmetic, read from
// resolve_received_bombing (BRE.OVR 0x04a09a). Golden literals, each checked
// against the Real48 port of the linked runtime (scripts/bre_real48.py), so a
// retune of the constants fails here instead of following along.
func TestBombingDamageMatchesTheOriginal(t *testing.T) {
	for _, c := range []struct{ supply, pct, want int }{
		{1000, 20, 200}, {1000, 99, 990}, {4000, 37, 1480}, {10000, 53, 5300},
	} {
		if got := foodMarketLoss(c.supply, c.pct); got != c.want {
			t.Errorf("food market %d at %d%%: lost %d, want %d", c.supply, c.pct, got, c.want)
		}
	}
	for _, c := range []struct{ qty, pct, want int }{
		{100, 5, 95}, {37, 9, 33}, {20, 7, 18},
	} {
		if got := marketKept(c.qty, c.pct); got != c.want {
			t.Errorf("listing %d at %d%%: kept %d, want %d", c.qty, c.pct, got, c.want)
		}
	}
	for _, c := range []struct {
		value int64
		pct   int
		want  int64
	}{
		{1000, 2, 980}, {1234, 5, 1172}, {1200, 3, 1164}, {1000, 5, 950},
	} {
		if got := undermineKept(c.value, c.pct); got != c.want {
			t.Errorf("investment %d at %d%%: kept %d, want %d", c.value, c.pct, got, c.want)
		}
	}

	// The three draws cover exactly the original's ranges: Random(80)+20,
	// Random(5)+5 and Random(4)+2.
	w := NewWorldSeed(DefaultConfig(), 3)
	for _, c := range []struct {
		name     string
		draw     func() int
		min, max int
	}{
		{"food", func() int { return BombFoodMarketLossPctMin + w.rng.Intn(BombFoodMarketLossPctSpread) }, 20, 99},
		{"market", w.bombMarketLossPct, 5, 9},
		{"undermine", w.undermineLossPct, 2, 5},
	} {
		seen := map[int]bool{}
		for range 5000 {
			p := c.draw()
			if p < c.min || p > c.max {
				t.Fatalf("%s drew %d%%, outside %d-%d", c.name, p, c.min, c.max)
			}
			seen[p] = true
		}
		if len(seen) != c.max-c.min+1 {
			t.Errorf("%s drew %d distinct shares in 5000, want all %d", c.name, len(seen), c.max-c.min+1)
		}
	}

	// Undermining reaches only investments at most three days from maturity —
	// the original's first four slots of a per-day array.
	d := w.AddHuman("d", "Target")
	d.Investments = []Investment{
		{Amount: 1000, Return: 1200, MaturesDay: w.GameDay + 3},
		{Amount: 1000, Return: 1200, MaturesDay: w.GameDay + 4},
	}
	if lost := w.undermineEffect(d, 5); lost != 50 {
		t.Errorf("undermined %d, want 50 from the near investment alone", lost)
	}
	if d.Investments[0] != (Investment{Amount: 950, Return: 1140, MaturesDay: w.GameDay + 3}) {
		t.Errorf("near investment now %+v", d.Investments[0])
	}
	if d.Investments[1].Amount != 1000 || d.Investments[1].Return != 1200 {
		t.Errorf("an investment four days out was touched: %+v", d.Investments[1])
	}
}

func contains(hay, needle string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		if hay[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// The Coordinator's sweep asks every other board and is answered with EVERY
// realm they hold, not one — that is what makes it global (#48).
func TestGlobalReconRequestAsksEveryBoardAboutEveryone(t *testing.T) {
	from, to, attacker, _ := specialOpWorlds(t)
	from.RemoteBoards = []RemoteBoard{{BoardID: "Bravo BBS"}}
	second := to.AddHuman("two", "Second Hold")
	second.Land = 500

	attacker.Agents = 3
	agents := attacker.Agents
	boards, err := from.GlobalReconRequest(attacker)
	if err != nil {
		t.Fatalf("GlobalReconRequest: %v", err)
	}
	if boards != 1 {
		t.Errorf("asked %d boards, want 1", boards)
	}
	// One agent for the sweep, however many boards it reached.
	if attacker.Agents != agents-1 {
		t.Errorf("agents %d, want %d", attacker.Agents, agents-1)
	}
	answer := to.ApplyPacket(from.Outbox[0])
	if len(answer.ReconReports) != 2 {
		t.Fatalf("got %d reports, want one per living realm on the far board", len(answer.ReconReports))
	}
}

// The four bombing ops are aimed at the PLANET, so they carry no realm and are
// not refused by one realm's New Realm Protection. Getting this backwards is
// how they were first built (from the local menu's model), which cost a
// released-then-corrected ChangeLog line.
func TestBombingOpsTargetThePlanetNotABaron(t *testing.T) {
	for _, op := range []SpecialOp{OpBombFood, OpBombMarket, OpBombRoutes, OpUndermine} {
		if !op.TargetsPlanet() {
			t.Errorf("%s should be aimed at the planet", op)
		}
	}
	for _, op := range []SpecialOp{OpNuclear, OpChemical, OpSabre} {
		if op.TargetsPlanet() {
			t.Errorf("%s ruins one realm's land and must name it", op)
		}
	}

	// A named baron is discarded rather than travelling, so nothing on the far
	// side can go looking for a realm.
	from, to, attacker, target := specialOpWorlds(t)
	to.FoodMarketSupply = 1000
	target.Protection = 99 // would refuse a realm-aimed op
	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpBombFood, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	if got := from.Outbox[0].SpecialOps[0].TargetEmpire; got != "" {
		t.Errorf("the packet names %q; a planet op carries no realm", got)
	}
	landNextBombingRun(to)
	answer := to.ApplyPacket(from.Outbox[0])
	if to.FoodMarketSupply >= 1000 {
		t.Errorf("the planet's food market holds %d, want less than 1000 — protection must not shield the planet",
			to.FoodMarketSupply)
	}
	if got := answer.Results[0].outcome(); got != OutcomeWon {
		t.Errorf("outcome %q, want %q", got, OutcomeWon)
	}
}

// New Realm Protection shields a realm from a missile but not from a bombing
// op: the original's bombing receiver calls no protection test and walks every
// occupied slot (resolve_received_bombing, BRE.OVR 0x04a09a +0x308). So a
// sheltered realm's investments are undermined with its neighbors', which the
// food-market case above cannot show — that market belongs to no realm.
func TestBombingOpsIgnoreTheTargetsProtection(t *testing.T) {
	from, to, attacker, target := specialOpWorlds(t)
	target.Protection = 99
	target.Investments = []Investment{{Amount: 1000, Return: 1200, MaturesDay: to.GameDay + 3}}
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "", OpUndermine, 0); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	landNextBombingRun(to)
	answer := to.ApplyPacket(from.Outbox[0])
	if got := answer.Results[0].outcome(); got != OutcomeWon {
		t.Fatalf("outcome %q, want %q — report %q", got, OutcomeWon, answer.Results[0].Report)
	}
	// A landed run takes 2-5% (TestBombingDamageMatchesTheOriginal).
	if got := target.Investments[0].Amount; got < 950 || got > 980 {
		t.Errorf("a protected realm's investment holds %d, want 950-980: protection must not shield it", got)
	}
}

// The three interplanetary missiles are priced off the TARGET's last-known
// territory, at a rate of their own each, and uncapped. Golden literals from a
// live capture rather than a rebuild from the constants: cap/eots-ibbs-02.cap
// quotes three costs against `Pirates Ahoy!`, whose IPScores row reads 8,112
// Territory, and they divide exactly — 20,758,608 / 21,853,728 / 36,122,736.
// Asserting the constant instead would follow a retune silently, which is the
// opposite of what a fidelity figure is for.
func TestInterplanetaryMissilePricesMatchTheCapture(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("a", "Alpha")
	for _, c := range []struct {
		op   SpecialOp
		want int64
	}{
		{OpNuclear, 20_758_608},
		{OpChemical, 21_853_728},
		{OpSabre, 36_122_736},
	} {
		if got := w.SpecialOpGoldCost(e, c.op, 8112); got != c.want {
			t.Errorf("%s against 8,112 regions = %d, want %d", c.op, got, c.want)
		}
	}
	// Uncapped: StrikeCostCap is the LOCAL path's ceiling and appears in none of
	// the three interplanetary branches.
	if got := w.SpecialOpGoldCost(e, OpNuclear, 100_000); got <= StrikeCostCap {
		t.Errorf("the interplanetary price is capped at %d: got %d", StrikeCostCap, got)
	}
	// The launcher's own size does not enter into it.
	e.Regions = RegionMix{Desert: 9000}
	e.syncLand()
	if got := w.SpecialOpGoldCost(e, OpNuclear, 8112); got != 20_758_608 {
		t.Errorf("price moved with the launcher's land: %d", got)
	}
	// The sysop's Terror Costs dial scales the bombing ops, not the missiles.
	w.Config.TerrorCosts = Level(2)
	if got := w.SpecialOpGoldCost(e, OpNuclear, 8112); got != 20_758_608 {
		t.Errorf("Terror Costs moved a missile price: %d", got)
	}
}
