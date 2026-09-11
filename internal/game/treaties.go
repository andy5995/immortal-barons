package game

// treaties.go — the seven pacts two realms can hold, as ONE table (#207).
//
// The same shape as units.go and regions.go, and for a sharper reason than
// tidiness: `Treaty.Type` is persisted in world.json and compared with `==`
// everywhere, so a treaty named by a string literal that does not match is a
// silent no-match — the pact simply stops applying, with nothing to fail and no
// compiler to catch it. Three files were quoting these names by hand.
//
// The stored spelling is a save-file key and is never renamed. Translation
// happens at render time, as it does for goods.

// Pact is one relation type: the name it is stored and compared under, and the
// one-line explanation the Diplomacy menu shows before asking who to send it to.
// The description lives here rather than in the menu because it describes what
// the pact DOES, which is this package's business; the menu translates it.
type Pact struct {
	Name string
	Desc string
}

// The seven pacts. Each is a pointer so a screen can hold the row and compare
// identity, as the goods table does.
var (
	FullDefenseAlliance = &Pact{"Full Defense Alliance",
		"Neither realm may attack the other. If either is attacked, the ally sends 30% of its troopers and tanks to reinforce the defense."}
	TariffTradeAgreement = &Pact{"Tariff Trade Agreement",
		"Opens a taxed trade route. Both realms earn a modest income each turn, scaled to population."}
	FreeTradeAgreement = &Pact{"Free Trade Agreement",
		"Opens an open trade route. Both realms earn a larger income each turn — about double a tariff — scaled to population."}
	ProtectiveTrade = &Pact{"Protective Trade",
		"Guards the trade route between the two realms: deals in transit between you survive covert bombing, whoever fires. Markets are not covered."}
	TerroristPrevention = &Pact{"Terrorist Prevention",
		"Pools covert agents for defense, making both realms harder to spy on and sabotage."}
	IntelligenceAlliance = &Pact{"Intelligence Alliance",
		"Shares intelligence — partner agents strengthen your covert operations, both attacking and defending."}
	TechnologyAgreement = &Pact{"Technology Agreement",
		"Shares technology — the partner with less advanced tech is pulled up toward the more advanced one."}
)

// Pacts is every pact. The ORDER is load-bearing where it is iterated — the AI
// walks it looking for a treaty to propose — so it stays as it was; a screen
// that wants the original's menu order declares that order itself, the way the
// goods table's callers do.
var Pacts = []*Pact{
	FullDefenseAlliance,
	TariffTradeAgreement,
	FreeTradeAgreement,
	ProtectiveTrade,
	TerroristPrevention,
	IntelligenceAlliance,
	TechnologyAgreement,
}

// PactNamed is the row stored under name, or nil for a name no pact carries —
// which is what a save written by a newer board, or an Enemy row, looks like.
func PactNamed(name string) *Pact {
	for _, p := range Pacts {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// TreatyTypes is every pact's stored name, for the callers that want the
// strings: the recap's highlighter, and the AI's search for one to propose.
var TreatyTypes = pactNames()

func pactNames() []string {
	out := make([]string, len(Pacts))
	for i, p := range Pacts {
		out[i] = p.Name
	}
	return out
}
