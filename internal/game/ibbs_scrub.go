package game

import (
	"reflect"
	"strings"
	"unicode"
)

// UnsafeIdentity reports whether a packet names a board with a control
// character in it: its sender, its addressee, or a planet on the roster it
// carries. Such a packet is refused whole rather than cleaned. Board names are
// the keys the origin check, the replay ledger and every per-board record are
// looked up by, and the origin check runs on the name as sent; rewriting it
// afterwards could turn a board with no key on file into one that has a key,
// with the packet already accepted as unsigned. No real board sends one.
func (p Packet) UnsafeIdentity() bool {
	if HasControl(p.FromBoard) || HasControl(p.ToBoard) {
		return true
	}
	for _, n := range p.LeagueNodes {
		if HasControl(n.Name) {
			return true
		}
	}
	return false
}

// HasControl reports whether s holds a control character, a newline included.
// A newline in a name breaks every table that lists it, and in a roster name it
// would start a new line of ibnodes.dat, which is read one field per line.
func HasControl(s string) bool { return strings.IndexFunc(s, unicode.IsControl) >= 0 }

// ScrubText replaces every control character in s, a newline included, with
// "?". It is the rule scrubPacket applies to every packet string but a mail
// body, and the one the sysop's logs apply to every line they keep.
func ScrubText(s string) string { return scrubWith(s, unsafeRune(false)) }

func scrubWith(s string, unsafe func(rune) bool) string {
	return strings.Map(func(r rune) rune {
		if unsafe(r) {
			return '?'
		}
		return r
	}, s)
}

// unsafeRune is the control character test for one string: a multiline field,
// a message body, keeps its newlines and loses everything else.
func unsafeRune(multiline bool) func(rune) bool {
	return func(r rune) bool { return unicode.IsControl(r) && !(multiline && r == '\n') }
}

// scrubPacket replaces every control character in every other string a packet
// carries with "?". The one exception is a field tagged `scrub:"multiline"` (a
// mail body), which keeps its newlines. Realm names,
// mail, news and reports from another board reach players' screens and the
// bulletin files, and an ESC in one is a terminal escape sequence written into
// somebody else's session. C1 controls count too: U+009B is the one-character
// form of the ESC [ that opens a sequence. Bytes that are not valid UTF-8 never
// get this far, since the JSON decoder has already replaced them.
//
// It walks the packet by reflection rather than naming fields, so a string
// added to the wire format later is covered as long as it sits in an exported
// field of a concrete type; a string inside an interface value or an
// unexported field is not reached. Byte slices are left alone: bulletin file
// data is the Coordinator's own artwork, ANSI included, and signatures are not
// text. TestScrubPacketReachesEveryString fails if the wire format gains a
// shape the walk would skip.
//
// It runs after the signatures are checked, since they cover the bytes as sent.
// A Packet's slices and maps are shared with the caller's copy, so the caller's
// packet is cleaned too; forwarding and holding use the raw bytes, so a relayed
// or held packet is never the cleaned one.
func scrubPacket(p *Packet) {
	scrubValue(reflect.ValueOf(p).Elem(), false)
}

// scrubValue cleans v in place and reports whether it changed anything.
// multiline is set for the string a `scrub:"multiline"` field holds.
func scrubValue(v reflect.Value, multiline bool) bool {
	changed := false
	switch v.Kind() {
	case reflect.String:
		unsafe := unsafeRune(multiline)
		if s := v.String(); v.CanSet() && strings.IndexFunc(s, unsafe) >= 0 {
			v.SetString(scrubWith(s, unsafe))
			changed = true
		}
	case reflect.Pointer:
		if !v.IsNil() {
			changed = scrubValue(v.Elem(), false)
		}
	case reflect.Struct:
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			changed = scrubValue(v.Field(i), t.Field(i).Tag.Get("scrub") == "multiline") || changed
		}
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return false
		}
		for i := 0; i < v.Len(); i++ {
			changed = scrubValue(v.Index(i), false) || changed
		}
	case reflect.Map:
		if !v.CanSet() {
			return false // reached through an unexported field
		}
		// A map value cannot be changed in place: copy it out, scrub the copy,
		// and put it back only if it changed. Keys are left as they are; they
		// name placeholders, and renaming one would detach its value from the
		// template.
		iter := v.MapRange()
		for iter.Next() {
			e := reflect.New(iter.Value().Type()).Elem()
			e.Set(iter.Value())
			if scrubValue(e, false) {
				v.SetMapIndex(iter.Key(), e)
				changed = true
			}
		}
	}
	return changed
}
