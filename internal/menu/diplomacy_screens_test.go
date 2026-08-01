package menu

import (
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// plainLines strips the SGR escapes and returns the rendered lines, so a test can
// measure BRE's column geometry.
func plainLines(out string) []string {
	return strings.Split(sgr.ReplaceAllString(out, ""), "\n")
}

func findLine(lines []string, contains string) string {
	for _, l := range lines {
		if strings.Contains(l, contains) {
			return l
		}
	}
	return ""
}

// The proposal runs the stats into the prompt on one unindented line, as BRE
// does: "Regions: N; Net Worth: N; Score: N; Accept? (Y/n)". IB comma-groups the
// figures where BRE does not.
func TestTreatyOfferMatchesBRE(t *testing.T) {
	w := newWorld()
	var proposer string
	w.With(func() {
		for _, e := range w.Empires {
			if e != w.Player() {
				proposer = e.Name
				e.Land, e.Score = 19445, 2442093
				break
			}
		}
		w.Player().TreatyOffers = []game.TreatyOffer{{From: proposer, Type: "Tariff Trade Agreement"}}
	})

	f := &fakeSession{keys: []rune("n")}
	reviewTreatyOffers(f, w)

	lines := plainLines(f.out.String())
	if got := findLine(lines, "proposes a"); got != proposer+" proposes a Tariff Trade Agreement." {
		t.Errorf("proposal line = %q", got)
	}
	stats := findLine(lines, "Regions:")
	if !strings.HasPrefix(stats, "Regions: 19,445 │ ") {
		t.Errorf("stats should be unindented, got %q", stats)
	}
	if !strings.Contains(stats, "Score: 2,442,093 │ Accept? (Y/n)") {
		t.Errorf("the prompt should run on from the stats, got %q", stats)
	}
}

// Relations lists every living realm — "None" included — under a 75-column inset
// rule, with the name in a 40-column field after "[X]  ".
func TestRelationsScreenMatchesBRE(t *testing.T) {
	w := newWorld()
	f := &fakeSession{}
	viewDiplomacy(f, w)

	lines := plainLines(f.out.String())
	if findLine(lines, "-*Relations*-") == "" {
		t.Error("expected BRE's -*Relations*- title")
	}
	rule := findLine(lines, "═")
	if n := len([]rune(rule)); n != relationsRuleWidth {
		t.Errorf("rule is %d columns, want %d: %q", n, relationsRuleWidth, rule)
	}
	var listed int
	for _, l := range lines {
		if !strings.HasPrefix(l, "[") {
			continue
		}
		listed++
		if got := strings.Index(l, "None"); got != 5+relationsNameWidth {
			t.Errorf("relation starts at column %d, want %d: %q", got, 5+relationsNameWidth, l)
		}
	}
	var alive int
	w.With(func() {
		for _, e := range w.Empires {
			if e.Alive && e != w.Player() {
				alive++
			}
		}
	})
	if listed != alive {
		t.Errorf("listed %d realms, want all %d living ones (treaty or not)", listed, alive)
	}
}

// Alliance Strength: the rule spans the columns, a zero reads "NONE", and the
// Total Forces line lines up with the ally rows.
func TestAllianceStrengthMatchesBRE(t *testing.T) {
	w := newWorld()
	p := w.Player()
	ally := recipients(w)[0]
	ally.Troopers, ally.Tanks, ally.Agents = 3210000, 617000, 0
	w.World.ProposeTreaty(ally, p, "Full Defense Alliance")
	w.World.AcceptTreaty(p, ally.Name, "Full Defense Alliance")

	f := &fakeSession{}
	allianceStrength(f, w)

	lines := plainLines(f.out.String())
	rule := findLine(lines, "═")
	if n := len([]rune(rule)); n != allyRuleWidth {
		t.Errorf("rule is %d columns, want %d: %q", n, allyRuleWidth, rule)
	}
	row := findLine(lines, ally.Name)
	total := findLine(lines, "Total Forces")
	if row == "" || total == "" {
		t.Fatalf("expected an ally row and a Total Forces row, got %q", f.out.String())
	}
	if !strings.Contains(row, "NONE") {
		t.Errorf("a zero contribution should read NONE, got %q", row)
	}
	if !strings.Contains(row, "963,000") { // 30% of the ally's 3,210,000 troopers
		t.Errorf("figures should be comma-grouped, got %q", row)
	}
	if len([]rune(row)) != len([]rune(total)) {
		t.Errorf("Total Forces must line up with the ally rows:\n%q\n%q", row, total)
	}
}
