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
	"unicode"

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
// the session ended — "quit", "idle", "time", "disconnect", "busy", or
// "closed" — for the front-end's logs.
func Run(s session.Session, id Identity, cfg game.Config, today string) (reason string, err error) {
	lock, err := store.Lock(cfg, false)
	if errors.Is(err, store.ErrBusy) {
		fmt.Fprintf(s, "\n%sThe game is busy — please try again shortly.%s\n", ansi.FgYellow, ansi.Reset)
		return "busy", nil
	}
	if err != nil {
		return "", err
	}
	defer lock.Release()

	w, err := store.Load(cfg)
	if err != nil {
		return "", err
	}
	w.Today = today
	w.DailyMaintenance(today)

	return Session(s, id, w, cfg, func() error { return store.Save(w, cfg) })
}

// Session plays one session against an already-loaded world owned by the
// caller. It does not take the flock, load, or run daily maintenance — the
// caller owns those. It calls save once at session end.
func Session(s session.Session, id Identity, w *game.World, cfg game.Config, save func() error) (reason string, err error) {
	// Bound the session: boot after IdleTimeoutSecs idle, or at the caller's
	// BBS time-left, so an abandoned session frees the world lock.
	var hard time.Time
	if id.TimeLeft > 0 {
		hard = time.Now().Add(id.TimeLeft)
	}
	// Read the charset capability before wrapping s in the Deadline (which does
	// not forward the marker): CP437 sessions get English-only, UTF-8 sessions
	// may use any language.
	utf8 := session.IsUTF8(s)
	d := session.NewDeadline(s, time.Duration(cfg.IdleTimeoutSecs)*time.Second, cfg.MaxIdleWarnings, hard)
	s = d

	// A boot/disconnect during the splash or onboarding unwinds via session.End
	// (GameLoop catches its own, but these run before it). Recover here so the
	// session ends cleanly and the world is still saved.
	defer func() {
		if r := recover(); r != nil {
			if _, ok := session.AsEnd(r); !ok {
				panic(r)
			}
			reason = d.Reason()
			if reason == "" {
				reason = "disconnect"
			}
			err = save()
		}
	}()

	menu.Splash(s)

	var joinOpen, boardFull bool
	var joinDate string
	var e *game.Empire
	w.With(func() {
		e = w.FindByOwner(id.Handle)
		if e == nil {
			joinOpen = w.Config.JoinOpen(w.Today)
			boardFull = w.BoardFull()
			joinDate = w.Config.JoinDate
		}
	})
	if e == nil {
		if !joinOpen {
			fmt.Fprintf(s, "\n%sThe game is closed to new barons (join cutoff %s has passed).%s\n", ansi.FgYellow, joinDate, ansi.Reset)
			return "closed", save()
		}
		if boardFull {
			fmt.Fprintf(s, "\n%sThis realm is full — no new barons may enroll.%s\n", ansi.FgYellow, ansi.Reset)
			return "closed", save()
		}
		// First run: a brand-new player picks their UI language once, before
		// naming their realm. Returning players (found above) never reach here.
		// The picker only appears in UTF-8 mode — a CP437 session can't display
		// non-English text, so it stays English.
		lang := ""
		if utf8 {
			lang = selectLanguage(s)
		}

		// Prompt for a realm name and insert atomically. Re-check under the
		// same lock that does the insert: while we prompted, another goroutine
		// may have onboarded this handle, filled the board, or claimed the name
		// we chose. On a name collision, re-prompt; the loop ends once we insert
		// (e != nil) or hit a terminal condition.
		for e == nil {
			realm := onboard(s, w, id.Handle, lang)
			var full, taken bool
			w.With(func() {
				if existing := w.FindByOwner(id.Handle); existing != nil {
					e = existing
					return
				}
				if w.BoardFull() {
					full = true
					return
				}
				if w.RealmNameTaken(realm) {
					taken = true
					return
				}
				e = w.AddHuman(id.Handle, realm)
				e.Language = lang
			})
			if full {
				fmt.Fprintf(s, "\n%sThis realm is full — no new barons may enroll.%s\n", ansi.FgYellow, ansi.Reset)
				return "closed", save()
			}
			if taken {
				fmt.Fprintf(s, "%s  That realm name was just taken — please choose another.%s\n", ansi.FgRed, ansi.Reset)
			}
		}
	}
	showEvents(s, w, e)

	// io.EOF means the caller dropped the connection or was booted; still persist
	// state below.
	gameErr := menu.GameLoop(s, w, e, utf8)
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
	return reason, save()
}

// languageOptions lists the selectable UI languages in menu order; index 0 is
// English (Empire.Language == ""), matching the field's own convention. Names
// are shown in their own language, not translated — a language picker's
// options aren't chrome the player has a language preference for yet.
var languageOptions = []struct{ code, name string }{
	{"", "English"},
	{"de", "Deutsch"},
	{"ru", "Русский"},
}

// selectLanguage is shown once, to a brand-new player, after the splash and
// before onboarding. Enter, an empty line, or any input that doesn't match a
// listed option selects English.
func selectLanguage(s session.Session) string {
	fmt.Fprintf(s, "\n%s%sSelect your language:%s\n", ansi.Clear, ansi.FgBrightCyan, ansi.Reset)
	for i, opt := range languageOptions {
		fmt.Fprintf(s, "  %s%d)%s %s\n", ansi.FgBrightWhite, i+1, ansi.Reset, opt.name)
	}
	fmt.Fprintf(s, "\n%sChoice (Enter for English):%s ", ansi.FgBrightWhite, ansi.Reset)
	line, err := session.ReadLine(s)
	if err != nil {
		return ""
	}
	n, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || n < 1 || n > len(languageOptions) {
		return ""
	}
	return languageOptions[n-1].code
}

func onboard(s session.Session, w *game.World, handle, lang string) string {
	var taken map[string]bool
	w.With(func() {
		taken = make(map[string]bool, len(w.Empires))
		for _, e := range w.Empires {
			taken[strings.ToLower(e.Name)] = true
		}
	})
	fmt.Fprint(s, ansi.Clear)
	for {
		fmt.Fprintf(s, "\n%s%s, %s%s ", ansi.FgBrightWhite, handle, i18n.T(lang, "Name your Realm:"), ansi.Reset)
		name, err := session.ReadLine(s)
		if err != nil {
			return handle // stream ended; fall back to the handle
		}
		name = strings.TrimSpace(name)
		if alnum(name) < 3 || taken[strings.ToLower(name)] {
			fmt.Fprintf(s, "%s  %s%s\n", ansi.FgRed, i18n.T(lang, "Invalid: at least 3 letters/numbers, not matching another realm."), ansi.Reset)
			continue
		}
		return name
	}
}

func showEvents(s session.Session, w *game.World, e *game.Empire) {
	var events []string
	w.With(func() {
		events = e.Events
		e.Events = nil
	})
	if len(events) == 0 {
		return
	}
	fmt.Fprintf(s, "%s%sWhile you were away:%s\n", ansi.Clear, ansi.FgBrightCyan, ansi.Reset)
	for _, ev := range events {
		fmt.Fprintf(s, "  %s\n", ev)
	}
	fmt.Fprintf(s, "\n%s-=<Paused>=-%s", ansi.FgBrightGreen, ansi.Reset)
	s.ReadKey() // intentional wait-for-keypress; result unused
}

func alnum(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			n++
		}
	}
	return n
}
