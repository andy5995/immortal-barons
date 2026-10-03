package game

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/numfmt"
)

// Msg is a sentence the engine writes for a player — an event on the recap, a
// report coming home from another planet — kept as its English template and the
// values that fill it, rather than as finished text. It is put into words when
// it is SHOWN, in the reader's language, which a finished English sentence
// cannot be (#297): the engine files an event long before anyone reads it, and
// an interplanetary report is written on a board that does not know who will.
//
// The template is the gettext msgid, so the catalogs translate it like any
// other UI string. Placeholders are named, {who} rather than %s, so a
// translation may put them in any order. A template that no catalog knows —
// one reworded since the event was filed, or one sent by a newer board —
// renders in English from the template it carries, so nothing is ever lost,
// only untranslated.
//
// The field names are short because every event in every save carries them.
type Msg struct {
	T string `json:"t,omitempty"`
	// P is the English plural of T, and N names the argument whose count picks
	// between them; empty for a message that has one form.
	P string         `json:"p,omitempty"`
	N string         `json:"n,omitempty"`
	A map[string]Arg `json:"a,omitempty"`
	// J is a message made of whole sentences shown one after another, for a
	// report with several parts; T is empty then. Joining sentences rather
	// than gluing clauses is what lets each one be translated on its own.
	J []Msg `json:"j,omitempty"`
	// R is a message made of lines, one under another: a battle report, a
	// list of treaties. T is empty then.
	R []Msg `json:"r,omitempty"`
	// I indents every line of the message by that many spaces, so a layout's
	// indent never sits inside a template for a translator to lose.
	I int `json:"i,omitempty"`
}

// Arg is one value in a Msg. Exactly one of S, N, W, M or L is set.
type Arg struct {
	// S is shown as written: a realm, a board, a handle. Never translated.
	S string `json:"s,omitempty"`
	// N is a figure, printed as F says: plain digits, "comma" for grouped in
	// the reader's locale (numfmt.Format), "short" for numfmt.Short, or "pct"
	// for a percentage, its sign included.
	N *int64 `json:"n,omitempty"`
	F string `json:"f,omitempty"`
	// M is a phrase translated in its own right: a unit's name, an operation.
	M *Msg `json:"m,omitempty"`
	// L is a list of phrases, joined in the reader's language ("a, b, and c").
	L []Msg `json:"l,omitempty"`
	// W is a moment, as Unix seconds, shown as a stamp in the reader's zone.
	W *int64 `json:"w,omitempty"`
	// Empty marks an L that is present but holds nothing, which renders as the
	// word for nothing rather than as a blank.
	Empty bool `json:"e,omitempty"`
}

// msgid marks a template that lives in a table rather than in a say call, so
// scripts/gen-ui-pot.py extracts it for the catalogs; it returns s unchanged.
func msgid(s string) string { return s }

// forms is a counted template's English singular and plural, held in a table.
type forms struct{ one, many string }

// msgidN is msgid for a counted pair, which the catalogs hold as ONE entry
// with a msgid_plural, so a language can give it more forms than two.
func msgidN(one, many string) forms { return forms{one, many} }

// sayIn is sayN for a pair held in a table.
func sayIn(f forms, count string, kv ...any) Msg { return sayN(f.one, f.many, count, kv...) }

// say builds a Msg from its English template and key/value pairs. A value may
// be a string (shown as written), an int or int64 (plain digits), an Arg built
// by comma or short, a Msg, or a []Msg list. Anything else is a programming
// error and panics, so it fails in the first test that files the message.
func say(t string, kv ...any) Msg {
	m := Msg{T: t}
	if len(kv)%2 != 0 {
		panic("say: odd key/value list for " + t)
	}
	for i := 0; i < len(kv); i += 2 {
		if m.A == nil {
			m.A = map[string]Arg{}
		}
		m.A[kv[i].(string)] = toArg(kv[i+1])
	}
	return m
}

// sayN is say for a message whose wording follows a count: one and many are
// the English singular and plural, and count is the key whose figure picks
// between them in the reader's language.
func sayN(one, many, count string, kv ...any) Msg {
	m := say(one, kv...)
	m.P, m.N = many, count
	if a, ok := m.A[count]; !ok || a.N == nil {
		panic("sayN: " + count + " is not a figure in " + one)
	}
	return m
}

func toArg(v any) Arg {
	switch v := v.(type) {
	case string:
		return Arg{S: v}
	case int:
		n := int64(v)
		return Arg{N: &n}
	case int64:
		return Arg{N: &v}
	case Arg:
		return v
	case Msg:
		return Arg{M: &v}
	case []Msg:
		return Arg{L: v, Empty: len(v) == 0}
	}
	panic(fmt.Sprintf("say: unsupported value %T", v))
}

// comma is a figure grouped in the reader's locale: 1,234,567.
func comma[T numfmt.Number](n T) Arg {
	v := int64(n)
	return Arg{N: &v, F: "comma"}
}

// moment is a time, shown as a stamp (Stamp) in the reader's own zone.
func moment(t time.Time) Arg {
	v := t.Unix()
	return Arg{W: &v}
}

// indent is m with every line indented by n spaces.
func indent(m Msg, n int) Msg {
	if m.IsZero() {
		return m
	}
	m.I = n
	return m
}

// percent is a percentage, printed with its sign. The sign travels with the
// figure rather than in the template, so no template carries a bare "%" for
// printf-minded tools to read as a verb.
func percent(n int) Arg {
	v := int64(n)
	return Arg{N: &v, F: "pct"}
}

// short is a figure in the original's shortened form: 1000k, 188m.
func short[T numfmt.Number](n T) Arg {
	v := int64(n)
	return Arg{N: &v, F: "short"}
}

// sentences joins whole sentences into one message, leaving out empty ones.
func sentences(parts ...Msg) Msg {
	var j []Msg
	for _, p := range parts {
		if !p.IsZero() {
			j = append(j, p)
		}
	}
	if len(j) == 1 {
		return j[0]
	}
	return Msg{J: j}
}

// IsZero reports whether m says nothing at all.
func (m Msg) IsZero() bool { return m.T == "" && len(m.J) == 0 && len(m.R) == 0 }

// lines stacks messages one under another, leaving out empty ones.
func lines(parts ...Msg) Msg {
	var r []Msg
	for _, p := range parts {
		if !p.IsZero() {
			r = append(r, p)
		}
	}
	if len(r) == 1 {
		return r[0]
	}
	return Msg{R: r}
}

// optMsg is m as an optional field: nil when it says nothing.
func optMsg(m Msg) *Msg {
	if m.IsZero() {
		return nil
	}
	return &m
}

// English is the message in English, as world.json and the tests read it.
func (m Msg) English() string { return m.In("") }

// String is English, so a Msg prints readably in a log or a test failure.
func (m Msg) String() string { return m.English() }

// In renders the message in lang ("" for English), with any time in UTC.
func (m Msg) In(lang string) string { return m.Render(lang, nil) }

// Render renders the message in lang, with any time (an Arg made by moment) in
// loc, the reader's zone; nil is UTC.
func (m Msg) Render(lang string, loc *time.Location) string {
	out := m.render(lang, loc)
	if m.I > 0 {
		pad := strings.Repeat(" ", m.I)
		out = pad + strings.ReplaceAll(out, "\n", "\n"+pad)
	}
	return out
}

func (m Msg) render(lang string, loc *time.Location) string {
	if len(m.J) > 0 {
		return joinIn(lang, loc, m.J, " ")
	}
	if len(m.R) > 0 {
		return joinIn(lang, loc, m.R, "\n")
	}
	t := i18n.T(lang, m.T)
	if m.P != "" {
		var n int64
		if a := m.A[m.N]; a.N != nil {
			n = *a.N
		}
		t = i18n.TN(lang, m.T, m.P, n)
	}
	if len(m.A) == 0 {
		return t
	}
	pairs := make([]string, 0, 2*len(m.A))
	for k, a := range m.A {
		pairs = append(pairs, "{"+k+"}", a.in(lang, loc))
	}
	return strings.NewReplacer(pairs...).Replace(t)
}

func joinIn(lang string, loc *time.Location, ms []Msg, sep string) string {
	parts := make([]string, len(ms))
	for i, p := range ms {
		parts[i] = p.Render(lang, loc)
	}
	return strings.Join(parts, sep)
}

func (a Arg) in(lang string, loc *time.Location) string {
	switch {
	case a.W != nil:
		return Stamp(time.Unix(*a.W, 0), loc)
	case a.N != nil:
		switch a.F {
		case "comma":
			return numfmt.Format(*a.N, lang)
		case "short":
			return numfmt.Short(*a.N)
		case "pct":
			return strconv.FormatInt(*a.N, 10) + "%"
		}
		return strconv.FormatInt(*a.N, 10)
	case a.M != nil:
		return a.M.Render(lang, loc)
	case len(a.L) > 0 || a.Empty:
		return listIn(lang, loc, a.L)
	}
	return a.S
}

// listIn joins phrases the way the original's tallies read in English — "a and
// b", "a, b, and c", "nothing" for none — with each joint a template of its
// own, so a language can join them its own way.
func listIn(lang string, loc *time.Location, l []Msg) string {
	switch len(l) {
	case 0:
		return i18n.T(lang, msgid("nothing"))
	case 1:
		return l[0].Render(lang, loc)
	}
	parts := make([]string, len(l))
	for i, m := range l {
		parts[i] = m.Render(lang, loc)
	}
	if len(parts) == 2 {
		return strings.NewReplacer("{a}", parts[0], "{b}", parts[1]).
			Replace(i18n.T(lang, msgid("{a} and {b}")))
	}
	head := strings.Join(parts[:len(parts)-1], ", ")
	return strings.NewReplacer("{list}", head, "{last}", parts[len(parts)-1]).
		Replace(i18n.T(lang, msgid("{list}, and {last}")))
}

// UnmarshalJSON reads a result saved before Protocol 4, whose report was
// English text under "Report". A board's own Outbox keeps replies across an
// upgrade (a league freeze holds them there by design), so an old world.json
// still carries them; read that way, the text survives, shown in English as it
// was written. Saving writes only ReportMsg.
func (r *AttackResult) UnmarshalJSON(b []byte) error {
	type plain AttackResult
	var aux struct {
		plain
		Legacy string `json:"Report"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*r = AttackResult(aux.plain)
	if r.Report == nil && aux.Legacy != "" {
		r.Report = &Msg{T: aux.Legacy}
	}
	return nil
}

// UnmarshalJSON reads a bid's answer saved before Protocol 4, whose reason was
// English text under "Reason"; see AttackResult.UnmarshalJSON.
func (f *IPTradeFill) UnmarshalJSON(b []byte) error {
	type plain IPTradeFill
	var aux struct {
		plain
		Legacy string `json:"Reason"`
	}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*f = IPTradeFill(aux.plain)
	if f.Reason == nil && aux.Legacy != "" {
		f.Reason = &Msg{T: aux.Legacy}
	}
	return nil
}
