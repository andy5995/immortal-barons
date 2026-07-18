package game

import (
	"encoding/binary"
	"hash/fnv"
	"io"
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
	w.advanceTech(e) // Technology bonus builds up a little each turn (not instant)
	w.processEconomy(e)
	if e.Score < 0 {
		e.Score = 0
	}
	if e.HQ > 0 && e.HQ < 100 {
		e.HQ += 5
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
	e.MaintUnderpaid = false // cleared for next turn; set again by PayForces/PayRegions on underpayment
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

// DailyMaintenance advances the world to `today`, running one pass per
// missed day. It is idempotent (no-op if already current) and self-catching
// up (loops over multiple missed days). The first call on a brand-new world
// just records the date. It returns a MaintReport summarizing what it did.
func (w *World) DailyMaintenance(today string) MaintReport {
	// Before the configured Game Start Date, players may sign up but the game
	// does not advance. Pin the clock to today so it doesn't catch up when the
	// start date arrives.
	if !w.Config.GameStarted(today) {
		w.LastMaintDate = today
		return MaintReport{NotStarted: true}
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
	days := 0
	for w.LastMaintDate < today {
		w.settleMarketProceeds()                   // "Depositing trading market money" — pay sellers at day-end (#17)
		w.FoodMarketSupply = FoodMarketDailySupply // refill the food market for the new day (#19)
		for _, e := range w.Empires {
			if e.Alive {
				e.TurnsLeft = w.Config.TurnsPerDay
				e.AttacksToday = 0 // fresh day: the individual-attack allotment resets
			}
		}
		w.aiPlay(w.LastMaintDate)
		w.piratesRaid()
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
		for _, e := range w.Empires {
			if e.Alive {
				w.matureInvestments(e)
			}
		}
		w.adjustInvestRate()
		w.postMasterNews()
		w.rollNews()
		if w.Config.GameLength > 0 && w.GameDay >= w.Config.GameLength {
			w.endGame()
		}
		days++
		next := w.nextDate(w.LastMaintDate)
		if next == w.LastMaintDate {
			w.LastMaintDate = today // malformed date; snap to today to stop repeating
			break
		}
		w.LastMaintDate = next
	}
	// Sweep stale husks even when no day rolled over (e.g. a same-day -maint on
	// an already-current world), so past-day dead realms don't linger. A realm
	// that died today (DiedDay == GameDay) is kept by removeDeadHusks.
	w.removeDeadHusks()
	return MaintReport{Days: days}
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
			w.Manufacture(e)   // industry production at turn start (#71)
			w.CollectIncome(e) // income in hand before anything is spent
			e.LastGoldPaid = 0
			w.PayForces(e, w.ForcesDue(e))
			w.PayRegions(e, w.RegionsDue(e))
			w.aiManageEconomy(e) // discretionary spending: food, military, land
			w.aiWageWar(e)       // aggressors strike a weak neighbour when clearly favored (#36)
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
			if buy > (e.Gold/2)/price {
				buy = (e.Gold / 2) / price
			}
			if buy > 0 {
				e.Food += buy
				e.Gold -= buy * price
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
	if produced < upkeep && e.Gold > w.Prices.Land {
		n := (upkeep-produced)/FoodPerAgri + 1
		if afford := e.Gold / w.Prices.Land; n > afford {
			n = afford
		}
		if n > AIAgriBuyMax {
			n = AIAgriBuyMax
		}
		if n > 0 {
			e.Regions.Agricultural += n
			e.syncLand()
			e.Gold -= n * w.Prices.Land
			return
		}
	}

	// 3. Food-healthy. Under New Realm Protection the AI can't be attacked, so it
	//    goes all-in on land like a strong human (community strategy guides: plow
	//    every coin into money-making regions during protection, compounding
	//    land -> income -> land). Once protection lapses it buys a defensive force
	//    and an HQ first, then keeps expanding with what's left. Surplus above the
	//    reserve after land is capped is invested (aiInvestIdle), not hoarded.
	if e.Protection == 0 {
		w.aiBuildForces(e)
		w.aiStartHQ(e)
	}
	w.aiExpandLand(e)
	w.aiInvestIdle(e)
}

// aiExpandLand plows the AI's surplus gold into Coastal regions — the compounding
// land rush a strong human runs under protection (community strategy guides:
// Coastal is the early pick while popular support is high). It buys through the
// same BuyRegions path a human uses, so the per-turn region cap and the rising
// holdings-based price apply identically. Gold below AIGoldReserve is left for
// food/maintenance; when the per-turn cap is hit the caller's aiInvestIdle parks
// the remainder instead of hoarding it.
func (w *World) aiExpandLand(e *Empire) {
	if e.Gold <= AIGoldReserve {
		return
	}
	budget := e.Gold - AIGoldReserve
	if e.aiSkill() == AISkillDull {
		budget = budget * AIDullLandBuyPct / 100 // dull barons hold back and grow slower
	}
	limit := w.regionBuyLimit(e)
	n, total := 0, 0
	for n < limit {
		cost := w.regionCost(e.Land + n)
		if total+cost > budget {
			break
		}
		total += cost
		n++
	}
	if n > 0 {
		w.BuyRegions(e, &e.Regions.Coastal, n)
	}
}

// aiStartHQ builds a HeadQuarters once the AI fields tanks (#36): HQ multiplies
// tank offense and defense, so it is only worth the gold when there are tanks
// to amplify. StartHQ re-checks affordability, and HQ then advances on its own
// each turn (see PlayTurn), so this fires once and needs no further management.
func (w *World) aiStartHQ(e *Empire) {
	if e.HQ == 0 && e.Tanks > 0 && e.Gold > HQCost {
		w.StartHQ(e)
	}
}

// aiBuildForces spends a share of the AI's gold on military each turn, split by
// gold-value across troopers, turrets, and tanks (#36). The old AI bought only
// troopers, so its realms had no turret defense; this gives them a real
// defensive posture while still fielding offense. Shares come from balance.go.
func (w *World) aiBuildForces(e *Empire) {
	budget := e.Gold * AIMilitaryBudgetPct / 100
	buy := func(share, price int, count *int) {
		if price <= 0 {
			return
		}
		if n := (budget * share / 100) / price; n > 0 {
			*count += n
			e.Gold -= n * price
		}
	}
	trooperPct, turretPct, tankPct := aiForceShares(e.aiProfile())
	buy(trooperPct, w.TrooperPrice(e), &e.Troopers)
	buy(turretPct, w.TurretPrice(e), &e.Turrets)
	buy(tankPct, w.TankPrice(e), &e.Tanks)
	if e.aiProfile() == AIProfileAggressor {
		buy(AIForceAgentPctWar, w.AgentPrice(e), &e.Agents) // agents for pre-war covert ops (#36)
	}
}

// aiInvestIdle parks the AI's gold above a working reserve into investments so
// idle treasury earns rather than sitting (#36). Runs after food, expansion,
// and military spending, so only a genuine surplus is locked away.
func (w *World) aiInvestIdle(e *Empire) {
	if e.Gold <= AIGoldReserve {
		return
	}
	if amt := (e.Gold - AIGoldReserve) * AIInvestPct / 100; amt > 0 {
		w.Invest(e, amt, MinInvestDays)
	}
}

// Support tuning (v1, tunable — see docs/mechanics-reference.md).
const (
	SupportStableTax = 15 // tax rate at which support holds at 100
	SupportDrift     = 3  // points support moves toward its target per turn
	RiotTaxFloor     = 10 // no riots at/below this tax rate (BRE.OVR: riots need tax > 10)
)

// Population model (IB's own tuning). BRE uses the same logistic shape —
// growth toward a carrying capacity — but with a runaway 50%/turn ceiling
// (recovered from a BRE.OVR disassembly). IB keeps the self-limiting shape and
// trades the ceiling for moderate rates; see docs/mechanics-reference.md.
const (
	PopGrowthCapPct   = 8  // max % of population gained or lost per turn
	PopGrowthApproach = 12 // headroom is closed at ~1/12 per turn (before the cap)

	// Carrying-capacity weights (per region / per support point). Land houses
	// people, urban and agricultural regions add extra headroom, and popular
	// support is the dominant lever — the same factor roles BRE uses.
	PopCapPerLand    = 20
	PopCapPerUrban   = 60
	PopCapPerAgri    = 20
	PopCapPerSupport = 30
)

// popCapacity is the carrying capacity the population grows toward. Selling
// urban/agricultural land or losing support lowers it, so population then
// drifts down toward the new capacity at the growth cap rather than via a
// separate housing-loss hit. IB's own values; see docs/mechanics-reference.md.
func (e *Empire) popCapacity() int {
	return e.Land*PopCapPerLand +
		e.Regions.Urban*PopCapPerUrban +
		e.Regions.Agricultural*PopCapPerAgri +
		e.Support*PopCapPerSupport
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

// regionYield returns this turn's income yield (percent, YieldMin..YieldMax)
// for empire e's region type identified by salt. It is deterministic in
// (w.GameDay, e.Name, salt) — NOT a fresh RNG draw — so IncomeThisTurn stays
// pure and the income report always equals what CollectIncome credits. The
// variance is per game-day: a good/bad "year" lasts the whole day.
func (w *World) regionYield(e *Empire, salt int) int {
	h := fnv.New32a()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(salt))
	h.Write(buf[:])
	io.WriteString(h, e.Name)
	span := YieldMax - YieldMin + 1
	return YieldMin + int(h.Sum32()%uint32(span))
}

// industrialGold is the gold one Industrial region yields this turn. BRE (live-
// verified) splits each region's capacity by the production percentages: the
// UNALLOCATED share ("The remaining N% will be used to produce Gold") pays out
// 1 gold per point. Units and gold come from ONE pool — allocating to units is a
// direct trade-off against industrial gold. So a realm at 100% allocation earns
// no industrial gold; leaving a remainder is how you buy gold.
func (w *World) industrialGold(e *Empire) int {
	allocated := e.ProdTroopers + e.ProdJets + e.ProdTurrets + e.ProdBombers + e.ProdTanks + e.ProdCarriers
	unalloc := 100 - allocated
	if unalloc < 0 {
		unalloc = 0
	}
	return IndustryPointsPerRegion * unalloc / 100
}

// riverGold is one River region's gold this turn. Rivers have the highest base
// but an occasional "bad year" (a small deterministic chance, keyed off a
// separate yield salt) that halves the take.
func (w *World) riverGold(e *Empire) int {
	if w.regionYield(e, 40) < YieldMin+5 { // ~10% bad-year dud (tunable)
		return RiverBase / 2
	}
	return w.regionYield(e, 4)*RiverRate/100 + RiverBase
}

// IncomeThisTurn itemizes e's income for the current turn. Each region's gold
// is BRE's perRegion = yield*Rate/100 + Base times its region count; Coastal is
// additionally scaled by a support floor (0.10 + 0.90·support, so tourism never
// zeroes out). TechFactor scales every gold source (the tech-factor's role as
// an income multiplier is otherwise deferred to #20). Products are widened to
// int64 so they stay correct on 32-bit builds even at money-cap scale.
func (w *World) IncomeThisTurn(e *Empire) IncomeBreakdown {
	tf := e.TechFactor()
	scale := func(n int64) int { return int(n * int64(100+tf) / 100) }
	perRegion := func(salt, rate, base int) int { return w.regionYield(e, salt)*rate/100 + base }

	support := 10 + 90*e.Support/100 // support factor ×100: 0.10 + 0.90·(Support/100)
	// Rivers do hydropower (gold) OR fishing (food) this turn, not both (#29).
	riverGold := 0
	if !w.riversFish(e) {
		riverGold = w.riverGold(e)
	}
	return IncomeBreakdown{
		Taxes:      scale(int64(e.People) * int64(e.Tax) / 100 * TaxGoldPerCapita),
		Ore:        scale(int64(perRegion(1, MountainRate, MountainBase)) * int64(e.Regions.Mountain)),
		Tourism:    scale(int64(perRegion(2, CoastalRate, CoastalBase)) * int64(support) / 100 * int64(e.Regions.Coastal)),
		Solar:      scale(int64(perRegion(3, DesertRate, DesertBase)) * int64(e.Regions.Desert)),
		Rivers:     scale(int64(riverGold) * int64(e.Regions.River)),
		Industrial: scale(int64(w.industrialGold(e)) * int64(e.Regions.Industrial)),
		Trade:      w.tradeIncome(e), // trade-treaty bonus (population-scaled)
		Food:       e.FoodProduced(),
		RiverFood:  w.riverFood(e),
	}
}

// riversFish reports whether e's rivers fish for food (vs run hydropower for
// gold) this turn. In BRE (#29) a "year" is one turn and the empire's people
// pick one or the other for that year, so ALL of e's rivers do the same thing
// and the choice is redrawn every turn. Deterministic in (GameDay, TurnsLeft,
// empire) so the income report matches what's credited within the turn.
func (w *World) riversFish(e *Empire) bool {
	h := fnv.New32a()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(e.TurnsLeft))
	h.Write(buf[:])
	io.WriteString(h, e.Name)
	return int(h.Sum32()%100) < RiverFishChance
}

// riverFood is the food e's rivers fish this turn: RiverFishFood per River on a
// fishing turn, 0 on a hydropower turn.
func (w *World) riverFood(e *Empire) int {
	if w.riversFish(e) {
		return e.Regions.River * RiverFishFood
	}
	return 0
}

// FoodGrown is the empire's total food production this turn: its tech-boosted
// food-region output (FoodProduced) plus river fishing. The single source of
// truth for produced food, so the turn engine, the AI, and the advisors agree.
func (w *World) FoodGrown(e *Empire) int {
	return e.FoodProduced() + w.riverFood(e)
}

// CollectIncome credits this turn's gold income (see IncomeThisTurn) at the
// start of the turn, so it is in hand for maintenance and spending. BRE shows
// the income report then, and its auto-deposit banks only the "extra" gold
// left at the end of the turn. Keeping this out of processEconomy (the
// end-of-turn steps: interest, food, population) is what lets start-of-turn
// maintenance be paid from the income the turn earns, instead of a turn
// behind. manufacture is likewise called at turn start, alongside this, so
// freshly-produced units are on hand the same turn (#71).
func (w *World) CollectIncome(e *Empire) {
	e.Gold += w.IncomeThisTurn(e).Gold()
}

func (w *World) processEconomy(e *Empire) {
	tf := e.TechFactor()

	// Bank interest scales with the league's Interest Rate knob, anchored so
	// the default (50) reproduces the historical ~1%/turn. Mapping to BRE's
	// exact per-day interest math is deferred; this keeps the knob linear.
	// Compute the whole interest step in int64 and clamp before storing: on a
	// 32-bit build (386) both the min(Bank,InterestCap)*InterestRate product AND
	// the Bank+interest sum can exceed int32 before the MoneyCap clamp. Storage
	// stays int.
	newBank := int64(e.Bank) + int64(min(e.Bank, InterestCap))*int64(w.Config.InterestRate)/5000
	if newBank > MoneyCap {
		newBank = MoneyCap
	}
	e.Bank = int(newBank)
	if e.Debt > 0 {
		e.Debt += e.Debt * DebtGrowthPct / 100
	}

	e.LastFoodConsumed = e.FoodUpkeep()
	e.Food += w.FoodGrown(e) - e.LastFoodConsumed
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
		spoiled := total * FoodSpoilPct / 100 * (100 - tf) / 100
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

	// Population follows a logistic curve toward popCapacity, capped at
	// PopGrowthCapPct%/turn in either direction (IB's own tuning; see the
	// constants above and docs/mechanics-reference.md). Positive growth needs
	// food; decline toward capacity — after selling land or losing support —
	// always runs, which is why selling urban regions drains people gradually
	// instead of via a separate housing-loss hit.
	e.LastPopGrowth = 0
	growth := (e.popCapacity() - e.People) / PopGrowthApproach
	if ceiling := e.People * PopGrowthCapPct / 100; growth > ceiling {
		growth = ceiling
	} else if growth < -ceiling {
		growth = -ceiling
	}
	if growth > 0 && e.Food <= 0 {
		growth = 0 // starving realms don't grow; starvation attrition is separate
	}
	if growth != 0 {
		e.People += growth
		e.LastPopGrowth = growth
	}

	// Support drifts toward a tax-based target (higher tax => lower target).
	target := 100 - (e.Tax-SupportStableTax)*3
	if target > 100 {
		target = 100
	}
	if target < 0 {
		target = 0
	}
	if e.Support < target {
		e.Support += min(SupportDrift, target-e.Support)
	} else if e.Support > target {
		e.Support -= min(SupportDrift, e.Support-target)
	}

	// A well-run realm — people fed AND maintenance paid in full — recovers some
	// popular support for free each turn (placeholder for BRE's pay-to-boost-support
	// mechanic, #39). LastStarved is 0 when fed (set above); MaintUnderpaid is set by
	// PayForces/PayRegions when an obligation is underpaid.
	if !e.MaintUnderpaid && e.LastStarved == 0 {
		e.Support = min(100, e.Support+SupportFedBoost)
	}

	// Riots: verified against a BRE.OVR disassembly — a riot fires iff
	// tax > 10 AND tax*tax >= Random(10000), i.e. probability = tax^2 / 10000
	// (quadratic, not linear), and each riot removes People div 15 (~6.67%).
	// BRE also cancels that turn's population growth via tax/3 (not modeled
	// here); the Support hit is IB's own reconstruction, not from BRE.
	e.LastRiot = false
	if e.Tax > RiotTaxFloor && e.Tax*e.Tax >= w.rng.Intn(10000) {
		e.LastRiot = true
		w.postRiotNews(e)
		e.Support -= 15
		if e.Support < 0 {
			e.Support = 0
		}
		e.People -= e.People / 15 // BRE (disassembly): People div 15 lost per riot
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

	if e.Gold > MoneyCap {
		e.Gold = MoneyCap
	}
	if e.Bank > MoneyCap {
		e.Bank = MoneyCap
	}
}

// Industrial production tuning (v1, tunable — see docs/mechanics-reference.md).
// Industrial gold is credited via IncomeThisTurn (see industrialGold); this
// governs unit production only.
// ProjectedProduction computes the units e would manufacture this turn at its
// current Industrial regions, percentages, and specialization — without
// applying them. Order matches the Set Industries screen: Troopers, Jets,
// Turrets, Bombers, Tanks, Carriers.
//
// The percentage split always governs how points are allocated. On top of that,
// specialization (per BRE's help: "increases the ability of your industries to
// develop a specific type of military unit ... decreases your ability to
// produce all other equipment") applies a per-unit efficiency modifier — a
// bonus to the specialized unit, a penalty to everything else. The magnitudes
// are reconstructed and tunable; the exact BRE values would come from a
// disassembly of the original binary.
func (w *World) ProjectedProduction(e *Empire) [6]int {
	pts := e.Regions.Industrial * IndustryPointsPerRegion
	made := func(name string, pct, cost int) int {
		units := (pts * pct / 100) / cost
		switch {
		case e.Specialized == "" || e.Specialized == name:
			if e.Specialized == name {
				units = units * (100 + SpecialtyBonusPct) / 100
			}
		default:
			units = units * (100 - SpecialtyPenaltyPct) / 100
		}
		return units
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
// (see industrialGold); e.IndustryGold is set for the report only.
func (w *World) Manufacture(e *Empire) {
	e.IndustryGold = w.industrialGold(e) * e.Regions.Industrial

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
