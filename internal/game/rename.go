package game

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// RealmNameMinChars is how many visible characters a realm name must carry.
// They need not be letters: a name drawn entirely from CP437's block and
// line-drawing glyphs is exactly what an ANSI-era player wants (#151), so the
// rule bars only the empty and the blank. Enforced at onboarding and on a
// rename alike.
const RealmNameMinChars = 3

var (
	// ErrRenameUsed is returned when the realm has already been renamed once.
	ErrRenameUsed = errors.New("Your realm has already been renamed once. The name it carries now is permanent.")
	// ErrRenameProtected is returned while the realm is still under new-realm
	// protection: a realm that cannot be attacked cannot slip its name either.
	ErrRenameProtected = errors.New("You may not rename your realm while it is under New Realm Protection.")
	// ErrRealmNameInvalid is returned for a name with too few visible characters.
	ErrRealmNameInvalid = errors.New("A realm name needs at least three visible characters.")
	// ErrRealmNameTaken is returned when another realm holds the name — including
	// a name a renamed realm used to carry, since packets still address it.
	ErrRealmNameTaken = errors.New("Another realm already answers to that name.")
)

// ValidRealmName reports whether name is usable as a realm name: at least
// RealmNameMinChars visible characters once surrounding space is trimmed, and
// no control characters anywhere. A control character is refused outright
// rather than merely uncounted — an escape in a realm name is printed on every
// screen that lists it, and would move the cursor or recolour the row.
func ValidRealmName(name string) bool {
	n := 0
	for _, r := range strings.TrimSpace(name) {
		switch {
		case !unicode.IsPrint(r) && !unicode.IsSpace(r):
			return false
		case !unicode.IsSpace(r):
			n++
		}
	}
	return n >= RealmNameMinChars
}

// RenameEmpire changes e's realm name, once in a realm's life, and rewrites
// every stored reference to the old name so nothing keyed on it is orphaned.
//
// The realm name is an identity key, not just a label: treaties, market
// listings, market proceeds, treaty offers, barter offers, mail senders,
// bribed-agent lists and the covert queue all record a realm by name (see
// MarketListing.Realm for why the owner handle cannot do that job). Everything
// in that list lives in this one world file and is rewritten here, under the
// caller's transaction, so the world is never observed half-renamed.
//
// What is NOT rewritten is what has already left the board. A committed
// interplanetary force finds its way home regardless — InFlightStrike and
// GroupAttack record their contributors by OWNER HANDLE, not by realm name — but
// a packet another board sent while the old name was current arrives addressed
// to it. FormerName is what answers those: remoteTarget and the inbound trade
// path resolve through it, so an attack, message or bid already in the air still
// lands. The old name is also held against re-use (RealmNameTaken) for as long
// as the realm lives, so no second realm can take delivery of it.
func (w *World) RenameEmpire(e *Empire, newName string) error {
	switch {
	case e.FormerName != "":
		return ErrRenameUsed
	case e.Protection > 0:
		return ErrRenameProtected
	}
	old, err := w.setRealmName(e, newName)
	if err != nil {
		return err
	}
	// FormerName is both halves of the deal: the name packets still answer to,
	// and the marker that spends the one rename. A sysop rename records its old
	// name in PriorNames instead, so it does not spend a player's allowance.
	e.FormerName = old
	return nil
}

// SysopRenameEmpire renames e on the sysop's authority (#161). It is the same
// rewrite RenameEmpire performs, without the two conditions that are the
// player's alone: the once-in-a-realm's-life limit and the New Realm Protection
// bar. Nor does it spend the player's rename — a name changed for them is not a
// name they chose, so the old one goes to PriorNames rather than FormerName.
func (w *World) SysopRenameEmpire(e *Empire, newName string) error {
	old, err := w.setRealmName(e, newName)
	if err != nil {
		return err
	}
	e.PriorNames = append(e.PriorNames, old)
	return nil
}

// setRealmName validates newName, moves the realm onto it, repoints every
// stored reference, and tells the planet. It returns the name the realm was
// carrying, for the caller to record as an alias — which of the two alias
// fields it lands in is what separates a player's rename from a sysop's.
func (w *World) setRealmName(e *Empire, newName string) (string, error) {
	name := strings.TrimSpace(newName)
	switch {
	case !ValidRealmName(name):
		return "", ErrRealmNameInvalid
	case w.RealmNameTaken(name):
		return "", ErrRealmNameTaken
	}
	old := e.Name
	e.Name = name
	w.rewriteRealmName(old, name)
	// BRE has no rename, so the wording is IB's own: the planet is told, because
	// every other baron's treaty list, market screen and target list just changed
	// under them.
	w.postNews(fmt.Sprintf("%s is henceforth known as %s.", old, name))
	return old, nil
}

// rewriteRealmName repoints every stored reference to realm `old` at `name`.
// Prose (events, past news) is left as it was written: it is a record of what
// was said at the time, not a reference anything resolves.
func (w *World) rewriteRealmName(old, name string) {
	swap := func(s *string) {
		if *s == old {
			*s = name
		}
	}
	for i := range w.Treaties {
		swap(&w.Treaties[i].A)
		swap(&w.Treaties[i].B)
		// A and B are held in sorted order so a pair has one key; the new name
		// may sort the other way round.
		w.Treaties[i].A, w.Treaties[i].B = treatyPair(w.Treaties[i].A, w.Treaties[i].B)
	}
	for i := range w.CovertQueue {
		swap(&w.CovertQueue[i].Attacker)
		swap(&w.CovertQueue[i].Target)
		swap(&w.CovertQueue[i].Partner)
	}
	for i := range w.Market {
		swap(&w.Market[i].Realm)
	}
	if gold, ok := w.MarketProceeds[old]; ok {
		delete(w.MarketProceeds, old)
		if w.MarketProceeds == nil {
			w.MarketProceeds = map[string]int64{}
		}
		w.MarketProceeds[name] += gold
	}
	swap(&w.CurrentMaster)
	swap(&w.LastMaster)
	for _, e := range w.Empires {
		for i := range e.TreatyOffers {
			swap(&e.TreatyOffers[i].From)
		}
		for i := range e.TradeDeals {
			swap(&e.TradeDeals[i].From)
		}
		for i := range e.Mail {
			// Local mail only: a message from another planet names a realm there,
			// which may well be spelled the same as one here.
			if e.Mail[i].FromBoard == "" {
				swap(&e.Mail[i].From)
			}
		}
		for i := range e.Bribed {
			swap(&e.Bribed[i])
		}
		if day, ok := e.ExposedFrom[old]; ok {
			delete(e.ExposedFrom, old)
			e.ExposedFrom[name] = day
		}
	}
}

// FindByNameOrFormer is FindByName widened to the name a realm used to carry,
// for a reference that was written before a rename and cannot be rewritten —
// an interplanetary packet already in the air. Local lookups keep using
// FindByName: their references are rewritten by rewriteRealmName.
func (w *World) FindByNameOrFormer(name string) *Empire {
	if e := w.FindByName(name); e != nil {
		return e
	}
	if name == "" {
		return nil
	}
	for _, e := range w.Empires {
		for _, old := range e.PriorRealmNames() {
			if old == name {
				return e
			}
		}
	}
	return nil
}
