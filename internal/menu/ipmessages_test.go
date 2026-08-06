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
	f := &fakeSession{keys: []rune("?\r4\rTerms.\r/s")}
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
	// would have wanted "1"), then the baron and 10 agents.
	f := &fakeSession{keys: []rune("?\r4\r1\r10\r ")}
	terroristOps(f, w)
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
