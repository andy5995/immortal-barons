// Package store persists the game world to disk. Save writes the world
// atomically (temp file + rename); Load reads it back, or returns a fresh
// world when no save exists yet.
package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
func NewGame(cfg game.Config) *game.World {
	w := game.NewWorld(cfg)
	loadLeagueNodes(w, cfg)
	loadLeagueKeys(w, cfg)
	return w
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
	return w, nil
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
		e.EnsureSupport()
		e.EnsureMorale()
		e.EnsureProduction()
	}
	w.EnsureInvestRate()
	w.EnsurePrefs() // older saves keep the preferences on the world
	w.EnsurePirates()
	w.EnsureTreaties()
	w.EnsureMarket()
	// After the treaty and market migrations, not before: an over-full save
	// loses its surplus realms here, and dropping one has to forget the pacts
	// and market rows it left — which EnsureTreaties has only just moved out of
	// their legacy fields.
	w.EnsureSlots()
	w.EnsureAttackSlots()
	w.EnsureNews()
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
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	tmp := worldPath(cfg) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return renameWorldFile(tmp, worldPath(cfg))
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
