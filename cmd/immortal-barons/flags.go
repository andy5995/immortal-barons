package main

import (
	"flag"

	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/store"
)

// opts holds every command-line flag. They are gathered in one place because
// there are three dozen of them and main's job is to dispatch on them, not to
// declare them — the declarations had made up most of the function.
//
// The fields are the pointers flag returns, so they are only valid after
// flag.Parse.
type opts struct {
	local           *bool
	name            *string
	dropPath        *string
	setDrop         *bool
	dataDir         *string
	maint           *bool
	planetary       *bool
	full            *bool
	detailed        *bool
	leagueConfig    *bool
	genCoordKey     *bool
	coordPub        *string
	genBoardKey     *bool
	leagueReset     *string
	leagueCheck     *bool
	leagueRoutes    *bool
	lastPacket      *bool
	bbsInfo         *bool
	playerList      *bool
	players         *bool
	reset           *bool
	boardID         *string
	inboundDir      *string
	outboundDir     *string
	importBoardCfg  *string
	ibbsReset       *bool
	resetFromConfig *bool
	addAI           *int
	spectate        *int
	dump            *bool
	dupeCheck       *string
	utf8            *bool
	cp437           *bool
	asciiOut        *bool
	noANSI          *bool
	version         *bool
}

// defineFlags registers every flag and returns the pointers. preDoor is the
// door config peeked before parsing, so the -dropfile help can name the format
// already configured; lang is the environment's locale, since there is no
// player context this early.
func defineFlags(lang string, preDoor store.DoorConfig) *opts {
	return &opts{
		local:           flag.Bool("local", false, i18n.T(lang, "play in your own terminal, not as a BBS door")),
		name:            flag.String("name", defaultName(), i18n.T(lang, "your player name (only used with -local)")),
		dropPath:        flag.String("dropfile", "", dropfileUsage(lang, preDoor.DropfileFormat)),
		setDrop:         flag.Bool("set-dropfile", false, i18n.T(lang, "choose which drop file format your BBS writes, save it, then exit")),
		dataDir:         flag.String("data", "./data", i18n.T(lang, "folder that holds the game data")),
		maint:           flag.Bool("maint", false, i18n.T(lang, "run the daily maintenance, then exit")),
		planetary:       flag.Bool("planetary", false, i18n.T(lang, "run the inter-BBS step: read incoming packets, run group attacks, write outgoing packets, then exit")),
		full:            flag.Bool("full", false, i18n.T(lang, "run the full cycle: read inbound packets, play a turn, write outbound packets, then exit")),
		detailed:        flag.Bool("detailed", false, i18n.T(lang, "show each packet as it is read and written (use with -full or -planetary)")),
		leagueConfig:    flag.Bool("league-config", false, i18n.T(lang, "send this board's league settings to the whole league (node #1 only), then exit")),
		genCoordKey:     flag.Bool("gen-coord-key", false, i18n.T(lang, "create this league's Coordinator key, print the public half to give the other boards, then exit (node #1 only)")),
		coordPub:        flag.String("coord-key", "", i18n.T(lang, "record the league Coordinator's public key (the value -gen-coord-key printed), then exit")),
		genBoardKey:     flag.Bool("gen-board-key", false, i18n.T(lang, "create this board's packet-signing key, print the public half to send to the League Coordinator, then exit")),
		leagueReset:     flag.String("league-reset", "", i18n.T(lang, "start a new season across the whole league on DATE (node #1 only), then exit")),
		leagueCheck:     flag.Bool("league-check", false, i18n.T(lang, "check this board's league setup — roster, board name, packet directories, keys — and report everything wrong at once, then exit")),
		leagueRoutes:    flag.Bool("league-routes", false, i18n.T(lang, "print which board each planet's packets are handed to, and the directory they are written in, then exit")),
		lastPacket:      flag.Bool("lastpacket", false, i18n.T(lang, "write LASTPACKET.LST — when a packet from each other board was last processed here, then exit")),
		bbsInfo:         flag.Bool("bbsinfo", false, i18n.T(lang, "write BBSINFO.LST — every board, when it was last heard from, and the version it runs, then exit")),
		playerList:      flag.Bool("playerlist", false, i18n.T(lang, "write PLAYERLIST.LST — every realm on every board (League Coordinator only), then exit")),
		players:         flag.Bool("players", false, i18n.T(lang, "list the players and change a caller's name, rename their realm, or remove it, then exit")),
		reset:           flag.Bool("reset", false, i18n.T(lang, "start a new game: change the settings, then clear all empires and rebuild the world (the old world is saved first)")),
		boardID:         flag.String("board-id", "", i18n.T(lang, "this board's name in the league, for -ibbs-reset. Giving it skips the settings editor, for a member board that takes its rules from the Coordinator")),
		inboundDir:      flag.String("inbound", "", i18n.T(lang, "directory where packets from the other boards arrive, for -ibbs-reset (default \"inbound\", under the data directory)")),
		outboundDir:     flag.String("outbound", "", i18n.T(lang, "directory the game writes packets to for the other boards, for -ibbs-reset (default \"outbound\", under the data directory)")),
		importBoardCfg:  flag.String("import-bbs-cfg", "", i18n.T(lang, "take this board's name, inbound directory and league number from an original Barren Realms Elite BBS.CFG at PATH, for -ibbs-reset")),
		ibbsReset:       flag.Bool("ibbs-reset", false, i18n.T(lang, "start a new game as a board in an inter-BBS league: like -reset, but the settings editor also asks the league settings")),
		resetFromConfig: flag.Bool("reset-from-config", false, i18n.T(lang, "start a new game from the current config.json without the editor: clear all empires and rebuild the world (the old world is saved first)")),
		addAI:           flag.Int("add-ai", 0, i18n.T(lang, "add N computer barons to the running game, then exit")),
		spectate:        flag.Int("spectate", 0, i18n.T(lang, "play the game forward N days of computer-baron turns, printing a per-day summary and final standings, then exit (a balance probe). ADVANCES AND SAVES the game, so it asks first and refuses on a game that has human realms")),
		dump:            flag.Bool("dump", false, i18n.T(lang, "print the normalized game world as JSON, then exit (after load-time migration; for scripts and balance checks)")),
		dupeCheck:       flag.String("dupe-check", "", i18n.T(lang, "force Dupe Checking `on|off` for this run only, for testing a league lockout. Nothing is saved: the sysop's setting is left as it is")),
		utf8:            flag.Bool("utf8", false, i18n.T(lang, "force UTF-8 output (needed for non-English languages; -local detects this from your locale)")),
		cp437:           flag.Bool("cp437", false, i18n.T(lang, "force CP437 output (the door default; overrides the -local locale detection)")),
		asciiOut:        flag.Bool("ascii", false, i18n.T(lang, "force plain 7-bit ASCII output, for a terminal that is neither CP437 nor UTF-8 (box rules and accents degrade to ASCII look-alikes)")),
		noANSI:          flag.Bool("no-ansi", false, i18n.T(lang, "send plain text with no ANSI escapes, as a terminal that cannot render them gets (for testing that path on a terminal that can)")),
		version:         flag.Bool("version", false, i18n.T(lang, "print the version, then exit")),
	}
}

// explicitMode reports whether a flag naming a mode was given. Only the default
// door front-end takes a positional argument (a dropfile path when -dropfile is
// not given), so a stray word alongside any of these is a mistake rather than
// something to ignore.
func (o *opts) explicitMode() bool {
	return *o.maint || *o.planetary || *o.full || *o.leagueConfig || *o.leagueRoutes ||
		*o.leagueCheck || *o.reset || *o.resetFromConfig || *o.ibbsReset ||
		*o.lastPacket || *o.bbsInfo || *o.playerList || *o.players ||
		*o.addAI > 0 || *o.dump || *o.spectate > 0 || *o.local || *o.setDrop
}
