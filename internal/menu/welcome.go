package menu

import (
	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// Welcome is the menu a brand-new caller meets before naming their realm (#28):
// the board's rules, the instructions, the help browser and the scoreboards are
// all reachable BEFORE committing to a game, rather than two menus in and a
// realm too late. It runs after the language picker, so it renders in the
// language just chosen.
//
// It reuses the opening menu's own actions — nothing here is a second rendering
// of a screen that already exists — and every one of them reads the world
// rather than the caller's empire, which is what lets them run with no empire
// yet. create is false when the caller quit instead of starting a realm.
func Welcome(s session.Session, w *game.World, handle, lang string, t Term) (create bool, err error) {
	defer session.GuardEnd(&err)
	c := &ctx{World: w, handle: handle, Term: t, lang: lang, noEmpire: true}
	menus := BuildMenus()
	c.bank = menus.Bank

	m := &Menu{Title: "Welcome", Color: ansi.FgBrightMagenta, Columns: 2}
	m.Items = []Item{
		{Key: '1', Label: "Create Realm", Do: func(session.Session, *ctx) Result {
			create = true
			return Quit
		}},
		{Key: '3', Label: "See Scores", Do: seeScores},
		{Key: 'A', Label: "Instructions", Do: showInstructions},
		{Key: 'G', Label: "Game Setup", Do: gameSetup},
		{Key: 'I', Label: "InterBBS Scores", Do: interbbsScores, Hidden: ibbsHidden},
		{Key: '?', Label: "Help", Do: helpBrowse},
		{Key: '0', Label: "Quit", Do: quit},
	}
	// Enter starts the realm, the way Enter on the opening menu starts a turn:
	// the caller came here to play, and everything else on the list is optional
	// reading on the way.
	m.DefaultOnEnter = func(g *ctx) *Item { return m.byKey('1', g) }

	if err := Run(s, c, m); err != nil {
		return false, err
	}
	return create, nil
}
