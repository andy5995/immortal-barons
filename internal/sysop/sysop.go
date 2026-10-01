// Package sysop is what the sysop panel (cmd/ib-sysop) reads and runs, kept
// out of that command so it builds and is tested with the rest of the tree:
// the panel is a separate module, because its GUI toolkit is a dependency the
// door must never carry.
//
// It never writes the world. A snapshot takes the world lock, loads, gathers
// and releases, as World.Read does; every action is the game binary run as a
// child process, so locking, transport and saving stay in the code that
// already does them.
package sysop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// ErrNoWorld is a directory holding no world.json: not a game data directory,
// or one whose game has not been created yet.
var ErrNoWorld = errors.New("no world.json here: this is not a game data directory")

// Snapshot is one board's state at the moment it was read.
type Snapshot struct {
	DataDir     string
	Taken       time.Time
	BoardID     string
	League      bool // the board is in a league (inter-BBS enabled)
	Coordinator bool // the board is node #1
	GameDay     int
	MinVersion  string
	// OwnRulesDiffer is this board playing rules the Coordinator has not sent.
	OwnRulesDiffer bool
	// LostForcesDays is the league's wait before an unanswered strike comes
	// home; 0 is recovery off.
	LostForcesDays int
	Boards         []Board
	InFlight       []InFlight
	Held           []store.HeldPacket
}

// Board is one other board in the league.
type Board struct {
	game.BBSInfoRow
	// LastHeard is LastRecon as a time; zero if never.
	LastHeard time.Time
	// SilentDays is days since a packet from it was last processed; -1 never.
	SilentDays int
	// RoundTrip is the average packet round trip in days; 0 if never measured.
	RoundTrip float64
	// ProbeBack is when a probe sent to it last came home; zero if never.
	ProbeBack time.Time
	// Held is a protocol hold that is still current: its latest packet was held
	// rather than applied (game.World.ProtocolHoldCurrent), the same rule that
	// pauses the lost-forces timer. A held file left over from before the board
	// upgraded does not count.
	Held bool
	// HeldSince is when the current hold was recorded; zero, with Held set, means
	// the hold carries no time (held by a build that did not record one).
	HeldSince time.Time
}

// Status is the board's condition in words. A table must never say it in color
// alone.
func (b Board) Status() string {
	var s []string
	if b.LastRecon == "" {
		s = append(s, "never heard")
	}
	if b.BelowMin {
		s = append(s, "below min")
	}
	if b.OtherRules {
		s = append(s, "other rules")
	}
	if b.Held {
		s = append(s, "held")
	}
	if len(s) == 0 {
		return "ok"
	}
	return strings.Join(s, ", ")
}

// InFlight is one strike, operation or bid that has left this board and has not
// been answered.
type InFlight struct {
	ID          int
	Kind        string // attack, terror, trade or special
	What        string
	Owner       string // realm names, comma-separated for a group attack
	TargetBoard string
	TargetRealm string
	LaunchedDay int
	Waiting     int  // game days since it left
	HeldDays    int  // game days spent held, the hold in progress included
	Held        bool // its board's packets are held right now
	// DaysLeft is the game days until the lost-forces timer returns it; 0 or
	// less means the next planetary run. Meaningless when !Recovers.
	DaysLeft int
	Recovers bool
}

// Open checks that dir is a game data directory and returns it cleaned and
// absolute, the form the panel keys its tabs by. A door's own folder, whose
// data directory is the default "data" under it, is taken to mean that one,
// since it is the folder a sysop thinks of as the game. It creates nothing:
// taking the lock on a mistyped path would make the directory.
func Open(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for _, d := range []string{abs, filepath.Join(abs, "data")} {
		_, err := os.Stat(filepath.Join(d, "world.json"))
		if err == nil {
			return d, nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
	}
	return "", ErrNoWorld
}

// Read takes one snapshot of the board in dir. The world lock is exclusive and
// blocking, the same one a save takes, so this waits for a running -maint: call
// it off the UI thread. Door nodes wait for it too, but only for the few
// milliseconds a load takes.
func Read(dir string, now time.Time) (Snapshot, error) {
	dir, err := Open(dir)
	if err != nil {
		return Snapshot{}, err
	}
	cfg, err := store.LoadConfig(dir)
	if err != nil {
		return Snapshot{}, fmt.Errorf("config: %w", err)
	}
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return Snapshot{}, err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	return gather(w, now)
}

func gather(w *game.World, now time.Time) (Snapshot, error) {
	s := Snapshot{
		DataDir:        w.Config.DataDir,
		Taken:          now,
		BoardID:        w.Config.BoardID,
		League:         w.Config.InterBBSEnabled(),
		Coordinator:    w.IsLeagueCoordinator(),
		GameDay:        w.GameDay,
		MinVersion:     w.Config.MinBoardVersion,
		OwnRulesDiffer: w.OwnRulesDiffer(),
		LostForcesDays: w.Config.LostForcesDays,
	}
	held, err := store.HeldPackets(w)
	if err != nil {
		return s, err
	}
	s.Held = held
	heldFrom := map[string]bool{}
	for _, h := range held {
		if h.Reason == store.HeldProtocol {
			heldFrom[h.FromBoard] = true
		}
	}
	for _, r := range w.BBSInfoRows() {
		b := Board{BBSInfoRow: r, SilentDays: w.LinkSilentDays(r.Name, now),
			RoundTrip: w.TravelTimes[r.Name], Held: heldFrom[r.Name] && w.ProtocolHoldCurrent(r.Name)}
		if t, ok := game.ParseStamp(w.ProtocolHeldAt[r.Name]); ok && b.Held {
			b.HeldSince = t
		}
		if t, ok := game.ParseStamp(r.LastRecon); ok {
			b.LastHeard = t
		}
		if t, err := time.Parse(time.RFC3339, w.TravelSeen[r.Name]); err == nil {
			b.ProbeBack = t
		}
		s.Boards = append(s.Boards, b)
	}
	for _, f := range w.InFlight {
		row := InFlight{ID: f.ID, Kind: f.Kind, What: what(f), Owner: owners(w, f),
			TargetBoard: f.TargetBoard, TargetRealm: f.TargetEmpire,
			LaunchedDay: f.LaunchedDay, Waiting: w.GameDay - f.LaunchedDay,
			HeldDays: f.HeldDays, Held: f.Held}
		if f.Held {
			// f.HeldDays counts only holds that have ended; add the open one,
			// or a strike paused since launch reads 0 days held.
			row.HeldDays += w.GameDay - f.HeldSince
		}
		switch {
		case f.Kind == "trade":
			row.TargetRealm = "-" // a bid goes to the planet's market, not a realm
		case f.Whole || row.TargetRealm == "":
			row.TargetRealm = "(whole planet)"
		}
		row.DaysLeft, row.Recovers = w.LostForcesDaysLeft(f)
		s.InFlight = append(s.InFlight, row)
	}
	return s, nil
}

// what names what an in-flight item is, from whichever field its kind uses.
func what(f game.InFlightStrike) string {
	switch f.Kind {
	case "terror":
		word := "agents"
		if f.Agents == 1 {
			word = "agent"
		}
		return fmt.Sprintf("%s (%d %s)", f.TerrorOp, f.Agents, word)
	case "special":
		return game.SpecialOpLabel(f.Op)
	case "trade":
		return fmt.Sprintf("bid: %d %s at %d", f.Qty, f.Good, f.Price)
	}
	kind := "attack"
	if f.Group {
		kind = "group attack"
	}
	return kind + ": " + f.Committed().Summary()
}

// owners names who sent an item by realm, falling back to the handle when the
// realm is gone.
func owners(w *game.World, f game.InFlightStrike) string {
	name := func(owner string) string {
		if e := w.FindByOwner(owner); e != nil {
			return e.Name
		}
		return owner
	}
	if f.Kind != "attack" {
		return name(f.Owner)
	}
	seen := map[string]bool{}
	var names []string
	for _, c := range f.Contributors {
		if n := name(c.Owner); !seen[n] {
			seen[n] = true
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
