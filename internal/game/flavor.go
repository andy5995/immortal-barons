package game

import "strings"

// flavor.go — the three failure pools: the lines a misfired missile, a
// driven-off bombing run and a caught agent are reported with. The missile and
// agent entries are PAIRS: the board where the strike lands picks one entry and
// uses both halves, the target's line and the firer's, so the two sides tell
// the same story. A bombing run files no firer event, so its pool holds the
// target planet's line alone. The firer's own planet news cannot follow the
// pick (it is written from the returned outcome alone), so it keeps one plain
// line per outcome.
//
// The placeholders are filled by fill: {missile}, {target}, {board}, {from},
// {who}.

// missileMisfire is one way an interplanetary missile fails on its own.
type missileMisfire struct {
	Yours  string // the firer's report: {missile}, {target}, {board}
	Theirs string // the target's event: {missile}, {from}
}

var missileMisfirePool = []missileMisfire{
	{"Your {missile} lost its guidance and drifted into a black hole on the way to {target} of {board}.",
		"A {missile} from {from} lost its guidance and drifted into a black hole before it reached you."},
	{"Your {missile} was hijacked by space pirates, rewired, and redirected to a player in League 8472.",
		"Space pirates hijacked a {missile} from {from} and sent it off to League 8472. Lucky you."},
	{"Your {missile} ran into a stray asteroid on the way to {target} of {board}.",
		"A {missile} from {from} ran into a stray asteroid on its way to you."},
	{"Your {missile}'s warhead failed to arm, and it sailed harmlessly past {target} of {board}.",
		"A {missile} from {from} sailed harmlessly past; its warhead never armed."},
}

// bomberDrivenOffPool is the target planet's news line for a bombing run that
// never reached what it was sent at: {from}, {target}.
var bomberDrivenOffPool = []string{
	"Bombers from {from} ran low on fuel and turned back before they reached the planet's {target}.",
	"A solar storm turned back bombers from {from} before they reached the planet's {target}.",
	"Bombers from {from} got lost in an asteroid field on their way to the planet's {target}.",
}

// agentCaught is one way an agent is caught, shared by interplanetary Terrorist
// Ops and the local covert ops.
type agentCaught struct {
	Singular string // "Your agent <Singular>." / "; one of them <Singular>."
	Plural   string // "; 3 of them <Plural>."
	// Theirs is the target's event. {who} is "3 agents sent by X of B" for
	// Terrorist Ops, "an agent behind <op>, in X's pay" for a local covert op.
	Theirs string
}

var agentCaughtPool = []agentCaught{
	{"didn't make it home", "didn't make it home",
		"Your security caught {who}."},
	{"was turned in by the locals", "were turned in by the locals",
		"Your well-paid and loyal peasants turned in {who}."},
	{"was caught and quietly disappeared", "were caught and quietly disappeared",
		"Your security quietly made {who} disappear."},
	{"walked into a bar and was never heard from again", "walked into a bar and were never heard from again",
		"Your security lured {who} into a bar with some attractive patrons."},
}

func (w *World) pickMissileMisfire() missileMisfire {
	return missileMisfirePool[w.rng.Intn(len(missileMisfirePool))]
}

func (w *World) pickBomberDrivenOff() string {
	return bomberDrivenOffPool[w.rng.Intn(len(bomberDrivenOffPool))]
}

func (w *World) pickAgentCaught() agentCaught {
	return agentCaughtPool[w.rng.Intn(len(agentCaughtPool))]
}

// fill replaces each {key} in tmpl with its value; kv alternates key, value.
func fill(tmpl string, kv ...string) string {
	pairs := make([]string, 0, len(kv))
	for i := 0; i+1 < len(kv); i += 2 {
		pairs = append(pairs, "{"+kv[i]+"}", kv[i+1])
	}
	return strings.NewReplacer(pairs...).Replace(tmpl)
}
