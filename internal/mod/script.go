package mod

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
	"go.starlark.net/syntax"

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

// viewKey holds the current call's game.PirateView on the thread, for the
// rand and day builtins.
const viewKey = "ib.view"

// predeclared are the names a script sees besides Starlark's own built-ins.
// Nothing here reads a clock or a file, so a script's answer depends only on
// what the game hands it.
var predeclared = starlark.StringDict{
	"rand": starlark.NewBuiltin("rand", func(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var n int
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &n); err != nil {
			return nil, err
		}
		v, ok := th.Local(viewKey).(*game.PirateView)
		if !ok {
			return nil, errors.New("rand may be called only inside raid()")
		}
		if n < 1 {
			return nil, fmt.Errorf("rand(%d): n must be at least 1", n)
		}
		return starlark.MakeInt(v.Rand(n)), nil
	}),
	"day": starlark.NewBuiltin("day", func(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
			return nil, err
		}
		v, ok := th.Local(viewKey).(*game.PirateView)
		if !ok {
			return nil, errors.New("day may be called only inside raid()")
		}
		return starlark.MakeInt(v.Day), nil
	}),
}

// pirateScript is a compiled pirates.star: the game.PirateBrain a mod adds to
// its rules.
type pirateScript struct {
	mod, hash string
	file      string // for messages: the mod folder's pirates.star
	raid      *starlark.Function
}

// newThread is a thread for one run, with the step cap set and print
// discarded: a script has no output but its answer.
func newThread(name string) *starlark.Thread {
	th := &starlark.Thread{Name: name, Print: func(*starlark.Thread, string) {}}
	th.SetMaxExecutionSteps(ScriptSteps)
	return th
}

// compilePirates runs src's top level and checks that it defines raid with two
// parameters. Starlark's defaults stay on: no while loops, no recursion, and
// no load(), since the thread has no loader.
func compilePirates(file, src string) (*starlark.Function, error) {
	globals, err := starlark.ExecFileOptions(&syntax.FileOptions{}, newThread(file), file, src, predeclared)
	var ee *starlark.EvalError
	if errors.As(err, &ee) {
		return nil, fmt.Errorf("%s: %w", file, scriptError(err))
	}
	if err != nil {
		return nil, err // a syntax error, which names the file already
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

// scriptError puts the script's line in a Starlark runtime error, which
// otherwise names only what went wrong.
func scriptError(err error) error {
	var ee *starlark.EvalError
	if errors.As(err, &ee) {
		// The innermost frame is a built-in when one raised the error, and a
		// built-in has no line; the first frame with one is the script's.
		for i := range len(ee.CallStack) {
			if line := ee.CallStack.At(i).Pos.Line; line > 0 {
				return fmt.Errorf("line %d: %s", line, ee.Msg)
			}
		}
	}
	return err
}

// Raid calls the script's raid() with v and reads its answer.
func (s *pirateScript) Raid(v game.PirateView) ([]game.PirateOrder, error) {
	th := newThread(s.file)
	th.SetLocal(viewKey, &v)
	res, err := starlark.Call(th, s.raid, starlark.Tuple{realmValue(v), factionsValue(v)}, nil)
	if err != nil {
		return nil, scriptError(err)
	}
	return readOrders(res)
}

// warnOut is where a failed script is reported; tests replace it.
var (
	warnMu  sync.Mutex
	warnOut io.Writer = os.Stderr
	warned            = map[string]bool{}
)

// Failed reports why the game raided the built-in way, once per mod per
// process: a script that fails once will usually fail every turn, and the
// sysop's log needs the reason, not a copy for every realm.
func (s *pirateScript) Failed(err error) {
	warnMu.Lock()
	defer warnMu.Unlock()
	key := s.mod + "\x00" + s.hash
	if warned[key] {
		return
	}
	warned[key] = true
	fmt.Fprintf(warnOut, "immortal-barons: mod %q: %s: %v; pirate raids use the built-in rules where it fails (reported once)\n", s.mod, PiratesFile, err)
}

func holdings(h game.PirateHoldings) starlark.StringDict {
	d := starlark.StringDict{}
	for s, n := range h {
		d[spoilName(s)] = starlark.MakeInt(n)
	}
	return d
}

// realmValue is raid()'s first argument: a read-only struct.
func realmValue(v game.PirateView) starlark.Value {
	r := v.Realm
	d := holdings(r.Units)
	d["name"] = starlark.String(r.Name)
	d["regions"] = starlark.MakeInt(r.Regions)
	d["gold"] = starlark.MakeInt64(r.Gold)
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
	orders := make([]game.PirateOrder, 0, min(list.Len(), game.ModPirateOrdersMax))
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
					return nil, fmt.Errorf("raid %d: faction: %v", i, err)
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
					return nil, fmt.Errorf("raid %d: take %s is not one of %s", i, kv[1], strings.Join(names, ", "))
				}
				o.Spoil, haveTake = spoils[k], true
			case "market":
				b, ok := kv[1].(starlark.Bool)
				if !ok {
					return nil, fmt.Errorf("raid %d: market must be True or False", i)
				}
				o.FromMarket = bool(b)
			default:
				return nil, fmt.Errorf("raid %d: unknown key %s", i, kv[0])
			}
		}
		if !haveFaction || !haveTake {
			return nil, fmt.Errorf("raid %d: needs both \"faction\" and \"take\"", i)
		}
		orders = append(orders, o)
	}
	return orders, nil
}

// newPirateScript compiles a mod's pirates.star.
func newPirateScript(path, name, hash, src string) (*pirateScript, error) {
	file := filepath.Join(path, PiratesFile)
	key := file + "\x00" + src
	if fn, ok := compiled.Load(key); ok {
		return &pirateScript{mod: name, hash: hash, file: file, raid: fn.(*starlark.Function)}, nil
	}
	fn, err := compilePirates(file, src)
	if err != nil {
		return nil, err
	}
	compiled.Store(key, fn)
	return &pirateScript{mod: name, hash: hash, file: file, raid: fn}, nil
}

// compiled holds each script compiled once per process, keyed by its file and
// source. A door loads the world, and its mods, on every screen drawn, and a
// script's top level may run up to ScriptSteps; its globals are frozen once
// run, so one compiled function is safe to share.
var compiled sync.Map
