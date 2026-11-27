package mod

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// This is the game's half of the script worker (worker.go): starting workers,
// sending them requests, and killing one that takes too long.
//
// Workers are kept in a small pool grown on demand, one request in flight on
// each. A worker is a whole process, so a script that does one slow call blocks
// only the goroutine that made it, and killing a worker ends that one call and
// no other. The pool is as large as GOMAXPROCS, to at most four: more would
// only add idle processes, each of which can hold up to ScriptMemory.
const workerPoolMax = 4

// workerLineMax is the longest reply the game reads. A real reply is a list of
// raids or one row of figures; a script cannot make a longer one.
const workerLineMax = 1 << 20

// errNoWorker means no worker could be started: the executable would not run.
// It says nothing about the script.
var errNoWorker = errors.New("a script worker process could not be started")

// scriptRef identifies one script to the workers: everything a worker needs to
// compile it again, which it must when a new worker has replaced one that was
// killed. The game never holds a compiled script.
type scriptRef struct {
	id, kind, file, src string
	options             *optionValues
}

func newScriptRef(kind, file, src string, options *optionValues) *scriptRef {
	sum := sha256.Sum256([]byte(kind + "\x00" + file + "\x00" + src + options.key()))
	return &scriptRef{id: hex.EncodeToString(sum[:]), kind: kind, file: file, src: src, options: options}
}

func (r *scriptRef) compileMsg() wireMsg {
	return wireMsg{Op: "compile", ID: r.id, Kind: r.kind, File: r.file, Src: r.src, Options: optionsToWire(r.options)}
}

// validated holds the scripts known to compile, so the settings editor, which
// reads every mod again on each change, does not compile them each time. A
// failure is not kept: it is reported each time, as it always was.
var validated sync.Map

// validate compiles the script in a worker, so a mod with a script that does
// not compile does not load. It reports an error for a script that fails, and
// none when no worker can be started: the mod loads, every call of its script
// fails with the reason, and the game uses its built-in rules there.
func (r *scriptRef) validate() error {
	if _, ok := validated.Load(r.id); ok {
		return nil
	}
	w, err := pool.get()
	if err != nil {
		warnOnce(err.Error() + "; mods with scripts play their built-in rules instead")
		return nil
	}
	defer pool.put(w)
	if err := w.ensure(r); err != nil {
		return err
	}
	validated.Store(r.id, true)
	return nil
}

// callWorker sends call, a request for the script ref, to a worker and returns
// its answer. onRand answers the worker's draws from the game's generator while
// the call runs. Any failure is an ordinary failed script to the caller.
func callWorker(r *scriptRef, call wireMsg, onRand func(n int) (int, error)) (wireMsg, error) {
	w, err := pool.get()
	if err != nil {
		return wireMsg{}, errors.New(clean(err.Error()))
	}
	defer pool.put(w)
	if err := w.ensure(r); err != nil {
		return wireMsg{}, err
	}
	res, err := w.roundTrip(call, onRand)
	if err != nil {
		return wireMsg{}, err
	}
	if res.Err != "" {
		return wireMsg{}, errors.New(clean(truncate(res.Err, 500)))
	}
	if call.Region != nil && res.Row == nil {
		return wireMsg{}, errors.New("the worker gave no answer")
	}
	return res, nil
}

type worker struct {
	cmd      *exec.Cmd
	enc      *json.Encoder
	sc       *bufio.Scanner
	stdin    io.Closer
	stderr   *tailBuffer
	release  func() // what attachLimits asks to be undone
	compiled map[string]bool
	dead     bool
}

// tailBuffer keeps the first bytes written to it: a worker's stderr, where
// its death says why on the first line and a trace follows.
type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
}

const tailMax = 2048

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if room := tailMax - len(t.buf); room > 0 {
		t.buf = append(t.buf, p[:min(room, len(p))]...)
	}
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// workerEnviron is the worker's whole environment. It gets nothing of the
// game's, and no data directory.
func workerEnviron() []string {
	env := []string{workerEnv + "=1", fmt.Sprintf("%s=%d", workerMemEnv, limMemory>>20)}
	if noSystemLimit {
		env = append(env, workerNoLimitEnv+"=1")
	}
	if runtime.GOOS == "windows" {
		env = append(env, "SYSTEMROOT="+os.Getenv("SYSTEMROOT"))
	}
	return env
}

// noSystemLimit has a test start workers that rely on their heap watchdog
// alone, as on macOS.
var noSystemLimit bool

// executable finds the program to run as a worker: this one. A test replaces it.
var executable = os.Executable

func startWorker() (*worker, error) {
	exe, err := executable()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoWorker, err)
	}
	cmd := exec.Command(exe, workerArg)
	cmd.Env = workerEnviron()
	cmd.Dir = os.TempDir()
	// The worker's stdin and stdout are pipes made for it. On a door the game's
	// own are the caller's connection, and the worker must never hold them.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoWorker, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errNoWorker, err)
	}
	tail := &tailBuffer{}
	cmd.Stderr = tail
	protectInheritedFDs()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", errNoWorker, err)
	}
	release, err := attachLimits(cmd, limMemory)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return nil, fmt.Errorf("%w: %v", errNoWorker, err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<16), workerLineMax)
	return &worker{
		cmd: cmd, enc: json.NewEncoder(stdin), sc: sc, stdin: stdin,
		stderr: tail, release: release, compiled: map[string]bool{},
	}, nil
}

// close ends the process, and waits for it.
func (w *worker) close() {
	w.dead = true
	w.cmd.Process.Kill()
	w.stdin.Close()
	w.cmd.Wait()
	if w.release != nil {
		w.release()
	}
}

// ensure has the worker compile r if it has not.
func (w *worker) ensure(r *scriptRef) error {
	if w.compiled[r.id] {
		return nil
	}
	res, err := w.roundTrip(r.compileMsg(), nil)
	if err != nil {
		return err
	}
	if res.Err != "" {
		return errors.New(clean(truncate(res.Err, 500)))
	}
	w.compiled[r.id] = true
	return nil
}

// roundTrip sends req and returns the worker's result, answering its requests
// for a random number on the way. A worker that is still working after
// limTimeout is killed. A worker that fails any way but with a result is dead
// after this, and the error says so and what happens next.
func (w *worker) roundTrip(req wireMsg, onRand func(n int) (int, error)) (wireMsg, error) {
	timeout := limTimeout
	var timedOut atomic.Bool
	timer := time.AfterFunc(timeout, func() {
		timedOut.Store(true)
		w.cmd.Process.Kill()
	})
	defer timer.Stop()
	fail := func(why string) (wireMsg, error) {
		w.close()
		switch {
		case timedOut.Load():
			return wireMsg{}, fmt.Errorf("the script ran for more than %v; its worker process was stopped, and the next script run starts a new one", timeout)
		default:
			tail := clean(truncate(firstLine(w.stderr.String()), 200))
			if tail != "" {
				tail = " (" + tail + ")"
			}
			return wireMsg{}, fmt.Errorf("the script worker process stopped (%s)%s; the next script run starts a new one", why, tail)
		}
	}
	if err := w.enc.Encode(&req); err != nil {
		return fail("could not send to it")
	}
	for {
		if !w.sc.Scan() {
			if errors.Is(w.sc.Err(), bufio.ErrTooLong) {
				return fail("its reply was too long")
			}
			return fail("it ended")
		}
		var m wireMsg
		if err := json.Unmarshal(w.sc.Bytes(), &m); err != nil {
			return fail("its reply was not understood")
		}
		// A worker runs a script, and a script is not trusted: nothing in a
		// message reaches the game unchecked.
		switch m.Op {
		case "rand":
			// The game's generator panics on a count below 1, and a panic here
			// ends a door session.
			if m.N < 1 || m.N > math.MaxInt32 {
				return fail("it asked for a draw the game refused")
			}
			reply := wireMsg{Op: "rand"}
			if onRand == nil {
				reply.Err = "no draws are allowed here"
			} else if v, err := onRand(m.N); err != nil {
				reply.Err = err.Error()
			} else {
				reply.V = v
			}
			if err := w.enc.Encode(&reply); err != nil {
				return fail("could not send to it")
			}
		case "result":
			if len(m.Orders) > game.ModPirateOrdersMax {
				return fail("its answer was too long")
			}
			if !timer.Stop() {
				// The timer fired as the answer arrived: the process is gone.
				w.close()
			}
			return m, nil
		default:
			return fail("its reply was not understood")
		}
	}
}

// firstLine is the first line of a worker's stderr: what it died of (a fatal
// error or a panic), before the trace that follows.
func firstLine(s string) string {
	s = strings.TrimLeft(s, "\r\n")
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	return s
}

// workerPool holds the idle workers, and counts all of them.
type workerPool struct {
	mu   sync.Mutex
	cond *sync.Cond
	idle []*worker
	n    int
	max  int
}

var pool = newWorkerPool()

func newWorkerPool() *workerPool {
	p := &workerPool{max: min(max(runtime.GOMAXPROCS(0), 1), workerPoolMax)}
	p.cond = sync.NewCond(&p.mu)
	return p
}

// get hands out an idle worker, starts one if the pool is not full, or waits.
func (p *workerPool) get() (*worker, error) {
	p.mu.Lock()
	for {
		if n := len(p.idle); n > 0 {
			w := p.idle[n-1]
			p.idle = p.idle[:n-1]
			p.mu.Unlock()
			return w, nil
		}
		if p.n < p.max {
			p.n++
			p.mu.Unlock()
			w, err := startWorker()
			if err != nil {
				p.mu.Lock()
				p.n--
				p.cond.Signal()
				p.mu.Unlock()
				return nil, err
			}
			return w, nil
		}
		p.cond.Wait()
	}
}

// put takes a worker back; a dead one is let go and its place is free.
func (p *workerPool) put(w *worker) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if w.dead {
		p.n--
	} else {
		p.idle = append(p.idle, w)
	}
	p.cond.Signal()
}

// flush ends every idle worker, so the next call starts new ones: with the
// limits as they now are, in a test that lowers one.
func (p *workerPool) flush() {
	p.mu.Lock()
	idle := p.idle
	p.idle = nil
	p.n -= len(idle)
	p.mu.Unlock()
	for _, w := range idle {
		w.close()
	}
	validated.Clear()
}

var (
	onceMu   sync.Mutex
	onceSeen = map[string]bool{}
)

// warnOnce reports msg to the sysop the first time only.
func warnOnce(msg string) {
	onceMu.Lock()
	seen := onceSeen[msg]
	onceSeen[msg] = true
	onceMu.Unlock()
	if !seen {
		Warn(msg)
	}
}
