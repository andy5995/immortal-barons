package game

import (
	"testing"
	"time"
)

// A watcher's warning now travels twice: as the frozen news line the original
// sends, and as a record the reader's Incoming view counts down (#268).
func TestAWatcherSendsBothTheLineAndTheRecord(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "boardA"
	w := NewWorldSeed(cfg, 1)
	raider := w.AddHuman("r", "Raider")
	raider.Regions = RegionMix{Desert: 5000}
	raider.Troopers, raider.Gold = 100_000, 10_000_000

	// No watcher: nothing goes out at all.
	if _, err := w.CreateGroupAttack(raider, "Wildside", "Victim", 24, AttackForce{Troopers: 100}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if p := w.outboxFor("Wildside"); len(p.News) != 0 || len(p.Threats) != 0 {
		t.Fatalf("an unwatched planet was told: %+v", p)
	}

	w.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 3})
	p := w.outboxFor("Wildside")
	if len(p.News) == 0 {
		t.Fatal("the arriving watcher was told nothing about the party already standing")
	}
	if len(p.Threats) != 1 {
		t.Fatalf("threats = %+v, want the standing party", p.Threats)
	}
	got := p.Threats[0]
	if got.FromBoard != "boardA" || got.Kind != ThreatAttack || got.Target != "Victim" {
		t.Errorf("threat = %+v", got)
	}
	if got.When().IsZero() {
		t.Errorf("the record carries no departure: %+v", got)
	}
}

// The reader files what arrives, replaces a repeat of the same thing rather
// than listing it twice, and drops it when it is called off.
func TestThreatsReplaceAndClear(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	at := time.Now().Add(3 * time.Hour)
	w.noteThreat(Threat{FromBoard: "far", Kind: ThreatAttack, ID: 4, At: ThreatAt(at)})
	w.noteThreat(Threat{FromBoard: "far", Kind: ThreatAttack, ID: 4, Target: "Victim", At: ThreatAt(at)})
	if len(w.Threats) != 1 {
		t.Fatalf("the same party was filed twice: %+v", w.Threats)
	}
	if w.Threats[0].Target != "Victim" {
		t.Errorf("the later report did not replace the earlier: %+v", w.Threats[0])
	}
	// A different party from the same planet is its own row.
	w.noteThreat(Threat{FromBoard: "far", Kind: ThreatAttack, ID: 9, At: ThreatAt(at)})
	if len(w.Threats) != 2 {
		t.Fatalf("a second party was folded into the first: %+v", w.Threats)
	}
	w.noteThreat(Threat{FromBoard: "far", Kind: ThreatAttack, ID: 4, Gone: true})
	if len(w.Threats) != 1 || w.Threats[0].ID != 9 {
		t.Errorf("calling one off took the wrong row: %+v", w.Threats)
	}
	// A call-off for something never heard of files nothing.
	w.noteThreat(Threat{FromBoard: "far", Kind: ThreatGooie, Gone: true})
	if len(w.Threats) != 1 {
		t.Errorf("a call-off created a row: %+v", w.Threats)
	}
}

// A threat long past drops off the list; one still to come, and one whose hour
// was never named, stay.
func TestThreatsExpire(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.Threats = []Threat{
		{FromBoard: "far", Kind: ThreatAttack, ID: 1, At: ThreatAt(time.Now().Add(-(ThreatMemoryHours + 1) * time.Hour))},
		{FromBoard: "far", Kind: ThreatAttack, ID: 2, At: ThreatAt(time.Now().Add(-time.Hour))},
		{FromBoard: "far", Kind: ThreatGooie},
	}
	w.expireThreats()
	if len(w.Threats) != 2 || w.Threats[0].ID != 2 {
		t.Errorf("threats after expiry = %+v", w.Threats)
	}
}

// The Coordinator calling a party off tells the planet it was aimed at (#270),
// as a record and not only as a sentence.
func TestCallingOffAPartyClearsTheTargetsRecord(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "boardA"
	w := NewWorldSeed(cfg, 1)
	coord := w.AddHuman("c", "Coord")
	coord.Regions = RegionMix{Desert: 5000}
	coord.Troopers, coord.Gold = 100_000, 10_000_000
	coord.CoordinatorVote = "c"
	w.receiveSpyGuy(SpyGuyDispatch{FromBoard: "Wildside", Days: 3})

	ga, err := w.CreateGroupAttack(coord, "Wildside", "", 24, AttackForce{Troopers: 100})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := w.DisbandGroupAttackByCoordinator(coord, ga.ID, time.Now()); err != nil {
		t.Fatalf("call off: %v", err)
	}
	var gone bool
	for _, tr := range w.outboxFor("Wildside").Threats {
		if tr.Kind == ThreatAttack && tr.ID == ga.ID && tr.Gone {
			gone = true
		}
	}
	if !gone {
		t.Errorf("the target was not told the party was called off: %+v", w.outboxFor("Wildside").Threats)
	}
}
