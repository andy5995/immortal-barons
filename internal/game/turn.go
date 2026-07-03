package game

import "time"

// PlayTurn processes one turn for a single empire: its economy runs, and
// its turn/protection counters tick down. Idle empires (whose owner never
// plays) never call this, so they stagnate.
func (w *World) PlayTurn(e *Empire, today string) {
	w.processEconomy(e)
	w.manufacture(e)
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
	if w.LastMaintDate == "" {
		w.LastMaintDate = today
		return
	}
	for w.LastMaintDate < today {
		for _, e := range w.Empires {
			if e.Alive {
				e.TurnsLeft = w.Config.TurnsPerDay
			}
		}
		w.aiPlay(w.LastMaintDate)
		w.piratesRaid()
		for _, e := range w.Empires {
			if e.Owner == "" {
				e.Events = nil
			}
		}
		for _, e := range w.Empires {
			if e.Alive && (e.Land <= 0 || e.People <= 0) {
				e.Alive = false
			}
		}
		w.GameDay++
		for _, e := range w.Empires {
			if e.Alive {
				w.matureInvestments(e)
			}
		}
		w.adjustInvestRate()
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
			w.PayForces(e, e.ForcesUpkeep())
			w.PayRegions(e, e.RegionUpkeep())
			w.PlayTurn(e, today)
		}
	}
}

// Support tuning (v1, tunable — see docs/mechanics-reference.md).
const (
	SupportStableTax = 15 // tax rate at which support holds at 100
	SupportDrift     = 3  // points support moves toward its target per turn
	RiotTaxFloor     = 20 // no riots at/below this tax rate
)

func (w *World) processEconomy(e *Empire) {
	tf := e.techFactor()

	base := e.Regions.income()
	coastal := e.Regions.Coastal * 25
	income := base - coastal + coastal*e.Support/100 // low support slashes tourism
	income = income * (100 + tf) / 100
	tax := e.People * e.Tax / 100 * 8 * (100 + tf) / 100
	e.Gold += tax + income

	e.Bank += min(e.Bank, InterestCap) / 100
	if e.Debt > 0 {
		e.Debt += e.Debt * 10 / 100
	}

	e.LastFoodConsumed = e.People + e.Troopers + e.Jets*2 + e.Tanks*2
	e.Food += e.Regions.foodProduced() - e.LastFoodConsumed
	if e.Food < 0 {
		e.People -= (-e.Food)/10 + 1
		if e.People < 0 {
			e.People = 0
		}
		e.Food = 0
	}

	// Hoarded food spoils beyond a two-turn buffer (v1 tunable — a modest
	// buffer is safe; it's why players sell surplus at the food market).
	buffer := (e.People + e.Troopers + e.Jets*2 + e.Tanks*2) * 2
	if e.Food > buffer {
		spoiled := (e.Food - buffer) / 25 * (100 - tf) / 100
		e.Food -= spoiled
		e.LastSpoiled = spoiled
	} else {
		e.LastSpoiled = 0
	}

	e.LastPopGrowth = 0
	if e.Food > 0 {
		if g := e.People * (10 - e.Tax/5) / 100; g > 0 {
			e.People += g
			e.LastPopGrowth = g
		}
	}
	e.People += e.Regions.Urban * 10 // urban regions draw settlers

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

	// Riots: chance rises with tax above the floor.
	e.LastRiot = false
	if e.Tax > RiotTaxFloor && w.rng.Intn(100) < (e.Tax-SupportStableTax)*2 {
		e.LastRiot = true
		e.Support -= 15
		if e.Support < 0 {
			e.Support = 0
		}
		e.People -= e.People / 10 // a tenth flee in the unrest
	}

	if e.Gold > MoneyCap {
		e.Gold = MoneyCap
	}
	if e.Bank > MoneyCap {
		e.Bank = MoneyCap
	}
}

// Industrial production tuning (v1, tunable — see docs/mechanics-reference.md).
const (
	IndustryPointsPerRegion = 10  // production points each Industrial region yields per turn
	IndustryGoldPerRegion   = 250 // gold each Industrial region yields per turn
)

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

// manufacture converts e's Industrial regions into production points and
// gold, then spends the points on units per e.ProdXxx percentages (or, if
// e.Specialized is set, entirely on that one unit type).
func (w *World) manufacture(e *Empire) {
	e.IndustryGold = e.Regions.Industrial * IndustryGoldPerRegion
	e.Gold += e.IndustryGold

	pts := e.Regions.Industrial * IndustryPointsPerRegion
	e.MadeTroopers, e.MadeJets, e.MadeTurrets = 0, 0, 0
	e.MadeBombers, e.MadeTanks, e.MadeCarriers = 0, 0, 0
	if pts <= 0 {
		return
	}

	if e.Specialized != "" {
		switch e.Specialized {
		case "Troopers":
			e.MadeTroopers = pts / CostTrooper
		case "Jets":
			e.MadeJets = pts / CostJet
		case "Turrets":
			e.MadeTurrets = pts / CostTurret
		case "Bombers":
			e.MadeBombers = pts / CostBomber
		case "Tanks":
			e.MadeTanks = pts / CostTank
		case "Carriers":
			e.MadeCarriers = pts / CostCarrier
		}
	} else {
		made := func(pct, cost int) int { return (pts * pct / 100) / cost }
		e.MadeTroopers = made(e.ProdTroopers, CostTrooper)
		e.MadeJets = made(e.ProdJets, CostJet)
		e.MadeTurrets = made(e.ProdTurrets, CostTurret)
		e.MadeBombers = made(e.ProdBombers, CostBomber)
		e.MadeTanks = made(e.ProdTanks, CostTank)
		e.MadeCarriers = made(e.ProdCarriers, CostCarrier)
	}

	e.Troopers += e.MadeTroopers
	e.Jets += e.MadeJets
	e.Turrets += e.MadeTurrets
	e.Bombers += e.MadeBombers
	e.Tanks += e.MadeTanks
	e.Carriers += e.MadeCarriers
}
