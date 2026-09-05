package menu

import (
	"fmt"
	"sort"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/session"
)

// actions_ipscores.go — the InterBBS Scores board and the planet rankings
// drawn from it.

type ipRankKind int

const (
	ipRankPlanetScore ipRankKind = iota + 1
	ipRankPlanetNW
	ipRankPlanetLand
	ipRankPlanetDensity
	ipRankPlayerScore
	ipRankPlayerNW
	ipRankPlayerLand
	ipRankPlayerDensity
)

type ipScoreRow struct {
	name            string
	planet          string // empty for planet views
	score, nw, land int
}

// ipScoreRows gathers every realm the board knows of -- the league's, from the
// score packets each board files, and its own -- as the flat list both the
// on-screen rankings and the bulletin files rank.
func ipScoreRows(w *ctx) []ipScoreRow {
	var rows []ipScoreRow
	w.Read(func() {
		for _, b := range w.RemoteBoards {
			for _, sc := range b.Scores {
				score := sc.Score
				if score == 0 {
					score = sc.NetWorth
				}
				rows = append(rows, ipScoreRow{
					name:   sc.Empire,
					planet: b.BoardID,
					score:  score,
					nw:     sc.NetWorth,
					land:   sc.Land,
				})
			}
		}
		for _, e := range w.Empires {
			if e.Alive && e.Owner != "" {
				rows = append(rows, ipScoreRow{
					name:   e.Name,
					planet: w.Config.BoardID,
					score:  e.Score,
					nw:     w.NetWorth(e),
					land:   e.Land,
				})
			}
		}
	})
	return rows
}

// interbbsScores opens BRE's IP Scores submenu: eight ranking views (four
// planet-level, four player-level) each showing a ranked table. Captured from
// BRE (docs/dev/bre-screens.md, "InterBBS Scores").
func interbbsScores(s session.Session, w *ctx) Result {
	rows := ipScoreRows(w)
	if len(rows) == 0 {
		ok(s, "No inter-BBS scores have been imported yet.")
		return Stay
	}
	boxW := 42
	boxPad := (boxW - len("InterBBS Scores")) / 2
	for {
		fmt.Fprintf(s, "\n%s%s%s\n",
			ansi.FgBrightBlack, strings.Repeat("─", boxW), ansi.Reset)
		fmt.Fprintf(s, "%s%s%s%s%s\n",
			ansi.FgBrightBlack, strings.Repeat(" ", boxPad),
			tr(s, "InterBBS Scores"), ansi.Reset,
			strings.Repeat(" ", boxW-boxPad-len("InterBBS Scores")))
		for i, item := range []struct {
			key   byte
			label string
		}{
			{'1', "Top Planets by Score"},
			{'2', "Top Planets by Net Worth"},
			{'3', "Top Planets by Land"},
			{'4', "Top Planets by Net Worth Density"},
			{'5', "Top Players by Score"},
			{'6', "Top Players by Net Worth"},
			{'7', "Top Players by Land"},
			{'8', "Top Players by Net Worth Density"},
		} {
			fmt.Fprintf(s, "%s(%s%d%s) %s%s%s\n",
				ansi.FgBrightBlack, ansi.FgBrightWhite, i+1,
				ansi.FgBrightBlack, ansi.FgWhite, tr(s, item.label), ansi.Reset)
		}
		fmt.Fprintf(s, "%s(%s%c%s) %s%s%s\n",
			ansi.FgBrightBlack, ansi.FgBrightWhite, '0',
			ansi.FgBrightBlack, ansi.FgWhite, tr(s, "Quit"), ansi.Reset)
		fmt.Fprintf(s, "%s%s%s\n",
			ansi.FgBrightBlack, strings.Repeat("─", boxW), ansi.Reset)

		fmt.Fprintf(s, "\n%s>%s ", ansi.FgBrightWhite, ansi.Reset)
		r, err := readKey(s)
		if err != nil {
			return Stay
		}
		drainInput(s)
		if r == '\r' || r == '\n' || r == '0' {
			fmt.Fprintln(s, tr(s, "Quit"))
			return Stay
		}
		n := int(r - '0')
		if r < '1' || r > '8' {
			fmt.Fprintln(s)
			continue
		}
		fmt.Fprintf(s, "%d\n", n)
		ipScoreRank(s, w, rows, ipRankKind(n))
	}
}

// ipScoreViewName returns the BRE label for each ranking view.
func ipScoreViewName(s session.Session, kind ipRankKind) string {
	switch kind {
	case ipRankPlanetScore:
		return tr(s, "Top Planets by Score")
	case ipRankPlanetNW:
		return tr(s, "Top Planets by Net Worth")
	case ipRankPlanetLand:
		return tr(s, "Top Planets by Land")
	case ipRankPlanetDensity:
		return tr(s, "Top Planets by Net Worth Density")
	case ipRankPlayerScore:
		return tr(s, "Top Players by Score")
	case ipRankPlayerNW:
		return tr(s, "Top Players by Net Worth")
	case ipRankPlayerLand:
		return tr(s, "Top Players by Land")
	case ipRankPlayerDensity:
		return tr(s, "Top Players by Net Worth Density")
	}
	return ""
}

type planetAgg struct {
	name            string
	score, nw, land int
}

// ipScoreRank sorts and renders one BRE ranking view. Planet views aggregate
// scores per board; player views show individual empires with a Planet column.
// Captured from BRE (docs/dev/bre-screens.md, "InterBBS Scores").
func ipScoreRank(s session.Session, w *ctx, rows []ipScoreRow, kind ipRankKind) {
	renderIPScoreRank(s, w.Term, rows, kind)
	pause(s)
}

// renderIPScoreRank draws one ranking and nothing else. The bulletin files are
// this same screen written to a file rather than a terminal, so they must not
// be given a keypress to wait for.
func renderIPScoreRank(s session.Session, term Term, rows []ipScoreRow, kind ipRankKind) {
	metric := tr(s, "Score")
	switch kind {
	case ipRankPlanetScore, ipRankPlayerScore:
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].score != rows[j].score {
				return rows[i].score > rows[j].score
			}
			return rows[i].name < rows[j].name
		})
	case ipRankPlanetNW, ipRankPlayerNW:
		metric = tr(s, "Net Worth")
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].nw != rows[j].nw {
				return rows[i].nw > rows[j].nw
			}
			return rows[i].name < rows[j].name
		})
	case ipRankPlanetLand, ipRankPlayerLand:
		metric = tr(s, "Land")
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].land != rows[j].land {
				return rows[i].land > rows[j].land
			}
			return rows[i].name < rows[j].name
		})
	case ipRankPlanetDensity, ipRankPlayerDensity:
		metric = tr(s, "Net Worth / Region")
		sort.Slice(rows, func(i, j int) bool {
			di, dj := 0, 0
			if rows[i].land > 0 {
				di = rows[i].nw / rows[i].land
			}
			if rows[j].land > 0 {
				dj = rows[j].nw / rows[j].land
			}
			if di != dj {
				return di > dj
			}
			return rows[i].name < rows[j].name
		})
	}
	isPlanet := kind <= ipRankPlanetDensity
	var viewRows []struct {
		name, planet string
		val          int
	}
	if isPlanet {
		agg := map[string]*planetAgg{}
		for _, r := range rows {
			a, ok := agg[r.planet]
			if !ok {
				a = &planetAgg{name: r.planet}
				agg[r.planet] = a
			}
			a.score += r.score
			a.nw += r.nw
			a.land += r.land
		}
		var list []planetAgg
		for _, a := range agg {
			list = append(list, *a)
		}
		// Re-sort the aggregated list using the same comparator.
		switch kind {
		case ipRankPlanetScore:
			sort.Slice(list, func(i, j int) bool {
				if list[i].score != list[j].score {
					return list[i].score > list[j].score
				}
				return list[i].name < list[j].name
			})
		case ipRankPlanetNW:
			sort.Slice(list, func(i, j int) bool {
				if list[i].nw != list[j].nw {
					return list[i].nw > list[j].nw
				}
				return list[i].name < list[j].name
			})
		case ipRankPlanetLand:
			sort.Slice(list, func(i, j int) bool {
				if list[i].land != list[j].land {
					return list[i].land > list[j].land
				}
				return list[i].name < list[j].name
			})
		case ipRankPlanetDensity:
			sort.Slice(list, func(i, j int) bool {
				di, dj := 0, 0
				if list[i].land > 0 {
					di = list[i].nw / list[i].land
				}
				if list[j].land > 0 {
					dj = list[j].nw / list[j].land
				}
				if di != dj {
					return di > dj
				}
				return list[i].name < list[j].name
			})
		}
		for _, a := range list {
			val := metricValue(kind, a.score, a.nw, a.land)
			viewRows = append(viewRows, struct {
				name, planet string
				val          int
			}{a.name, "", val})
		}
	} else {
		for _, r := range rows {
			val := metricValue(kind, r.score, r.nw, r.land)
			viewRows = append(viewRows, struct {
				name, planet string
				val          int
			}{r.name, r.planet, val})
		}
	}
	viewName := ipScoreViewName(s, kind)
	rule := strings.Repeat("─", 72)
	// Title: "<game name>: <view name>" (bright-white + gray ": " + bright-red name).
	//
	// DELIBERATE DIVERGENCE, and one that must never be "corrected" back: the
	// original heads this screen with its OWN product name, so copying the layout
	// faithfully means substituting ours. A title is branding, not a mechanic —
	// see the licence note in the bre-gather skill. IB printed the original's name
	// here until 2026-08-23.
	fmt.Fprintf(s, "\n%s%s%s: %s%s%s\n", ansi.FgBrightWhite, tr(s, "Immortal Barons"),
		ansi.FgBrightBlack, ansi.FgBrightRed, viewName, ansi.Reset)
	// Planetary Post banner: ──═Planetary Post═── (red ──, bright-red ═, bright-white title).
	fmt.Fprintf(s, "%s                          %s──%s═%s%s%s═%s──%s\n",
		ansi.FgRed, ansi.FgRed, ansi.FgBrightRed,
		ansi.FgBrightWhite, tr(s, "Planetary Post"), ansi.FgBrightRed,
		ansi.FgRed, ansi.Reset)
	fmt.Fprintf(s, "\n")
	// Column header. Both table shapes end their metric on column 46; the player
	// views then put Planet LAST, beginning on column 52 (docs/dev/bre-screens.md).
	// The name field is 22 wide so the widest metric heading, "Net Worth / Region",
	// exactly fills the 18 columns right-aligned against 46.
	if isPlanet {
		fmt.Fprintf(s, "%s      %-22s%18s%s\n", ansi.FgBrightWhite, tr(s, "Name"), metric, ansi.Reset)
	} else {
		fmt.Fprintf(s, "%s      %-22s%18s     %s%s\n", ansi.FgBrightWhite, tr(s, "Name"), metric, tr(s, "Planet"), ansi.Reset)
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightBlack, rule, ansi.Reset)
	for i, r := range viewRows {
		if isPlanet {
			fmt.Fprintf(s, "%s(%s%3d%s) %s%s%s%s%18d%s\n",
				ansi.FgRed, ansi.FgBrightRed, i+1, ansi.FgRed,
				ansi.FgWhite, padColumn(term, r.name, 22), ansi.Reset,
				ansi.FgBrightWhite, r.val, ansi.Reset)
		} else {
			// The Planet column's own width is not captured; 21 ends it on the rule.
			fmt.Fprintf(s, "%s(%s%3d%s) %s%s%s%s%18d%s     %s%s%s\n",
				ansi.FgRed, ansi.FgBrightRed, i+1, ansi.FgRed,
				ansi.FgWhite, padColumn(term, r.name, 22), ansi.Reset,
				ansi.FgBrightWhite, r.val, ansi.Reset,
				ansi.FgWhite, fitColumn(term, r.planet, 21), ansi.Reset)
		}
	}
	fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightBlack, rule, ansi.Reset)
}

func metricValue(kind ipRankKind, score, nw, land int) int {
	switch kind {
	case ipRankPlanetScore, ipRankPlayerScore:
		return score
	case ipRankPlanetNW, ipRankPlayerNW:
		return nw
	case ipRankPlanetLand, ipRankPlayerLand:
		return land
	case ipRankPlanetDensity, ipRankPlayerDensity:
		if land > 0 {
			return nw / land
		}
	}
	return 0
}

// askOneOrAll asks BRE's whole-planet question and takes one key for it. The
// capture (docs/dev/bre-screens.md, "Create Group Attack") shows the "A" answer
// echoing "Entire Planet"; the "O" wording is IB's, since no capture has it. The
// prompt offers no default, so it waits for one of the two letters — a player
// who arrives here by mistake leaves BRE's own way, by sending no forces.
// Reports false, false when the session ends under it.
func askOneOrAll(s session.Session) (all, answered bool) {
	fmt.Fprintf(s, "\n%s ", tr(s, "Do you wish to target (O)ne Dominion or (A)ll?"))
	for {
		r, err := readKey(s)
		if err != nil {
			return false, false
		}
		drainInput(s) // drop a trailing Enter typed with the single-key answer
		switch r {
		case 'a', 'A':
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightWhite, tr(s, "Entire Planet"), ansi.Reset)
			return true, true
		case 'o', 'O':
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightWhite, tr(s, "One Dominion"), ansi.Reset)
			return false, true
		}
	}
}

// fitColumn cuts text to width with the truncation marker the caller's terminal
// will actually render in one column. The CP437 and plain-ASCII writers rewrite
// "…" as three ASCII dots below every layer that counts columns, so a fitted
// cell came out two columns wide for them and shifted every column after it
// (#196) — the common case, since CP437 is the default.
// Take the charset from Term rather than asking the Session. Every wrapper does
// forward the marker now, so the two agree — but Term is captured once in
// play.Run and passed down, which is what makes a cell's width the same on
// every screen that draws one, however the session was wrapped for that call.
//
// It measures in COLUMNS, not runes, so a value carrying a character the writer
// expands — an em dash in a realm name, which ValidRealmName accepts — is cut to
// what will actually fit rather than to a rune count that renders wider.
func fitColumn(t Term, text string, width int) string {
	if visWidth(t, text) <= width {
		return text
	}
	mark := "…"
	if !t.UTF8 {
		mark = "..."
	}
	room := width - visWidth(t, mark)
	if room <= 0 {
		return trimToWidth(t, text, width)
	}
	return trimToWidth(t, text, room) + mark
}

// padColumn fits text into width and pads it to exactly that many COLUMNS on
// the caller's terminal — what a table cell holding player-controlled text
// needs and a `%-*s` verb cannot give it. fmt measures RUNES and never
// truncates, so such a cell both overran its column (a realm name may run to
// game.RealmNameMaxChars, wider than several of the cells that carry one) and
// under-padded on a charset whose writer expands a rune ("—", "…" and the
// curly quotes on CP437; a much wider set under -ascii).
func padColumn(t Term, text string, width int) string {
	text = fitColumn(t, text, width)
	return text + strings.Repeat(" ", max(width-visWidth(t, text), 0))
}

// trimToWidth drops runes off the end until what is left fits width columns on
// the caller's terminal.
func trimToWidth(t Term, text string, width int) string {
	r := []rune(text)
	for len(r) > 0 && visWidth(t, string(r)) > width {
		r = r[:len(r)-1]
	}
	return string(r)
}

// visWidth is how many columns text will really occupy on the caller's
// terminal. len([]rune()) is only that for a UTF-8 caller: the CP437 and
// plain-ASCII writers rewrite "—", "…", "→" and the rest as two or three ASCII
// characters, and they sit below every layer that counts columns, so a fixed
// rule measured in runes came out short and the drawn line overhung it (#192).
// Term, not session.IsUTF8(s), for the reason given on fitColumn.
//
// The three charsets each measure differently, and ToASCII does NOT stand in for
// CP437: it expands a dozen characters CP437 draws in one column (±, °, ½, ß,
// Σ, ≤ …), so a realm named "±Tatooine" was padded two columns short and every
// figure on its row sat left of the heading. CP437 is the default charset, so
// that is the common case.
func visWidth(t Term, text string) int {
	switch {
	case t.UTF8:
		return len([]rune(text))
	case t.ASCII:
		return len([]rune(session.ToASCII(text)))
	default:
		return session.CP437Width(text)
	}
}
