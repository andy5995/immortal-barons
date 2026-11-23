package game

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// fakeBrain answers every realm the same way and records what it was told.
type fakeBrain struct {
	orders []PirateOrder
	err    error
	failed []error
	seen   []PirateView
}

func (b *fakeBrain) Raid(v PirateView) ([]PirateOrder, error) {
	b.seen = append(b.seen, v)
	return b.orders, b.err
}
func (b *fakeBrain) Failed(err error) { b.failed = append(b.failed, err) }

func brainWorld(b PirateBrain) (*World, *Empire) {
	w := NewWorldSeed(DefaultConfig(), 1)
	if b != nil {
		w.Rules = BaseRules()
		w.Rules.Pirates = b
	}
	e := w.AddHuman("e", "Empire")
	e.Protection = 0
	stockAll(w, e, 1<<20)
	return w, e
}

func TestPirateBrainOrderRunsTheSharedRaid(t *testing.T) {
	b := &fakeBrain{orders: []PirateOrder{{Slot: 1, Spoil: SpoilJets}}}
	w, e := brainWorld(b)
	e.Jets = 33_000
	before := w.Pirates[1].LootJets
	w.maybePirateRaid(e)
	want := []PirateHit{{Faction: w.Pirates[1].Name, Slot: 1, Spoil: SpoilJets, Amount: 1000}}
	if !reflect.DeepEqual(e.PirateHits, want) {
		t.Fatalf("hits %+v, want %+v", e.PirateHits, want)
	}
	if e.Jets != 32_000 || w.Pirates[1].LootJets-before != 3000 {
		t.Errorf("jets %d, faction gained %d; want 32000 and 3000", e.Jets, w.Pirates[1].LootJets-before)
	}
	if v := b.seen[0]; v.Realm.Units.Jets != 33_000 || len(v.Factions) != len(w.Pirates) || v.Factions[1].Slot != 1 {
		t.Errorf("view %+v", v)
	}
}

func TestPirateBrainEmptyAnswerMeansNoRaid(t *testing.T) {
	b := &fakeBrain{}
	w, e := brainWorld(b)
	for range 500 {
		w.maybePirateRaid(e)
	}
	if len(e.PirateHits) != 0 || len(b.failed) != 0 {
		t.Errorf("%d hits, %d failures; want none", len(e.PirateHits), len(b.failed))
	}
	if len(b.seen) != 500 {
		t.Errorf("asked %d times, want 500", len(b.seen))
	}
}

// A refused answer raids the built-in way, consuming the RNG exactly as a
// world with no brain does when the brain drew nothing from it first.
func TestPirateBrainFailureFallsBackToTheBuiltInRaid(t *testing.T) {
	tooMany := make([]PirateOrder, ModPirateOrdersMax+1)
	for _, c := range []struct {
		name   string
		brain  *fakeBrain
		reason string
	}{
		{"error", &fakeBrain{err: errors.New("boom")}, "boom"},
		{"slot", &fakeBrain{orders: []PirateOrder{{Slot: 99, Spoil: SpoilJets}}}, "faction 99"},
		{"spoil", &fakeBrain{orders: []PirateOrder{{Spoil: SpoilAgents + 1}}}, "not something a raid takes"},
		{"gold market", &fakeBrain{orders: []PirateOrder{{Spoil: SpoilGold, FromMarket: true}}}, "gold is not traded"},
		{"too many", &fakeBrain{orders: tooMany}, "at most"},
	} {
		t.Run(c.name, func(t *testing.T) {
			w, e := brainWorld(c.brain)
			plain, pe := brainWorld(nil)
			for range 200 {
				w.maybePirateRaid(e)
				plain.maybePirateRaid(pe)
			}
			if len(pe.PirateHits) == 0 || !reflect.DeepEqual(e.PirateHits, pe.PirateHits) {
				t.Errorf("fallback raids differ from the built-in ones:\n%+v\n%+v", e.PirateHits, pe.PirateHits)
			}
			if len(c.brain.failed) != 200 || !strings.Contains(c.brain.failed[0].Error(), c.reason) {
				t.Errorf("failures %d, first %v; want 200 naming %q", len(c.brain.failed), c.brain.failed[0], c.reason)
			}
		})
	}
}
