package menu

import (
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// gameflow.go — the session and the turn: the loop a caller sits in, and the
// ordered sequence of stages one turn runs through. The stages themselves are
// in turnstages.go and the recap screens in turnrecap.go.

// GameLoop is the top-level session flow: show the Game Menu until the
// player quits. "Play Game" runs a turn pipeline. handle identifies the empire
// playing THIS session — per-session state, not shared World state, so the web
// front-end can run concurrent sessions against one World, and re-resolved each
// transaction so it survives the door's per-action world reload.
func GameLoop(s session.Session, w *game.World, handle string, t Term) (err error) {
	// A prompt read failing mid-turn (idle boot or dropped connection) unwinds
	// the whole session via session.End; catch it here and report it as io.EOF,
	// which the caller (play.Session) treats as a clean save-and-exit end.
	defer session.GuardEnd(&err)
	c := &ctx{World: w, handle: handle, Term: t}
	// Seed the active-empire cache under the raw world lock so the first resolve
	// can't race a concurrent AddHuman (web onboarding). Later resolves are
	// cache hits until a reload bumps the generation.
	w.Lock()
	c.Player()
	w.Unlock()
	menus := BuildMenus()
	// Expand Ctrl-<letter> into the active player's saved macro keystrokes.
	// This single wrap covers every front-end, since all of them reach the
	// menu through GameLoop.
	ms := session.NewMacroExpander(s, func(letter string) (string, bool) {
		p := c.Player()
		if p == nil {
			return "", false
		}
		seq, ok := p.Macros[letter]
		return seq, ok
	})
	return Run(ms, c, menus.Game)
}

// collectTurnIncome runs the turn-start block — industry production, income
// collection, and the per-turn region-cap reset — guarded so a turn REPLAYED
// after an idle-boot does not collect income (or reset the region cap) twice
// (#10). The mutation and the IncomeCollected flag commit in one transaction, so
// a boot leaves them consistent. Returns false if the empire vanished mid-turn
// (eliminated by another node), like every withPlayer transaction here.
func collectTurnIncome(w *ctx) bool {
	return withPlayer(w, func(p *game.Empire) {
		if p.TurnProgress.IncomeCollected {
			return
		}
		w.World.Manufacture(p)   // industry production at turn start, alongside income (#71)
		w.World.CollectIncome(p) // credit this turn's income up front, so maintenance and spending draw from it
		w.World.GrowFood(p)      // credit this turn's food at turn start too, so it can be sold this turn (matches BRE)
		p.RegionsBoughtThisTurn = 0
		p.TurnProgress.IncomeCollected = true
	})
}

// showTurnIntro prints the per-turn income report and the empire status pages.
// The income report covers THIS turn's production/income, so on a replay after an
// idle-boot (income already collected before the boot) it is skipped. The empire
// status is shown unconditionally — including on re-entry — so a player dropped
// back mid-turn sees their current state (gold, food, military) before acting,
// instead of landing cold at a submenu (#10).
func showTurnIntro(s session.Session, w *ctx, replaying bool) {
	if !replaying {
		incomeReport(s, w)
	}
	renderEmpireStatus(s, w)
}

// runStageOnce runs one turn stage unless it already completed this turn, in
// which case it is skipped — so a turn REPLAYED after an idle-boot does not walk
// the player back through a menu they already exited (#10). fn runs the stage
// (typically a nested Run); on a clean return the stage is marked done. If fn
// returns an error (a bare io.EOF stream end) the stage is left unmarked, and a
// session boot panics out of fn (via session.End) before the mark is written —
// both leave the stage to re-run on replay, resuming where the boot hit.
func runStageOnce(w *ctx, get func(game.TurnProgress) bool, set func(*game.TurnProgress), fn func() error) error {
	done := false
	withPlayer(w, func(p *game.Empire) { done = get(p.TurnProgress) })
	if done {
		return nil
	}
	if err := fn(); err != nil {
		return err
	}
	withPlayer(w, func(p *game.Empire) { set(&p.TurnProgress) })
	return nil
}

// withPlayer runs fn under w's lock against the FRESHLY-resolved active empire.
// It returns false when the empire has vanished (eliminated/abdicated by another
// node mid-turn), so a turn-pipeline caller can abort instead of mutating a
// stale *Empire that a reload may have rebound to a rival's data. Every mutating
// transaction in the turn pipeline goes through here rather than closing over a
// p captured across a w.With boundary.
func withPlayer(w *ctx, fn func(p *game.Empire)) bool {
	alive := false
	w.With(func() {
		if p := w.Player(); p != nil {
			fn(p)
			alive = true
		}
	})
	return alive
}

// runTurn is the "Play Game" action. It shows the event log, then walks the
// per-turn pipeline (industry production, income report, status,
// spending/attack/covert/trading/message stages, then end-of-turn) for as
// many turns as the player wants to play (#70). Diplomacy and Change Production
// are no longer pre-turn stops here — they stay reachable from the System menu.
func runTurn(s session.Session, w *ctx) Result {
	menus := BuildMenus()
	// abort ends the turn cleanly when the active empire has been eliminated by
	// another node mid-turn (withPlayer returned false). The active *Empire is
	// re-resolved inside every transaction below, never captured across a w.With,
	// so a reload that reshapes the empire set can't rebind it to a rival's data.
	abort := func() Result {
		ok(s, "Your realm is no longer active.")
		return Stay
	}
	var turnsLeft int
	if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
		return abort()
	}
	if turnsLeft <= 0 {
		// Out of turns still gets the recap and the mailbox, and skips the offer
		// prompts. BRE gates process_trade_offer and process_diplomatic_proposal
		// on turns remaining (BRE.EXE 0x3842) but runs write_data_report and
		// read_local_messages either way (0x385F), reaching "Sorry, you have used
		// all of your turns today." only afterwards (0x38D7 -> 0x3F8D).
		showTurnEvents(s, w)
		readTurnMail(s, w, true)
		ok(s, "Sorry, you have used all of your turns today.")
		seeScores(s, w)
		return Stay
	}

	showCoordinatorNotice(s, w)
	openTurnRecap(s, w)
	annihilatorDefense(s, w)

	firstTurn := true
	for {
		if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
			return abort()
		}
		if turnsLeft <= 0 {
			ok(s, "Sorry, you have used all of your turns today.")
			seeScores(s, w)
			return Stay
		}

		// New mail may arrive between turns (from another node), so check each
		// turn, not just once in the pre-turn flow (#3).
		readTurnMail(s, w, firstTurn)
		if firstTurn {
			showQueenRefund(s, w)
			showLottery(s, w)
		}
		firstTurn = false

		// Capture whether this is a replay BEFORE collecting income (which sets the
		// flag), so the intro screens can be skipped on a booted-turn replay.
		replaying := false
		withPlayer(w, func(p *game.Empire) { replaying = p.TurnProgress.IncomeCollected })

		if !collectTurnIncome(w) {
			return abort()
		}

		showTurnIntro(s, w, replaying)
		summarised := paymentStage(s, w, menus.Bank)
		// Was the realm already fed before this pass? On a replay it may have been
		// (fed before the boot), in which case the feed stage and its "food consumed"
		// summary are both skipped — so the replay lands at the first unfinished
		// stage without re-showing a screen the player already saw (#10).
		fedBefore := false
		withPlayer(w, func(p *game.Empire) { fedBefore = p.TurnProgress.Fed })
		silentFeed := false
		if err := runStageOnce(w,
			func(tp game.TurnProgress) bool { return tp.Fed },
			func(tp *game.TurnProgress) { tp.Fed = true },
			func() error {
				var err error
				silentFeed, err = feedStage(s, w, menus.Food, summarised)
				return err
			}); err != nil {
			return Stay
		}
		// Only the silent path gets the summary and its pause. When the market
		// ran, the player has just answered both obligations by hand and BRE goes
		// straight on to the bank — the line would be telling them what they were
		// asked two prompts ago (cap/eots-ibbs-01.cap).
		if !fedBefore && silentFeed {
			var foodUpkeep int
			if !withPlayer(w, func(p *game.Empire) { foodUpkeep = p.FoodUpkeep() }) {
				return abort()
			}
			statLine(s, foodUpkeep, "units of Food consumed.")
			pause(s)
		}

		// Covert Operations runs right after maintenance and before Spending, per
		// BRE's turn order (Payment/Food Market -> Covert -> Spending). Shown only
		// when the player keeps the step on (Preferences) AND holds at least one
		// covert agent to act with — a fresh realm starts with none.
		if w.prefs().VisitCovert {
			var agents int
			if !withPlayer(w, func(p *game.Empire) { agents = p.Agents }) {
				return abort()
			}
			if agents >= 1 {
				if err := runStageOnce(w,
					func(tp game.TurnProgress) bool { return tp.CovertDone },
					func(tp *game.TurnProgress) { tp.CovertDone = true },
					func() error { return Run(s, w, menus.Covert) }); err != nil {
					return Stay
				}
			}
		}

		if err := runStageOnce(w,
			func(tp game.TurnProgress) bool { return tp.SpendingDone },
			func(tp *game.TurnProgress) { tp.SpendingDone = true },
			func() error { return Run(s, w, menus.Spending) }); err != nil {
			return Stay
		}
		if err := runStageOnce(w,
			func(tp game.TurnProgress) bool { return tp.AttackDone },
			func(tp *game.TurnProgress) { tp.AttackDone = true },
			func() error { return Run(s, w, menus.Attack) }); err != nil {
			return Stay
		}
		if w.prefs().VisitTrading {
			if err := runStageOnce(w,
				func(tp game.TurnProgress) bool { return tp.TradingDone },
				func(tp *game.TurnProgress) { tp.TradingDone = true },
				func() error { return Run(s, w, menus.Trading) }); err != nil {
				return Stay
			}
		}
		// InterPlanetary Ops is an InterBBS-only step, shown after Trading.
		if w.Config.InterBBSEnabled() {
			if err := runStageOnce(w,
				func(tp game.TurnProgress) bool { return tp.InterPlanetaryDone },
				func(tp *game.TurnProgress) { tp.InterPlanetaryDone = true },
				func() error { return Run(s, w, menus.InterPlanetary) }); err != nil {
				return Stay
			}
		}
		if w.prefs().VisitMessage {
			_ = runStageOnce(w,
				func(tp game.TurnProgress) bool { return tp.MessageDone },
				func(tp *game.TurnProgress) { tp.MessageDone = true },
				func() error {
					if AskYesNo(s, "Send a message?", false) {
						sendMessage(s, w)
					}
					return nil
				})
		}

		// "Deposit gold at End of Turn" is banked inside PlayTurn now, just ahead of
		// the interest, so the deposit earns on the turn that made it (#216).
		if !withPlayer(w, func(p *game.Empire) {
			w.World.PlayTurn(p, w.Today)
		}) {
			return abort()
		}

		endOfTurnStats(s, w)

		if !withPlayer(w, func(p *game.Empire) { turnsLeft = p.TurnsLeft }) {
			return abort()
		}
		// Running out mid-play is told, not just acted on: the loop used to fold
		// this into the prompt's own condition, so the last turn ended by
		// short-circuiting past both the prompt and any message and dropped the
		// player on the opening menu with no word about why. Same line someone
		// sees who starts Play with nothing left.
		if turnsLeft <= 0 {
			ok(s, "Sorry, you have used all of your turns today.")
			return Stay
		}
		if !AskYesNo(s, "Continue to your next turn?", true) {
			return Stay
		}
	}
}
