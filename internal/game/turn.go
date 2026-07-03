package game

import "time"

// PlayTurn processes one turn for a single empire: its economy runs, and
// its turn/protection counters tick down. Idle empires (whose owner never
// plays) never call this, so they stagnate.
func (w *World) PlayTurn(e *Empire, today string) {
	w.processEconomy(e)
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
	base := e.Regions.income()
	coastal := e.Regions.Coastal * 25
	income := base - coastal + coastal*e.Support/100 // low support slashes tourism
	e.Gold += e.People*e.Tax/100*8 + income

	e.Bank += min(e.Bank, InterestCap) / 100
	if e.Debt > 0 {
		e.Debt += e.Debt * 10 / 100
	}

	e.Food += e.Regions.foodProduced() - (e.People + e.Troopers + e.Jets*2 + e.Tanks*2)
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
		spoiled := (e.Food - buffer) / 25
		e.Food -= spoiled
		e.LastSpoiled = spoiled
	} else {
		e.LastSpoiled = 0
	}

	maint := e.Troopers*6 + e.Jets*12 + e.Turrets*9 + e.Tanks*6 + e.Carriers*1
	if e.Gold >= maint {
		e.Gold -= maint
	} else {
		e.Gold = 0
		e.Troopers -= e.Troopers / 10
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
