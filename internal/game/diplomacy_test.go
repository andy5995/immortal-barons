package game

import "testing"

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
