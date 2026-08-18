package game

import (
	"errors"
	"strings"
	"testing"
)

// renameWorld is a two-realm world with a reference to "Old" of every kind the
// rename has to rewrite.
func renameWorld(t *testing.T) (*World, *Empire, *Empire) {
	t.Helper()
	w := NewWorldSeed(DefaultConfig(), 7)
	w.Empires = nil
	old := w.AddHuman("someone", "Old")
	other := w.AddHuman("rival", "Rival")
	old.Protection = 0
	w.Treaties = []Treaty{{Type: fullDefenseAlliance, A: "Old", B: "Rival"}}
	w.CovertQueue = []QueuedCovertOp{{Attacker: "Old", Target: "Rival"}, {Attacker: "Rival", Target: "Old", Partner: "Old"}}
	w.Market = []MarketListing{{Realm: "Old", Good: "Food", Qty: 10, Price: 5}}
	w.MarketProceeds = map[string]int64{"Old": 400}
	w.CurrentMaster, w.LastMaster = "Old", "Old"
	other.TreatyOffers = []TreatyOffer{{From: "Old", Type: fullDefenseAlliance}}
	other.TradeDeals = []TradeDeal{{From: "Old"}}
	other.Mail = []Message{{From: "Old", Body: "hello"}, {From: "Old", FromBoard: "Elsewhere", Body: "different realm"}}
	other.Bribed = []string{"Old"}
	other.ExposedFrom = map[string]int{"Old": 4}
	return w, old, other
}

func TestRenameEmpireRewritesReferences(t *testing.T) {
	w, old, other := renameWorld(t)
	if err := w.RenameEmpire(old, "New"); err != nil {
		t.Fatalf("RenameEmpire: %v", err)
	}
	if old.Name != "New" || old.FormerName != "Old" {
		t.Fatalf("empire = %q (was %q); want New (was Old)", old.Name, old.FormerName)
	}
	// Treaty names are held sorted, so the pair re-canonicalizes: "New" < "Rival".
	if got := w.Treaties[0]; got.A != "New" || got.B != "Rival" {
		t.Errorf("treaty = %q/%q, want New/Rival", got.A, got.B)
	}
	if !w.HasTreaty(old, other, fullDefenseAlliance) {
		t.Error("the alliance should survive the rename")
	}
	if w.CovertQueue[0].Attacker != "New" || w.CovertQueue[1].Target != "New" || w.CovertQueue[1].Partner != "New" {
		t.Errorf("covert queue = %+v", w.CovertQueue)
	}
	if w.Market[0].Realm != "New" {
		t.Errorf("market listing realm = %q, want New", w.Market[0].Realm)
	}
	if w.MarketProceeds["New"] != 400 || len(w.MarketProceeds) != 1 {
		t.Errorf("market proceeds = %v, want only New:400", w.MarketProceeds)
	}
	if w.CurrentMaster != "New" || w.LastMaster != "New" {
		t.Errorf("masters = %q/%q, want New/New", w.CurrentMaster, w.LastMaster)
	}
	if other.TreatyOffers[0].From != "New" {
		t.Errorf("treaty offer from = %q, want New", other.TreatyOffers[0].From)
	}
	if other.TradeDeals[0].From != "New" {
		t.Errorf("trade deal from = %q, want New", other.TradeDeals[0].From)
	}
	if other.Mail[0].From != "New" {
		t.Errorf("local mail sender = %q, want New", other.Mail[0].From)
	}
	// Mail from another planet names a realm THERE, which may share the spelling.
	if other.Mail[1].From != "Old" {
		t.Errorf("interplanetary mail sender = %q, want Old (untouched)", other.Mail[1].From)
	}
	if other.Bribed[0] != "New" {
		t.Errorf("bribed = %v, want New", other.Bribed)
	}
	if other.ExposedFrom["New"] != 4 || len(other.ExposedFrom) != 1 {
		t.Errorf("exposedFrom = %v, want only New:4", other.ExposedFrom)
	}
	// The planet is told.
	if !strings.Contains(strings.Join(w.NewsToday, "\n"), "Old is henceforth known as New.") {
		t.Errorf("news = %v, want the rename announced", w.NewsToday)
	}
}

func TestRenameEmpireIsOnceOnly(t *testing.T) {
	w, old, _ := renameWorld(t)
	if err := w.RenameEmpire(old, "New"); err != nil {
		t.Fatalf("first rename: %v", err)
	}
	if err := w.RenameEmpire(old, "Newer"); !errors.Is(err, ErrRenameUsed) {
		t.Errorf("second rename = %v, want ErrRenameUsed", err)
	}
	if old.Name != "New" {
		t.Errorf("name = %q, want New unchanged", old.Name)
	}
}

func TestRenameEmpireRefusals(t *testing.T) {
	w, old, _ := renameWorld(t)
	old.Protection = 3
	if err := w.RenameEmpire(old, "New"); !errors.Is(err, ErrRenameProtected) {
		t.Errorf("protected rename = %v, want ErrRenameProtected", err)
	}
	old.Protection = 0
	if err := w.RenameEmpire(old, " x "); !errors.Is(err, ErrRealmNameInvalid) {
		t.Errorf("a one-character name = %v, want ErrRealmNameInvalid", err)
	}
	if err := w.RenameEmpire(old, "rival"); !errors.Is(err, ErrRealmNameTaken) {
		t.Errorf("taken name = %v, want ErrRealmNameTaken", err)
	}
	if old.Name != "Old" || old.FormerName != "" {
		t.Errorf("a refused rename must change nothing: %q (was %q)", old.Name, old.FormerName)
	}
}

// A packet already in the air is addressed to the old name, and must still find
// the realm — and nobody else may take that name meanwhile.
func TestFormerNameStillResolves(t *testing.T) {
	w, old, _ := renameWorld(t)
	if err := w.RenameEmpire(old, "New"); err != nil {
		t.Fatalf("RenameEmpire: %v", err)
	}
	if got := w.FindByNameOrFormer("Old"); got != old {
		t.Errorf("FindByNameOrFormer(Old) = %v, want the renamed realm", got)
	}
	if got := w.remoteTarget("Old"); got != old {
		t.Errorf("remoteTarget(Old) = %v, want the renamed realm", got)
	}
	if !w.RealmNameTaken("old") {
		t.Error("the old name must stay reserved while the realm lives")
	}
	// A name nobody has ever held still resolves to nothing.
	if got := w.FindByNameOrFormer("Nobody"); got != nil {
		t.Errorf("FindByNameOrFormer(Nobody) = %v, want nil", got)
	}
}

func TestValidRealmName(t *testing.T) {
	for _, c := range []struct {
		name string
		want bool
	}{
		{"Vega", true},
		{"-=[ Vega ]=-", true},
		{"a1b", true},
		// A name drawn from CP437's block and line-drawing glyphs is the point of
		// #151, and carries no letters at all.
		{"╫╫╓", true},
		{"▓▒░", true},
		{"ab", false},
		{"╫╓", false},
		{"  ", false},
		{"", false},
		// An escape in a realm name would move the cursor on every screen that
		// lists it.
		{"Vega\x1b[31m", false},
	} {
		if got := ValidRealmName(c.name); got != c.want {
			t.Errorf("ValidRealmName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
