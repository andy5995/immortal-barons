//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package mod

import (
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
)

// workerSelfLimit has the worker limit itself before it reads a script: its
// data segment, which Go's heap lives in, to limit plus room for the runtime
// (RLIMIT_DATA counts every private writable mapping on Linux since 4.7).
// macOS does not enforce it, so the heap watchdog is the limit there. A limit
// that cannot be set is ignored: the watchdog still holds.
func workerSelfLimit(limit uint64) {
	if runtime.GOOS == "darwin" {
		return
	}
	n := limit + workerRuntimeRoom
	var r syscall.Rlimit
	setLimit(&r.Cur, n)
	setLimit(&r.Max, n)
	syscall.Setrlimit(syscall.RLIMIT_DATA, &r)
}

// workerRuntimeRoom is what the Go runtime needs beyond the script's own
// memory: the garbage collector's bookkeeping, goroutine stacks, the binary's
// writable data. Measured on Linux: the runtime starts and runs with as little as 8 MiB beyond a 64 MiB limit, so this leaves room for the collector to work near the limit.
const workerRuntimeRoom = 128 << 20

var cpuExitOnce sync.Once

// workerArmCPU gives the worker CPU seconds for one request on top of what it
// has used, so a process left running for a game that is gone is stopped. The
// game's own timeout comes first, and it kills the worker. The system signals
// SIGXCPU at the soft limit, which the Go runtime ignores unless asked for it,
// so the worker asks and exits.
func workerArmCPU() {
	cpuExitOnce.Do(func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGXCPU)
		go func() {
			<-c
			os.Exit(4)
		}()
	})
	var ru syscall.Rusage
	if syscall.Getrusage(syscall.RUSAGE_SELF, &ru) != nil {
		return
	}
	used := uint64(ru.Utime.Sec) + uint64(ru.Stime.Sec)
	var r syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_CPU, &r) != nil {
		return
	}
	setLimit(&r.Cur, used+workerCPUBackstop) // the hard limit stays as it is
	syscall.Setrlimit(syscall.RLIMIT_CPU, &r)
}

// attachLimits is the game's side of the worker's limits: none are needed from
// outside on these systems, the worker sets its own.
func attachLimits(cmd *exec.Cmd, limit uint64) (func(), error) { return nil, nil }

// setLimit stores v in a Rlimit field, which is int64 on some systems and
// uint64 on others.
func setLimit[T ~int64 | ~uint64](p *T, v uint64) { *p = T(v) }
