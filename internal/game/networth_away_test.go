package game

import "testing"

// Units committed to an interplanetary attack keep their net-worth value while
// they are away, as the original's do (record +0x125, added by
// configure_attack_forces and read by the net-worth function). Sending a strike
// must not make a realm look poorer, and the value must leave with the
// detachment when the answer comes home rather than being counted twice.
func TestForcesAwayKeepTheirNetWorth(t *testing.T) {
	wA, wB, attacker, _ := twoBoards(t)
	before := wA.NetWorth(attacker)

	if _, err := wA.CreateIndividualAttack(attacker, "boardB", "Victim", NormalAttack,
		AttackForce{Troopers: 500_000, Tanks: 5000}); err != nil {
		t.Fatalf("CreateIndividualAttack: %v", err)
	}
	if _, err := wA.CreateGroupAttack(attacker, "boardB", "Victim", 12,
		AttackForce{Troopers: 100_000}); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	if got := wA.NetWorth(attacker); got != before {
		t.Errorf("net worth with forces away = %d, want %d as before they left", got, before)
	}

	// What is away is worth 500,000 x 0.25 + 5,000 x 1.25 + 100,000 x 0.25.
	inFlight, parties := wA.InFlight, wA.GroupAttacks
	wA.InFlight, wA.GroupAttacks = nil, nil
	if dip := before - wA.NetWorth(attacker); dip != 156_250 {
		t.Errorf("forces away are worth %d, want 156250", dip)
	}
	wA.InFlight, wA.GroupAttacks = inFlight, parties

	result := wB.receive(wA.Outbox[0])
	wA.Outbox = nil
	wA.receive(result)
	if len(wA.InFlight) != 0 {
		t.Fatalf("%d strikes still in flight after the answer came home", len(wA.InFlight))
	}
	// The survivors are back in the army and out of the away count, so a strike
	// that lost units can only have lowered net worth.
	if got := wA.NetWorth(attacker); got > before {
		t.Errorf("net worth after the strike came home = %d, above the %d before it left", got, before)
	}
}

// Goods listed on the Trading Market leave inventory for escrow, and the
// original's net worth adds each type's escrow to its home count (record
// +0x211), so listing a stock for sale must not lower net worth.
func TestListedGoodsKeepTheirNetWorth(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("seller", "Seller")
	e.Protection = 0
	e.Tanks = 1000
	before := w.NetWorth(e)
	if err := w.SetMarketListing(e, Tank.Singular, 400, 100); err != nil {
		t.Fatalf("SetMarketListing: %v", err)
	}
	if e.Tanks != 600 {
		t.Fatalf("listing escrowed %d tanks, want 400", 1000-e.Tanks)
	}
	if got := w.NetWorth(e); got != before {
		t.Errorf("net worth with 400 tanks listed = %d, want %d", got, before)
	}
}

// Goods offered in a trade deal are escrowed until it is answered, and the
// original adds their value to record +0x125 when the offer is made
// (create_trade_offer), so a pending offer keeps its goods' net worth. The
// carrier spent as transport is consumed, and is not counted.
func TestOfferedGoodsKeepTheirNetWorth(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	from := w.AddHuman("f", "Fromland")
	to := w.AddHuman("t", "Toland")
	pastProtection(w)
	pactAll(w, fullDefenseAlliance)
	from.Tanks, from.Carriers, from.Gold = 500, 1, 300_000
	before := w.NetWorth(from)
	if err := w.SendTradeDeal(from, to, TradeBasket{Tanks: 100}, TradeBasket{Gold: 5_000}, TradeDealMinDays); err != nil {
		t.Fatalf("send: %v", err)
	}
	// One carrier at 1.000 went as transport; the 100 tanks only moved to escrow.
	if got := w.NetWorth(from); got != before-1 {
		t.Errorf("net worth with 100 tanks on offer = %d, want %d", got, before-1)
	}
}
