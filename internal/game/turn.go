package game

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"math"
	"time"
)

// PlayTurn processes one turn for a single empire: its economy runs, and
// its turn/protection counters tick down. Idle empires (whose owner never
// plays) never call this, so they stagnate. Industry production
// (manufacture) is a turn-start step, run alongside CollectIncome by the
// caller, not here.
func (w *World) PlayTurn(e *Empire, today string) {
	// BRE score: each turn played awards a flat amount. processEconomy may then
	// subtract small riot/spoilage penalties.
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
}

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
	{
		w.settleMarketProceeds()                   // "Depositing trading market money" — pay sellers at day-end (#17)
		w.FoodMarketSupply = FoodMarketDailySupply // refill the food market for the new day (#19)
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
				e.TurnProgress = TurnProgress{} // abandon any turn left uncommitted at rollover (#10)
			}
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
				e.Alive = false
				e.DiedDay = w.GameDay
			}
		}
		w.GameDay++
		// Sweep out husks whose death is now in the past so dead barons (AI
		// included) don't linger on the scoreboard or in the world. A realm
		// that died today (DiedDay == GameDay before the increment above) is
		// removed on this pass; the owner rebuilds on a later login.
		w.removeDeadHusks()
		// A caller who created a realm and never played a turn is erased; they may
		// start fresh whenever they next log in.
		w.removeUnplayedEmpires()
		w.removeIdleEmpires(today) // abandoned realms fade out (#83)
		for _, e := range w.Empires {
			if e.Alive {
				w.matureInvestments(e)
				w.matureLoans(e)
			}
		}
		w.adjustInvestRate()
		w.postMasterNews()
		w.postProclamationNews()
		w.rollNews()
		if w.Config.GameLength > 0 && w.GameDay >= w.Config.GameLength {
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
	return MaintReport{Days: 1}
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

// aiPlay runs each AI empire's turns for one day.
func (w *World) aiPlay(today string) {
	for _, e := range w.AIEmpires() {
		w.aiHandleDiplomacy(e) // answer pending treaty offers before playing (#36)
		// e.Alive is re-checked because an earlier aggressor in this pass may have
		// conquered this realm before its own turn came up (#36).
		for e.Alive && e.TurnsLeft > 0 {
			// Mirror the human turn: produce and collect income, pay maintenance from
			// that income, THEN spend what's left. Spending before paying maintenance
			// (the old order) let the AI blow its treasury on expansion and new
			// military it then couldn't maintain, so its forces deserted and its
			// regions revolted every turn — a self-inflicted boom-bust.
			w.aiSetProduction(e) // point industry at units this profile uses, not BRE's even split
			w.Manufacture(e)     // industry production at turn start (#71)
			w.CollectIncome(e)   // income in hand before anything is spent
			w.GrowFood(e)        // food credited at turn start too, so aiManageEconomy sees it (matches the human flow)
			e.LastGoldPaid = 0
			// Borrow BEFORE paying: a loan taken after the shortfall has already
			// cost desertion and revolts is worth nothing.
			w.aiManageDebt(e) // cover a shortfall, or repay from surplus (#69)
			w.PayForces(e, w.ForcesDue(e))
			w.PayRegions(e, w.RegionsDue(e))
			w.aiDecontaminate(e)    // clean up what a strike ruined, in the maintenance slot
			w.aiSetTax(e)           // reactive tax policy (#73)
			w.aiRebalanceRegions(e) // sell surplus land to fund farmland when starving (#69)
			// The market steps sit HERE, not inside aiManageEconomy: that function
			// returns early on its food-emergency branch, which fires on most turns,
			// so anything placed after it effectively never runs. Shop the market
			// before paying shop prices, and list the surplus once the turn's buying
			// has settled so the surplus is the real one.
			w.aiShopMarket(e)    // prefer a listing that undercuts the shop (#69)
			w.aiManageEconomy(e) // discretionary spending: food, military, land
			w.aiListSurplus(e)   // offer what it cannot use instead of paying upkeep on it (#69)
			w.aiProposeTreaty(e) // open diplomacy, not just answer it (#73)
			w.aiCovertOps(e)     // spy/agitate/shield, not just one pre-war demoralize (#57)
			w.aiWageWar(e)       // strike a weak neighbour when clearly favored (#36, #71)
			w.PlayTurn(e, today)
		}
	}
}

// aiManageEconomy spends an AI empire's gold for the turn like a human would:
// it keeps a few turns of food in reserve, and when its food production can't
// cover its army it grows Agriculture rather than buying more troops it can't
// feed. Only when the realm is food-healthy does it convert spare gold into
// troopers. This keeps AI realms from starving themselves to death.
func (w *World) aiManageEconomy(e *Empire) {
	produced := w.FoodGrown(e)
	upkeep := e.FoodUpkeep()

	// 1. Keep a food buffer, spending at most half the treasury on it so expansion
	//    gold survives. AIFoodBufferTurns is sized to ride out a day of consumption
	//    plus the 5% per-turn spoilage; too small a buffer let the correct BRE
	//    spoilage drain the granary mid-day and starve the realm's people.
	if target := upkeep * AIFoodBufferTurns; e.Food < target {
		if price := w.FoodBuyPrice(); price > 0 {
			buy := target - e.Food
			if afford := unitsAffordable(e.Gold/2, price); buy > afford {
				buy = afford
			}
			if buy > 0 {
				e.Food += buy
				e.Gold -= goldCost(buy, price)
			}
		}
	}

	// 2. If food production can't cover this turn's consumption, expand Agriculture
	//    to close the whole gap before buying anything else — buying enough regions
	//    (up to AIAgriBuyMax) to actually cover the shortfall, not a token few. A
	//    fast-growing realm's consumption climbs several regions' worth of food per
	//    turn, so the old 5-region trickle never caught up and the population
	//    outran its food into starvation; the food buffer in step 1 rides out the
	//    turn while this closes the gap.
	if produced < upkeep && e.Gold > int64(w.Prices.Land) {
		n := (upkeep-produced)/FoodAgriBase + 1 // conservative: the floor of the yield band
		if afford := unitsAffordable(e.Gold, w.Prices.Land); n > afford {
			n = afford
		}
		if n > AIAgriBuyMax {
			n = AIAgriBuyMax
		}
		if n > e.LandAvailable {
			n = e.LandAvailable // this realm's land allowance is finite
		}
		if n > 0 {
			e.Regions.Agricultural += n
			e.syncLand()
			e.Gold -= goldCost(n, w.Prices.Land)
			e.LandAvailable -= n
			return
		}
	}

	// 3. Food-healthy: expand land FIRST, every turn, protected or not (community
	//    strategy guides: plow every coin into money-making regions, compounding
	//    land -> income -> land). Expansion running before military is what keeps a
	//    realm growing after New Realm Protection lapses — funding a defensive force
	//    ahead of it skimmed half the treasury off the top and stalled the land
	//    engine right when protection ended. Once exposed, the force and an HQ come
	//    out of what land buying leaves; the remaining surplus is invested, not hoarded.
	w.aiExpandLand(e)
	if e.Protection == 0 {
		w.aiBuildForces(e)
		w.aiStartHQ(e)
	}
	w.aiInvestIdle(e)
}

// aiReserve is the gold an AI keeps back for food and maintenance before it
// will expand or invest: AIReserveTurns turns of its actual upkeep, floored so a
// tiny realm still holds something (#70). A flat figure did not track realm size
// — a grown realm earned it back in a fraction of a turn, so the same threshold
// gated both land buying and investing however large the economy became.
func (w *World) aiReserve(e *Empire) int64 {
	return max(AIGoldReserveMin, (w.ForcesDue(e)+w.RegionsDue(e))*AIReserveTurns)
}

// aiExpandLand plows the AI's surplus gold into land — the compounding land rush
// a strong human runs (community strategy guides: plow every coin into
// money-making regions). Which TYPE it buys is whichever its personality is
// furthest short of (aiNextRegionType); it used to buy Coastal and nothing else. It buys through the
// same BuyRegions path a human uses, so the per-turn region cap and the rising
// holdings-based price apply identically. Gold below the working reserve is left
// for food/maintenance; when the per-turn cap is hit the caller's aiInvestIdle
// parks the remainder instead of hoarding it.
func (w *World) aiExpandLand(e *Empire) {
	reserve := w.aiReserve(e)
	if e.Gold <= reserve {
		return
	}
	budget := e.Gold - reserve
	if e.aiSkill() == AISkillDull {
		budget = budget * AIDullLandBuyPct / 100 // dull barons hold back and grow slower
	}
	limit := min(w.regionBuyLimit(e), e.LandAvailable)
	n, total := 0, int64(0)
	for n < limit {
		cost := int64(w.regionCost(e.Land + n))
		if total+cost > budget {
			break
		}
		total += cost
		n++
	}
	if n > 0 {
		w.BuyRegions(e, e.aiNextRegionType(), n)
	}
}

// aiStartHQ builds a HeadQuarters once the AI fields tanks (#36): HQ multiplies
// tank offense and defense, so it is only worth the gold when there are tanks
// to amplify. StartHQ re-checks affordability, and HQ then advances on its own
// each turn (see PlayTurn), so this fires once and needs no further management.
func (w *World) aiStartHQ(e *Empire) {
	if e.HQ == 0 && e.Tanks > 0 && e.Gold > int64(w.HQPrice(e)) {
		w.StartHQ(e)
	}
}

// aiBuildForces spends a share of the AI's gold on military each turn, split by
// gold-value across the mix its personality calls for (#36, #71, #72). Shares
// come from balance.go.
//
// Carriers come first and are not a share: jets cannot reach a battle without
// them (JetsPerCarrier), so buying jets while owning no carriers is buying
// nothing. The AI covers the jets it already holds, then buys more jets with its
// share — so the lift always arrives before the aircraft.
//
// Agents stop at AIAgentsPerRegion per region. As a pure percentage they
// compounded into six-figure stockpiles on a large realm with nothing to spend
// them on (#57).
func (w *World) aiBuildForces(e *Empire) {
	budget := pctOf(e.Gold, AIMilitaryBudgetPct)
	buy := func(share, price int, count *int) {
		if price <= 0 {
			return
		}
		if n := unitsAffordable(pctOf(budget, share), price); n > 0 {
			*count += n
			e.Gold -= goldCost(n, price)
		}
	}
	mix := aiForceShares(e.aiProfile())
	// Under threat the realm stops building the army it wants and buys the one it
	// needs: a bigger slice of gold, spent turret-heavy.
	if w.aiUnderThreat(e) {
		budget = pctOf(e.Gold, AIThreatBudgetPct)
		mix = aiForceMix{AIForceTrooperPctPanic, AIForceTurretPctPanic, AIForceTankPctPanic, AIForceJetPctPanic, AIForceAgentPctPanic}
	}
	w.aiSellIdleCarriers(e)
	w.aiBuyCarriers(e)
	buy(mix.trooper, w.TrooperPrice(e), &e.Troopers)
	buy(mix.turret, w.TurretPrice(e), &e.Turrets)
	buy(mix.tank, w.TankPrice(e), &e.Tanks)
	buy(mix.jet, w.JetPrice(e), &e.Jets)
	if want := e.Land * AIAgentsPerRegion; e.Agents < want {
		buy(mix.agent, w.AgentPrice(e), &e.Agents)
	}
}

// aiBuyCarriers buys the lift the AI's jets need, one carrier per JetsPerCarrier
// jets, paid straight from gold rather than from a budget share — an uncarried
// jet contributes nothing, so this is the highest-value military gold the AI can
// spend. Capped at the shortfall so it never over-buys hulls.
func (w *World) aiBuyCarriers(e *Empire) {
	price := w.CarrierPrice(e)
	if price <= 0 {
		return
	}
	need := (e.Jets + JetsPerCarrier - 1) / JetsPerCarrier
	short := need - e.Carriers
	if short <= 0 {
		return
	}
	if afford := unitsAffordable(e.Gold, price); short > afford {
		short = afford
	}
	if short > 0 {
		e.Carriers += short
		e.Gold -= goldCost(short, price)
	}
}

// aiShopMarket buys from the general Trading Market before paying shop prices
// (#69). The AI ignored the market entirely, which had two bad effects: a human
// could list goods that no one would ever buy, and the AI passed up cheaper
// stock sitting in plain sight. It takes only listings that undercut the shop by
// AIMarketBuyDiscountPct, spends at most AIMarketBuyBudgetPct of its surplus, and
// buys through the same BuyFromMarket path a human uses — so escrow, the
// self-purchase refusal, and the seller's proceeds all behave identically.
func (w *World) aiShopMarket(e *Empire) {
	reserve := w.aiReserve(e)
	if e.Gold <= reserve {
		return
	}
	budget := (e.Gold - reserve) * AIMarketBuyBudgetPct / 100
	for _, good := range MarketGoods {
		shop := w.shopPrice(e, good)
		if shop <= 0 {
			continue
		}
		want := shop * (100 - AIMarketBuyDiscountPct) / 100
		for _, l := range w.MarketSellers(good, e.Owner) {
			if l.Price > want || l.Price <= 0 {
				continue
			}
			n := min(l.Qty, unitsAffordable(budget, l.Price))
			if n <= 0 {
				continue
			}
			if w.BuyFromMarket(e, l.Owner, good, n) == nil {
				budget -= goldCost(n, l.Price)
			}
		}
	}
}

// aiListSurplus puts goods the AI holds more of than it can use on the market,
// priced just under the shop so they actually sell (#69).
//
// Jets beyond carrier lift are the reliable case: a jet with no carrier cannot
// reach a battle, so it is pure upkeep, and a growing AI runs a few thousand of
// them. Listing them is strictly better than the shop's third-price sell-back
// AND gives the other barons something worth buying, which is what makes
// aiShopMarket more than decoration.
//
// Food only qualifies when production has outrun consumption, which the food
// buffer in aiManageEconomy usually prevents — it is handled anyway so a
// farming-heavy realm does not sit on a granary that only spoils.
func (w *World) aiListSurplus(e *Empire) {
	list := func(good string, surplus, shop int) {
		if surplus <= 0 || shop <= 0 {
			return
		}
		// Do NOT stack onto an existing listing: the surplus is recomputed every
		// turn, so adding to it each time grows the escrow without bound (a sim ran
		// one baron to 600k jets listed). One offer at a time, replaced only once
		// the last has cleared.
		if w.MarketForSale(e.Owner, good) > 0 {
			return
		}
		if qty := surplus * AIMarketListPct / 100; qty > 0 {
			w.SetMarketListing(e, good, qty, shop*(100-AIMarketUndercutPct)/100)
		}
	}
	list("Jet", e.Jets-e.Carriers*JetsPerCarrier, w.JetPrice(e))
	list("Food", e.Food-e.FoodUpkeep()*AIFoodBufferTurns, w.FoodBuyPrice())
}

// shopPrice is what e would pay the shop for one unit of a market good, or 0 for
// goods the shop does not sell at a per-unit price.
func (w *World) shopPrice(e *Empire, good string) int {
	switch good {
	case "Trooper":
		return w.TrooperPrice(e)
	case "Jet":
		return w.JetPrice(e)
	case "Turret":
		return w.TurretPrice(e)
	case "Bomber":
		return w.BomberPrice(e)
	case "Tank":
		return w.TankPrice(e)
	case "Carrier":
		return w.CarrierPrice(e)
	case "Agent":
		return w.AgentPrice(e)
	case "Food":
		return w.FoodBuyPrice()
	}
	return 0
}

// aiManageDebt borrows to cover a maintenance shortfall and repays out of a
// clear surplus (#69). The AI never touched Cash Relief, so a bad turn cost it
// desertion and revolts even when a short loan would have carried it. It borrows
// ONLY for the shortfall — funding expansion on credit compounds debt it cannot
// service — and only what the ceiling allows.
func (w *World) aiManageDebt(e *Empire) {
	due := w.ForcesDue(e) + w.RegionsDue(e)
	if short := due - e.Gold; short > 0 {
		want := short * AILoanHeadroomPct / 100
		if ceiling := w.LoanCeiling(e); want > ceiling {
			want = ceiling
		}
		if want > 0 {
			w.TakeLoan(e, want, AILoanDays)
		}
		return
	}
	// Solvent: put part of the clear surplus toward any defaulted debt.
	if e.Debt <= 0 {
		return
	}
	surplus := e.Gold - w.aiReserve(e)
	if surplus < AIMinSurplusToRepay {
		return
	}
	pay := min(surplus*AIDebtRepayPct/100, e.Debt)
	w.Repay(e, pay)
}

// aiDecontaminate spends the AI's spare gold on cleaning waste regions. Without
// it a struck AI realm carries the ruined land for the rest of the game, paying
// upkeep on ground that earns nothing — a nuclear strike would be permanent
// against an AI and merely inconvenient against a human. It buys what it can
// afford above its working reserve, so it never cleans itself into starvation.
func (w *World) aiDecontaminate(e *Empire) {
	if w.DecontaminateAllowance(e) <= 0 {
		return
	}
	spare := e.Gold - w.aiReserve(e)
	if spare <= 0 {
		return
	}
	w.Decontaminate(e, min(spare, w.DecontaminateCost(e)))
}

// aiRebalanceRegions sells regions the AI holds in surplus to fund the farmland
// it is short of (#69). It never sold regions at all, so a realm whose
// population outgrew its agriculture could starve while sitting on plenty of
// land it could have converted. Only fires when food production genuinely falls
// short AND it cannot pay for the farmland outright.
func (w *World) aiRebalanceRegions(e *Empire) {
	if w.FoodGrown(e) >= e.FoodUpkeep() {
		return
	}
	if e.Gold >= int64(w.LandPrice(e)) || e.LandAvailable <= 0 {
		return // it can already afford land, or there is none to buy
	}
	// Sell from whichever non-food type it holds most of, so the mix evens out
	// rather than gutting one specialty.
	biggest, field := 0, (*int)(nil)
	for _, c := range []struct {
		n int
		f *int
	}{
		{e.Regions.Coastal, &e.Regions.Coastal}, {e.Regions.Desert, &e.Regions.Desert},
		{e.Regions.Mountain, &e.Regions.Mountain}, {e.Regions.Urban, &e.Regions.Urban},
	} {
		if c.n > biggest {
			biggest, field = c.n, c.f
		}
	}
	if field == nil || biggest <= 0 {
		return
	}
	w.SellRegions(e, field, min(biggest/2, AIRebalanceSellMax))
}

// aiInvestIdle parks the AI's gold above a working reserve into investments so
// idle treasury earns rather than sitting (#36). Runs after food, expansion,
// and military spending, so only a genuine surplus is locked away.
func (w *World) aiInvestIdle(e *Empire) {
	reserve := w.aiReserve(e)
	if e.Gold <= reserve {
		return
	}
	if amt := pctOf(e.Gold-reserve, AIInvestPct); amt > 0 {
		w.Invest(e, amt, MinInvestDays)
	}
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

// IncomeBreakdown itemizes a turn's income by source (gold), plus the food
// grown. The income report and the actual gold credit both derive from this,
// so what the player is shown equals what is credited to the last coin. Urban
// and Technology regions produce no direct gold (BRE-verified), so they are
// not listed here.
type IncomeBreakdown struct {
	Taxes, Ore, Tourism, Solar, Rivers, Industrial, Trade int
	Food                                                  int // grown by Agricultural regions
	RiverFood                                             int // fished from rivers this turn (0 on a hydropower turn)
}

// Gold sums the gold-producing sources.
func (b IncomeBreakdown) Gold() int {
	return b.Taxes + b.Ore + b.Tourism + b.Solar + b.Rivers + b.Industrial + b.Trade
}

// CrownTaxBase is the income the crown tax is levied on: the six region/tax
// income lines, excluding Trade. BRE accumulates its base at exactly six sites,
// one per income line, and trading proceeds are not among them.
func (b IncomeBreakdown) CrownTaxBase() int {
	return b.Taxes + b.Ore + b.Tourism + b.Solar + b.Rivers + b.Industrial
}

// CrownTax is the Queen Royale's per-turn tax: a flat share of the turn's gross
// gold income, and a pure sink — no recipient treasury. Binary-verified against
// BRE (cost function at BRE.OVR 0x2EA09), where it reproduces 28 of 28 captured
// charges exactly:
//
//	crownTax = trunc(goldEarnedThisTurn * PlanetaryTaxRate / 100)
//
// The base is income alone — not gold on hand, bank balance, net worth, score,
// or units. BRE stores its rate in tenths of a percent (its default is 50,
// shown as 5.0%); IB stores whole percent instead, so a sysop and the config
// editor deal in one unit rather than two. Same arithmetic, one fewer trap.
func (w *World) CrownTax(e *Empire) int64 {
	return pctOf(int64(w.IncomeThisTurn(e).CrownTaxBase()), w.Config.PlanetaryTaxRate)
}

// regionDraw returns this turn's income draw for empire e's region type
// identified by salt: a uniform integer in [0, n). It is deterministic in
// (w.GameDay, e.Name, salt) — NOT a fresh RNG draw — so IncomeThisTurn stays
// pure and the income report always equals what CollectIncome credits. The
// variance is per game-day: a good/bad "year" lasts the whole day, and each
// region type draws independently.
//
// BRE draws the same way, straight over the Rate: its income sites call
// Random(Rate) and add the Base, so per-region income is Base + [0, Rate). IB
// used to draw a 0–100 percent and scale it, which could only land on multiples
// of Rate/100 — the same range and mean, but coarser. Live play shows the finer
// steps (one ore turn came out at Base + 279, which is not a multiple of 4).
func (w *World) regionDraw(e *Empire, salt, n int) int {
	if n <= 0 {
		return 0
	}
	h := fnv.New32a()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(salt))
	h.Write(buf[:])
	io.WriteString(h, e.Name)
	return int(h.Sum32() % uint32(n))
}

// industrialGold is the empire's TOTAL industrial gold this turn (not per
// region). The UNALLOCATED share of production — what is left after the six unit
// percentages — is paid out as gold, so allocating to units trades directly
// against it and a realm at 100% allocation earns none.
//
// Binary-verified (BRE.OVR 0x34545–0x345D1), including the order of operations:
//
//	perRegion = [0, IndustryGoldRate) + IndustryGoldBase
//	total     = perRegion * count / 100 * unallocated%
//
// Dividing by 100 before applying the percentage is BRE's own order, and it is
// why industry is the one region type whose total does not divide evenly by its
// region count — the division discards the remainder.
func (w *World) industrialGold(e *Empire) int {
	allocated := e.ProdTroopers + e.ProdJets + e.ProdTurrets + e.ProdBombers + e.ProdTanks + e.ProdCarriers
	// Whatever is not building units pays gold, whether the player set it aside
	// on the Gold row or simply left it unallocated.
	unalloc := 100 - allocated
	if unalloc < 0 {
		unalloc = 0
	}
	return w.ProjectedIndustrialGold(e, unalloc)
}

// ProjectedIndustrialGold is the gold that pct% of this empire's industrial
// capacity pays out this turn. Shared with the Set Industries screen so the
// figure a player is shown is the one industrialGold will credit — regionDraw
// varies by empire and game day, not per call, so the two always agree.
func (w *World) ProjectedIndustrialGold(e *Empire, pct int) int {
	if pct <= 0 {
		return 0
	}
	perRegion := w.regionDraw(e, 5, IndustryGoldRate) + IndustryGoldBase
	return perRegion * e.Regions.Industrial / 100 * pct
}

// riverGold is one River region's hydropower gold this turn: the full yield
// less the share taken as food (see RiverFishShare). Rivers have the highest
// base but an occasional "bad year" (a small deterministic chance, keyed off a
// separate yield salt) that halves the take.
func (w *World) riverGold(e *Empire) int {
	yield := w.regionDraw(e, 4, RiverRate) + RiverBase
	if w.regionDraw(e, 40, 100) < RiverDudChancePct {
		yield = RiverBase / 2
	}
	return yield * (100 - RiverFishShare) / 100
}

// IncomeThisTurn itemizes e's income for the current turn. Each region's gold
// is BRE's perRegion = Base + [0, Rate) times its region count; Coastal is
// additionally scaled by a support floor (0.10 + 0.90·support, so tourism never
// zeroes out). TechFactor scales every gold source (the tech-factor's role as
// an income multiplier is otherwise deferred to #20). Products are widened to
// int64 so they stay correct on 32-bit builds even at money-cap scale.
func (w *World) IncomeThisTurn(e *Empire) IncomeBreakdown {
	// Region income and population tax draw on DIFFERENT research slots in BRE,
	// so they scale independently.
	gold := int64(e.TechGoldFactor())
	scale := func(n int64) int { return int(n * gold / TechFactorUnit) }
	tax := int64(e.TechTaxFactor())
	scaleTax := func(n int64) int { return int(n * tax / TechFactorUnit) }
	perRegion := func(salt, rate, base int) int { return w.regionDraw(e, salt, rate) + base }

	support := 10 + 90*e.Support/100 // support factor ×100: 0.10 + 0.90·(Support/100)
	riverGold := w.riverGold(e)
	return IncomeBreakdown{
		Taxes:      scaleTax(int64(e.People) * int64(e.Tax) / 100 * TaxGoldPerCapita),
		Ore:        scale(int64(perRegion(1, MountainRate, MountainBase)) * int64(e.Regions.Mountain)),
		Tourism:    scale(int64(perRegion(2, CoastalRate, CoastalBase)) * int64(support) / 100 * int64(e.Regions.Coastal)),
		Solar:      scale(int64(perRegion(3, DesertRate, DesertBase)) * int64(e.Regions.Desert)),
		Rivers:     scale(int64(riverGold) * int64(e.Regions.River)),
		Industrial: scale(int64(w.industrialGold(e))),
		Trade:      w.tradeIncome(e), // trade-treaty bonus (population-scaled)
		Food:       w.FoodProduced(e),
		RiverFood:  w.riverFood(e),
	}
}

// riverFood is the food e's rivers fish this turn. Unlike BRE, where a river
// runs hydropower OR fishes and the empire finds out which only when the income
// report prints, IB's rivers always do both — RiverFishShare of the yield as
// food, the rest as gold (see riverGold).
func (w *World) riverFood(e *Empire) int {
	return e.Regions.River * RiverFishFood * RiverFishShare / 100
}

// FoodGrown is the empire's total food production this turn: its tech-boosted
// food-region output (FoodProduced) plus river fishing. The single source of
// truth for produced food, so the turn engine, the AI, and the advisors agree.
func (w *World) FoodGrown(e *Empire) int {
	return w.FoodProduced(e) + w.riverFood(e)
}

// CollectIncome credits this turn's gold income (see IncomeThisTurn) at the
// start of the turn, so it is in hand for maintenance and spending. BRE shows
// the income report then, and its auto-deposit banks only the "extra" gold
// left at the end of the turn. Keeping this out of processEconomy (the
// end-of-turn steps: interest, food, population) is what lets start-of-turn
// maintenance be paid from the income the turn earns, instead of a turn
// behind. manufacture is likewise called at turn start, alongside this, so
// freshly-produced units are on hand the same turn (#71).
// GrowFood credits this turn's food yield (region output + river fishing) at the
// START of the turn, alongside Manufacture (military) and CollectIncome (gold) —
// the three things the start-of-turn income report announces. BRE grows food at
// turn start, so the player can sell or spend this turn's growth the same turn;
// deferring it to processEconomy (the old behavior) meant the growth arrived
// after the food market and always spoiled 5% (verified against BRE, 2026-07-20).
// Consumption and spoilage stay in processEconomy (end of turn).
func (w *World) GrowFood(e *Empire) {
	e.Food += w.FoodGrown(e)
}

func (w *World) CollectIncome(e *Empire) {
	w.creditGold(e, int64(w.IncomeThisTurn(e).Gold()), "this turn's income")
}

func (w *World) processEconomy(e *Empire) {
	// Savings interest (BRE-faithful, config-help verified): the Interest Rate knob
	// is "the interest the bank gives in 10 days", so config/10 is the DAILY rate
	// (shown in View Bank Rates: config 50 → 5.0%/day). BRE credits it "at the end
	// of each turn", so per turn it is the daily rate spread across the day's turns:
	// interest = Bank × (InterestRate/10)/100 / TurnsPerDay = Bank × InterestRate /
	// (1000 × TurnsPerDay). int64 throughout and clamp before storing: on a 32-bit
	// build the min(Bank,InterestCap)*InterestRate product overflows int32 before
	// the divide. Storage stays int.
	tpd := int64(w.Config.TurnsPerDay)
	if tpd < 1 {
		tpd = 1
	}
	e.Bank += min(e.Bank, InterestCap) * int64(w.Config.InterestRate) / (1000 * tpd)
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
	e.LastFoodConsumed = e.FoodUpkeep()
	e.Food -= e.LastFoodConsumed
	if e.Food < 0 {
		// Underfeeding hits popular support and drives people away, worse the
		// hungrier the realm. shortPct is the % of this turn's food need left unmet.
		// Our own reconstruction (BRE publishes no rate), calibrated to a live BRE
		// point: ~73% short dropped support ~50 points in one turn.
		shortPct := 100
		if e.LastFoodConsumed > 0 {
			shortPct = min(100, (-e.Food)*100/e.LastFoodConsumed)
		}
		e.Support -= shortPct * FoodShortfallSupportDrop / 100
		if e.Support < 0 {
			e.Support = 0
		}
		e.adjustMorale(-shortPct * FoodShortfallMoraleDrop / 100) // hungry troops lose heart too (breins.txt)
		left := e.People * shortPct * FoodShortfallEmigrationPct / 10000
		if left < 1 {
			left = 1 // at least a token loss while starving
		}
		if left > e.People {
			left = e.People
		}
		e.People -= left
		e.LastStarved = left
		e.Food = 0
		w.postStarvationNews(e)
	} else {
		e.LastStarved = 0
	}

	// Food spoilage (BRE-verified by driving the original, 2026-07-16): FoodSpoilPct
	// (5%) of the ENTIRE stored food spoils each turn — floor(0.05 × food) — with NO
	// floor below which nothing spoils. Technology decreases it (via tf). Food
	// escrowed on the Trading Market counts toward the total, so listing food doesn't
	// dodge spoilage — only attacks (#17). Spoilage comes out of the granary first,
	// then the listing.
	listedFood := w.MarketForSale(e.Owner, "Food")
	if total := e.Food + listedFood; total > 0 {
		spoiled := techLower(total*FoodSpoilPct/100, e.TechDecayFactor())
		e.LastSpoiled = spoiled
		fromGranary := spoiled
		if fromGranary > e.Food {
			fromGranary = e.Food
		}
		e.Food -= fromGranary
		w.spoilListedFood(e.Owner, spoiled-fromGranary)
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
	if growth > 0 && e.Food <= 0 {
		growth = 0 // starving realms don't grow; starvation attrition is separate
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
	e.LastRiot = false
	riotPenalty := 0
	if e.Tax > RiotTaxFloor && e.Tax*e.Tax >= w.rng.Intn(RiotChanceDenom) {
		e.LastRiot = true
		w.postRiotNews(e)
		riotPenalty = e.Tax / RiotSupportDivisor
		e.People -= e.People / RiotPeopleDivisor
	}
	e.adjustSupport(-riotPenalty - (e.Tax-SupportTaxNeutral)/SupportTaxDivisor)
	if e.Support < MoraleDrainSupport {
		e.adjustMorale(-(MoraleDrainSupport - e.Support))
	}
	// Taxing very lightly buys back support, but only while the realm is unhappy.
	if e.Tax < LowTaxBonusBelow && e.Support < LowTaxSupportCeil {
		e.adjustSupport(LowTaxBonusBelow - e.Tax)
	}

	// Morale slowly recovers toward 100 each turn (a paid, quiet army regains
	// heart); paying forces short knocks it back down in PayForces. When morale
	// stays very low, troops desert.
	if e.Morale < 100 {
		e.Morale = min(100, e.Morale+MoraleDrift)
	}
	if e.Morale <= MoraleDesertThreshold {
		lost := 0
		desert := func(n *int) {
			d := *n * MoraleDesertRate / 100
			*n -= d
			lost += d
		}
		desert(&e.Troopers)
		desert(&e.Jets)
		desert(&e.Turrets)
		desert(&e.Tanks)
		e.LastMoraleDesertion = lost
	} else {
		e.LastMoraleDesertion = 0
	}

	if e.Gold > w.MoneyCap() {
		e.Gold = w.MoneyCap()
	}
	if e.Bank > w.MoneyCap() {
		e.Bank = w.MoneyCap()
	}
}

// Industrial production tuning (v1, tunable — see docs/mechanics-reference.md).
// Industrial gold is credited via IncomeThisTurn (see industrialGold); this
// governs unit production only.
// industryMountainBoost is the multiplier Mountain regions give to unit
// manufacturing, returned as an exact fraction num/den: BRE computes
// 1 + MountainIndustryNum*Mountain/Total in floating point and truncates only
// the final unit count, so rounding it to whole percent here costs real units.
//
// BRE ties industrial output to the *share* of the realm that is mountains, so
// the boost dilutes as an empire expands elsewhere — a realm cannot hold the cap
// without buying mountains alongside everything else.
func industryMountainBoost(r RegionMix) (num, den int) {
	total := r.Total()
	if total == 0 {
		return 1, 1
	}
	// Capped once MountainIndustryNum*Mountain/Total exceeds the cap's fraction.
	if (100 + MountainIndustryNum*100*r.Mountain/total) > MountainIndustryCapPct {
		return MountainIndustryCapPct, 100
	}
	return total + MountainIndustryNum*r.Mountain, total
}

// ProjectedProduction computes the units e would manufacture this turn at its
// current Industrial regions, percentages, and specialization — without
// applying them. Order matches the Set Industries screen: Troopers, Jets,
// Turrets, Bombers, Tanks, Carriers.
//
// The percentage split always governs how points are allocated. On top of that,
// specialization applies a per-unit efficiency modifier — a bonus to the
// specialized unit, a penalty to everything else — and Mountain regions boost
// the whole pool (see industryMountainBoost).
//
// Binary-verified against BRE.OVR: the unit pool is UnitPointsPerRegion, which
// is NOT the gold pool, and BRE keeps the whole chain in floating point and
// ROUNDS once at the end. All three matter — using the gold pool overproduces by
// ~24%, rounding the mountain boost to whole percent loses up to 0.3%, and
// truncating instead of rounding is off by one unit whenever the fraction
// exceeds a half (which is what the 1086-industrial live sample caught).
func (w *World) ProjectedProduction(e *Empire) [6]int {
	boostNum, boostDen := industryMountainBoost(e.Regions)
	made := func(name string, pct, cost int) int {
		spec := 100
		switch {
		case e.Specialized == name:
			spec = 100 + SpecialtyBonusPct
		case e.Specialized != "":
			spec = 100 - SpecialtyPenaltyPct
		}
		n := int64(e.Regions.Industrial) * UnitPointsPerRegion *
			int64(pct) * int64(spec) * int64(boostNum) * int64(e.TechUnitFactor())
		d := int64(cost) * 100 * 100 * int64(boostDen) * TechFactorUnit
		return int((n + d/2) / d)
	}
	return [6]int{
		made("Troopers", e.ProdTroopers, CostTrooper),
		made("Jets", e.ProdJets, CostJet),
		made("Turrets", e.ProdTurrets, CostTurret),
		made("Bombers", e.ProdBombers, CostBomber),
		made("Tanks", e.ProdTanks, CostTank),
		made("Carriers", e.ProdCarriers, CostCarrier),
	}
}

// Manufacture converts e's Industrial regions into production points and spends
// them on units per e.ProdXxx percentages (see ProjectedProduction).
// Specialization applies a per-unit efficiency bonus/penalty on top; it never
// overrides the percentage split. Called at turn start, alongside CollectIncome
// (#71). Industrial GOLD is not credited here — it flows through IncomeThisTurn
// (see industrialGold).
func (w *World) Manufacture(e *Empire) {

	p := w.ProjectedProduction(e)
	e.MadeTroopers, e.MadeJets, e.MadeTurrets = p[0], p[1], p[2]
	e.MadeBombers, e.MadeTanks, e.MadeCarriers = p[3], p[4], p[5]

	e.Troopers += e.MadeTroopers
	e.Jets += e.MadeJets
	e.Turrets += e.MadeTurrets
	e.Bombers += e.MadeBombers
	e.Tanks += e.MadeTanks
	e.Carriers += e.MadeCarriers
}
