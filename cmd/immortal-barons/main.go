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
// The sysop first runs -set-dropfile once to declare which drop file the BBS
// writes; with no -dropfile the door then looks for that file in the working
// directory.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/ibbs"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func main() {
	// -help/usage text is translated to the environment's locale (there is no
	// player context at flag-parse time). i18n.T falls back to English for lang "".
	lang := helpLang()
	// The -dropfile help text names the configured format (or points to
	// -set-dropfile), so peek door.json before defining flags. Best-effort: -data
	// may override the data dir, but it isn't parsed yet.
	preDoor, _ := store.LoadDoorConfig(peekDataDir())
	local := flag.Bool("local", false, i18n.T(lang, "play in your own terminal, not as a BBS door"))
	name := flag.String("name", defaultName(), i18n.T(lang, "your player name (only used with -local)"))
	dropPath := flag.String("dropfile", "", dropfileUsage(lang, preDoor.DropfileFormat))
	setDrop := flag.Bool("set-dropfile", false, i18n.T(lang, "choose which drop file format your BBS writes, save it, then exit"))
	dataDir := flag.String("data", "./data", i18n.T(lang, "folder that holds the game data"))
	maint := flag.Bool("maint", false, i18n.T(lang, "run the daily maintenance, then exit"))
	export := flag.String("export", "", i18n.T(lang, "write this board's score packet to FILE, then exit"))
	imp := flag.String("import", "", i18n.T(lang, "read a score packet from FILE, then exit"))
	planetary := flag.Bool("planetary", false, i18n.T(lang, "run the inter-BBS step: read incoming packets, run group attacks, write outgoing packets, then exit"))
	leagueConfig := flag.Bool("league-config", false, i18n.T(lang, "send this board's league settings to the whole league (node #1 only), then exit"))
	reset := flag.Bool("reset", false, i18n.T(lang, "start a new game: change the settings, then clear all empires and rebuild the world (the old world is saved first)"))
	resetFromConfig := flag.Bool("reset-from-config", false, i18n.T(lang, "start a new game from the current config.json without the editor: clear all empires and rebuild the world (the old world is saved first)"))
	addAI := flag.Int("add-ai", 0, i18n.T(lang, "add N computer barons to the running game, then exit"))
	dump := flag.Bool("dump", false, i18n.T(lang, "print the normalized game world as JSON, then exit (after load-time migration; for scripts and balance checks)"))
	utf8 := flag.Bool("utf8", false, i18n.T(lang, "force UTF-8 output (needed for non-English languages; -local detects this from your locale)"))
	cp437 := flag.Bool("cp437", false, i18n.T(lang, "force CP437 output (the door default; overrides the -local locale detection)"))
	version := flag.Bool("version", false, i18n.T(lang, "print the version, then exit"))
	// Group -help by audience (Play / Character set / Sysop / Inter-BBS / Info)
	// instead of the flat alphabetical default; mirrors docs/command-reference.md (#34).
	flag.Usage = groupedUsage(flag.CommandLine, lang)
	flag.Parse()

	if *version {
		printVersion()
		return
	}

	if *utf8 && *cp437 {
		fmt.Fprintln(os.Stderr, "immortal-barons: use only one of -utf8 and -cp437")
		os.Exit(2)
	}

	// Only the default door front-end takes a positional argument (a dropfile path
	// when -dropfile isn't given). Every explicit-mode flag consumes none, so a
	// stray word alongside one is a mistake — flag it instead of silently ignoring
	// it. (Unknown -flags are already rejected by the flag package.)
	explicitMode := *maint || *planetary || *leagueConfig || *reset || *resetFromConfig ||
		*addAI > 0 || *dump || *local || *export != "" || *imp != "" || *setDrop
	if flag.NArg() > 0 && explicitMode {
		fmt.Fprintf(os.Stderr, "immortal-barons: unknown argument %q\n\n", flag.Arg(0))
		flag.Usage() // show -help, the common convention for a bad invocation
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

	if *setDrop {
		if err := runSetDrop(cfg.DataDir); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -set-dropfile:", err)
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

	// Running as a door: the sysop must have declared which drop file the BBS
	// writes (run -set-dropfile once, stored in door.json). Hard-error rather than
	// guess — a wrong guess silently misreads the caller. -help and every other
	// mode returned above, so this gates only real door launches.
	doorCfg, err := store.LoadDoorConfig(cfg.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons: door.json:", err)
		os.Exit(1)
	}
	if doorCfg.DropfileFormat == "" {
		fmt.Fprintln(os.Stderr, "immortal-barons: no drop file format configured.")
		fmt.Fprintln(os.Stderr, "Run 'immortal-barons -set-dropfile' once to tell the door which drop file your BBS writes.")
		os.Exit(2)
	}

	path := *dropPath
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	if path == "" {
		path = findDropfile(doorCfg.DropfileFormat)
	}
	if path == "" {
		// Log to ib-door.log too: with a live caller this is a silent bounce
		// (issue #37) — the BBS didn't write the drop file where we looked.
		want := doorCfg.DropfileFormat
		if f, ok := door.FormatByID(want); ok {
			want = f.File
		}
		doorLog(cfg.DataDir, "no drop file found: format=%q searched cwd for %s (pass -dropfile PATH)", doorCfg.DropfileFormat, want)
		fmt.Fprintln(os.Stderr, "immortal-barons: no dropfile found.")
		fmt.Fprintln(os.Stderr, "Run it as a BBS door with -dropfile PATH, or play in your terminal with -local.")
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(2)
	}
	caller, err := door.ParseDropfileAs(path, doorCfg.DropfileFormat)
	if err != nil {
		// Log to ib-door.log too: a parse failure with a live caller is the
		// silent no-splash bounce issue #37 is about, and stderr isn't the
		// caller's connection under a BBS, so without this it leaves no trace.
		doorLog(cfg.DataDir, "drop file parse failed: path=%q format=%q err=%v", path, doorCfg.DropfileFormat, err)
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		os.Exit(1)
	}

	// Diagnostic (data/ib-door.log): a silent no-splash bounce (issue #37) leaves
	// no other trace, so record what the dropfile gave us at launch — the I/O mode,
	// time-left, and socket handle name the environment. A file (not stderr) so a
	// remote tester needs no door-config change to capture it.
	doorLog(cfg.DataDir, "launch handle=%q node=%d io=%s seconds-left=%d socket=%d os=%s stdin-tty=%v",
		caller.Handle, caller.Node, ioModeName(caller.IO), caller.SecondsLeft, caller.Socket, runtime.GOOS, session.StdinIsTerminal())

	s, closeSession, err := openSession(caller)
	if err != nil {
		// Log the attach failure to the file too: a winsock socket attach that
		// fails here (issue #37, Windows socket doors) exits before the "session
		// ended" line, so without this it would leave only the launch line.
		doorLog(cfg.DataDir, "openSession failed: %v", err)
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
	reason, err := play.Run(s, id, cfg, today)
	// Diagnostic (data/ib-door.log): record how the session ended. A silent
	// no-splash bounce (issue #37) shows up here as reason="disconnect" right
	// after launch — an I/O read that failed immediately, which points at a dead
	// handle rather than the time-left deadline (that prints before it boots).
	doorLog(cfg.DataDir, "session ended handle=%q reason=%q err=%v", handle, reason, err)
	if err != nil {
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

// doorLog appends one timestamped diagnostic line to <dataDir>/ib-door.log. It
// is best-effort — a logging failure must never stop the door, so errors are
// ignored — and writes to a file (not stderr) so a remote sysop can capture the
// silent no-splash bounce (issue #37) without changing the door command. Short
// O_APPEND lines are atomic on a local filesystem, so concurrent nodes don't
// interleave.
func doorLog(dataDir, format string, args ...any) {
	f, err := os.OpenFile(filepath.Join(dataDir, "ib-door.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("2006-01-02 15:04:05")+" "+format+"\n", args...)
}

// ioModeName renders a dropfile I/O mode for the launch diagnostic.
func ioModeName(m door.IOMode) string {
	switch m {
	case door.IOSerial:
		return "serial"
	case door.IOSocket:
		return "socket"
	case door.IOStdio:
		return "stdio"
	default:
		return "local"
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

// helpLang picks the language for -help/usage text from the environment locale
// (LC_ALL, then LC_MESSAGES, then LANG), when the game ships a UI catalog for it —
// e.g. LANG=nl_NL.UTF-8 → "nl". There is no player context at flag-parse time, so
// the locale is the only signal; an unknown or unset locale falls back to English.
func helpLang() string {
	loc := os.Getenv("LC_ALL")
	if loc == "" {
		loc = os.Getenv("LC_MESSAGES")
	}
	if loc == "" {
		loc = os.Getenv("LANG")
	}
	if i := strings.IndexAny(loc, "_.@"); i >= 0 {
		loc = loc[:i]
	}
	if i18n.Has(loc) {
		return loc
	}
	return ""
}

// printVersion writes the app version, the Go runtime, and — when built from a
// VCS checkout (Go embeds this automatically) — the revision. This is the
// conventional -version output for a Go program.
func printVersion() {
	fmt.Printf("immortal-barons %s\n", game.VersionString())
	fmt.Printf("go: %s\n", runtime.Version())
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
	st := session.NewStdio()
	return st, st.Close, nil // Close restores a pty stdin's mode (no-op for a pipe)
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
	// A drop file format must be configured before the door can run. If the sysop
	// hasn't chosen one yet, run the -set-dropfile chooser first (writing door.json,
	// which is independent of the game data this reset rebuilds), then continue
	// into the reset (BRE asks the drop file type during its one-time install too).
	if !fromConfig {
		dc, err := store.LoadDoorConfig(cfg.DataDir)
		if err != nil {
			return err
		}
		if dc.DropfileFormat == "" {
			c := session.NewConsole()
			format, ok := chooseDropfile(c, "")
			c.Close()
			if !ok {
				fmt.Println("\nCancelled. The game was left unchanged.")
				return nil
			}
			dc.DropfileFormat = format
			if err := store.SaveDoorConfig(cfg.DataDir, dc); err != nil {
				return err
			}
		}
	}

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

	// On a real terminal use the tabbed tview editor (issue #7); fall back to the
	// line-based editor when stdin is piped/redirected or the TUI can't init.
	saved, usedTUI := false, false
	if session.StdinIsTerminal() {
		if s, err := menu.ConfigEditorTUI(w); err == nil {
			saved, usedTUI = s, true
		}
	}
	if !usedTUI {
		c := session.NewConsole()
		fmt.Fprint(c, "\r\nConfigure the game below (starting from defaults). Choose S to save the settings and start a fresh game, or Q to cancel.\r\n")
		saved = menu.ConfigEditor(c, w)
		c.Close()
	}

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

// findDropfile looks in the working directory for the configured format's drop
// file (either letter case), a convenience for when -dropfile isn't given. The
// real door invocation passes -dropfile explicitly.
func findDropfile(format string) string {
	f, ok := door.FormatByID(format)
	if !ok {
		return ""
	}
	for _, n := range []string{f.File, strings.ToLower(f.File)} {
		if _, err := os.Stat(n); err == nil {
			return n
		}
	}
	return ""
}

// peekDataDir scans os.Args for -data/--data so the -dropfile help text can name
// the configured format before flag.Parse runs. Best-effort; defaults to ./data.
func peekDataDir() string {
	args := os.Args[1:]
	for i, a := range args {
		switch {
		case a == "-data" || a == "--data":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "-data="):
			return strings.TrimPrefix(a, "-data=")
		case strings.HasPrefix(a, "--data="):
			return strings.TrimPrefix(a, "--data=")
		}
	}
	return "./data"
}

// dropfileUsage builds the -dropfile help text: it names the configured format,
// or points an unconfigured sysop at -set-dropfile.
func dropfileUsage(lang, format string) string {
	base := i18n.T(lang, "path to the BBS drop file")
	if f, ok := door.FormatByID(format); ok {
		return base + " (" + f.Name + ")"
	}
	return base + " (" + i18n.T(lang, "run -set-dropfile first") + ")"
}

// runSetDrop lets the sysop choose which drop file format the BBS writes and
// saves it to door.json (BRE asks this during its one-time install). The door
// then reads that format; -reset runs this automatically when it isn't set yet.
func runSetDrop(dataDir string) error {
	dc, err := store.LoadDoorConfig(dataDir)
	if err != nil {
		return err
	}
	c := session.NewConsole()
	format, ok := chooseDropfile(c, dc.DropfileFormat)
	c.Close()
	if !ok {
		fmt.Println("\nDrop file format left unchanged.")
		return nil
	}
	dc.DropfileFormat = format
	if err := store.SaveDoorConfig(dataDir, dc); err != nil {
		return err
	}
	f, _ := door.FormatByID(format)
	fmt.Printf("\nDrop file format set to %s. Configure your BBS to launch the door with the %s path.\n", f.Name, f.File)
	return nil
}

// chooseDropfile shows the drop file-format selection screen (styled like the
// reset screens: a BRE-style separator, a numbered list, a Choice> prompt) and
// returns the chosen Format ID. ok is false if the sysop quits. current is the
// presently-configured format, marked in the list.
func chooseDropfile(s session.Session, current string) (string, bool) {
	sep := strings.Repeat("─", 5) + strings.Repeat("═", 15) + strings.Repeat("─", 40)
	for {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, sep, ansi.Reset)
		fmt.Fprintf(s, "%s  Drop File Format%s\n\n", ansi.FgBrightWhite, ansi.Reset)
		fmt.Fprintf(s, "  Which drop file does your BBS write when it launches a door?\n\n")
		for i, f := range door.Formats {
			cur := ""
			if f.ID == current {
				cur = ansi.FgWhite + "  (current)" + ansi.Reset
			}
			fmt.Fprintf(s, "  %s%d)%s %s%s\n", ansi.FgBrightYellow, i+1, ansi.Reset, f.Name, cur)
		}
		fmt.Fprintf(s, "  %s0)%s Quit (leave unchanged)\n", ansi.FgBrightYellow, ansi.Reset)
		fmt.Fprint(s, "\nChoice> ")

		line, err := session.ReadLine(s)
		if err != nil {
			return "", false
		}
		line = strings.TrimSpace(line)
		if line == "" || line == "0" {
			return "", false
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(door.Formats) {
			fmt.Fprintf(s, "  %sPlease enter a number from 0 to %d.%s\n", ansi.FgBrightRed, len(door.Formats), ansi.Reset)
			continue
		}
		return door.Formats[n-1].ID, true
	}
}
