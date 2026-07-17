// Command immortal-barons runs the game. Normal (door) mode reads the
// caller's dropfile and plays over stdio or a socket, for use under a BBS.
// With -local it instead plays locally in your terminal against the shared
// persistent world. With -maint it runs daily maintenance non-interactively
// (for the sysop's nightly event).
//
// Configure your BBS to run it with the dropfile path, e.g.:
//
//	immortal-barons -dropfile /path/to/node/DOOR32.SYS
//
// With no -dropfile it looks for DOOR32.SYS or DOOR.SYS in the working directory.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/ibbs"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func main() {
	local := flag.Bool("local", false, "play locally in your terminal instead of running as a BBS door")
	name := flag.String("name", defaultName(), "your player handle (-local only)")
	dropPath := flag.String("dropfile", "", "path to the BBS dropfile (DOOR32.SYS or DOOR.SYS)")
	dataDir := flag.String("data", "./data", "game data directory")
	maint := flag.Bool("maint", false, "run daily maintenance and exit")
	export := flag.String("export", "", "write this board's score packet to FILE and exit")
	imp := flag.String("import", "", "import a score packet from FILE and exit")
	planetary := flag.Bool("planetary", false, "run the inter-BBS PLANETARY step (read inbound, launch attacks, write outbound) and exit")
	leagueConfig := flag.Bool("league-config", false, "broadcast this board's league settings to the league (coordinator/node #1 only) and exit")
	reset := flag.Bool("reset", false, "start a fresh game: edit the settings (starting from defaults), then wipe empires and re-seed (backs up world.json first)")
	resetFromConfig := flag.Bool("reset-from-config", false, "start a fresh game using the current config.json as-is (no editor): wipe empires and re-seed (backs up world.json first)")
	addAI := flag.Int("add-ai", 0, "add N AI barons to the running game and exit")
	dump := flag.Bool("dump", false, "print the game world as JSON to stdout and exit (for scripting and balance checks)")
	utf8 := flag.Bool("utf8", false, "force UTF-8 output (needed for non-English languages; -local auto-detects this from your locale)")
	cp437 := flag.Bool("cp437", false, "force CP437 output (the door default; overrides -local locale auto-detection)")
	version := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *version {
		printVersion()
		return
	}

	if *utf8 && *cp437 {
		fmt.Fprintln(os.Stderr, "immortal-barons: use only one of -utf8 and -cp437")
		os.Exit(2)
	}

	cfg, err := store.LoadConfig(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	today := time.Now().Format("2006-01-02")
	// IB_GAME_DATE overrides the game's "today" for testing (e.g.
	// IB_GAME_DATE=2026-07-17 to advance a day and run daily maintenance without
	// changing the system clock). An env var, not a flag, so it stays out of -help
	// and off a casual player's radar. A malformed value errors rather than
	// silently using the real date.
	if d := os.Getenv("IB_GAME_DATE"); d != "" {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons: IB_GAME_DATE must be YYYY-MM-DD:", err)
			os.Exit(2)
		}
		today = d
	}

	if *export != "" {
		if err := runExport(cfg, *export, today); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -export:", err)
			os.Exit(1)
		}
		return
	}

	if *imp != "" {
		if err := runImport(cfg, *imp); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -import:", err)
			os.Exit(1)
		}
		return
	}

	if *maint {
		if err := runMaint(cfg, today); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -maint:", err)
			os.Exit(1)
		}
		return
	}

	if *planetary {
		if err := runPlanetary(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -planetary:", err)
			os.Exit(1)
		}
		return
	}

	if *leagueConfig {
		if err := runLeagueConfig(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -league-config:", err)
			os.Exit(1)
		}
		return
	}

	if *reset {
		if err := runReset(cfg, false); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -reset:", err)
			os.Exit(1)
		}
		return
	}

	if *resetFromConfig {
		if err := runReset(cfg, true); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -reset-from-config:", err)
			os.Exit(1)
		}
		return
	}

	if *addAI > 0 {
		if err := runAddAI(cfg, *addAI); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -add-ai:", err)
			os.Exit(1)
		}
		return
	}

	if *dump {
		if err := runDump(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -dump:", err)
			os.Exit(1)
		}
		return
	}

	if *local {
		runLocal(cfg, *name, today, wantUTF8(*utf8, *cp437, true))
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
		fmt.Fprintln(os.Stderr, "immortal-barons: no dropfile found.")
		fmt.Fprintln(os.Stderr, "Run it as a BBS door with -dropfile PATH, or play in your terminal with -local.")
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(2)
	}
	caller, err := door.ParseDropfile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		os.Exit(1)
	}

	s, closeSession, err := openSession(caller)
	if err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		os.Exit(1)
	}
	defer closeSession()

	// Traditional BBS terminals expect CP437, so transcode UTF-8 -> CP437 unless
	// the sysop forces UTF-8.
	if !wantUTF8(*utf8, *cp437, false) {
		s = session.NewCP437Writer(s)
	}

	handle := caller.Handle
	if handle == "" {
		handle = fmt.Sprintf("node%d", caller.Node)
	}
	// The caller's remaining BBS time is a hard session cap: play.Run's deadline
	// boots at it, saving the world and releasing the lock cleanly (unlike the
	// old os.Exit, which lost the turn's progress).
	id := play.Identity{Handle: handle, TimeLeft: time.Duration(caller.SecondsLeft) * time.Second}
	if _, err := play.Run(s, id, cfg, today); err != nil {
		// Fail loudly to the CALLER, not just the BBS log. A bootstrap failure
		// (world load, lock, I/O) otherwise drops the caller straight back to the
		// BBS menu with no splash and no reason — looking like the door is broken.
		// Write the reason to their screen and hold it briefly so they can read it
		// before the BBS reclaims the screen.
		fmt.Fprintf(s, "\r\nImmortal Barons could not start:\r\n  %v\r\n\r\nPlease tell the sysop. Returning to the BBS...\r\n", err)
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		time.Sleep(4 * time.Second)
		os.Exit(1)
	}
}

// runLocal plays Immortal Barons locally in the caller's terminal against the
// shared persistent world, for someone testing or playing outside a BBS.
func runLocal(cfg game.Config, name, today string, utf8 bool) {
	c := session.NewConsole()
	defer c.Close()

	// utf8 was resolved from the flags and the locale by wantUTF8.
	var s session.Session = c
	if !utf8 {
		s = session.NewCP437Writer(c)
	}
	if _, err := play.Run(s, play.Identity{Handle: name}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons -local:", err)
		return // no sign-off after a startup failure (e.g. no game — run -reset)
	}
	fmt.Fprint(s, "\nUntil next turn, Baron.\n")
}

// wantUTF8 resolves the output charset. An explicit -utf8/-cp437 always wins;
// otherwise a local session follows the process locale, and a door assumes
// CP437 (its locale reflects the BBS server, not the caller's terminal).
func wantUTF8(forceUTF8, forceCP437, local bool) bool {
	switch {
	case forceUTF8:
		return true
	case forceCP437:
		return false
	case local:
		return session.LocaleIsUTF8()
	default:
		return false
	}
}

func defaultName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "sysop"
}

// printVersion writes the app version, the Go runtime, and — when built from a
// VCS checkout (Go embeds this automatically) — the revision. This is the
// conventional -version output for a Go program.
func printVersion() {
	fmt.Printf("immortal-barons %s\n", game.Version)
	fmt.Printf("go: %s\n", runtime.Version())
	if bi, ok := debug.ReadBuildInfo(); ok {
		var rev, mod string
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				rev = s.Value
			case "vcs.modified":
				if s.Value == "true" {
					mod = " (modified)"
				}
			}
		}
		if rev != "" {
			fmt.Printf("revision: %s%s\n", rev, mod)
		}
	}
}

// openSession attaches to the caller per the dropfile's I/O mode and platform.
//
// On Unix, Synchronet and Mystic run native doors with the caller's connection
// wired to our stdin/stdout (Synchronet's EX_STDIO mode pipes the socket to us
// and handles telnet itself), so stdio is correct even though the dropfile
// reports a socket with a handle — that handle is the BBS's own socket, not
// something the door attaches to. Only on Windows does the door attach to the
// inherited winsock handle directly. The returned func releases the connection.
func openSession(caller *door.Caller) (session.Session, func(), error) {
	if caller.IO == door.IOSocket && runtime.GOOS == "windows" {
		sock, err := session.NewSocket(caller.Socket)
		if err != nil {
			return nil, nil, fmt.Errorf("attaching to the winsock handle %d from the dropfile failed: %w", caller.Socket, err)
		}
		return sock, sock.Close, nil
	}
	if caller.IO == door.IOSerial {
		return nil, nil, fmt.Errorf("serial (FOSSIL) doors are not supported; configure your BBS for a socket or stdio door")
	}
	return session.NewStdio(), func() {}, nil
}

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

// runReset is BRE's sysop reset: present the game-settings menu (the
// Configuration Editor) so the sysop sets up the new game, then wipe all
// empires (humans re-onboard on their next login), re-seed AI, and save. The
// old world is backed up first. It does not crown a winner.
// runReset starts a fresh game. With fromConfig=false (-reset) it opens the
// settings editor seeded from defaults and saves the edited config.json. With
// fromConfig=true (-reset-from-config) it skips the editor and keeps the current
// config.json as-is. Either way the world is wiped and re-seeded.
func runReset(cfg game.Config, fromConfig bool) error {
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
	if errors.Is(err, store.ErrNoWorld) {
		w = store.NewGame(cfg) // first-ever reset: no prior world to load
	} else if err != nil {
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
		return nil
	}

	// Seed the settings editor from defaults (keeping the data directory), so a
	// plain -reset also resets config.json to defaults. The editor saves
	// config.json on exit (S); Q cancels the whole reset.
	def := game.DefaultConfig()
	def.DataDir = cfg.DataDir
	w.Config = def

	c := session.NewConsole()
	fmt.Fprint(c, "\r\nConfigure the game below (starting from defaults). Choose S to save the settings and start a fresh game, or Q to cancel.\r\n")
	saved := menu.ConfigEditor(c, w)
	c.Close()

	if !saved {
		fmt.Println("\nCancelled. The game was left unchanged.")
		return nil
	}

	w.Reset()
	if err := store.Save(w, cfg); err != nil {
		return err
	}
	fmt.Println("\nGame started with the new settings (config reset to defaults). Empires cleared and AI re-seeded.")
	if backedUp {
		fmt.Println("The previous world was backed up to world.json.bak.")
	}
	return nil
}

// runAddAI injects up to n new AI barons into the running world (no reset),
// picking unused names from the pool. It reports how many were actually added,
// noting when the name pool was exhausted before reaching n.
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
	if added < n {
		fmt.Printf("(Requested %d, but the AI name pool is exhausted.)\n", n)
	}
	return nil
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

func findDropfile() string {
	for _, n := range []string{"door32.sys", "DOOR32.SYS", "door.sys", "DOOR.SYS"} {
		if _, err := os.Stat(n); err == nil {
			return n
		}
	}
	return ""
}
