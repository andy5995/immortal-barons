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
			name, lines, err := importBoardConfig(league.ImportPath, &def)
			if err != nil {
				return err
			}
			league.FTNLines = lines
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
		printBoardConfig(w.Config, league.FTNLines)
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
		// The editor honors the same charset flags the rest of the game does, so
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
		printBoardConfig(w.Config, league.FTNLines)
	}
	noteDropfileUnset(cfg.DataDir)
	preparePacketDirs(w.Config)
	return nil
}

// printBoardConfig ends a league reset with the one file the game will not
// write: the board's own identity. A reset rewrites config.json from defaults,
// and writing bbs.cfg alongside it used to return a correctly set-up board to
// "local" and node 0 (#152). The lines are filled in from -board-id,
// -game-inbound, -game-outbound and -import-bbs-cfg, so for a sysop who gave
// those it is a paste. ftnLines are the transport's lines from an import.
func printBoardConfig(cfg game.Config, ftnLines []string) {
	path := filepath.Join(cfg.DataDir, store.BoardConfigFile)
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("\nThis board's identity is read from %s, which was left exactly as it was.\nCheck that it reads:\n\n", path)
	} else {
		fmt.Printf("\nThis board's identity lives in %s, which the game never writes.\nCreate it with:\n\n", path)
	}
	fmt.Print(store.BoardConfigText(cfg))
	for _, line := range ftnLines {
		fmt.Println(line)
	}
	fmt.Println()
}

// preparePacketDirs creates the inter-BBS packet directories the reset just
// configured, and moves aside any packets they still hold. Files left over from
// the previous season are applied to the fresh world by the next -planetary run —
// dead realms' attacks landing on a game that has just started — so they go
// into a dated archive, and the report goes last, where a sysop watching the
// reset scroll past will see it.
//
// The held directory is swept too (#261). A packet held for a newer protocol is
// moved back into inbound as soon as this board upgrades, and a season boundary
// is exactly when boards upgrade, so without this last season's packets reached
// the new world by the one door the inbound sweep did not watch.
func preparePacketDirs(cfg game.Config) {
	if !cfg.InterBBSEnabled() {
		return
	}
	for _, dir := range []string{cfg.Inbound(), cfg.Outbound()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fmt.Printf("Could not create the packet directory %s: %v\n", dir, err)
			continue
		}
		if moved, archive := archiveLeftoverPackets(dir); moved > 0 {
			fmt.Printf("Moved %d leftover packet(s) from %s to %s\n", moved, dir, archive)
		}
	}
	held := filepath.Join(cfg.DataDir, store.HeldDir)
	if moved, archive := archiveLeftoverPackets(held); moved > 0 {
		fmt.Printf("Moved %d held packet(s) from %s to %s: they were waiting for an upgrade and belong to the old season\n",
			moved, held, archive)
	}
}

// archiveLeftoverPackets moves the game packets in dir into a dated reset-
// subdirectory of it, and reports how many moved and where. Only packets move:
// an inbound directory is usually the BBS's own FTN inbound, which holds mail
// bundles and subdirectories that are none of the game's business. A missing
// dir moves nothing.
func archiveLeftoverPackets(dir string) (int, string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, ""
	}
	var brps []string
	for _, e := range entries {
		if !e.IsDir() && store.IsPacketFile(e.Name()) {
			brps = append(brps, e.Name())
		}
	}
	if len(brps) == 0 {
		return 0, ""
	}
	archive := filepath.Join(dir, fmt.Sprintf("reset-%s", time.Now().Format("2006-01-02")))
	if err := os.MkdirAll(archive, 0o755); err != nil {
		fmt.Printf("Could not create the archive directory %s: %v\n", archive, err)
		return 0, ""
	}
	moved := 0
	for _, name := range brps {
		src := filepath.Join(dir, name)
		if err := os.Rename(src, filepath.Join(archive, name)); err != nil {
			fmt.Printf("Could not move %s: %v\n", src, err)
			continue
		}
		moved++
	}
	return moved, archive
}
