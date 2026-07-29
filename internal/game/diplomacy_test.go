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

func TestTradeTreatyAddsIncome(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	a.People = 4000
	if got := w.tradeIncome(a); got != 0 {
		t.Fatalf("no treaties -> no trade income, got %d", got)
	}
	w.ProposeTreaty(a, b, "Free Trade Agreement")
	w.AcceptTreaty(b, a.Name, "Free Trade Agreement")
	if got := w.tradeIncome(a); got != 4000/20 {
		t.Errorf("free trade should add People/20 = %d, got %d", 4000/20, got)
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
	w.bleedAllies(a, 20)
	if b.Troopers != 940 || b.Tanks != 470 {
		t.Errorf("bleedAllies(20%%): want Beta 940 troopers / 470 tanks, got %d / %d", b.Troopers, b.Tanks)
	}
}
