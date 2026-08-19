package main

import (
	"bufio"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/numfmt"
	"github.com/andy5995/immortal-barons/internal/store"
)

// The sysop's player editor (#161), after BRE's VIEW command (manage_players,
// BRE.OVR 0x405a): list the players and, for one of them, change the caller's
// name, change the realm's name, or delete the realm. It is a command-line mode
// rather than a game menu because it is sysop work, and because two of the
// three edits are things no player may do to themselves.
//
// The original browses one realm per screen — up and down arrows step through
// the roster, a letter jumps to that realm, ESC leaves, and RETURN opens the
// action prompt. IB keeps the parts that survive a plain terminal: the realm is
// picked by its ROSTER LETTER, which is the identity every game screen already
// addresses it by, and the action prompt carries BRE's keys in BRE's order
// (Delete, Player name, Empire name, Quit). The arrow-key browse is not
// reproduced — this mode reads a pipe as readily as a terminal, and a raw-mode
// cursor needs neither. E for "Empire Name" is R here, since a realm is what IB
// calls the thing everywhere else.
//
// Each edit is its own locked transaction — take the lock, re-read the world,
// apply, write back, release — which is what every door action does. The
// prompts run OUTSIDE the lock, so a sysop who wanders off mid-edit never holds
// a caller's turn or the nightly maintenance up. The realm is re-found by its
// caller handle each time, so an edit made against a stale listing (another
// node removed the realm meanwhile) fails to find it and says so instead of
// writing over whatever took its place.

// playerRow is one line of the listing, snapshotted so the world is not held
// open across the prompts.
type playerRow struct {
	handle string
	realm  string
	id     string // the realm's roster letter, as every game screen addresses it
	worth  int
	played string
	alive  bool
}

func runPlayers(cfg game.Config, in io.Reader, out io.Writer) error {
	r := bufio.NewReader(in)
	for {
		rows, err := listPlayers(cfg)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			// The original says so plainly rather than showing an empty table.
			fmt.Fprintln(out, "No players are in this game yet.")
			return nil
		}
		printPlayers(out, rows)
		choice, eof := ask(r, out, "Which realm (letter, or Q to quit)? ")
		if eof || choice == "" || strings.EqualFold(choice, "q") {
			fmt.Fprintln(out)
			return nil
		}
		row, ok := rowByLetter(rows, choice)
		if !ok {
			fmt.Fprintf(out, "No realm answers to %s.\n\n", strings.ToUpper(choice))
			continue
		}
		if err := editPlayer(cfg, r, out, row); err != nil {
			return err
		}
	}
}

// listPlayers snapshots the caller-owned realms, in roster order. Computer
// barons are left out: they belong to nobody, so none of the three edits means
// anything for one.
func listPlayers(cfg game.Config) ([]playerRow, error) {
	w, err := store.Load(cfg)
	if err != nil {
		return nil, err
	}
	var rows []playerRow
	for _, e := range w.Empires {
		if e.Owner == "" {
			continue
		}
		rows = append(rows, playerRow{
			handle: e.Owner, realm: e.Name, id: e.Letter(),
			worth: w.NetWorth(e), played: e.LastPlayed, alive: e.Alive,
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	return rows, nil
}

func rowByLetter(rows []playerRow, letter string) (playerRow, bool) {
	for _, r := range rows {
		if strings.EqualFold(r.id, letter) {
			return r, true
		}
	}
	return playerRow{}, false
}

func printPlayers(out io.Writer, rows []playerRow) {
	fmt.Fprintf(out, "\n%-2s %-22s %-20s %13s  %s\n", "Id", "Realm", "Player", "Net Worth", "Last played")
	fmt.Fprintln(out, strings.Repeat("-", 76))
	for _, r := range rows {
		realm := r.realm
		if !r.alive {
			realm += " (eliminated)"
		}
		played := r.played
		if played == "" {
			played = "never"
		}
		fmt.Fprintf(out, "%-2s %-22s %-20s %13s  %s\n", r.id, realm, r.handle, numfmt.Comma(int64(r.worth)), played)
	}
	fmt.Fprintln(out)
}

// editPlayer runs the action prompt for one realm. The keys and their order are
// BRE's: Delete, Player name, Empire name, Quit.
func editPlayer(cfg game.Config, r *bufio.Reader, out io.Writer, row playerRow) error {
	for {
		fmt.Fprintf(out, "%s (%s)\n", row.realm, row.handle)
		choice, eof := ask(r, out, "[D]elete realm, [P]layer name, [R]ealm name, or [Q]uit? ")
		if eof {
			return nil
		}
		switch strings.ToUpper(choice) {
		case "", "Q":
			fmt.Fprintln(out)
			return nil
		case "P":
			name, eof := ask(r, out, "Change player name to? ")
			if eof || name == "" {
				continue
			}
			if err := withPlayer(cfg, row.handle, func(w *game.World, e *game.Empire) error {
				return w.RenameOwner(e, name)
			}); err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			row.handle = strings.ToLower(strings.TrimSpace(name))
			fmt.Fprintf(out, "%s now belongs to %s.\n\n", row.realm, row.handle)
		case "R":
			name, eof := ask(r, out, "Change realm name to? ")
			if eof || name == "" {
				continue
			}
			if err := withPlayer(cfg, row.handle, func(w *game.World, e *game.Empire) error {
				return w.SysopRenameEmpire(e, name)
			}); err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			fmt.Fprintf(out, "%s is now known as %s.\n\n", row.realm, strings.TrimSpace(name))
			row.realm = strings.TrimSpace(name)
		case "D":
			// Deleting is the one edit nothing can undo: the realm's land,
			// treasury, treaties and market position all go with it. BRE asks
			// "Are you sure?" and takes only Y; so does this.
			answer, eof := ask(r, out, fmt.Sprintf("Delete %s? This cannot be undone. Are you sure? (y/N) ", row.realm))
			if eof || !yes(answer) {
				fmt.Fprintf(out, "%s is unchanged.\n\n", row.realm)
				continue
			}
			if err := withPlayer(cfg, row.handle, func(w *game.World, e *game.Empire) error {
				w.RemoveEmpire(e)
				return nil
			}); err != nil {
				fmt.Fprintln(out, err)
				continue
			}
			fmt.Fprintf(out, "%s is gone. %s may build a fresh realm at their next login.\n\n", row.realm, row.handle)
			return nil
		default:
			fmt.Fprintf(out, "There is no option %s.\n\n", choice)
		}
	}
}

// withPlayer runs one edit as a single locked transaction: lock, re-read, find
// the realm by its caller handle, apply, save. The re-read is what makes the
// listing safe to act on — nothing is written against a world that has moved on
// since it was printed.
func withPlayer(cfg game.Config, handle string, fn func(*game.World, *game.Empire) error) error {
	lock, err := store.Lock(cfg, true)
	if err != nil {
		return err
	}
	defer lock.Release()
	w, err := store.Load(cfg)
	if err != nil {
		return err
	}
	e := w.FindByOwner(handle)
	if e == nil {
		return fmt.Errorf("no realm belongs to %s any more; it may have been removed since this list was printed", handle)
	}
	if err := fn(w, e); err != nil {
		return err
	}
	return store.Save(w, cfg)
}

// ask prints a prompt and reads one trimmed line. It reports EOF separately so
// a piped-in script that runs out of input ends the mode instead of looping on
// an empty read.
func ask(r *bufio.Reader, out io.Writer, prompt string) (answer string, eof bool) {
	fmt.Fprint(out, prompt)
	line, err := r.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(out)
		return "", true
	}
	return strings.TrimSpace(line), false
}

func yes(answer string) bool {
	a := strings.ToLower(answer)
	return a == "y" || a == "yes"
}
