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
	"runtime"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/store"
)

// errCancelled reports that the sysop backed out of an interactive step. The
// reason is already on screen, so exitOn adds no second message — but the exit
// status must still be non-zero, so an unattended run (a drop file chooser with
// no one to answer it, say) can tell that the reset did not happen.
var errCancelled = errors.New("cancelled")

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
	o := defineFlags(lang, preDoor)
	// Group -help by audience (Play / Character set / Sysop / Inter-BBS / Info)
	// instead of the flat alphabetical default; mirrors docs/command-reference.md (#34).
	flag.Usage = groupedUsage(flag.CommandLine, lang)
	flag.Parse()

	if *o.version {
		printVersion()
		return
	}

	if n := btoi(*o.utf8) + btoi(*o.cp437) + btoi(*o.asciiOut); n > 1 {
		fmt.Fprintln(os.Stderr, "immortal-barons: use only one of -utf8, -cp437 and -ascii")
		os.Exit(2)
	}

	// Every mode that draws at THIS terminal resolves the charset the same way;
	// only the door reads it from the caller's BBS instead (encodeFor, below).
	localCS := wantCharset(*o.utf8, *o.cp437, *o.asciiOut, true)

	// A stray word alongside a mode flag is a mistake, not something to ignore.
	// (Unknown -flags are already rejected by the flag package.)
	if flag.NArg() > 0 && o.explicitMode() {
		fmt.Fprintf(os.Stderr, "immortal-barons: unknown argument %q\n\n", flag.Arg(0))
		flag.Usage() // show -help, the common convention for a bad invocation
		os.Exit(2)
	}

	cfg, err := store.LoadConfig(*o.dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	// A modifier, not a mode: it rides whatever else was asked for (-local, the
	// door, -planetary), like -no-ansi. It is applied to the in-memory config
	// only — Config.DupeCheckOverride never reaches config.json.
	if *o.dupeCheck != "" {
		on, ok := parseOnOff(*o.dupeCheck)
		if !ok {
			fmt.Fprintf(os.Stderr, "immortal-barons: -dupe-check takes on or off, not %q\n", *o.dupeCheck)
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

	if *o.maint {
		exitOn("-maint", runMaint(cfg, today))
		return
	}

	if *o.planetary {
		exitOn("-planetary", runPlanetary(cfg, *o.detailed))
		return
	}

	if *o.full {
		exitOn("-full", runFull(cfg, *o.name, today, localCS, *o.noANSI, *o.detailed))
		return
	}

	if *o.leagueConfig {
		exitOn("-league-config", runLeagueConfig(cfg))
		return
	}
	if *o.leagueCheck {
		if !runLeagueCheck(cfg) {
			os.Exit(1)
		}
		return
	}
	if *o.leagueRoutes {
		exitOn("-league-routes", runLeagueRoutes(cfg))
		return
	}
	// The original's three sysop reports (docs/bre.doc, "Command-Line Options").
	for _, r := range []struct {
		on   bool
		flag string
	}{{*o.lastPacket, "lastpacket"}, {*o.bbsInfo, "bbsinfo"}, {*o.playerList, "playerlist"}} {
		if !r.on {
			continue
		}
		if err := runLeagueReport(cfg, r.flag); err != nil {
			fmt.Fprintf(os.Stderr, "immortal-barons -%s: %v\n", r.flag, err)
			os.Exit(1)
		}
		return
	}
	if *o.players {
		exitOn("-players", runPlayers(cfg, os.Stdin, os.Stdout))
		return
	}
	if *o.genCoordKey {
		runGenCoordKey(cfg)
		return
	}
	if *o.genBoardKey {
		runGenBoardKey(cfg)
		return
	}
	if *o.coordPub != "" {
		exitOn("-coord-key", store.InstallCoordPub(cfg.DataDir, *o.coordPub))
		fmt.Println("Coordinator key recorded. League orders will be checked against it.")
		return
	}
	if *o.leagueReset != "" {
		exitOn("-league-reset", runLeagueReset(cfg, *o.leagueReset))
		return
	}

	if *o.boardID != "" || *o.inboundDir != "" || *o.outboundDir != "" || *o.importBoardCfg != "" {
		if !*o.ibbsReset {
			fmt.Fprintln(os.Stderr, "immortal-barons: -board-id, -inbound, -outbound and -import-bbs-cfg are settings for -ibbs-reset")
			os.Exit(2)
		}
	}

	if *o.reset || *o.ibbsReset {
		mode := "-reset"
		var league *leagueSetup
		if *o.ibbsReset {
			mode = "-ibbs-reset"
			league = &leagueSetup{BoardID: *o.boardID, Inbound: *o.inboundDir, Outbound: *o.outboundDir, ImportPath: *o.importBoardCfg}
		}
		exitOn(mode, runReset(cfg, false, league, localCS, *o.noANSI))
		return
	}

	if *o.resetFromConfig {
		// The ibbs argument seeds the editor, which this mode never opens: it
		// keeps whatever config.json already says, league board or not.
		exitOn("-reset-from-config", runReset(cfg, true, nil, localCS, *o.noANSI))
		return
	}

	if *o.setDrop {
		exitOn("-set-dropfile", runSetDrop(cfg.DataDir))
		return
	}

	if *o.addAI > 0 {
		exitOn("-add-ai", runAddAI(cfg, *o.addAI))
		return
	}

	if *o.spectate > 0 {
		exitOn("-spectate", runSpectate(cfg, *o.spectate))
		return
	}

	if *o.dump {
		exitOn("-dump", runDump(cfg))
		return
	}

	if *o.local {
		runLocal(cfg, *o.name, today, localCS, *o.noANSI)
		return
	}

	runDoor(cfg, o, today, localCS)
}

const (
	charsetCP437 charset = iota
	charsetUTF8
	charsetASCII
)

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

// isMalformedWorld reports whether err is the world file failing to parse rather
// than failing to be read. Only the first is safe for a reset to discard: an
// unparseable world is exactly what a reset replaces, while a permission or I/O
// error is a problem that wiping the world would hide.
func isMalformedWorld(err error) bool {
	var syn *json.SyntaxError
	var typ *json.UnmarshalTypeError
	return errors.As(err, &syn) || errors.As(err, &typ)
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
