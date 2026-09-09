package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// faultHookTimeout bounds the sysop's command. A planetary run is on a timer
// and holds nothing open, but a hook that blocks — a mail command waiting on a
// dead relay, a webhook with no route — would stall every later run behind it.
const faultHookTimeout = 30 * time.Second

// runFaultHook runs the command the sysop put in bbs.cfg's OnFault, once, after
// a run that recorded a fault it had not already reported (#187).
//
// Through the platform's shell on purpose: what a sysop wants here is their own
// one-liner — a mail command, a curl to a push service, a chat webhook — and
// most of those are pipelines rather than a bare program. The summary reaches it
// two ways, because the two platforms differ on what is convenient: as $IB_FAULTS
// (both) and as the first argument (Unix, where sh -c takes them).
//
// A failing hook is reported and never fatal. It is the alarm, not the work: a
// run that moved the league's mail must not be called a failure because a push
// service was down.
func runFaultHook(cfg game.Config, faults []string) {
	command := strings.TrimSpace(cfg.OnFault)
	if command == "" || len(faults) == 0 {
		return
	}
	summary := strings.Join(faults, " | ")
	ctx, cancel := context.WithTimeout(context.Background(), faultHookTimeout)
	defer cancel()
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		c = exec.CommandContext(ctx, "sh", "-c", command, "sh", summary)
	}
	c.Env = append(os.Environ(),
		"IB_FAULTS="+summary,
		"IB_BOARD="+cfg.BoardID,
		"IB_DATA="+cfg.DataDir,
	)
	out, err := c.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "OnFault command failed: %v\n", err)
	}
	if len(out) > 0 {
		fmt.Printf("OnFault said: %s\n", strings.TrimSpace(string(out)))
	}
}
