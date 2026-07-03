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

	// Interest: the bank pays, debt costs.
	e.Bank += e.Bank * 5 / 100
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

	// Maintenance: unpaid armies desert.
	maint := e.Troopers*2 + e.Jets*10 + e.Tanks*20
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
		if p != nil && p.Alive && e.Power() > p.Power()*2 && w.rng.Intn(100) < 10 {
			w.Attack(e, p)
		}
	}
}
