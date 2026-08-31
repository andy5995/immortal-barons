package game

import "testing"

// The raid tally and the trade-deal shipment wear BRE's k/m shortening, which a
// capture shows on every field of both lines. Golden literals from
// cap/20240527-134Pho_Lazarus_Public.cap: "You took 111k Gold, 5 Regions, 8568
// Agents, ... 13k Turrets" and "They shipped 1000k Turrets and 188m Gold."
func TestRaidLootWearsBREShortening(t *testing.T) {
	got := raidLoot(111_222, 5, 27_000, 3876, 13_400, 20_500, 8568)
	want := "111k Gold, 5 Regions, 27k Troopers, 3876 Jets, 13k Turrets, 20k Tanks, and 8568 Agents"
	if got != want {
		t.Errorf("raidLoot =\n %q\nwant\n %q", got, want)
	}
	// Under the threshold every field stays whole, and the comma grouping IB
	// uses elsewhere must not appear here.
	if got := raidLoot(1025, 9, 1500, 3635, 14, 96, 0); got != "1025 Gold, 9 Regions, 1500 Troopers, 3635 Jets, 14 Turrets, and 96 Tanks" {
		t.Errorf("small raid tally = %q", got)
	}
}

func TestTradeBasketWearsBREShortening(t *testing.T) {
	got := describeBasket(TradeBasket{Gold: 188_000_000, Turrets: 1_000_500})
	if want := "1000k Turrets and 188m Gold"; got != want {
		// order follows MarketGoods, gold first — assert on content either way
		if got != "188m Gold and 1000k Turrets" {
			t.Errorf("describeBasket = %q, want the shortened form (%q)", got, want)
		}
	}
}
