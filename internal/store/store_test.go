package store

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func cfgIn(dir string) game.Config {
	c := game.DefaultConfig()
	c.DataDir = dir
	return c
}

func TestLoadMissingReturnsErrNoWorld(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	if _, err := Load(cfg); !errors.Is(err, ErrNoWorld) {
		t.Errorf("Load of a missing world: want ErrNoWorld, got %v", err)
	}
}

func TestNewGameSeedsAI(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	cfg.AICount = 3
	w := NewGame(cfg)
	if len(w.Empires) != 3 {
		t.Errorf("NewGame should seed %d AI, got %d", 3, len(w.Empires))
	}
}

// A covert agent in the field outlives the door session that sent it. The whole
// point of queuing an operation for daily maintenance is that the sender hangs
// up in between, so a queue held only in memory would lose the fee, the agent
// and the operation together.
func TestCovertQueueSurvivesTheSession(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	a := w.AddHuman("khan", "Khan's Realm")
	d := w.AddHuman("rival", "Rivalia")
	a.Gold, a.Agents = 1_000_000_000, 10
	if _, err := w.Bribery(a, d); err != nil {
		t.Fatal(err)
	}

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if len(got.CovertQueue) != 1 {
		t.Fatalf("the queue holds %d records after a reload, want 1", len(got.CovertQueue))
	}
	rec := got.CovertQueue[0]
	if rec.Attacker != "Khan's Realm" || rec.Target != "Rivalia" || rec.Op != game.OpBribery {
		t.Errorf("queued record came back as %+v", rec)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Gold = 4242
	e.Events = []game.Event{{Text: "hello"}}
	e.Investments = []game.Investment{{Amount: 1000, Return: 1150, MaturesDay: 5}}
	w.GameDay = 7
	w.LastMaintDate = "2026-07-03"
	w.InvestRate = 42

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.GameDay != 7 || got.LastMaintDate != "2026-07-03" {
		t.Errorf("world scalars not preserved: day=%d date=%q", got.GameDay, got.LastMaintDate)
	}
	if got.InvestRate != 42 {
		t.Errorf("InvestRate=%d, want 42", got.InvestRate)
	}
	ge := got.FindByOwner("khan")
	if ge == nil || ge.Gold != 4242 || len(ge.Events) != 1 {
		t.Errorf("empire not preserved: %+v", ge)
	}
	if ge != nil {
		if len(ge.Investments) != 1 {
			t.Fatalf("Investments not preserved: %+v", ge.Investments)
		}
		inv := ge.Investments[0]
		if inv.Amount != 1000 || inv.Return != 1150 || inv.MaturesDay != 5 {
			t.Errorf("Investment fields not preserved: %+v", inv)
		}
	}
}

func TestLoadMigratesPreRegionTypesSave(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Regions = game.RegionMix{} // simulate a save written before region types

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ge := got.FindByOwner("khan")
	if ge == nil {
		t.Fatal("empire not found after load")
	}
	if ge.Regions.Total() != ge.Land {
		t.Errorf("Regions.Total()=%d, Land=%d: migration did not run", ge.Regions.Total(), ge.Land)
	}
	if ge.Land != 15 {
		t.Errorf("Land=%d, want 15", ge.Land)
	}
}

func TestSupportMigration(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	e.Support = 0 // simulate a save written before Support existed

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ge := got.FindByOwner("khan")
	if ge == nil {
		t.Fatal("empire not found after load")
	}
	if ge.Support != 100 {
		t.Errorf("Support=%d, want migrated default 100", ge.Support)
	}
}

func TestSaveIsAtomic(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	// no leftover temp file
	if _, err := os.Stat(filepath.Join(cfg.DataDir, "world.json.tmp")); !os.IsNotExist(err) {
		t.Error("temp file should not remain after save")
	}

	// The atomicity claim itself: block the temp path with a directory so the
	// staging write fails, and demand Save errors while the existing world.json
	// keeps its previous bytes. A Save rewriting world.json in place (no temp
	// file at all) would pass the leftover check above but truncate the world
	// here.
	before, err := os.ReadFile(filepath.Join(cfg.DataDir, "world.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(cfg.DataDir, "world.json.tmp"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.GameDay = 99
	if err := Save(w, cfg); err == nil {
		t.Error("Save with a blocked temp path should fail")
	}
	after, err := os.ReadFile(filepath.Join(cfg.DataDir, "world.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("a failed Save must leave the previous world.json untouched")
	}
}

// fillValue sets every exported field of a struct (recursively) to a non-zero
// value, so the round-trip test below can detect a field silently dropped from
// serialization — it would come back as its zero value.
func fillValue(v reflect.Value) {
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(7)
	case reflect.Bool:
		v.SetBool(true)
	case reflect.String:
		if v.String() == "" {
			v.SetString("x")
		}
	case reflect.Float32, reflect.Float64:
		v.SetFloat(7)
	case reflect.Slice:
		if v.Len() > 0 { // keep structure the world already has (e.g. Empires)
			for i := 0; i < v.Len(); i++ {
				fillValue(v.Index(i))
			}
			return
		}
		el := reflect.New(v.Type().Elem()).Elem()
		if el.Kind() == reflect.Pointer {
			el.Set(reflect.New(el.Type().Elem()))
		}
		fillValue(el)
		v.Set(reflect.Append(reflect.MakeSlice(v.Type(), 0, 1), el))
	case reflect.Map:
		m := reflect.MakeMap(v.Type())
		mv := reflect.New(v.Type().Elem()).Elem()
		fillValue(mv)
		mk := reflect.New(v.Type().Key()).Elem()
		fillValue(mk)
		m.SetMapIndex(mk, mv)
		v.Set(m)
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			fillValue(v.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			if v.Type().Field(i).IsExported() {
				fillValue(v.Field(i))
			}
		}
	case reflect.Pointer:
		// A nil pointer field would round-trip as nil and read as "lost", so
		// allocate one and fill it — that is what actually exercises the save.
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		fillValue(v.Elem())
	}
}

// zeroFields returns the dotted names of exported serialized fields (json:"-"
// excluded) that are zero after a round trip.
func zeroFields(prefix string, v reflect.Value) []string {
	var out []string
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || strings.HasPrefix(f.Tag.Get("json"), "-") {
			continue
		}
		fv := v.Field(i)
		name := prefix + f.Name
		if fv.Kind() == reflect.Struct {
			out = append(out, zeroFields(name+".", fv)...)
			continue
		}
		if fv.IsZero() {
			out = append(out, name)
		}
	}
	return out
}

// TestSaveLoadRoundTripAllFields fills EVERY serialized World and Empire field
// with a non-zero value, saves, loads, and demands each one comes back
// non-zero. The narrow round-trip test above checks a handful of fields; this
// one catches a load-path or repair/migration step clobbering a persisted
// field, and structure lost from slices and maps. What it CANNOT catch is a
// json tag change — the same tag drives both save and load, so a rename or an
// added "-" round-trips consistently; the frozen fixture in
// TestLoadFrozenV003Fixture is the guard for that.
func TestSaveLoadRoundTripAllFields(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorldSeed(cfg, 1)
	e := w.AddHuman("khan", "Khan's Realm")
	fillValue(reflect.ValueOf(e).Elem())
	fillValue(reflect.ValueOf(w).Elem())
	e.Land = e.Regions.Total() // EnsureRegions reconciles the mix against Land on load
	// SDI is likewise derived on load, from SDIFunding — which needs a value big
	// enough to show as at least 1%, or the derived field reads as lost.
	e.SDIFunding = 10_000_000
	e.SDI = game.SDIStrength(e.SDIFunding, e.Land)

	// Legacy fields the EnsureTreaties migration deliberately nils on load — zero
	// after a round trip is their correct state, not a loss.
	migrated := map[string]bool{
		"World.Alliances":       true,
		"Empire.AllianceOffers": true,
	}

	if err := Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if zeros := drop(zeroFields("World.", reflect.ValueOf(got).Elem()), migrated); len(zeros) > 0 {
		t.Errorf("world fields lost in the round trip: %v", zeros)
	}
	ge := got.FindByOwner("khan")
	if ge == nil {
		t.Fatal("empire lost entirely")
	}
	if zeros := drop(zeroFields("Empire.", reflect.ValueOf(ge).Elem()), migrated); len(zeros) > 0 {
		t.Errorf("empire fields lost in the round trip: %v", zeros)
	}
}

// drop filters names present in skip.
func drop(names []string, skip map[string]bool) []string {
	var out []string
	for _, n := range names {
		if !skip[n] {
			out = append(out, n)
		}
	}
	return out
}

// TestLoadFrozenV002Fixture pins the shape BEFORE v0.0.3, where mail was a list
// of bare strings. be48fb2 gave the field a struct element type with no way back,
// so a board that had not reset since v0.0.2 could not load its world at all —
// and neither could -reset, which reads the world before wiping it. Same rule as
// the v0.0.3 fixture: fix the loader, never the fixture.
func TestLoadFrozenV002Fixture(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	fixture, err := os.ReadFile("testdata/world-v0.0.2.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "world.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := got.FindByOwner("khan")
	if e == nil {
		t.Fatal("empire from the old save not found")
	}
	if len(e.Mail) != 2 {
		t.Fatalf("want 2 messages from the old save, got %d", len(e.Mail))
	}
	if e.Mail[0].Body != "Meet me at the border." || e.Mail[1].Body != "Your realm is next." {
		t.Errorf("legacy string mail must decode as the body, got %+v", e.Mail)
	}
	if e.Mail[0].From != "" || e.Mail[0].When != "" {
		t.Errorf("legacy mail carries no sender or stamp, got %+v", e.Mail[0])
	}
}

// TestLoadFrozenV003Fixture loads a world.json FROZEN at the v0.0.3 shape —
// committed bytes, not one this test wrote — so a JSON key rename breaks it
// even though save and load share the tag and every same-marshaller round trip
// stays green. It pins the load-bearing legacy aliases: the "Bulletin" key
// (NewsToday's tag exists solely for old saves), Events as bare strings
// (pre-timestamp saves), and the owner-handle market keys
// TestOwnerKeyedMarketMigratesToRealmNames reads. Editing the fixture to make
// this pass means breaking every real pre-existing world.json — don't.
func TestLoadFrozenV003Fixture(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	fixture, err := os.ReadFile("testdata/world-v0.0.3.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "world.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's rate, 12, is in the whole percents that release used; loading
	// converts it to tenths and holds it at the band's 10.0%/day ceiling.
	if got.GameDay != 7 || got.InvestRate != 100 {
		t.Errorf("world scalars: day=%d rate=%d, want 7 and 100", got.GameDay, got.InvestRate)
	}
	if len(got.NewsToday) != 1 || got.NewsToday[0] != "A bulletin line from an old save" {
		t.Errorf(`the legacy "Bulletin" key must load into NewsToday, got %v`, got.NewsToday)
	}
	e := got.FindByOwner("khan")
	if e == nil {
		t.Fatal("empire from the old save not found")
	}
	if e.Gold != 4242 || e.Bank != 900 || e.Land != 15 || e.Regions.Mountain != 5 {
		t.Errorf("empire fields: %+v", e)
	}
	if len(e.Events) != 1 || e.Events[0].Text != "A dragon attacked your regions." {
		t.Errorf("legacy string events must decode with their text, got %v", e.Events)
	}
	if !e.Events[0].When.IsZero() {
		t.Errorf("legacy events carry no stamp, got %v", e.Events[0].When)
	}
	if e.Support != 100 {
		t.Errorf("Support=0 in an old save must migrate to 100, got %d", e.Support)
	}
	// The fixture predates the inter-BBS Epoch marker (#104): EnsureEpoch must
	// give an old save its first generation rather than leaving it at the zero
	// value, which a packet reads as "no epoch info" (see game.ApplyPacket).
	if got.Epoch != 1 {
		t.Errorf("Epoch in a save from before #104 must migrate to 1, got %d", got.Epoch)
	}
}

// The frozen v0.0.3 fixture's market is keyed the way every save before the
// realm-name key wrote one — by owner handle. Nothing carries those rows over,
// since no released version's market had been traded on, but they must not
// arrive as ownerless stock either: a row with no realm still loads, and the
// market counts every row whatever it is keyed on, so the goods would be
// purchasable from a seller who does not exist.
func TestPreNameKeyedMarketRowsAreDropped(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	fixture, err := os.ReadFile("testdata/world-v0.0.3.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "world.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(cfg)
	if err != nil {
		t.Fatal(err)
	}

	for _, l := range got.Market {
		if l.Realm == "" {
			t.Errorf("a listing with no realm survived the load: %+v", l)
		}
	}
	// Counted straight off the table rather than through MarketTotalForSale,
	// which reports what one realm may see: this is about what survived the
	// load, not about who can look at it.
	for _, good := range []string{"Tank", "Trooper", "Jet"} {
		n := 0
		for _, l := range got.Market {
			if l.Good == good {
				n += l.Qty
			}
		}
		if n != 0 {
			t.Errorf("%d %s of the old handle-keyed market are still on the shelf", n, good)
		}
	}
}
