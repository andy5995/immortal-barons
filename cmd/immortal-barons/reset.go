package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// runReset is BRE's sysop reset: present the game-settings menu (the
// Configuration Editor) so the sysop sets up the new game, then wipe all
// empires (humans re-onboard on their next login), re-seed AI, and save. The
// old world is backed up first. It does not crown a winner.
//
// With fromConfig=true (-reset-from-config) it skips the editor and keeps the
// current config.json as-is. Either way the world is wiped and re-seeded.
func runReset(cfg game.Config, fromConfig bool, league *leagueSetup, cs charset, noANSI bool) error {
	// No drop file prompt here: a reset seeds the world for -local play and the
	// maintenance modes too, none of which reads a drop file. The door names
	// -set-dropfile when it needs it.
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	backedUp, err := store.BackupWorld(cfg)
	if err != nil {
		return err
	}
	w, err := store.Load(cfg)
	switch {
	case errors.Is(err, store.ErrNoWorld):
		w = store.NewGame(cfg) // first-ever reset: no prior world to load
	case isMalformedWorld(err):
		// A reset is the sysop's way out of a world the game can no longer read,
		// so refusing to run because of what it is about to discard leaves them
		// stuck. The old file survives as world.json.bak, backed up just above.
		fmt.Printf("\nThe existing world could not be read (%v).\nStarting from a fresh one; the unreadable file was kept as world.json.bak.\n", err)
		w = store.NewGame(cfg)
	case err != nil:
		return err
	}

	if fromConfig {
		w.Config = cfg // keep the current config.json untouched
		w.Reset()
		if err := store.Save(w, cfg); err != nil {
			return err
		}
		fmt.Println("\nWorld cleared and re-seeded using the current config.json.")
		if backedUp {
			fmt.Println("The previous world was backed up to world.json.bak.")
		}
		noteDropfileUnset(cfg.DataDir)
		preparePacketDirs(w.Config)
		return nil
	}

	// Seed the settings editor from defaults (keeping the data directory), so a
	// plain -reset also resets config.json to defaults. The editor saves
	// config.json on exit (S); Q cancels the whole reset.
	def := game.DefaultConfig()
	def.DataDir = cfg.DataDir
	// Whether this board is in a league is chosen by which reset command was run,
	// not by a setting: it decides which questions the editor asks, and BRE's own
	// model is that the ruleset is fixed at reset and never edited mid-game.
	def.IBBS = league != nil
	if league != nil {
		if league.ImportPath != "" {
			name, err := importBoardConfig(league.ImportPath, &def)
			if err != nil {
				return err
			}
			// A board named in the imported file has answered what the editor
			// would ask, exactly as -board-id does; an explicit flag still wins.
			if league.BoardID == "" {
				league.BoardID = name
			}
		}
		if league.Inbound != "" {
			def.InboundDir = league.Inbound
		}
		if league.Outbound != "" {
			def.OutboundDir = league.Outbound
		}
	}
	w.Config = def

	// A board named on the command line has said everything the editor would
	// ask that is its own to answer, so it is not opened.
	if league != nil && league.BoardID != "" {
		w.Config.BoardID = league.BoardID
		// Checked before anything is written: the name is compared byte for
		// byte at transport time, and catching it here costs a retyped command
		// rather than a reset that looked like it worked (#154).
		if err := store.CheckBoardInRoster(cfg.DataDir, w.Config.BoardID); err != nil {
			return err
		}
		if err := store.SaveConfig(w.Config); err != nil {
			return err
		}
		w.Reset()
		if err := store.Save(w, cfg); err != nil {
			return err
		}
		fmt.Printf("\nBoard %q is set up for league play. Empires cleared and a fresh world seeded.\n", league.BoardID)
		if backedUp {
			fmt.Println("The previous world was backed up to world.json.bak.")
		}
		printBoardConfig(w.Config)
		// Naming the board skips the editor, which is the member path; but the
		// Coordinator's board can be set up this way too, and at reset time
		// there is often no roster yet to tell which this is.
		fmt.Println("If this board is a league member, the rules arrive from the Coordinator on the next -planetary run.")
		noteDropfileUnset(cfg.DataDir)
		preparePacketDirs(w.Config)
		return nil
	}

	// On a real terminal use the tabbed tview editor (issue #7); fall back to the
	// line-based editor when stdin is piped/redirected, the console cannot render
	// ANSI at all (a legacy Windows console — issue #98; tcell writes the escapes
	// regardless and never notices they came out as text), the sysop asked for no
	// ANSI, or the TUI can't init. -no-ansi has to bypass the TUI rather than be
	// applied to it: tview draws with escapes and offers no way not to.
	saved, usedTUI := false, false
	vtOK, restoreVT := session.EnableVirtualTerminal()
	defer restoreVT()
	if session.StdinIsTerminal() && vtOK && !noANSI {
		if s, err := menu.ConfigEditorTUI(w); err == nil {
			saved, usedTUI = s, true
		}
	}
	if !usedTUI {
		c := session.NewConsole()
		if noANSI {
			c.SetPlain()
		}
		// The editor honours the same charset flags the rest of the game does, so
		// a terminal that reads neither CP437 nor UTF-8 gets its rules in ASCII.
		s := encodeFor(session.Session(c), cs)
		fmt.Fprint(s, "\r\nConfigure the game below (starting from defaults). Choose S to save the settings and start a fresh game, or Q to cancel.\r\n")
		saved = menu.ConfigEditor(s, w)
		c.Close()
	}

	if !saved {
		fmt.Println("\nCancelled. The game was left unchanged.")
		return errCancelled
	}

	w.Reset()
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	fmt.Println("\nGame started with the new settings (config reset to defaults). Empires cleared and AI re-seeded.")
	if backedUp {
		fmt.Println("The previous world was backed up to world.json.bak.")
	}
	if league != nil {
		printBoardConfig(w.Config)
	}
	noteDropfileUnset(cfg.DataDir)
	preparePacketDirs(w.Config)
	return nil
}

// printBoardConfig ends a league reset with the one file the game will not
// write: the board's own identity. A reset rewrites config.json from defaults,
// and writing bbs.cfg alongside it used to return a correctly set-up board to
// "local" and node 0 (#152). The lines are filled in from -board-id, -inbound,
// -outbound and -import-bbs-cfg, so for a sysop who gave those it is a paste.
func printBoardConfig(cfg game.Config) {
	path := filepath.Join(cfg.DataDir, store.BoardConfigFile)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("\nThis board's identity is read from %s, which was left exactly as it was.\nCheck that it reads:\n\n", path)
	} else {
		fmt.Printf("\nThis board's identity lives in %s, which the game never writes.\nCreate it with:\n\n", path)
	}
	fmt.Println(store.BoardConfigText(cfg))
}

// preparePacketDirs creates the inter-BBS packet directories the reset just
// configured, and warns if either still holds packets. Files left over from the
// previous season are applied to the fresh world by the next -planetary run —
// dead realms' attacks landing on a game that has just started — so the warning
// goes last, where a sysop watching the reset scroll past will see it.
func preparePacketDirs(cfg game.Config) {
	if !cfg.InterBBSEnabled() {
		return
	}
	for _, dir := range []string{cfg.Inbound(), cfg.Outbound()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Printf("Could not create the packet directory %s: %v\n", dir, err)
			continue
		}
		// Count and move game packets: an inbound directory is usually the BBS's
		// own FTN inbound, which holds mail bundles and subdirectories that are
		// none of the game's business.
		var brps []string
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() && store.IsPacketFile(e.Name()) {
				brps = append(brps, e.Name())
			}
		}
		if len(brps) == 0 {
			continue
		}
		archive := filepath.Join(dir, fmt.Sprintf("reset-%s", time.Now().Format("2006-01-02")))
		if err := os.MkdirAll(archive, 0o755); err != nil {
			fmt.Printf("Could not create the archive directory %s: %v\n", archive, err)
			continue
		}
		moved := 0
		for _, name := range brps {
			src := filepath.Join(dir, name)
			dst := filepath.Join(archive, name)
			if err := os.Rename(src, dst); err != nil {
				fmt.Printf("Could not move %s: %v\n", src, err)
				continue
			}
			moved++
		}
		if moved > 0 {
			fmt.Printf("Moved %d leftover packet(s) from %s to %s\n", moved, dir, archive)
		}
	}
}

// runAddAI injects up to n new AI barons into the running world (no reset),
// picking unused names from the pool. It reports how many were actually added,
// and why it stopped short: the planet's realm slots run out long before the
// name pool does.
func runAddAI(cfg game.Config, n int) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	added := w.AddAIEmpires(n)
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	fmt.Printf("Added %d AI barons.\n", added)
	switch {
	case added < n && w.PlanetFull():
		fmt.Printf("(Requested %d, but the planet's %d realms are all held.)\n", n, game.PlanetSlots)
	case added < n:
		fmt.Printf("(Requested %d, but the AI name pool is exhausted.)\n", n)
	}
	return nil
}
