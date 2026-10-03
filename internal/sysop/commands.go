package sysop

import (
	"bufio"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// Command is one immortal-barons mode the panel can run.
type Command struct {
	Flag string // e.g. "-maint"
	Help string
	// Coordinator marks a node #1 command; the panel offers it only there.
	Coordinator bool
	// Detailed marks a mode that honors -detailed.
	Detailed bool
	// Arg names the value the flag takes, "" for none. The panel asks for it.
	Arg string
	// Confirm is asked before running, for a command that acts on every board
	// in the league. "" runs it straight away.
	Confirm string
}

// Commands is every mode the panel runs, in the order it lists them. -reset is
// left out on purpose: it opens an interactive editor, which a log pane cannot
// drive.
var Commands = []Command{
	{Flag: "-maint", Detailed: true, Help: "Daily maintenance, plus the inter-BBS step on a league board"},
	{Flag: "-planetary", Detailed: true, Help: "The inter-BBS step alone: read, route and write packets"},
	{Flag: "-ftn-status", Help: "What the FTN transport's spools hold and why; changes nothing"},
	{Flag: "-league-check", Help: "Check the league setup and report everything wrong"},
	{Flag: "-league-routes", Help: "Which board each planet's packets go to, and where"},
	{Flag: "-bbsinfo", Help: "Write BBSINFO.LST"},
	{Flag: "-lastpacket", Help: "Write LASTPACKET.LST"},
	{Flag: "-gen-board-key", Help: "Create this board's packet-signing key"},
	{Flag: "-league-config", Coordinator: true, Help: "Send this board's league settings to every board"},
	{Flag: "-league-freeze", Coordinator: true, Arg: "message shown to callers",
		Help:    "Freeze the whole league for an update",
		Confirm: "This freezes play on EVERY board in the league until -league-thaw."},
	{Flag: "-league-thaw", Coordinator: true, Help: "End a league freeze on every board",
		Confirm: "This resumes play on every board in the league."},
	{Flag: "-league-reset", Coordinator: true, Arg: "start date, YYYY-MM-DD",
		Help:    "Reset every board in the league",
		Confirm: "This ends the current game on EVERY board in the league. It cannot be undone."},
	{Flag: "-gen-coord-key", Coordinator: true, Help: "Create the league's Coordinator key"},
}

var dateArg = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// Args is the command line for c against dataDir. It refuses a missing or
// malformed value rather than let the game read the next flag as one.
func (c Command) Args(dataDir string, detailed bool, arg string) ([]string, error) {
	args := []string{"-data", dataDir, c.Flag}
	if c.Arg != "" {
		arg = strings.TrimSpace(arg)
		if arg == "" {
			return nil, errors.New(c.Flag + " needs a " + c.Arg)
		}
		if c.Flag == "-league-reset" && !dateArg.MatchString(arg) {
			return nil, errors.New("-league-reset takes a date as YYYY-MM-DD")
		}
		args = append(args, arg)
	}
	if detailed && c.Detailed {
		args = append(args, "-detailed")
	}
	return args, nil
}

// GameProgram is the immortal-barons binary to run: the one beside this
// program if there is one, else the first on PATH. "" when neither exists.
func GameProgram() string {
	name := "immortal-barons"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if self, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(self), name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// Run is one child process, its output delivered a line at a time.
type Run struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
	once sync.Once
}

// Start runs program with args from the data directory's parent, where a door
// install keeps the binary and runs it from, and hands each line of stdout and
// stderr to line as it arrives. line is called from another goroutine.
func Start(program, dataDir string, args []string, line func(string)) (*Run, error) {
	// exec resolves a relative path against cmd.Dir, not the directory the
	// sysop typed it from.
	if strings.ContainsRune(program, filepath.Separator) || strings.ContainsRune(program, '/') {
		if abs, err := filepath.Abs(program); err == nil {
			program = abs
		}
	}
	cmd := exec.Command(program, args...)
	cmd.Dir = filepath.Dir(dataDir)
	pr, pw := io.Pipe()
	cmd.Stdout, cmd.Stderr = pw, pw
	cmd.Stdin = nil // a mode that prompts gets EOF and ends, never hangs
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	r := &Run{cmd: cmd, done: make(chan struct{})}
	scanned := make(chan struct{})
	go func() {
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line(sc.Text())
		}
		io.Copy(io.Discard, pr) // an over-long line must not block the child
		close(scanned)
	}()
	go func() {
		r.err = cmd.Wait()
		pw.Close()
		<-scanned
		close(r.done)
	}()
	return r, nil
}

// Done is closed once the process has exited and all its output delivered.
func (r *Run) Done() <-chan struct{} { return r.done }

// Err is the exit status, valid after Done. A mode that met a new fault exits
// non-zero on purpose.
func (r *Run) Err() error { <-r.done; return r.err }

// Stop asks the process to end: an interrupt where the platform has one, a
// kill on Windows, which does not. The world and packets are written by
// replace-on-rename and the lock dies with the process, so a stop loses the run
// in progress and nothing more.
func (r *Run) Stop() {
	r.once.Do(func() {
		if runtime.GOOS == "windows" || r.cmd.Process.Signal(os.Interrupt) != nil {
			r.cmd.Process.Kill()
		}
	})
}
