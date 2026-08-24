package game

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// packetShape lists the JSON fields of Packet AND of every struct reachable
// from it, in declaration order, with their tags. Declaration order IS the
// marshalled byte order and the tag decides whether an empty field is written at
// all, so this string is exactly what a signature depends on.
//
// It recurses because Packet's own field list is not the whole wire format: a
// field added to LeagueConfig or LeagueNode changes the Coordinator's signed
// bytes without touching Packet at all, and a check that stopped at the top
// level would wave that through.
func packetShape() string {
	var b strings.Builder
	seen := map[reflect.Type]bool{}
	var walk func(reflect.Type)
	walk = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		fmt.Fprintf(&b, "%s:\n", t.Name())
		var nested []reflect.Type
		for i := range t.NumField() {
			f := t.Field(i)
			b.WriteString("  " + f.Name)
			if tag, ok := f.Tag.Lookup("json"); ok {
				b.WriteString(" `json:\"" + tag + "\"`")
			}
			b.WriteString("\n")
			nested = append(nested, f.Type)
		}
		// Nested types after this whole struct, so the listing reads in the order
		// the bytes are written rather than jumping mid-struct.
		for _, n := range nested {
			walk(n)
		}
	}
	walk(reflect.TypeOf(Packet{}))
	return b.String()
}

// PacketShapeFile is the frozen wire shape: every struct the packet format
// reaches, its fields in declaration order, and their json tags.
//
// It is frozen because boardSigningBytes marshals the WHOLE packet, so adding a
// field without omitempty changes the bytes of every packet a board signs. An
// older board drops the field it does not know, re-marshals without it, and
// every origin signature fails — not just for the new feature, but for ordinary
// mail and scores too. That is not hypothetical: the Coordinator payload broke
// exactly this way when Bulletins was added (1da5698), and a six-board league's
// orders stopped the same day while it looked like a wrong key.
//
// When this test fails the change is probably fine, but it has to be a decision:
//
//   - Adding a field? Give it `json:",omitempty"`, or zero it in
//     boardSigningBytes the way Protocol is, so a packet that does not use it is
//     byte-identical to what older boards produce.
//   - Changing the format in a way older boards cannot read anyway? Bump
//     game.Protocol, so they hold the packet instead of failing to verify it.
//   - Reordering or renaming? Do not. It invalidates every signature in flight
//     and buys nothing.
//
// Then regenerate the file in the same commit.
const PacketShapeFile = "testdata/packet-shape.txt"

func TestPacketWireShapeIsFrozen(t *testing.T) {
	raw, err := os.ReadFile(PacketShapeFile)
	if err != nil {
		t.Fatal(err)
	}
	// Strip CR even though .gitattributes now pins this file to LF: a checkout
	// made before that landed, or a source tarball unpacked on Windows, still
	// hands it back with CRLF. The failure is a diff between two strings that
	// look identical, on one platform, so it is worth not being able to happen.
	want := strings.ReplaceAll(string(raw), "\r\n", "\n")
	got := packetShape()
	if got == want {
		return
	}
	t.Errorf("the packet's wire shape moved, which can invalidate every signature in the league.\n"+
		"Read the comment on PacketShapeFile before regenerating it.\n\n%s",
		firstDifference(got, want))
}

// firstDifference names the line that moved, so the failure points at the change
// rather than printing two hundred lines and leaving the reader to diff them.
func firstDifference(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(g) || i < len(w); i++ {
		gl, wl := "", ""
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return fmt.Sprintf("first difference at line %d:\n  now:    %q\n  frozen: %q", i+1, gl, wl)
		}
	}
	return "the two differ only in length"
}
