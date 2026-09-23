package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ftn"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
	"github.com/andy5995/immortal-barons/internal/textwrap"
)

// The FTN transport runs inside the inter-BBS modes, where the original runs
// its own: unwrap what the mailer brought before the planetary step reads the
// game's inbound, hand off what the step wrote after the world is saved. That
// is BRE PLANETARY's order (inbound, then outbound) and BRE FULL's (inbound,
// play, outbound). A plain door session moves no league mail, as in BRE.
//
// Both halves take the transport's own lock, never while the world lock is
// held: the world lock is what every caller's next action queues on, and a
// mailer handoff has no business holding them up.

// checkTransportSettings refuses a board whose settings are spelled the way an
// older version wrote them: bbs.cfg's Inbound, Outbound and two-field Link, and
// a separate ftn.cfg. Each refusal carries the replacement lines. It is asked by
// the modes that move packets, and not by a door session, which reads neither.
func checkTransportSettings(cfg game.Config) error {
	var msgs []string
	for _, err := range []error{store.LegacyBoardRefusal(cfg.DataDir, ftn.IsLink), ftn.LegacyRefusal(cfg.DataDir)} {
		if err != nil {
			msgs = append(msgs, err.Error())
		}
	}
	if len(msgs) > 0 {
		return errors.New(strings.Join(msgs, "\n\n"))
	}
	for _, w := range store.BoardWarnings(cfg.DataDir, ftn.Keys()) {
		transportWarn(w)
	}
	return nil
}

// transportIn unwraps what the mailer delivered into the game's inbound, and
// then any bundle that was delivered straight into the game's inbound. It never
// fails the run it is part of: whatever is already in the game's inbound is
// applied either way, and a fault in the mailer's directory is not a reason to
// hold those back. With wait false it gives up at once when another run holds
// the transport lock -- that run is doing this same work.
func transportIn(cfg game.Config, wait bool, out io.Writer) {
	tc, err := ftn.LoadConfig(cfg.DataDir)
	if err != nil {
		transportWarn(fmt.Sprintf("FTN unwrap skipped: %v. Applying what is already in %s.", err, cfg.Inbound()))
	} else if tc.Receives() && mailerIn(cfg, wait, out) {
		return
	}
	gameInboundIn(cfg, wait, out)
}

// mailerIn unwraps what the mailer delivered into IncomingFileDir, reporting
// whether it gave way to another run holding the transport lock.
func mailerIn(cfg game.Config, wait bool, out io.Writer) (busy bool) {
	run := ftn.RunIn
	if !wait {
		run = ftn.TryRunIn
	}
	result, err := run(cfg.DataDir)
	if errors.Is(err, store.ErrBusy) {
		fmt.Fprintln(out, "FTN unwrap skipped: another run holds the transport lock and is doing it.")
		return true
	}
	printTransportWarnings(result)
	if err != nil {
		transportWarn(fmt.Sprintf("FTN unwrap failed: %v. Applying what is already in %s.", err, cfg.Inbound()))
		return false
	}
	for _, queued := range result.Queued {
		fmt.Fprintf(out, "Forwarded %s for %s (%s) as %s\n",
			queued.PacketPath, queued.NextHop, queued.Address, queued.Message)
	}
	if result.Delivered > 0 {
		fmt.Fprintf(out, "Unwrapped %d packet(s) from the mailer's inbound.\n", result.Delivered)
	}
	return false
}

// gameInboundIn unwraps a bundle that reached the game's inbound without
// passing through IncomingFileDir: the mailer of a board with no FTN settings
// delivers there, and a bundle is otherwise unreadable to the planetary step
// (#230). It needs no FTN settings, and does nothing when no bundle is there.
func gameInboundIn(cfg game.Config, wait bool, out io.Writer) {
	run := ftn.UnwrapGameInbound
	if !wait {
		run = ftn.TryUnwrapGameInbound
	}
	result, err := run(cfg.DataDir)
	if errors.Is(err, store.ErrBusy) {
		fmt.Fprintf(out, "Unwrapping the bundles in %s skipped: another run holds the transport lock and is doing it.\n", cfg.Inbound())
		return
	}
	printTransportWarnings(result)
	if err != nil {
		transportWarn(fmt.Sprintf("Could not unwrap the transport bundles in %s: %v. They stay there for the next run.", cfg.Inbound(), err))
		return
	}
	if result.Delivered > 0 {
		fmt.Fprintf(out, "Unwrapped %d packet(s) from transport bundles in %s.\n", result.Delivered, cfg.Inbound())
	}
}

// transportOut hands what the game wrote to the mailer. Its error is the
// caller's to raise: a handoff that fails leaves the league's mail sitting in
// the game's outbound, which is exactly what a scheduler's alarm is for.
func transportOut(cfg game.Config, wait bool, out io.Writer) error {
	tc, err := ftn.LoadConfig(cfg.DataDir)
	if err != nil {
		return err
	}
	if !tc.Sends() {
		return nil
	}
	run := ftn.RunOut
	if !wait {
		run = ftn.TryRunOut
	}
	result, err := run(cfg.DataDir)
	if errors.Is(err, store.ErrBusy) {
		fmt.Fprintln(out, "FTN handoff skipped: another run holds the transport lock and is doing it.")
		return errHandoffSkipped
	}
	printTransportWarnings(result)
	for _, queued := range result.Queued {
		fmt.Fprintf(out, "Queued %s for %s (%s) as %s\n",
			queued.PacketPath, queued.NextHop, queued.Address, queued.Message)
	}
	if result.Snapshots > 0 {
		// An empty system and a stalled one both queue nothing, and a scheduled
		// run's log is the only place a sysop would see the difference (#228).
		fmt.Fprintf(out, "FTN: %d queued; %d snapshot(s) still waiting on %d peer(s): %s (oldest %s without progress)\n",
			len(result.Queued), result.Snapshots, len(result.Waiting),
			strings.Join(result.Waiting, ", "), result.OldestWait.Round(time.Minute))
		for _, stalled := range result.Stalled {
			fmt.Fprintf(out, "  %s\n", stalled)
		}
	}
	return err
}

// errHandoffSkipped is transportOut giving way to another run holding the
// transport lock. That run's outcome is the one to record, so a skip neither
// raises nor clears the alarm.
var errHandoffSkipped = errors.New("handoff skipped")

// handoff runs the handoff and returns the fault line for reportFaults when it
// failed in a way the sysop has not been alarmed about. A failure that repeats
// unchanged is still printed on every run, but alarms only once, as the
// planetary step's faults do (#187); it alarms again when its text changes, or
// once a run has succeeded in between.
func handoff(cfg game.Config, wait bool, out io.Writer) []string {
	err := transportOut(cfg, wait, out)
	if errors.Is(err, errHandoffSkipped) {
		return nil
	}
	fault := ""
	if err != nil {
		fault = "FTN handoff failed: " + err.Error()
	}
	fresh, recErr := ftn.NoteHandoffFault(cfg.DataDir, fault)
	if recErr != nil {
		// Unrecorded, it cannot be told apart from a new one: alarm rather
		// than risk staying silent about it.
		transportWarn(fmt.Sprintf("could not record the handoff's outcome: %v", recErr))
		fresh = fault != ""
	}
	if fault == "" {
		return nil
	}
	if !fresh {
		transportWarn(fault + " (unchanged since it was reported)")
		return nil
	}
	return []string{fault}
}

func transportWarn(msg string) {
	const prefix = "immortal-barons: warning: "
	fmt.Fprint(os.Stderr, prefix, textwrap.Wrap(msg, textwrap.Console, strings.Repeat(" ", len(prefix))), "\n")
}

func printTransportWarnings(result ftn.Result) {
	for _, warning := range result.Warnings {
		transportWarn(warning)
	}
}

// runFTNStatus answers the question a file count cannot: not how many files
// are in the spool, but what is unfinished, for whom, for how long, and why. It
// reads and prints; a sysop reaching for it is trying to find out what is
// happening, which is the worst moment to move anything (#228).
func runFTNStatus(cfg game.Config) error {
	// Settings under old names are reported, not refused: an upgrade is when a
	// sysop most needs to see what the spools are holding.
	if err := checkTransportSettings(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "immortal-barons -ftn-status: %v\n\n", err)
	}
	status, err := ftn.Status(cfg.DataDir)
	if err != nil {
		return err
	}
	if len(status.Peers) == 0 {
		fmt.Println("Outbound: nothing waiting.")
	} else {
		fmt.Println("Outbound, longest wait first:")
		for _, peer := range status.Peers {
			fmt.Printf("  %-24s %d snapshot(s), %s without progress\n",
				peer.Name, peer.Snapshots, peer.Oldest.Round(time.Minute))
			if peer.LastError != "" {
				fmt.Printf("  %-24s last failure: %s\n", "", peer.LastError)
			}
		}
		fmt.Println("  A snapshot is kept whole until every target in it publishes, so it")
		fmt.Println("  also holds bundles for peers that already went out.")
	}
	if len(status.Inbound) == 0 {
		fmt.Println("Inbound: nothing pending.")
	} else {
		fmt.Println("Inbound, longest wait first:")
		for _, receipt := range status.Inbound {
			fmt.Printf("  %-24s %s: %s\n", receipt.ID, receipt.Age.Round(time.Minute), receipt.Reason)
		}
	}
	for _, dir := range status.Unreadable {
		fmt.Printf("Unreadable journal in %s: neither retry state nor quarantine, and nothing will retry it.\n", dir)
	}
	if len(status.Unclaimed) > 0 {
		fmt.Println("Unclaimed in the mailer's inbound, longest wait first:")
		for _, waiting := range status.Unclaimed {
			fmt.Printf("  %-24s %s: %s\n",
				filepath.Base(waiting.Path), waiting.Age.Round(time.Minute), waiting.Where())
		}
		fmt.Println("  These are in neither spool, so nothing above counts them and nothing")
		fmt.Println("  will retry them on its own.")
	}
	if status.SetAside > 0 {
		fmt.Printf("Set aside: %d packet(s) nothing retries; read and clear them once the cause is fixed.\n", status.SetAside)
	}
	return nil
}
