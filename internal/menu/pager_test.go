package menu

import "testing"

// drainCounter is a fake session that counts DrainInput calls.
type drainCounter struct {
	*fakeSession
	drains int
}

func (d *drainCounter) DrainInput() { d.drains++ }

// A page key is read with the rest of its line dropped. A terminal that sends
// CR LF for Enter would otherwise leave the LF to answer the next page's
// prompt at once, and that page would scroll past unread. The help pager did
// not drain when it was written; the bulletin pager it replaced did.
func TestPagerDrainsTheRestOfTheLine(t *testing.T) {
	s := &drainCounter{fakeSession: &fakeSession{keys: []rune("  ")}}
	page := &linePager{s: s, perPage: 2}
	for _, l := range []string{"one", "two", "three", "four", "five"} {
		if !page.line(l) {
			t.Fatalf("the pager stopped at %q without a Q", l)
		}
	}
	if s.drains != 2 {
		t.Errorf("drained %d times across two page breaks, want 2", s.drains)
	}
}
