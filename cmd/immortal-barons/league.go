package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ftn"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
	"github.com/andy5995/immortal-barons/internal/textwrap"
)

// runLeagueReset is the Coordinator starting a new season for the whole league:
// this board resets, and a signed order goes out for every other board to do the
// same on its next planetary run.
func runLeagueReset(cfg game.Config, date string) error {
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	if err := w.DeclareLeagueReset(date, ""); err != nil {
		return err
	}
	run, err := store.RunPlanetary(w, cfg.Inbound(), cfg.Outbound(), false)
	if err != nil {
		return err
	}
	reportPlanetary(cfg, run)
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	fmt.Printf("Season %d declared, starting %s. The order is in the outbound folder for the other boards.\n", w.Season, date)
	return nil
}

// runLeagueReport writes one of the original's sysop report files into the data
// directory and says where it went. They are pure reads over what inbound
// packets already told this board, so none of them touches the world.
func runLeagueReport(cfg game.Config, which string) error {
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	var name, body string
	switch which {
	case "lastpacket":
		name, body = "LASTPACKET.LST", w.LastPacketReport()
	case "bbsinfo":
		name, body = "BBSINFO.LST", w.BBSInfoReport()
	case "playerlist":
		// The original restricts this one to the League Coordinator: it is the
		// whole league's player roll, not this board's business to publish.
		if !w.IsLeagueCoordinator() {
			return errors.New("only the League Coordinator (node 1) may write the league player list")
		}
		name, body = "PLAYERLIST.LST", w.PlayerListReport()
	default:
		return fmt.Errorf("unknown report %q", which)
	}
	path := filepath.Join(cfg.DataDir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote %s\n", path)
	return nil
}

// runLeagueCheck prints every league setup item at once and reports whether
// all of them are usable. A sysop joining a league gets a handful of settings
// exactly right or a transport run fails three steps from the cause (#154).
func runLeagueCheck(cfg game.Config) bool {
	allOK := true
	for _, c := range store.Checkup(cfg) {
		mark := "ok  "
		if !c.OK {
			mark, allOK = "FAIL", false
		}
		prefix := fmt.Sprintf("%s  %-20s", mark, c.Name)
		fmt.Println(prefix + textwrap.Wrap(c.Detail, textwrap.Console, strings.Repeat(" ", len(prefix))))
	}
	for _, c := range spoolChecks(cfg) {
		mark := "ok  "
		if !c.OK {
			mark, allOK = "FAIL", false
		}
		prefix := fmt.Sprintf("%s  %-20s", mark, c.Name)
		fmt.Println(prefix + textwrap.Wrap(c.Detail, textwrap.Console, strings.Repeat(" ", len(prefix))))
	}
	if !allOK {
		fmt.Println("\nFix the FAIL lines above; see the Door Setup guide for what each one wants.")
	}
	return allOK
}

// spoolChecks reports the FTN transport's own backlog, when there is one to
// report. It answers here rather than only in barons-ftn's output because the
// run that met a failure is long gone by the time a sysop asks why a board has
// gone quiet, and this is the command they are told to reach for (#228). A
// board with no transport spool has nothing to say and says nothing.
func spoolChecks(cfg game.Config) []store.Check {
	status, err := ftn.Status(cfg.DataDir)
	if err != nil {
		return []store.Check{{Name: "FTN spool", OK: false, Detail: err.Error()}}
	}
	var checks []store.Check
	for _, peer := range status.Peers {
		detail := fmt.Sprintf("%d snapshot(s) for %s, %s without progress",
			peer.Snapshots, peer.Name, peer.Oldest.Round(time.Minute))
		if peer.LastError != "" {
			detail += "; last failure: " + peer.LastError
		}
		// Waiting is not a setup fault: a peer can be legitimately offline for
		// days. It is reported so a sysop can see how long, not marked wrong.
		checks = append(checks, store.Check{Name: "Outbound waiting", OK: true, Detail: detail})
	}
	for _, receipt := range status.Inbound {
		checks = append(checks, store.Check{Name: "Inbound pending", OK: true,
			Detail: fmt.Sprintf("%s, %s: %s", receipt.ID, receipt.Age.Round(time.Minute), receipt.Reason)})
	}
	for _, dir := range status.Unreadable {
		checks = append(checks, store.Check{Name: "Unreadable journal", OK: false,
			Detail: dir + " — neither retry state nor quarantine, and nothing will retry it"})
	}
	if status.SetAside > 0 {
		checks = append(checks, store.Check{Name: "Set-aside packets", OK: true,
			Detail: fmt.Sprintf("%d in the transport's bad folder; nothing retries them", status.SetAside)})
	}
	return checks
}

// runLeagueRoutes reports where this board sends each planet's packets — BRE's
// "BRE TEST", whose whole job is letting a sysop see the routing the roster
// gives them before they wonder why nothing arrives.
func runLeagueRoutes(cfg game.Config) error {
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	planets := w.LeaguePlanets()
	if len(planets) == 0 {
		fmt.Printf("No league roster loaded (%s). Every packet goes straight to the board it names, in %s\n",
			filepath.Join(cfg.DataDir, store.NodeListFile), cfg.Outbound())
		return nil
	}
	fmt.Printf("This board: %s (node %d), league %s\n", cfg.BoardID, w.NodeNumber(cfg.BoardID), leagueLabel(cfg.LeagueNumber))
	if !w.Routed() {
		broadcastDir := cfg.Outbound()
		others := 0
		var elsewhere []string
		for _, p := range planets {
			if p.Name == cfg.BoardID {
				continue
			}
			others++
			// A Link line is the sysop saying this neighbour's files go in a
			// directory of its own, which is what a per-node queue needs. An
			// unaddressed broadcast is never written to one of those.
			if dir, ok := cfg.OutboundLink(w.NodeNumber(p.Name)); ok && dir != broadcastDir {
				elsewhere = append(elsewhere, p.Name)
			}
		}
		boards := fmt.Sprintf("the other %d boards", others)
		if others == 1 {
			boards = "the one other board"
		}
		// A league without HOST lines is a legitimate setup, so this is a
		// description and not a FAIL: only the sysop knows whether the
		// transport copies one file to everybody. Saying it here is what a
		// three-board rig needed four exchange cycles to notice (#168).
		fmt.Println(textwrap.Wrap(fmt.Sprintf("The roster carries no HOST routing, so this board links to every other one. "+
			"A broadcast is written unaddressed to %s, and your transport has to copy that one file to %s. "+
			"A per-node queue, such as a binkp file box or an FTN file attach, delivers it to one board unless a "+
			"connector in front of it fans the file out. See \"How packets move\" in the Inter-BBS guide.",
			broadcastDir, boards), textwrap.Console, ""))
		if len(elsewhere) > 0 {
			body := fmt.Sprintf("%s are handed packets in directories of their own, and a broadcast is written "+
				"to none of them. Nothing this board writes carries one to them unless something copies it out of %s.",
				strings.Join(elsewhere, ", "), broadcastDir)
			if len(elsewhere) == 1 {
				body = fmt.Sprintf("%s is handed packets in a directory of its own, and a broadcast is not written "+
					"there. Nothing this board writes carries one to it unless something copies it out of %s.",
					elsewhere[0], broadcastDir)
			}
			const prefix = "warning: "
			fmt.Print(prefix, textwrap.Wrap(body, textwrap.Console-len(prefix), strings.Repeat(" ", len(prefix))), "\n")
		}
	}
	fmt.Printf("\n%-24s %-24s %s\n", "PLANET", "HANDED TO", "WRITTEN IN")
	for _, p := range planets {
		if p.Name == cfg.BoardID {
			continue
		}
		hop := w.NextHop(p.Name)
		dir, ok := cfg.OutboundLink(w.NodeNumber(hop))
		if !ok {
			dir = cfg.Outbound()
		}
		fmt.Printf("%-24s %-24s %s\n", p.Name, hop, dir)
	}
	return nil
}

// leagueLabel names the league for a sysop. The number is the whole of it: it
// is what routes, and it is what tells two leagues sharing an inbound directory
// apart.
func leagueLabel(n int) string {
	if n <= 0 {
		return "(number not set)"
	}
	return fmt.Sprintf("#%d", n)
}

// runLeagueConfig broadcasts this board's league rules (turns, protection,
// game length) to the league. Only the League Coordinator (node #1 in the
// roster) may author it; member boards adopt it on their next PLANETARY run.
func runLeagueConfig(cfg game.Config) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	if !w.IsLeagueCoordinator() {
		return fmt.Errorf("this board (%q) is not the League Coordinator (node #1 in %s)", cfg.BoardID, store.NodeListFile)
	}
	w.ExportLeagueConfig()
	// Sign and number it. Every other outbound path reaches this through
	// RunPlanetary; this one writes the outbox directly, and an unsigned ruleset
	// is refused by every board that receives it.
	w.StampOutbox()
	if _, err := store.WriteOutbox(w, cfg.Outbound(), false); err != nil {
		return err
	}
	// A routed league writes one copy per member, each to its own link, so
	// naming the default directory would name where most of them did not go.
	where := cfg.Outbound()
	if len(cfg.OutboundDirs) > 0 {
		where = "every board in the league"
	}
	fmt.Printf("Broadcast league config (turns/day=%d, protection=%d, length=%d) to %s\n",
		cfg.TurnsPerDay, cfg.ProtectionTurns, cfg.GameLength, where)
	return store.Save(w, cfg)
}

// leagueSetup is the per-board half of a league reset (-ibbs-reset): the
// settings no Coordinator can broadcast, because they name this board and its
// own directories. A nil *leagueSetup means a stand-alone board. Naming the
// board on the command line skips the settings editor, so a member sysop sets
// up in one command and takes the ruleset from the Coordinator's next
// broadcast.
type leagueSetup struct {
	BoardID    string
	Inbound    string
	Outbound   string
	ImportPath string // an original BRE BBS.CFG to take this board's identity from
}

// importBoardConfig takes what an original BRE BBS.CFG can tell us into cfg.
// A sysop converting a league they already run has typed all of this once
// already, and the node numbers and directories are exactly what is tedious to
// re-enter correctly.
//
// Three of BRE's seven lines have no counterpart here and are left behind: the
// sysop's name, the FTN address, and the mailer's name, none of which IB uses
// because it addresses nothing and writes no netmail. The netmail directory is
// deliberately NOT read as the outbound directory — BRE puts .MSG files there,
// while IB's outbound holds the packets themselves, so the two mean different
// things despite sitting next to each other in the original.
// Returns the board name it read, so the caller can treat an imported name the
// same as one given with -board-id.
func importBoardConfig(path string, cfg *game.Config) (string, error) {
	bc, err := store.ParseBoardConfig(path)
	if err != nil {
		return "", err
	}
	var took []string
	if bc.PlanetName != "" {
		cfg.BoardID = bc.PlanetName
		took = append(took, fmt.Sprintf("board name %q", bc.PlanetName))
	}
	if bc.InboundDir != "" {
		cfg.InboundDir = bc.InboundDir
		took = append(took, "inbound directory "+bc.InboundDir)
	}
	if bc.League > 0 {
		cfg.LeagueNumber = bc.League
		took = append(took, fmt.Sprintf("league number %d", bc.League))
	}
	if len(took) == 0 {
		return "", fmt.Errorf("%s holds none of the settings this reads", path)
	}
	// Printed rather than assumed: the file is positional, so a line out of
	// place produces a plausible-looking wrong answer, and the sysop is the only
	// one who can tell.
	fmt.Printf("From %s: %s\n", path, strings.Join(took, ", "))
	return bc.PlanetName, nil
}

// runGenCoordKey creates this league's Coordinator key and prints the line the
// other boards run to record its public half.
func runGenCoordKey(cfg game.Config) {
	pub, err := store.GenerateCoordKey(cfg.DataDir)
	if err != nil {
		if os.IsExist(err) {
			fmt.Fprintln(os.Stderr, "immortal-barons -gen-coord-key: this board already has a Coordinator key. Delete", filepath.Join(cfg.DataDir, store.CoordKeyFile), "only if you mean to lock every other board out.")
		} else {
			fmt.Fprintln(os.Stderr, "immortal-barons -gen-coord-key:", err)
		}
		os.Exit(1)
	}
	fmt.Println("Coordinator key created. Give every other board in the league this line:")
	fmt.Println()
	fmt.Println("    immortal-barons -coord-key", pub)
	fmt.Println()
	fmt.Println("Keep", filepath.Join(cfg.DataDir, store.CoordKeyFile), "secret. Copying it is how coordinatorship is handed on.")
}

// runGenBoardKey creates this board's packet-signing key and prints the public
// half for the League Coordinator to put on the roster.
func runGenBoardKey(cfg game.Config) {
	pub, err := store.GenerateBoardKey(cfg.DataDir)
	if err != nil {
		if os.IsExist(err) {
			fmt.Fprintln(os.Stderr, "immortal-barons -gen-board-key: this board already has a signing key. Replacing it makes every packet this board sends fail its neighbours' checks until the Coordinator publishes the new one, so delete", filepath.Join(cfg.DataDir, store.BoardKeyFile), "only if you mean to do that.")
		} else {
			fmt.Fprintln(os.Stderr, "immortal-barons -gen-board-key:", err)
		}
		os.Exit(1)
	}
	// The key is printed alone, with the board named in the prose around it
	// rather than on the same line. It goes on the roster's seventh line by
	// itself, and a line carrying anything else fails to decode — which
	// leaves that board unchecked instead of reporting an error, so the
	// output must not invite pasting a board name along with it.
	fmt.Printf("Board signing key created for %s.\n", cfg.BoardID)
	fmt.Println()
	fmt.Println("Send your League Coordinator this key:")
	fmt.Println()
	fmt.Println("   ", pub)
	fmt.Println()
	fmt.Println("They add it to this board's entry in the league roster, as a seventh line")
	fmt.Println("with the key on it and nothing else. The roster is signed and broadcast, so")
	fmt.Println("every board can then check that a packet naming yours really came from it.")
	fmt.Println("Keep", filepath.Join(cfg.DataDir, store.BoardKeyFile), "secret; anyone holding it can send packets as this board.")
}
