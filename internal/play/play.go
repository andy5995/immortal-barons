// Package play is the shared session orchestration used by every front-end:
// lock the world, load it, run missed maintenance, find or onboard the
// caller's empire, show pending events, play, and save.
package play

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
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
// session (a door caller's remaining BBS minutes); 0 means unlimited. Language
// is a code the game ships translations for, from a drop file that names the
// caller's own language; "" means the board did not say, or said English.
// UserKey is the stable per-user key from DOORDROP.INI (USER_KEY); when set it
// takes precedence over Handle for finding the realm, and an existing realm
// keyed only by the handle adopts it.
type Identity struct {
	Handle   string
	UserKey  string
	TimeLeft time.Duration
	Language string
}

// userKey is the caller's stable per-user key, or "" when the drop file had
// none.
func (id Identity) userKey() string { return strings.TrimSpace(id.UserKey) }

// handleLock names the session lock for a realm owner's handle. The prefix
// keeps a USER_KEY and a handle of the same text from sharing a lock.
func handleLock(handle string) string {
	if h := game.OwnerKey(handle); h != "" {
		return "handle:" + h
	}
	return ""
}

// findCaller returns the realm belonging to this caller: the per-user key
// first, then the handle. A keyed caller does not get a realm another account's
// key already holds: a host may give a freed alias to a new account, and the
// key is what tells the two apart. It takes no mutation, so it is safe under
// Read.
func findCaller(w *game.World, id Identity) *game.Empire {
	if e := w.FindByUserKey(id.userKey()); e != nil {
		return e
	}
	e := w.FindByOwner(id.Handle)
	if e != nil && e.UserKey != "" && id.userKey() != "" {
		return nil
	}
	return e
}

// adoptUserKey records the caller's USER_KEY on a realm that was keyed only by
// its handle: a realm founded before IB read keys, by a format without one, or
// reassigned by the sysop. Call it under With, once the realm is bound.
func adoptUserKey(e *game.Empire, id Identity) {
	if e != nil && e.UserKey == "" && id.userKey() != "" {
		e.UserKey = id.userKey()
	}
}

// Run plays one session for the given caller. The returned reason describes how
// the session ended — "quit", "idle", "time", "disconnect", "closed", or "busy"
// when the realm is already being played — for the front-end's logs.
func Run(s session.Session, id Identity, cfg game.Config, today string) (reason string, err error) {
	// One session per realm. A BBS usually keeps a user to one node, but not
	// always, and two -local terminals can name the same handle; without this the
	// same realm could spend its daily allowances in two places at once. The lock
	// is held for the whole session and the OS drops it if the process dies.
	busy := func() (string, error) {
		term := menu.Term{UTF8: session.IsUTF8(s), ASCII: session.IsASCII(s), Plain: !session.HasANSI(s)}
		lang := menu.FittingLang(term, id.Language)
		notice(s, i18n.T(lang, "Your realm is already being played elsewhere."))
		return "busy", nil
	}
	held, err := lockRealm(cfg, id)
	if errors.Is(err, store.ErrBusy) {
		return busy()
	}
	if err != nil {
		return "", err
	}
	if held != nil {
		defer held.Release()
	}

	w, err := store.Load(cfg)
	if err != nil {
		return "", err
	}
	// The door/-local path is multi-node: every mutation is its own file-locked
	// transaction, so there is no session-long lock and no session-end save that
	// could clobber another node's concurrent changes. Load once for a stable
	// *World pointer, then route every With through the FileStore.
	w.SetStore(store.NewFileStore(w, cfg))

	// A keyed caller whose alias changed reaches a realm still owned by the old
	// handle, and a login without a key under that handle reaches it too, so
	// hold the owner's lock as well as the caller's own.
	var owner string
	w.Read(func() {
		if e := findCaller(w, id); e != nil {
			owner = e.Owner
		}
	})
	if k := handleLock(owner); held != nil && k != "" && k != handleLock(id.Handle) {
		if err := held.take(cfg, k); errors.Is(err, store.ErrBusy) {
			return busy()
		} else if err != nil {
			return "", err
		}
	}
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
		if e := findCaller(w, id); e != nil && !e.Alive {
			deadName = e.Name
		}
		maint = w.DailyMaintenance(today)
		if deadName != "" && findCaller(w, id) == nil {
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

// realmLock holds the session locks for one caller. A keyed caller takes its
// USER_KEY lock and its handle lock: the key is the stable identity, but a
// login through a format without a key still resolves to the same realm by
// handle, and the two must not play it at once.
type realmLock struct{ held []*store.FileLock }

func (l *realmLock) Release() {
	for i := len(l.held) - 1; i >= 0; i-- {
		l.held[i].Release()
	}
}

// take adds the lock named key, without waiting: a lock someone else holds
// gives store.ErrBusy. Each file is named by a hash of the key, so any key,
// however long, makes a valid file name.
func (l *realmLock) take(cfg game.Config, key string) error {
	sum := sha256.Sum256([]byte(key))
	held, err := store.LockPath(filepath.Join(cfg.DataDir, "sessions", hex.EncodeToString(sum[:])+".lock"), false)
	if err != nil {
		return err
	}
	l.held = append(l.held, held)
	return nil
}

// lockRealm takes the session locks for a caller, without waiting: a second
// login gets store.ErrBusy. An empty identity owns no realm and takes no lock
// (nil, nil).
func lockRealm(cfg game.Config, id Identity) (*realmLock, error) {
	var keys []string
	if k := id.userKey(); k != "" {
		keys = append(keys, "user:"+k)
	}
	if k := handleLock(id.Handle); k != "" {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return nil, nil
	}
	l := &realmLock{}
	for _, key := range keys {
		if err := l.take(cfg, key); err != nil {
			l.Release()
			return nil, err
		}
	}
	return l, nil
}

// maintNotice tells the caller what the login's daily maintenance did — that the
// world advanced (and by how many days) or was already current for today. Shown
// right after the splash, with no pause of its own so the opening menu follows
// immediately.
func maintNotice(s session.Session, r game.MaintReport) {
	switch {
	case r.NotStarted, r.Frozen:
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
	//
	// owner starts as the handle because this defer is installed before the
	// realm is bound; it is replaced with the realm's own owner once known, so a
	// keyed login that follows an alias change clears the right roster entry.
	owner := id.Handle
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
			markOffline(w, owner)
			err = save()
		}
	}()

	menu.Splash(s)
	// A frozen league lets nobody in: the Coordinator has stopped it so its
	// packets can drain before an upgrade (game/ibbs_freeze.go). Asked before
	// the maintenance notice, which has nothing to report while frozen.
	var frozen bool
	var freezeMsg string
	lang := id.Language
	w.Read(func() {
		frozen, freezeMsg = w.Frozen, w.FreezeMessage
		if e := findCaller(w, id); e != nil && e.Language != "" {
			lang = e.Language
		}
	})
	if frozen {
		menu.ShowLeagueFrozen(s, menu.FittingLang(term, lang), freezeMsg)
		return "frozen", save()
	}
	maintNotice(s, maint)

	var joinOpen bool
	var refusal string
	var joinDate string
	var e *game.Empire
	var localReborn string // a husk maintenance didn't sweep yet; swept here as a fallback
	var deadToday string   // realm destroyed today; no play until a later day
	var deadLang string    // that realm's language, for the refusal
	var dupeLocked string  // the league's duplicate-user check shut this caller out
	var lockedRealm string // the realm dupeLocked shut, named in the erase offer
	var lockedLang string  // that realm's language, for the offer
	var dupeJoin string    // a realm erased today whose baron another board still lists
	w.With(func() {
		e = findCaller(w, id)
		adoptUserKey(e, id)
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
				deadToday, deadLang = e.Name, e.Language
				// A baron another board still lists cannot found a new realm, so
				// the return-later promise below would be a lie. Covers a realm
				// erased at the dupe offer and any other death the same day.
				if board := w.DupeElsewhere(e.Owner); board != "" {
					dupeJoin = game.DupeJoinMessage(board).In(e.Language)
				}
			}
		}
		// A realm erased at the offer below is dead and still carries the lock,
		// so it takes the died-today refusal instead of the offer again.
		if e != nil && e.Alive && w.DupeLocked(e) {
			dupeLocked = w.DupeLockMessage(e).In(e.Language)
			lockedRealm, lockedLang = e.Name, e.Language
		}
		if e == nil {
			joinOpen = w.Config.JoinOpen(w.Today)
			refusal = joinRefusal(w, id.Handle, id.Language)
			joinDate = w.Config.JoinDate
		}
	})
	if e != nil {
		owner = e.Owner
	}
	if dupeLocked != "" {
		notice(s, dupeLocked)
		// BRE offers the way out here, since the baron may be locked out of the
		// other board too and could otherwise give up neither realm (#323). The
		// warning and the question are the menu's Abdicate's, in the realm's
		// language.
		if menu.ConfirmErase(onboardLang{Session: s, lang: lockedLang}, lockedRealm) {
			// Another node may have lifted the lock, or ended the realm, while the
			// baron read the question.
			var erased, unlocked bool
			w.With(func() {
				e := findCaller(w, id)
				switch {
				case e == nil || !e.Alive:
				case w.DupeLocked(e):
					w.Abdicate(e)
					erased = true
				default:
					unlocked = true
				}
			})
			switch {
			case erased:
				notice(s, i18n.T(lockedLang, "This realm has been erased."))
			case unlocked:
				notice(s, i18n.T(lockedLang, "This realm is no longer locked, so it was not erased. Call again to play it."))
			}
		}
		return "dupe", save()
	}
	if deadToday != "" {
		if dupeJoin != "" {
			notice(s, fmt.Sprintf(i18n.T(deadLang, "Your realm %s was destroyed."), deadToday))
			notice(s, dupeJoin)
		} else {
			notice(s, fmt.Sprintf(i18n.T(deadLang, "Your realm %s was destroyed. Return on a later day to build a new realm."), deadToday))
		}
		return "dead", save()
	}
	// rebornFrom (captured by the caller before login maintenance swept the husk)
	// or localReborn (a husk we swept just now): announce the fresh start before
	// onboarding prompts for a new realm name. A caller refused a new realm below
	// is told only that the old one fell, in the language of the refusal.
	if reborn := rebornFrom; reborn != "" || localReborn != "" {
		if reborn == "" {
			reborn = localReborn
		}
		if joinOpen && refusal == "" {
			notice(s, fmt.Sprintf(i18n.T(id.Language, "Your former realm %s was destroyed; you begin anew."), reborn))
		} else {
			notice(s, fmt.Sprintf(i18n.T(id.Language, "Your realm %s was destroyed."), reborn))
		}
	}
	if e == nil {
		if !joinOpen {
			notice(s, fmt.Sprintf("The game is closed to new barons (join cutoff %s has passed).", joinDate))
			return "closed", save()
		}
		if refusal != "" {
			notice(s, refusal)
			return "closed", save()
		}
		// First run: a brand-new player picks their UI language once, before
		// naming their realm. Returning players (found above) never reach here.
		// The signup prompt only appears in UTF-8 mode; a CP437 caller who wants a
		// CP437-representable language (e.g. German) sets it later from the
		// in-game Preferences menu, keeping the common English signup unchanged.
		//
		// A drop file that names the caller's language answers the question
		// before it is asked, so the picker is skipped entirely. It sets the new
		// empire's language only: a returning player was found above, and their
		// own choice from Preferences is not overridden by what a board says.
		lang := id.Language
		if lang == "" && term.UTF8 {
			lang = selectLanguage(s)
		}
		if lang != "" {
			// Translate the onboarding text (name prompt, Confirm?/Quit?) to the
			// language just picked — the empire doesn't exist yet, so menu.sessionLang
			// has nothing to read until GameLoop; this wrapper reports it early.
			s = onboardLang{Session: s, lang: lang}
		}

		// The board's rules, the instructions, the help browser and the
		// scoreboards, offered before the realm is named rather than two menus in
		// and a realm too late (#28). Quitting here leaves without a realm, which
		// is a normal way to end a first visit.
		create, err := menu.Welcome(s, w, id.Handle, lang, term)
		if err != nil {
			// An idle boot or a dropped connection at the Welcome menu ends the
			// session like it does everywhere else, not as a failure: the door
			// would otherwise exit 1 and tell the caller it "could not start".
			if errors.Is(err, session.ErrSessionEnded) || errors.Is(err, io.EOF) {
				reason = d.Reason()
				if reason == "" {
					reason = "disconnect"
				}
				return reason, save()
			}
			return "", err
		}
		if !create {
			return "quit", save()
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
				if existing := findCaller(w, id); existing != nil {
					e = existing
					return
				}
				// The slot is claimed inside the same lock that checked for one,
				// so two nodes onboarding at once cannot both take it.
				if full = joinRefusal(w, id.Handle, lang); full != "" {
					return
				}
				if w.RealmNameTaken(realm) {
					taken = true
					return
				}
				e = w.AddHumanKey(id.Handle, id.userKey(), realm)
				e.Language = lang
			})
			if full != "" {
				notice(s, full)
				return "closed", save()
			}
			if taken {
				fmt.Fprintf(s, "%s%s%s\n", ansi.FgBrightRed,
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

// notice prints a line the session ends on, or that comes before onboarding, in
// the yellow every such line takes, wrapped to the screen.
func notice(s session.Session, text string) {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgYellow, menu.WrapIndented(text, ""), ansi.Reset)
}

// joinRefusal is the sentence shown to a caller who may not found a realm, or
// "" when they may. Call it under the world lock.
//
// A caller another board in the league lists as one of its barons is refused
// first, as BRE refuses them (game.DupeElsewhere).
//
// The two capacity refusals are different facts and read differently. A planet with no
// free slot has hit the game's own ceiling and will reopen when a realm falls;
// the Max Players Per BBS cap is the sysop's choice and may never move. BRE
// tells a caller much the same in one message; this is IB's own wording.
func joinRefusal(w *game.World, handle, lang string) string {
	// Reached only when findCaller found no realm, so a realm owned by this
	// handle belongs to another account's key. A second realm under one handle
	// would leave the roster and packets unable to tell them apart.
	if w.FindByOwner(handle) != nil {
		return "Another account's realm is already played under your name. Ask the sysop to sort it out."
	}
	if board := w.DupeElsewhere(handle); board != "" {
		return game.DupeJoinMessage(board).In(lang)
	}
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
