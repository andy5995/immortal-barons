package game

import "fmt"

// retire_ai.go — TEMPORARY MIGRATION, added in v0.2.0. Delete this whole file,
// the World.AIRetired field it sets, and the repair() call that drives it once
// a few releases have passed — v0.1.6 or later, and the line may go v0.1.5 to
// v0.2.0, so read it as "three or four releases on" rather than as one number.
// By then every board that had barons has loaded once and been swept.
//
// AI barons were removed as a playable feature in v0.2.0: a computer realm
// cannot be negotiated with, so the diplomacy a BRE veteran plays — sounding a
// rival out, buying a pact, timing an attack with a partner — has nothing to
// bite on, and no accept-table deep enough to fix that. The sysop settings that
// made them are gone (Config.AICount and -add-ai), but a board that already had
// them would otherwise keep them forever, with the count only ever falling.
//
// A world is swept ONCE and remembers it. Sweeping on every load instead would
// undo IB_ADD_AI, which is how a baron is seeded for testing now.

// RetireAIBarons removes every computer baron from a world saved before they
// were retired, and records that it has run so a later seeding survives.
//
// It goes through dropEmpires, so a baron's treaties, its pending offers and
// whatever it had escrowed in the trading market are forgotten with it — those
// key on the realm NAME, and the next caller to take that name would otherwise
// inherit its pacts and its goods.
//
// The removal is announced in the news rather than being silent: a board that
// had barons meets a smaller planet, a changed score table and pacts that are
// simply gone, and unexplained that reads as the game losing realms. The news
// is the only channel that reaches anyone here — SysopNotices surfaces through
// a -planetary run, which is a league board, and a league board never had a
// baron to sweep. The sysop's own account of the change is the ChangeLog.
func (w *World) RetireAIBarons() {
	if w.AIRetired {
		return
	}
	w.AIRetired = true
	var names []string
	// An empty Owner is what marks a baron, and this deliberately does NOT also
	// test Alive the way AIEmpires does: a dead baron's husk goes too, since
	// nothing is left to rebuild it.
	w.dropEmpires(func(e *Empire) bool {
		if e.Owner != "" {
			return false
		}
		names = append(names, e.Name)
		return true
	})
	if len(names) == 0 {
		return
	}
	for _, n := range names {
		w.postNews(fmt.Sprintf("%s withdraws from the planet. Its holdings pass out of the "+
			"reckoning, and any pact it held is void.", n))
	}
}
