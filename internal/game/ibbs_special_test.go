package game

import "testing"

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
	return from, to, attacker, target
}

// The whole round trip: the op leaves with the gold, lands on the other board's
// realm, and the answer files a report with the baron who sent it.
func TestSpecialOpCrossesAndReportsBack(t *testing.T) {
	from, to, attacker, target := specialOpWorlds(t)
	to.FoodMarketSupply = 4000
	_ = target

	goldBefore := attacker.Gold
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "", OpBombFood); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	cost := from.SpecialOpGoldCost(attacker, OpBombFood)
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
	answer := to.ApplyPacket(from.Outbox[0])

	if to.FoodMarketSupply != 2000 {
		t.Errorf("the planet's food market holds %d, want half of 4000", to.FoodMarketSupply)
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
	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpNuclear); err != nil {
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
		if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", op); err != ErrNeedBombers {
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
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", OpBombFood); err != ErrBombingOpsDisabled {
		t.Errorf("bombing op with Bombing Ops off: %v", err)
	}
	from.Config.BombingOps = true
	from.Config.MissileOps = false
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", OpNuclear); err != ErrMissileOpsDisabled {
		t.Errorf("missile op with Missile Ops off: %v", err)
	}
}

// A packet that never comes back must not leave the op in flight for good.
func TestLostSpecialOpIsGivenUp(t *testing.T) {
	from, _, attacker, _ := specialOpWorlds(t)
	from.Config.LostForcesDays = 3
	if err := from.SendSpecialOp(attacker, "Bravo BBS", "Anyone", OpBombFood); err != nil {
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

// The interplanetary ops must run the SAME effect as their local counterparts,
// not a second copy of the arithmetic that can drift when one is tuned.
func TestSpecialOpsShareTheLocalEffects(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 3)
	d := w.AddHuman("d", "Target")

	d.Food = 900
	if lost := bombFoodEffect(d); lost != 450 || d.Food != 450 {
		t.Errorf("bombFoodEffect lost %d leaving %d, want 450/450", lost, d.Food)
	}

	d.Investments = []Investment{{Amount: 1000, Return: 1200}}
	if lost := undermineEffect(d); lost != 250 {
		t.Errorf("undermineEffect lost %d, want a quarter of 1000", lost)
	}
	if d.Investments[0].Amount != 750 || d.Investments[0].Return != 950 {
		t.Errorf("investment now %+v; the return must fall by the same gold as the principal", d.Investments[0])
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
	for _, op := range []SpecialOp{OpNuclear, OpChemical, OpSlappenheimer} {
		if op.TargetsPlanet() {
			t.Errorf("%s ruins one realm's land and must name it", op)
		}
	}

	// A named baron is discarded rather than travelling, so nothing on the far
	// side can go looking for a realm.
	from, to, attacker, target := specialOpWorlds(t)
	to.FoodMarketSupply = 1000
	target.Protection = 99 // would refuse a realm-aimed op
	if err := from.SendSpecialOp(attacker, "Bravo BBS", target.Name, OpBombFood); err != nil {
		t.Fatalf("SendSpecialOp: %v", err)
	}
	if got := from.Outbox[0].SpecialOps[0].TargetEmpire; got != "" {
		t.Errorf("the packet names %q; a planet op carries no realm", got)
	}
	answer := to.ApplyPacket(from.Outbox[0])
	if to.FoodMarketSupply != 500 {
		t.Errorf("the planet's food market holds %d, want 500 — protection must not shield the planet",
			to.FoodMarketSupply)
	}
	if got := answer.Results[0].outcome(); got != OutcomeWon {
		t.Errorf("outcome %q, want %q", got, OutcomeWon)
	}
}
