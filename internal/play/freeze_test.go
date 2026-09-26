package play

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

// A caller on a frozen league meets the Coordinator's message after the splash
// and goes no further: no maintenance notice, no onboarding, no menu.
func TestAFrozenLeagueLetsNobodyIn(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune(" \r1Khanate\r0")}
	if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w.Frozen, w.FreezeMessage = true, "Back in 2-48 hours."
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}

	f2 := &fakeSession{keys: []rune("  ")}
	reason, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-04")
	if err != nil {
		t.Fatal(err)
	}
	out := f2.out.String()
	if reason != "frozen" {
		t.Errorf("reason = %q, want \"frozen\"\n%s", reason, out)
	}
	for _, want := range []string{"The league is paused", "Back in 2-48 hours."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "daily maintenance") {
		t.Errorf("a frozen board ran its day:\n%s", out)
	}
}

// The paused-league line is shown in the caller's own language, although it
// comes before the menu engine that normally supplies it.
func TestAFrozenLeagueSpeaksTheCallersLanguage(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	f1 := &fakeSession{keys: []rune(" \r1Khanate\r0")}
	if _, err := Run(f1, Identity{Handle: "Khan"}, cfg, "2026-07-03"); err != nil {
		t.Fatal(err)
	}
	w, err := store.Load(cfg)
	if err != nil {
		t.Fatal(err)
	}
	w.Frozen = true
	w.FindByOwner("khan").Language = "de"
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	f2 := &fakeSession{keys: []rune("  ")}
	if _, err := Run(f2, Identity{Handle: "Khan"}, cfg, "2026-07-04"); err != nil {
		t.Fatal(err)
	}
	if out := f2.out.String(); !strings.Contains(out, "Die Liga ist pausiert") {
		t.Errorf("the paused line is not in German:\n%s", out)
	}
}
