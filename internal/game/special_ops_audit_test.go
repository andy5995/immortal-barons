package game

import "testing"

// Every item on the InterPlanetary Special Operations menu makes the whole round
// trip: the op leaves with the gold, crosses as a packet, is resolved by the
// receiving board, and comes home to the sending planet's news, and for a
// missile that reached its target to the baron who sent it as well (the
// original tells a bombing run's firer, and a failed Sabre's, through the news
// alone). One test over all seven, so an op cannot be added to the menu with
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

			answer := to.receive(from.Outbox[0])
			if len(answer.Results) != 1 {
				t.Fatalf("the receiving board answered nothing: %+v", answer.Results)
			}
			res := answer.Results[0]
			if isMissileOp(op) && res.Report == nil {
				t.Fatal("the answer carries no report, so the sender learns nothing")
			}

			newsBefore := len(from.NewsToday)
			from.applyAttackResult(res, nil)
			if len(from.InFlight) != 0 {
				t.Errorf("still in flight after its answer came home: %+v", from.InFlight)
			}
			if got := from.NewsToday[newsBefore:]; len(got) != 1 || !contains(got[0].Text, attacker.Name) {
				t.Fatalf("the sending planet's news did not name the sender: %v", got)
			}
			inPerson := isMissileOp(op) && !(op == OpSabre && sabreReturnFailed(res))
			if !inPerson {
				if len(attacker.Events) != 0 {
					t.Errorf("told in person, where the original uses the news alone: %v", attacker.Events)
				}
				return
			}
			if len(attacker.Events) == 0 {
				t.Fatal("the sender was told nothing")
			}
			last := attacker.Events[len(attacker.Events)-1].Text
			weapon := map[SpecialOp]string{OpNuclear: "nuclear strike", OpChemical: "chemical strike", OpSabre: "S3-Sabre"}[op]
			if !contains(last, weapon) {
				t.Errorf("the report does not name the weapon: %q", last)
			}
			if !contains(last, target.Name) || !contains(last, "Bravo BBS") {
				t.Errorf("the report does not name the target and its board: %q", last)
			}
		})
	}
}

// And the far planet hears about it: a bombing op through its news alone, as the
// original's receiver writes no per-realm event, and a missile to the realm it
// hit as well.
func TestSpecialOpsTellTheReceivingPlanet(t *testing.T) {
	for _, op := range []SpecialOp{OpBombFood, OpUndermine, OpNuclear, OpChemical} {
		t.Run(SpecialOpLabel(op), func(t *testing.T) {
			from, to, attacker, target := specialOpWorlds(t)
			to.FoodMarketSupply = 4000
			target.People = 50_000
			target.Regions = RegionMix{Desert: 500, Mountain: 500}
			target.syncLand()
			target.Investments = []Investment{{Amount: 1_000_000, Return: 1_200_000, MaturesDay: to.GameDay + 3}}

			aimed := ""
			if isMissileOp(op) {
				aimed = target.Name
			}
			if err := from.SendSpecialOp(attacker, "Bravo BBS", aimed, op, 1); err != nil {
				t.Fatalf("SendSpecialOp: %v", err)
			}
			if !isMissileOp(op) {
				landNextBombingRun(to) // a run driven off tells nobody but the news
			}
			newsBefore := len(to.NewsToday)
			to.receive(from.Outbox[0])
			if len(to.NewsToday) == newsBefore {
				t.Errorf("the receiving planet's news says nothing about %s", SpecialOpLabel(op))
			}
			if told := len(target.Events) > 0; told != isMissileOp(op) {
				t.Errorf("%s: target told in person = %v, want %v", SpecialOpLabel(op), told, isMissileOp(op))
			}
		})
	}
}
