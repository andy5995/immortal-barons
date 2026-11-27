package mod

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sync/atomic"
	"time"

	"go.starlark.net/starlark"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A script never runs in the game's own process. The game starts a copy of its
// own executable as a worker, which is this file's other half: it compiles
// scripts and runs their calls, one request at a time, and talks to the game in
// JSON lines over a pipe made for it. The operating system limits the worker's
// memory where it can, and the game kills it when a call takes too long. A
// script that hangs, runs out of memory or crashes the interpreter costs one
// failed call, and the next call starts a new worker. See workerhost.go for the
// game's side.

const (
	// workerEnv is set in a worker's environment, and only there. Every binary
	// that imports this package becomes the worker when it is set, before its
	// own main runs (init below), so no binary needs wiring and test binaries
	// work the same as the game.
	workerEnv    = "IB_SCRIPT_WORKER"
	workerMemEnv = "IB_SCRIPT_MEMORY_MB"
	// workerNoLimitEnv, set by a test, has the worker skip the system's limit.
	workerNoLimitEnv = "IB_SCRIPT_NO_SYSTEM_LIMIT"
)

// workerCPUBackstop is how many CPU seconds past the ones already used a worker
// is allowed for one request before the system kills it. The game's wall-clock
// timeout comes first; this is for a game that has died or hung.
const workerCPUBackstop = 10

// The two kinds of script, in a request's Kind.
const (
	kindPirates = "pirates"
	kindRegion  = "region"
)

// wireMsg is one line of the conversation, either way. Op says which fields
// mean anything: "compile" and "call" from the game, "rand" from the worker
// with N, "rand" from the game back with V or Err, and "result" from the worker.
type wireMsg struct {
	Op   string `json:"op"`
	ID   string `json:"id,omitempty"`
	Kind string `json:"kind,omitempty"`
	File string `json:"file,omitempty"`
	Src  string `json:"src,omitempty"`

	Options map[string]wireOption `json:"options,omitempty"`
	// PrefMod and Prefs say which mod's preferences a call may read and what
	// each one defaults to.
	PrefMod string          `json:"prefmod,omitempty"`
	Prefs   map[string]bool `json:"prefs,omitempty"`

	Pirate *wirePirate `json:"pirate,omitempty"`
	Region *wireRegion `json:"region,omitempty"`

	N   int    `json:"n,omitempty"`
	V   int    `json:"v,omitempty"`
	Err string `json:"err,omitempty"`

	Orders []game.PirateOrder `json:"orders,omitempty"`
	Row    *game.RegionRules  `json:"row,omitempty"`
}

type wireOption struct {
	Bool  bool
	Value int64
}

// wirePirate is game.PirateView without its Rand: a draw is asked of the game
// while the call runs, so the world's generator is drawn from in the same order
// a script in the game's own process would.
type wirePirate struct {
	Day      int
	Realm    game.PirateRealmView
	Factions []game.PirateFactionView
}

type wireRegion struct {
	View game.RegionView
	Row  game.RegionRules
}

// workerArg is the first argument the game starts a worker with. A binary is a
// worker only with both it and workerEnv: the variable alone, set from outside
// or inherited, must not turn the game itself into one.
const workerArg = "--ib-script-worker"

func init() {
	if os.Getenv(workerEnv) != "1" {
		return
	}
	if len(os.Args) < 2 || os.Args[1] != workerArg {
		// Not started as a worker: drop the variable so a child the game
		// starts (an OnFault command, say) does not inherit it.
		os.Unsetenv(workerEnv)
		return
	}
	in, out := os.Stdin, os.Stdout
	// Nothing but the conversation may be written to the game's pipe.
	os.Stdout = os.Stderr
	runWorker(in, out)
	os.Exit(0)
}

// workerScript is a compiled script held by the worker.
type workerScript struct {
	kind string
	file string
	fn   *starlark.Function
}

// callContext is what the builtins that talk to the game need during a call.
type callContext struct {
	day  int
	rand func(n int) (int, error)
}

var workerBusy atomic.Bool

func runWorker(in io.Reader, out io.Writer) {
	limit := limMemory
	if mb := os.Getenv(workerMemEnv); mb != "" {
		var n uint64
		if _, err := fmt.Sscan(mb, &n); err == nil && n > 0 {
			limit = n << 20
		}
	}
	if os.Getenv(workerNoLimitEnv) == "" && !raceBuild {
		workerSelfLimit(limit)
	}
	debug.SetMemoryLimit(int64(limit))
	go workerWatchdog(limit)

	dec := json.NewDecoder(bufio.NewReaderSize(in, 1<<16))
	enc := json.NewEncoder(out)
	scripts := map[string]*workerScript{}
	for {
		var m wireMsg
		if err := dec.Decode(&m); err != nil {
			return // the game closed the pipe, or went away
		}
		workerBusy.Store(true)
		workerArmCPU()
		var reply wireMsg
		switch m.Op {
		case "compile":
			reply = workerCompile(scripts, m)
		case "call":
			reply = workerCall(scripts, m, enc, dec)
		default:
			reply = wireMsg{Op: "result", Err: "unknown request " + clean(m.Op)}
		}
		workerBusy.Store(false)
		reply.Op = "result"
		if err := enc.Encode(&reply); err != nil {
			return
		}
	}
}

// workerWatchdog ends the worker when its heap passes limit, which is the
// memory limit where the system does not enforce one, and a tighter one where
// it does. The count includes garbage not yet swept, so it collects before
// deciding.
func workerWatchdog(limit uint64) {
	for {
		time.Sleep(5 * time.Millisecond)
		if !workerBusy.Load() || heapBytes() <= limit {
			continue
		}
		runtime.GC()
		if heapBytes() > limit {
			fmt.Fprintf(os.Stderr, "the script used more than %d MiB of memory\n", limit>>20)
			os.Exit(3)
		}
	}
}

// heapBytes is the bytes of heap objects, garbage not yet swept included.
func heapBytes() uint64 {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return s[0].Value.Uint64()
}

func optionsFromWire(w map[string]wireOption) *optionValues {
	v := &optionValues{defs: map[string]*Option{}, chosen: map[string]int64{}}
	for id, o := range w {
		typ := OptionNumber
		if o.Bool {
			typ = OptionBool
		}
		v.defs[id] = &Option{Type: typ}
		v.chosen[id] = o.Value
	}
	return v
}

func optionsToWire(v *optionValues) map[string]wireOption {
	out := map[string]wireOption{}
	if v == nil {
		return out
	}
	for id, o := range v.defs {
		out[id] = wireOption{Bool: o.Type == OptionBool, Value: v.chosen[id]}
	}
	return out
}

func prefsFromWire(mod string, w map[string]bool) *prefValues {
	v := &prefValues{mod: mod, defs: map[string]*Pref{}}
	for id, d := range w {
		v.defs[id] = &Pref{Default: &d}
	}
	return v
}

func prefsToWire(v *prefValues) (string, map[string]bool) {
	if v == nil {
		return "", nil
	}
	out := map[string]bool{}
	for id, p := range v.defs {
		out[id] = *p.Default
	}
	return v.mod, out
}

func workerCompile(scripts map[string]*workerScript, m wireMsg) (reply wireMsg) {
	defer func() {
		if p := recover(); p != nil {
			reply = wireMsg{Err: fmt.Sprintf("the script crashed the interpreter: %v", p)}
		}
	}()
	options := optionsFromWire(m.Options)
	var fn *starlark.Function
	var err error
	switch m.Kind {
	case kindPirates:
		fn, err = compilePirates(m.File, m.Src, options)
	case kindRegion:
		fn, err = compileRegion(m.File, m.Src, options)
	default:
		err = fmt.Errorf("unknown kind of script %q", clean(m.Kind))
	}
	if err != nil {
		return wireMsg{Err: err.Error()}
	}
	scripts[m.ID] = &workerScript{kind: m.Kind, file: m.File, fn: fn}
	return wireMsg{}
}

func workerCall(scripts map[string]*workerScript, m wireMsg, enc *json.Encoder, dec *json.Decoder) (reply wireMsg) {
	defer func() {
		if p := recover(); p != nil {
			reply = wireMsg{Err: fmt.Sprintf("the script crashed the interpreter: %v", p)}
		}
	}()
	s := scripts[m.ID]
	if s == nil {
		return wireMsg{Err: "the worker does not hold this script"}
	}
	th := newThread(s.file)
	prefs := prefsFromWire(m.PrefMod, m.Prefs)
	switch s.kind {
	case kindPirates:
		if m.Pirate == nil {
			return wireMsg{Err: "no view"}
		}
		v := game.PirateView{Day: m.Pirate.Day, Realm: m.Pirate.Realm, Factions: m.Pirate.Factions}
		th.SetLocal(viewKey, &callContext{day: v.Day, rand: func(n int) (int, error) {
			if err := enc.Encode(&wireMsg{Op: "rand", N: n}); err != nil {
				return 0, err
			}
			var r wireMsg
			if err := dec.Decode(&r); err != nil {
				return 0, err
			}
			if r.Err != "" {
				return 0, errors.New(r.Err)
			}
			return r.V, nil
		}})
		res, err := starlark.Call(th, s.fn, starlark.Tuple{realmValue(v, prefs), factionsValue(v)}, nil)
		if err != nil {
			return wireMsg{Err: scriptError(err).Error()}
		}
		orders, err := readOrders(res)
		if err != nil {
			return wireMsg{Err: err.Error()}
		}
		return wireMsg{Orders: orders}
	case kindRegion:
		if m.Region == nil {
			return wireMsg{Err: "no view"}
		}
		res, err := starlark.Call(th, s.fn, starlark.Tuple{regionRealmValue(m.Region.View, prefs), regionRowValue(m.Region.Row)}, nil)
		if err != nil {
			return wireMsg{Err: scriptError(err).Error()}
		}
		row, err := readRegionRow(res, m.Region.Row)
		if err != nil {
			return wireMsg{Err: err.Error()}
		}
		return wireMsg{Row: &row}
	}
	return wireMsg{Err: "unknown kind of script"}
}
