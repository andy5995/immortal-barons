package game

import "testing"

func TestProposeAllianceAddsOfferAndMails(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeAlliance(a, b)

	if len(b.AllianceOffers) != 1 || b.AllianceOffers[0] != a.Name {
		t.Fatalf("want offer from %q, got %v", a.Name, b.AllianceOffers)
	}
	if len(b.Mail) != 1 {
		t.Fatalf("want 1 mail, got %d", len(b.Mail))
	}

	// duplicate proposal is a no-op
	w.ProposeAlliance(a, b)
	if len(b.AllianceOffers) != 1 {
		t.Fatalf("duplicate proposal should be a no-op, got %v", b.AllianceOffers)
	}
	if len(b.Mail) != 1 {
		t.Fatalf("duplicate proposal should not mail again, got %d", len(b.Mail))
	}
}

func TestAcceptAllianceFormsMutualAllianceAndConsumesOffer(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeAlliance(a, b)
	if ok := w.AcceptAlliance(b, a.Name); !ok {
		t.Fatal("want AcceptAlliance to succeed")
	}

	if !w.AreAllied(a, b) {
		t.Error("want AreAllied(a, b)")
	}
	if !w.AreAllied(b, a) {
		t.Error("want AreAllied(b, a)")
	}
	if len(b.AllianceOffers) != 0 {
		t.Errorf("want offer consumed, got %v", b.AllianceOffers)
	}
}

func TestAcceptAllianceClearsReciprocalOffer(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeAlliance(a, b)
	w.ProposeAlliance(b, a)

	if ok := w.AcceptAlliance(a, "Beta"); !ok {
		t.Fatal("want AcceptAlliance to succeed")
	}

	if !w.AreAllied(a, b) {
		t.Error("want AreAllied(a, b)")
	}
	for _, o := range b.AllianceOffers {
		if o == a.Name {
			t.Errorf("want B's offer from %q cleared, got %v", a.Name, b.AllianceOffers)
		}
	}
}

func TestAcceptAllianceWithNoOfferFails(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	if ok := w.AcceptAlliance(b, a.Name); ok {
		t.Fatal("want AcceptAlliance to fail with no pending offer")
	}
	if w.AreAllied(a, b) {
		t.Error("want no alliance formed")
	}
}

func TestTargetsExcludesAllies(t *testing.T) {
	cfg := DefaultConfig()
	cfg.AICount = 1
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")
	a.Protection = 0
	b.Protection = 0
	for _, e := range w.Empires {
		e.Protection = 0
	}

	w.ProposeAlliance(a, b)
	w.AcceptAlliance(b, a.Name)

	targets := w.Targets(a)
	for _, e := range targets {
		if e.Name == b.Name {
			t.Errorf("want ally %q excluded from targets, got %v", b.Name, names(targets))
		}
	}
	found := false
	for _, e := range targets {
		if e == w.Empires[0] {
			found = true
		}
	}
	if !found {
		t.Errorf("want non-allied living unprotected empire in targets, got %v", names(targets))
	}
}

func TestBreakAllianceEndsAlliance(t *testing.T) {
	cfg := DefaultConfig()
	w := NewWorldSeed(cfg, 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Beta")

	w.ProposeAlliance(a, b)
	w.AcceptAlliance(b, a.Name)
	if !w.AreAllied(a, b) {
		t.Fatal("setup: want alliance formed")
	}

	w.BreakAlliance(a, b)
	if w.AreAllied(a, b) {
		t.Error("want AreAllied false after BreakAlliance")
	}
}

func names(es []*Empire) []string {
	r := make([]string, len(es))
	for i, e := range es {
		r[i] = e.Name
	}
	return r
}
