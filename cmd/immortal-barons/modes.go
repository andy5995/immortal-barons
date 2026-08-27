package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
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

// runMaint blocks on the lock (waits for any active player) then advances
// the world.
func runMaint(cfg game.Config, today string) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	switch r := w.DailyMaintenance(today); {
	case r.NotStarted:
		fmt.Println("The game has not started yet; maintenance did not advance the world.")
	case r.Days > 0:
		fmt.Printf("Daily maintenance ran: advanced %d day(s) to game day %d.\n", r.Days, w.GameDay)
	default:
		fmt.Println("Maintenance has already been run today.")
	}
	if cfg.IBBS {
		run, err := store.RunPlanetary(w, cfg.Inbound(), cfg.Outbound(), false)
		if err != nil {
			return err
		}
		reportPlanetary(cfg, run)
	} else if _, err := store.SyncBulletins(w); err != nil {
		// A league board reconciles its bulletins inside the planetary step
		// above; a stand-alone one has no such step, so this is where a bulletin
		// the sysop added reaches the news.
		return err
	}
	return store.Save(w, cfg)
}

// runPlanetary runs the inter-BBS maintenance step on its own (BRE's
// "BRE PLANETARY"): apply inbound packets, launch due group attacks, export
// scores, and write the outbox. Can run several times a day.
func runPlanetary(cfg game.Config, verbose bool) error {
	if cfg.InterBBSEnabled() {
		if err := store.CheckLeagueNumber(cfg); err != nil {
			return err
		}
	}
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	run, err := store.RunPlanetary(w, cfg.Inbound(), cfg.Outbound(), verbose)
	if err != nil {
		return err
	}
	reportPlanetary(cfg, run)
	return store.Save(w, cfg)
}

// reportPlanetary says what the inter-BBS step did. Silence is the wrong answer
// for a command whose whole job is moving mail: a sysop cannot tell a run that
// had nothing to do from one that read the wrong directory.
func reportPlanetary(cfg game.Config, run store.PlanetaryRun) {
	skipped := run.OtherLeague + run.MeshCopy + run.AlreadySeen + run.Refused + run.Held + run.Quarantined + run.Deferred
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
		fmt.Printf("Released %d held packet(s): this board can now read their format.\n", run.Released)
	}
	if run.Held > 0 {
		// Not "once the builds match": that is true only of a packet from a
		// NEWER board, which upgrading here releases. One from an older board
		// is held by a format this build has moved past and no upgrade of
		// theirs brings it back, so the per-board notices say which is which.
		fmt.Printf("Held %d packet(s) for a protocol this build cannot read; they are in %s. The notes below say, per board, whether upgrading releases them.\n",
			run.Held, filepath.Join(cfg.DataDir, store.HeldDir))
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
	if run.Bulletins == 1 {
		fmt.Println("Broadcast 1 league bulletin to the league.")
	} else if run.Bulletins > 1 {
		fmt.Printf("Broadcast %d league bulletins to the league.\n", run.Bulletins)
	}
	// Where the files went is only one directory when this board has no
	// per-neighbour links; naming it otherwise would be wrong for most of them.
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
	skipped := run.OtherLeague + run.MeshCopy + run.AlreadySeen + run.Refused + run.Held + run.Quarantined + run.Deferred
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
	// Held ranks next: nothing is lost, but the league is out of step and
	// somebody has to act before those packets move.
	if run.Held > 0 {
		parts = append(parts, fmt.Sprintf("%d held for a protocol this build does not read", run.Held))
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
	pkt := "skipped"
	if skipped == 1 {
		pkt = "skipped 1:"
	}
	return fmt.Sprintf("%s %d: %s", pkt, skipped, strings.Join(parts, ", "))
}

// runFull chains the three steps a sysop's batch file runs: inbound, play,
// outbound (BRE's "BRE FULL"). It requires either -local with a name or a BBS
// drop file to identify the caller for the play step.
func runFull(cfg game.Config, name, today string, cs charset, noANSI, verbose bool) error {
	// Step 1: read inbound packets.
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

	// Step 2: play a turn. Detect whether we have -local with a name or a drop
	// file to identify the caller.
	if strings.TrimSpace(name) != "" {
		// -local path: play locally.
		c := session.NewConsole()
		defer c.Close()
		if noANSI {
			c.SetPlain()
		}
		s := encodeFor(session.Session(c), cs)
		if _, err := play.Run(s, play.Identity{Handle: name}, cfg, today); err != nil {
			return err
		}
		fmt.Fprint(s, "\nUntil next turn, Baron.\n")
	} else {
		// Door path: try to find a drop file.
		doorCfg, derr := store.LoadDoorConfig(cfg.DataDir)
		if derr != nil {
			return fmt.Errorf("could not read door config: %w", derr)
		}
		if doorCfg.DropfileFormat == "" {
			return fmt.Errorf("requires -local or a BBS drop file (run -set-dropfile first)")
		}
		path := findDropfile(doorCfg.DropfileFormat)
		if path == "" {
			return fmt.Errorf("requires -local or a BBS drop file (no %s found in the working directory)", doorCfg.DropfileFormat)
		}
		caller, cerr := door.ParseDropfileAs(path, doorCfg.DropfileFormat)
		if cerr != nil {
			return fmt.Errorf("drop file: %w", cerr)
		}
		s, closeSession, serr := openSession(caller)
		if serr != nil {
			return serr
		}
		defer closeSession()
		s = encodeFor(s, wantCharset(false, false, false, false))
		if !caller.ANSI || noANSI {
			s = session.NewPlain(s)
		}
		handle := caller.Handle
		if handle == "" {
			handle = fmt.Sprintf("node%d", caller.Node)
		}
		id := play.Identity{Handle: handle, TimeLeft: time.Duration(caller.SecondsLeft) * time.Second}
		if _, err := play.Run(s, id, cfg, today); err != nil {
			return err
		}
	}

	// Step 3: write outbound packets.
	lock, err = store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err = store.Load(cfg)
	if err != nil {
		return err
	}
	w.StampOutbox()
	sent, err := store.WriteOutbox(w, cfg.Outbound(), verbose)
	if err != nil {
		return err
	}
	if err := store.Save(w, cfg); err != nil {
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
	return nil
}
