package game

import "testing"

// The defender is told which kind of strike hit them — IB's own divergence, see
// invasionReport. A group attack leaves Kind at its zero value, which IS
// QuickStrike, so the group case must not report a quick strike.
func TestInvasionReportNamesTheStrikeKind(t *testing.T) {
	for _, tc := range []struct {
		name  string
		atk   RemoteAttack
		want  string
		avoid string
	}{
		{"quick", RemoteAttack{FromBoard: "Far", Kind: QuickStrike}, "Quick Strike", ""},
		{"normal", RemoteAttack{FromBoard: "Far", Kind: NormalAttack}, "Normal Attack", ""},
		{"extended", RemoteAttack{FromBoard: "Far", Kind: ExtendedBattle}, "Extended Battle", ""},
		{"group", RemoteAttack{FromBoard: "Far", Group: true}, "group attack", "Quick Strike"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := invasionReport(tc.atk, true, UnitLoss{Troopers: 1}, 5)
			if !contains(got, tc.want) {
				t.Errorf("report does not name %q:\n%s", tc.want, got)
			}
			if tc.avoid != "" && contains(got, tc.avoid) {
				t.Errorf("report wrongly names %q (Kind's zero value leaked):\n%s", tc.avoid, got)
			}
		})
	}
}
