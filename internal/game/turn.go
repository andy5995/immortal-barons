package game

import (
	"errors"
	"math"
	"time"
)

// turn.go — the turn and the day: what committing a turn does, what daily
// maintenance runs through, and the economy step that charges a realm for what
// it holds.

// PlayTurn processes one turn for a single empire: its economy runs, and
// its turn/protection counters tick down. Idle empires (whose owner never
// plays) never call this, so they stagnate. Industry production
// (manufacture) is a turn-start step, run alongside CollectIncome by the
// caller, not here.
func (w *World) PlayTurn(e *Empire, today string) {
	// A played turn awards a flat 213 (binary-verified; see ScorePerTurn). Nothing
	// in the economy touches Score — no riot or spoilage ding.
	e.Score += ScorePerTurn
	e.TurnsPlayed++      // lifetime count; the HeadQuarters price rises with it
	w.maybePirateRaid(e) // ~1-in-5-turns pirate raid; notice surfaces next turn's income (#21)
	w.advanceTech(e)     // Technology bonus builds up a little each turn (not instant)
	w.processEconomy(e)
	// Maintenance shortfalls charged during this turn land now, at rollover, so
	// the drop shows on the next turn's display — BRE's own ordering.
	if e.PendingSupportPenalty > 0 {
		e.adjustSupport(-e.PendingSupportPenalty)
		e.PendingSupportPenalty = 0
	}
	if e.Score < 0 {
		e.Score = 0
	}
	if e.HQ > 0 && e.HQ < 100 {
		e.HQ += HQBuildPerTurn
		if e.HQ > 100 {
			e.HQ = 100
		}
	}
	// Advance this empire's price walk for next turn (#30). Done after the turn's
	// buys so the price stays stable within the turn (shown == charged), and keyed
	// on the still-current TurnsLeft so each turn's step is distinct.
	w.stepPrices(e)
	if e.TurnsLeft > 0 {
		e.TurnsLeft--
	}
	if e.Protection > 0 {
		e.Protection--
	}
	// Clear the per-turn region-buy budget so the next turn starts fresh. The human
	// menu flow already zeroes this at turn start (gameflow), but aiPlay drives its
	// turns straight through PlayTurn — without this the AI's cap accumulates across
	// the day and strands it at MaxRegions total instead of MaxRegions per turn.
	e.RegionsBoughtThisTurn = 0
	e.TurnProgress = TurnProgress{} // turn committed: the next turn starts with a clean slate (#10)
	e.LastPlayed = today
}

// MaintReport summarizes what a DailyMaintenance call did, so the login flow and
// the sysop -maint command can tell the player/operator whether maintenance ran.
// Days is how many game days it advanced (0 when already current for today).
// NotStarted is set when the configured Game Start Date has not arrived yet.
type MaintReport struct {
	Days       int
	NotStarted bool
	// Steps names each stage the run actually performed, in the order it ran
	// them, for the front-end to print one per line — the way the original
	// reports its own daily maintenance. A stage that had nothing to do adds no
	// line, so the list is a record of what happened rather than a fixed menu.
	// English, as every engine-authored string is; the front-end translates.
	Steps []string
}

// step records a stage of daily maintenance for the caller's report.
func (r *MaintReport) step(name string) { r.Steps = append(r.Steps, name) }

// DailyMaintenance advances the world by AT MOST ONE game day per real day,
// however long the game has sat idle. A realm nobody has touched for four days
// comes back to one day of change, not four: skipped days are lost, not banked.
// Catching up all of them at once meant a returning player met four days of
// pirate raids, riots, AI turns and price moves in a single login, which is both
// unreadable and unfair to whoever was actually playing.
//
// It is idempotent within a real day (LastMaintRun), so several callers logging
// in on the same day run it once between them. The first call on a brand-new
// world just records the date. Returns a MaintReport summarizing what it did.
func (w *World) DailyMaintenance(today string) MaintReport {
	// Before the configured Game Start Date, players may sign up but the game
	// does not advance. Pin the clock to today so it doesn't catch up when the
	// start date arrives.
	if !w.Config.GameStarted(today) {
		w.LastMaintDate = today
		return MaintReport{NotStarted: true}
	}
	// The first maintenance that runs with the game underway is the day it began.
	if w.StartedDate == "" {
		w.StartedDate = today
	}
	if w.LastMaintDate == "" {
		// First maintenance of a fresh game: play the AI barons' opening turns (they
		// were seeded with a full day's TurnsLeft) so the first human to log in meets
		// developed rivals, not static starting-state realms. Without this the AIs
		// sit idle until the next calendar day's maintenance.
		w.aiPlay(today)
		w.LastMaintDate = today
		return MaintReport{}
	}
	// One game day per real day. LastMaintRun is the real date this last ran;
	// LastMaintDate is the game clock, which falls behind while nobody plays.
	if w.LastMaintRun == today || w.LastMaintDate >= today {
		w.removeDeadHusks()
		return MaintReport{}
	}
	rep := MaintReport{Days: 1}
	{
		rep.step("Paying out trading market sales")
		w.settleMarketProceeds()                   // "Depositing trading market money" — pay sellers at day-end (#17)
		w.FoodMarketSupply = FoodMarketDailySupply // refill the food market for the new day (#19)
		rep.step("Restocking the food market")
		for _, e := range w.Empires {
			if e.Alive {
				e.LandAvailable += w.Config.LandPerDay // Daily Land Creation
				e.TurnsLeft = w.Config.TurnsPerDay
				// Fresh day: every per-day attack allotment resets.
				e.AttacksToday = 0
				e.GroupAttacksToday = 0
				e.TerrorOpsToday = 0
				e.BombingOpsToday = 0
				e.RefundTaken = false
				e.LotteryTaken = false
				e.TurnProgress = TurnProgress{} // abandon any turn left uncommitted at rollover (#10)
			}
		}
		rep.step("Granting new land and refreshing turns")
		// Every covert agent sent out yesterday lands now, before anyone plays —
		// BRE's run_daily_maintenance drains the queue the same way. Ahead
		// of aiPlay so an AI's own operations wait a day exactly as a player's
		// do, rather than resolving inside the run that queued them.
		if len(w.CovertQueue) > 0 {
			rep.step("Resolving covert operations")
		}
		w.resolveCovertQueue()
		if len(w.Empires) > w.HumanCount() {
			rep.step("Playing the computer barons' turns")
		}
		w.aiPlay(w.LastMaintDate)
		// Pirate raids are per-turn now (maybePirateRaid in PlayTurn), not a daily
		// sweep — so they land randomly across turns (~1-in-5) instead of clustering
		// on the day's first turn (#21).
		for _, e := range w.Empires {
			if e.Alive {
				maybeRandomEvent(w, e)
			}
		}
		for _, e := range w.Empires {
			if e.Owner == "" {
				e.Events = nil
			}
		}
		for _, e := range w.Empires {
			if e.Alive && (e.Land <= 0 || e.People <= 0) {
				w.Kill(e)
			}
		}
		rep.step("Checking for fallen realms")
		// The planet's own weapon goes off to war by itself once construction has
		// run (#114), and any weapon squatting here takes its daily bite (#112).
		if w.Annihilator != nil || w.Incoming != nil {
			rep.step("Resolving Clingy Annihilator operations")
		}
		w.LaunchDueAnnihilator()
		w.TickAnnihilator()
		w.RetireSpentAnnihilator()
		w.GameDay++
		// Sweep out husks whose death is now in the past so dead barons (AI
		// included) don't linger on the scoreboard or in the world. A realm
		// that died today (DiedDay == GameDay before the increment above) is
		// removed on this pass; the owner rebuilds on a later login.
		w.removeDeadHusks()
		// A caller who created a realm and never played a turn is erased; they may
		// start fresh whenever they next log in.
		before := len(w.Empires)
		w.removeUnplayedEmpires()
		w.removeIdleEmpires(today) // abandoned realms fade out (#83)
		if len(w.Empires) < before {
			rep.step("Clearing away abandoned realms")
		}
		for _, e := range w.Empires {
			if e.Alive {
				// Kept on the empire so every turn of the day reports the same
				// figure, which is how BRE shows it (cap/eots-ibbs-01.cap: the
				// one day's 14,699,020 repeats on all ten turns).
				e.InvestReturnsToday = w.matureInvestments(e)
				w.matureLoans(e)
			}
		}
		rep.step("Settling investments and loans")
		w.expireSpyGuys() // a watcher's stay is a day shorter, silently (SpyGuy)
		w.adjustInvestRate()
		rep.step("Setting the bank's rate")
		w.postMasterNews()
		w.postProclamationNews()
		w.rollNews()
		rep.step("Writing the daily news bulletin")
		if w.Config.GameLength > 0 && w.GameDay >= w.Config.GameLength {
			rep.step("Crowning the Planetary Master")
			w.endGame()
		}
		w.LastMaintRun = today
		if next := w.nextDate(w.LastMaintDate); next != w.LastMaintDate {
			w.LastMaintDate = next
		} else {
			w.LastMaintDate = today // malformed date; snap to today rather than stall
		}
	}
	// Sweep stale husks even when no day rolled over (e.g. a same-day -maint on
	// an already-current world), so past-day dead realms don't linger. A realm
	// that died today (DiedDay == GameDay) is kept by removeDeadHusks.
	w.removeDeadHusks()
	return rep
}

// rollNews snapshots the day's planet totals into BulletinToday (rolling the
// previous snapshot into BulletinYesterday, with Change the per-field delta)
// and rolls NewsToday into NewsYesterday. News/event generation for the day
// happens elsewhere during maintenance, before this runs.
func (w *World) rollNews() {
	newTotals := planetTotals(w)
	prev := w.BulletinToday
	w.BulletinYesterday = w.BulletinToday
	w.BulletinToday = DailyBulletin{
		Totals: newTotals,
		Change: PlanetTotals{
			Population: newTotals.Population - prev.Totals.Population,
			Regions:    newTotals.Regions - prev.Totals.Regions,
			NetWorth:   newTotals.NetWorth - prev.Totals.NetWorth,
		},
	}
	w.NewsYesterday = w.NewsToday
	w.NewsToday = nil
}

func (w *World) nextDate(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, 1).Format("2006-01-02")
}

// DateForDay renders the calendar date of GameDay `day` as MM/DD/YYYY, the way
// BRE shows investment-maturity and loan-due dates. It is computed from Today and
// the current GameDay, so day == GameDay is today and future days project forward.
func (w *World) DateForDay(day int) string {
	t, err := time.Parse("2006-01-02", w.Today)
	if err != nil {
		return ""
	}
	return t.AddDate(0, 0, day-w.GameDay).Format("01/02/2006")
}

// popCapacity is the carrying capacity migration pulls the population toward.
// BINARY-VERIFIED against BRE's end-of-turn routine (BRE.OVR 0xD08A-0xD213):
//
//	capacity = Σ(regions × PopCapWeight) × support/90 × 10/max(3, tax) + 50
//
// converted from BRE's population unit to IB's by PopBREUnitScale.
//
// Three factors move it, which is what makes migration a lever rather than a
// clock: WHAT you own (urban housing dwarfs everything else), how POPULAR you
// are, and how hard you TAX — the last two multiplicatively, so a heavily taxed
// realm with poor support has a fraction of the capacity its land could hold.
// Selling urban land or losing support lowers the capacity and people then
// drain away toward it, rather than via a separate housing-loss hit.
func (e *Empire) popCapacity() int {
	cap := e.Regions.Coastal*PopCapCoastal +
		e.Regions.River*PopCapRiver +
		e.Regions.Agricultural*PopCapAgricultural +
		e.Regions.Desert*PopCapDesert +
		e.Regions.Industrial*PopCapIndustrial +
		e.Regions.Urban*PopCapUrban +
		e.Regions.Mountain*PopCapMountain +
		e.Regions.Technology*PopCapTechnology +
		e.Regions.Waste*PopCapWaste // zero, and deliberately spelled out
	// The weights are BRE's, one unit to a million; IB counts twenty people to
	// that unit. Convert before the divisions rather than after, so the capacity
	// keeps its granularity instead of rounding in steps of twenty.
	cap *= PopBREUnitScale
	cap = cap * e.Support / PopCapSupportDivisor
	tax := e.Tax
	if tax < PopCapTaxFloor {
		tax = PopCapTaxFloor
	}
	return cap*PopCapTaxNumerator/tax + PopCapBase*PopBREUnitScale
}

// ErrAlreadySpecialized is returned when a realm's industry is already
// committed. The choice is permanent, so a second attempt keeps the first.
var ErrAlreadySpecialized = errors.New("Your industry is already specialized.")

// Specialize commits e's industry to one military good, permanently.
func (w *World) Specialize(e *Empire, g *Good) error {
	if e.Specialized != "" {
		return ErrAlreadySpecialized
	}
	e.Specialized = g.Plural
	return nil
}

func (w *World) processEconomy(e *Empire) {
	// Savings interest (BRE-faithful, config-help verified): the Interest Rate knob
	// is "the interest the bank gives in 10 days", so config/10 is the DAILY rate
	// (shown in View Bank Rates: config 50 → 5.0%/day). BRE credits it "at the end
	// of each turn", so per turn it is the daily rate spread across the day's turns:
	// interest = Bank × (InterestRate/10)/100 / TurnsPerDay = Bank × InterestRate /
	// (1000 × TurnsPerDay). int64 throughout and clamp before storing: on a 32-bit
	// build the Bank*InterestRate product overflows int32 before the divide.
	// Storage stays int.
	//
	// The whole balance earns. IB used to stop paying interest above 1.6 billion,
	// a figure that came from a player guide and turned out to be in neither
	// binary; the money cap is the only ceiling the bank has.
	tpd := int64(w.Config.TurnsPerDay)
	if tpd < 1 {
		tpd = 1
	}
	// "Deposit gold at End of Turn" is banked HERE, ahead of the interest, so the
	// turn's takings earn on the turn they were made rather than sitting idle
	// until the next one. It ran in the menu flow after PlayTurn until 2026-08-25,
	// which meant the deposit always missed that turn's interest. It stays behind
	// the pirate raid at the top of PlayTurn — banking early must not become a way
	// to keep gold out of the raiders' reach. Only a caller's realm has menu
	// preferences; the AI manages its own treasury.
	if e.Owner != "" && e.Prefs.DepositEndTurn && e.Gold > 0 {
		_ = w.Deposit(e, e.Gold)
	}
	interest := e.Bank * int64(w.Config.InterestRate) / (1000 * tpd)
	e.Bank += interest
	// Reported at the start of the next turn (#216), so it has to survive the
	// save between the two door runs.
	e.LastInterest = interest
	// A bank sitting at the cap pays its interest into the treasury rather than
	// having it destroyed: the cap limits what one purse holds, and a full purse
	// is no reason to burn the earnings. Gold has the same cap, so a baron whose
	// hand is full too still loses the overflow — the clamp at the end of this
	// function. Whether the original does this is unverified; IB chooses it
	// because the alternative silently deletes money the player earned.
	if over := e.Bank - w.MoneyCap(); over > 0 {
		e.Bank = w.MoneyCap()
		w.creditGold(e, over, "bank interest")
	}
	if e.Debt > 0 {
		e.Debt += e.Debt * DebtGrowthPct / 100
	}

	// Food growth was already credited at turn start (GrowFood); here we only
	// consume and then spoil. (Was `Food += FoodGrown - consumed`, which grew the
	// food at turn end where it couldn't be sold and always spoiled.)
	// Feeding the realm, and what going short costs (see morale.go): popular
	// support for the people's shortfall, military morale for the army's, and a
	// civil war when the people got under two thirds of their need. Both penalties
	// are filed and applied at rollover, as BRE files them.
	w.feed(e)

	// Food spoilage (BRE-verified by driving the original, 2026-07-16): FoodSpoilPct
	// (5%) of the ENTIRE stored food spoils each turn — floor(0.05 × food).
	// Technology decreases it (via tf). Food escrowed on the Trading Market counts
	// toward the total, so listing food doesn't dodge spoilage — only attacks
	// (#17); the same sum is what BRE's decay block tests against FoodSpoilFloor,
	// below which nothing rots at all. Spoilage comes out of the food in hand first,
	// then the listing.
	listedFood := w.MarketForSale(e.Name, "Food")
	if total := e.Food + listedFood; total > FoodSpoilFloor {
		spoiled := techLower(total*FoodSpoilPct/100, e.TechDecayFactor())
		e.LastSpoiled = spoiled
		fromStock := spoiled
		if fromStock > e.Food {
			fromStock = e.Food
		}
		e.Food -= fromStock
		w.spoilListedFood(e.Name, spoiled-fromStock)
	} else {
		e.LastSpoiled = 0
	}

	// Migration: the population moves a slice of the way toward its carrying
	// capacity each turn. BINARY-VERIFIED shape (BRE.OVR 0xD219-0xD3CC); see
	// popCapacity for the capacity itself and docs/mechanics-reference.md for
	// the whole model.
	e.LastPopGrowth = 0
	capacity := e.popCapacity()
	// A percentage of the HEADROOM, not of the population, so a realm far below
	// capacity fills fast and one near it barely moves.
	growth := (capacity - e.People) * (w.rng.Intn(PopMoveJitter) + PopMoveMinPct) / 100
	if growth < 0 {
		// Leaving is faster than arriving, and taxes drive it: a shrinking realm
		// loses people in proportion to the root of its tax rate.
		growth = -int(math.Sqrt(float64(e.Tax)) * float64(-growth) / PopDeclineTaxDivisor)
	}
	// Churn, so an empire sitting exactly at capacity still moves a little.
	growth += w.rng.Intn(max(1, capacity/PopChurnUpDivisor)) - w.rng.Intn(max(1, capacity/PopChurnDownDivisor))
	if e.Tax > PopPunitiveTaxRate {
		// Above a punitive rate people leave on top of everything else, whether
		// the realm was growing or shrinking.
		cut := growth * PopPunitiveTaxPct / 100
		if cut < 0 {
			cut = -cut
		}
		growth -= cut
	}
	if ceiling := e.People / PopGrowthCeilingDivisor; growth > ceiling {
		growth = ceiling // no realm more than half again as big in one turn
	}
	if e.People+growth < 0 {
		growth = -e.People
	}
	if growth != 0 {
		e.People += growth
		e.LastPopGrowth = growth
	}

	// Taxes, riots and popular support, verified against a BRE.OVR disassembly
	// (end-of-turn routine at 0xCE97) and reproduced exactly by a live capture.
	// A riot fires iff tax > RiotTaxFloor AND tax² >= Random(10000) — quadratic,
	// so 1.4% a turn at 12% tax but 15% at 39% — and costs both people and
	// support. On top of any riot, support always drifts by -(tax-30)/10, which
	// is a free gain below SupportTaxNeutral and a bleed above it.
	//
	// The civil-unrest step runs first, as it does in the original: the pending
	// morale penalty lands, then the army deserts at a rate drawn from the morale
	// it lands on, then a famine's civil war (if any) is spent.
	if e.PendingMoralePenalty != 0 {
		e.adjustMorale(-e.PendingMoralePenalty)
		e.PendingMoralePenalty = 0
	}
	w.moraleDesertion(e)
	w.resolveCivilWar(e)

	e.LastRiot = false
	riotPenalty := 0
	if e.Tax > RiotTaxFloor && e.Tax*e.Tax >= w.rng.Intn(RiotChanceDenom) {
		e.LastRiot = true
		w.postRiotNews(e)
		riotPenalty = e.Tax / RiotSupportDivisor
		e.People -= e.People / RiotPeopleDivisor
	} else if e.Support < LowSupportNewsCeil && w.rng.Intn(LowSupportNewsOdds) == 0 {
		// An unpopular realm makes the planet news even in a turn with no tax
		// riot, and this one costs nothing beyond the embarrassment.
		w.postRiotNews(e)
	}
	e.adjustSupport(-riotPenalty - (e.Tax-SupportTaxNeutral)/SupportTaxDivisor)
	if e.Support < MoraleDrainSupport {
		e.adjustMorale(-(MoraleDrainSupport - e.Support))
	}
	// Taxing very lightly buys back support, but only while the realm is unhappy.
	if e.Tax < LowTaxBonusBelow && e.Support < LowTaxSupportCeil {
		e.adjustSupport(LowTaxBonusBelow - e.Tax)
	}

	if e.Gold > w.MoneyCap() {
		e.Gold = w.MoneyCap()
	}
	if e.Bank > w.MoneyCap() {
		e.Bank = w.MoneyCap()
	}
}
