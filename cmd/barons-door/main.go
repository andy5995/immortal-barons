// Command barons-door runs Immortal Barons as a native BBS door. Normal
// mode reads the caller's dropfile and plays over stdio. With -maint it runs
// daily maintenance non-interactively (for the sysop's nightly event).
//
// Configure your BBS to run it with the dropfile path, e.g.:
//
//	barons-door -dropfile /path/to/node/DOOR32.SYS
//
// With no -dropfile it looks for DOOR32.SYS or DOOR.SYS in the working directory.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/ibbs"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func main() {
	dropPath := flag.String("dropfile", "", "path to the BBS dropfile (DOOR32.SYS or DOOR.SYS)")
	dataDir := flag.String("data", "./data", "game data directory")
	maint := flag.Bool("maint", false, "run daily maintenance and exit")
	setup := flag.Bool("setup", false, "interactively configure the game and exit")
	export := flag.String("export", "", "write this board's score packet to FILE and exit")
	imp := flag.String("import", "", "import a score packet from FILE and exit")
	planetary := flag.Bool("planetary", false, "run the inter-BBS PLANETARY step (read inbound, launch attacks, write outbound) and exit")
	leagueConfig := flag.Bool("league-config", false, "broadcast this board's league settings to the league (coordinator/node #1 only) and exit")
	flag.Parse()

	cfg, err := store.LoadConfig(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	today := time.Now().Format("2006-01-02")

	if *export != "" {
		if err := runExport(cfg, *export, today); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -export:", err)
			os.Exit(1)
		}
		return
	}

	if *imp != "" {
		if err := runImport(cfg, *imp); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -import:", err)
			os.Exit(1)
		}
		return
	}

	if *setup {
		if err := runSetup(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -setup:", err)
			os.Exit(1)
		}
		return
	}

	if *maint {
		if err := runMaint(cfg, today); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -maint:", err)
			os.Exit(1)
		}
		return
	}

	if *planetary {
		if err := runPlanetary(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -planetary:", err)
			os.Exit(1)
		}
		return
	}

	if *leagueConfig {
		if err := runLeagueConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "barons-door -league-config:", err)
			os.Exit(1)
		}
		return
	}

	path := *dropPath
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	if path == "" {
		path = findDropfile()
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "barons-door: no dropfile found; pass -dropfile PATH")
		os.Exit(2)
	}
	caller, err := door.ParseDropfile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "barons-door:", err)
		os.Exit(1)
	}

	s := session.NewStdio()
	if caller.SecondsLeft > 0 {
		go func() {
			time.Sleep(time.Duration(caller.SecondsLeft) * time.Second)
			fmt.Fprint(s, "\r\n\r\nYour BBS time is up. Farewell, Baron!\r\n")
			os.Exit(0)
		}()
	}
	handle := caller.Handle
	if handle == "" {
		handle = fmt.Sprintf("node%d", caller.Node)
	}
	if err := play.Run(s, play.Identity{Handle: handle}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "barons-door:", err)
		os.Exit(1)
	}
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
	w.DailyMaintenance(today)
	if cfg.IBBS {
		if err := store.RunPlanetary(w, cfg.InboundDir, cfg.OutboundDir); err != nil {
			return err
		}
	}
	return store.Save(w, cfg)
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
	if err := store.WriteOutbox(w, cfg.OutboundDir); err != nil {
		return err
	}
	fmt.Printf("Broadcast league config (turns/day=%d, protection=%d, length=%d) to %s\n",
		cfg.TurnsPerDay, cfg.ProtectionTurns, cfg.GameLength, cfg.OutboundDir)
	return store.Save(w, cfg)
}

// runPlanetary runs the inter-BBS maintenance step on its own (BRE's
// "BRE PLANETARY"): apply inbound packets, launch due group attacks, export
// scores, and write the outbox. Can run several times a day.
func runPlanetary(cfg game.Config) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	if err := store.RunPlanetary(w, cfg.InboundDir, cfg.OutboundDir); err != nil {
		return err
	}
	return store.Save(w, cfg)
}

// runExport writes this board's alive-empire scores to path as an inter-BBS
// packet, for a sysop's mailer to carry to another board.
func runExport(cfg game.Config, path, today string) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	packet := ibbs.Packet{BoardID: cfg.BoardID, Date: today}
	for _, e := range w.Empires {
		if !e.Alive {
			continue
		}
		packet.Scores = append(packet.Scores, ibbs.Score{
			Empire:   e.Name,
			NetWorth: w.NetWorth(e),
			Land:     e.Land,
		})
	}
	if err := ibbs.Write(path, packet); err != nil {
		return err
	}
	fmt.Printf("Exported %d scores to %s\n", len(packet.Scores), path)
	return nil
}

// runImport reads an inter-BBS packet from path and records it as a
// RemoteBoard in this board's world.
func runImport(cfg game.Config, path string) error {
	packet, err := ibbs.Read(path)
	if err != nil {
		return err
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
	board := game.RemoteBoard{BoardID: packet.BoardID, Date: packet.Date}
	for _, sc := range packet.Scores {
		board.Scores = append(board.Scores, game.RemoteScore{
			Empire:   sc.Empire,
			NetWorth: sc.NetWorth,
			Land:     sc.Land,
		})
	}
	w.ImportBoard(board)
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	fmt.Printf("Imported board %s (%d scores)\n", board.BoardID, len(board.Scores))
	return nil
}

// runSetup interactively prompts the sysop for game rules and saves them to
// config.json.
func runSetup(cfg game.Config) error {
	r := bufio.NewReader(os.Stdin)
	cfg.TurnsPerDay = askInt(r, "Turns per day", cfg.TurnsPerDay)
	cfg.ProtectionTurns = askInt(r, "New-realm protection turns", cfg.ProtectionTurns)
	cfg.AICount = askInt(r, "Number of AI barons (0 = human only)", cfg.AICount)
	cfg.GameLength = askInt(r, "Game length in days (0 = endless)", cfg.GameLength)
	if err := store.SaveConfig(cfg); err != nil {
		return err
	}
	fmt.Printf("Saved configuration to %s\n", filepath.Join(cfg.DataDir, "config.json"))
	return nil
}

// askInt prompts with the current value in brackets; empty input keeps it.
func askInt(r *bufio.Reader, label string, cur int) int {
	fmt.Printf("%s [%d]: ", label, cur)
	line, _ := r.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return cur
	}
	if n, err := strconv.Atoi(line); err == nil && n >= 0 {
		return n
	}
	return cur
}

func findDropfile() string {
	for _, n := range []string{"door32.sys", "DOOR32.SYS", "door.sys", "DOOR.SYS"} {
		if _, err := os.Stat(n); err == nil {
			return n
		}
	}
	return ""
}
