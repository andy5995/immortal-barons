package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// playersWorld writes a two-realm game to a temp data dir and returns its config.
func playersWorld(t *testing.T) game.Config {
	t.Helper()
	dir := t.TempDir()
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := store.NewGame(cfg)
	w.Empires = nil
	w.AddHuman("oldhandle", "Selby")
	w.AddHuman("rival", "Buyar")
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func runPlayersScript(t *testing.T, cfg game.Config, keys ...string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runPlayers(cfg, strings.NewReader(strings.Join(keys, "\n")+"\n"), &out); err != nil {
		t.Fatalf("runPlayers: %v", err)
	}
	return out.String()
}

func TestPlayersEditorRenamesTheCaller(t *testing.T) {
	cfg := playersWorld(t)
	out := runPlayersScript(t, cfg, "a", "p", "NewHandle", "q", "q")
	if !strings.Contains(out, "Selby") || !strings.Contains(out, "oldhandle") {
		t.Fatalf("the listing should name the realm and its caller:\n%s", out)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := w.FindByOwner("newhandle")
	if e == nil || e.Name != "Selby" {
		t.Fatalf("Selby should now answer to newhandle; got %+v", e)
	}
}

func TestPlayersEditorRenamesTheRealm(t *testing.T) {
	cfg := playersWorld(t)
	runPlayersScript(t, cfg, "A", "R", "Selby Prime", "q", "q")
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	e := w.FindByOwner("oldhandle")
	if e == nil || e.Name != "Selby Prime" {
		t.Fatalf("realm = %+v, want Selby Prime", e)
	}
	// The sysop rename must leave the player's own rename unspent.
	if e.FormerName != "" {
		t.Errorf("FormerName = %q, want empty", e.FormerName)
	}
	if w.FindByNameOrFormer("Selby") != e {
		t.Error("the old name should still take delivery")
	}
}

func TestPlayersEditorDeletesOnlyOnConfirmation(t *testing.T) {
	cfg := playersWorld(t)
	// Anything but yes leaves the realm alone — Enter included.
	runPlayersScript(t, cfg, "A", "D", "", "q", "q")
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w.FindByOwner("oldhandle") == nil {
		t.Fatal("Selby should survive an unconfirmed delete")
	}

	runPlayersScript(t, cfg, "A", "D", "y", "q")
	if w, err = store.Load(cfg); err != nil {
		t.Fatal(err)
	}
	if w.FindByOwner("oldhandle") != nil {
		t.Error("Selby should be gone after a confirmed delete")
	}
	if w.FindByOwner("rival") == nil {
		t.Error("the other realm should be untouched")
	}
}

// A refused edit reports why and changes nothing, rather than dropping out of
// the mode.
func TestPlayersEditorReportsARefusedRename(t *testing.T) {
	cfg := playersWorld(t)
	out := runPlayersScript(t, cfg, "A", "R", "Buyar", "q", "q")
	if !strings.Contains(out, game.ErrRealmNameTaken.Error()) {
		t.Fatalf("want the taken-name refusal:\n%s", out)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if e := w.FindByOwner("oldhandle"); e == nil || e.Name != "Selby" {
		t.Errorf("realm = %+v, want Selby unchanged", e)
	}
}

// The realm is picked by its roster letter, as every other game screen
// addresses it — not by a row number, which would move as realms come and go.
func TestPlayersEditorPicksByRosterLetter(t *testing.T) {
	cfg := playersWorld(t)
	out := runPlayersScript(t, cfg, "1", "Z", "q")
	for _, want := range []string{"No realm answers to 1.", "No realm answers to Z."} {
		if !strings.Contains(out, want) {
			t.Errorf("want %q in:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "B  Buyar") {
		t.Errorf("the listing should letter each realm:\n%s", out)
	}
}

// An empty roster is said plainly, the way the original does, rather than
// printed as a table with no rows.
func TestPlayersEditorSaysWhenNobodyIsPlaying(t *testing.T) {
	dir := t.TempDir()
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	w := store.NewGame(cfg)
	w.Empires = nil
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	out := runPlayersScript(t, cfg)
	if !strings.Contains(out, "No players are in this game yet.") {
		t.Errorf("out = %q", out)
	}
}
