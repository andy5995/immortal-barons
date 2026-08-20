package game

// The Queen's lottery: a ticket of six letters against six drawn letters, one
// draw per realm per game day. It rides the same first-play-of-the-day event
// block as the Queen's tax refund and is settled immediately after it.
//
// The numbers are in balance.go, all of them binary-verified.

// LotteryOffered reports whether e should be offered a ticket now. The price is
// checked here rather than at the prompt because BRE never offers a ticket the
// realm cannot pay for — a baron who spent down to 2,000 gold sees nothing at
// all, and gets no second offer later the same day.
func (w *World) LotteryOffered(e *Empire) bool {
	return w.Config.Lottery && !e.LotteryTaken && e.Gold >= LotteryTicketPrice
}

// BuyLotteryTicket takes the ticket price out of e's gold in hand. It reports
// false, and charges nothing, when the gold is not there.
func (w *World) BuyLotteryTicket(e *Empire) bool {
	if e.Gold < LotteryTicketPrice {
		return false
	}
	e.Gold -= LotteryTicketPrice
	return true
}

// RandomLotteryLetter returns a uniform letter from 'A' to 'Z'. It is both the
// draw and the ticket's own default: BRE fills an empty ticket slot with a
// random letter, so a player who holds Enter still gets a playable ticket.
func (w *World) RandomLotteryLetter() byte {
	return byte('A' + w.rng.Intn(LotteryAlphabet))
}

// DrawLottery draws LotteryLetters letters and scores them against ticket,
// returning the letters drawn, whether each one matched, and the match count.
//
// The rule is set intersection with multiplicity, not position: a drawn letter
// matches any unused letter on the ticket, and consumes it, so a ticket holding
// one 'A' scores at most one 'A' however many are drawn. Verified from the
// original's own scan, which blanks the ticket slot it matched.
func (w *World) DrawLottery(ticket []byte) (draw []byte, hit []bool, matches int) {
	left := make([]byte, len(ticket))
	copy(left, ticket)
	draw = make([]byte, LotteryLetters)
	hit = make([]bool, LotteryLetters)
	for i := range draw {
		draw[i] = w.RandomLotteryLetter()
		for j, c := range left {
			if c == draw[i] {
				left[j] = ' '
				hit[i] = true
				matches++
				break
			}
		}
	}
	return draw, hit, matches
}

// LotteryPrize is the payout for n matched letters.
func LotteryPrize(n int) int64 {
	if n < 0 || n >= len(LotteryPrizes) {
		return 0
	}
	return LotteryPrizes[n]
}

// PayLotteryPrize banks the winnings for n matches and returns what was paid.
// The prize goes to the bank, as the original announces it does.
//
// DIVERGENCE: where the prize would carry the bank past the money cap, BRE pays
// nothing at all — the whole win is discarded on the ceiling. IB banks what fits
// and pays the rest into gold in hand through creditGold, which holds it at the
// cap and files an event naming what was lost. The bank's own interest already
// spills the same way; silently deleting a win the player just watched being
// announced is the behaviour IB does not copy.
func (w *World) PayLotteryPrize(e *Empire, n int) int64 {
	prize := LotteryPrize(n)
	if prize <= 0 {
		return 0
	}
	banked := prize
	if room := w.MoneyCap() - e.Bank; banked > room {
		banked = max(room, 0)
	}
	e.Bank += banked
	if over := prize - banked; over > 0 {
		w.creditGold(e, over, "a lottery win")
	}
	return prize
}
