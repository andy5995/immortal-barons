// Package play is the shared session orchestration used by every front-end:
// lock the world, load it, run missed maintenance, find or onboard the
// caller's empire, show pending events, play, and save.
package play

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// Identity is who is playing. TimeLeft, when > 0, is a hard cap on the whole
// session (a door caller's remaining BBS minutes); 0 means unlimited.
type Identity struct {
	Handle   string
	TimeLeft time.Duration
}

// Run plays one session for the given caller. The returned reason describes how
// the session ended — "quit", "idle", "time", "disconnect", or "closed" — for
// the front-end's logs.
func Run(s session.Session, id Identity, cfg game.Config, today string) (reason string, err error) {
	w, err := store.Load(cfg)
	if err != nil {
		return "", err
	}
	// The door/-local path is multi-node: every mutation is its own file-locked
	// transaction, so there is no session-long lock and no session-end save that
	// could clobber another node's concurrent changes. Load once for a stable
	// *World pointer, then route every With through the FileStore.
	w.SetStore(store.NewFileStore(w, cfg))
	// A transaction that could not be committed ends the session with an error
	// rather than a normal exit. With() cannot return one (see game.With), so it
	// records the first failure and this reports it on whichever path Run leaves
	// by — including the ones inside Session. Without this the door exits 0 on a
	// failed save and the caller's turn is silently gone, which is exactly how a
	// Windows CI failure read as "its process's write was lost" with no error
	// anywhere in the log.
	defer func() {
		if err == nil {
			if storeErr := w.StoreErr(); storeErr != nil {
				err = fmt.Errorf("the world could not be saved, so this session was not recorded: %w", storeErr)
			}
		}
	}()
	// Login date-rollover maintenance is itself a transaction (reload → maintain
	// → save), so it can't race another node's login. Note whether the caller's
	// realm was a dead husk that this maintenance sweeps, so Session can announce
	// the fresh start (the husk is gone by the time Session binds the empire).
	var rebornFrom string
	var maint game.MaintReport
	var bulletinErr error
	w.With(func() {
		w.Today = today
		var deadName string
		if e := w.FindByOwner(id.Handle); e != nil && !e.Alive {
			deadName = e.Name
		}
		maint = w.DailyMaintenance(today)
		if deadName != "" && w.FindByOwner(id.Handle) == nil {
			rebornFrom = deadName
		}
		// A bulletin the sysop copied in since the last login is news, and the
		// login transaction is the one pass every board makes — a stand-alone
		// board never runs the planetary step where the league's are reconciled.
		_, bulletinErr = store.SyncBulletins(w)
	})
	if bulletinErr != nil {
		return "", bulletinErr
	}

	// Each action already persisted via its Transact; the session-end save is a
	// no-op here (saving w's in-memory state would overwrite concurrent nodes).
	return Session(s, id, w, cfg, rebornFrom, maint, func() error { return nil })
}

// maintNotice tells the caller what the login's daily maintenance did — that the
// world advanced (and by how many days) or was already current for today. Shown
// right after the splash, with no pause of its own so the opening menu follows
// immediately.
func maintNotice(s session.Session, r game.MaintReport) {
	switch {
	case r.NotStarted:
		// Nothing ran — the game hasn't started yet; the opening menu shows the
		// start date, so no notice is needed here.
	case r.Days > 0:
		// The original's shape: a marked header, then one indented line per
		// stage as it is carried out, closed by a bright completion line
		// (docs/dev/bre-screens.md, captured live). The steps come from the run
		// itself, so a stage with nothing to do prints no line.
		//
		// The STEP NAMES are a deliberate divergence: they are IB's own tasks in
		// IB's own words, since half of BRE's list belongs to file formats IB
		// does not have. Recorded in bre-screens.md and mechanics-reference.md
		// beside the shape they keep.
		fmt.Fprintf(s, "\n%s\u25a0  %s%s%s\n", ansi.FgBrightYellow, ansi.FgWhite, "Running daily maintenance", ansi.Reset)
		for _, step := range r.Steps {
			fmt.Fprintf(s, "%s%s%s\n", ansi.FgWhite, menu.WrapIndented(step, "     "), ansi.Reset)
		}
		fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightWhite, menu.WrapIndented("Daily maintenance complete", "     "), ansi.Reset)
	default:
		fmt.Fprintf(s, "\n%sMaintenance has already been run today.%s\n", ansi.FgWhite, ansi.Reset)
	}
}

// Session plays one session against an already-loaded world owned by the
// caller. It does not take the flock, load, or run daily maintenance — the
// caller owns those. save is called once at session end: a front-end holding
// one in-memory world persists it there, while the door passes a no-op because
// every action already committed through its FileStore transaction.
func Session(s session.Session, id Identity, w *game.World, cfg game.Config, rebornFrom string, maint game.MaintReport, save func() error) (reason string, err error) {
	// Bound the session: boot after IdleTimeoutSecs idle, or at the caller's
	// BBS time-left, so an abandoned session frees the world lock.
	var hard time.Time
	if id.TimeLeft > 0 {
		hard = time.Now().Add(id.TimeLeft)
	}
	// Read the terminal capabilities before wrapping s in the Deadline (which does
	// not forward the markers): CP437 sessions get English-only, UTF-8 sessions
	// may use any language, and a session that cannot render ANSI gets numbered
	// lists in place of lightbars.
	term := menu.Term{UTF8: session.IsUTF8(s), ASCII: session.IsASCII(s), Plain: !session.HasANSI(s)}
	d := session.NewDeadline(s, time.Duration(cfg.IdleTimeoutSecs)*time.Second, cfg.MaxIdleWarnings, hard)
	s = d

	// A boot/disconnect during the splash or onboarding unwinds via session.End
	// (GameLoop catches its own, but these run before it). Recover here so the
	// session ends cleanly and the world is still saved.
	defer func() {
		if r := recover(); r != nil {
			endErr, ok := session.AsEnd(r)
			if !ok {
				panic(r)
			}
			reason = d.Reason()
			if reason == "" {
				reason = "disconnect"
			}
			// Surface a genuine read error (e.g. a winsock errno hit during the
			// splash/onboarding, before GameLoop installs its own guard) in the
			// reason, so the door log records WHY the session ended and not just
			// "disconnect". A clean EOF/boot carries no extra detail.
			if endErr != nil && !errors.Is(endErr, io.EOF) {
				reason = fmt.Sprintf("%s (%v)", reason, endErr)
			}
			// id.Handle, not e.Owner: this defer is installed before e is
			// declared. FindByOwner lowercases and trims, so both resolve.
			markOffline(w, id.Handle)
			err = save()
		}
	}()

	menu.Splash(s)
	maintNotice(s, maint)

	var joinOpen bool
	var refusal string
	var joinDate string
	var e *game.Empire
	var localReborn string // a husk maintenance didn't sweep yet; swept here as a fallback
	var deadToday string   // realm destroyed today; no play until a later day
	var dupeLocked string  // the league's duplicate-user check shut this caller out
	w.With(func() {
		e = w.FindByOwner(id.Handle)
		if e != nil && !e.Alive {
			if w.GameDay > e.DiedDay {
				// A past-day husk maintenance hasn't swept: sweep it now and let
				// the fresh-onboard path below build a new realm.
				localReborn = e.Name
				w.RemoveEmpire(e)
				e = nil
			} else {
				// Died today: keep the husk and end the session; the owner
				// rebuilds on a later login.
				deadToday = e.Name
			}
		}
		if e != nil && w.DupeLocked(e) {
			dupeLocked = w.DupeLockMessage(e)
		}
		if e == nil {
			joinOpen = w.Config.JoinOpen(w.Today)
			refusal = joinRefusal(w)
			joinDate = w.Config.JoinDate
		}
	})
	if dupeLocked != "" {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, menu.WrapIndented(dupeLocked, ""), ansi.Reset)
		return "dupe", save()
	}
	if deadToday != "" {
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow,
			menu.WrapIndented(fmt.Sprintf("Your realm %s was destroyed. Return on a later day to build a new realm.", deadToday), ""),
			ansi.Reset)
		return "dead", save()
	}
	// rebornFrom (captured by the caller before login maintenance swept the husk)
	// or localReborn (a husk we swept just now): announce the fresh start before
	// onboarding prompts for a new realm name.
	if reborn := rebornFrom; reborn != "" || localReborn != "" {
		if reborn == "" {
			reborn = localReborn
		}
		fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow,
			menu.WrapIndented(fmt.Sprintf("Your former realm %s was destroyed; you begin anew.", reborn), ""),
			ansi.Reset)
	}
	if e == nil {
		if !joinOpen {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow,
				menu.WrapIndented(fmt.Sprintf("The game is closed to new barons (join cutoff %s has passed).", joinDate), ""),
				ansi.Reset)
			return "closed", save()
		}
		if refusal != "" {
			fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, menu.WrapIndented(refusal, ""), ansi.Reset)
			return "closed", save()
		}
		// First run: a brand-new player picks their UI language once, before
		// naming their realm. Returning players (found above) never reach here.
		// The signup prompt only appears in UTF-8 mode; a CP437 caller who wants a
		// CP437-representable language (e.g. German) sets it later from the
		// in-game Preferences menu, keeping the common English signup unchanged.
		lang := ""
		if term.UTF8 {
			lang = selectLanguage(s)
		}
		if lang != "" {
			// Translate the onboarding text (name prompt, Confirm?/Quit?) to the
			// language just picked — the empire doesn't exist yet, so menu.sessionLang
			// has nothing to read until GameLoop; this wrapper reports it early.
			s = onboardLang{Session: s, lang: lang}
		}

		// Prompt for a realm name and insert atomically. Re-check under the
		// same lock that does the insert: while we prompted, another goroutine
		// may have onboarded this handle, filled the board, or claimed the name
		// we chose. On a name collision, re-prompt; the loop ends once we insert
		// (e != nil) or hit a terminal condition.
		for e == nil {
			realm, quit := onboard(s, w, id.Handle, lang)
			if quit {
				return "quit", save()
			}
			var full string
			var taken bool
			w.With(func() {
				if existing := w.FindByOwner(id.Handle); existing != nil {
					e = existing
					return
				}
				// The slot is claimed inside the same lock that checked for one,
				// so two nodes onboarding at once cannot both take it.
				if full = joinRefusal(w); full != "" {
					return
				}
				if w.RealmNameTaken(realm) {
					taken = true
					return
				}
				e = w.AddHuman(id.Handle, realm)
				e.Language = lang
			})
			if full != "" {
				fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, menu.WrapIndented(full, ""), ansi.Reset)
				return "closed", save()
			}
			if taken {
				fmt.Fprintf(s, "%s%s%s\n", ansi.FgRed,
					menu.WrapIndented("That realm name was just taken — please choose another.", "  "), ansi.Reset)
			}
		}
	}
	// The "since your last play" events are shown by runTurn after the caller
	// hits Play (their first turn of the day), matching BRE — not here before the
	// opening menu.

	// Put the caller on the online roster before the opening menu is drawn, not
	// at their first action. Someone who has used their turns sits at that menu
	// without acting, which is exactly when another baron wants to know they are
	// reachable (#123) — waiting for an action would leave them invisible for
	// the whole visit.
	markActive(w, e.Owner)

	// io.EOF means the caller dropped the connection or was booted; still persist
	// state below.
	gameErr := menu.GameLoop(s, w, e.Owner, term)
	reason = d.Reason()
	if reason == "" {
		if errors.Is(gameErr, io.EOF) {
			reason = "disconnect"
		} else {
			reason = "quit"
		}
	}
	if gameErr != nil && !errors.Is(gameErr, io.EOF) {
		return reason, gameErr
	}
	markOffline(w, e.Owner)
	return reason, save()
}

// markActive puts the caller on the online roster. It is re-resolved by handle
// rather than reusing a bound *Empire because a reload may have replaced every
// pointer since.
func markActive(w *game.World, handle string) {
	w.With(func() {
		if e := w.FindByOwner(handle); e != nil {
			e.MarkActive()
		}
	})
}

// markOffline drops the caller the moment their session ends rather than
// leaving them there for OnlineWindowSecs. Both callers sit immediately before
// a save, since an unsaved clear would not reach the other nodes. The one exit
// that skips it is a genuine GameLoop error, which returns without saving at
// all; that session falls back to the window.
func markOffline(w *game.World, handle string) {
	w.With(func() {
		if e := w.FindByOwner(handle); e != nil {
			e.MarkOffline()
		}
	})
}

// selectLanguage is shown once, to a brand-new player, after the splash and
// before onboarding. The options are English (always index 1, the default and
// fallback for untranslated text) followed by i18n.Languages, so adding a
// language to that single registry surfaces it here automatically. Names are
// shown in their own language, not translated. Enter, an empty line, or any
// unmatched input selects English.
func selectLanguage(s session.Session) string {
	type opt struct{ code, name string }
	opts := []opt{{"", "English"}}
	for _, l := range i18n.Languages {
		opts = append(opts, opt{l.Code, l.Name})
	}
	fmt.Fprintf(s, "\n%sSelect your language:%s\n", ansi.FgBrightCyan, ansi.Reset)
	for i, o := range opts {
		fmt.Fprintf(s, "  %s%d)%s %s\n", ansi.FgBrightWhite, i+1, ansi.Reset, o.name)
	}
	fmt.Fprintf(s, "\n%sChoice (Enter for English):%s ", ansi.FgBrightWhite, ansi.Reset)
	line, err := session.ReadLine(s)
	if err != nil {
		// Same as the realm-name prompt below: a boot or hang-up here ends the
		// session instead of falling through to onboarding, which would only be
		// booted again at its own first read.
		session.End(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(opts) {
		return ""
	}
	return opts[n-1].code
}

// onboardLang wraps the session during first-run onboarding so text shown before
// the empire exists (the realm-name prompt, Confirm?/Quit?) translates to the
// language just picked — menu.sessionLang reads a Lang() method. SetInputLine is
// forwarded so an idle warning can still reprint the current input line.
type onboardLang struct {
	session.Session
	lang string
}

func (o onboardLang) Lang() string { return o.lang }

func (o onboardLang) SetInputLine(line string) {
	if ls, ok := o.Session.(session.InputLineSetter); ok {
		ls.SetInputLine(line)
	}
}

// DrainInput forwards the single-key trailing-terminator drain (menu.AskYesNo
// drains after the onboarding Quit?/Confirm? answers) to the inner session.
func (o onboardLang) DrainInput() { session.Drain(o.Session) }

// joinRefusal is the sentence shown to a caller who may not found a realm, or
// "" when they may. Call it under the world lock.
//
// The two refusals are different facts and read differently. A planet with no
// free slot has hit the game's own ceiling and will reopen when a realm falls;
// the Max Players Per BBS cap is the sysop's choice and may never move. BRE
// tells a caller much the same in one message; this is IB's own wording.
func joinRefusal(w *game.World) string {
	if w.PlanetFull() {
		return fmt.Sprintf("All %d realms of this planet are held. Call back when one has fallen.", game.PlanetSlots)
	}
	if w.BoardFull() {
		return "This realm is full — no new barons may enroll."
	}
	return ""
}

// onboard names a brand-new player's realm, and reports quit when they chose to
// leave without creating one (the caller then ends the session). The screen
// itself is menu.AskRealmName, shared with the rename in Preferences, so both
// ask, reject and confirm in the same words. Enter on an empty line offers the
// way out — "Quit? (n,Y)" — so a player who reached the prompt by mistake is not
// forced to invent a realm.
func onboard(s session.Session, w *game.World, handle, lang string) (name string, quit bool) {
	var taken map[string]bool
	w.Read(func() {
		taken = make(map[string]bool, len(w.Empires))
		for _, e := range w.Empires {
			taken[strings.ToLower(e.Name)] = true
			// A renamed realm holds its old names too — packets in the air still
			// address them (game.RenameEmpire).
			for _, old := range e.PriorRealmNames() {
				taken[strings.ToLower(old)] = true
			}
		}
	})
	return menu.AskRealmName(s, lang, handle+", ",
		func(n string) bool { return taken[strings.ToLower(n)] },
		func() bool { return menu.AskYesNo(s, "Quit?", true) })
}
