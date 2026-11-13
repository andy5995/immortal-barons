// Package store persists the game world to disk. Save writes the world
// atomically (temp file + rename); Load reads it back, or returns a fresh
// world when no save exists yet.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// ErrNoWorld is returned by Load when no saved world exists yet. A game must be
// created with -reset (or -reset-from-config) before it can be played or
// maintained; a missing world is a setup error, not a fresh empty game.
var ErrNoWorld = errors.New("no game found — run with -reset to create one")

// NewGame builds a fresh, unsaved world from the config. It is the only path
// that conjures a world from nothing (used by -reset); every other entry point
// loads an existing one and errors with ErrNoWorld if it is missing.
func NewGame(cfg game.Config) (*game.World, error) {
	w := game.NewWorld(cfg)
	loadLeagueNodes(w, cfg)
	loadLeagueKeys(w, cfg)
	// A new game on a board that has played before keeps its packet ledger: a
	// reset from an unreadable world must not number its packets from 1 again.
	if err := loadLedger(w, cfg, nil); err != nil {
		return nil, err
	}
	return w, nil
}

// LedgerFile holds the packet ledger (World.OutSeq, HighSeq, ReplayFloor,
// SeenSeqs and BattleSeq): the
// numbering of this board's packets and the record of packets applied here.
// It is kept apart from world.json because a reset erases the world and must
// not erase this — see World.OutSeq.
const LedgerFile = "packet-ledger.json"

// ledger is the file's contents. Its keys are the World fields' own names, so
// a world.json written before the ledger had a file of its own reads into it
// too (loadLedger's migration).
type ledger struct {
	OutSeq      uint64              `json:",omitempty"`
	HighSeq     map[string]uint64   `json:",omitempty"`
	ReplayFloor map[string]uint64   `json:",omitempty"`
	SeenSeqs    map[string][]uint64 `json:",omitempty"`
	BattleSeq   int                 `json:",omitempty"`
	// Gen is the save this ledger was written by, and world.json carries the
	// same number (savedWorld). The ledger is renamed into place first, so a
	// save that stops between the two renames leaves it one ahead of the world:
	// it then records packets as applied that the world on disk never took, and
	// the next run would delete them unread (#311). Before is what undoes that.
	Gen uint64 `json:",omitempty"`
	// Before holds, for each board whose record of applied packets this save
	// changed, that record as it stood. Only a planetary run changes one, so it
	// is empty after a player's save.
	Before map[string]seenRecord `json:",omitempty"`
}

// seenRecord is one board's record of applied packets: its entries in HighSeq,
// ReplayFloor and SeenSeqs. A nil ReplayFloor means the board had none.
type seenRecord struct {
	HighSeq     uint64   `json:",omitempty"`
	ReplayFloor *uint64  `json:",omitempty"`
	SeenSeqs    []uint64 `json:",omitempty"`
}

func (l *ledger) record(board string) seenRecord {
	r := seenRecord{HighSeq: l.HighSeq[board], SeenSeqs: l.SeenSeqs[board]}
	if f, ok := l.ReplayFloor[board]; ok {
		r.ReplayFloor = &f
	}
	return r
}

func (r seenRecord) equal(o seenRecord) bool {
	if (r.ReplayFloor == nil) != (o.ReplayFloor == nil) || (r.ReplayFloor != nil && *r.ReplayFloor != *o.ReplayFloor) {
		return false
	}
	return r.HighSeq == o.HighSeq && slices.Equal(r.SeenSeqs, o.SeenSeqs)
}

// rollBack returns the record of applied packets to what it was before the
// save that wrote this ledger. OutSeq is left alone: packets may have gone out
// under the numbers it counts, and a number is never used twice.
func (l *ledger) rollBack() {
	for board, was := range l.Before {
		setOrDelete(&l.HighSeq, board, was.HighSeq, was.HighSeq != 0)
		setOrDelete(&l.SeenSeqs, board, was.SeenSeqs, len(was.SeenSeqs) > 0)
		if was.ReplayFloor != nil {
			setOrDelete(&l.ReplayFloor, board, *was.ReplayFloor, true)
		} else {
			delete(l.ReplayFloor, board)
		}
	}
	l.Before = nil
}

func setOrDelete[V any](m *map[string]V, key string, v V, set bool) {
	if !set {
		delete(*m, key)
		return
	}
	if *m == nil {
		*m = map[string]V{}
	}
	(*m)[key] = v
}

// changedSince lists each board whose record differs between was and l, with
// the record it had in was.
func (l *ledger) changedSince(was *ledger) map[string]seenRecord {
	var before map[string]seenRecord
	for _, m := range []map[string]uint64{l.HighSeq, l.ReplayFloor, was.HighSeq, was.ReplayFloor} {
		for board := range m {
			if old := was.record(board); !old.equal(l.record(board)) {
				if before == nil {
					before = map[string]seenRecord{}
				}
				before[board] = old
			}
		}
	}
	return before
}

func ledgerPath(cfg game.Config) string { return filepath.Join(cfg.DataDir, LedgerFile) }

// readLedger reads the ledger file as the world saved at generation worldGen
// should see it: a ledger one save ahead of that world is rolled back. found
// is false when there is no file.
func readLedger(cfg game.Config, worldGen uint64) (l ledger, found bool, err error) {
	data, err := readWorldFile(ledgerPath(cfg))
	if os.IsNotExist(err) {
		return l, false, nil
	}
	if err != nil {
		return l, false, fmt.Errorf("the packet ledger could not be read: %w", err)
	}
	if err := json.Unmarshal(data, &l); err != nil {
		return l, false, fmt.Errorf("the packet ledger %s could not be read (%v); restore it from a backup, or delete it to number this board's packets from 1 again", ledgerPath(cfg), err)
	}
	if l.Gen == worldGen+1 {
		l.rollBack()
	}
	return l, true, nil
}

// loadLedger puts the ledger on w from its file. A board upgrading from a
// build that kept the ledger in world.json has no file yet; worldData, the
// world file just read, is where its ledger still is, and the next save
// writes it out to the file. worldData is nil for a new game, which has no
// world file and takes the ledger as it stands.
//
// A ledger that exists but cannot be read is an error, not an empty ledger:
// starting OutSeq from 0 would have every other board drop this board's
// packets as replays, silently, until the count passed where it had been.
func loadLedger(w *game.World, cfg game.Config, worldData []byte) error {
	var saved struct{ LedgerGen uint64 }
	if worldData != nil {
		// The world has already been decoded from these bytes.
		json.Unmarshal(worldData, &saved)
	}
	l, found, err := readLedger(cfg, saved.LedgerGen)
	if err != nil {
		return err
	}
	switch {
	case worldData == nil:
		saved.LedgerGen = l.Gen
	case !found:
		if err := json.Unmarshal(worldData, &l); err != nil {
			return err
		}
	}
	w.OutSeq, w.HighSeq, w.ReplayFloor, w.SeenSeqs = l.OutSeq, l.HighSeq, l.ReplayFloor, l.SeenSeqs
	w.BattleSeq = l.BattleSeq
	w.LedgerGen = saved.LedgerGen
	return nil
}

// writeLedgerTemp writes w's ledger, as generation gen, to a temporary file
// beside the ledger's own and returns that file's path for Save to rename into
// place.
func writeLedgerTemp(w *game.World, cfg game.Config, gen uint64) (string, error) {
	was, _, err := readLedger(cfg, w.LedgerGen)
	if err != nil {
		return "", err
	}
	l := ledger{OutSeq: w.OutSeq, HighSeq: w.HighSeq, ReplayFloor: w.ReplayFloor, SeenSeqs: w.SeenSeqs, BattleSeq: w.BattleSeq, Gen: gen}
	l.Before = l.changedSince(&was)
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return "", err
	}
	tmp := ledgerPath(cfg) + ".tmp"
	return tmp, os.WriteFile(tmp, data, 0o644)
}

// savedWorld is world.json: the world, and the generation its ledger carries.
type savedWorld struct {
	*game.World
	LedgerGen uint64
}

func worldPath(cfg game.Config) string { return filepath.Join(cfg.DataDir, "world.json") }

// Windows refuses an open of a file another process is replacing, and refuses a
// replacing rename while another process has the file open (see
// isSharingViolation). Both windows are the width of one rename, and both are
// reachable the moment a second BBS node exists: a node starting a session
// reads world.json outside the game lock, which is the one read that is not
// serialized against another node's save. It surfaced as a Windows-only failure
// of TestCrossProcessConcurrentPlay, where the second process lost its whole
// session to "The process cannot access the file because it is being used by
// another process".
//
// Retrying is the fix rather than locking the startup read, because a lock
// there would deadlock any caller that already holds one, and because either
// version of the file is a correct answer to that read — a node re-reads under
// the lock before it changes anything.
const (
	shareRetries = 20
	shareBackoff = 20 * time.Millisecond
)

// retryShared runs op until it stops failing on a sharing violation. Any other
// error, including the last one, is returned as it is.
func retryShared(op func() error) error {
	err := op()
	for i := 0; i < shareRetries && isSharingViolation(err); i++ {
		time.Sleep(shareBackoff)
		err = op()
	}
	return err
}

// readWorldFile reads a world file through retryShared.
func readWorldFile(path string) ([]byte, error) {
	var data []byte
	err := retryShared(func() error {
		var err error
		data, err = os.ReadFile(path)
		return err
	})
	return data, err
}

// renameWorldFile replaces a world file through retryShared.
func renameWorldFile(from, to string) error {
	return retryShared(func() error { return os.Rename(from, to) })
}

// Load reads the saved world, or returns a fresh one if none exists.
func Load(cfg game.Config) (*game.World, error) {
	data, err := readWorldFile(worldPath(cfg))
	if os.IsNotExist(err) {
		return nil, ErrNoWorld
	}
	if err != nil {
		return nil, err
	}
	w := game.NewWorld(cfg) // seeds rng; JSON overwrites exported fields
	clearRandomSeeded(w)
	if err := json.Unmarshal(data, w); err != nil {
		return nil, err
	}
	repair(w, cfg)
	if err := loadLedger(w, cfg, data); err != nil {
		return nil, err
	}
	if err := checkClockOffset(w); err != nil {
		return nil, err
	}
	return w, nil
}

// ClockRewindVar lets an operator load a world that was written under a LARGER
// test-clock shift than the one now in force. It exists because the refusal has
// to be overridable on a rig that is deliberately winding back, and it is a
// separate variable so that winding back is never something a run does by
// simply forgetting to set the offset.
const ClockRewindVar = "IB_CLOCK_REWIND"

// checkClockOffset refuses a world whose instants were written from a clock
// further ahead than this run's, and otherwise records this run's shift so the
// next load can make the same check. See World.ClockOffset.
func checkClockOffset(w *game.World) error {
	now := game.ClockOffset()
	var saved time.Duration
	if w.ClockOffset != "" {
		// An unreadable value is NOT treated as a shift. Failing closed would be
		// the usual instinct for a guard, but this one guards a testing knob that
		// is empty in every real game, and a hand-edited or future-format value
		// would then end a real board's session over something that is no evidence
		// of a shifted clock at all. Say so and carry on.
		d, err := time.ParseDuration(w.ClockOffset)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"immortal-barons: ignoring an unreadable ClockOffset %q in world.json (%v)\n",
				w.ClockOffset, err)
		}
		saved = d
	}
	if saved > now && os.Getenv(ClockRewindVar) == "" {
		return fmt.Errorf(
			"this game was last written with its clock shifted by %s and this run is shifted by %s, "+
				"so its instants are in the future and weapons and probes would never resolve; "+
				"run with a shift of at least %s, or set %s=1 to load it anyway",
			saved, now, saved, ClockRewindVar)
	}
	return nil
}

// stampClockOffset records the shift this run's instants were written under. It
// belongs to SAVE, not to load: a world created by -reset and saved without ever
// being loaded still has shifted instants in it, and recording on load would
// leave that file claiming a real clock.
func stampClockOffset(w *game.World) {
	if d := game.ClockOffset(); d != 0 {
		w.ClockOffset = d.String()
	} else {
		w.ClockOffset = ""
	}
}

// clearRandomSeeded empties the world fields a FRESH game fills with a RANDOM
// value, so that the save decides whether they are there.
//
// It is deliberately narrow. Unmarshalling a save over a fresh world is how
// back-compat works here: `encoding/json` leaves a field alone when the document
// has no key for it, so a save written before a field existed comes up with that
// field's fresh-game default, which is what the migration contract wants — the
// v0.0.3 fixture carries seven top-level keys and relies on inheriting some
// thirty. Zeroing them all instead looks tidier and breaks that: it left the
// fixture with every unit price at 0.
//
// A RANDOM default is the exception, because inheriting one is not a default at
// all, it is a fresh roll per load. The nine pirate factions are the case that
// bit: a save predating them showed a different hoard every time the Attack
// Pirates screen was drawn, and whichever one a write happened to catch stuck.
// repair's EnsurePirates then backfills the names without a hoard, an under-way
// game not being something to retrofit.
func clearRandomSeeded(w *game.World) {
	w.Pirates = nil
	// Same shape as a random default, for the opposite reason: a fresh world is
	// born already swept of computer barons, and inheriting that would tell the
	// migration a pre-v0.2.0 save had nothing to do. TEMPORARY — goes with
	// internal/game/retire_ai.go.
	w.AIRetired = false
}

// copySaved copies everything the SAVE owns from src onto dst, leaving dst's own
// process state alone: fields marked `json:"-"` (the config, the league roster,
// the keys, the sysop notices) and every unexported field, which is what keeps
// the lock and the three rngs intact.
//
// It is how reload gets a world that says what the file says and nothing more.
// Unmarshalling straight into the live world cannot: an absent key leaves the
// field alone, so there it means "whatever this process last held" rather than
// "the fresh default", and more than twenty `omitempty` fields on World go
// missing from the file whenever they are empty. A node kept a weapon whose
// siege another node had ended, and wrote it back on its next transaction; the
// same went for Threats and Battles. Unmarshalling into a fresh world instead
// gives absent its proper meaning, and this copies the result across.
//
// A new field is covered the day it is added, with nobody having to know this
// exists, which is why it is reflection rather than a list to keep up to date.
func copySaved(dst, src *game.World) {
	d, s := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	t := d.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() || f.Tag.Get("json") == "-" {
			continue
		}
		d.Field(i).Set(s.Field(i))
	}
}

// repair re-runs the migration/normalization Load applies after unmarshalling:
// per-empire Ensure* backfills, world-level backfills, re-pointing Config, and
// loading the league roster. Factored out so FileStore can reload the JSON into
// an EXISTING *World (keeping the caller's pointer valid) without duplicating
// this list.
func repair(w *game.World, cfg game.Config) {
	// The config goes on first: the world's own repairs read the sysop's
	// settings (EnsurePirates seeds the factions only when pirates are enabled).
	w.Config = cfg
	for _, e := range w.Empires {
		// SDI first: it reads the saved percentage to rebuild a pre-pool save's
		// funding, and EnsureRegions recomputes the percentage from the pool.
		e.EnsureSDIFunding()
		e.EnsureRegions()
		e.EnsureProduction()
	}
	w.EnsureInvestRate()
	w.EnsurePrefs() // older saves keep the preferences on the world
	w.EnsurePirates()
	w.EnsureIncoming()
	w.EnsureTreaties()
	w.EnsureMarket()
	// After the treaty and market migrations, not before: an over-full save
	// loses its surplus realms here, and dropping one has to forget the pacts
	// and market rows it left — which EnsureTreaties has only just moved out of
	// their legacy fields.
	w.EnsureSlots()
	w.EnsureAttackSlots()
	w.EnsureNews()
	// After EnsureSlots and the treaty/market migrations: retiring a baron frees
	// its slot and has to forget the pacts and market rows it leaves, which those
	// have only just moved out of their legacy fields. TEMPORARY (v0.2.0) — goes
	// with internal/game/retire_ai.go.
	w.RetireAIBarons()
	loadLeagueNodes(w, cfg)
	loadLeagueKeys(w, cfg)
}

// NodeListFile is the league roster filename. The clone isn't binary-compatible
// with BRE (its own packet format), so it uses its own name rather than BRE's
// BRNODES.DAT — the format is still the BRNODES layout ParseNodeList reads.
const NodeListFile = "ibnodes.dat"

// loadLeagueNodes loads the league roster from ibnodes.dat in the data dir. A
// missing file just means no roster — fine for a single-BBS game.
func loadLeagueNodes(w *game.World, cfg game.Config) {
	if nodes, err := ParseNodeList(filepath.Join(cfg.DataDir, NodeListFile)); err == nil {
		w.LeagueNodes = nodes
	}
}

// Save writes the world atomically (temp file + rename).
func Save(w *game.World, cfg game.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return err
	}
	stampClockOffset(w)
	// Both files are written in full before either is renamed into place, so a
	// write that fails (a full disk) leaves the pair on disk as it was. The
	// ledger is renamed first: a world saved without it would number its next
	// packet from a count the file never recorded. A save that stops between
	// the two renames is what the generation is for (ledger.Gen).
	gen := w.LedgerGen + 1
	data, err := json.MarshalIndent(savedWorld{w, gen}, "", "  ")
	if err != nil {
		return err
	}
	tmp := worldPath(cfg) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	ledgerTmp, err := writeLedgerTemp(w, cfg, gen)
	if err != nil {
		return err
	}
	if err := renameWorldFile(ledgerTmp, ledgerPath(cfg)); err != nil {
		return err
	}
	if err := renameWorldFile(tmp, worldPath(cfg)); err != nil {
		return err
	}
	w.LedgerGen = gen
	return nil
}

// BackupWorld copies the current world file to world.json.bak, so a destructive
// operation (like -reset) is recoverable. It reports whether a backup was made;
// a missing world file (a fresh install) is not an error and returns false.
func BackupWorld(cfg game.Config) (bool, error) {
	data, err := readWorldFile(worldPath(cfg))
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := os.WriteFile(worldPath(cfg)+".bak", data, 0o644); err != nil {
		return false, err
	}
	return true, nil
}
