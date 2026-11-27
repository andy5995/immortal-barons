package mod

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/andy5995/immortal-barons/internal/game"
)

// PiratesFile is a mod's optional pirate script, written in Starlark
// (https://github.com/google/starlark-go). It must define raid(realm,
// factions); the game asks it once per realm per turn which pirate raids hit
// that realm.
const PiratesFile = "pirates.star"

// ScriptSteps is how many Starlark steps one run of a script may take, its
// top level when the mod loads or one call of raid(). A run that goes past it
// is stopped and counts as a failed run. It is set by the game, not the mod,
// and is not a gameplay number: it only keeps a runaway script from stalling
// a turn. A million steps take well under a second.
const ScriptSteps = 1_000_000

// spoilName is what a script calls s, as a holding and as a "take": the
// spoil's own label, lowercased ("jets", "gold").
func spoilName(s game.PirateSpoil) string { return strings.ToLower(s.Label()) }

// viewKey holds the current call's callContext on the thread, for the rand and
// day builtins.
const viewKey = "ib.view"

// predeclared are the names a script sees besides Starlark's own built-ins.
// Nothing here reads a clock or a file, so a script's answer depends only on
// what the game hands it. option() is added at compile time: its values are
// fixed for the game being loaded.
var predeclared = starlark.StringDict{
	"rand": starlark.NewBuiltin("rand", func(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var n int
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &n); err != nil {
			return nil, err
		}
		c, ok := th.Local(viewKey).(*callContext)
		if !ok {
			return nil, errors.New("rand may be called only inside raid()")
		}
		if n < 1 {
			return nil, fmt.Errorf("rand(%d): n must be at least 1", n)
		}
		r, err := c.rand(n)
		if err != nil {
			return nil, fmt.Errorf("rand: %v", err)
		}
		return starlark.MakeInt(r), nil
	}),
	"day": starlark.NewBuiltin("day", func(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
			return nil, err
		}
		c, ok := th.Local(viewKey).(*callContext)
		if !ok {
			return nil, errors.New("day may be called only inside raid()")
		}
		return starlark.MakeInt(c.day), nil
	}),
}

// predeclaredFor adds a mod's chosen options to the names a script sees. The
// base dict is never written to: the same process compiles mods with different
// options at the same time.
func predeclaredFor(options *optionValues) starlark.StringDict {
	out := starlark.StringDict{}
	maps.Copy(out, predeclared)
	if options != nil {
		out["option"] = options.builtin()
	}
	return out
}

// pirateScript is a compiled pirates.star: the game.PirateBrain a mod adds to
// its rules.
type pirateScript struct {
	mod, hash string
	file      string // for messages: the mod folder's pirates.star
	prefs     *prefValues
	ref       *scriptRef
}

// newThread is a thread for one run, with the step cap set and print
// discarded: a script has no output but its answer.
func newThread(name string) *starlark.Thread {
	th := &starlark.Thread{Name: name, Print: func(*starlark.Thread, string) {}}
	th.SetMaxExecutionSteps(ScriptSteps)
	return th
}

// execScript runs src's top level in the worker. scriptFileOptions leaves
// Starlark's defaults as they are: no while loops, no recursion, and no load(),
// since the thread has no loader.
func execScript(file, src string, pre starlark.StringDict) (starlark.StringDict, error) {
	return starlark.ExecFileOptions(scriptFileOptions, newThread(file), file, src, pre)
}

// compilePirates runs src's top level and checks that it defines raid with two
// parameters. It runs in the worker.
func compilePirates(file, src string, options *optionValues) (*starlark.Function, error) {
	globals, err := execScript(file, src, predeclaredFor(options))
	if err != nil {
		return nil, scriptFailure(file, err)
	}
	fn, ok := globals["raid"].(*starlark.Function)
	if !ok {
		return nil, fmt.Errorf("%s: it must define raid(realm, factions)", file)
	}
	if fn.NumParams() != 2 || fn.HasVarargs() || fn.HasKwargs() || fn.NumKwonlyParams() != 0 {
		return nil, fmt.Errorf("%s: raid must take exactly two parameters, raid(realm, factions)", file)
	}
	return fn, nil
}

// scriptFailure is err from compiling file, with the file's name on a runtime
// error. A syntax error names the file already.
func scriptFailure(file string, err error) error {
	var ee *starlark.EvalError
	if errors.As(err, &ee) {
		return fmt.Errorf("%s: %w", file, scriptError(err))
	}
	return errors.New(clean(err.Error()))
}

// scriptError puts the script's line in a Starlark runtime error, which
// otherwise names only what went wrong. The text is cleaned: it can hold a
// string the script chose, such as a fail() message, and it is printed on the
// sysop's terminal and written to a log.
func scriptError(err error) error {
	var ee *starlark.EvalError
	if errors.As(err, &ee) {
		// The innermost frame is a built-in when one raised the error, and a
		// built-in has no line; the first frame with one is the script's.
		for i := range len(ee.CallStack) {
			if line := ee.CallStack.At(i).Pos.Line; line > 0 {
				return fmt.Errorf("line %d: %s", line, clean(ee.Msg))
			}
		}
		return errors.New(clean(ee.Msg))
	}
	return errors.New(clean(err.Error()))
}

// Raid asks the script's raid() about v, in a worker, and reads its answer.
func (s *pirateScript) Raid(v game.PirateView) ([]game.PirateOrder, error) {
	call := wireMsg{
		Op: "call", ID: s.ref.id,
		Pirate: &wirePirate{Day: v.Day, Realm: v.Realm, Factions: v.Factions},
	}
	call.PrefMod, call.Prefs = prefsToWire(s.prefs)
	res, err := callWorker(s.ref, call, func(n int) (int, error) { return v.Rand(n), nil })
	if err != nil {
		return nil, err
	}
	return res.Orders, nil
}

// ScriptWarnings is how many different reasons one script's failures are
// reported for in a run. Like ScriptSteps it is not a gameplay number: it
// keeps a script that fails a new way for every realm from filling the log.
const ScriptWarnings = 5

// warnOut is where a failed script is reported; tests replace it. warned holds,
// for each script, the reasons already reported.
var (
	warnMu  sync.Mutex
	warnOut io.Writer = os.Stderr
	warned            = map[string]map[string]bool{}
)

// SetWarnOutput sends what the mods report while a game runs, a failed script
// or a mod changed since the reset, to w instead of standard error.
func SetWarnOutput(w io.Writer) {
	warnMu.Lock()
	defer warnMu.Unlock()
	warnOut = w
}

// ResetWarnings forgets which script failures were already reported, so a new
// run in the same process, such as a second simulation, reports them again.
func ResetWarnings() {
	warnMu.Lock()
	defer warnMu.Unlock()
	warned = map[string]map[string]bool{}
}

// Warn reports msg about the mods in play where SetWarnOutput says.
func Warn(msg string) {
	warnMu.Lock()
	defer warnMu.Unlock()
	fmt.Fprintln(warnOut, "immortal-barons: "+clean(msg))
}

// warnScript reports that script file of mod failed with err, and what the
// game did instead. Each reason is reported once per run: a script that fails
// one way usually fails that way every turn, and the sysop's log needs the
// reason, not a copy for every realm. A script that fails only for some realms
// can fail a second way later, so a new reason is reported too, up to
// ScriptWarnings of them.
func warnScript(id, mod, file string, err error, instead string) {
	warnMu.Lock()
	defer warnMu.Unlock()
	seen := warned[id]
	if seen == nil {
		seen = map[string]bool{}
		warned[id] = seen
	}
	err = errors.New(clean(err.Error()))
	reason := err.Error()
	switch {
	case seen[reason] || len(seen) > ScriptWarnings:
		return
	case len(seen) == ScriptWarnings:
		seen[reason] = true
		fmt.Fprintf(warnOut, "immortal-barons: mod %q: %s has failed in more than %d ways; no more are reported\n", mod, file, ScriptWarnings)
		return
	}
	seen[reason] = true
	fmt.Fprintf(warnOut, "immortal-barons: mod %q: %s: %v; %s\n", mod, file, err, instead)
}

// Failed reports why the game raided the built-in way (warnScript).
func (s *pirateScript) Failed(err error) {
	warnScript(s.mod+"\x00"+s.hash+"\x00"+PiratesFile, s.mod, PiratesFile, err, "pirate raids use the built-in rules where it fails")
}

func holdings(h game.PirateHoldings) starlark.StringDict {
	d := starlark.StringDict{}
	for s, n := range h {
		d[spoilName(s)] = starlark.MakeInt(n)
	}
	return d
}

// realmValue is raid()'s first argument: a read-only struct.
func realmValue(v game.PirateView, prefs *prefValues) starlark.Value {
	r := v.Realm
	d := holdings(r.Units)
	d["name"] = starlark.String(r.Name)
	d["regions"] = starlark.MakeInt(r.Regions)
	d["gold"] = starlark.MakeInt64(r.Gold)
	d["pref"] = prefs.builtin(r.ModPrefs)
	d["market"] = starlarkstruct.FromStringDict(starlarkstruct.Default, holdings(r.Market))
	return starlarkstruct.FromStringDict(starlarkstruct.Default, d)
}

// factionsValue is raid()'s second argument: a frozen list of structs, one
// per faction in slot order.
func factionsValue(v game.PirateView) starlark.Value {
	out := make([]starlark.Value, len(v.Factions))
	for i, f := range v.Factions {
		d := holdings(f.Loot)
		d["slot"] = starlark.MakeInt(f.Slot)
		d["name"] = starlark.String(f.Name)
		d["regions"] = starlark.MakeInt(f.Regions)
		d["gold"] = starlark.MakeInt64(f.Gold)
		out[i] = starlarkstruct.FromStringDict(starlarkstruct.Default, d)
	}
	l := starlark.NewList(out)
	l.Freeze()
	return l
}

// readOrders reads raid()'s answer: a list of dicts such as
// {"faction": 0, "take": "jets", "market": False}. "market" may be left out.
// Whether each order is one the game allows is game.askPirateBrain's check.
func readOrders(res starlark.Value) ([]game.PirateOrder, error) {
	list, ok := res.(*starlark.List)
	if !ok {
		return nil, fmt.Errorf("raid returned a %s; it must return a list", res.Type())
	}
	// game.askPirateBrain holds the count too, but only after every order is
	// read; refusing a long list here spares reading it.
	if list.Len() > game.ModPirateOrdersMax {
		return nil, fmt.Errorf("%d raids ordered; at most %d are allowed", list.Len(), game.ModPirateOrdersMax)
	}
	orders := make([]game.PirateOrder, 0, list.Len())
	for i := range list.Len() {
		d, ok := list.Index(i).(*starlark.Dict)
		if !ok {
			return nil, fmt.Errorf("raid %d: a %s, not a dict", i, list.Index(i).Type())
		}
		var o game.PirateOrder
		var haveFaction, haveTake bool
		for _, kv := range d.Items() {
			key, _ := starlark.AsString(kv[0])
			switch key {
			case "faction":
				n, err := starlark.AsInt32(kv[1])
				if err != nil {
					return nil, fmt.Errorf("raid %d: faction: %s", i, clean(truncate(err.Error(), 100)))
				}
				o.Slot, haveFaction = n, true
			case "take":
				name, _ := starlark.AsString(kv[1])
				spoils := game.PirateSpoils()
				k := slices.IndexFunc(spoils, func(s game.PirateSpoil) bool { return spoilName(s) == name })
				if k < 0 {
					var names []string
					for _, s := range spoils {
						names = append(names, spoilName(s))
					}
					return nil, fmt.Errorf("raid %d: take %s is not one of %s", i, show(kv[1]), strings.Join(names, ", "))
				}
				o.Spoil, haveTake = spoils[k], true
			case "market":
				b, ok := kv[1].(starlark.Bool)
				if !ok {
					return nil, fmt.Errorf("raid %d: market must be True or False", i)
				}
				o.FromMarket = bool(b)
			default:
				return nil, fmt.Errorf("raid %d: unknown key %s", i, show(kv[0]))
			}
		}
		if !haveFaction || !haveTake {
			return nil, fmt.Errorf("raid %d: needs both \"faction\" and \"take\"", i)
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// newPirateScript checks a mod's pirates.star compiles with its chosen options,
// once per process for a given file, source and option values (see validated).
func newPirateScript(path, name, hash, src string, options *optionValues) (*pirateScript, error) {
	file := filepath.Join(path, PiratesFile)
	ref := newScriptRef(kindPirates, file, src, options)
	if err := ref.validate(); err != nil {
		return nil, err
	}
	return &pirateScript{mod: name, hash: hash, file: file, ref: ref}, nil
}
