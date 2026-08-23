package menu

import (
	"strings"
	"testing"
)

// The Request basket must never quote a ceiling. BRE bounds its "Demand how
// many <item>?" prompt at 0xFFFFFFFF and prints no hint at all, because what the
// other realm holds is not the asker's to see; IB printed that sentinel as
// "(0; 1.0737B)", which reads like a fact about the target and is not. The Offer
// side keeps its bound — that one IS the player's own count, and the Owned
// column beside it says the same thing.
func TestTradeRequestQuotesNoCeiling(t *testing.T) {
	render := func(limitToOwned bool) string {
		w := newWorld()
		f := &fakeSession{keys: []rune("2100\r0")}
		buildTradeBasket(f, w, "Basket:", limitToOwned)
		return stripANSI(f.out.String())
	}
	request := render(false)
	if strings.Contains(request, "1.0737B") || strings.Contains(request, "; ") {
		t.Errorf("the Request prompt quotes a ceiling:\n%s", request)
	}
	if strings.Contains(request, "Owned") {
		t.Errorf("the Request basket shows an Owned column:\n%s", request)
	}
	offer := render(true)
	if !strings.Contains(offer, "Owned") {
		t.Errorf("the Offer basket lost its Owned column:\n%s", offer)
	}
}
