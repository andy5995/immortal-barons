package game

import (
	"strings"
	"testing"
)

// A Technology Agreement adds a partner's research to your own (#11). BRE bounds
// the contribution by whichever side holds LESS Technology, so the pact
// accelerates a realm that is already researching — it is not a substitute for
// holding Technology regions.
func TestTechnologyAgreementSharesTech(t *testing.T) {
	banked := func(e *Empire) int {
		n := 0
		for _, v := range e.TechSlots {
			n += v
		}
		return n
	}

	// A realm with no Technology regions gains nothing, treaty or not.
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	a.Regions = RegionMix{Coastal: 100}
	a.Land = a.Regions.Total()
	b := w.AddHuman("b", "Beta")
	b.Regions = RegionMix{Coastal: 60, Technology: 40}
	b.Land = b.Regions.Total()
	w.ProposeTreaty(a, b, "Technology Agreement")
	if !w.AcceptTreaty(b, a.Name, "Technology Agreement") {
		t.Fatal("AcceptTreaty failed")
	}
	for range 20 {
		w.advanceTech(a)
	}
	if banked(a) != 0 {
		t.Errorf("a realm holding no Technology gains nothing from the pact, got %d", banked(a))
	}

	// Two realms that both research: the one with a partner advances faster.
	mix := RegionMix{Coastal: 60, Technology: 40}
	w2 := NewWorldSeed(DefaultConfig(), 1)
	solo := w2.AddHuman("s", "Solo")
	solo.Regions, solo.Land = mix, mix.Total()
	paired := w2.AddHuman("p", "Paired")
	paired.Regions, paired.Land = mix, mix.Total()
	partner := w2.AddHuman("q", "Partner")
	partner.Regions, partner.Land = mix, mix.Total()
	w2.ProposeTreaty(paired, partner, "Technology Agreement")
	if !w2.AcceptTreaty(partner, paired.Name, "Technology Agreement") {
		t.Fatal("AcceptTreaty failed")
	}
	for range 20 {
		w2.advanceTech(solo)
		w2.advanceTech(paired)
	}
	if banked(paired) <= banked(solo) {
		t.Errorf("the pact should accelerate research: solo=%d paired=%d", banked(solo), banked(paired))
	}
}

// A Protective Trade agreement guards the two realms' trade, so a partner cannot
// bomb the other's trade routes or market (#11).
func TestProtectiveTradeGuardsTradeCovert(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	d := w.AddHuman("d", "Delta")
	partner := w.AddHuman("p", "Partner")
	a.Agents = 10
	d.Tanks = 10
	// d has a trade treaty (a target for Bomb Trade Routes) and a market listing.
	w.ProposeTreaty(d, partner, "Free Trade Agreement")
	w.AcceptTreaty(partner, d.Name, "Free Trade Agreement")
	if err := w.SetMarketListing(d, "Tank", 5, 1000); err != nil {
		t.Fatalf("list: %v", err)
	}

	w.ProposeTreaty(a, d, "Protective Trade")
	if !w.AcceptTreaty(d, a.Name, "Protective Trade") {
		t.Fatal("AcceptTreaty failed")
	}

	// Bomb Trade Routes is refused: agent kept, d's trade treaty intact.
	before := a.Agents
	msg, err := w.BombTradeRoutes(a, d)
	if err != nil {
		t.Fatalf("BombTradeRoutes: %v", err)
	}
	if !strings.Contains(msg, "guarded") {
		t.Errorf("expected a 'guarded' message, got %q", msg)
	}
	if a.Agents != before {
		t.Errorf("agent lost against a Protective Trade partner: %d -> %d", before, a.Agents)
	}
	if !w.HasTreaty(d, partner, "Free Trade Agreement") {
		t.Error("d's Free Trade treaty should be intact")
	}

	// Bomb Trading Market is refused: d's listing survives.
	if _, err := w.BombTradingMarket(a, d); err != nil {
		t.Fatalf("BombTradingMarket: %v", err)
	}
	if got := w.MarketForSale("d", "Tank"); got != 5 {
		t.Errorf("d's market listing should be untouched, got %d", got)
	}
}

func TestProposeTreatyAddsOfferAndMails(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeTreaty(a, b, fullDefenseAlliance)

	if len(b.TreatyOffers) != 1 || b.TreatyOffers[0].From != a.Name || b.TreatyOffers[0].Type != fullDefenseAlliance {
		t.Fatalf("want offer from %q, got %v", a.Name, b.TreatyOffers)
	}
	if len(b.Mail) != 1 {
		t.Fatalf("want 1 mail, got %d", len(b.Mail))
	}
	w.ProposeTreaty(a, b, fullDefenseAlliance) // duplicate = no-op
	if len(b.TreatyOffers) != 1 || len(b.Mail) != 1 {
		t.Fatal("duplicate proposal should not add another offer or mail")
	}
}

func TestAcceptTreatyFormsItAndConsumesOffer(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeTreaty(a, b, fullDefenseAlliance)
	if !w.AcceptTreaty(b, a.Name, fullDefenseAlliance) {
		t.Fatal("want AcceptTreaty to succeed")
	}
	if !w.AreAllied(a, b) || !w.AreAllied(b, a) {
		t.Error("want a mutual alliance")
	}
	if len(b.TreatyOffers) != 0 {
		t.Errorf("want the offer consumed, got %v", b.TreatyOffers)
	}
}

// A reply must reach the proposer's event log — BRE files "X accepted your ...
// proposal." there, and without it a proposal is answered with silence.
func TestTreatyReplyNotifiesTheProposer(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	c := w.AddHuman("c", "Gamma")

	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)
	w.ProposeTreaty(a, c, fullDefenseAlliance)
	w.DeclineTreaty(c, a.Name, fullDefenseAlliance)

	want := []string{
		"Beta accepted your Full Defense Alliance proposal.",
		"Gamma rejected your Full Defense Alliance proposal.",
	}
	if len(a.Events) != 2 || a.Events[0].Text != want[0] || a.Events[1].Text != want[1] {
		t.Errorf("proposer's events = %v, want %v", a.Events, want)
	}
	if len(b.Events) != 0 || len(c.Events) != 0 {
		t.Errorf("only the proposer is notified, got b=%v c=%v", b.Events, c.Events)
	}
}

// A reply with no matching offer is not a reply — it must not manufacture a
// notice on the named empire's log.
func TestTreatyReplyWithNoOfferNotifiesNobody(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)
	w.DeclineTreaty(b, a.Name, fullDefenseAlliance)

	if len(a.Events) != 0 {
		t.Errorf("want no events, got %v", a.Events)
	}
}

func TestAcceptTreatyWithNoOfferFails(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	if w.AcceptTreaty(b, a.Name, fullDefenseAlliance) {
		t.Fatal("want failure with no pending offer")
	}
	if w.AreAllied(a, b) {
		t.Error("want no alliance formed")
	}
}

func TestBreakTreatyEnds(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)
	w.BreakTreaty(a, b, fullDefenseAlliance)
	if w.AreAllied(a, b) {
		t.Error("want the alliance ended")
	}
}

// A non-defense treaty must NOT count as an alliance (no attack block).
func TestTradeTreatyIsNotAnAlliance(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	w.ProposeTreaty(a, b, "Tariff Trade Agreement")
	w.AcceptTreaty(b, a.Name, "Tariff Trade Agreement")
	if w.AreAllied(a, b) {
		t.Error("a trade treaty should not count as a defense alliance")
	}
	if !w.HasTreaty(a, b, "Tariff Trade Agreement") {
		t.Error("want the trade treaty recorded")
	}
}

func TestTargetsExcludesAllies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	for _, e := range w.Empires {
		e.Protection = 0
	}
	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)

	for _, e := range w.Targets(a) {
		if e.Name == b.Name {
			t.Errorf("want ally %q excluded from targets, got %v", b.Name, names(w.Targets(a)))
		}
	}
}

func TestEnsureTreatiesMigratesLegacyAlliances(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	// Simulate an old save: an untyped alliance and an alliance offer.
	x, y := treatyPair(a.Name, b.Name)
	w.Alliances = []string{x + "\x00" + y}
	b.AllianceOffers = []string{"Gamma"}

	w.EnsureTreaties()

	if !w.AreAllied(a, b) {
		t.Error("legacy alliance should migrate to a Full Defense Alliance")
	}
	if len(w.Alliances) != 0 {
		t.Error("legacy Alliances should be cleared")
	}
	if len(b.TreatyOffers) != 1 || b.TreatyOffers[0].From != "Gamma" || b.TreatyOffers[0].Type != fullDefenseAlliance {
		t.Errorf("legacy alliance offer should migrate, got %v", b.TreatyOffers)
	}
	if len(b.AllianceOffers) != 0 {
		t.Error("legacy AllianceOffers should be cleared")
	}
}

// Trade income pays on the SMALLER of the two populations at a fixed rate, and
// New Realm Protection cuts that rate. Golden literals from the binary — 11 a
// head for Free Trade, 6 for a Tariff, less 5/5 and 3/2 respectively for a
// protected self / partner.
//
// The rates are per BRE population unit, so the counts below are chosen to be
// clean multiples of PopBREUnitScale and every expectation is written out as
// "BRE-unit population x rate". Asserting the products of the raw People count
// instead is how this pact came to pay twenty times its price.
func TestTradeTreatyAddsIncome(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	a.People, b.People = 4000, 3000 // 200 and 150 in BRE's unit
	a.Protection, b.Protection = 0, 0
	if got := w.tradeIncome(a); got != 0 {
		t.Fatalf("no treaties -> no trade income, got %d", got)
	}

	w.setRelation(a.Name, b.Name, freeTradeAgreement)
	if got := w.tradeIncome(a); got != 150*11 {
		t.Errorf("free trade on the smaller population: got %d, want 150 x 11 = %d", got, 150*11)
	}
	b.Protection = 5
	if got := w.tradeIncome(a); got != 150*6 {
		t.Errorf("a protected partner cuts the rate to 6: got %d, want %d", got, 150*6)
	}

	b.Protection = 0
	w.setRelation(a.Name, b.Name, tariffTradeAgreement)
	if got := w.tradeIncome(a); got != 150*6 {
		t.Errorf("tariff trade: got %d, want 150 x 6 = %d", got, 150*6)
	}
	a.Protection = 5
	if got := w.tradeIncome(a); got != 150*3 {
		t.Errorf("protection of your own cuts a tariff to 3: got %d, want %d", got, 150*3)
	}
}

func TestIntelligenceAllianceLendsAgents(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	ally := w.AddHuman("c", "Gamma")
	ally.Agents = 100
	w.ProposeTreaty(a, ally, "Intelligence Alliance")
	w.AcceptTreaty(ally, a.Name, "Intelligence Alliance")
	if got := w.allyAgents(a, "Intelligence Alliance"); got != 100 {
		t.Errorf("allyAgents should sum the ally's agents (100), got %d", got)
	}
}

func names(es []*Empire) []string {
	r := make([]string, len(es))
	for i, e := range es {
		r[i] = e.Name
	}
	return r
}

// A Full Defense Alliance partner sends exactly AllyDefenseContribPct% (30%,
// BRE-verified) of its troopers, tanks, and agents to aid the defender, and that
// detachment bleeds at the defender's casualty rate.
func TestAllyDefenders30Pct(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	b.Troopers, b.Tanks, b.Agents = 1000, 500, 100
	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)

	d := w.AllyDefenders(a)
	if len(d) != 1 || d[0].Name != "Beta" {
		t.Fatalf("want Beta as the sole defender, got %v", d)
	}
	if d[0].Troopers != 300 || d[0].Tanks != 150 || d[0].Agents != 30 {
		t.Errorf("want 30%% sent (300/150/30), got %d/%d/%d", d[0].Troopers, d[0].Tanks, d[0].Agents)
	}
	if w.allyDefenseBoost(a) <= 0 {
		t.Error("ally defense boost should be positive with a tank-holding ally")
	}

	// The committed 30% (300 troopers / 150 tanks) bleeds at, say, a 20% rate:
	// 60 troopers and 30 tanks lost from Beta.
	w.bleedAllies(a, 0.20)
	if b.Troopers != 940 || b.Tanks != 470 {
		t.Errorf("bleedAllies(0.20): want Beta 940 troopers / 470 tanks, got %d / %d", b.Troopers, b.Tanks)
	}
}

// A proposal is stored on the recipient, so the sender's pending list is derived
// by scanning (#92).
func TestProposalsFromListsWhatYouSent(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	c := w.AddHuman("c", "Gamma")

	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.ProposeTreaty(c, a, fullDefenseAlliance) // incoming, not ours

	got := w.ProposalsFrom(a)
	if len(got) != 1 || got[0].To != b.Name || got[0].Type != fullDefenseAlliance {
		t.Fatalf("want only the offer a sent to b, got %v", got)
	}
	if w.AcceptTreaty(b, a.Name, fullDefenseAlliance); len(w.ProposalsFrom(a)) != 0 {
		t.Error("an answered proposal should leave the pending list")
	}
}

// Proposing a different pact to the same realm replaces the pending one, since
// only one can ever be agreed (#88/#92).
func TestSecondProposalReplacesThePending(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.ProposeTreaty(a, b, "Free Trade Agreement")

	if len(b.TreatyOffers) != 1 || b.TreatyOffers[0].Type != "Free Trade Agreement" {
		t.Fatalf("want only the newer offer pending, got %v", b.TreatyOffers)
	}
	if got := w.ProposalsFrom(a); len(got) != 1 || got[0].Type != "Free Trade Agreement" {
		t.Errorf("sender's list should follow, got %v", got)
	}
}

// Rejecting a NEW offer must not disturb the treaty already in place: only
// accepting replaces a relation (#88). Proposing a trade pact to a standing ally
// and being turned down leaves the alliance intact.
func TestRejectingANewOfferKeepsTheOldTreaty(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)

	w.ProposeTreaty(a, b, "Free Trade Agreement")
	w.DeclineTreaty(b, a.Name, "Free Trade Agreement")

	if !w.HasTreaty(a, b, fullDefenseAlliance) {
		t.Errorf("the alliance should stand, got %v", w.TreatiesBetween(a, b))
	}
	if len(b.TreatyOffers) != 0 {
		t.Errorf("the rejected offer should be gone, got %v", b.TreatyOffers)
	}
}

// A treaty of any type makes a partner; Enemy is a relation, not a treaty.
func TestTreatyPartnersCountsEveryTreatyType(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Aland")
	b := w.AddHuman("b", "Bland")
	c := w.AddHuman("c", "Cland")
	d := w.AddHuman("d", "Dland")

	w.setRelation(a.Name, b.Name, "Free Trade Agreement")
	w.setRelation(a.Name, c.Name, fullDefenseAlliance)
	w.setRelation(a.Name, d.Name, RelationEnemy)

	got := w.TreatyPartners(a)
	if len(got) != 2 {
		t.Fatalf("TreatyPartners = %d realms, want 2", len(got))
	}
	for _, e := range got {
		if e == d {
			t.Errorf("TreatyPartners included the enemy %q", e.Name)
		}
	}
}

// A Full Defense Alliance is a LOCAL pact. BRE's manual says so outright of the
// treaty ("effective only in Local Games"), and the binary agrees: the relation
// row is read for the current player's own planet and never travels in a packet.
// So an interplanetary strike meets the target's own defense and nothing else,
// and the ally bleeds nothing for a battle it was not in.
func TestFullDefenseAllianceDoesNotDefendAgainstInterplanetaryStrikes(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	victim := w.AddHuman("bob", "Rome")
	ally := w.AddHuman("carol", "Carthage")
	victim.Protection, ally.Protection = 0, 0
	victim.Troopers, victim.Turrets, victim.Tanks = 100, 10, 5
	ally.Troopers, ally.Tanks = 1_000_000, 100_000
	w.setRelation(victim.Name, ally.Name, fullDefenseAlliance)
	if len(w.AllyDefenders(victim)) != 1 {
		t.Fatal("the alliance must stand, or this proves nothing")
	}

	// One point over the victim's OWN defense. If the ally's detachment were
	// counted the strike would be repelled instead.
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "Rome", Kind: NormalAttack,
		Offense:      victim.Defense() + 1,
		Contributors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 1000}}},
	})
	if !res.Won {
		t.Error("a Full Defense Alliance reinforced an interplanetary defense; BRE keeps it to local games")
	}
	if ally.Troopers != 1_000_000 || ally.Tanks != 100_000 {
		t.Errorf("the ally bled for a battle it is not in: %d troopers / %d tanks", ally.Troopers, ally.Tanks)
	}
}

// Declaring war is the costly route, not the cheap one: BRE takes a quarter off
// both popular support and military morale (v/4*3, truncating on the divide).
// Golden literals from the binary — 99 keeps 72, not 74. Declaring on a realm
// you hold no agreement with costs nothing, because BRE offers the option only
// against a standing treaty.
func TestDeclareWarCostsSupportAndMorale(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	c := w.AddHuman("c", "Gamma")
	a.Support, a.Morale = 99, 100

	w.DeclareWar(a, c) // no pact stood
	if a.Support != 99 || a.Morale != 100 {
		t.Errorf("declaring on a realm you had no pact with should be free: %d/%d", a.Support, a.Morale)
	}

	w.ProposeTreaty(a, b, fullDefenseAlliance)
	w.AcceptTreaty(b, a.Name, fullDefenseAlliance)
	w.DeclareWar(a, b)
	if a.Support != 72 {
		t.Errorf("support after declaring war = %d, want 72", a.Support)
	}
	if a.Morale != 75 {
		t.Errorf("morale after declaring war = %d, want 75", a.Morale)
	}
	if w.AreAllied(a, b) {
		t.Error("the alliance should end the moment war is declared, as BRE ends it")
	}
	if w.Relation(a, b) != RelationEnemy {
		t.Errorf("relation = %q, want %q", w.Relation(a, b), RelationEnemy)
	}
}
