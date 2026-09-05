package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The played-today '+' is league-only (#249). Off a league it would tell an
// attacker which neighbours have not been on today — by the rows it does NOT
// mark — so it is drawn for nobody. The (O) online mark is unaffected either way.
func TestPlayedTodayMarkIsLeagueOnly(t *testing.T) {
	for _, tc := range []struct {
		name string
		ibbs bool
		want string
	}{
		{"in a league", true, presencePlayed},
		{"on its own", false, presenceNone},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := game.DefaultConfig()
			cfg.AICount = 0
			cfg.IBBS = tc.ibbs
			w := game.NewWorldSeed(cfg, 1)
			w.AddHuman("alice", "Alicia")
			rival := w.AddHuman("bob", "Bobbia")
			rival.LastPlayed = w.Today
			c := &ctx{World: w, handle: "alice"}

			if got := presenceOf(c, rival, false, w.Today); got != tc.want {
				t.Errorf("presence = %q, want %q", got, tc.want)
			}
		})
	}
}

// Online is a different fact and stays visible whether or not a league is on.
func TestOnlineMarkIsNotLeagueGated(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 0
	cfg.IBBS = false
	w := game.NewWorldSeed(cfg, 1)
	w.AddHuman("alice", "Alicia")
	rival := w.AddHuman("bob", "Bobbia")
	rival.LastPlayed = w.Today
	rival.MarkActive()
	c := &ctx{World: w, handle: "alice"}

	if got := presenceOf(c, rival, false, w.Today); got != presenceOnline {
		t.Errorf("presence = %q, want %q", got, presenceOnline)
	}
}
