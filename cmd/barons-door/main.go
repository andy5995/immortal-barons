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
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func main() {
	dropPath := flag.String("dropfile", "", "path to the BBS dropfile (DOOR32.SYS or DOOR.SYS)")
	dataDir := flag.String("data", "./data", "game data directory")
	maint := flag.Bool("maint", false, "run daily maintenance and exit")
	setup := flag.Bool("setup", false, "interactively configure the game and exit")
	flag.Parse()

	cfg, err := store.LoadConfig(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	today := time.Now().Format("2006-01-02")

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
	return store.Save(w, cfg)
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
	if n, err := strconv.Atoi(line); err == nil {
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
