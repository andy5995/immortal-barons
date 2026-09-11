package game

import "testing"

// Every item on the InterPlanetary Special Operations menu makes the whole round
// trip: the op leaves with the gold, crosses as a packet, is resolved by the
// receiving board, and comes home as a report that names it to the baron who
// sent it. One test over all seven, so an op cannot be added to the menu with
// half a chain behind it.
//
// It asserts the CHAIN, not the damage: three of these land on a roll (a trade
// route, a missile meeting SDI), and what each does when it lands is covered
// beside the effect itself.
func TestEverySpecialOpMakesTheRoundTrip(t *testing.T) {
	for _, op := range []SpecialOp{
		OpBombFood, OpBombMarket, OpBombRoutes, OpUndermine,
		OpNuclear, OpChemical, OpSabre,
	} {
		t.Run(SpecialOpLabel(op), func(t *testing.T) {
			from, to, attacker, target := specialOpWorlds(t)
			// Something on the far planet for each op to find.
			to.FoodMarketSupply = 4000
			target.People, target.Food = 50_000, 10_000
			target.Regions = RegionMix{Desert: 500, Mountain: 500}
			target.syncLand()

			// A missile names a baron; a bombing op is aimed at the planet.
			aimed := ""
			if isMissileOp(op) {
				aimed = target.Name
			}
			goldBefore := attacker.Gold
			if err := from.SendSpecialOp(attacker, "Bravo BBS", aimed, op, 1); err != nil {
				t.Fatalf("SendSpecialOp: %v", err)
			}
			if attacker.Gold >= goldBefore {
				t.Errorf("the op was free: %d gold before, %d after", goldBefore, attacker.Gold)
			}
			if len(from.InFlight) != 1 {
				t.Fatalf("not booked in flight: %+v", from.InFlight)
			}
			if len(from.Outbox) != 1 || len(from.Outbox[0].SpecialOps) != 1 {
				t.Fatalf("no packet carries it: %+v", from.Outbox)
			}
			if !from.Outbox[0].HasPayload() {
				t.Fatal("the packet reads as empty, so it would never be written")
			}

			answer := to.ApplyPacket(from.Outbox[0])
			if len(answer.Results) != 1 {
				t.Fatalf("the receiving board answered nothing: %+v", answer.Results)
			}
			if answer.Results[0].Report == "" {
				t.Fatal("the answer carries no report, so the sender learns nothing")
			}

			from.applyAttackResult(answer.Results[0])
			if len(from.InFlight) != 0 {
				t.Errorf("still in flight after its answer came home: %+v", from.InFlight)
			}
			if len(attacker.Events) == 0 {
				t.Fatal("the sender was told nothing")
			}
			last := attacker.Events[len(attacker.Events)-1].Text
			if !contains(last, SpecialOpLabel(op)) {
				t.Errorf("the report does not name the operation: %q", last)
			}
		})
	}
}

// And the far planet hears about it: a bombing op is planet-wide, so every
// living realm there is told, and a missile is told to the realm it hit.
func TestSpecialOpsTellTheReceivingPlanet(t *testing.T) {
	for _, op := range []SpecialOp{OpBombFood, OpUndermine, OpNuclear, OpChemical} {
		t.Run(SpecialOpLabel(op), func(t *testing.T) {
			from, to, attacker, target := specialOpWorlds(t)
			to.FoodMarketSupply = 4000
			target.People = 50_000
			target.Regions = RegionMix{Desert: 500, Mountain: 500}
			target.syncLand()
			target.Investments = []Investment{{Amount: 1_000_000, Return: 1_200_000, MaturesDay: to.GameDay + 5}}

			aimed := ""
			if isMissileOp(op) {
				aimed = target.Name
			}
			if err := from.SendSpecialOp(attacker, "Bravo BBS", aimed, op, 1); err != nil {
				t.Fatalf("SendSpecialOp: %v", err)
			}
			to.ApplyPacket(from.Outbox[0])
			if len(target.Events) == 0 {
				t.Errorf("nobody on the receiving planet was told about %s", SpecialOpLabel(op))
			}
		})
	}
}
