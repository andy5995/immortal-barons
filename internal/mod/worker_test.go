package mod

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// lowerLimits sets limits for one test, and ends the workers that started
// under the old ones, before and after.
func lowerLimits(t *testing.T) {
	to, mem := limTimeout, limMemory
	pool.flush()
	t.Cleanup(func() {
		limTimeout, limMemory, noSystemLimit = to, mem, false
		SetStateDir("")
		skipClock = time.Now
		pool.flush()
	})
}

// regionAnswer runs src, a region script, once on a fresh view.
func regionAnswer(t testing.TB, src string) (game.RegionRules, error) {
	t.Helper()
	ref := newScriptRef(kindRegion, "t.star", src, &optionValues{})
	call := wireMsg{Op: "call", ID: ref.id, Region: &wireRegion{}}
	res, err := callWorker(ref, call, nil)
	if err != nil {
		return game.RegionRules{}, err
	}
	return *res.Row, nil
}

const quick = "def region(realm, row):\n    return {\"food_base\": 7}\n"

// A loop of calls that each take a long time in Go, which the step cap does not
// see: three hundred thousand scans of a ten-megabyte string.
const slow = "def region(realm, row):\n    s = 'a' * 10000000\n    for i in range(300000):\n        s.count('a')\n    return {}\n"

func TestAQuickScriptRunsInAWorker(t *testing.T) {
	got, err := regionAnswer(t, quick)
	if err != nil || got.FoodBase != 7 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestATimeoutKillsTheWorkerAndTheNextCallWorks(t *testing.T) {
	lowerLimits(t)
	limTimeout = time.Second
	start := time.Now()
	_, err := regionAnswer(t, slow)
	if err == nil || !strings.Contains(err.Error(), "ran for more than") || !strings.Contains(err.Error(), "starts a new one") {
		t.Fatalf("err = %v, want the time limit and what happens next", err)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Errorf("the run took %v to be stopped", took)
	}
	got, err := regionAnswer(t, quick)
	if err != nil || got.FoodBase != 7 {
		t.Fatalf("the next call: %+v, %v", got, err)
	}
}

func TestMemoryRunawayKillsOnlyTheWorker(t *testing.T) {
	lowerLimits(t)
	limMemory = 64 << 20
	_, err := regionAnswer(t, "def region(realm, row):\n    x = [0]\n    for i in range(27):\n        x = x + x\n    return {}\n")
	if err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("err = %v, want the worker stopped", err)
	}
	if got, err := regionAnswer(t, quick); err != nil || got.FoodBase != 7 {
		t.Fatalf("the next call: %+v, %v", got, err)
	}
}

// Where the system sets no limit, as on macOS, the worker's own heap watchdog
// ends it.
func TestTheHeapWatchdogStopsWhatTheSystemDoesNot(t *testing.T) {
	lowerLimits(t)
	limMemory, noSystemLimit = 64<<20, true
	// The watchdog is under test, not the timeout: give a loaded CI runner
	// (the 32-bit job took past 3s) all the time it needs.
	limTimeout = 30 * time.Second
	_, err := regionAnswer(t, "def region(realm, row):\n    x = [0]\n    for i in range(27):\n        x = x + x\n    return {}\n")
	if err == nil || !strings.Contains(err.Error(), "used more than 64 MiB") {
		t.Fatalf("err = %v, want the watchdog's message", err)
	}
	if got, err := regionAnswer(t, quick); err != nil || got.FoodBase != 7 {
		t.Fatalf("the next call: %+v, %v", got, err)
	}
}

// One allocation the heap watchdog never sees: the system's limit stops it.
func TestOneHugeAllocationIsStoppedBySystemLimit(t *testing.T) {
	if runtime.GOOS != "linux" || raceBuild {
		t.Skip("the limit was measured on Linux, without the race detector")
	}
	lowerLimits(t)
	limMemory = 64 << 20
	_, err := regionAnswer(t, "def region(realm, row):\n    s = 'a' * 1000000000\n    return {}\n")
	if err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("err = %v, want the worker stopped", err)
	}
	if got, err := regionAnswer(t, quick); err != nil || got.FoodBase != 7 {
		t.Fatalf("the next call: %+v, %v", got, err)
	}
}

func TestACrashedWorkerIsOneFailedCall(t *testing.T) {
	lowerLimits(t)
	if _, err := regionAnswer(t, quick); err != nil {
		t.Fatal(err)
	}
	pool.mu.Lock()
	for _, w := range pool.idle {
		w.cmd.Process.Kill()
		w.cmd.Process.Wait()
	}
	pool.mu.Unlock()
	if _, err := regionAnswer(t, quick); err == nil || !strings.Contains(err.Error(), "stopped") {
		t.Fatalf("err = %v, want the crashed worker reported", err)
	}
	if got, err := regionAnswer(t, quick); err != nil || got.FoodBase != 7 {
		t.Fatalf("the next call: %+v, %v", got, err)
	}
}

// A kill ends one call. Others, running in other workers, are not touched.
func TestAKillDoesNotFailOtherCalls(t *testing.T) {
	lowerLimits(t)
	limTimeout = time.Second
	if runtime.GOMAXPROCS(0) < 2 {
		t.Skip("one worker serves every call here")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := regionAnswer(t, slow); err == nil {
			errs <- errors.New("the slow script was not stopped")
		}
	}()
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if got, err := regionAnswer(t, quick); err != nil || got.FoodBase != 7 {
					errs <- errors.New("a quick call failed")
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// A draw is made by the game, in the order the script asks.
func TestRandRoundTripsInOrder(t *testing.T) {
	src := "def raid(realm, factions):\n    a = rand(10)\n    b = rand(7)\n    return [{\"faction\": 0, \"take\": \"gold\"}] * (a + b - a - b + 1)\n"
	s, err := newPirateScript(t.TempDir(), "m", "h", src, &optionValues{})
	if err != nil {
		t.Fatal(err)
	}
	var asked []int
	orders, err := s.Raid(game.PirateView{Rand: func(n int) int { asked = append(asked, n); return n - 1 }})
	if err != nil || len(orders) != 1 {
		t.Fatalf("orders %v, err %v", orders, err)
	}
	if len(asked) != 2 || asked[0] != 10 || asked[1] != 7 {
		t.Errorf("the draws asked for were %v, want [10 7]", asked)
	}
}

// On a door the game's own stdin and stdout are the caller. The worker must
// hold neither.
func TestTheWorkerDoesNotInheritStdinOrStdout(t *testing.T) {
	lowerLimits(t)
	inR, inW, _ := os.Pipe()
	outR, outW, _ := os.Pipe()
	oldIn, oldOut := os.Stdin, os.Stdout
	os.Stdin, os.Stdout = inR, outW
	defer func() { os.Stdin, os.Stdout = oldIn, oldOut }()
	inW.WriteString("the caller's keystrokes\n")

	if got, err := regionAnswer(t, quick); err != nil || got.FoodBase != 7 {
		t.Fatalf("%+v, %v", got, err)
	}
	pool.flush()

	// Nothing the worker did reached the game's stdout...
	outW.Close()
	if b, _ := io.ReadAll(outR); len(b) != 0 {
		t.Errorf("the worker wrote %q to the game's stdout", b)
	}
	// ...and what the caller typed is still there to be read.
	buf := make([]byte, 64)
	inR.SetReadDeadline(time.Now().Add(time.Second))
	n, err := inR.Read(buf)
	if string(buf[:n]) != "the caller's keystrokes\n" {
		t.Errorf("the game's stdin held %q (%v) after the worker ran", buf[:n], err)
	}
}

func TestNoWorkerMeansAClearFailureNotAnInProcessRun(t *testing.T) {
	lowerLimits(t)
	old := executable
	executable = func() (string, error) { return "/nonexistent/ib-worker", nil }
	defer func() { executable = old }()
	var buf = captureWarnings(t)
	onceMu.Lock()
	onceSeen = map[string]bool{}
	onceMu.Unlock()

	src := "def region(realm, row):\n    return {}\n"
	// The mod still loads.
	s, err := newRegionScript(t.TempDir(), "m", "h", game.Desert, src, &optionValues{})
	if err != nil {
		t.Fatalf("the mod did not load: %v", err)
	}
	if !strings.Contains(buf.String(), "could not be started") {
		t.Errorf("no warning that the worker cannot start: %q", buf.String())
	}
	// Its calls fail, and say why.
	if _, err := s.Region(game.RegionView{}, game.RegionRules{}); err == nil || !strings.Contains(err.Error(), "could not be started") {
		t.Errorf("err = %v, want the worker's failure", err)
	}
}

func BenchmarkWorkerRegionCall(b *testing.B) {
	ref := newScriptRef(kindRegion, "b.star", quick, &optionValues{})
	call := wireMsg{Op: "call", ID: ref.id, Region: &wireRegion{}}
	if _, err := callWorker(ref, call, nil); err != nil {
		b.Fatal(err)
	}
	for b.Loop() {
		if _, err := callWorker(ref, call, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWorkerRaidWithDraws(b *testing.B) {
	src := "def raid(realm, factions):\n    if rand(10) > 5:\n        return [{\"faction\": 0, \"take\": \"gold\"}]\n    return []\n"
	s, err := newPirateScript(b.TempDir(), "m", "h", src, &optionValues{})
	if err != nil {
		b.Fatal(err)
	}
	v := game.PirateView{Realm: game.PirateRealmView{Units: game.PirateHoldings{}, Market: game.PirateHoldings{}}, Rand: func(n int) int { return 3 }}
	for b.Loop() {
		if _, err := s.Raid(v); err != nil {
			b.Fatal(err)
		}
	}
}

// fakeWorker is a worker whose replies are a shell script's: what a script
// that took the worker over could send.
func fakeWorker(t *testing.T, script string) *worker {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs sh")
	}
	cmd := exec.Command("sh", "-c", script)
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	tail := &tailBuffer{}
	cmd.Stderr = tail
	if err := cmd.Start(); err != nil {
		t.Skip(err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 1<<16), workerLineMax)
	w := &worker{cmd: cmd, enc: json.NewEncoder(stdin), sc: sc, stdin: stdin, stderr: tail, compiled: map[string]bool{}}
	t.Cleanup(w.close)
	return w
}

func TestABadMessageFromTheWorkerIsAFailedCallAndKillsIt(t *testing.T) {
	for name, reply := range map[string]string{
		"draw of zero":    `{"op":"rand","n":0}`,
		"negative draw":   `{"op":"rand","n":-5}`,
		"draw past int32": `{"op":"rand","n":4294967296}`,
		"unknown op":      `{"op":"exec"}`,
		"not json":        `hello`,
		"too many orders": `{"op":"result","orders":[` + strings.Repeat(`{"Slot":0},`, game.ModPirateOrdersMax) + `{"Slot":0}]}`,
	} {
		t.Run(name, func(t *testing.T) {
			w := fakeWorker(t, "read x; echo '"+reply+"'; exec sleep 5")
			called := false
			start := time.Now()
			_, err := w.roundTrip(wireMsg{Op: "call"}, func(n int) (int, error) { called = true; return 1, nil })
			if err == nil || !strings.Contains(err.Error(), "stopped") {
				t.Fatalf("err = %v, want a failed call", err)
			}
			if called {
				t.Error("the game was asked to draw")
			}
			if !w.dead || time.Since(start) > 2*time.Second {
				t.Errorf("the worker was not killed at once (dead %v, %v)", w.dead, time.Since(start))
			}
		})
	}
}

func TestAGoodDrawFromAWorkerIsAnswered(t *testing.T) {
	w := fakeWorker(t, `read x; echo '{"op":"rand","n":6}'; read y; echo '{"op":"result"}'`)
	var asked int
	if _, err := w.roundTrip(wireMsg{Op: "call"}, func(n int) (int, error) { asked = n; return 2, nil }); err != nil || asked != 6 {
		t.Fatalf("asked %d, err %v", asked, err)
	}
}

// The variable alone, as an outside environment might carry it, leaves a
// binary what it is: only the game's own argument makes a worker.
func TestTheVariableAloneMakesNoWorker(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), workerEnv+"=1")
	cmd.Stdin = strings.NewReader("")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), "PASS") {
		t.Fatalf("the test binary ran as a worker: %v, %q", err, out)
	}
}

// A region script that hangs for a realm with one region, and is quick for the
// rest. The views differ, so the answers are not remembered between calls.
const hangsForOne = "def region(realm, row):\n    if realm.land == 1:\n        s = 'a' * 10000000\n        for i in range(300000):\n            s.count('a')\n    return {\"food_base\": 7}\n"

func regionCall(t *testing.T, s *regionScript, land int) error {
	t.Helper()
	_, err := s.Region(game.RegionView{Land: land}, game.RegionRules{})
	return err
}

func TestAScriptThatTimesOutThreeTimesInARowIsSkipped(t *testing.T) {
	lowerLimits(t)
	limTimeout = 300 * time.Millisecond
	buf := captureWarnings(t)
	bad, err := newRegionScript(t.TempDir(), "m", "h", game.Desert, hangsForOne, &optionValues{})
	if err != nil {
		t.Fatal(err)
	}
	other, err := newRegionScript(t.TempDir(), "n", "h", game.Desert, quick, &optionValues{})
	if err != nil {
		t.Fatal(err)
	}
	for i := range ScriptTimeoutsToSkip {
		// A failed answer is not remembered, so the same view can be asked again.
		if err := regionCall(t, bad, 1); err == nil || !strings.Contains(err.Error(), "ran for more than") {
			t.Fatalf("call %d: err = %v, want a timeout", i+1, err)
		}
	}
	if !strings.Contains(buf.String(), "is skipped until") || !strings.Contains(buf.String(), `mod "m"`) {
		t.Errorf("no warning naming the script: %q", buf.String())
	}
	// The next call is refused at once, with no worker started.
	old := executable
	executable = func() (string, error) { t.Error("a worker was started"); return "", errors.New("no") }
	defer func() { executable = old }()
	start := time.Now()
	if err := regionCall(t, bad, 1); !errors.Is(err, errScriptSkipped) {
		t.Fatalf("err = %v, want the script skipped", err)
	}
	if err := regionCall(t, bad, 2); !errors.Is(err, errScriptSkipped) {
		t.Fatalf("a view that would have succeeded: err = %v, want the script skipped", err)
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Errorf("a skipped call took %v", time.Since(start))
	}
	executable = old
	// Another script goes on running, and the skipped one's later calls add no warning.
	if err := regionCall(t, other, 5); err != nil {
		t.Errorf("another script: %v", err)
	}
	bad.Failed(errScriptSkipped)
	if strings.Count(buf.String(), "\n") != 1 {
		t.Errorf("more than the one warning: %q", buf.String())
	}
}

func TestASuccessBetweenTimeoutsStartsTheCountAgain(t *testing.T) {
	lowerLimits(t)
	limTimeout = 300 * time.Millisecond
	s, err := newRegionScript(t.TempDir(), "m", "h", game.Desert, hangsForOne, &optionValues{})
	if err != nil {
		t.Fatal(err)
	}
	for round := range 3 {
		for range ScriptTimeoutsToSkip - 1 {
			if err := regionCall(t, s, 1); err == nil {
				t.Fatalf("round %d: the hanging call returned", round)
			}
		}
		if err := regionCall(t, s, 2+round); err != nil {
			t.Fatalf("round %d: the quick call failed: %v", round, err)
		}
	}
	if err := regionCall(t, s, 1); errors.Is(err, errScriptSkipped) {
		t.Fatal("the script was skipped though no three timeouts came in a row")
	}
}

func TestAnotherFailureDoesNotCountAsATimeout(t *testing.T) {
	lowerLimits(t)
	s, err := newRegionScript(t.TempDir(), "m", "h", game.Desert, "def region(realm, row):\n    fail('no')\n", &optionValues{})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 * ScriptTimeoutsToSkip {
		if err := regionCall(t, s, 1); err == nil || errors.Is(err, errScriptSkipped) {
			t.Fatalf("err = %v, want an ordinary failure every time", err)
		}
	}
}

// The same script played with other option values is a different script to the
// breaker: tripping it with one choice leaves the other running.
func TestTheBreakerCountsEachOptionChoiceOnItsOwn(t *testing.T) {
	lowerLimits(t)
	limTimeout = 300 * time.Millisecond
	captureWarnings(t)
	src := "def region(realm, row):\n    if option(\"slow\"):\n        s = 'a' * 10000000\n        for i in range(300000):\n            s.count('a')\n    return {\"food_base\": 7}\n"
	defs := map[string]*Option{"slow": {Type: OptionBool, Label: "Slow", Default: false}}
	with := func(v int64) *regionScript {
		t.Helper()
		s, err := newRegionScript(t.TempDir(), "m", "h", game.Desert, src, &optionValues{defs: defs, chosen: map[string]int64{"slow": v}})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	slowOne := with(1)
	for i := range ScriptTimeoutsToSkip {
		if err := regionCall(t, slowOne, i+1); err == nil {
			t.Fatalf("call %d did not time out", i+1)
		}
	}
	if err := regionCall(t, slowOne, 9); !errors.Is(err, errScriptSkipped) {
		t.Fatalf("err = %v, want the slow choice skipped", err)
	}
	if err := regionCall(t, with(0), 1); err != nil {
		t.Errorf("the other choice was skipped too: %v", err)
	}
}

// trip trips the breaker for a script that hangs for one view, in the
// state directory dir, and returns the script.
func trip(t *testing.T, dir string, optionValue int64) *regionScript {
	t.Helper()
	SetStateDir(dir)
	limTimeout = 300 * time.Millisecond
	s := hangingScript(t, "h", optionValue)
	for range ScriptTimeoutsToSkip {
		if err := regionCall(t, s, 1); err == nil {
			t.Fatal("the hanging call returned")
		}
	}
	return s
}

func hangingScript(t *testing.T, hash string, optionValue int64) *regionScript {
	opts := &optionValues{defs: map[string]*Option{"x": {Type: OptionNumber}}, chosen: map[string]int64{"x": optionValue}}
	s, err := newRegionScript("mods/m", "m", hash, game.Desert, hangsForOne, opts)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func refuseWorkers(t *testing.T) {
	old := executable
	executable = func() (string, error) { t.Error("a worker was started"); return "", errors.New("no") }
	t.Cleanup(func() { executable = old })
}

func TestASkippedScriptStaysSkippedForTheNextProcess(t *testing.T) {
	lowerLimits(t)
	dir := t.TempDir()
	buf := captureWarnings(t)
	trip(t, dir, 1)
	if !strings.Contains(buf.String(), "is skipped until") || !strings.Contains(buf.String(), skipFile) {
		t.Errorf("the warning does not say until when, or where: %q", buf.String())
	}
	if _, err := os.Stat(filepath.Join(dir, skipFile)); err != nil {
		t.Fatalf("no file: %v", err)
	}

	// A new process: nothing in memory, the same script.
	resetBreaker()
	buf = captureWarnings(t)
	refuseWorkers(t)
	s := hangingScript(t, "h", 1)
	for range 3 {
		if err := regionCall(t, s, 1); !errors.Is(err, errScriptSkipped) {
			t.Fatalf("err = %v, want the script skipped from the file", err)
		}
		s.Failed(errScriptSkipped)
	}
	if got := buf.String(); strings.Count(got, "\n") != 1 || !strings.Contains(got, "is still skipped after timing out (until ") {
		t.Errorf("want one short line, got %q", got)
	}
}

func TestASkipEndsAfterADay(t *testing.T) {
	lowerLimits(t)
	dir := t.TempDir()
	trip(t, dir, 1)
	resetBreaker()
	now := time.Now()
	skipClock = func() time.Time { return now.Add(ScriptSkipHold - time.Minute) }
	if err := regionCall(t, hangingScript(t, "h", 1), 2); !errors.Is(err, errScriptSkipped) {
		t.Fatalf("a minute short of a day: err = %v, want it still skipped", err)
	}
	resetBreaker()
	skipClock = func() time.Time { return now.Add(ScriptSkipHold + time.Minute) }
	if err := regionCall(t, hangingScript(t, "h", 1), 2); err != nil {
		t.Fatalf("a day on: err = %v, want it to run again", err)
	}
}

func TestAChangedModOrOptionIsNotSkipped(t *testing.T) {
	lowerLimits(t)
	dir := t.TempDir()
	trip(t, dir, 1)
	resetBreaker()
	for name, s := range map[string]*regionScript{
		"another option value": hangingScript(t, "h", 2),
		"another mod hash":     hangingScript(t, "other", 1),
	} {
		if err := regionCall(t, s, 2); err != nil {
			t.Errorf("%s: err = %v, want it to run", name, err)
		}
	}
}

func TestABadSkipFileIsIgnored(t *testing.T) {
	lowerLimits(t)
	dir := t.TempDir()
	big := `{"skipped":{},"pad":"` + strings.Repeat("x", skipFileMax) + `"}`
	for name, content := range map[string]string{"garbage": "not json {", "empty": "", "too big": big} {
		os.WriteFile(filepath.Join(dir, skipFile), []byte(content), 0o644)
		resetBreaker()
		SetStateDir(dir)
		if err := regionCall(t, hangingScript(t, "h", 1), 2); err != nil {
			t.Errorf("%s: err = %v, want the file ignored", name, err)
		}
	}
}

func TestSavingASkipLeavesNoTempFileAndNoStateDirNoFile(t *testing.T) {
	lowerLimits(t)
	dir := t.TempDir()
	trip(t, dir, 1)
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != skipFile {
		t.Errorf("the directory holds %v, want only %s", entries, skipFile)
	}

	// No state directory, as in -check-mod and -sim-mod: nothing is written.
	resetBreaker()
	empty := t.TempDir()
	SetStateDir("")
	limTimeout = 300 * time.Millisecond
	s := hangingScript(t, "h2", 1)
	for range ScriptTimeoutsToSkip {
		regionCall(t, s, 1)
	}
	if err := regionCall(t, s, 1); !errors.Is(err, errScriptSkipped) {
		t.Fatalf("not skipped in memory: %v", err)
	}
	if entries, _ := os.ReadDir(empty); len(entries) != 0 {
		t.Errorf("files appeared: %v", entries)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, skipFile)); strings.Contains(string(b), "h2") {
		t.Error("an entry was written with no state directory set")
	}
}
