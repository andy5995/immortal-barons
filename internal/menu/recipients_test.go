package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// #144: the picker used to stop at the first 25 entries of the Empires slice
// while nothing stopped the slice from growing, so a realm past that point could
// act on everyone and be acted on by nobody. With the roster bounded at the
// planet's slots, the picker cannot fall short of the game: on a FULL planet,
// every living realm appears in every other living realm's picker, under the
// letter that realm answers to everywhere else.
func TestEveryLivingRealmIsAddressableByEveryOther(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.AICount = 0
	w := game.NewWorldSeed(cfg, 1)
	for i := 0; i < 25; i++ { // fill the planet
		if e := w.AddHuman(string(rune('a'+i)), "Realm"+string(rune('A'+i))); e == nil {
			t.Fatalf("realm %d of 25 was refused a slot", i+1)
		}
	}

	for i := 0; i < 25; i++ {
		c := &ctx{World: w, handle: string(rune('a' + i)), Term: Term{UTF8: true}}
		c.Today = "2026-07-03"
		rows, _ := pickRows(c, pickOpts{})
		if len(rows) != 24 {
			t.Fatalf("%s sees %d of the other 24 realms", c.Player().Name, len(rows))
		}
		for _, r := range rows {
			if r.e == c.Player() {
				t.Errorf("%s can address itself", c.Player().Name)
			}
			if got := r.e.Letter(); got != string(r.letter) {
				t.Errorf("%s sees %s as (%c), but its letter is %q",
					c.Player().Name, r.e.Name, r.letter, got)
			}
		}
	}
}

func TestRecipientsExcludesPlayerAndDead(t *testing.T) {
	w := newWorld()
	// newWorld seeds one AI empire plus the human player.
	if len(w.Empires) < 2 {
		t.Fatalf("expected at least 2 empires, got %d", len(w.Empires))
	}
	rs := recipients(w)
	for _, e := range rs {
		if e == w.Player() {
			t.Errorf("recipients() included the player %q", e.Name)
		}
		if !e.Alive {
			t.Errorf("recipients() included a dead empire %q", e.Name)
		}
	}

	// Kill the first non-player recipient and confirm it disappears.
	if len(rs) > 0 {
		rs[0].Alive = false
		rs2 := recipients(w)
		for _, e := range rs2 {
			if e == rs[0] {
				t.Errorf("recipients() still included empire killed after first call")
			}
		}
		if len(rs2) != len(rs)-1 {
			t.Errorf("recipients() after death = %d entries, want %d", len(rs2), len(rs)-1)
		}
	}
}

// A picker letter is the empire's SLOT letter, so a dead realm or the caller's
// own leaves a GAP instead of renumbering the rows below it — BRE indexes its
// empire array with the letter itself (BRE.OVR 0x1b575). The letters must also
// agree with the ones a message records in "Message To  :".
func TestPickIndexUsesSlotLetters(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(3) })
	var gap *game.Empire
	w.With(func() {
		for _, e := range w.Empires { // the first realm that is neither dead nor the caller
			if e != w.Player() {
				gap = e
				break
			}
		}
		gap.Alive = false
	})
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Fatal("no selectable realms")
	}
	for _, r := range rows {
		if r.e == gap {
			t.Fatalf("a dead realm is still selectable as (%c)", r.letter)
		}
		if r.e == w.Player() {
			t.Fatalf("the caller is selectable as (%c)", r.letter)
		}
		if got := pickIndex(rows, r.letter); got < 0 || rows[got].e != r.e {
			t.Errorf("pickIndex(%q) did not find its own row", r.letter)
		}
		w.With(func() {
			if want := w.EmpireLetter(r.e); want != string(r.letter) {
				t.Errorf("row letter = %q, want the empire's slot letter %q", r.letter, want)
			}
		})
	}
	if pickIndex(rows, '0') != -1 {
		t.Error("a non-letter key selected a row")
	}
}

func TestPickRecipientSelectsByLetter(t *testing.T) {
	w := newWorld()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{rows[0].letter}}
	if got := pickRecipient(f, w, pickOpts{prompt: "Trade with:"}); got != rows[0].e {
		t.Errorf("pickRecipient(%q) = %v, want %v", rows[0].letter, got, rows[0].e)
	}
}

// The picker's Score column must show the empire's cumulative Score, not a
// second copy of Land (which made every row read Land==Score).
func TestPickRecipientShowsScoreNotLand(t *testing.T) {
	w := newWorld()
	rs := recipients(w)
	if len(rs) == 0 {
		t.Skip("no recipients seeded")
	}
	w.With(func() {
		rs[0].Land = 4242
		rs[0].Score = 7777
	})
	f := &fakeSession{keys: []rune("?0")} // '?' prints the roster, then cancel
	pickRecipient(f, w, pickOpts{prompt: "Trade with:"})
	out := f.out.String()
	// The spelling is BRE's: four digits print bare, and Score/Net Worth
	// shorten past 10,000 (numfmt.Short/GroupLong). Pinned here so this table
	// cannot drift away from the two other screens that share it.
	for _, want := range []string{"4242", "7777"} {
		if !strings.Contains(out, want) {
			t.Errorf("roster should show %q (Land 4242 / Score 7777):\n%s", want, out)
		}
	}
	if strings.Contains(out, "4,242") {
		t.Errorf("figures should use BRE's spelling, not full grouping:\n%s", out)
	}
}

// Z marks every realm; RETURN then closes the list with all of them on it.
func TestPickRecipientsAll(t *testing.T) {
	w := newWorld()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune("Z\r")}
	got := pickRecipients(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if len(got) != len(rows) {
		t.Fatalf("pickRecipients(\"Z\\r\") = %d realms, want %d", len(got), len(rows))
	}
	for i, r := range rows {
		if got[i] != r.e {
			t.Errorf("realm %d = %v, want %v", i, got[i], r.e)
		}
	}
}

// The single-target picker takes one keypress, and a key that names no realm
// cancels it.
func TestPickRecipientCancel(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune{'0'}}
	if got := pickRecipient(f, w, pickOpts{prompt: "Trade with:"}); got != nil {
		t.Errorf("pickRecipient('0') = %v, want nil", got)
	}
}

// treatyWith makes the player and e hold a standing treaty, the state that
// unlocks the all-allies target.
func treatyWith(w *ctx, e *game.Empire) {
	p := w.Player()
	w.World.ProposeTreaty(e, p, "Free Trade Agreement")
	w.World.AcceptTreaty(p, e.Name, "Free Trade Agreement")
}

// '*' marks the treaty partners and nobody else — IB's own key, composing with
// the multi-select rather than short-circuiting it: the list still closes on
// RETURN, so a letter can be added to or taken off the allies afterwards.
func TestPickRecipientsAllAllies(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	ally, extra := rows[0], rows[2]
	treatyWith(w, ally.e)
	f := &fakeSession{keys: []rune("*" + string(extra.letter) + "\r")}
	got := pickRecipients(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if len(got) != 2 || got[0] != ally.e || got[1] != extra.e {
		t.Fatalf("pickRecipients(\"*%c\\r\") = %v, want the ally %s plus %s",
			extra.letter, got, ally.name, extra.name)
	}
	// Strip the SGR runs: BRE colours each piece of this prompt separately, so
	// the escape sequences sit between the '*' and its label.
	if !strings.Contains(sgr.ReplaceAllString(f.out.String(), ""), "*=All Allies") {
		t.Errorf("prompt should offer *=All Allies:\n%s", f.out.String())
	}
}

// With no treaty there is nobody to reach that way, so the key is not offered
// and marks nothing.
func TestPickRecipientsAllAlliesHiddenWithoutTreaty(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune("*\r")}
	if got := pickRecipients(f, w, pickOpts{prompt: "Send to:", allowAll: true}); len(got) != 0 {
		t.Errorf("pickRecipients('*') without a treaty = %v, want nothing marked", got)
	}
	if strings.Contains(f.out.String(), "All Allies") {
		t.Errorf("prompt should not offer All Allies without a treaty:\n%s", f.out.String())
	}
}

// The all-allies send reaches every treaty partner and nobody else.
func TestSendMessageToAllAllies(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	rs := recipients(w)
	if len(rs) < 3 {
		t.Fatalf("need 3 recipients, got %d", len(rs))
	}
	ally, other := rs[0], rs[1]
	treatyWith(w, ally)
	body := "Muster at the river."
	// '*' marks the allies, RETURN closes the list, then the message.
	f := &fakeSession{keys: []rune("*\r" + body + "\r/s")}
	sendMessage(f, w)
	if len(ally.Mail) != 1 {
		t.Errorf("ally got %d messages, want 1", len(ally.Mail))
	} else if !strings.Contains(ally.Mail[0].Body, body) {
		t.Errorf("ally message = %q, want it to carry %q", ally.Mail[0].Body, body)
	}
	if len(other.Mail) != 0 {
		t.Errorf("a realm with no treaty got %d messages, want 0", len(other.Mail))
	}
}

// BRE's Send Message picker is a MULTI-select list: letters toggle, RETURN
// closes it. Two letters must therefore reach two inboxes from one composition,
// and every copy must carry the same "Message To  :" letters, in letter order.
func TestSendMessageToSeveralRealms(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	first, second, untouched := rows[0], rows[2], rows[1]
	body := "Meet at dawn."
	keys := string(second.letter) + string(first.letter) + "\r" + body + "\r/s"
	f := &fakeSession{keys: []rune(keys)}
	sendMessage(f, w)

	if !strings.Contains(f.out.String(), "lines for your message") {
		t.Fatalf("never reached the editor:\n%s", f.out.String())
	}
	want := string(first.letter) + string(second.letter)
	for _, r := range []pickRow{first, second} {
		if len(r.e.Mail) != 1 {
			t.Fatalf("%s got %d messages, want 1", r.name, len(r.e.Mail))
		}
		if !strings.Contains(r.e.Mail[0].Body, body) {
			t.Errorf("%s message = %q, want it to carry %q", r.name, r.e.Mail[0].Body, body)
		}
		if r.e.Mail[0].To != want {
			t.Errorf("%s message addressed to %q, want %q", r.name, r.e.Mail[0].To, want)
		}
	}
	if len(untouched.e.Mail) != 0 {
		t.Errorf("%s got %d messages, want 0", untouched.name, len(untouched.e.Mail))
	}
}

// Z addresses everyone and does NOT close the prompt — BRE toggles each letter
// A..Y in turn and still waits for RETURN (BRE.OVR 0x1b65e, its 0x1427 branch).
func TestSendMessageZAllNeedsReturn(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	body := "Everyone hears this."
	f := &fakeSession{keys: []rune("Z\r" + body + "\r/s")}
	sendMessage(f, w)

	if !strings.Contains(f.out.String(), "lines for your message") {
		t.Fatalf("Z then RETURN never reached the editor:\n%s", f.out.String())
	}
	var want strings.Builder
	for _, r := range rows {
		want.WriteRune(r.letter)
	}
	for _, r := range rows {
		if len(r.e.Mail) != 1 {
			t.Fatalf("%s got %d messages, want 1", r.name, len(r.e.Mail))
		}
		if r.e.Mail[0].To != want.String() {
			t.Errorf("%s message addressed to %q, want %q", r.name, r.e.Mail[0].To, want.String())
		}
	}
}

// Pressing a letter twice takes it back off the list, so Z followed by one
// letter addresses everybody except that realm.
func TestSendMessageZThenDeselect(t *testing.T) {
	w := newWorld()
	w.With(func() { w.AddAIEmpires(2) })
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) < 3 {
		t.Fatalf("need 3 selectable realms, got %d", len(rows))
	}
	dropped := rows[1]
	f := &fakeSession{keys: []rune("Z" + string(dropped.letter) + "\rnote\r/s")}
	sendMessage(f, w)

	if !strings.Contains(f.out.String(), "lines for your message") {
		t.Fatalf("never reached the editor:\n%s", f.out.String())
	}
	if len(dropped.e.Mail) != 0 {
		t.Errorf("%s was deselected but got %d messages", dropped.name, len(dropped.e.Mail))
	}
	for _, r := range rows {
		if r.e == dropped.e {
			continue
		}
		if len(r.e.Mail) != 1 {
			t.Errorf("%s got %d messages, want 1", r.name, len(r.e.Mail))
		}
	}
}

// RETURN with an empty list is BRE's cancel: no editor, no mail.
func TestSendMessageEmptySelectionCancels(t *testing.T) {
	w := newWorld()
	rows, _ := pickRows(w, pickOpts{})
	if len(rows) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune("\r")}
	sendMessage(f, w)
	if strings.Contains(f.out.String(), "lines for your message") {
		t.Errorf("an empty selection opened the editor:\n%s", f.out.String())
	}
	for _, r := range rows {
		if len(r.e.Mail) != 0 {
			t.Errorf("%s got mail from a cancelled send", r.name)
		}
	}
}

// The prompt names the whole addressable range whatever the realm count: BRE
// prints "(A-Y,Z=All,?=List)" in a two-realm game too (docs/dev/bre-screens.md).
func TestPickPromptAlwaysSpansAToY(t *testing.T) {
	w := newWorld()
	if len(recipients(w)) == 0 {
		t.Skip("no recipients seeded")
	}
	f := &fakeSession{keys: []rune("\r")}
	pickRecipients(f, w, pickOpts{prompt: "Send to:", allowAll: true})
	if got := sgr.ReplaceAllString(f.out.String(), ""); !strings.Contains(got, "(A-Y,Z=All,?=List) Send to:") {
		t.Errorf("prompt = %q, want BRE's (A-Y,Z=All,?=List) Send to:", got)
	}
}
