package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/store"
	"github.com/andy5995/immortal-barons/internal/textwrap"
)

// runDump prints the loaded game world as indented JSON to stdout — a read-only
// snapshot for scripting and balance checks (pipe to jq). It reads the last
// committed world.json (written atomically), so it needs no lock and doesn't
// block players; derived figures (net worth, tech factor, income) can be worked
// out from the persisted fields it shows.
func runDump(cfg game.Config) error {
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

// runMaint waits for the world lock, which a caller's session holds only for
// one save at a time, then advances the world. On a league board it also runs
// the planetary step, with the FTN transport on either side of it (see
// transport.go).
func runMaint(cfg game.Config, today string) error {
	// A league board runs the planetary step below, so it is refused on the same
	// terms as -planetary: with no league number it would take every league's
	// packets as its own.
	if cfg.InterBBSEnabled() {
		if err := store.CheckLeagueNumber(cfg); err != nil {
			return err
		}
		if err := checkTransportSettings(cfg); err != nil {
			return err
		}
		transportIn(cfg, true, os.Stdout)
	}
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	w, err := store.Load(cfg)
	if err != nil {
		lock.Release()
		return err
	}
	switch r := w.DailyMaintenance(today); {
	case r.NotStarted:
		fmt.Println("The game has not started yet; maintenance did not advance the world.")
	case r.Frozen:
		fmt.Println("The league is frozen; maintenance did not advance the world.")
	case r.Days > 0:
		fmt.Printf("Daily maintenance ran: advanced %d day(s) to game day %d.\n", r.Days, w.GameDay)
	default:
		fmt.Println("Maintenance has already been run today.")
	}
	var run store.PlanetaryRun
	if cfg.InterBBSEnabled() {
		run, err = store.RunPlanetary(w, cfg.Inbound(), cfg.Outbound(), false)
		if err != nil {
			lock.Release()
			return err
		}
		reportPlanetary(cfg, run)
	} else if _, err := store.SyncBulletins(w); err != nil {
		// A league board reconciles its bulletins inside the planetary step
		// above; a stand-alone one has no such step, so this is where a bulletin
		// the sysop added reaches the news.
		lock.Release()
		return err
	}
	// Here as well as in the planetary step: a stand-alone board never runs that
	// one, and its scoreboard and news are as worth showing on a bulletin menu
	// as a league board's. It writes no World Report -- that one is the LEAGUE's
	// wars, and a board playing alone has no world to report on (#233).
	writeBulletins(cfg, w)
	err = store.Save(w, cfg)
	lock.Release()
	if err != nil {
		return err
	}
	if cfg.InterBBSEnabled() {
		// After the save and outside the world lock: a handoff that fails or
		// waits must not cost the work above or hold up the callers.
		run.NewFaults = append(run.NewFaults, handoff(cfg, true, os.Stdout)...)
	}
	// The same alarm as -planetary, and it has to be: the planetary step above
	// has already marked these faults as reported, so a later -planetary would
	// not raise them. -maint was silent here until #289, when it became the
	// command a league board's timer runs. A stand-alone board's run is empty
	// and raises nothing.
	return reportFaults(cfg, run, "-maint")
}

// runPlanetary runs the inter-BBS maintenance step on its own (BRE's
// "BRE PLANETARY"): unwrap what the mailer brought, apply inbound packets,
// launch due group attacks, export scores, write the outbox, and hand it to the
// mailer. Can run several times a day.
func runPlanetary(cfg game.Config, verbose bool) error {
	// The transport runs only for a league board, as in -maint and -full: a
	// board off the league is never refused over stale transport keys, so it
	// must not act on them either.
	if cfg.InterBBSEnabled() {
		if err := store.CheckLeagueNumber(cfg); err != nil {
			return err
		}
		if err := checkTransportSettings(cfg); err != nil {
			return err
		}
		transportIn(cfg, true, os.Stdout)
	}
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	w, err := store.Load(cfg)
	if err != nil {
		lock.Release()
		return err
	}
	run, err := store.RunPlanetary(w, cfg.Inbound(), cfg.Outbound(), verbose)
	if err != nil {
		lock.Release()
		return err
	}
	reportPlanetary(cfg, run)
	writeBulletins(cfg, w)
	err = store.Save(w, cfg)
	lock.Release()
	if err != nil {
		return err
	}
	if cfg.InterBBSEnabled() {
		run.NewFaults = append(run.NewFaults, handoff(cfg, true, os.Stdout)...)
	}
	// After the save, so a hook that hangs or a run that ends non-zero cannot
	// cost the work the run just did — and so the faults reported here are not
	// reported again by the next run.
	return reportFaults(cfg, run, "-planetary")
}

// reportFaults raises the alarm for a run that met a fault the sysop has not
// been told about: the command they configured, and a non-zero exit for the
// scheduler that started this. Both are deliberately keyed to NEW faults, not to
// faults outstanding: a board that has been unreachable for a week must not fail
// its unit every quarter of an hour, or the failure stops meaning anything.
func reportFaults(cfg game.Config, run store.PlanetaryRun, mode string) error {
	if len(run.NewFaults) == 0 {
		return nil
	}
	runFaultHook(cfg, run.NewFaults)
	// stderr, because that is what a scheduler mails and what a journal marks.
	fmt.Fprintf(os.Stderr, "immortal-barons %s: %s\n", mode, strings.Join(run.NewFaults, " "))
	return errFaults
}

// writeBulletins refreshes the files the BBS shows on its own bulletin menu.
// Reported but never fatal: a bulletin that cannot be written is worth saying
// out loud, and is not a reason to fail a run that moved the league's mail.
func writeBulletins(cfg game.Config, w *game.World) {
	for _, err := range menu.WriteBulletins(w, cfg.Bulletins()) {
		fmt.Printf("Could not write a bulletin file: %v\n", err)
	}
}

// reportPlanetary says what the inter-BBS step did. Silence is the wrong answer
// for a command whose whole job is moving mail: a sysop cannot tell a run that
// had nothing to do from one that read the wrong directory.
func reportPlanetary(cfg game.Config, run store.PlanetaryRun) {
	skipped := run.OtherLeague + run.MeshCopy + run.AlreadySeen + run.Refused + run.Held + run.HeldRules + run.Quarantined + run.Deferred + run.Bundles
	switch {
	case run.Applied == 0 && skipped == 0:
		fmt.Printf("No packets waiting in %s\n", cfg.Inbound())
	case run.Applied == 1 && skipped == 0:
		fmt.Printf("Applied 1 packet from %s\n", cfg.Inbound())
	case run.Applied > 1 && skipped == 0:
		fmt.Printf("Applied %d packets from %s\n", run.Applied, cfg.Inbound())
	case run.Applied == 0 && skipped > 0:
		fmt.Printf("No packets applied from %s (%s)\n", cfg.Inbound(),
			skipSummary(run))
	default:
		pkt := "packet"
		if run.Applied != 1 {
			pkt = "packets"
		}
		fmt.Printf("Applied %d %s from %s (%s)\n", run.Applied, pkt,
			cfg.Inbound(), skipSummary(run))
	}
	if run.Forwarded == 1 {
		fmt.Println("Passed 1 packet on towards the board it is addressed to.")
	} else if run.Forwarded > 1 {
		fmt.Printf("Passed %d packets on towards the boards they are addressed to.\n", run.Forwarded)
	}
	if run.Released > 0 {
		// Deliberately not "this board can now read their format": a packet is
		// held for several reasons now, and a release is only a return to the
		// inbound queue for re-checking — the run's own held count says whether
		// any of them were set aside again.
		fmt.Printf("Returned %d held packet(s) to the inbound queue to be checked again.\n", run.Released)
	}
	if run.Held > 0 {
		// Not "once the builds match": that is true only of a packet from a
		// NEWER board, which upgrading here releases. One from an older board
		// is held by a format this build has moved past and no upgrade of
		// theirs brings it back, so the per-board notices say which is which.
		fmt.Printf("Held %d packet(s) for a protocol this build cannot read; they are in %s. The notes below say, per board, whether upgrading releases them.\n",
			run.Held, filepath.Join(cfg.DataDir, store.HeldDir))
	}
	if run.HeldRules > 0 {
		fmt.Printf("Held %d packet(s) from a board that is not playing the league's rules; they are in %s. The notes below name it.\n",
			run.HeldRules, filepath.Join(cfg.DataDir, store.HeldDir))
	}
	if len(run.RecoveryPaused) > 0 {
		fmt.Println(textwrap.Wrap(store.RecoveryPausedNotice(cfg, run.RecoveryPaused), textwrap.Console, ""))
	}
	if run.Quarantined > 0 {
		fmt.Printf("Set aside %d packet(s) that could not be read at all; they are in %s.\n",
			run.Quarantined, filepath.Join(cfg.DataDir, store.BadDir))
	}
	if run.Deferred > 0 {
		fmt.Printf("Left %d packet(s) in inbound untouched: too young to trust as a complete write yet, will retry next run.\n",
			run.Deferred)
	}
	if run.RosterUpdated {
		fmt.Println("The League Coordinator's roster replaced this board's copy.")
	}
	for _, n := range run.Notices {
		fmt.Println(textwrap.Wrap("  "+n, textwrap.Console, "  "))
	}
	if len(run.Notices) > 0 {
		fmt.Printf("  (also recorded in %s)\n", filepath.Join(cfg.DataDir, store.PlanetaryLogFile))
	}
	if run.Frozen {
		fmt.Println("The league is frozen: this board sent nothing of its own.")
	}
	if run.Bulletins == 1 {
		fmt.Println("Broadcast 1 league bulletin to the league.")
	} else if run.Bulletins > 1 {
		fmt.Printf("Broadcast %d league bulletins to the league.\n", run.Bulletins)
	}
	// Where the files went is only one directory when this board has no
	// per-neighbor links; naming it otherwise would be wrong for most of them.
	where := " to " + cfg.Outbound()
	if len(cfg.OutboundDirs) > 0 {
		where = ""
	}
	switch run.Sent {
	case 0:
		fmt.Println("Nothing to send.")
	case 1:
		fmt.Printf("Wrote 1 packet%s\n", where)
	default:
		fmt.Printf("Wrote %d packets%s\n", run.Sent, where)
	}
}

// skipSummary returns a human-readable breakdown of why packets were skipped,
// e.g. "skipped 3: 2 already seen, 1 for another league". Each reason is shown
// only when its count is above zero.
func skipSummary(run store.PlanetaryRun) string {
	skipped := run.OtherLeague + run.MeshCopy + run.AlreadySeen + run.Refused + run.Held + run.HeldRules + run.Quarantined + run.Deferred + run.Bundles
	if skipped == 0 {
		return ""
	}
	var parts []string
	// First: a refusal is the one reason here that means something is wrong,
	// rather than a packet this board had no business with.
	if run.Refused > 0 {
		parts = append(parts, fmt.Sprintf("%d refused, not matching the sender's key", run.Refused))
	}
	// Quarantined ranks next: like a refusal, this is a file that needed
	// somebody's attention, not routine traffic that will resolve itself.
	if run.Quarantined > 0 {
		parts = append(parts, fmt.Sprintf("%d could not be read at all", run.Quarantined))
	}
	// Deferred ranks next: usually resolves itself by next run, but is
	// worth naming here too so it is not invisible for the (rare, and
	// itself worth noticing) run where the same file is still too young a
	// second time.
	if run.Deferred > 0 {
		parts = append(parts, fmt.Sprintf("%d left in place, too new to trust as complete", run.Deferred))
	}
	if run.Bundles > 0 {
		parts = append(parts, fmt.Sprintf("%d transport bundle(s) left for the next unwrap", run.Bundles))
	}
	// Held ranks next: nothing is lost, but the league is out of step and
	// somebody has to act before those packets move.
	if run.Held > 0 {
		parts = append(parts, fmt.Sprintf("%d held for a protocol this build does not read", run.Held))
	}
	if run.HeldRules > 0 {
		parts = append(parts, fmt.Sprintf("%d held: the sending board is not playing the league's rules", run.HeldRules))
	}
	if run.AlreadySeen > 0 {
		parts = append(parts, fmt.Sprintf("%d already seen", run.AlreadySeen))
	}
	if run.OtherLeague > 0 {
		parts = append(parts, fmt.Sprintf("%d for another league", run.OtherLeague))
	}
	if run.MeshCopy > 0 {
		parts = append(parts, fmt.Sprintf("%d mesh copy", run.MeshCopy))
	}
	return fmt.Sprintf("skipped %d: %s", skipped, strings.Join(parts, ", "))
}

// fullExchangeAllowed reports whether -full may run the league exchange around
// the caller's turn. Settings under an older release's names stop -planetary
// and -maint, where the sysop reads the exit code; here they would stop a caller
// from playing, so the exchange is skipped with the same message and the turn
// goes ahead. The exchange cannot run correctly until the lines are moved.
func fullExchangeAllowed(cfg game.Config, w io.Writer) bool {
	if !cfg.InterBBSEnabled() {
		return true
	}
	if err := checkTransportSettings(cfg); err != nil {
		fmt.Fprintf(w, "immortal-barons -full: %v\n\nThe league exchange is skipped until then; the caller plays as usual.\n", err)
		return false
	}
	return true
}

// runFull chains the three steps a sysop's batch file runs: inbound, play,
// outbound (BRE's "BRE FULL"). The play step is exactly what the same command
// line without -full runs: -local plays in the terminal, anything else is the
// door, drop file and all. It used to decide by whether a player name was set,
// but -name defaults to the OS user, so every door launch played on the BBS
// machine's console instead of the caller's connection.
func runFull(cfg game.Config, o *opts, today string, cs charset) error {
	verbose := *o.detailed
	// It runs the planetary step, so it is refused on the same terms as
	// -planetary: with no league number it would take every league's packets.
	if cfg.InterBBSEnabled() {
		if err := store.CheckLeagueNumber(cfg); err != nil {
			return err
		}
	}
	exchange := fullExchangeAllowed(cfg, os.Stderr)
	if exchange {
		if err := fullInbound(cfg, verbose); err != nil {
			return err
		}
	}

	if *o.local {
		// A play step that failed skips the outbound half, as a door that
		// fails does by exiting.
		if err := runLocal(cfg, *o.name, today, cs, *o.noANSI); err != nil {
			return err
		}
	} else {
		runDoor(cfg, o, today, cs)
	}

	if !exchange {
		return nil
	}
	return fullOutbound(cfg, verbose)
}

// fullInbound is -full's first step: unwrap what the mailer brought, then read
// the game's inbound. A caller is waiting behind it, so the transport never
// waits for its lock -- a run already holding it is doing the same work -- and
// what the transport says goes to stderr, the sysop's log, rather than onto
// the caller's screen.
func fullInbound(cfg game.Config, verbose bool) error {
	// The transport runs only for a league board, the same condition runFull
	// checks its settings under: a board off the league is never refused over
	// stale transport keys, so it must not act on them either.
	if cfg.InterBBSEnabled() {
		transportIn(cfg, false, os.Stderr)
	}
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	w, err := store.Load(cfg)
	if err != nil {
		lock.Release()
		return err
	}
	run, err := store.RunPlanetary(w, cfg.Inbound(), cfg.Outbound(), verbose)
	if err != nil {
		lock.Release()
		return err
	}
	if err := store.Save(w, cfg); err != nil {
		lock.Release()
		return err
	}
	lock.Release()
	reportPlanetary(cfg, run)
	// The hook, but not the exit code: this run has a caller waiting behind it,
	// and a door that exits non-zero on a league fault is a door the BBS reports
	// as broken to the player who just played it.
	runFaultHook(cfg, run.NewFaults)
	return nil
}

// fullOutbound is -full's last step: write the outbox, then hand it to the
// mailer. A failed handoff raises the hook and nothing else, for the reason
// fullInbound gives.
func fullOutbound(cfg game.Config, verbose bool) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	w, err := store.Load(cfg)
	if err != nil {
		lock.Release()
		return err
	}
	w.StampOutbox()
	sent, err := store.WriteOutbox(w, cfg.Outbound(), verbose)
	if err != nil {
		lock.Release()
		return err
	}
	err = store.Save(w, cfg)
	lock.Release()
	if err != nil {
		return err
	}
	switch sent {
	case 0:
		fmt.Println("Nothing to send.")
	case 1:
		fmt.Println("Wrote 1 outbound packet.")
	default:
		fmt.Printf("Wrote %d outbound packets.\n", sent)
	}
	if !cfg.InterBBSEnabled() {
		return nil
	}
	if faults := handoff(cfg, false, os.Stderr); len(faults) > 0 {
		fmt.Fprintf(os.Stderr, "immortal-barons -full: %s\n", faults[0])
		runFaultHook(cfg, faults)
	}
	return nil
}
