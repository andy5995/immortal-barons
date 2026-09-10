package menu

import (
	"testing"
	"time"
)

// The short form steps hours → minutes → seconds as a wait shortens, rounds up
// so it never reads as less than it is, and never runs past four columns — the
// cell the original's whole-hours figure sat in (#269).
func TestShortDurationSteps(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{120 * time.Hour, "120h"},
		{8*time.Hour - time.Second, "8h"},
		{2 * time.Hour, "2h"},
		{100*time.Minute + time.Second, "2h"},
		{100 * time.Minute, "100m"},
		{90 * time.Minute, "90m"},
		{61 * time.Second, "2m"},
		{60 * time.Second, "60s"},
		{45 * time.Second, "45s"},
		{0, "0s"},
		{-time.Hour, "0s"},
	}
	for _, c := range cases {
		got := shortDuration(c.d)
		if got != c.want {
			t.Errorf("shortDuration(%s) = %q, want %q", c.d, got, c.want)
		}
		if len(got) > gaWidthLeave-1 {
			t.Errorf("shortDuration(%s) = %q, wider than the Leave cell", c.d, got)
		}
	}
}
