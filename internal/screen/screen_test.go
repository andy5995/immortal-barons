package screen

import "testing"

func id(s string) string { return s } // identity translator

func TestRenderSubstitutesLabelsAndValues(t *testing.T) {
	tpl := []byte("{Gold  } [gold     ]\n")
	out := Render(tpl, map[string]string{"gold": "1,234"}, id)
	// Label left-justified in its 8-col span, value right-justified in its 11-col span.
	want := "Gold     " + "      1,234\n"
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestRenderTranslatesLabels(t *testing.T) {
	tpl := []byte("{Gold      }")
	out := Render(tpl, nil, func(s string) string {
		if s == "Gold" {
			return "Золото"
		}
		return s
	})
	if out != "Золото      " { // 6 runes + 6 pad = 12-col span
		t.Errorf("got %q (%d runes)", out, len([]rune(out)))
	}
}

func TestRenderTruncatesOverflow(t *testing.T) {
	// A value wider than its 6-col span ([x   ]) is cut, not allowed to shift.
	out := Render([]byte("[x   ]|"), map[string]string{"x": "1234567"}, id)
	if out != "123456|" {
		t.Errorf("got %q, want %q", out, "123456|")
	}
}

func TestRenderRightAlignsLabelWithLeadingSpace(t *testing.T) {
	out := Render([]byte("{   Gold}|"), nil, id)
	if out != "     Gold|" { // 9-col span, right-aligned
		t.Errorf("got %q", out)
	}
}

func TestRenderKeepsAnsiColorTokensIntact(t *testing.T) {
	// The '[' in the color codes must not swallow the following [value] token.
	tpl := []byte("\x1b[37m{Gold}\x1b[96m[gold]\x1b[0m")
	out := Render(tpl, map[string]string{"gold": "99"}, id)
	want := "\x1b[37mGold  \x1b[96m    99\x1b[0m" // {Gold}=6-col label, [gold]=6-col value
	if out != want {
		t.Errorf("got %q, want %q", out, want)
	}
}

func TestRenderDoubledDelimiterIsLiteral(t *testing.T) {
	out := Render([]byte("{{ not a token }}"), nil, id)
	if out != "{ not a token }" {
		t.Errorf("got %q", out)
	}
}

func TestFromCP437MapsHighBytesAndKeepsEscapes(t *testing.T) {
	// 0xDB is the full block; the ESC color sequence (all ASCII) is untouched.
	out := Render([]byte{0x1b, '[', '3', '6', 'm', 0xDB, 0xB0}, nil, id)
	if out != "\x1b[36m█░" {
		t.Errorf("got %q", out)
	}
}

func TestLabelsReportsMsgidAndWidth(t *testing.T) {
	got := Labels([]byte("{Gold  }x[v]y{Popular Support   }"))
	want := []Label{{"Gold", 8}, {"Popular Support", 20}}
	if len(got) != len(want) {
		t.Fatalf("got %d labels, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("label %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}
