package game

import "testing"

// The prize ladder, as golden literals rather than the table they come from —
// these are the fidelity contract, and asserting the constant against itself
// would let a retune pass unnoticed. A six-letter ticket pays ten million; the
// hundred million players talk about is not in the original.
func TestLotteryPrizeLadder(t *testing.T) {
	want := []int64{0, 2_500, 10_000, 500_000, 1_000_000, 4_000_000, 10_000_000}
	for n, w := range want {
		if got := LotteryPrize(n); got != w {
			t.Errorf("%d matches paid %d, want %d", n, got, w)
		}
	}
	if got := LotteryPrize(7); got != 0 {
		t.Errorf("seven matches paid %d, want 0", got)
	}
}

// One match is still a loss: the ticket costs twice what it pays back.
func TestOneMatchLosesGold(t *testing.T) {
	if LotteryPrize(1) >= LotteryTicketPrice {
		t.Errorf("a single match pays %d against a %d ticket — it must not profit",
			LotteryPrize(1), LotteryTicketPrice)
	}
}

// The offer is withheld from a realm that cannot pay, and spent for the day once
// it has been made — the two rules together are why a baron can find themselves
// with no lottery after a big spend.
func TestLotteryOfferedOnlyWithTheGoldToPay(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")

	e.Gold = LotteryTicketPrice - 1
	if w.LotteryOffered(e) {
		t.Error("offered a ticket to a realm that cannot pay for it")
	}
	e.Gold = LotteryTicketPrice
	if !w.LotteryOffered(e) {
		t.Error("not offered a ticket with exactly the price in hand")
	}
	e.LotteryTaken = true
	if w.LotteryOffered(e) {
		t.Error("offered a second ticket the same day")
	}
	e.LotteryTaken = false
	w.Config.Lottery = false
	if w.LotteryOffered(e) {
		t.Error("offered a ticket on a board that has the lottery switched off")
	}
}

func TestBuyingATicketChargesTheHiddenPrice(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")

	e.Gold = 12_000
	if !w.BuyLotteryTicket(e) {
		t.Fatal("could not buy with 12,000 gold in hand")
	}
	if e.Gold != 7_000 {
		t.Errorf("gold after buying = %d, want 7,000", e.Gold)
	}
	e.Gold = 4_999
	if w.BuyLotteryTicket(e) {
		t.Error("bought a ticket without the gold for it")
	}
	if e.Gold != 4_999 {
		t.Errorf("a refused purchase charged %d gold", 4_999-e.Gold)
	}
}

// The match rule is set intersection with multiplicity: a drawn letter matches
// any unused letter on the ticket wherever it sits, and consumes it, so one 'A'
// on the ticket cannot score twice. Position has nothing to do with it.
func TestLotteryMatchesByLetterNotPosition(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	cases := []struct {
		ticket, draw string
		want         int
	}{
		{"ABCDEF", "FEDCBA", 6}, // every letter present, none in place
		{"ABCDEF", "AAAAAA", 1}, // one 'A' on the ticket scores once
		{"AABBCC", "ABCABC", 6}, // duplicates on both sides pair up
		{"AABBCC", "AAADDD", 2}, // two 'A's held, three drawn
		{"ABCDEF", "OZDQEF", 3}, // the captured draw: D, E and F
		{"QRSTUV", "ABCDEF", 0}, // nothing in common
	}
	for _, c := range cases {
		got := countMatches(w, c.ticket, c.draw)
		if got != c.want {
			t.Errorf("ticket %s against draw %s matched %d, want %d", c.ticket, c.draw, got, c.want)
		}
	}
}

// countMatches scores a fixed draw against a ticket with the same consuming scan
// DrawLottery uses, so the rule is asserted without fighting the RNG.
func countMatches(w *World, ticket, draw string) int {
	left := []byte(ticket)
	n := 0
	for i := range draw {
		for j := range left {
			if left[j] == draw[i] {
				left[j] = ' '
				n++
				break
			}
		}
	}
	return n
}

// The draw itself: six letters, all of them A-Z, and its own scoring agrees with
// the scan above.
func TestDrawLotteryShape(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 7)
	for range 200 {
		draw, hit, matches := w.DrawLottery([]byte("ABCDEF"))
		if len(draw) != LotteryLetters || len(hit) != LotteryLetters {
			t.Fatalf("drew %d letters and %d flags, want %d of each", len(draw), len(hit), LotteryLetters)
		}
		n := 0
		for i, c := range draw {
			if c < 'A' || c > 'Z' {
				t.Fatalf("drew %q, which is not a letter", c)
			}
			if hit[i] {
				n++
			}
		}
		if n != matches || matches != countMatches(w, "ABCDEF", string(draw)) {
			t.Fatalf("draw %s scored %d, flags say %d", draw, matches, n)
		}
	}
}

// Winnings go to the bank. Where they would carry it past the money cap, IB
// banks what fits and pays the rest into gold in hand rather than discarding the
// whole prize as the original does.
func TestLotteryPrizeIsBanked(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("t", "T")

	e.Bank = 0
	e.Gold = 0
	if got := w.PayLotteryPrize(e, 3); got != 500_000 {
		t.Errorf("paid %d for three matches, want 500,000", got)
	}
	if e.Bank != 500_000 || e.Gold != 0 {
		t.Errorf("bank %d gold %d, want the whole prize banked", e.Bank, e.Gold)
	}
	if got := w.PayLotteryPrize(e, 0); got != 0 || e.Bank != 500_000 {
		t.Errorf("a losing ticket paid %d", got)
	}

	e.Bank = w.MoneyCap() - 1000
	e.Gold = 0
	if got := w.PayLotteryPrize(e, 2); got != 10_000 {
		t.Errorf("paid %d, want the full 10,000 reported", got)
	}
	if e.Bank != w.MoneyCap() {
		t.Errorf("bank = %d, want it held at the cap", e.Bank)
	}
	if e.Gold != 9_000 {
		t.Errorf("gold = %d, want the 9,000 that did not fit in the bank", e.Gold)
	}
}
