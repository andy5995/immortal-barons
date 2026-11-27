package mod

import (
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"runtime/metrics"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// The limits on a running script. None is a gameplay number: they keep a
// hostile or buggy script from taking the game process down, so they live
// here with ScriptSteps and not in balance*.go. Starlark has no file, network
// or clock access and IB gives a script none; these cover CPU and memory,
// which Starlark does not limit by itself. They are best effort: see "Scripts"
// in docs/dev/mods-design.md for what they cannot stop.
const (
	// ScriptTimeout is the wall-clock time one run of a script may take. The
	// step cap does not bound a built-in that loops in Go, so this is checked
	// too. It is looked at between Starlark steps: one built-in call that
	// never returns is not interrupted.
	ScriptTimeout = 2 * time.Second

	// ScriptMemory is how far the process heap may grow while a script runs
	// before the run is cancelled. It is sampled every scriptSample, so a
	// single allocation larger than this is not stopped in time.
	ScriptMemory = 256 << 20

	// ScriptSequenceMax is the longest range(), repeated string, list or tuple
	// (x * n) and formatted string a script may build in one operation.
	// Starlark's own limit on a repeat is a gigabyte of elements, far past what
	// a door should allow; ScriptSteps already caps a loop at the same size.
	ScriptSequenceMax = 1_000_000

	// ScriptIntBits is the longest integer a product may give. Starlark's
	// integers have no limit, and x * x doubles an integer's length each step.
	ScriptIntBits = 4096

	// MaxModFileSize is the largest file a mod may hold, and MaxModFiles the
	// most files. They apply to a mod read from a folder, one saved in a
	// world, and one carried in a league's reset order.
	MaxModFileSize = 256 << 10
	MaxModFiles    = 64
)

// scriptSample is how often the memory watchdog looks at the heap.
const scriptSample = 5 * time.Millisecond

// The limits as the code reads them, so a test can lower one.
var (
	limTimeout = ScriptTimeout
	limMemory  = uint64(ScriptMemory)
	limSeq     = ScriptSequenceMax
	limIntBits = ScriptIntBits
)

// errScriptCrash marks an error out of a recovered panic.
var errScriptCrash = errors.New("the script crashed the interpreter")

// errScriptHalted is what a script run returns once an earlier run was
// abandoned: see runScript.
var errScriptHalted = errors.New("scripts are switched off for the rest of this run: an earlier script did not stop when told to")

// scriptGrace is how long a cancelled run has to stop before runScript gives
// up on it, and halted is set once it has.
var (
	limGrace = 500 * time.Millisecond
	halted   atomic.Bool
)

// runScript runs f, which calls into Starlark on th, under the wall-clock and
// memory limits, and turns a Go panic into an error. Every place the game
// calls into a script goes through it.
//
// f runs on its own goroutine. A run past ScriptTimeout is cancelled, and the
// interpreter stops at its next step; one stuck inside a single built-in
// call cannot be told to stop. After limGrace runScript gives up waiting,
// leaves that goroutine behind and returns an error, so a stuck script costs
// the game a failed run instead of the whole process. The goroutine keeps
// the CPU it was using, and no script runs again in this process: halted is
// set, and every later run fails at once, which the callers already treat as a
// failed script.
func runScript(th *starlark.Thread, f func() error) error {
	if halted.Load() {
		return errScriptHalted
	}
	r := watch.add(th)
	defer watch.remove(r)
	done := make(chan error, 1)
	go func() {
		defer func() {
			if p := recover(); p != nil {
				done <- fmt.Errorf("%w: %v", errScriptCrash, p)
			}
		}()
		done <- f()
	}()
	timeout, grace := limTimeout, limGrace
	cancel := time.AfterFunc(timeout, func() {
		th.Cancel(fmt.Sprintf("the script ran for more than %v", timeout))
	})
	defer cancel.Stop()
	giveUp := time.NewTimer(timeout + grace)
	defer giveUp.Stop()
	select {
	case err := <-done:
		return err
	case <-giveUp.C:
		halted.Store(true)
		return fmt.Errorf("the script did not stop after %v and was abandoned; %w", timeout+grace, errScriptHalted)
	}
}

// A watchdog samples the heap while any script runs. One goroutine serves
// every run in the process, started by the first and ended with the last. The
// heap is the whole process's, so growth is not the script's alone: when two
// runs overlap, or the game allocates for another reason, a run may be
// cancelled for memory it did not use. The limit is large enough that this is
// rare, and a cancelled run is an ordinary failed script.
type watchdog struct {
	mu      sync.Mutex
	runs    map[*scriptRun]struct{}
	sampler bool
}

type scriptRun struct {
	th    *starlark.Thread
	base  uint64
	limit uint64
}

var watch = &watchdog{runs: map[*scriptRun]struct{}{}}

// heapBytes is the bytes of heap objects, garbage not yet swept included.
func heapBytes() uint64 {
	s := []metrics.Sample{{Name: "/memory/classes/heap/objects:bytes"}}
	metrics.Read(s)
	if s[0].Value.Kind() != metrics.KindUint64 {
		return 0
	}
	return s[0].Value.Uint64()
}

func (w *watchdog) add(th *starlark.Thread) *scriptRun {
	r := &scriptRun{th: th, base: heapBytes(), limit: limMemory}
	w.mu.Lock()
	w.runs[r] = struct{}{}
	if !w.sampler {
		w.sampler = true
		go w.sample()
	}
	w.mu.Unlock()
	return r
}

func (w *watchdog) remove(r *scriptRun) {
	w.mu.Lock()
	delete(w.runs, r)
	w.mu.Unlock()
}

// over lists the runs whose heap growth is past the limit.
func (w *watchdog) over(heap uint64) []*scriptRun {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []*scriptRun
	for r := range w.runs {
		if heap > r.base+r.limit {
			out = append(out, r)
		}
	}
	return out
}

func (w *watchdog) sample() {
	tick := time.NewTicker(scriptSample)
	defer tick.Stop()
	for range tick.C {
		w.mu.Lock()
		if len(w.runs) == 0 {
			w.sampler = false
			w.mu.Unlock()
			return
		}
		w.mu.Unlock()
		if len(w.over(heapBytes())) == 0 {
			continue
		}
		// The count includes garbage the collector has not swept yet, which is
		// not the script's to answer for. Collect before cancelling anything.
		runtime.GC()
		for _, r := range w.over(heapBytes()) {
			r.th.Cancel(fmt.Sprintf("the script used more than %d MiB of memory", r.limit>>20))
		}
	}
}

// clean makes s safe to print on a terminal or write to a log: a control
// character, and a character that reorders or hides text (Unicode format
// characters), becomes a \u escape. Anything a script or a mod file chose
// goes through it before it is shown.
func clean(s string) string {
	if !strings.ContainsFunc(s, unsafeRune) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if unsafeRune(r) {
			fmt.Fprintf(&b, "\\u%04x", r)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Clean is clean for a screen outside this package that shows what a mod said.
func Clean(s string) string { return clean(s) }

func unsafeRune(r rune) bool { return unicode.IsControl(r) || unicode.Is(unicode.Cf, r) }

// hasUnsafe reports whether s holds a character clean would escape.
func hasUnsafe(s string) bool { return strings.ContainsFunc(s, unsafeRune) }

// scriptFileOptions is what every script is parsed with: Starlark's defaults,
// all off. There is no while loop, no recursion, no set(), no reassigning a
// top-level name, no if or for at the top level, and load() has nothing to
// load from.
var scriptFileOptions = &syntax.FileOptions{}

// Names the guard puts in a script's place. A script that uses one itself is
// refused, so it cannot stand in for the guard.
const (
	guardMul = "__ib_mul"
	guardMod = "__ib_mod"
	// guardCall stands in for x.join(...), x.replace(...) and x.format(...).
	guardCall = "__ib_call"
)

// bannedAttrs are string methods whose result can be many times the size of
// what they are given, in one call that nothing can interrupt: replace and
// format repeat a piece of their argument, join repeats its separator. A call
// x.join(...) is rewritten into guardCall, which checks the size first. The
// method used any other way, as a value (f = x.join) or through getattr, is
// refused, since nothing then stands between it and its call.
var bannedAttrs = map[string]bool{"replace": true, "join": true, "format": true}

// compileScript parses src and runs its top level under the limits, after the
// guard has rewritten the operators that can build something huge in one step
// into calls that check its size. pre is the script's predeclared names.
func compileScript(file, src string, pre starlark.StringDict) (starlark.StringDict, error) {
	f, err := scriptFileOptions.Parse(file, src, 0)
	if err != nil {
		return nil, err
	}
	if err := guard(f); err != nil {
		return nil, err
	}
	pre = guarded(pre)
	prog, err := starlark.FileProgram(f, pre.Has)
	if err != nil {
		return nil, err
	}
	var globals starlark.StringDict
	th := newThread(file)
	err = runScript(th, func() error {
		var err error
		globals, err = prog.Init(th, pre)
		return err
	})
	globals.Freeze()
	return globals, err
}

// guarded is pre with the checked builtins added, and the ones of Starlark's
// own that can build too much replaced. The base dict is never written to.
func guarded(pre starlark.StringDict) starlark.StringDict {
	out := starlark.StringDict{}
	for k, v := range pre {
		out[k] = v
	}
	out["range"] = starlark.NewBuiltin("range", rangeChecked)
	out["getattr"] = starlark.NewBuiltin("getattr", getattrChecked)
	out["str"] = starlark.NewBuiltin("str", strChecked)
	out["repr"] = starlark.NewBuiltin("repr", strChecked)
	out["fail"] = starlark.NewBuiltin("fail", failChecked)
	out["print"] = starlark.NewBuiltin("print", printDiscarded)
	out[guardMul] = starlark.NewBuiltin(guardMul, mulChecked)
	out[guardMod] = starlark.NewBuiltin(guardMod, modChecked)
	out[guardCall] = starlark.NewBuiltin(guardCall, methodChecked)
	return out
}

// rangeChecked is range, refusing a length past ScriptSequenceMax. Starlark's
// range is lazy, but list(range(n)), sorted and max walk it in Go where no step
// cap reaches.
func rangeChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var start, stop, step int
	step = 1
	if len(kwargs) > 0 {
		return nil, fmt.Errorf("range: unexpected keyword arguments")
	}
	if err := starlark.UnpackPositionalArgs("range", args, nil, 1, &start, &stop, &step); err != nil {
		return nil, err
	}
	if len(args) == 1 {
		start, stop = 0, start
	}
	if step == 0 {
		return nil, fmt.Errorf("range: step argument must not be zero")
	}
	// Work in float64 so a huge stop - start cannot wrap.
	n := (float64(stop) - float64(start)) / float64(step)
	if n > float64(limSeq) {
		return nil, fmt.Errorf("range: more than %d numbers", limSeq)
	}
	// The length is small, so the builtin is Starlark's own.
	return starlark.Call(th, rangeBuiltin, args, nil)
}

// rangeBuiltin is Starlark's own range, kept for rangeChecked to call after
// its check.
var rangeBuiltin = starlark.Universe["range"]

// printDiscarded is print: a script has no output, so it formats nothing.
func printDiscarded(*starlark.Thread, *starlark.Builtin, starlark.Tuple, []starlark.Tuple) (starlark.Value, error) {
	return starlark.None, nil
}

// expandedSize counts the nodes of v as str would print them, stopping once
// the count passes budget. A list holding the same list a thousand times, three
// levels deep, is a few kilobytes in memory and a billion elements printed.
func expandedSize(v starlark.Value, budget *int) {
	*budget--
	if *budget < 0 {
		return
	}
	switch v := v.(type) {
	case starlark.String:
		*budget -= len(v)
	case starlark.Bytes:
		*budget -= len(v)
	case *starlark.List:
		for i := 0; i < v.Len() && *budget >= 0; i++ {
			expandedSize(v.Index(i), budget)
		}
	case starlark.Tuple:
		for _, e := range v {
			if *budget < 0 {
				return
			}
			expandedSize(e, budget)
		}
	case *starlark.Dict:
		for _, kv := range v.Items() {
			if *budget < 0 {
				return
			}
			expandedSize(kv[0], budget)
			expandedSize(kv[1], budget)
		}
	}
}

func checkPrintable(name string, args starlark.Tuple) error {
	budget := limSeq
	for _, a := range args {
		expandedSize(a, &budget)
	}
	if budget < 0 {
		return fmt.Errorf("%s: the value would print as more than %d characters", name, limSeq)
	}
	return nil
}

// strChecked is str and repr, refusing a value whose text would pass
// ScriptSequenceMax.
func strChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := checkPrintable(b.Name(), args); err != nil {
		return nil, err
	}
	return starlark.Call(th, starlark.Universe[b.Name()], args, kwargs)
}

func failChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := checkPrintable(b.Name(), args); err != nil {
		return nil, err
	}
	return starlark.Call(th, starlark.Universe["fail"], args, kwargs)
}

func getattrChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) >= 2 {
		if name, ok := starlark.AsString(args[1]); ok && bannedAttrs[name] {
			return nil, fmt.Errorf("getattr: %s is not available in a script", name)
		}
	}
	return starlark.Call(th, starlark.Universe["getattr"], args, kwargs)
}

// mulChecked is what a script's x * y is rewritten to: Starlark's own product,
// after refusing a repeat or a product past the limits.
func mulChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 2 || len(kwargs) != 0 {
		return nil, errors.New("internal: bad product")
	}
	x, y := args[0], args[1]
	if n, other, ok := repeatOperands(x, y); ok {
		count, err := starlark.AsInt32(n)
		// A count that does not fit an int32 is Starlark's own error.
		if err == nil && count > 0 {
			if size := seqLen(other); size > 0 && int64(size)*int64(count) > int64(limSeq) {
				return nil, fmt.Errorf("excessive repeat (%d * %d elements); the limit is %d", size, count, limSeq)
			}
		}
	}
	if xi, ok := x.(starlark.Int); ok {
		if yi, ok := y.(starlark.Int); ok && bitLen(xi)+bitLen(yi) > limIntBits {
			return nil, fmt.Errorf("integer product past %d bits", limIntBits)
		}
	}
	return starlark.Binary(syntax.STAR, x, y)
}

func bitLen(i starlark.Int) int {
	b := i.BigInt()
	return new(big.Int).Abs(b).BitLen()
}

// repeatOperands reports whether x * y repeats a sequence, and which is which.
func repeatOperands(x, y starlark.Value) (count, seq starlark.Value, ok bool) {
	if _, isInt := x.(starlark.Int); isInt && seqLen(y) >= 0 {
		return x, y, true
	}
	if _, isInt := y.(starlark.Int); isInt && seqLen(x) >= 0 {
		return y, x, true
	}
	return nil, nil, false
}

// seqLen is the length of a string, bytes, list or tuple, and -1 for any other
// value.
func seqLen(v starlark.Value) int {
	switch v := v.(type) {
	case starlark.String:
		return len(v)
	case starlark.Bytes:
		return len(v)
	case *starlark.List:
		return v.Len()
	case starlark.Tuple:
		return len(v)
	}
	return -1
}

// methodChecked is what x.join(...), x.replace(...) and x.format(...) are
// rewritten to: guardCall("join", x, ...). When x is a string, the size of the
// result is worked out from the arguments and refused past ScriptSequenceMax;
// either way the method is then called as written, so what it does is
// Starlark's. For any other x, such as a struct with a field named join, it is
// an ordinary attribute call.
func methodChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 2 {
		return nil, errors.New("internal: bad method call")
	}
	name, _ := starlark.AsString(args[0])
	recv, rest := args[1], args[2:]
	if str, ok := recv.(starlark.String); ok {
		if err := checkStringMethod(name, string(str), rest, kwargs); err != nil {
			return nil, fmt.Errorf("%s: %v", name, err)
		}
	}
	ha, ok := recv.(starlark.HasAttrs)
	if !ok {
		return nil, fmt.Errorf("%s has no .%s field or method", recv.Type(), name)
	}
	fn, err := ha.Attr(name)
	if err != nil {
		return nil, err
	}
	if fn == nil {
		return nil, fmt.Errorf("%s has no .%s field or method", recv.Type(), name)
	}
	return starlark.Call(th, fn, rest, kwargs)
}

var errTooBig = fmt.Errorf("the result may be more than %d characters", limSeq)

// checkStringMethod refuses a call whose result would pass the limit. A call
// whose arguments are not the kind the method takes passes: the method itself
// reports that.
func checkStringMethod(name, recv string, args starlark.Tuple, kwargs []starlark.Tuple) error {
	limit := int64(limSeq)
	switch name {
	case "join":
		if len(args) != 1 || len(kwargs) != 0 {
			return nil
		}
		it, ok := args[0].(starlark.Iterable)
		if !ok {
			return nil
		}
		iter := it.Iterate()
		defer iter.Done()
		var total, n int64
		var x starlark.Value
		for iter.Next(&x) {
			if n > 0 {
				total += int64(len(recv))
			}
			n++
			if e, ok := x.(starlark.String); ok {
				total += int64(len(e))
			}
			if total > limit {
				return errTooBig
			}
		}
	case "replace":
		if len(args) < 2 || len(args) > 3 || len(kwargs) != 0 {
			return nil
		}
		old, ok1 := starlark.AsString(args[0])
		repl, ok2 := starlark.AsString(args[1])
		if !ok1 || !ok2 {
			return nil
		}
		count := int64(-1)
		if len(args) == 3 {
			c, err := starlark.AsInt32(args[2])
			if err != nil {
				return nil
			}
			count = int64(c)
		}
		var found int64
		if old == "" {
			found = int64(utf8Len(recv)) + 1
		} else {
			found = int64(strings.Count(recv, old))
		}
		if count >= 0 {
			found = min(found, count)
		}
		if int64(len(recv))+found*int64(len(repl)-len(old)) > limit {
			return errTooBig
		}
	case "format":
		widest := 0
		for _, v := range append(append(starlark.Tuple{}, args...), kwValues(kwargs)...) {
			w, err := formatWidth(v)
			if err != nil {
				return err
			}
			widest = max(widest, w)
		}
		if int64(len(recv))+int64(strings.Count(recv, "{"))*int64(widest) > limit {
			return errTooBig
		}
	}
	return nil
}

func utf8Len(s string) int { return len([]rune(s)) }

func kwValues(kwargs []starlark.Tuple) starlark.Tuple {
	out := make(starlark.Tuple, 0, len(kwargs))
	for _, kv := range kwargs {
		out = append(out, kv[1])
	}
	return out
}

// formatWidth is the most characters v can add to a formatted string: its
// length for a string, a fixed few for a number. Anything else is refused,
// since formatting it prints all of it.
func formatWidth(v starlark.Value) (int, error) {
	switch v := v.(type) {
	case starlark.String:
		return len(v), nil
	case starlark.Int:
		return limIntBits/3 + 2, nil
	case starlark.Float, starlark.Bool, starlark.NoneType:
		return 32, nil
	}
	return 0, fmt.Errorf("a %s cannot be formatted into a string in a script", v.Type())
}

// modChecked is what a script's x % y is rewritten to. A number on the left is
// a remainder, which cannot grow. A string on the left is a format: its result
// is the template with each % filled by an argument, so it is held to the
// limit by its largest possible size, and the arguments to plain values.
func modChecked(th *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 2 || len(kwargs) != 0 {
		return nil, errors.New("internal: bad format")
	}
	x, y := args[0], args[1]
	if tmpl, ok := x.(starlark.String); ok {
		vals := []starlark.Value{y}
		if t, ok := y.(starlark.Tuple); ok {
			vals = t
		}
		widest := 0
		for _, v := range vals {
			w, err := formatWidth(v)
			if err != nil {
				return nil, err
			}
			widest = max(widest, w)
		}
		if int64(len(tmpl))+int64(strings.Count(string(tmpl), "%"))*int64(widest) > int64(limSeq) {
			return nil, fmt.Errorf("the formatted string may be more than %d characters", limSeq)
		}
	}
	return starlark.Binary(syntax.PERCENT, x, y)
}

// guard walks a parsed script, rewrites each product and remainder into a call
// of the checked builtin, and refuses what a script may not use.
func guard(f *syntax.File) error {
	g := &guarder{}
	f.Stmts = g.stmts(f.Stmts)
	return g.err
}

type guarder struct{ err error }

func (g *guarder) fail(pos syntax.Position, format string, a ...any) {
	if g.err == nil {
		g.err = fmt.Errorf("%s:%d:%d: %s", pos.Filename(), pos.Line, pos.Col, fmt.Sprintf(format, a...))
	}
}

func (g *guarder) stmts(list []syntax.Stmt) []syntax.Stmt {
	for _, s := range list {
		g.stmt(s)
	}
	return list
}

func call(name string, pos syntax.Position, args ...syntax.Expr) syntax.Expr {
	return &syntax.CallExpr{Fn: &syntax.Ident{NamePos: pos, Name: name}, Lparen: pos, Args: args, Rparen: pos}
}

func (g *guarder) stmt(s syntax.Stmt) {
	switch s := s.(type) {
	case *syntax.AssignStmt:
		s.RHS = g.expr(s.RHS)
		var name string
		switch s.Op {
		case syntax.STAR_EQ:
			name = guardMul
		case syntax.PERCENT_EQ:
			name = guardMod
		}
		if name == "" {
			s.LHS = g.expr(s.LHS)
			return
		}
		id, ok := s.LHS.(*syntax.Ident)
		if !ok {
			g.fail(s.OpPos, "write x[i] = x[i] * n, not x[i] *= n: the checked operators take a plain name on the left")
			return
		}
		s.Op = syntax.EQ
		s.RHS = call(name, s.OpPos, &syntax.Ident{NamePos: id.NamePos, Name: id.Name}, s.RHS)
	case *syntax.DefStmt:
		g.expr(s.Name)
		g.params(s.Params)
		g.stmts(s.Body)
	case *syntax.ExprStmt:
		s.X = g.expr(s.X)
	case *syntax.IfStmt:
		s.Cond = g.expr(s.Cond)
		g.stmts(s.True)
		g.stmts(s.False)
	case *syntax.ForStmt:
		s.Vars = g.expr(s.Vars)
		s.X = g.expr(s.X)
		g.stmts(s.Body)
	case *syntax.WhileStmt:
		s.Cond = g.expr(s.Cond)
		g.stmts(s.Body)
	case *syntax.ReturnStmt:
		if s.Result != nil {
			s.Result = g.expr(s.Result)
		}
	case *syntax.LoadStmt:
		g.fail(s.Load, "load() is not available in a script")
	}
}

func (g *guarder) params(ps []syntax.Expr) {
	for i, p := range ps {
		if b, ok := p.(*syntax.BinaryExpr); ok && b.Op == syntax.EQ { // name=default
			b.Y = g.expr(b.Y)
			continue
		}
		ps[i] = g.expr(p)
	}
}

func (g *guarder) exprs(list []syntax.Expr) {
	for i, e := range list {
		list[i] = g.expr(e)
	}
}

func (g *guarder) expr(e syntax.Expr) syntax.Expr {
	switch e := e.(type) {
	case nil:
		return nil
	case *syntax.Ident:
		if e.Name == guardMul || e.Name == guardMod || e.Name == guardCall {
			g.fail(e.NamePos, "the name %s is reserved", e.Name)
		}
	case *syntax.BinaryExpr:
		e.X = g.expr(e.X)
		e.Y = g.expr(e.Y)
		switch e.Op {
		case syntax.STAR:
			return call(guardMul, e.OpPos, e.X, e.Y)
		case syntax.PERCENT:
			return call(guardMod, e.OpPos, e.X, e.Y)
		}
	case *syntax.UnaryExpr:
		e.X = g.expr(e.X)
	case *syntax.ParenExpr:
		e.X = g.expr(e.X)
	case *syntax.CallExpr:
		if d, ok := e.Fn.(*syntax.DotExpr); ok && bannedAttrs[d.Name.Name] {
			// x.join(a, b) becomes __ib_call("join", x, a, b).
			name := d.Name.Name
			lit := &syntax.Literal{Token: syntax.STRING, TokenPos: d.NamePos, Raw: strconv.Quote(name), Value: name}
			g.exprs(e.Args)
			recv := g.expr(d.X)
			return call(guardCall, d.NamePos, append([]syntax.Expr{lit, recv}, e.Args...)...)
		}
		e.Fn = g.expr(e.Fn)
		g.exprs(e.Args)
	case *syntax.DotExpr:
		if bannedAttrs[e.Name.Name] {
			g.fail(e.NamePos, "%s is not available in a script", e.Name.Name)
		}
		e.X = g.expr(e.X)
	case *syntax.IndexExpr:
		e.X = g.expr(e.X)
		e.Y = g.expr(e.Y)
	case *syntax.SliceExpr:
		e.X = g.expr(e.X)
		e.Lo = g.expr(e.Lo)
		e.Hi = g.expr(e.Hi)
		e.Step = g.expr(e.Step)
	case *syntax.CondExpr:
		e.Cond = g.expr(e.Cond)
		e.True = g.expr(e.True)
		e.False = g.expr(e.False)
	case *syntax.ListExpr:
		g.exprs(e.List)
	case *syntax.TupleExpr:
		g.exprs(e.List)
	case *syntax.DictExpr:
		g.exprs(e.List)
	case *syntax.DictEntry:
		e.Key = g.expr(e.Key)
		e.Value = g.expr(e.Value)
	case *syntax.LambdaExpr:
		g.params(e.Params)
		e.Body = g.expr(e.Body)
	case *syntax.Comprehension:
		e.Body = g.expr(e.Body)
		for _, c := range e.Clauses {
			switch c := c.(type) {
			case *syntax.ForClause:
				c.Vars = g.expr(c.Vars)
				c.X = g.expr(c.X)
			case *syntax.IfClause:
				c.Cond = g.expr(c.Cond)
			}
		}
	}
	return e
}
