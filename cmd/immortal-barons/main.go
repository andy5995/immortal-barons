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
	"github.com/andy5995/immortal-barons/internal/textwrap"
)

// errCancelled reports that the sysop backed out of an interactive step. The
// reason is already on screen, so exitOn adds no second message — but the exit
// status must still be non-zero, so an unattended run (a drop file chooser with
// no one to answer it, say) can tell that the reset did not happen.
var errCancelled = errors.New("cancelled")

// dropfileUnsetMsg is what both places that notice an unconfigured drop file
// say: the note after a reset, and the door's own refusal to start. One string
// so the two never drift into telling the sysop different things.
const dropfileUnsetMsg = "No drop file format is set. If you intend to run this as a BBS door, run -set-dropfile to choose the format your BBS writes."

// exitOn reports a mode's failure and exits non-zero, staying quiet for a
// cancellation the sysop already saw.
func exitOn(mode string, err error) {
	if err == nil {
		return
	}
	if !errors.Is(err, errCancelled) {
		fmt.Fprintln(os.Stderr, "immortal-barons "+mode+":", err)
	}
	os.Exit(1)
}

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
	full := flag.Bool("full", false, i18n.T(lang, "run the full cycle: read inbound packets, play a turn, write outbound packets, then exit"))
	scores := flag.Bool("scores", false, i18n.T(lang, "write this board's score packet to the outbound directory, then exit"))
	detailed := flag.Bool("detailed", false, i18n.T(lang, "show each packet as it is read and written (use with -full or -planetary)"))
	leagueConfig := flag.Bool("league-config", false, i18n.T(lang, "send this board's league settings to the whole league (node #1 only), then exit"))
	genCoordKey := flag.Bool("gen-coord-key", false, i18n.T(lang, "create this league's Coordinator key, print the public half to give the other boards, then exit (node #1 only)"))
	coordPub := flag.String("coord-key", "", i18n.T(lang, "record the league Coordinator's public key (the value -gen-coord-key printed), then exit"))
	genBoardKey := flag.Bool("gen-board-key", false, i18n.T(lang, "create this board's packet-signing key, print the public half to send to the League Coordinator, then exit"))
	leagueReset := flag.String("league-reset", "", i18n.T(lang, "start a new season across the whole league on DATE (node #1 only), then exit"))
	leagueCheck := flag.Bool("league-check", false, i18n.T(lang, "check this board's league setup — roster, board name, packet directories, keys — and report everything wrong at once, then exit"))
	leagueRoutes := flag.Bool("league-routes", false, i18n.T(lang, "print which board each planet's packets are handed to, and the directory they are written in, then exit"))
	lastPacket := flag.Bool("lastpacket", false, i18n.T(lang, "write LASTPACKET.LST — when a packet from each other board was last processed here, then exit"))
	bbsInfo := flag.Bool("bbsinfo", false, i18n.T(lang, "write BBSINFO.LST — every board, when it was last heard from, and the version it runs, then exit"))
	playerList := flag.Bool("playerlist", false, i18n.T(lang, "write PLAYERLIST.LST — every realm on every board (League Coordinator only), then exit"))
	players := flag.Bool("players", false, i18n.T(lang, "list the players and change a caller's name, rename their realm, or remove it, then exit"))
	reset := flag.Bool("reset", false, i18n.T(lang, "start a new game: change the settings, then clear all empires and rebuild the world (the old world is saved first)"))
	boardID := flag.String("board-id", "", i18n.T(lang, "this board's name in the league, for -ibbs-reset. Giving it skips the settings editor, for a member board that takes its rules from the Coordinator"))
	inboundDir := flag.String("inbound", "", i18n.T(lang, "directory where packets from the other boards arrive, for -ibbs-reset (default \"inbound\", under the data directory)"))
	outboundDir := flag.String("outbound", "", i18n.T(lang, "directory the game writes packets to for the other boards, for -ibbs-reset (default \"outbound\", under the data directory)"))
	importBoardCfg := flag.String("import-bbs-cfg", "", i18n.T(lang, "take this board's name, inbound directory and league number from an original Barren Realms Elite BBS.CFG at PATH, for -ibbs-reset"))
	ibbsReset := flag.Bool("ibbs-reset", false, i18n.T(lang, "start a new game as a board in an inter-BBS league: like -reset, but the settings editor also asks the league settings"))
	resetFromConfig := flag.Bool("reset-from-config", false, i18n.T(lang, "start a new game from the current config.json without the editor: clear all empires and rebuild the world (the old world is saved first)"))
	addAI := flag.Int("add-ai", 0, i18n.T(lang, "add N computer barons to the running game, then exit"))
	spectate := flag.Int("spectate", 0, i18n.T(lang, "play the game forward N days of computer-baron turns, printing a per-day summary and final standings, then exit (a balance probe). ADVANCES AND SAVES the game, so it asks first and refuses on a game that has human realms"))
	dump := flag.Bool("dump", false, i18n.T(lang, "print the normalized game world as JSON, then exit (after load-time migration; for scripts and balance checks)"))
	dupeCheck := flag.String("dupe-check", "", i18n.T(lang, "force Dupe Checking `on|off` for this run only, for testing a league lockout. Nothing is saved: the sysop's setting is left as it is"))
	utf8 := flag.Bool("utf8", false, i18n.T(lang, "force UTF-8 output (needed for non-English languages; -local detects this from your locale)"))
	cp437 := flag.Bool("cp437", false, i18n.T(lang, "force CP437 output (the door default; overrides the -local locale detection)"))
	asciiOut := flag.Bool("ascii", false, i18n.T(lang, "force plain 7-bit ASCII output, for a terminal that is neither CP437 nor UTF-8 (box rules and accents degrade to ASCII look-alikes)"))
	noANSI := flag.Bool("no-ansi", false, i18n.T(lang, "send plain text with no ANSI escapes, as a terminal that cannot render them gets (for testing that path on a terminal that can)"))
	version := flag.Bool("version", false, i18n.T(lang, "print the version, then exit"))
	// Group -help by audience (Play / Character set / Sysop / Inter-BBS / Info)
	// instead of the flat alphabetical default; mirrors docs/command-reference.md (#34).
	flag.Usage = groupedUsage(flag.CommandLine, lang)
	flag.Parse()

	if *version {
		printVersion()
		return
	}

	if n := btoi(*utf8) + btoi(*cp437) + btoi(*asciiOut); n > 1 {
		fmt.Fprintln(os.Stderr, "immortal-barons: use only one of -utf8, -cp437 and -ascii")
		os.Exit(2)
	}

	// Every mode that draws at THIS terminal resolves the charset the same way;
	// only the door reads it from the caller's BBS instead (encodeFor, below).
	localCS := wantCharset(*utf8, *cp437, *asciiOut, true)

	// Only the default door front-end takes a positional argument (a dropfile path
	// when -dropfile isn't given). Every explicit-mode flag consumes none, so a
	// stray word alongside one is a mistake — flag it instead of silently ignoring
	// it. (Unknown -flags are already rejected by the flag package.)
	explicitMode := *maint || *planetary || *full || *scores || *leagueConfig || *leagueRoutes || *leagueCheck || *reset || *resetFromConfig || *ibbsReset ||
		*lastPacket || *bbsInfo || *playerList || *players ||
		*addAI > 0 || *dump || *spectate > 0 || *local || *export != "" || *imp != "" || *setDrop
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
	// A modifier, not a mode: it rides whatever else was asked for (-local, the
	// door, -planetary), like -no-ansi. It is applied to the in-memory config
	// only — Config.DupeCheckOverride never reaches config.json.
	if *dupeCheck != "" {
		on, ok := parseOnOff(*dupeCheck)
		if !ok {
			fmt.Fprintf(os.Stderr, "immortal-barons: -dupe-check takes on or off, not %q\n", *dupeCheck)
			os.Exit(2)
		}
		cfg.DupeCheckOverride = &on
		word := "off"
		if on {
			word = "on"
		}
		// stderr, not stdout: under a BBS this reaches the sysop's log rather
		// than the caller's screen, where it would land in the middle of the
		// game's own output.
		fmt.Fprintf(os.Stderr, "immortal-barons: Dupe Checking forced %s for this run only; the saved setting is unchanged.\n", word)
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
		if err := runPlanetary(cfg, *detailed); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -planetary:", err)
			os.Exit(1)
		}
		return
	}

	if *full {
		exitOn("-full", runFull(cfg, *name, today, localCS, *noANSI, *detailed))
		return
	}

	if *scores {
		if err := runScores(cfg, today); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -scores:", err)
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
	if *leagueCheck {
		if !runLeagueCheck(cfg) {
			os.Exit(1)
		}
		return
	}
	if *leagueRoutes {
		if err := runLeagueRoutes(cfg); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -league-routes:", err)
			os.Exit(1)
		}
		return
	}
	// The original's three sysop reports (docs/bre.doc, "Command-Line Options").
	for _, r := range []struct {
		on   bool
		flag string
	}{{*lastPacket, "lastpacket"}, {*bbsInfo, "bbsinfo"}, {*playerList, "playerlist"}} {
		if !r.on {
			continue
		}
		if err := runLeagueReport(cfg, r.flag); err != nil {
			fmt.Fprintf(os.Stderr, "immortal-barons -%s: %v\n", r.flag, err)
			os.Exit(1)
		}
		return
	}
	if *players {
		if err := runPlayers(cfg, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -players:", err)
			os.Exit(1)
		}
		return
	}
	if *genCoordKey {
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
		return
	}
	if *genBoardKey {
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
		return
	}
	if *coordPub != "" {
		if err := store.InstallCoordPub(cfg.DataDir, *coordPub); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -coord-key:", err)
			os.Exit(1)
		}
		fmt.Println("Coordinator key recorded. League orders will be checked against it.")
		return
	}
	if *leagueReset != "" {
		if err := runLeagueReset(cfg, *leagueReset); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -league-reset:", err)
			os.Exit(1)
		}
		return
	}

	if *boardID != "" || *inboundDir != "" || *outboundDir != "" || *importBoardCfg != "" {
		if !*ibbsReset {
			fmt.Fprintln(os.Stderr, "immortal-barons: -board-id, -inbound, -outbound and -import-bbs-cfg are settings for -ibbs-reset")
			os.Exit(2)
		}
	}

	if *reset || *ibbsReset {
		mode := "-reset"
		var league *leagueSetup
		if *ibbsReset {
			mode = "-ibbs-reset"
			league = &leagueSetup{BoardID: *boardID, Inbound: *inboundDir, Outbound: *outboundDir, ImportPath: *importBoardCfg}
		}
		exitOn(mode, runReset(cfg, false, league, localCS, *noANSI))
		return
	}

	if *resetFromConfig {
		// The ibbs argument seeds the editor, which this mode never opens: it
		// keeps whatever config.json already says, league board or not.
		exitOn("-reset-from-config", runReset(cfg, true, nil, localCS, *noANSI))
		return
	}

	if *setDrop {
		exitOn("-set-dropfile", runSetDrop(cfg.DataDir))
		return
	}

	if *addAI > 0 {
		if err := runAddAI(cfg, *addAI); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -add-ai:", err)
			os.Exit(1)
		}
		return
	}

	if *spectate > 0 {
		if err := runSpectate(cfg, *spectate); err != nil {
			fmt.Fprintln(os.Stderr, "immortal-barons -spectate:", err)
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
		runLocal(cfg, *name, today, localCS, *noANSI)
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
		fmt.Fprintln(os.Stderr, "immortal-barons:", dropfileUnsetMsg)
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
	doorLog(cfg.DataDir, "launch handle=%q node=%d io=%s seconds-left=%d socket=%d os=%s stdin-tty=%v ansi=%v",
		caller.Handle, caller.Node, ioModeName(caller.IO), caller.SecondsLeft, caller.Socket, runtime.GOOS, session.StdinIsTerminal(), caller.ANSI)

	s, closeSession, err := openSession(caller)
	// ...and separately, which backend actually opened. The line above reports
	// what the DROP FILE said; a sysop chasing a dead session needs to know what
	// the door then did with it, and the two used to differ silently.
	doorLog(cfg.DataDir, "session i/o backend=%s", backendName(caller))
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
	// the sysop forces another charset.
	s = encodeFor(s, wantCharset(*utf8, *cp437, *asciiOut, false))
	// A caller at the far end of a socket cannot be probed for ANSI the way a
	// local console can, so the dropfile flag their BBS profile filled in is the
	// only signal (issue #101). Without ANSI they would otherwise read every
	// escape as literal text.
	if !caller.ANSI || *noANSI {
		s = session.NewPlain(s)
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
func runLocal(cfg game.Config, name, today string, cs charset, noANSI bool) {
	// -name "" would build a realm with no owner handle, which is the marker for a
	// computer baron — the same fallback the door path applies to a dropfile with
	// no alias.
	if strings.TrimSpace(name) == "" {
		name = defaultName()
	}
	c := session.NewConsole()
	defer c.Close()

	// -no-ansi forces the console's own plain mode, which a legacy Windows console
	// selects for itself (issue #98) — so the same one path is exercised.
	if noANSI {
		c.SetPlain()
	}

	// cs was resolved from the flags and the locale by wantCharset.
	s := encodeFor(session.Session(c), cs)
	if _, err := play.Run(s, play.Identity{Handle: name}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons -local:", err)
		return // no sign-off after a startup failure (e.g. no game — run -reset)
	}
	fmt.Fprint(s, "\nUntil next turn, Baron.\n")
}

// charset is what the caller's terminal reads. CP437 is the BBS tradition,
// UTF-8 the modern default, and ASCII the fallback for a terminal that is
// neither and would render either as mojibake.
type charset int

const (
	charsetCP437 charset = iota
	charsetUTF8
	charsetASCII
)

// wantCharset resolves the output charset. An explicit -utf8/-cp437/-ascii
// always wins (in that order when more than one is given); otherwise a local
// session follows the process locale, and a door assumes CP437 (its locale
// reflects the BBS server, not the caller's terminal).
func wantCharset(forceUTF8, forceCP437, forceASCII, local bool) charset {
	switch {
	case forceUTF8:
		return charsetUTF8
	case forceCP437:
		return charsetCP437
	case forceASCII:
		return charsetASCII
	case local && session.LocaleIsUTF8():
		return charsetUTF8
	default:
		return charsetCP437
	}
}

// encodeFor wraps s in the writer for cs. UTF-8 needs none: it is what the
// engine already emits.
func encodeFor(s session.Session, cs charset) session.Session {
	switch cs {
	case charsetCP437:
		return session.NewCP437Writer(s)
	case charsetASCII:
		return session.NewASCIIWriter(s)
	default:
		return s
	}
}

// btoi counts a set flag, for the "only one of these" check.
func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
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

// useSocket is the one place that decides between the socket and stdio, so the
// logged backend cannot drift from the one actually opened. See openSession for
// why the terminal test is Unix-only.
func useSocket(caller *door.Caller) bool {
	return caller.IO == door.IOSocket && (runtime.GOOS == "windows" || !session.StdinIsTerminal())
}

// backendName is the I/O path openSession takes for this caller — the thing a
// sysop actually needs in the log, as against what the drop file claimed.
func backendName(caller *door.Caller) string {
	switch {
	case useSocket(caller):
		return "socket"
	case caller.IO == door.IOSerial:
		return "unsupported-serial"
	default:
		return "stdio"
	}
}

// openSession attaches to the caller by the path useSocket picks, and returns a
// func that releases the connection.
func openSession(caller *door.Caller) (session.Session, func(), error) {
	// Which of the two I/O paths to take is NOT decided by the platform, and not
	// by the drop file alone. Two live Linux boards settle it:
	//
	//	Mystic       io=socket socket=8   stdin-tty=TRUE    works on stdio
	//	Synchronet   io=socket socket=58  stdin-tty=FALSE   dropped instantly
	//
	// Both name a socket, so the drop file cannot tell them apart. What differs
	// is whether the BBS also gave this process the connection on standard input:
	// Mystic hands over a terminal and expects the door to use it, while a native
	// Synchronet door with I/O interception off is given the socket as an
	// inherited descriptor and a stdin attached to nothing — so its first read
	// hit EOF and the caller was dropped the moment the door started.
	//
	// So: on Unix a live stdin wins, because a BBS that went to the trouble of
	// handing us one means it to be used. Only when there is no terminal AND the
	// drop file names a socket do we take the socket.
	//
	// That test is Unix-only, and deliberately so. A native Windows door is a
	// child of the BBS and inherits ITS console, so standard input is a terminal
	// there whether or not it carries the caller — the test cannot tell a handed-
	// over connection from the server's own console, and answering "terminal"
	// sends the door to a console the caller cannot see. Windows therefore keeps
	// the older rule it has always used: a drop file naming a socket means the
	// socket. Widening the Unix heuristic to Windows hung the door for a caller
	// (reported against the win32 snapshot).
	if useSocket(caller) {
		sock, err := session.NewSocket(caller.Socket)
		if err != nil {
			return nil, nil, fmt.Errorf("the drop file names socket %d and standard input is not a terminal, "+
				"but attaching to the socket failed: %w"+
				"\n(if your BBS pipes the connection through standard I/O instead, turn the door's"+
				"\nI/O interception on so it writes a Local drop file)",
				caller.Socket, err)
		}
		return sock, sock.Close, nil
	}
	if caller.IO == door.IOSerial {
		return nil, nil, fmt.Errorf("serial (FOSSIL) doors are not supported; configure your BBS for a socket or stdio door")
	}
	st := session.NewStdio()
	return st, st.Close, nil // Close restores a pty stdin's mode (no-op for a pipe)
}

// isMalformedWorld reports whether err is the world file failing to parse rather
// than failing to be read. Only the first is safe for a reset to discard: an
// unparseable world is exactly what a reset replaces, while a permission or I/O
// error is a problem that wiping the world would hide.
func isMalformedWorld(err error) bool {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	return errors.As(err, &syn) || errors.As(err, &typ)
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

// runLeagueRoutes reports where this board sends each planet's packets — BRE's
// "BRE TEST", whose whole job is letting a sysop see the routing the roster
// gives them before they wonder why nothing arrives.
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
	if !allOK {
		fmt.Println("\nFix the FAIL lines above; see the Door Setup guide for what each one wants.")
	}
	return allOK
}

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

// parseOnOff reads the value of -dupe-check. Only on and off are accepted, in
// either letter case — the words the Configuration Editor and the game settings
// screen already show for this switch.
func parseOnOff(v string) (on, ok bool) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "on":
		return true, true
	case "off":
		return false, true
	}
	return false, false
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

// runReset is BRE's sysop reset: present the game-settings menu (the
// Configuration Editor) so the sysop sets up the new game, then wipe all
// empires (humans re-onboard on their next login), re-seed AI, and save. The
// old world is backed up first. It does not crown a winner.
//
// With fromConfig=true (-reset-from-config) it skips the editor and keeps the
// current config.json as-is. Either way the world is wiped and re-seeded.
func runReset(cfg game.Config, fromConfig bool, league *leagueSetup, cs charset, noANSI bool) error {
	// No drop file prompt here: a reset seeds the world for the web front-end and
	// -local play too, neither of which reads a drop file. The door names
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

// noteDropfileUnset reminds the sysop after a reset that the door still needs a
// drop file format. A note, not a prompt: the world it just seeded is also what
// the web front-end and -local play use, and neither reads a drop file. A
// door.json that can't be read is left to the door to report.
func noteDropfileUnset(dataDir string) {
	if dc, err := store.LoadDoorConfig(dataDir); err == nil && dc.DropfileFormat == "" {
		fmt.Println(dropfileUnsetMsg)
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

// runPlanetary runs the inter-BBS maintenance step on its own (BRE's
// "BRE PLANETARY"): apply inbound packets, launch due group attacks, export
// scores, and write the outbox. Can run several times a day.
func runPlanetary(cfg game.Config, verbose bool) error {
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
	skipped := run.OtherLeague + run.MeshCopy + run.AlreadySeen + run.Refused
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
	skipped := run.OtherLeague + run.MeshCopy + run.AlreadySeen + run.Refused
	if skipped == 0 {
		return ""
	}
	var parts []string
	// First: a refusal is the one reason here that means something is wrong,
	// rather than a packet this board had no business with.
	if run.Refused > 0 {
		parts = append(parts, fmt.Sprintf("%d refused, not matching the sender's key", run.Refused))
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

// runScores writes this board's score packet to the outbound directory (BRE's
// "BRE SCORES"), then exits. The destination is always the default outbound
// directory; use -export to write to a specific path.
func runScores(cfg game.Config, today string) error {
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
	outDir := cfg.Outbound()
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return err
	}
	name := cfg.BoardID
	if name == "" {
		name = "scores"
	}
	path := filepath.Join(outDir, name+store.PacketExt)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	fmt.Printf("Wrote score packet (%d scores) to %s\n", len(packet.Scores), path)
	return nil
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
		return errCancelled
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
// reset screens: the BRE inset rule, a numbered list, the shared menu prompt)
// and returns the chosen Format ID. ok is false if the sysop quits. current is
// the presently-configured format, marked in the list.
func chooseDropfile(s session.Session, current string) (string, bool) {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, menu.InsetRule, ansi.Reset)
	fmt.Fprintf(s, "%s  Drop File Format%s\n\n", ansi.FgBrightWhite, ansi.Reset)
	fmt.Fprint(s, "  Which drop file does your BBS write when it launches a door?\n\n")
	for i, f := range door.Formats {
		cur := ""
		if f.ID == current {
			cur = ansi.FgWhite + "  (current)" + ansi.Reset
		}
		fmt.Fprintf(s, "  %s%d)%s %s%s\n", ansi.FgBrightYellow, i+1, ansi.Reset, f.Name, cur)
	}
	fmt.Fprintf(s, "  %s0)%s Quit (leave unchanged)\n", ansi.FgBrightYellow, ansi.Reset)

	// The shared prompt the menu engine uses for every numbered list, so this
	// screen reads and behaves like the rest of the game (one keypress, Enter
	// selects the shown Quit default).
	n := menu.ChoiceQuit(s, len(door.Formats))
	if n == 0 {
		return "", false
	}
	return door.Formats[n-1].ID, true
}
