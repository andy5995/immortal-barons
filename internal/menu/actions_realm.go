package menu

import (
	"fmt"
	"io"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_realm.go — the three things a baron may do to their own realm's
// standing: give it up, waive its new-realm protection, and spend its one
// rename.

// abdicate ends the player's empire (BRE.DOC: "delete your empire from the game
// so you may start over the next day").
//
// ONE yes/no confirmation, defaulting to No — BRE's `confirm_end_game` asks
// "Are you POSITIVE you wish to erase your empire?" and takes the answer, with
// "Glad you changed your mind!" on a refusal. IB made the player retype their
// realm name instead, which is not the original's and was justified by the same
// mistaken idea that realm names get typed anywhere: they do not, every picker
// in the game selects by Id letter.
//
// The realm is marked dead (not removed) so the same next-day rule as a
// battlefield death applies: the husk lingers until a LATER day, and only then
// does a login rebuild a fresh realm. Daily maintenance sweeps the husk once
// GameDay passes DiedDay.
func abdicate(s session.Session, w *ctx) Result {
	p := w.Player()
	fmt.Fprintf(s, "\n%s"+tr(s, "Abdicating deletes %s permanently. This cannot be undone.")+"%s\n",
		ansi.FgBrightRed, p.Name, ansi.Reset)
	if !AskYesNo(s, "Are you POSITIVE you wish to erase your empire?", false) {
		okNoPause(s, "Glad you changed your mind!")
		pause(s)
		return Stay
	}
	// Re-resolve inside the transaction: a p captured before the confirmation
	// prompt could rebind to the WRONG empire after a reload reshaped the empire
	// set. Mark the freshly-resolved empire dead instead of removing it, so the
	// husk survives to enforce the next-day rebuild rule.
	w.With(func() {
		if p := w.Player(); p != nil {
			w.Kill(p)
		}
	})
	// The session ENDS here, as the original's does — it signs off and hangs up
	// rather than returning the player to a menu their realm no longer belongs
	// to. session.End unwinds to GameLoop even from a nested Run, the same route
	// endCollapsed takes for an elimination.
	//
	// The wording is IB's own: the original's two sign-off lines are its display
	// text, which this project reconstructs rather than copies.
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, tr(s, "Your empire is no more. Return on a later day to build a new realm."), ansi.Reset)
	pause(s)
	session.End(io.EOF)
	return Quit
}

// endProtection lets a player waive their remaining new-realm protection early
// (IB's own — BRE has no such option). It is one-way: the realm becomes open to
// attacks and pirate raids at once, so it takes a confirmation.
func endProtection(s session.Session, w *ctx) Result {
	p := w.Player()
	if p.Protection <= 0 {
		okNoPause(s, "Your realm is not under new-realm protection.")
		return Stay
	}
	fmt.Fprintf(s, "\n%s"+tr(s, "Your realm has %d turns of new-realm protection left.")+"%s\n",
		ansi.FgBrightYellow, p.Protection, ansi.Reset)
	if !AskYesNo(s, "You will be out of protection at the end of this turn. Are you sure?", false) {
		return Stay
	}
	w.With(func() {
		if fp := w.Player(); fp != nil {
			w.World.WaiveProtection(fp)
		}
	})
	ok(s, "Your new-realm protection will end when this turn does.")
	return Stay
}

// changeRealmName renames the realm, once in its life (IB's own — BRE has no
// rename). It is refused while the realm is under New Realm Protection: a realm
// nobody can touch should not also be able to shed the name its rivals know it
// by. The item stays on the menu, drawn dim, until the rename is spent, so a
// player who reads about it under protection can find it again afterwards.
func changeRealmName(s session.Session, w *ctx) Result {
	p := w.Player()
	if p.Protection > 0 {
		fail(s, game.ErrRenameProtected)
		return Stay
	}
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightYellow,
		WrapIndented(tr(s, "A realm may be renamed once and never again."), ""), ansi.Reset)
	// The same naming screen the realm was christened on (AskRealmName), so the
	// prompt, the rejection and the confirmation read alike. An empty line here
	// cancels rather than offering to quit — this is a menu, not onboarding.
	name, cancelled := AskRealmName(s, playerLang(w), "",
		func(n string) bool {
			var taken bool
			w.With(func() { taken = w.RealmNameTaken(n) })
			return taken
		},
		func() bool { return true })
	if cancelled {
		return Stay
	}
	old := p.Name
	// The rename and every reference it rewrites land in ONE transaction, so no
	// other node reads a world where half the treaties still name the old realm.
	if err := w.mutatePlayer(func(p *game.Empire) error { return w.RenameEmpire(p, name) }); err != nil {
		fail(s, err)
		return Stay
	}
	okNoPause(s, "%s is now known as %s.", old, name)
	// Other planets learn the new name with this board's next score export, the
	// same round trip every cross-board fact takes; anything already in the air
	// still reaches the realm under its old name.
	if !ibbsHidden(w) {
		okNoPause(s, "Other planets will see the new name once this board's next scores go out.")
	}
	pause(s)
	return Stay
}
