package game

// PlanetSlots is how many realms one planet holds. BRE keeps its empires in a
// fixed array of this size and addresses each by the letter that indexes it, so
// A..Y is both the whole roster and the whole addressable range — the Z key is
// taken for "All" in every picker. Its own config help states the same figure
// ("the normal maximum is 25", game/reset.hlp), and its save record carries a
// 25-word diplomatic relation row, one per letter (docs/dev/bre-save-format.md).
//
// The bound is over EVERY realm on the planet, humans and computer barons
// alike: they share one array in BRE and one Empires slice here, and it is that
// combined set the pickers letter.
const PlanetSlots = 25

// Letter is the realm's public identity — the character its Id column and every
// picker key shows. A realm with no slot (a legacy save EnsureSlots has not
// reached yet) has no letter and renders "?", which no key can select.
func (e *Empire) Letter() string {
	if e.Slot < 1 || e.Slot > PlanetSlots {
		return "?"
	}
	return string(rune('A' + e.Slot - 1))
}

// EmpireLetter is e's slot letter, or "?" for an empire this world does not
// hold. The world lookup is what makes a stale pointer visible rather than
// silently lettered.
func (w *World) EmpireLetter(e *Empire) string {
	for _, x := range w.Empires {
		if x == e {
			return e.Letter()
		}
	}
	return "?"
}

// takenSlots marks which of the planet's slots are occupied. A husk counts:
// it still sits in Empires, still shows on the score table, and BRE likewise
// frees a record only when it deletes it.
func (w *World) takenSlots() [PlanetSlots + 1]bool {
	var taken [PlanetSlots + 1]bool
	for _, e := range w.Empires {
		if e.Slot >= 1 && e.Slot <= PlanetSlots {
			taken[e.Slot] = true
		}
	}
	return taken
}

// freeSlot is the lowest unoccupied slot, or 0 when the planet is full. A slot
// freed by a removal is available again immediately — dropEmpires is the single
// removal path and it forgets the treaties, pending offers and market position
// the departing realm left behind, all of which key on its NAME rather than on
// its slot, so a new occupant inherits nothing.
func (w *World) freeSlot() int {
	taken := w.takenSlots()
	for n := 1; n <= PlanetSlots; n++ {
		if !taken[n] {
			return n
		}
	}
	return 0
}

// PlanetFull reports whether every slot is held, so no realm of any kind — a
// caller's or a computer baron's — can be founded.
func (w *World) PlanetFull() bool { return w.freeSlot() == 0 }

// EnsureSlots settles Empire.Slot on a world saved before realms had one, and
// is idempotent: a world whose slots are all distinct and in range comes back
// unchanged, so it may run on every reload without renumbering anyone twice.
//
// Slots are handed out in the saved slice order, which is the order the pickers
// used to letter by, so a live board's realms keep the letters their players
// already know. Humans go first when there are not enough slots to go round:
// only an over-full world reaches that, and a computer baron is the cheaper
// thing to lose.
//
// A realm left with no slot is removed. It could never be addressed by anyone —
// which is the defect this replaces (#144) — and leaving it in place would keep
// exactly the asymmetry the slots exist to end.
func (w *World) EnsureSlots() {
	var taken [PlanetSlots + 1]bool
	for _, e := range w.Empires {
		if e.Slot >= 1 && e.Slot <= PlanetSlots && !taken[e.Slot] {
			taken[e.Slot] = true
			continue
		}
		e.Slot = 0 // out of range, or a duplicate of one already claimed
	}
	assign := func(want func(*Empire) bool) {
		for _, e := range w.Empires {
			if e.Slot != 0 || !want(e) {
				continue
			}
			for n := 1; n <= PlanetSlots; n++ {
				if !taken[n] {
					e.Slot, taken[n] = n, true
					break
				}
			}
		}
	}
	assign(func(e *Empire) bool { return e.Owner != "" })
	assign(func(e *Empire) bool { return e.Owner == "" })
	w.dropEmpires(func(e *Empire) bool { return e.Slot == 0 })
}
