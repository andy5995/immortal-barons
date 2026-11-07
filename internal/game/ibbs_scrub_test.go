package game

import (
	"reflect"
	"testing"
)

// A realm name, a mail body and a report argument from another board arrive
// with every control character turned into "?", so none of them can write an
// escape sequence into a player's screen or a bulletin. A mail body alone keeps
// its newlines; in a name or a roster field a newline is cleaned like the rest.
// Bulletin data is bytes and is left exactly as sent.
func TestScrubPacketRemovesControlCharacters(t *testing.T) {
	p := Packet{
		FromBoard: "Far BBS",
		Scores:    []RemoteScore{{Empire: "Real\x1b]0;owned\x07m", PriorNames: []string{"Old\nName"}}},
		News:      []string{"two\nlines"},
		IPMessages: []IPMessage{{
			FromEmpire: "Sender",
			Body:       "line one\nline\x1b[1A two\u009b",
		}},
		Results:   []AttackResult{{Report: &Msg{T: "{who} won", A: map[string]Arg{"who": {S: "X\x1bY"}, "clean": {S: "fine"}}}}},
		Bulletins: &BulletinSet{Files: []BulletinFile{{Name: "art", Title: "T\x1b", Data: []byte("\x1b[31mred\x1b[0m")}}},
	}
	scrubPacket(&p)
	for _, got := range []string{
		p.Scores[0].Empire, p.Scores[0].PriorNames[0], p.IPMessages[0].Body,
		p.Results[0].Report.A["who"].S, p.Bulletins.Files[0].Title, p.News[0],
	} {
		if got != p.IPMessages[0].Body && HasControl(got) {
			t.Errorf("a control character survived: %q", got)
		}
	}
	if want := "line one\nline?[1A two?"; p.IPMessages[0].Body != want {
		t.Errorf("body = %q, want %q", p.IPMessages[0].Body, want)
	}
	if want := "Old?Name"; p.Scores[0].PriorNames[0] != want {
		t.Errorf("prior name = %q, want %q", p.Scores[0].PriorNames[0], want)
	}
	if want := "Real?]0;owned?m"; p.Scores[0].Empire != want {
		t.Errorf("realm = %q, want %q", p.Scores[0].Empire, want)
	}
	if p.Results[0].Report.A["clean"].S != "fine" {
		t.Errorf("a clean argument was changed: %q", p.Results[0].Report.A["clean"].S)
	}
	if string(p.Bulletins.Files[0].Data) != "\x1b[31mred\x1b[0m" {
		t.Errorf("bulletin data was changed: %q", p.Bulletins.Files[0].Data)
	}
}

// What a board stores from a packet is already clean.
func TestApplyPacketStoresScrubbedNames(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	w.ApplyPacket(Packet{FromBoard: "Far BBS", Seq: 1, Date: "2026-10-01", Scores: []RemoteScore{{Empire: "A\x1b[5mB"}}})
	if len(w.RemoteBoards) != 1 || w.RemoteBoards[0].Scores[0].Empire != "A?[5mB" {
		t.Errorf("stored %+v", w.RemoteBoards)
	}
}

// A board name with a control character is refused, never rewritten: the
// origin check runs on the name as sent, so cleaning it afterwards could pass a
// packet off as one from a board whose key it was never checked against.
func TestApplyPacketRefusesControlInBoardName(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "Home"
	for _, p := range []Packet{
		{FromBoard: "What\x01BBS", Seq: 1, Date: "2026-10-01", Scores: []RemoteScore{{Empire: "Spoof"}}},
		{FromBoard: "Far BBS", ToBoard: "Home\x1b", Seq: 1, Date: "2026-10-01", Scores: []RemoteScore{{Empire: "Spoof"}}},
		{FromBoard: "Far BBS", Seq: 1, Date: "2026-10-01", LeagueNodes: []LeagueNode{{Number: 1, Name: "Bad\x1bName"}}},
		{FromBoard: "Far BBS", Seq: 1, Date: "2026-10-01", LeagueNodes: []LeagueNode{{Number: 1, Name: "Bad\nPUBKEY"}}},
		{FromBoard: "Far\nBBS", Seq: 1, Date: "2026-10-01", Scores: []RemoteScore{{Empire: "Spoof"}}},
	} {
		w := NewWorldSeed(cfg, 1)
		w.ApplyPacket(p)
		if len(w.RemoteBoards) != 0 || len(w.LeagueNodes) != 0 || len(w.LastPacketFrom) != 0 {
			t.Errorf("a packet naming %q / %q was applied", p.FromBoard, p.ToBoard)
		}
	}
}

// A realm name may not hold a control character, including the ones Unicode
// also counts as spaces: another board would store it with a "?" in its place.
func TestValidRealmNameRefusesControlSpaces(t *testing.T) {
	for _, name := range []string{"Foo\tBar", "Foo\rBar", "Foo\u0085Bar", "Foo\x1bBar"} {
		if ValidRealmName(name) {
			t.Errorf("ValidRealmName(%q) = true", name)
		}
	}
	if !ValidRealmName("Foo Bar") {
		t.Error("an ordinary space was refused")
	}
}

// scrubPacket walks the packet by reflection and reaches strings only through
// exported fields, pointers, slices and map values. A wire field added in any
// other shape - an interface, an array, an unexported field, a map keyed by
// text - would carry control characters past it unnoticed, so this fails on one
// until the walk learns that shape. Msg's argument map is the one map keyed by
// text: its keys name placeholders, are matched against the template and never
// shown.
func TestScrubPacketReachesEveryString(t *testing.T) {
	seen := map[reflect.Type]bool{}
	var walk func(typ reflect.Type, path string)
	walk = func(typ reflect.Type, path string) {
		if seen[typ] {
			return
		}
		seen[typ] = true
		switch typ.Kind() {
		case reflect.Interface, reflect.Array:
			t.Errorf("%s is a %s, which scrubPacket does not walk", path, typ.Kind())
		case reflect.Pointer, reflect.Slice:
			walk(typ.Elem(), path)
		case reflect.Map:
			if typ.Key().Kind() == reflect.String && typ != reflect.TypeFor[map[string]Arg]() {
				t.Errorf("%s is keyed by text, and scrubPacket leaves map keys alone", path)
			}
			walk(typ.Elem(), path)
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				f := typ.Field(i)
				if !f.IsExported() {
					t.Errorf("%s.%s is unexported, and scrubPacket cannot set it", path, f.Name)
					continue
				}
				walk(f.Type, path+"."+f.Name)
			}
		}
	}
	walk(reflect.TypeFor[Packet](), "Packet")
	// Proof the walk went all the way down rather than stopping early.
	for _, want := range []reflect.Type{reflect.TypeFor[Arg](), reflect.TypeFor[IPMessage](), reflect.TypeFor[LeagueNode]()} {
		if !seen[want] {
			t.Errorf("the walk never reached %s", want)
		}
	}
}
