package menu

import "testing"

// The mood line has BRE's eleven uneven bands (process_end_of_turn, BRE.OVR
// 0xD5D8): 0-10, 11-17, 18-23, 24-30, 31-38, 39-47, 48-65, 66-80, 81-88, 89-96,
// 97-100. Each pair below is one band's two edges, written out as literals so a
// retuned table fails here.
func TestPeopleMoodBands(t *testing.T) {
	bands := [][2]int{
		{0, 10}, {11, 17}, {18, 23}, {24, 30}, {31, 38}, {39, 47},
		{48, 65}, {66, 80}, {81, 88}, {89, 96}, {97, 100},
	}
	seen := map[string]bool{}
	for _, b := range bands {
		lo, hi := peopleMood(b[0]), peopleMood(b[1])
		if lo != hi {
			t.Errorf("support %d and %d share a band but read %q and %q", b[0], b[1], lo, hi)
		}
		if seen[lo] {
			t.Errorf("band %d-%d repeats an earlier band's line %q", b[0], b[1], lo)
		}
		seen[lo] = true
	}
}
