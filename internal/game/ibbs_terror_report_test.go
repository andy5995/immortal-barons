package game

import "testing"

// The sender's report must account for every agent it paid for: a batch bigger
// than the target can absorb otherwise reads exactly like a batch that was the
// right size.
func TestTerrorOpReportAccountsForEveryAgent(t *testing.T) {
	for _, tc := range []struct {
		name              string
		sent, hit, caught int
		want              string
	}{
		{"batch outruns the target", 25, 7, 1, "Sabotage HQ: 7 of your 25 agents got through. 1 caught, 17 achieved nothing."},
		{"clean sweep", 4, 4, 0, "Sabotage HQ: all 4 of your agents got through."},
		{"all stopped", 3, 0, 3, "Sabotage HQ: none of your 3 agents got through. 3 caught."},
		{"lone agent lands", 1, 1, 0, "Sabotage HQ: your agent got through."},
		{"lone agent caught", 1, 0, 1, "Sabotage HQ: your agent was caught."},
		{"lone agent wasted", 1, 0, 0, "Sabotage HQ: your agent got through and achieved nothing."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := terrorOpReport(TerrorOpSabotageHQ, tc.sent, tc.hit, tc.caught)
			if got != tc.want {
				t.Errorf("terrorOpReport(%d, %d, %d):\n got %q\nwant %q", tc.sent, tc.hit, tc.caught, got, tc.want)
			}
		})
	}
}
