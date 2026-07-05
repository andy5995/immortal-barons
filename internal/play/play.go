// Package play is the shared session orchestration used by every front-end:
// lock the world, load it, run missed maintenance, find or onboard the
// caller's empire, show pending events, play, and save.
package play

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
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
	d := session.NewDeadline(s, time.Duration(cfg.IdleTimeoutSecs)*time.Second, cfg.MaxIdleWarnings, hard)
	s = d

	menu.Splash(s)

	e := w.FindByOwner(id.Handle)
	if e == nil {
		if !w.Config.JoinOpen(w.Today) {
			fmt.Fprintf(s, "\n%sThe game is closed to new barons (join cutoff %s has passed).%s\n", ansi.FgYellow, w.Config.JoinDate, ansi.Reset)
			return "closed", save()
		}
		if w.BoardFull() {
			fmt.Fprintf(s, "\n%sThis realm is full — no new barons may enroll.%s\n", ansi.FgYellow, ansi.Reset)
			return "closed", save()
		}
		realm := onboard(s, w, id.Handle)
		w.With(func() { e = w.AddHuman(id.Handle, realm) })
	}
	showEvents(s, e)

	// io.EOF means the caller dropped the connection or was booted; still persist
	// state below.
	gameErr := menu.GameLoop(s, w, e)
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

func onboard(s session.Session, w *game.World, handle string) string {
	taken := map[string]bool{}
	for _, e := range w.Empires {
		taken[strings.ToLower(e.Name)] = true
	}
	fmt.Fprintf(s, "%s%sName Your Empire%s\n", ansi.Clear, ansi.FgBrightCyan, ansi.Reset)
	for {
		fmt.Fprintf(s, "\n%sName your Realm:%s ", ansi.FgBrightWhite, ansi.Reset)
		name, err := session.ReadLine(s)
		if err != nil {
			return handle // stream ended; fall back to the handle
		}
		name = strings.TrimSpace(name)
		if alnum(name) < 3 || taken[strings.ToLower(name)] {
			fmt.Fprintf(s, "%s  Invalid: at least 3 letters/numbers, not matching another realm.%s\n", ansi.FgRed, ansi.Reset)
			continue
		}
		return name
	}
}

func showEvents(s session.Session, e *game.Empire) {
	if len(e.Events) == 0 {
		return
	}
	fmt.Fprintf(s, "%s%sWhile you were away:%s\n", ansi.Clear, ansi.FgBrightCyan, ansi.Reset)
	for _, ev := range e.Events {
		fmt.Fprintf(s, "  %s\n", ev)
	}
	e.Events = nil
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
