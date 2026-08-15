package game

import (
	"strings"
	"testing"
)

// twoTradingBoards builds a buyer's board and a seller's board that have marked
// each other Allied, with the seller offering `qty` tanks at `price`.
func twoTradingBoards(t *testing.T, qty, price int) (buyerW *World, buyer *Empire, sellerW *World, seller *Empire) {
	t.Helper()
	bc := DefaultConfig()
	bc.IBBS, bc.BoardID = true, "Alpha"
	buyerW = NewWorldSeed(bc, 1)
	buyer = buyerW.AddHuman("buyer", "Ironhold")
	buyer.Gold = 10_000_000
	buyer.Protection = 0 // a realm still under New Realm Protection cannot trade
	buyerW.SetPlanetRelationWith("Bravo", PlanetAllied)

	sc := DefaultConfig()
	sc.IBBS, sc.BoardID = true, "Bravo"
	sellerW = NewWorldSeed(sc, 2)
	seller = sellerW.AddHuman("seller", "Redlands")
	seller.Protection = 0
	seller.Tanks = qty
	if err := sellerW.SetMarketListing(seller, "Tank", qty, price); err != nil {
		t.Fatalf("list: %v", err)
	}
	sellerW.SetPlanetRelationWith("Alpha", PlanetAllied)
	return
}

// The whole round trip: the seller's market rides its scores packet, the buyer
// bids against that snapshot, the gold leaves at once, the far board fills it
// and pays the seller, and the goods come home.
func TestInterplanetaryBidRoundTrip(t *testing.T) {
	bw, buyer, sw, seller := twoTradingBoards(t, 100, 500)

	// The seller's market reaches the buyer with its scores.
	sw.ExportScores()
	if len(sw.Outbox) == 0 {
		t.Fatal("the seller board queued no scores packet")
	}
	bw.ApplyPacket(sw.Outbox[0])
	snap := bw.RemoteMarket("Bravo")
	if len(snap) != 1 || snap[0].Realm != "Redlands" || snap[0].Qty != 100 || snap[0].Price != 500 {
		t.Fatalf("the buyer sees %+v, want one Redlands tank listing of 100 at 500", snap)
	}

	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	if spent := goldBefore - buyer.Gold; spent != 20_000 {
		t.Errorf("the bid escrowed %d gold, want 40x500 = 20,000", spent)
	}
	if buyer.Tanks != 0 {
		t.Errorf("the buyer has %d tanks already; nothing should arrive before the packet does", buyer.Tanks)
	}

	// The bid lands, fills, and the answer comes home.
	bid := bw.Outbox[len(bw.Outbox)-1]
	reply := sw.ApplyPacket(bid)
	if seller.Tanks != 0 {
		t.Errorf("the seller kept %d tanks in hand; the listing was escrowed", seller.Tanks)
	}
	if left := sw.MarketForSale("Redlands", "Tank"); left != 60 {
		t.Errorf("the listing has %d tanks left, want 60", left)
	}
	if got := sw.MarketProceeds["Redlands"]; got != 20_000 {
		t.Errorf("the seller is owed %d, want the 20,000 the buyer paid", got)
	}
	bw.ApplyPacket(reply)
	if buyer.Tanks != 40 {
		t.Errorf("the buyer received %d tanks, want 40", buyer.Tanks)
	}
	if buyer.Gold != goldBefore-20_000 {
		t.Errorf("gold = %d, want the escrow spent and nothing refunded", buyer.Gold)
	}
	if len(bw.InFlight) != 0 {
		t.Errorf("%d bids still in flight after the answer arrived", len(bw.InFlight))
	}
}

// A listing that moved on refuses the bid, and every coin goes back.
func TestInterplanetaryBidRefundedWhenThePriceMoved(t *testing.T) {
	bw, buyer, sw, seller := twoTradingBoards(t, 100, 500)
	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	// The seller reprices before the bid lands — the stale-snapshot case this
	// whole design exists for.
	if err := sw.SetMarketListing(seller, "Tank", 100, 900); err != nil {
		t.Fatalf("reprice: %v", err)
	}

	reply := sw.ApplyPacket(bw.Outbox[len(bw.Outbox)-1])
	if sw.MarketForSale("Redlands", "Tank") != 100 {
		t.Error("the listing should be untouched by a refused bid")
	}
	if sw.MarketProceeds["Redlands"] != 0 {
		t.Error("the seller was paid for a sale that did not happen")
	}
	bw.ApplyPacket(reply)
	if buyer.Tanks != 0 {
		t.Errorf("the buyer got %d tanks from a refused bid", buyer.Tanks)
	}
	if buyer.Gold != goldBefore {
		t.Errorf("gold = %d, want the full %d back", buyer.Gold, goldBefore)
	}
	var ev []string
	for _, e := range buyer.Events {
		ev = append(ev, e.Text)
	}
	if joined := strings.Join(ev, "\n"); !strings.Contains(joined, "repriced") {
		t.Errorf("the buyer should be told WHY, got:\n%s", joined)
	}
}

// A shrunken listing sells what it has and refunds the rest, so the buyer is
// never charged for goods that did not exist.
func TestInterplanetaryBidFillsPartially(t *testing.T) {
	bw, buyer, sw, seller := twoTradingBoards(t, 100, 500)
	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 100, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	if err := sw.SetMarketListing(seller, "Tank", 30, 500); err != nil { // sold most of it locally
		t.Fatalf("shrink: %v", err)
	}

	bw.ApplyPacket(sw.ApplyPacket(bw.Outbox[len(bw.Outbox)-1]))
	if buyer.Tanks != 30 {
		t.Errorf("the buyer received %d tanks, want the 30 that were left", buyer.Tanks)
	}
	// Paid for 30, refunded for 70.
	if want := goldBefore - 30*500; buyer.Gold != want {
		t.Errorf("gold = %d, want %d — charged only for what arrived", buyer.Gold, want)
	}
}

// A bid whose answer never comes home is given up on and refunded, the same
// timer that recovers a lost strike's forces.
func TestInterplanetaryBidRefundedWhenThePacketIsLost(t *testing.T) {
	bw, buyer, _, _ := twoTradingBoards(t, 100, 500)
	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	if buyer.Gold == goldBefore {
		t.Fatal("the bid escrowed nothing, so the refund proves nothing")
	}

	bw.GameDay += bw.Config.LostForcesDays // nothing ever came back
	if n := bw.ReturnLostForces(); n != 1 {
		t.Fatalf("recovered %d, want the one abandoned bid", n)
	}
	if buyer.Gold != goldBefore {
		t.Errorf("gold = %d, want the full %d returned", buyer.Gold, goldBefore)
	}
	// ...and a late answer must not pay a second time.
	bw.applyTradeFill(IPTradeFill{ID: 1, Filled: true, Good: "Tank", Qty: 40})
	if buyer.Tanks != 0 {
		t.Errorf("a late answer handed over %d tanks after the refund", buyer.Tanks)
	}
}

// Bidding is a favour between allies: any other standing is refused, and the
// gold never leaves.
func TestInterplanetaryBidNeedsAnAlliedPlanet(t *testing.T) {
	for _, r := range []PlanetRelation{PlanetNone, PlanetPeace, PlanetEnemy} {
		bw, buyer, _, _ := twoTradingBoards(t, 100, 500)
		bw.SetPlanetRelationWith("Bravo", r)
		before := buyer.Gold
		if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 10, 500); err != ErrNotAllied {
			t.Errorf("standing %v: err = %v, want ErrNotAllied", r, err)
		}
		if buyer.Gold != before {
			t.Errorf("standing %v: gold moved on a refused bid", r)
		}
	}
}

// A packet that carries no trading must marshal exactly as an older board would
// write it, or the origin signature stops verifying across versions.
func TestTradingFieldsAreOmittedWhenEmpty(t *testing.T) {
	b, err := boardSigningBytes(Packet{FromBoard: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"TradeBids", "TradeFills", "Market"} {
		if strings.Contains(string(b), field) {
			t.Errorf("%s appears in a packet that carries no trading: %s", field, b)
		}
	}
}

// The league switch is real: with allied trading off, a bid is refused and the
// gold never moves.
func TestInterplanetaryBidRefusedWhenTheLeagueTurnsItOff(t *testing.T) {
	bw, buyer, _, _ := twoTradingBoards(t, 100, 500)
	bw.Config.IPTrading = false
	before := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 10, 500); err != ErrIPTradingOff {
		t.Errorf("err = %v, want ErrIPTradingOff", err)
	}
	if buyer.Gold != before {
		t.Error("gold moved on a refused bid")
	}
	if len(bw.InFlight) != 0 {
		t.Error("a refused bid was recorded in flight")
	}
}

// The standing is judged when the bid LANDS, not when it was sent. An alliance
// broken while the packet was in transit closes the market, and the gold goes
// home untouched — the same arrival-time rule an incoming strike's protection
// check follows.
func TestInterplanetaryBidRefusedWhenTheAllianceEndsInTransit(t *testing.T) {
	bw, buyer, sw, _ := twoTradingBoards(t, 100, 500)
	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	// Bravo falls out with Alpha while the bid is on its way.
	sw.SetPlanetRelationWith("Alpha", PlanetEnemy)

	reply := sw.ApplyPacket(bw.Outbox[len(bw.Outbox)-1])
	if left := sw.MarketForSale("Redlands", "Tank"); left != 100 {
		t.Errorf("the listing lost %d tanks to a bid that should have been refused", 100-left)
	}
	if sw.MarketProceeds["Redlands"] != 0 {
		t.Error("the seller was paid by a planet it is no longer allied with")
	}
	bw.ApplyPacket(reply)
	if buyer.Tanks != 0 {
		t.Errorf("the buyer received %d tanks from a closed market", buyer.Tanks)
	}
	if buyer.Gold != goldBefore {
		t.Errorf("gold = %d, want the full %d back", buyer.Gold, goldBefore)
	}
	var ev []string
	for _, e := range buyer.Events {
		ev = append(ev, e.Text)
	}
	if joined := strings.Join(ev, "\n"); !strings.Contains(joined, "alliance") {
		t.Errorf("the buyer should be told the alliance ended, got:\n%s", joined)
	}
}

// The far board's own switch is judged at arrival too: a league that turns
// trading off between the bid and its landing refuses it and refunds.
func TestInterplanetaryBidRefusedWhenTheFarBoardStopsTrading(t *testing.T) {
	bw, buyer, sw, _ := twoTradingBoards(t, 100, 500)
	goldBefore := buyer.Gold
	if _, err := bw.SendTradeBid(buyer, "Bravo", "Redlands", "Tank", 40, 500); err != nil {
		t.Fatalf("SendTradeBid: %v", err)
	}
	sw.Config.IPTrading = false

	bw.ApplyPacket(sw.ApplyPacket(bw.Outbox[len(bw.Outbox)-1]))
	if buyer.Tanks != 0 || buyer.Gold != goldBefore {
		t.Errorf("tanks = %d, gold = %d; want nothing bought and %d refunded",
			buyer.Tanks, buyer.Gold, goldBefore)
	}
}
