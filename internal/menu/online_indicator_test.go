package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
)

// twoRealmWorld returns a ctx whose non-player realm is either online or not,
// so a screen can be rendered in both states from one place.
func twoRealmWorld(t *testing.T, rivalOnline bool) (*ctx, string) {
	t.Helper()
	w := newWorld()
	var rival string
	w.With(func() {
		for _, e := range w.Empires {
			if e == w.Player() {
				continue
			}
			rival = e.Name
			if rivalOnline {
				e.MarkActive()
			}
		}
	})
	if rival == "" {
		t.Fatal("test world has no rival realm to mark")
	}
	return w, rival
}

// rowFor returns the rendered line naming realm, or fails.
func rowFor(t *testing.T, out, realm string) string {
	t.Helper()
	for _, l := range plainLines(out) {
		if strings.Contains(l, realm) {
			return l
		}
	}
	t.Fatalf("no rendered row for %q in:\n%s", realm, out)
	return ""
}

// TestScoresMarksOnlineRealms checks both states on the Scores table, since a
// mark that never clears looks identical to a working indicator when only the
// online case is asserted.
func TestScoresMarksOnlineRealms(t *testing.T) {
	for _, tc := range []struct {
		name   string
		online bool
	}{{"online", true}, {"offline", false}} {
		t.Run(tc.name, func(t *testing.T) {
			w, rival := twoRealmWorld(t, tc.online)
			f := &fakeSession{}
			printScores(f, w)

			lines := plainLines(f.out.String())
			// Marker unique to this screen: without it the assertions below could
			// pass against some other table that happens to name the realm.
			if findLine(lines, "Net Worth") == "" {
				t.Fatal("expected the Scores table heading")
			}
			row := rowFor(t, f.out.String(), rival)
			assertOnlineSuffix(t, row, rival, tc.online)
			// The mark must not have widened the table: BRE-style name field.
			if got := strings.Index(row, rival); got != 6 {
				t.Errorf("name starts at column %d, want 6: %q", got, row)
			}
		})
	}
}

// TestAttackPickerMarksOnlineRealms covers the Scores table's twin. The two
// screens share scoreTableRow, so this guards the wiring — snapshotTargets has
// to carry Online() through, and nothing else would notice if it stopped.
func TestAttackPickerMarksOnlineRealms(t *testing.T) {
	w, rival := twoRealmWorld(t, true)
	f := &fakeSession{}
	pickAttackTarget(f, snapshotTargets(w), "Attack which realm?")

	lines := plainLines(f.out.String())
	if findLine(lines, "Net Worth") == "" {
		t.Fatal("expected the Scores table heading")
	}
	row := rowFor(t, f.out.String(), rival)
	assertOnlineSuffix(t, row, rival, true)
}

// TestRelationsMarksOnlineWithoutMovingColumns is the fidelity half: the marker
// rides inside the name column, so Relations must still start at the captured
// column in BOTH states. TestRelationsScreenMatchesBRE only ever renders
// offline realms, so it cannot see an online row shift.
func TestRelationsMarksOnlineWithoutMovingColumns(t *testing.T) {
	for _, tc := range []struct {
		name   string
		online bool
	}{{"online", true}, {"offline", false}} {
		t.Run(tc.name, func(t *testing.T) {
			w, rival := twoRealmWorld(t, tc.online)
			f := &fakeSession{}
			viewDiplomacy(f, w)

			lines := plainLines(f.out.String())
			if findLine(lines, "-*Relations*-") == "" {
				t.Fatal("expected BRE's -*Relations*- title")
			}
			row := rowFor(t, f.out.String(), rival)
			assertOnlineSuffix(t, row, rival, tc.online)
			// Golden literal: BRE's capture puts the relation at column 45, and
			// measuring against relationsNameWidth would follow a drift silently.
			if got := strings.Index(row, "None"); got != 45 {
				t.Errorf("relation starts at column %d, want BRE's 45: %q", got, row)
			}
		})
	}
}

// TestMenuActionStampsPresence pins the wiring end to end: taking any menu
// action has to leave the caller on the roster. postActionCheck is the only
// place the stamp is refreshed, so if that one call is lost every baron reads
// as offline for their whole session and no screen test above would fail —
// they all set the stamp by hand.
func TestMenuActionStampsPresence(t *testing.T) {
	menus := BuildMenus()
	// 'S' is See Scores on the System menu, then RETURN dismisses its pause and
	// '0' quits.
	f, w, err := run(t, "S\r0", menus.System)
	if err != nil {
		t.Fatalf("session ended with %v", err)
	}
	// Assert the script REACHED the screen. Without this the session ends
	// cleanly when the keys run dry, so a re-mapped hotkey upstream would leave
	// the test green while it never triggered an action at all.
	if !strings.Contains(f.out.String(), "Net Worth") {
		t.Fatalf("script never reached See Scores; output was:\n%s", f.out.String())
	}
	var p *game.Empire
	w.With(func() { p = w.Player() })
	if p.LastActive == 0 {
		t.Error("a menu action left no presence stamp")
	}
	if !p.Online() {
		t.Error("the caller must read as online during their own session")
	}
}

// assertOnlineSuffix checks the marker in both states: "(O)" hugging the realm's
// name on its left when it is on, and a blank of the same width when it is off.
// Asserting the offline case matters twice over — a marker that never clears
// reads as a working indicator, and a blank of the wrong width moves the name.
func assertOnlineSuffix(t *testing.T, row, realm string, online bool) {
	t.Helper()
	i := strings.Index(row, realm)
	if i < 3 {
		t.Fatalf("no room for a marker before %q: %q", realm, row)
	}
	got, want := row[i-3:i], "   "
	if online {
		want = "(O)"
	}
	if got != want {
		t.Errorf("marker before the name = %q, want %q: %q", got, want, row)
	}
}

// TestScoresNeverMarksSelf pins the one row whose stamp is always fresh: your
// own. It tells you nothing you do not know, and See Scores is the only screen
// that lists you at all (the attack picker and Relations both exclude you).
func TestScoresNeverMarksSelf(t *testing.T) {
	w := newWorld()
	var self string
	w.With(func() {
		p := w.Player()
		self = p.Name
		p.MarkActive()
	})
	f := &fakeSession{}
	printScores(f, w)

	if findLine(plainLines(f.out.String()), "Net Worth") == "" {
		t.Fatal("expected the Scores table heading")
	}
	row := rowFor(t, f.out.String(), self)
	assertOnlineSuffix(t, row, self, false)
}

// TestIDCellKeepsTheNameColumn covers the three id widths the score tables
// actually produce. scoreID is three columns for the first 26 realms and four
// past them, and the attack picker leaves it EMPTY for a realm that cannot be
// attacked — each of which shifted the whole row when the id was sized by its
// own text rather than to a fixed cell.
func TestIDCellKeepsTheNameColumn(t *testing.T) {
	for _, tc := range []struct {
		id       string
		nameCol  int
		whatItIs string
	}{
		{"(A)", 6, "lettered id"},
		{"(Z)", 6, "last lettered realm"},
		{"", 6, "unattackable realm, no id"},
	} {
		t.Run(tc.whatItIs, func(t *testing.T) {
			f := &fakeSession{}
			scoreTableRow(f, tc.id, "Realm", ansi.FgBrightWhite, presenceOnline, 1, 2, 3)
			row := plainLines(f.out.String())[0]
			assertOnlineSuffix(t, row, "Realm", true)
			if got := strings.Index(row, "Realm"); got != tc.nameCol {
				t.Errorf("name starts at column %d, want %d: %q", got, tc.nameCol, row)
			}
		})
	}
}

// TestRecipientPickerMarksOnlineRealms and TestPlayerListMarksOnlineRealms
// cover the two rosters the first pass of this feature missed. Both list local
// realms, so both have to answer "who is on" the same way the score tables do.
func TestRecipientPickerMarksOnlineRealms(t *testing.T) {
	w, rival := twoRealmWorld(t, true)
	// '?' prints the roster, then a bare RETURN cancels the picker.
	f := &fakeSession{keys: []rune("?\r")}
	pickRecipient(f, w, pickOpts{})

	lines := plainLines(f.out.String())
	head := findLine(lines, "Empire Name")
	if head == "" {
		t.Fatalf("script never reached the roster; output was:\n%s", f.out.String())
	}
	row := rowFor(t, f.out.String(), rival)
	assertOnlineSuffix(t, row, rival, true)
	// The rows used to sit one column left of their own heading.
	if strings.Index(row, rival) != strings.Index(head, "Empire Name") {
		t.Errorf("name column does not line up with the heading:\n%q\n%q", head, row)
	}
}

func TestPlayerListMarksOnlineRealms(t *testing.T) {
	w, rival := twoRealmWorld(t, true)
	f := &fakeSession{keys: []rune("\r")}
	playerList(f, w)

	lines := plainLines(f.out.String())
	if findLine(lines, "Player List") == "" {
		t.Fatal("expected the Player List heading")
	}
	row := rowFor(t, f.out.String(), rival)
	assertOnlineSuffix(t, row, rival, true)
}

// A realm name is only floored at three characters when it is created — nothing
// caps it — so the tables have to clip. The figures beside the name must sit at
// the same column whatever the name's length, and the marker must survive the
// clip, since it is the row's only statement about presence.
func TestLongNameCannotMoveTheColumns(t *testing.T) {
	var want string
	for _, name := range []string{
		"Ab",
		"Gale Horde",
		"Eighteen Chars Xyz",
		"A Realm Name Of Truly Excessive Length Indeed",
	} {
		for _, presence := range []string{presenceOnline, presenceNone} {
			f := &fakeSession{}
			scoreTableRow(f, "(A)", name, ansi.FgBrightWhite, presence, 15, 0, 231)
			row := plainLines(f.out.String())[0]
			if presence == presenceOnline && !strings.Contains(row, "(O)") {
				t.Errorf("clip dropped the marker for %q: %q", name, row)
			}
			tail := row[len(row)-34:]
			if want == "" {
				want = tail
			} else if tail != want {
				t.Errorf("figures moved for %q (presence=%q):\n got %q\nwant %q", name, presence, tail, want)
			}
		}
	}
}
