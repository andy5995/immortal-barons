package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// runTerrorOps drives InterPlanetary item 2 and reports a script that ran dry
// as an error rather than a panic.
func runTerrorOps(f *fakeSession, w *ctx) (err error) {
	defer session.GuardEnd(&err)
	terroristOps(BuildMenus().TerrorOps)(f, w)
	return nil
}

// terrorWorld is a caller able to send, facing three barons on The Eclipse
// (roster number 4), with the original's captured allowance of 15 a day.
func terrorWorld() *ctx {
	w := ipWorldWithRoster()
	w.Config.MaxTerrorOps = 15
	p := w.Player()
	p.Agents, p.Protection, p.Gold = 50, 0, 1_000_000_000
	return w
}

// queuedTerrors is every terror op in the outbox, in send order.
func queuedTerrors(w *ctx) []game.RemoteTerror {
	var out []game.RemoteTerror
	for _, p := range w.Outbox {
		out = append(out, p.Terrors...)
	}
	return out
}

// The original's order, from cap/eots-ibbs-02.cap: planet, then baron, then the
// ops menu, which stays on that baron send after send — `(1; 15)` then `(1; 7)`
// — and closes on the send that uses up the day. The baron is asked for once.
func TestTerrorOpsPickTheTargetOnceAndLoopTheMenu(t *testing.T) {
	w := terrorWorld()
	// Demoralize x8 (accepted), Sabotage HQ x7 (accepted) — nothing after: the
	// menu must close by itself once fifteen have gone.
	f := &fakeSession{keys: []rune("?4\rB" + "3" + "8\r" + "y" + "9" + "7\r" + "y")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("the flow asked for more than the original does: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	for _, want := range []string{"Send how many? (1; 15)", "Send how many? (1; 7)", "8 agents sent out.", "7 agents sent out."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "Terrorize which baron?"); n != 1 {
		t.Errorf("the baron was asked for %d times, want once", n)
	}
	if strings.Index(out, "Terrorize which baron?") > strings.Index(out, "[Terrorist Ops]") {
		t.Error("the ops menu was drawn before the target was chosen")
	}
	got := queuedTerrors(w)
	if len(got) != 2 || got[0].TargetEmpire != "The Empire of Queg" || got[1].TargetEmpire != "The Empire of Queg" {
		t.Fatalf("queued %+v, want two sends at The Empire of Queg", got)
	}
	if got[0].Op != game.TerrorOpDemoralize || got[1].Op != game.TerrorOpSabotageHQ {
		t.Errorf("ops = %v, %v", got[0].Op, got[1].Op)
	}
	if p := w.Player(); p.TerrorOpsToday != 15 {
		t.Errorf("TerrorOpsToday = %d, want 15", p.TerrorOpsToday)
	}
}

// One agent goes without the price being asked about, and Enter at the count
// sends one: the original's number reader returns its lower bound.
func TestOneTerrorAgentSendsWithoutConfirming(t *testing.T) {
	w := terrorWorld()
	f := &fakeSession{keys: []rune("?4\rA" + "2" + "\r" + "0" + "\r")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "1 agent sent out.") {
		t.Fatalf("never sent:\n%s", out)
	}
	if strings.Contains(out, "Accept?") {
		t.Errorf("a lone agent was asked to confirm:\n%s", out)
	}
	if got := queuedTerrors(w); len(got) != 1 || got[0].Agents != 1 || got[0].Op != game.TerrorOpBombIntel {
		t.Errorf("queued %+v, want one Bomb Intelligence agent", got)
	}
}

// Leaving the ops menu goes back to the baron prompt on the same planet, not
// out of the item, and the next send goes at the baron chosen there.
func TestQuittingTerrorOpsAsksForTheBaronAgain(t *testing.T) {
	w := terrorWorld()
	f := &fakeSession{keys: []rune("?4\rA" + "0" + "C" + "1" + "1\r" + "0" + "\r")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	if n := strings.Count(out, "Terrorize which baron?"); n != 3 {
		t.Errorf("the baron was asked for %d times, want 3", n)
	}
	if n := strings.Count(out, "Terrorize which planet?"); n != 1 {
		t.Errorf("the planet was asked for %d times, want once", n)
	}
	if got := queuedTerrors(w); len(got) != 1 || got[0].TargetEmpire != "Gap Origix" {
		t.Errorf("queued %+v, want one send at Gap Origix", got)
	}
}

// With the day's allowance gone the item refuses before any target is asked
// for, as the original does (BRE.OVR 0x2afbf, unit ovr_02aca8 +0x372).
func TestSpentTerrorAllowanceRefusesBeforeTheTarget(t *testing.T) {
	w := terrorWorld()
	w.Player().TerrorOpsToday = 15
	f := &fakeSession{keys: []rune(" ")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v", err)
	}
	out := stripANSI(f.out.String())
	if !strings.Contains(out, game.ErrTerrorOpsExhausted.Error()) {
		t.Errorf("no refusal:\n%s", out)
	}
	if strings.Contains(out, "Terrorize which planet?") {
		t.Errorf("asked for a target first:\n%s", out)
	}
}

// One send is at most 255 agents, whatever is held, when the sysop sets no
// daily cap.
func TestTerrorSendIsCappedAt255(t *testing.T) {
	w := terrorWorld()
	w.Config.MaxTerrorOps = 0
	w.Player().Agents = 1_000
	// Send Spy, then 0 at the count backs out of it.
	f := &fakeSession{keys: []rune("?4\rA" + "1" + "0\r" + "0" + "\r")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Send how many? (1; 255)") {
		t.Errorf("the prompt is not capped at 255:\n%s", out)
	}
	if got := queuedTerrors(w); len(got) != 0 {
		t.Errorf("a cancelled count still sent: %+v", got)
	}
}

// Declining the price sends nothing and charges nothing, and the menu stays on
// the same baron: the original zeroes the count on a no (unit ovr_02aca8
// +0x853) and skips the send (+0x85d..+0x870).
func TestDecliningTheTerrorPriceSendsNothing(t *testing.T) {
	w := terrorWorld()
	p := w.Player()
	gold, agents := p.Gold, p.Agents
	// Demoralize x5, decline, Quit the ops menu, Enter at the baron prompt.
	f := &fakeSession{keys: []rune("?4\rA" + "3" + "5\r" + "n" + "0" + "\r")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	if !strings.Contains(out, "Accept?") {
		t.Fatalf("never asked to confirm:\n%s", out)
	}
	if got := queuedTerrors(w); len(got) != 0 {
		t.Errorf("a declined send was queued: %+v", got)
	}
	if p.Gold != gold || p.Agents != agents || p.TerrorOpsToday != 0 {
		t.Errorf("a declined send cost something: gold %d->%d, agents %d->%d, ops today %d",
			gold, p.Gold, agents, p.Agents, p.TerrorOpsToday)
	}
	// The menu comes back on the same baron: it is drawn a second time with the
	// baron asked for only once before it.
	menus := strings.Split(out, "[Terrorist Ops]")
	if len(menus) != 3 {
		t.Fatalf("the ops menu was drawn %d times, want 2:\n%s", len(menus)-1, out)
	}
	if n := strings.Count(menus[0]+menus[1], "Terrorize which baron?"); n != 1 {
		t.Errorf("the baron was asked for %d times before the menu came back, want 1", n)
	}
}

// Enter at the first baron prompt leaves the item without opening the ops
// menu (unit ovr_02aca8 +0x510..+0x517, then +0x9b0 returns).
func TestEnterAtTheBaronPromptLeavesTerrorOps(t *testing.T) {
	w := terrorWorld()
	f := &fakeSession{keys: []rune("?4\r" + "\r")}
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	if n := strings.Count(out, "Terrorize which baron?"); n != 1 {
		t.Fatalf("the baron was asked for %d times, want once:\n%s", n, out)
	}
	if strings.Contains(out, "[Terrorist Ops]") {
		t.Errorf("the ops menu opened with no target:\n%s", out)
	}
	if got := queuedTerrors(w); len(got) != 0 {
		t.Errorf("queued %+v, want nothing", got)
	}
}

// A baron the last scores packet had under New Realm Protection is refused at
// the baron prompt, and the item ends there, as every strike's target picker
// does: the ops menu never opens and nothing is sent.
func TestProtectedBaronIsRefusedAtTheTerrorPrompt(t *testing.T) {
	w := terrorWorld()
	w.RemoteBoards[0].Scores[1].Protected = true   // The Empire of Queg, letter B
	f := &fakeSession{keys: []rune("?4\rB" + " ")} // B, then a key for the pause
	if err := runTerrorOps(f, w); err != nil {
		t.Fatalf("script ran dry: %v\n%s", err, stripANSI(f.out.String()))
	}
	out := stripANSI(f.out.String())
	if want := "The Empire of Queg is under New Realm Protection"; !strings.Contains(out, want) {
		t.Fatalf("missing %q:\n%s", want, out)
	}
	if strings.Contains(out, "[Terrorist Ops]") {
		t.Errorf("the ops menu opened for a protected baron:\n%s", out)
	}
	if got := queuedTerrors(w); len(got) != 0 {
		t.Errorf("queued %+v, want nothing", got)
	}
}
