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
func TestCivilianAdvisorFoodProjectionDoesNotDoubleCountGrowth(t *testing.T) {
	var d advisorData
	d.p.Food = 52_000 // post-growth stock
	d.foodGrown = 47_000
	d.foodEaten = 60_000 // pre-growth stock 5,000; 5,000 - 60,000 < 0 -> starves this turn
	d.p.Support = 100

	s := &fakeSession{}
	joined := joinAdvisor(advisorReport(s, d, advisorCivilian))
	if !strings.Contains(joined, "will not last the turn") {
		t.Errorf("post-growth stock below this turn's consumption should warn it won't last the turn, got:\n%s", joined)
	}
}

// With ample stock, the run-out is computed from the pre-growth stock divided by
// the shortfall (not the post-growth stock, which would over-count by a turn's
// growth).
func TestCivilianAdvisorRunOutUsesPreGrowthStock(t *testing.T) {
	var d advisorData
	d.p.Food = 200_000 // post-growth
	d.foodGrown = 40_000
	d.foodEaten = 60_000 // shortfall 20,000; pre-growth stock 160,000 -> 160000/20000 = 8 turns
	d.p.Support = 100

	s := &fakeSession{}
	joined := joinAdvisor(advisorReport(s, d, advisorCivilian))
	if !strings.Contains(joined, "run out") || !strings.Contains(joined, "8 turns") {
		t.Errorf("run-out should be pre-growth stock / shortfall = 8 turns, got:\n%s", joined)
	}
}
