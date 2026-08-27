package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ipWorld is a test ctx on a named board with two other planets in the league
// roster.
func ipWorld() *ctx {
	w := newWorld()
	w.Config.BoardID = "Nova Hub"
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Nova Hub", City: "Brisbane", State: "QL", Country: "AUS"},
		{Number: 3, Name: "Eye of the Storm", City: "New Plymouth", State: "TN", Country: "NZ"},
		{Number: 4, Name: "The Eclipse", City: "Sydney", State: "NS", Country: "AUS"},
	}
	return w
}

// TestIPMessageSinglePlanetReachesEditorAndQueues drives the whole screen: the
// planet list, a pick by number, the relations line, the editor, and a saved
// message queued for that planet alone. Reaching the editor is asserted by its
// banner AND by the queued packet, so a flow change upstream cannot leave this
// passing on a script that ran dry.
func TestIPMessageSinglePlanetReachesEditorAndQueues(t *testing.T) {
	f := &fakeSession{keys: []rune("?4\rTerms.\r/s")}
	w := ipWorld()
	ipMessageSingle(f, w)
	out := f.out.String()
	for _, want := range []string{
		"List of Planets", "Planet Name", "Location", "Sydney, NS, AUS",
		"Enter Planet Name or Number", "Our current relations with", "The Eclipse",
		"lines for your message",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Single Planet screen missing %q:\n%s", want, out)
		}
	}
	// This board is in the list, as it is in BRE's own — see knownPlanets.
	if !strings.Contains(out, "Nova Hub") {
		t.Error("the planet list left out this board")
	}
	if len(w.Outbox) != 1 || w.Outbox[0].ToBoard != "The Eclipse" {
		t.Fatalf("queued %+v, want one packet for The Eclipse", w.Outbox)
	}
	if got := w.Outbox[0].IPMessages[0].Body; got != "Terms." {
		t.Errorf("queued body %q", got)
	}
}

// TestIPMessageAllPlanetsQueuesEachPlanet checks All Planets skips the picker
// and writes to every other planet, this board excluded.
func TestIPMessageAllPlanetsQueuesEachPlanet(t *testing.T) {
	f := &fakeSession{keys: []rune("Hello league.\r/s")}
	w := ipWorld()
	ipMessageAll(f, w)
	if !strings.Contains(f.out.String(), "lines for your message") {
		t.Fatalf("All Planets did not reach the editor:\n%s", f.out.String())
	}
	var to []string
	for _, p := range w.Outbox {
		to = append(to, p.ToBoard)
	}
	if len(to) != 2 {
		t.Fatalf("queued for %v, want the two other planets", to)
	}
}

// TestIPMessageSelectPlanetsStopsOnEmptyLine checks BRE's multi-pick loop: it
// keeps prompting until an empty answer, and a planet named twice is queued
// once.
func TestIPMessageSelectPlanetsStopsOnEmptyLine(t *testing.T) {
	f := &fakeSession{keys: []rune("3\r3\r\rTerms.\r/s")}
	w := ipWorld()
	ipMessageSelect(f, w)
	out := f.out.String()
	if !strings.Contains(out, "To end selection, hit enter at the prompt.") {
		t.Errorf("Select Planets is missing its instruction line:\n%s", out)
	}
	if len(w.Outbox) != 1 || w.Outbox[0].ToBoard != "Eye of the Storm" {
		t.Fatalf("queued %+v, want one packet for Eye of the Storm", w.Outbox)
	}
}

// TestIPMessageAbortSendsNothing checks /A leaves the outbox empty. It asserts
// the editor was REACHED and the abort was taken first: an empty outbox is also
// what a script that never got that far leaves behind, so the state check alone
// would keep passing after any flow change upstream.
func TestIPMessageAbortSendsNothing(t *testing.T) {
	f := &fakeSession{keys: []rune("4\rTerms.\r/a")}
	w := ipWorld()
	ipMessageSingle(f, w)
	out := f.out.String()
	for _, want := range []string{"lines for your message", "Abort"} {
		if !strings.Contains(out, want) {
			t.Fatalf("never reached the editor's abort, so the empty outbox proves nothing (missing %q):\n%s", want, out)
		}
	}
	if len(w.Outbox) != 0 {
		t.Errorf("an aborted message was queued: %+v", w.Outbox)
	}
}

// TestRemoteTargetUsesBREPlanetPrompt covers the picker every targeting screen
// shares after #1 of the slop audit: BRE asks for a planet by name or number
// with "?" for the list, and the number is the planet's ROSTER number, not its
// position in whatever subset a screen can reach. Terrorist Ops stands in for
// the four screens that go through pickRemoteTarget.
func TestRemoteTargetUsesBREPlanetPrompt(t *testing.T) {
	w := ipWorld()
	w.RemoteBoards = []game.RemoteBoard{
		{BoardID: "The Eclipse", Scores: []game.RemoteScore{{Empire: "Iron Dominion", Land: 900}}},
	}
	p := w.Player()
	p.Agents, p.Protection = 50, 0
	// "?" lists the planets, "4" names The Eclipse by its ROSTER number (it is
	// third in the roster and the only reachable board, so a positional picker
	// would have wanted "1"), then the baron by its LETTER — every roster of
	// players is lettered, as the original letters them — 10 agents, and Y to
	// the price.
	f := &fakeSession{keys: []rune("?4\rA10\ry ")}
	doTerrorOp(f, w, game.TerrorOpSpy)
	out := f.out.String()
	for _, want := range []string{"Enter Planet Name or Number", "List of Planets", "Terrorize which baron?"} {
		if !strings.Contains(out, want) {
			t.Errorf("Terrorist Ops missing %q:\n%s", want, out)
		}
	}
	if len(w.Outbox) != 1 || len(w.Outbox[0].Terrors) != 1 {
		t.Fatalf("no terror op was queued: %+v", w.Outbox)
	}
	if got := w.Outbox[0].ToBoard; got != "The Eclipse" {
		t.Errorf("queued against %q, want The Eclipse", got)
	}
}

// "?" at the planet prompt shows the list on the KEYPRESS. It is a shortcut,
// not an answer, so requiring Enter to confirm a request to look at something
// costs a keystroke and buys nothing. A typed planet number still reads as a
// line, first character included.
func TestPlanetPromptListsOnTheQuestionMarkAlone(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Nova Hub"}, {Number: 2, Name: "The Eclipse"},
		}
	})
	// No Enter after "?": the list appears, then "2" and Enter pick a planet.
	f := &fakeSession{keys: []rune("?2\r")}
	got := pickPlanetNamed(f, w, []string{"Nova Hub", "The Eclipse"})

	out := stripANSI(f.out.String())
	if !strings.Contains(out, "List of Planets") {
		t.Errorf("? alone did not show the list:\n%s", out)
	}
	if got != "The Eclipse" {
		t.Errorf("picked %q after the list, want The Eclipse — the typed number must still read as a line", got)
	}
}

// A multi-character answer typed straight in still works: the peeked first key
// is handed back to the line reader rather than swallowed.
func TestPlanetPromptKeepsTheFirstTypedCharacter(t *testing.T) {
	w := newWorld()
	w.With(func() {
		w.World.Config.IBBS = true
		w.World.LeagueNodes = []game.LeagueNode{{Number: 1, Name: "Nova Hub"}}
	})
	f := &fakeSession{keys: []rune("Nova Hub\r")}
	if got := pickPlanetNamed(f, w, []string{"Nova Hub"}); got != "Nova Hub" {
		t.Errorf("picked %q, want Nova Hub — the first letter must not be eaten", got)
	}
}

// unroutableWorld is ipWorld having also heard, over a packet, from a board
// that never introduced itself to the Coordinator — the live fault, where a
// board ran with its BoardID unset and every league member recorded "local"
// forever. Its node number is invented by LeaguePlanets, so routing cannot
// place it.
func unroutableWorld() *ctx {
	w := ipWorld()
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "local"}}
	return w
}

// The picker must not offer a board a packet cannot reach: choosing one costs
// the player the action and destroys the packet several boards away. The script
// names it and is refused, then names a real planet, so the prompt is proved to
// have been reached and still working.
func TestThePickerDoesNotOfferAnUnroutablePlanet(t *testing.T) {
	f := &fakeSession{keys: []rune("?local\r4\rTerms.\r/s")}
	w := unroutableWorld()
	ipMessageSingle(f, w)
	out := f.out.String()
	if !strings.Contains(out, "List of Planets") {
		t.Fatalf("never reached the planet list:\n%s", out)
	}
	table := out[strings.Index(out, "List of Planets"):]
	if i := strings.Index(table, "Enter Planet Name"); i >= 0 {
		table = table[:i]
	}
	if strings.Contains(table, "local") {
		t.Errorf("the picker listed a planet the roster cannot place:\n%s", table)
	}
	// Three prompts: the one "?" answered, the one that took "local", and the one
	// it had to ask again because that name was refused.
	if n := strings.Count(out, "Enter Planet Name"); n != 3 {
		t.Errorf("the prompt ran %d times, want 3 — naming the unroutable planet was not refused:\n%s", n, out)
	}
	if !strings.Contains(out, "lines for your message") {
		t.Fatalf("the prompt never recovered to take a real planet:\n%s", out)
	}
	if len(w.Outbox) != 1 || w.Outbox[0].ToBoard != "The Eclipse" {
		t.Fatalf("queued %+v, want one packet for The Eclipse", w.Outbox)
	}
}

// All Planets never goes through the picker, so the send itself has to filter.
func TestAllPlanetsSkipsAnUnroutablePlanet(t *testing.T) {
	f := &fakeSession{keys: []rune("Hello league.\r/s")}
	w := unroutableWorld()
	ipMessageAll(f, w)
	if !strings.Contains(f.out.String(), "lines for your message") {
		t.Fatalf("All Planets did not reach the editor:\n%s", f.out.String())
	}
	var to []string
	for _, p := range w.Outbox {
		to = append(to, p.ToBoard)
	}
	if len(to) != 2 {
		t.Fatalf("queued for %v, want only the two planets on the roster", to)
	}
}

// Unroutable is not invisible: a board heard from before the Coordinator's
// roster catches up is a normal, temporary state, and it becomes routable on
// its own once the roster lands. It stays on the screens that only SHOW the
// league.
func TestAnUnroutablePlanetIsStillShownWhereTheScreenOnlyInforms(t *testing.T) {
	w := unroutableWorld()
	out := drive(t, w, "\r", planetaryTreaties).out.String()
	if !strings.Contains(out, "Planetary Treaties") {
		t.Fatalf("never reached the chart:\n%s", out)
	}
	if !strings.Contains(out, "local") {
		t.Errorf("the treaty chart hid a planet it has merely heard from:\n%s", out)
	}
}

// ipWorldWithRoster is ipWorld having also heard The Eclipse's scores, so Send
// Message -> Single Planet has a roster to letter.
func ipWorldWithRoster() *ctx {
	w := ipWorld()
	w.RemoteBoards = []game.RemoteBoard{{BoardID: "The Eclipse", Scores: []game.RemoteScore{
		{Empire: "Imperially", Land: 7596, Score: 734000, NetWorth: 9017000},
		{Empire: "The Empire of Queg", Land: 7187, Score: 728000, NetWorth: 11000000},
		{Empire: "Gap Origix", Land: 10212, Score: 247000, NetWorth: 10000000},
	}}}
	return w
}

// ipQueuedTo is the ToEmpire of every message queued for board.
func ipQueuedTo(w *ctx, board string) []string {
	var to []string
	for _, p := range w.Outbox {
		if p.ToBoard != board {
			continue
		}
		for _, m := range p.IPMessages {
			to = append(to, m.ToEmpire)
		}
	}
	return to
}

// '?' at the recipient prompt lists the planet's roster and asks again, as the
// original does (cap/eots-ibbs-01.cap at the Send Message prompt). The script
// then picks a realm and sends, so the roster is proved to have been drawn on
// the way to a real send rather than at the end of a script that ran dry.
func TestIPMessageSinglePlanetListsThePlanetRoster(t *testing.T) {
	f := &fakeSession{keys: []rune("4\r?B\rTerms.\r/s")}
	w := ipWorldWithRoster()
	ipMessageSingle(f, w)
	out := stripANSI(f.out.String())
	for _, want := range []string{
		"Send to:", "-*Players at The Eclipse*-",
		"Empire Name", "Territory", "Net Worth",
		"Imperially", "The Empire of Queg", "Gap Origix",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the recipient roster is missing %q:\n%s", want, out)
		}
	}
	// Two prompts: the one '?' answered and the one that took the letter.
	if n := strings.Count(out, "Send to:"); n != 2 {
		t.Errorf("the prompt ran %d times, want 2 — '?' must re-draw it", n)
	}
	if got := ipQueuedTo(w, "The Eclipse"); len(got) != 1 || got[0] != "The Empire of Queg" {
		t.Fatalf("queued for %v, want the realm lettered B", got)
	}
}

// A letter toggles: pressing it once marks that realm, pressing it again takes
// it off. The script marks A, marks B, then presses A again, so only B may be
// written to — and the erase BRE writes (BS/space/BS) has to appear, since a
// picker that silently ignored the second A would queue the same one message.
func TestIPMessageRecipientLetterToggles(t *testing.T) {
	f := &fakeSession{keys: []rune("4\rABA\rTerms.\r/s")}
	w := ipWorldWithRoster()
	ipMessageSingle(f, w)
	raw := f.out.String()
	out := stripANSI(raw)
	if !strings.Contains(out, "Send to:") {
		t.Fatalf("never reached the recipient prompt:\n%s", out)
	}
	if !strings.Contains(raw, "\b \b") {
		t.Errorf("nothing was rubbed off the prompt line, so the second A did not un-mark:\n%q", raw)
	}
	if !strings.Contains(out, "lines for your message") {
		t.Fatalf("never reached the editor:\n%s", out)
	}
	if got := ipQueuedTo(w, "The Eclipse"); len(got) != 1 || got[0] != "The Empire of Queg" {
		t.Fatalf("queued for %v, want B alone — A was marked and un-marked", got)
	}
}

// 'Z' marks every realm on the planet, and RETURN then sends one message per
// marked realm, each addressed to that realm alone.
func TestIPMessageZMarksEveryBaronOnThePlanet(t *testing.T) {
	f := &fakeSession{keys: []rune("4\rZ\rTerms.\r/s")}
	w := ipWorldWithRoster()
	ipMessageSingle(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Send to:") || !strings.Contains(out, "lines for your message") {
		t.Fatalf("never reached the prompt and the editor:\n%s", out)
	}
	got := ipQueuedTo(w, "The Eclipse")
	want := []string{"Imperially", "The Empire of Queg", "Gap Origix"}
	if len(got) != len(want) {
		t.Fatalf("queued for %v, want one message each for %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("queued for %v, want %v", got, want)
		}
	}
	if body := w.Outbox[0].IPMessages[0].Body; body != "Terms." {
		t.Errorf("queued body %q", body)
	}
}

// RETURN with nothing marked leaves without sending — and without opening the
// editor, which is what the original's own capture shows (it goes straight back
// to asking whether to send another message). The editor banner is asserted
// ABSENT after the prompt is proved reached, so an empty outbox cannot come
// from a script that stopped earlier.
func TestIPMessageEmptyRecipientListSendsNothing(t *testing.T) {
	f := &fakeSession{keys: []rune("4\r\r")}
	w := ipWorldWithRoster()
	ipMessageSingle(f, w)
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Send to:") {
		t.Fatalf("never reached the recipient prompt:\n%s", out)
	}
	if strings.Contains(out, "lines for your message") {
		t.Errorf("an empty recipient list still opened the editor:\n%s", out)
	}
	if len(w.Outbox) != 0 {
		t.Errorf("an unaddressed message was queued: %+v", w.Outbox)
	}
}

// A planet this board has no scores for has no roster to letter, so the message
// goes to the planet as a whole rather than the screen refusing to send. The
// prompt must NOT appear — a picker over an empty roster would have no key that
// does anything.
func TestIPMessageWithoutARosterWritesToTheWholePlanet(t *testing.T) {
	f := &fakeSession{keys: []rune("3\rTerms.\r/s")}
	w := ipWorldWithRoster() // scores for The Eclipse only; "3" is Eye of the Storm
	ipMessageSingle(f, w)
	out := stripANSI(f.out.String())
	if strings.Contains(out, "Send to:") {
		t.Errorf("a planet with no known barons still asked which of them:\n%s", out)
	}
	if !strings.Contains(out, "lines for your message") {
		t.Fatalf("never reached the editor:\n%s", out)
	}
	if got := ipQueuedTo(w, "Eye of the Storm"); len(got) != 1 || got[0] != "" {
		t.Fatalf("queued for %q, want one planet-wide message (no ToEmpire)", got)
	}
}

// matchPlanet, against select_planet (BRE.OVR 0x021dd9) — #183. The behaviour
// each case pins is read out of the binary, not guessed: see the doc comment on
// matchPlanet for the offsets.
func TestMatchPlanetFollowsTheOriginalsParser(t *testing.T) {
	planets := []game.LeagueNode{
		{Number: 1, Name: "The X-Bit BBS"},
		{Number: 2, Name: "The uniX-Bit BBS"},
		{Number: 5, Name: "Nite Eyes BBS"},
		{Number: 12, Name: "A-Net Online BBS"},
	}
	name := func(p *game.LeagueNode) string {
		if p == nil {
			return ""
		}
		return p.Name
	}
	for _, c := range []struct {
		typed, want, why string
	}{
		{"5", "Nite Eyes BBS", "an exact roster number resolves on its own"},
		{"12", "A-Net Online BBS", "a two-digit number is exact, not a substring"},
		{"99", "", "a number with no slot behind it is refused"},
		{"Nite Eyes BBS", "Nite Eyes BBS", "a full name still resolves"},
		{"nite eyes bbs", "Nite Eyes BBS", "and case does not matter"},
		// The case #183 was opened for.
		{"Nite E", "Nite Eyes BBS", "a unique partial name resolves"},
		// Pos() searches from anywhere in the name, so this is not prefix-only.
		{"Eyes", "Nite Eyes BBS", "a unique substring resolves from the middle"},
		{"A-NET", "A-Net Online BBS", "matching is case-insensitive both ways"},
		// The question #183 left open, settled at unit offset 0x15c0: only a
		// count of exactly one resolves.
		{"The ", "", "a prefix shared by two planets is refused"},
		{"X-Bit", "", "a substring shared by two planets is refused"},
		{"night eyes", "", "a misspelling matches nothing"},
		{"", "", "an empty answer selects nothing"},
	} {
		t.Run(c.typed, func(t *testing.T) {
			if got := name(matchPlanet(planets, c.typed)); got != c.want {
				t.Errorf("matchPlanet(%q) = %q, want %q — %s", c.typed, got, c.want, c.why)
			}
		})
	}
}

// An ambiguous answer must not quietly pick one, which is the failure that would
// send an attack or a message to the wrong planet. Asserted separately because
// it is the whole reason #183 refused to guess.
func TestAnAmbiguousPlanetNameSelectsNothing(t *testing.T) {
	planets := []game.LeagueNode{
		{Number: 1, Name: "The X-Bit BBS"},
		{Number: 2, Name: "The uniX-Bit BBS"},
	}
	for _, typed := range []string{"The", "The ", "Bit", "BBS", "X-Bit"} {
		if p := matchPlanet(planets, typed); p != nil {
			t.Errorf("matchPlanet(%q) picked %q; two planets match, so it must refuse", typed, p.Name)
		}
	}
}
