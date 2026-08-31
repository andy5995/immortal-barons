package numfmt

import "testing"

// The figures are BRE's own, asserted as golden literals rather than derived
// from ShortThreshold: they are the fidelity contract, so a retune has to fail
// here and produce new evidence. The 1213k / 3180k / 12m rows are read off
// real captures (docs/dev/bre-screens.md).
func TestShortMatchesBRE(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{9999, "9999"},        // under the threshold: printed whole
		{10000, "10k"},        // the threshold itself shortens
		{89123, "89k"},        // capture: (C) Infinitie score
		{290456, "290k"},      // capture: score column
		{1213456, "1213k"},    // ONE divide leaves 1213, under the threshold: no "m"
		{1962000, "1962k"},    // capture: (A) Dynoland score
		{3180999, "3180k"},    // capture: (A) Imperial net worth — 3.18M is still "k"
		{9999999, "9999k"},    // the last value before the second divide
		{10000000, "10m"},     // the crossover, at ten million and not at one
		{12345678, "12m"},     // capture: (A) Dynoland net worth
		{2000000000, "2000m"}, // no "b" tier exists
		{-50000, "-50000"},    // negatives are never shortened
	} {
		if got := Short(tc.in); got != tc.want {
			t.Errorf("Short(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// BRE groups a table figure only once its digits run past four — the same
// screen prints Territory 3469 bare and 14,203 grouped. Golden literals, from
// the capture and the length test at BRE.EXE image 0x0E2F7.
func TestGroupLongMatchesBRE(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{3469, "3469"},    // capture: (A) Imperial territory — four digits, bare
		{9999, "9999"},    // the widest that stays bare
		{10000, "10,000"}, // five digits: grouped
		{14203, "14,203"}, // capture: (A) Dynoland territory
		{1234567, "1,234,567"},
		{-12345, "-12,345"}, // the sign is not one of the four digits
		{-3469, "-3469"},
	} {
		if got := GroupLong(tc.in, "en"); got != tc.want {
			t.Errorf("GroupLong(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
	if got := GroupLong(14203, "de"); got != "14.203" {
		t.Errorf("GroupLong de = %q, want %q", got, "14.203")
	}
}

// The Daily Bulletin's Total Net Worth row: divided to thousands and ALWAYS
// suffixed, so a planet worth nothing prints "0k", and never stepping to "m".
// Golden literals from cap/121125-666H4H_Camembert_Public.cap.
func TestThousandsMatchesBRE(t *testing.T) {
	for _, tc := range []struct {
		in   int64
		want string
	}{
		{0, "0k"},            // capture: a planet on day one
		{12_500, "12k"},      // capture
		{1_255_000, "1255k"}, // four digits: no comma
		{34_833_289, "34,833k"},
		{97_678_000, "97,678k"},       // capture
		{2_000_000_000, "2,000,000k"}, // never steps to m or b
	} {
		if got := Thousands(tc.in, "en"); got != tc.want {
			t.Errorf("Thousands(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
