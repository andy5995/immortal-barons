package game

import (
	"testing"
	"time"
)

// The weapon is scheduled to an INSTANT, not to a game day. Scheduling by day
// could only ever say 72, 48, 24 or 0 hours left, and said 0 for the whole of
// launch day, since the launch itself waits for a run.
func TestAnnihilatorLaunchesOnAnInstant(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS, cfg.BoardID = true, "boardA"
	w := NewWorldSeed(cfg, 1)
	builder := w.AddHuman("b", "Builder")
	builder.Regions = RegionMix{Desert: 5000}
	builder.Gold = 1

	funded := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := timeNow
	timeNow = func() time.Time { return funded }
	defer func() { timeNow = restore }()

	if err := w.StartAnnihilator(builder, "boardB"); err != nil {
		t.Fatalf("start: %v", err)
	}
	builder.Gold = goldCost(w.Annihilator.CostMillion, AnnihilatorMillion)
	if _, err := w.FundAnnihilator(builder, w.Annihilator.CostMillion); err != nil {
		t.Fatalf("fund: %v", err)
	}
	d := w.Annihilator
	if !d.Funded {
		t.Fatal("the weapon is paid for and not marked funded")
	}
	if want := funded.Add(AnnihilatorBuildDays * 24 * time.Hour); !d.LaunchAt.Equal(want) {
		t.Errorf("LaunchAt = %v, want %v", d.LaunchAt, want)
	}

	// The countdown is real all the way down, and never reads zero early.
	for _, c := range []struct {
		at   time.Time
		want time.Duration
	}{
		{funded, 72 * time.Hour},
		{funded.Add(70*time.Hour + 30*time.Minute), 90 * time.Minute},
		{d.LaunchAt.Add(-45 * time.Second), 45 * time.Second},
		{d.LaunchAt.Add(time.Hour), 0},
	} {
		if got := d.LaunchIn(c.at, w.GameDay); got != c.want {
			t.Errorf("LaunchIn at %v = %s, want %s", c.at, got, c.want)
		}
	}

	// It does not go early, and it does go on the run after its hour.
	w.LaunchDueAnnihilatorAt(d.LaunchAt.Add(-time.Second))
	if d.Launched {
		t.Error("the weapon launched a second early")
	}
	w.LaunchDueAnnihilatorAt(d.LaunchAt)
	if !d.Launched {
		t.Fatal("the weapon did not launch on its hour")
	}
}

// A weapon saved before launches carried an instant still goes, on the game day
// it was scheduled against.
func TestLegacyAnnihilatorStillLaunches(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.GameDay = 8
	w.Annihilator = &Annihilator{TargetBoard: "boardB", Funded: true, LaunchDay: 9, Intact: 100}

	w.LaunchDueAnnihilatorAt(time.Now())
	if w.Annihilator.Launched {
		t.Error("a legacy weapon launched a day early")
	}
	if got := w.Annihilator.LaunchIn(time.Now(), w.GameDay); got != 24*time.Hour {
		t.Errorf("LaunchIn = %s, want 24h — all a day-scheduled record knows", got)
	}
	w.GameDay = 9
	w.LaunchDueAnnihilatorAt(time.Now())
	if !w.Annihilator.Launched {
		t.Fatal("a legacy weapon never launched")
	}
}
