package game

import (
	"errors"
	"fmt"
	"time"
)

// The SpyGuy — BRE's interplanetary watcher, and not a spy in the covert-agent
// sense. He costs GOLD rather than an agent, he is sent at a PLANET rather than
// a baron, he gathers no intelligence for the Spy Database, and he cannot be
// caught: he simply sits on the far planet for the days that were paid for and
// sends word home whenever that planet aims something at his own.
//
// The whole model is binary-verified. Sending is `run_bombing_operations_menu`
// (BRE.OVR 0x029ea9): it sums total_regions over every realm the SENDER holds,
// multiplies by SpyGuyGoldPerRegion for a price per day, offers a stay bounded
// by the caller's gold and SpyGuyMaxDays, charges days x price, and writes a
// SPY_GUY packet of three bytes — our board, their board, the days. Receiving
// is `ovr_044225_entry_0000`, which keeps the LONGER of the stay it already had
// and the one just paid for, then answers immediately with whatever is already
// aimed at the spy's planet (`show_gooie_arrival_time`, `estimate_attack_arrival`).
// `run_daily_maintenance` ("Processing SpyGuys") walks all 255 counters and
// decrements each one that is above zero.
//
// Every report travels as a NEWS_DATA record (`append_news_record`), so it
// reaches the spying planet as PLANET NEWS rather than as mail — and the
// watched planet is told nothing whatever. There is no discovery and no
// execution: the "found and executed" wording belongs to `investigate_traitors`
// and "was found spying on you" to the LOCAL Send Spy, both different mechanics.

var (
	ErrSpyGuyDays   = fmt.Errorf("A SpyGuy may stay between 1 and %d days.", SpyGuyMaxDays)
	ErrSpyGuyTarget = errors.New("Name the planet to watch.")
)

// SpyGuyDispatch rides the packet: a watcher sent by FromBoard for Days days.
type SpyGuyDispatch struct {
	FromBoard string
	Days      int
}

// SpyGuyCostPerDay prices a day of watching: the sending planet's own total
// regions times SpyGuyGoldPerRegion. A big planet pays more, which is BRE's
// own shape — the same one Terrorist Ops uses.
func (w *World) SpyGuyCostPerDay() int64 {
	var regions int64
	for _, e := range w.Empires {
		if e.Alive {
			regions += int64(e.Land)
		}
	}
	return regions * SpyGuyGoldPerRegion
}

// SpyGuyDaysAffordable is the longest stay e could pay for, capped at
// SpyGuyMaxDays. Zero means the planet cannot afford a single day.
func (w *World) SpyGuyDaysAffordable(e *Empire) int {
	cost := w.SpyGuyCostPerDay()
	if cost <= 0 {
		return SpyGuyMaxDays
	}
	days := int(e.Gold / cost)
	if days > SpyGuyMaxDays {
		days = SpyGuyMaxDays
	}
	return days
}

// SendSpyGuy posts a watcher to another planet for days days, charging the
// whole stay up front. The far board decides nothing: it is told how long the
// man was paid for and keeps him that long.
func (w *World) SendSpyGuy(e *Empire, board string, days int) error {
	if board == "" {
		return ErrSpyGuyTarget
	}
	if days < 1 || days > SpyGuyMaxDays {
		return ErrSpyGuyDays
	}
	cost := w.SpyGuyCostPerDay() * int64(days)
	if e.Gold < cost {
		return ErrCantAfford
	}
	e.Gold -= cost
	p := w.outboxFor(board)
	p.SpyGuys = append(p.SpyGuys, SpyGuyDispatch{FromBoard: w.Config.BoardID, Days: days})
	return nil
}

// receiveSpyGuy lodges an arriving watcher. A stay already paid for is never
// shortened by a new, briefer one — BRE compares the two and keeps the longer.
// The arrival is answered at once with the planet's current threats, so a
// watcher posted the day before a strike still earns his fee.
func (w *World) receiveSpyGuy(d SpyGuyDispatch) {
	if d.FromBoard == "" || d.Days < 1 {
		return
	}
	if w.SpyGuys == nil {
		w.SpyGuys = map[string]int{}
	}
	if d.Days > w.SpyGuys[d.FromBoard] {
		w.SpyGuys[d.FromBoard] = d.Days
	}
	w.reportStandingThreats(d.FromBoard)
}

// expireSpyGuys runs once a game day: every watcher's stay is a day shorter,
// and one whose days are spent is gone. Nobody is told, on either planet.
func (w *World) expireSpyGuys() {
	for board, days := range w.SpyGuys {
		if days <= 1 {
			delete(w.SpyGuys, board)
			continue
		}
		w.SpyGuys[board] = days - 1
	}
}

// watching reports whether that planet has a watcher here.
func (w *World) watching(board string) bool { return w.SpyGuys[board] > 0 }

// reportToSpy sends one line of news to a planet watching this one. It is
// planet news on arrival, not mail, which is what BRE's NEWS_DATA record makes
// it — every realm there reads it.
func (w *World) reportToSpy(board, line string) {
	if !w.watching(board) {
		return
	}
	p := w.outboxFor(board)
	p.News = append(p.News, line)
}

// reportStandingThreats tells a newly arrived watcher what is already aimed at
// his planet: a group attack assembling against it, and a Gooie Kablooie
// being built for it.
func (w *World) reportStandingThreats(board string) {
	for _, g := range w.GroupAttacks {
		if g.TargetBoard == board {
			w.reportToSpy(board, groupAttackSpyLine(w.Config.BoardID, g))
		}
	}
	if d := w.Annihilator; d != nil && d.TargetBoard == board && !d.Launched {
		w.reportToSpy(board, annihilatorSpyLine(w.Config.BoardID, d))
	}
}

// hoursUntil rounds up the whole hours left before t, floored at one: a strike
// leaving inside the hour is still leaving, and "in 0 hours" reads as a bug.
func hoursUntil(t time.Time) int {
	if t.IsZero() {
		return 1
	}
	d := time.Until(t)
	if d <= 0 {
		return 1
	}
	hours := int((d + time.Hour - 1) / time.Hour)
	if hours < 1 {
		hours = 1
	}
	return hours
}

// groupAttackSpyLine is the warning a watcher sends home about a strike being
// assembled. BRE names the hours because its group attacks are timed in hours,
// and IB's are too.
func groupAttackSpyLine(from string, g GroupAttack) string {
	hours := hoursUntil(g.DepartAt)
	if g.TargetEmpire == "" {
		return fmt.Sprintf("Our agent on %s reports a strike leaving for our planet in %d hours.", from, hours)
	}
	return fmt.Sprintf("Our agent on %s reports a strike leaving for %s in %d hours.", from, g.TargetEmpire, hours)
}

// annihilatorSpyLine reports the state of the weapon being built for us.
func annihilatorSpyLine(from string, d *Annihilator) string {
	if d.Funded {
		return fmt.Sprintf("Our agent on %s reports their Gooie Kablooie is funded and launches in %d hours.",
			from, AnnihilatorBuildDays*24)
	}
	return fmt.Sprintf("Our agent on %s reports work has begun on a Gooie Kablooie aimed at us.", from)
}
