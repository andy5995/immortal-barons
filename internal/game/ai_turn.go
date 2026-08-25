package game

// ai_turn.go — how a computer baron spends its turn. ai.go holds the profiles
// and skills that colour these choices; this is the sequence each one runs.

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
	upkeep := w.FoodDue(e)

	// 1. Keep a food buffer, spending at most half the treasury on it so expansion
	//    gold survives. AIFoodBufferTurns is sized to ride out a day of consumption
	//    plus the 5% per-turn spoilage; too small a buffer let the correct BRE
	//    spoilage drain the food mid-day and starve the realm's people.
	if target := upkeep * AIFoodBufferTurns; e.Food < target {
		if price := w.FoodBuyPrice(); price > 0 {
			buy := target - e.Food
			if afford := UnitsAffordable(e.Gold/2, price); buy > afford {
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
		if afford := UnitsAffordable(e.Gold, w.Prices.Land); n > afford {
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
	budget := pctOf(e.Gold-reserve, AILandBudgetPct)
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
	// Spend out of the SURPLUS above the working reserve — the same pot land
	// buying draws on. Taking a share of ALL gold pinned a grown realm's
	// treasury at about one turn's income and stopped it expanding for good:
	// aiExpandLand waits for gold to clear three days of upkeep, and half of
	// everything saved toward that went on soldiers before it ever got there.
	// Two of four AI realms froze this way within thirty days.
	spendable := max(int64(0), e.Gold-w.aiReserve(e))
	budget := pctOf(spendable, AIMilitaryBudgetPct)
	buy := func(share, price int, count *int) {
		if price <= 0 {
			return
		}
		if n := UnitsAffordable(pctOf(budget, share), price); n > 0 {
			*count += n
			e.Gold -= goldCost(n, price)
		}
	}
	mix := aiForceShares(e.aiProfile())
	// Under threat the realm stops building the army it wants and buys the one it
	// needs: a bigger slice of gold, spent turret-heavy.
	if w.aiUnderThreat(e) {
		budget = pctOf(spendable, AIThreatBudgetPct)
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
	if afford := UnitsAffordable(e.Gold, price); short > afford {
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
	for _, g := range MarketGoods {
		good := g.Singular
		shop := w.shopPrice(e, good)
		if shop <= 0 {
			continue
		}
		want := shop * (100 - AIMarketBuyDiscountPct) / 100
		for _, l := range w.MarketSellers(good, e.Name) {
			if l.Price > want || l.Price <= 0 {
				continue
			}
			n := min(l.Qty, UnitsAffordable(budget, l.Price))
			if n <= 0 {
				continue
			}
			if w.BuyFromMarket(e, l.Realm, good, n) == nil {
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
// farming-heavy realm does not sit on food that only spoils.
func (w *World) aiListSurplus(e *Empire) {
	list := func(good string, surplus, shop int) {
		if surplus <= 0 || shop <= 0 {
			return
		}
		// While the shop sells the unit freely, listing it under the shop price
		// arms nobody — it only offers a discount on something a rival could
		// already buy. Where the sysop has closed the shop, the market is the
		// planet's only military supply, and an AI stocking it would be handing
		// every enemy the units the setting was meant to withhold.
		if good != "Food" && w.Config.BuyMilitary != BuyYes {
			return
		}
		// Do NOT stack onto an existing listing: the surplus is recomputed every
		// turn, so adding to it each time grows the escrow without bound (a sim ran
		// one baron to 600k jets listed). One offer at a time, replaced only once
		// the last has cleared.
		if w.MarketForSale(e.Name, good) > 0 {
			return
		}
		if qty := surplus * AIMarketListPct / 100; qty > 0 {
			w.SetMarketListing(e, good, qty, shop*(100-AIMarketUndercutPct)/100)
		}
	}
	list("Jet", e.Jets-e.Carriers*JetsPerCarrier, w.JetPrice(e))
	list("Food", e.Food-w.FoodDue(e)*AIFoodBufferTurns, w.FoodBuyPrice())
}

// shopPrice is what e would pay the shop for one unit of a market good, or 0 for
// goods the shop does not sell at a per-unit price.
func (w *World) shopPrice(e *Empire, good string) int {
	g := GoodByName(good)
	if g == nil || g.Price == nil {
		return 0
	}
	return g.Price(w, e)
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
		if ceiling := w.LoanCeiling(e, AILoanDays); want > ceiling {
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
	if w.FoodGrown(e) >= w.FoodDue(e) {
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
