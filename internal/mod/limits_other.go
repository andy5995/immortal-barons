//go:build !(linux || darwin || freebsd || netbsd || openbsd || dragonfly || windows)

package mod

import "os/exec"

// No system limits here: the worker's heap watchdog and the game's timeout are
// all there is.
func workerSelfLimit(limit uint64)                             {}
func workerArmCPU()                                            {}
func attachLimits(cmd *exec.Cmd, limit uint64) (func(), error) { return nil, nil }
