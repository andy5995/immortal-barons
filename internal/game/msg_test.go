package game

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// text is an optional report in English, "" when there is none.
func text(m *Msg) string {
	if m == nil {
		return ""
	}
	return m.English()
}

// list is a list of phrases in English, "" when it is empty.
func list(l []Msg) string {
	if len(l) == 0 {
		return ""
	}
	return listIn("", nil, l)
}

func TestMsgFillsNamedPlaceholders(t *testing.T) {
	m := say("{who} of {board} sent {n} agents, {gold} gold.", "who", "Selby", "board", "Home", "n", 3, "gold", comma(1234567))
	if got, want := m.English(), "Selby of Home sent 3 agents, 1,234,567 gold."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// A value is put in after the template is translated, and a value that looks
// like a placeholder is left alone.
func TestMsgDoesNotReplaceInsideValues(t *testing.T) {
	m := say("{a} and {b}", "a", "{b}", "b", "x")
	if got := m.English(); got != "{b} and x" {
		t.Errorf("got %q", got)
	}
}

func TestMsgPluralFollowsItsCount(t *testing.T) {
	for n, want := range map[int]string{1: "1 region fell.", 2: "2 regions fell."} {
		m := sayN("{n} region fell.", "{n} regions fell.", "n", "n", n)
		if got := m.English(); got != want {
			t.Errorf("n=%d: got %q, want %q", n, got, want)
		}
	}
}

func TestMsgListsJoinAsTheOriginalDoes(t *testing.T) {
	troops := counted(Trooper, short(5000))
	jets := counted(Jet, short(1_500_000))
	tanks := counted(Tank, short(3))
	for _, tc := range []struct {
		l    []Msg
		want string
	}{
		{nil, "nothing"},
		{[]Msg{troops}, "5000 Troopers"},
		{[]Msg{troops, jets}, "5000 Troopers and 1500k Jets"},
		{[]Msg{troops, jets, tanks}, "5000 Troopers, 1500k Jets, and 3 Tanks"},
	} {
		if got := say("{l}", "l", tc.l).English(); got != tc.want {
			t.Errorf("got %q, want %q", got, tc.want)
		}
	}
}

func TestMsgJoinsSentencesAndLines(t *testing.T) {
	a, b := say("One."), say("Two.")
	if got := sentences(a, Msg{}, b).English(); got != "One. Two." {
		t.Errorf("sentences = %q", got)
	}
	if got := lines(a, b).English(); got != "One.\nTwo." {
		t.Errorf("lines = %q", got)
	}
	if got := sentences(Msg{}, a); got.English() != "One." || len(got.J) != 0 {
		t.Errorf("a single sentence should stand alone, got %+v", got)
	}
}

// The point of the type (#297): the same stored message reads in the reader's
// language, nested phrases included, and falls back to English per template.
func TestMsgRendersInTheReadersLanguage(t *testing.T) {
	m := say("{n} {unit}", "n", 3, "unit", say("Troopers"))
	if got := m.In("de"); got != "3 Soldaten" {
		t.Errorf("de = %q, want 3 Soldaten", got)
	}
	if got := say("{l}", "l", []Msg{}).In("de"); got != "nichts" {
		t.Errorf("an empty list in de = %q, want nichts", got)
	}
	if got := say("No catalog has this {x}.", "x", "line").In("de"); got != "No catalog has this line." {
		t.Errorf("an unknown template should fall back to English, got %q", got)
	}
}

// A Msg crosses the wire and sits in world.json, so it must come back whole:
// figures, their format, nested phrases, lists and plural forms.
func TestMsgSurvivesJSON(t *testing.T) {
	in := sentences(
		sayN("{n} agent sent by {who}", "{n} agents sent by {who}", "n", "n", 2, "who", "Selby"),
		say("{l} returned.", "l", []Msg{counted(Jet, short(2_000_000)), counted(Tank, comma(1234))}),
		say("Your {op} failed.", "op", say("S3-Sabre")),
	)
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var out Msg
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if got, want := out.English(), in.English(); got != want {
		t.Errorf("after a round trip %q, want %q\n%s", got, want, raw)
	}
	if want := "2 agents sent by Selby 2000k Jets and 1,234 Tanks returned. Your S3-Sabre failed."; in.English() != want {
		t.Errorf("rendered %q, want %q", in.English(), want)
	}
}

// An event keeps its English Text beside its Msg, for world.json's reader, and
// one saved before Msg existed still renders from Text.
func TestEventRendersMsgOrLegacyText(t *testing.T) {
	e := &Empire{}
	e.addEvent(say("{n} {unit}", "n", 3, "unit", say("Troopers")))
	ev := e.Events[0]
	if ev.Text != "3 Troopers" || ev.Render("de", nil) != "3 Soldaten" {
		t.Errorf("event Text %q, de %q", ev.Text, ev.Render("de", nil))
	}
	var old Event
	if err := json.Unmarshal([]byte(`{"When":"2026-01-02T03:04:05Z","Text":"An old line."}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Render("de", nil) != "An old line." {
		t.Errorf("a legacy event should render its Text, got %q", old.Render("de", nil))
	}
}

// The report a target board writes travels as a Msg under a key of its own.
// One saved before Protocol 4 carried English text under "Report", and a
// board's own Outbox keeps such replies in world.json across its upgrade, so
// loading reads that text rather than dropping it; the same holds for a bid's
// reason.
func TestAttackResultReportTravelsAsAMsg(t *testing.T) {
	res := AttackResult{ID: 1, Report: optMsg(say("Your {op} hit {who}.", "op", say("S3-Sabre"), "who", "Victim"))}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var back AttackResult
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if text(back.Report) != "Your S3-Sabre hit Victim." {
		t.Errorf("report after the wire = %q (%s)", text(back.Report), raw)
	}

	var p Packet
	old := `{"FromBoard":"B","Results":[{"ID":1,"Report":"Your agents sank Victim's morale."}],` +
		`"TradeFills":[{"ID":2,"Reason":"Seller had only 3 left."}]}`
	if err := json.Unmarshal([]byte(old), &p); err != nil {
		t.Fatalf("an Outbox saved before Protocol 4 should load: %v", err)
	}
	if got := text(p.Results[0].Report); got != "Your agents sank Victim's morale." || p.Results[0].ID != 1 {
		t.Errorf("an old report loaded as %q (ID %d)", got, p.Results[0].ID)
	}
	if got := text(p.TradeFills[0].Reason); got != "Seller had only 3 left." || p.TradeFills[0].ID != 2 {
		t.Errorf("an old reason loaded as %q (ID %d)", got, p.TradeFills[0].ID)
	}
	resaved, _ := json.Marshal(p.Results[0])
	if strings.Contains(string(resaved), `"Report"`) {
		t.Errorf("a loaded old report should save under ReportMsg only: %s", resaved)
	}
}

// raidText is RaidFaction with its report in English.
func raidText(w *World, a *Empire, faction, troopers, jets, tanks int) (string, int) {
	m, land := w.RaidFaction(a, faction, troopers, jets, tanks)
	return m.English(), land
}

// A strike's returning report is laid out as the original's: header with the
// target and its planet, a Date line with the verdict, the outcome, and the
// three tallies indented under it in BRE's order. The date is the departure,
// shown in the reader's zone.
func TestStrikeReportWearsBREsLayout(t *testing.T) {
	launched := time.Date(2026, 9, 1, 20, 35, 16, 0, time.UTC)
	sent := InFlightStrike{TargetBoard: "The Eclipse", TargetEmpire: "Jason Bourne", Launched: launched}
	res := AttackResult{Kind: "Extended Battle", TargetBoard: "The Eclipse", TargetEmpire: "Jason Bourne",
		Outcome: OutcomeWon, Won: true, LandTaken: 1637,
		Enemy: UnitLoss{Troopers: 53_000, Jets: 19_000, Turrets: 515_000, Tanks: 2102}}
	committed := AttackForce{Troopers: 20_638, Jets: 14_433_000, Tanks: 1_551_000, Bombers: 50_500}
	back := AttackForce{Troopers: 20_000, Jets: 14_000_000, Tanks: 1_505_000, Bombers: 49_000}
	want := "Extended Battle Results.  Target: Jason Bourne (The Eclipse)\n" +
		"Date: 09/01/2026  20:35:16 UTC    Result: SUCCESS\n" +
		"Your forces broke the enemy and captured 1637 regions.\n" +
		"  20k Troopers, 14m Jets, 1505k Tanks, and 49k Bombers returned.\n" +
		"  You lost 638 Troopers, 433k Jets, 46k Tanks, and 1500 Bombers!\n" +
		"  You destroyed 53k Troopers, 19k Jets, 515k Turrets, and 2102 Tanks!"
	if got := strikeReport(sent, res, committed, back).English(); got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}
	east := time.FixedZone("EST", -5*3600)
	if got := strikeReport(sent, res, committed, back).Render("", east); !strings.Contains(got, "Date: 09/01/2026  15:35:16 EST") {
		t.Errorf("the date should be in the reader's zone:\n%s", got)
	}
	sent.Launched = time.Time{}
	if got := strikeReport(sent, res, committed, back).English(); !strings.Contains(got, "\nResult: SUCCESS\n") {
		t.Errorf("a strike saved without a departure time should still give its verdict:\n%s", got)
	}
}

// The defender's report is laid out as the original's (eots-ibbs-03.cap): the
// planet and realm the strike came from, then indented the force, the verdict,
// the losses and the land. IB adds the strike's kind to the header, and says
// so when a strike hit the whole planet.
func TestInvasionReportWearsBREsLayout(t *testing.T) {
	atk := RemoteAttack{FromBoard: "The Eclipse", FromEmpire: "Pirates Ahoy!", TargetEmpire: "Victim",
		Kind: NormalAttack, Contributors: []Contribution{{AttackForce: AttackForce{Troopers: 75_000, Jets: 3_338_000, Tanks: 5_727_000}}}}
	got := invasionReport(atk, true, UnitLoss{Troopers: 8075, Turrets: 1_125_000, Tanks: 23_000}, 1013,
		AttackForce{Jets: 412_000, Tanks: 690_000}).English()
	want := "Invasion From The Eclipse by Pirates Ahoy! (Normal Attack)\n" +
		"  75k Troopers, 3338k Jets, and 5727k Tanks attacked!\n" +
		"  Your forces lost the battle!\n" +
		"  You lost 8075 Troopers, 1125k Turrets, and 23k Tanks!\n" +
		"  You also lost 1013 regions!\n" +
		"  The attackers lost 412k Jets and 690k Tanks!"
	if got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}
	whole := RemoteAttack{FromBoard: "Starship Junkyard", Group: true,
		Contributors: []Contribution{{AttackForce: AttackForce{Troopers: 1}}}}
	got = invasionReport(whole, false, UnitLoss{}, 0, AttackForce{}).English()
	want = "Global Invasion From Starship Junkyard (group attack)\n" +
		"  It struck the whole planet.\n" +
		"  1 Trooper attacked!\n" +
		"  Your forces won the battle!\n" +
		"  You lost nothing!"
	if got != want {
		t.Errorf("planet-wide report:\n%s\nwant:\n%s", got, want)
	}
}
