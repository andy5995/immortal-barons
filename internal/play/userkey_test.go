package play

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// A DOORDROP.INI caller carries a stable USER_KEY. It becomes the lookup
// identity, while Owner stays the handle that inter-BBS packets address.
func TestUserKeyOnboardsWithTheKey(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f := &fakeSession{keys: onboardKeys("Khanate")}
	if _, err := Run(f, Identity{Handle: "Khan", UserKey: "42-1693526400"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := w.FindByUserKey("42-1693526400")
	if e == nil {
		t.Fatal("the realm was not keyed on USER_KEY")
	}
	if e.Owner != "khan" {
		t.Errorf("Owner = %q, want the normalized handle", e.Owner)
	}
}

// A realm founded before keys were read (or by a format without one) adopts the
// key on its owner's next keyed login.
func TestUserKeyAdoptsARealmFoundedByHandle(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorld(cfg)
	w.AddHuman("khan", "Khanate")
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	f := &fakeSession{keys: []rune(" 0")} // log in, quit without playing
	if _, err := Run(f, Identity{Handle: "Khan", UserKey: "42-1693526400"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w2, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := w2.FindByUserKey("42-1693526400")
	if e == nil || e.Owner != "khan" {
		t.Fatalf("the existing realm did not adopt the key: %+v", e)
	}
	if n := len(w2.Empires); n != 1 {
		t.Errorf("got %d realms, want 1 (a second was founded)", n)
	}
}

// The key, not the alias, is what finds the realm. A caller whose board changed
// their alias keeps the same empire.
func TestUserKeyFollowsAnAliasChange(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.UserKey = "42-1693526400"
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	f := &fakeSession{keys: []rune(" 0")}
	if _, err := Run(f, Identity{Handle: "Khan Renamed", UserKey: "42-1693526400"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w2, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w2.FindByUserKey("42-1693526400") == nil {
		t.Fatal("the keyed realm was not found after the alias changed")
	}
	if n := len(w2.Empires); n != 1 {
		t.Fatalf("got %d realms, want 1: a second was founded for the new alias", n)
	}
}

// A keyed login also holds the handle lock, so a later format without USER_KEY
// cannot enter the same realm beside it.
func TestUserKeyLoginBlocksAHandleOnlyLogin(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	held, err := lockRealm(cfg, Identity{Handle: "Khan", UserKey: "42-1693526400"})
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	f := &fakeSession{keys: onboardKeys("Khanate")}
	reason, err := Run(f, Identity{Handle: "Khan"}, cfg, "2026-07-03")
	if err != nil || reason != "busy" {
		t.Fatalf("handle-only login: reason %q, err %v; want busy", reason, err)
	}
}

// A host may give a freed alias to a new account. The new account's key does
// not match the realm's, so it neither plays that realm nor founds a second one
// under the same handle.
func TestUserKeyDoesNotTakeAnotherAccountsRealm(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.UserKey = "42-1693526400"
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	f := &fakeSession{keys: onboardKeys("Second")}
	if _, err := Run(f, Identity{Handle: "Khan", UserKey: "99-1700000000"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.out.String(), "Another account's realm") {
		t.Fatalf("the refusal was not shown:\n%s", f.out.String())
	}
	w2, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(w2.Empires); n != 1 {
		t.Errorf("got %d realms, want 1", n)
	}
	if got := w2.FindByOwner("khan"); got == nil || got.UserKey != "42-1693526400" {
		t.Errorf("the original realm changed hands: %+v", got)
	}
}

// After an alias change the realm is still owned by the old handle, so a
// keyed login must take that handle's lock too: a login without a key as the
// old handle reaches the same realm.
func TestUserKeyLoginHoldsTheRealmOwnersLock(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	w := game.NewWorld(cfg)
	e := w.AddHuman("khan", "Khanate")
	e.UserKey = "42-1693526400"
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	held, err := lockRealm(cfg, Identity{Handle: "Khan"}) // a session as Khan, no key
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()

	f := &fakeSession{keys: []rune(" 0")}
	reason, err := Run(f, Identity{Handle: "Bob", UserKey: "42-1693526400"}, cfg, "2026-07-03")
	if err != nil || reason != "busy" {
		t.Fatalf("keyed login under the new alias: reason %q, err %v; want busy", reason, err)
	}
}
