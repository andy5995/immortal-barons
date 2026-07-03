package game

// allyKey returns a canonical, order-independent key for an empire pair.
func allyKey(a, b string) string {
	if a < b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

// AreAllied reports whether a and b have a standing alliance.
func (w *World) AreAllied(a, b *Empire) bool {
	k := allyKey(a.Name, b.Name)
	for _, x := range w.Alliances {
		if x == k {
			return true
		}
	}
	return false
}

// ProposeAlliance records a pending alliance offer from `from` to `to` and
// mails the target. No-op if already allied or an identical offer is pending.
func (w *World) ProposeAlliance(from, to *Empire) {
	if w.AreAllied(from, to) {
		return
	}
	for _, o := range to.AllianceOffers {
		if o == from.Name {
			return
		}
	}
	to.AllianceOffers = append(to.AllianceOffers, from.Name)
	to.Mail = append(to.Mail, from.Name+" proposes an alliance (accept it in the Diplomacy menu).")
}

// AcceptAlliance forms a mutual alliance if `me` has a pending offer from
// fromName; it consumes the offer. Returns false if there was no such offer.
func (w *World) AcceptAlliance(me *Empire, fromName string) bool {
	found := false
	kept := me.AllianceOffers[:0]
	for _, o := range me.AllianceOffers {
		if o == fromName {
			found = true
		} else {
			kept = append(kept, o)
		}
	}
	me.AllianceOffers = kept
	if !found {
		return false
	}
	k := allyKey(me.Name, fromName)
	for _, x := range w.Alliances {
		if x == k {
			return true
		}
	}
	w.Alliances = append(w.Alliances, k)

	// If the proposer had also sent me.Name a reverse offer, clear it too
	// so it doesn't linger as a phantom pending offer.
	for _, e := range w.Empires {
		if e.Name != fromName {
			continue
		}
		kept := e.AllianceOffers[:0]
		for _, o := range e.AllianceOffers {
			if o != me.Name {
				kept = append(kept, o)
			}
		}
		e.AllianceOffers = kept
		break
	}

	return true
}

// BreakAlliance ends any alliance between a and b.
func (w *World) BreakAlliance(a, b *Empire) {
	k := allyKey(a.Name, b.Name)
	out := w.Alliances[:0]
	for _, x := range w.Alliances {
		if x != k {
			out = append(out, x)
		}
	}
	w.Alliances = out
}
