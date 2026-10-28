package menu

import (
	"strings"
	"testing"
)

// joinAdvisor joins an advisor report's line texts, for substring assertions.
func joinAdvisor(lines []advisorLine) string {
	var texts []string
	for _, l := range lines {
		texts = append(texts, l.Text)
	}
	return strings.Join(texts, "\n")
}

// Food is credited at turn start now, so d.p.Food already includes this turn's
// growth. The Civilian advisor's food projection must not add the growth a
// second time. Here the post-growth stock cannot cover this turn's consumption,
// so the advisor must warn it will not last the turn — not project several.
func TestCivilianAdvisorWarnsWhenUnfedStoresMissTheMeal(t *testing.T) {
	var d advisorData
	d.p.Food = 52_000    // post-growth stock
	d.foodEaten = 60_000 // more than the stores hold -> starves this turn
	d.p.Support = 100

	s := &fakeSession{}
	joined := joinAdvisor(advisorReport(s, d, advisorCivilian))
	if !strings.Contains(joined, "will not last the turn") {
		t.Errorf("post-growth stock below this turn's consumption should warn it won't last the turn, got:\n%s", joined)
	}
}

// After the feed stage, which comes before the menus, the stores are what is
// left once this turn has eaten. Less than a turn's meal is not a famine then:
// the next shortfall is a turn away. Taken from play: 4,345 in store against
// 87,749 eaten and about 87,129 grown.
func TestCivilianAdvisorAfterFeedingCountsWhatIsLeft(t *testing.T) {
	var d advisorData
	d.p.TurnProgress.Fed = true
	d.p.Food = 4_345
	d.avgAgri = 41_445
	d.avgRiver = 45_684
	d.foodEaten = 87_749 // shortfall 620; 4,345 / 620 -> 7 turns covered, out on the 8th
	d.p.Regions.Agricultural, d.p.Regions.River = 1, 1
	d.p.Support = 100

	joined := joinAdvisor(advisorReport(&fakeSession{}, d, advisorCivilian))
	if strings.Contains(joined, "will not last the turn") {
		t.Errorf("a fed realm was told its food will not last the turn:\n%s", joined)
	}
	for _, want := range []string{"In all, we grow about 87,129 units", "net loss of about 620 units", "run out in about 8 turns"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q:\n%s", want, joined)
		}
	}
}

// The Civilian advisor names agricultural and river food on separate lines, and
// leaves the river line out for a realm with no rivers.
func TestCivilianAdvisorSplitsFoodBySource(t *testing.T) {
	var d advisorData
	d.p.Food = 100_000
	d.p.Support = 100
	d.p.Regions.Agricultural, d.p.Regions.River = 10, 4
	d.avgAgri, d.avgRiver = 3_020, 119

	s := &fakeSession{}
	joined := joinAdvisor(advisorReport(s, d, advisorCivilian))
	if !strings.Contains(joined, "agricultural regions grow about 3,020") || !strings.Contains(joined, "rivers add about 119") {
		t.Errorf("want separate agricultural and river figures, got:\n%s", joined)
	}
	d.p.Regions.River = 0
	if joined = joinAdvisor(advisorReport(s, d, advisorCivilian)); strings.Contains(joined, "rivers") {
		t.Errorf("no rivers, no river line; got:\n%s", joined)
	}
}

// A realm without farms is not told its farms grow nothing: the rivers stand on
// their own, and a realm with neither says it grows no food at all.
func TestCivilianAdvisorFoodWithoutFarms(t *testing.T) {
	var d advisorData
	d.p.Food = 100_000
	d.p.Support = 100
	d.p.Regions.River = 4
	d.avgRiver = 119

	s := &fakeSession{}
	joined := joinAdvisor(advisorReport(s, d, advisorCivilian))
	if strings.Contains(joined, "agricultural regions grow") || !strings.Contains(joined, "rivers yield about 119") {
		t.Errorf("rivers only: want the river line alone, got:\n%s", joined)
	}
	d.p.Regions.River, d.avgRiver = 0, 0
	if joined = joinAdvisor(advisorReport(s, d, advisorCivilian)); !strings.Contains(joined, "we grow no food") {
		t.Errorf("no food regions: want it said, got:\n%s", joined)
	}
}

// The same turn gets the same forecast either side of the feed stage: the
// meal moves food out of the stores, not the turn they run out on.
func TestCivilianAdvisorRunOutIsTheSameFedOrNot(t *testing.T) {
	var unfed, fed advisorData
	for _, d := range []*advisorData{&unfed, &fed} {
		d.avgAgri = 40_000
		d.foodEaten = 60_000 // shortfall 20,000
		d.p.Support = 100
	}
	unfed.p.Food = 200_000 // the meal still to come
	fed.p.Food = 140_000   // after it
	fed.p.TurnProgress.Fed = true

	want := "run out in about 8 turns" // 140,000 / 20,000 = 7 covered, out on the 8th
	for name, d := range map[string]advisorData{"unfed": unfed, "fed": fed} {
		if joined := joinAdvisor(advisorReport(&fakeSession{}, d, advisorCivilian)); !strings.Contains(joined, want) {
			t.Errorf("%s: want %q, got:\n%s", name, want, joined)
		}
	}
}

// Growth that exactly matches the meal is said plainly, not as a gain of 0.
func TestCivilianAdvisorBalancedFood(t *testing.T) {
	var d advisorData
	d.p.TurnProgress.Fed = true
	d.p.Food = 1_000
	d.avgAgri, d.foodEaten = 60_000, 60_000
	d.p.Support = 100

	joined := joinAdvisor(advisorReport(&fakeSession{}, d, advisorCivilian))
	if !strings.Contains(joined, "That leaves nothing to spare.") || strings.Contains(joined, "net gain") {
		t.Errorf("want the balanced line, got:\n%s", joined)
	}
}

// Stores smaller than one turn's shortfall run out next turn, fed or not; "about
// 1 turn" would leave the reader to work that out.
func TestCivilianAdvisorRunOutNextTurn(t *testing.T) {
	for _, fed := range []bool{true, false} {
		var d advisorData
		d.p.TurnProgress.Fed = fed
		d.p.Food = 4_345
		d.avgAgri = 9_956
		d.foodEaten = 87_749 // shortfall 77,793
		if !fed {
			d.p.Food += d.foodEaten // this turn's meal still to come
		}
		d.p.Support = 100

		joined := joinAdvisor(advisorReport(&fakeSession{}, d, advisorCivilian))
		if !strings.Contains(joined, "Our stores will run out next turn.") || strings.Contains(joined, "run out in") {
			t.Errorf("fed=%v: want the run-out next turn, got:\n%s", fed, joined)
		}
	}
}
