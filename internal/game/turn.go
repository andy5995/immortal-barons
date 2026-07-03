package game

// EndTurn advances the world one turn: every living empire's economy is
// processed, rival AIs act, collapsed empires are marked dead, and the
// turn counter increments. It returns human-readable event lines.
func (w *World) EndTurn() []string {
	var log []string
	for _, e := range w.Empires {
		if e.Alive {
			w.processEconomy(e)
		}
	}
	w.aiActions()
	for _, e := range w.Empires {
		if e.Alive && (e.Land <= 0 || e.People <= 0) {
			e.Alive = false
			log = append(log, e.Name+" has collapsed to nothingness!")
		}
	}
	w.Turn++
	return log
}

func (w *World) processEconomy(e *Empire) {
	// Taxes and land revenue.
	e.Gold += e.People*e.Tax/100*8 + e.Land*20

	// Interest: the bank pays ~1% per turn, but only on gold up to the
	// interest cap. Debt costs 10% per turn.
	e.Bank += min(e.Bank, InterestCap) / 100
	if e.Debt > 0 {
		e.Debt += e.Debt * 10 / 100
	}

	// Food: regions produce, people and army consume.
	e.Food += e.Land*100 - (e.People + e.Troopers + e.Jets*2 + e.Tanks*2)
	if e.Food < 0 {
		e.People -= (-e.Food)/10 + 1
		if e.People < 0 {
			e.People = 0
		}
		e.Food = 0
	}

	// Maintenance (reference ratios: tanks cheap, jets/turrets dear).
	// Unpaid armies desert.
	maint := e.Troopers*6 + e.Jets*12 + e.Turrets*9 + e.Tanks*6 + e.Carriers*1
	if e.Gold >= maint {
		e.Gold -= maint
	} else {
		e.Gold = 0
		e.Troopers -= e.Troopers / 10
	}

	// Population growth when fed; lower taxes grow faster.
	if e.Food > 0 {
		if g := e.People * (10 - e.Tax/5) / 100; g > 0 {
			e.People += g
		}
	}

	// No empire can hold more than the money cap.
	if e.Gold > MoneyCap {
		e.Gold = MoneyCap
	}
	if e.Bank > MoneyCap {
		e.Bank = MoneyCap
	}
}

// aiActions is a deliberately simple rival AI: reinvest spare gold into
// troopers, and attack the player when clearly stronger.
func (w *World) aiActions() {
	p := w.Player()
	for _, e := range w.Rivals() {
		if e.Gold > 5000 {
			n := (e.Gold / 4) / w.Prices.Trooper
			e.Troopers += n
			e.Gold -= n * w.Prices.Trooper
		}
		if p != nil && p.Alive && e.Offense() > p.Defense()*2 && w.rng.Intn(100) < 10 {
			w.Attack(e, p)
		}
	}
}
