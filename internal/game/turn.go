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
	// BRE score: each turn played awards the day-start net worth (flat within a
	// day). processEconomy may then subtract small riot/spoilage penalties.
	e.Score += e.DayStartNetWorth
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
	if e.TurnsLeft > 0 {
		e.TurnsLeft--
	}
	if e.Protection > 0 {
		e.Protection--
	}
	e.LastPlayed = today
}

// DailyMaintenance advances the world to `today`, running one pass per
// missed day. It is idempotent (no-op if already current) and self-catching
// up (loops over multiple missed days). The first call on a brand-new world
// just records the date.
func (w *World) DailyMaintenance(today string) {
	// Before the configured Game Start Date, players may sign up but the game
	// does not advance. Pin the clock to today so it doesn't catch up when the
	// start date arrives.
	if !w.Config.GameStarted(today) {
		w.LastMaintDate = today
		return
	}
	if w.LastMaintDate == "" {
		w.LastMaintDate = today
		return
	}
	for w.LastMaintDate < today {
		w.FoodMarketSupply = FoodMarketDailySupply // refill the food market for the new day (#19)
		for _, e := range w.Empires {
			if e.Alive {
				e.TurnsLeft = w.Config.TurnsPerDay
				// Re-snapshot the day-start net worth so the per-turn Score award
				// tracks the empire's size at the start of each new day.
				e.DayStartNetWorth = w.NetWorth(e)
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

// aiPlay runs each AI empire's turns for one day.
func (w *World) aiPlay(today string) {
	for _, e := range w.AIEmpires() {
		for e.TurnsLeft > 0 {
			if e.Gold > 5000 {
				n := (e.Gold / 4) / w.Prices.Trooper
				e.Troopers += n
				e.Gold -= n * w.Prices.Trooper
			}
			e.LastGoldPaid = 0
			w.PayForces(e, w.ForcesDue(e))
			w.PayRegions(e, w.RegionsDue(e))
			w.Manufacture(e)   // industry production also runs at turn start (#71)
			w.CollectIncome(e) // same point income used to be credited (start of PlayTurn)
			w.PlayTurn(e, today)
		}
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

// industrialGold is one Industrial region's gold this turn: the BRE per-region
// value, scaled by IB's industry-efficiency modifier. IB has no separate gold
// efficiency factor today, so the modifier is 100% (a no-op hook for a future
// tech-driven industry bonus). This is credited once, via IncomeThisTurn — the
// old double-count (here AND in Manufacture) is gone.
func (w *World) industrialGold(e *Empire) int {
	base := w.regionYield(e, 5)*IndustrialRate/100 + IndustrialBase
	return base // × industry-efficiency modifier (100% today; hook for a future tech bonus)
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
		Food:       e.Regions.foodProduced(),
		RiverFood:  w.riverFood(e),
	}
}

// riversFish reports whether e's rivers fish for food (vs run hydropower for
// gold) this turn — a per-turn, per-empire either/or (BRE #29; live ~50/50).
// Deterministic in (GameDay, empire) so the income report matches what's
// credited.
func (w *World) riversFish(e *Empire) bool {
	span := YieldMax - YieldMin + 1
	return w.regionYield(e, 99)-YieldMin < span*RiverFishChance/100
}

// riverFood is the food e's rivers fish this turn: RiverFishFood per River on a
// fishing turn, 0 on a hydropower turn.
func (w *World) riverFood(e *Empire) int {
	if w.riversFish(e) {
		return e.Regions.River * RiverFishFood
	}
	return 0
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
	e.Food += e.Regions.foodProduced() + w.riverFood(e) - e.LastFoodConsumed
	if e.Food < 0 {
		e.People -= (-e.Food)/10 + 1
		if e.People < 0 {
			e.People = 0
		}
		e.Food = 0
		w.postStarvationNews(e)
	}

	// Food spoilage (BRE shape, issue #19): stored food at/below FoodSpoilFloor
	// (~1000) never spoils; above it, a fraction of the EXCESS decays, reduced by
	// Technology regions (via tf). This is why players sell surplus at the market.
	if e.Food > FoodSpoilFloor {
		spoiled := (e.Food - FoodSpoilFloor) / 25 * (100 - tf) / 100
		e.Food -= spoiled
		e.LastSpoiled = spoiled
		if spoiled > 0 {
			e.Score -= e.DayStartNetWorth / ScoreSpoilPenaltyDiv // IB: spoilage dings score a little
		}
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

	// Riots: verified against a BRE.OVR disassembly — a riot fires iff
	// tax > 10 AND tax*tax >= Random(10000), i.e. probability = tax^2 / 10000
	// (quadratic, not linear), and each riot removes People div 15 (~6.67%).
	// BRE also cancels that turn's population growth via tax/3 (not modeled
	// here); the Support hit is IB's own reconstruction, not from BRE.
	e.LastRiot = false
	if e.Tax > RiotTaxFloor && e.Tax*e.Tax >= w.rng.Intn(10000) {
		e.LastRiot = true
		e.Score -= e.DayStartNetWorth / ScoreRiotPenaltyDiv // IB: a riot dings score a little
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
const IndustryPointsPerRegion = 10 // production points each Industrial region yields per turn

// Point cost to manufacture one of each unit type; cheaper units convert
// from more of the same points.
const (
	CostTrooper = 1
	CostJet     = 2
	CostTurret  = 2
	CostCarrier = 1
	CostTank    = 4
	CostBomber  = 5
)

// Industrial specialization efficiency modifiers (reconstructed; tunable). BRE
// describes specialization as increasing output of the chosen unit and
// decreasing all others, but the exact magnitudes live in compiled code.
const (
	SpecialtyBonusPct   = 50 // + to the specialized unit's production
	SpecialtyPenaltyPct = 50 // - to every other unit's production
)

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
