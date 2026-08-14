package game

import "testing"

// A realm ground to nothing is collected as a husk, and its name goes back on
// the market. Nothing may follow the name across: BRE clears the pair's relation
// in both directions before it frees a slot, and IB must too, or the caller who
// takes the name next inherits alliances and Enemy standing they never agreed to.
func TestRebornRealmHoldsNoRelationOfItsPredecessor(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.LastMaintDate = "2026-07-02"
	doomed := w.AddHuman("owner", "Doomed")
	ally := w.AddHuman("ally", "Ally")
	foe := w.AddHuman("foe", "Foe")
	asked := w.AddHuman("asked", "Asked")
	for _, e := range []*Empire{ally, foe, asked} {
		// Played today, so the never-played and idle sweeps leave them alone and
		// the husk is the only realm this maintenance collects.
		e.LastPlayed = "2026-07-03"
	}
	w.setRelation(doomed.Name, ally.Name, fullDefenseAlliance)
	w.setRelation(doomed.Name, foe.Name, RelationEnemy)
	w.ProposeTreaty(doomed, asked, "Free Trade Agreement")

	doomed.Land = 0 // ground to nothing: dies as the day rolls
	w.DailyMaintenance("2026-07-03")

	if w.FindByName("Doomed") != nil {
		t.Fatal("the husk was not collected, so nothing below tests the reborn case")
	}
	reborn := w.AddHuman("owner", "Doomed")
	if rel := w.Relation(reborn, ally); rel != "" {
		t.Errorf("reborn realm inherited %q with its predecessor's ally", rel)
	}
	if rel := w.Relation(reborn, foe); rel != "" {
		t.Errorf("reborn realm inherited %q with its predecessor's enemy", rel)
	}
	for _, o := range asked.TreatyOffers {
		if o.From == "Doomed" {
			t.Errorf("a proposal from the dead realm still stands on %s", asked.Name)
		}
	}
	for _, tt := range w.Treaties {
		if tt.A == "Doomed" || tt.B == "Doomed" {
			t.Errorf("orphan treaty row survived the husk: %+v", tt)
		}
	}
}

// Abdication removes the realm outright, so it takes its diplomacy with it too.
func TestAbdicationForgetsItsRelations(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	quitter := w.AddHuman("q", "Quitter")
	ally := w.AddHuman("a", "Ally")
	w.setRelation(quitter.Name, ally.Name, fullDefenseAlliance)
	w.ProposeTreaty(quitter, ally, "Technology Agreement")

	w.RemoveEmpire(quitter)

	if w.FindByName("Quitter") != nil {
		t.Fatal("Quitter was not removed, so nothing below tests the abdication case")
	}
	if rel := w.Relation(w.AddHuman("q", "Quitter"), ally); rel != "" {
		t.Errorf("a realm rebuilt under the old name holds %q", rel)
	}
	if len(ally.TreatyOffers) != 0 {
		t.Errorf("the abdicated realm's proposal still stands: %v", ally.TreatyOffers)
	}
}

// The abandoned-realm sweep frees a name the same way, so it must forget the
// same rows (#83).
func TestIdleRemovalForgetsItsRelations(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Config.IdleDaysRemove = 7
	absent := w.AddHuman("gone", "Absent")
	absent.LastPlayed = "2026-06-01"
	ally := w.AddHuman("a", "Ally")
	ally.LastPlayed = "2026-07-03"
	w.setRelation(absent.Name, ally.Name, fullDefenseAlliance)

	w.removeIdleEmpires("2026-07-03")

	if w.FindByName("Absent") != nil {
		t.Fatal("the abandoned realm was not removed, so nothing below is tested")
	}
	if rel := w.Relation(w.AddHuman("gone", "Absent"), ally); rel != "" {
		t.Errorf("a realm rebuilt under the abandoned name holds %q", rel)
	}
}
