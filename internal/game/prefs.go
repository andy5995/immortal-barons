package game

// Prefs are one player's own menu settings — BRE calls them "user-defined
// settings", and the Preferences menu exists so a player can skip the menus
// THEY do not use. They sat on the World, which made them the
// board's rather than the player's: three humans on one BBS shared a single
// set and each overwrote the others' every time they played.
type Prefs struct {
	EnterExitsBuy  bool
	DepositEndTurn bool
	AutoPayMaint   bool
	AutoFeed       bool
	VisitCovert    bool
	VisitTrading   bool
	VisitMessage   bool
	// Set separates a realm that has never carried its own preferences from one
	// that has turned every setting off. Without it an older save, whose
	// empires unmarshal to the zero value, is indistinguishable from a player's
	// deliberate all-off choice, and the migration below would run forever.
	Set bool
}

// DefaultPrefs is what a new realm starts on. These are IB's defaults, not
// BRE's: BRE opens with the three Visit menus and the two buy/deposit toggles
// on and the two automations off, so an untouched realm walks through every
// optional menu and answers the same maintenance and food prompts by hand each
// turn. IB starts with the walk-through menus off and the automations on,
// leaving the prompts to players who turn them back on.
func DefaultPrefs() Prefs {
	return Prefs{DepositEndTurn: true, AutoPayMaint: true, AutoFeed: true, Set: true}
}

// EnsurePrefs gives every realm its own copy of the preferences, seeded from the
// board-wide settings an older save carries. Seeding from the world rather
// than from DefaultPrefs is what keeps the upgrade invisible: whatever the board
// was set to is what each player goes on playing with.
func (w *World) EnsurePrefs() {
	legacy := Prefs{
		EnterExitsBuy:  w.EnterExitsBuy,
		DepositEndTurn: w.DepositEndTurn,
		AutoPayMaint:   w.AutoPayMaint,
		AutoFeed:       w.AutoFeed,
		VisitCovert:    w.VisitCovert,
		VisitTrading:   w.VisitTrading,
		VisitMessage:   w.VisitMessage,
		Set:            true,
	}
	for _, e := range w.Empires {
		if !e.Prefs.Set {
			e.Prefs = legacy
		}
	}
}
