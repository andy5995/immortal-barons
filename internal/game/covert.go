package game

import (
	"errors"
	"fmt"
	"strings"
)

// covertSuccess reports whether a covert op by `a` against `d` succeeds:
// the more agents the attacker has relative to the defender, the likelier.
func (w *World) covertSuccess(a, d *Empire) bool {
	// Expose Enemy Ops shields d from every incoming covert op for a few
	// game-days, regardless of who the attacker is.
	if d.ShieldedUntilDay > 0 && w.GameDay <= d.ShieldedUntilDay {
		return false
	}
	// If d has bribed one of a's agents (Bribery), a's ops against d fail.
	for _, name := range d.ImmuneFrom {
		if name == a.Name {
			return false
		}
	}
	// Intelligence Alliance lends half an ally's agents to the attacker's
	// covert strength; Terrorist Prevention lends half to the defender's.
	aAgents := a.Agents + w.allyAgents(a, "Intelligence Alliance")/2
	dAgents := d.Agents + w.allyAgents(d, "Terrorist Prevention")/2
	total := aAgents + dAgents
	if total == 0 {
		return false
	}
	return w.rng.Intn(total) < aAgents
}

// ErrCovertCapReached is returned when an EFFECT covert op is attempted after
// one already ran this turn (BRE: "Limit one try per turn!"). Info ops (Send
// Spy, Spy on Relations) are exempt.
var ErrCovertCapReached = errors.New("You may run only one covert operation per turn.")

// covertCost gates a covert op: the attacker must hold at least one agent and
// enough gold for the op's fee, which is charged up front (BRE charges per op).
// When capped, it also enforces BRE's one-effect-op-per-turn limit and marks the
// op as used. The agent check comes first so a broke-but-agentless caller still
// sees ErrNoAgents. No state changes when it returns an error.
func (w *World) covertCost(a *Empire, cost int, capped bool) error {
	if a.Agents < 1 {
		return ErrNoAgents
	}
	if capped && a.TurnProgress.CovertOpUsed {
		return ErrCovertCapReached
	}
	if a.Gold < cost {
		return ErrCantAfford
	}
	a.Gold -= cost
	if capped {
		a.TurnProgress.CovertOpUsed = true
	}
	return nil
}

// SendSpy gathers military intel on d. Needs at least one agent. On failure
// the agent is caught (lost) and the victim is alerted.
func (w *World) SendSpy(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostSendSpy, false); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		return fmt.Sprintf("Intel on %s — Land %d, Troops %d, Turrets %d, Tanks %d, Offense %d, Defense %d, Gold %d, Agents %d",
			d.Name, d.Land, d.Troopers, d.Turrets, d.Tanks, d.Offense(), d.Defense(), d.Gold, d.Agents), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your counter-intelligence caught an enemy spy.")
	return "Your spy was caught and did not return.", nil
}

// SupportDissensions agitates d's own troopers into fleeing — eliminating
// ~10% of them on success; on failure the agent is lost. Covert ops are
// secret, so the victim event does not name the attacker.
func (w *World) SupportDissensions(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostSupportDissensions, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		lost := d.Troopers / 10
		d.Troopers -= lost
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs struck your army — %d troopers lost.", lost))
		return fmt.Sprintf("Your agents sowed dissension in %s: %d troopers eliminated.", d.Name, lost), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy sabotage attempt.")
	return "The operation failed and your agent was lost.", nil
}

// DemoralizeForces lowers d's military morale on success, weakening combat
// and risking desertion (see moraleFactor and MoraleDesertThreshold). On
// failure the agent is lost.
func (w *World) DemoralizeForces(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostDemoralizeForces, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		d.adjustMorale(-15)
		d.Events = append(d.Events, "Agents demoralized your forces — morale fell.")
		return fmt.Sprintf("You demoralized %s's forces, lowering their morale.", d.Name), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy attempt to demoralize your forces.")
	return "The operation failed and your agent was lost.", nil
}

// SetUp tricks d and one of its Full Defense Alliance partners into believing
// the other declared war, voiding the alliance between them — useful against
// a defense pact protecting a target you want to attack. On failure the
// agent is lost.
func (w *World) SetUp(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostSetUp, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		if allies := w.alliesOf(d, fullDefenseAlliance); len(allies) > 0 {
			partner := allies[0]
			w.BreakTreaty(d, partner, fullDefenseAlliance)
			d.Events = append(d.Events, fmt.Sprintf("Agents tricked you and %s into believing you had declared war — your alliance is void.", partner.Name))
			return fmt.Sprintf("You tricked %s and %s into voiding their Full Defense Alliance.", d.Name, partner.Name), nil
		}
		return fmt.Sprintf("%s holds no alliance for us to unravel.", d.Name), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy attempt to set us up.")
	return "The operation failed and your agent was lost.", nil
}

// ExposeOpsShieldDays is how many game-days Expose Enemy Ops shields the
// caller from incoming covert operations (BRE.OVR: "Bribed Agent will expose
// enemy operations for 24 Hours" — reinterpreted as game-days here, since IB
// tracks whole days rather than hours).
const ExposeOpsShieldDays = 1

// ExposeEnemyOps spends an agent to shield a from every incoming covert
// operation for ExposeOpsShieldDays game-days. Unlike the rest of Covert
// Operations this is a defensive action on the caller, not an attack on d;
// d is unused but kept so the action fits the same target-picker flow as
// every other item on this menu.
func (w *World) ExposeEnemyOps(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostExposeEnemyOps, true); err != nil {
		return "", err
	}
	a.ShieldedUntilDay = w.GameDay + ExposeOpsShieldDays
	return "Your agents expose enemy covert operations — you are shielded from incoming ops for the next day.", nil
}

// SpyOnRelations reveals every treaty d holds with other empires — useful
// pre-war intelligence on alliance networks and trade partners. On failure the
// agent is lost.
func (w *World) SpyOnRelations(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostSpyOnRelations, false); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		var lines []string
		for _, other := range w.Empires {
			if other == d || !other.Alive {
				continue
			}
			for _, tt := range w.TreatiesBetween(d, other) {
				lines = append(lines, fmt.Sprintf("  %s with %s: %s", d.Name, other.Name, tt))
			}
		}
		if len(lines) == 0 {
			return fmt.Sprintf("%s holds no treaties.", d.Name), nil
		}
		return fmt.Sprintf("Treaties of %s:\n%s", d.Name, strings.Join(lines, "\n")), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your counter-intelligence caught a spy probing your relations.")
	return "Your spy was caught and did not return.", nil
}

// Bribery buys off an agent inside d, so that from now on d's covert
// operations against you fail. On failure your own agent is lost.
func (w *World) Bribery(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostBribery, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		for _, n := range a.ImmuneFrom {
			if n == d.Name {
				return fmt.Sprintf("You already hold a bribed agent inside %s.", d.Name), nil
			}
		}
		a.ImmuneFrom = append(a.ImmuneFrom, d.Name)
		d.Events = append(d.Events, "A rival power bribed one of your agents.")
		return fmt.Sprintf("You bribed an agent in %s. Their covert ops against you will now fail.", d.Name), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled a bribery attempt.")
	return "The operation failed and your agent was lost.", nil
}

// StirRevolts spreads propaganda that lowers d's popular support (rioting and
// revolt), weakening its economy and its troopers. On failure the agent is
// lost.
func (w *World) StirRevolts(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostStirRevolts, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		d.adjustSupport(-15)
		d.Events = append(d.Events, "Agitators stirred revolts — your popular support fell.")
		return fmt.Sprintf("You stirred revolts in %s, lowering its popular support.", d.Name), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy agitation attempt.")
	return "The operation failed and your agent was lost.", nil
}

// BombFood destroys much of d's food reserve, which can trigger a death
// spiral. On failure the agent is lost.
func (w *World) BombFood(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostBombEnemyTargets, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		lost := d.Food / 2
		d.Food -= lost
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs torched your food stores — %d units lost.", lost))
		return fmt.Sprintf("You destroyed %d units of %s's food.", lost, d.Name), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy raid on your food stores.")
	return "The operation failed and your agent was lost.", nil
}

// BombingBombersRequired is the Bombers an empire must hold to run any Bomb
// Enemy Targets submenu op (BRE.OVR: "All missiles and bombs require 500
// Bombers to deliver their payloads").
const BombingBombersRequired = 500

// BombTradingMarket destroys a share of d's goods listed on the general market
// and its pending sale proceeds (#17; BRE: "destroys a portion of all goods
// stored in an opposing planet's trading market"). If d has nothing on the
// market, it falls back to a small gold raid so the op still bites. On failure
// the agent is lost.
func (w *World) BombTradingMarket(a, d *Empire) (string, error) {
	// A Protective Trade agreement guards the two realms' trade, so a partner
	// cannot bomb the other's market (#11; BRE: "preventing bombing of trade deals").
	// Checked before the fee so a guarded strike costs nothing.
	if w.HasTreaty(a, d, "Protective Trade") {
		return fmt.Sprintf("%s's trade is guarded by your Protective Trade agreement — the strike cannot proceed.", d.Name), nil
	}
	if err := w.covertCost(a, CostBombEnemyTargets, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		goods, proceeds := w.bombMarketPosition(d, BombMarketLossPct)
		if goods == 0 && proceeds == 0 {
			lost := d.Gold / 4
			d.Gold -= lost
			d.Events = append(d.Events, fmt.Sprintf("Saboteurs disrupted your trading market — %d gold lost.", lost))
			return fmt.Sprintf("You disrupted %s's trading market: %d gold lost.", d.Name, lost), nil
		}
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs wrecked your trading market — %d listed goods and %d gold in proceeds destroyed.", goods, proceeds))
		return fmt.Sprintf("You wrecked %s's trading market: %d listed goods and %d gold destroyed.", d.Name, goods, proceeds), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy strike on your trading market.")
	return "The operation failed and your agent was lost.", nil
}

// tradeTreatyTypes are the treaty types BombTradeRoutes and SetUp look for
// when severing d's standing agreements.
var tradeTreatyTypes = []string{"Tariff Trade Agreement", "Free Trade Agreement", "Protective Trade"}

// BombTradeRoutes severs one of d's standing trade treaties. On failure the
// agent is lost.
func (w *World) BombTradeRoutes(a, d *Empire) (string, error) {
	// A Protective Trade agreement guards the trade routes between the two realms,
	// so a partner cannot bomb them (#11; BRE: "preventing bombing of trade deals").
	// Checked before the fee so a guarded strike costs nothing.
	if w.HasTreaty(a, d, "Protective Trade") {
		return fmt.Sprintf("%s's trade routes are guarded by your Protective Trade agreement — the strike cannot proceed.", d.Name), nil
	}
	if err := w.covertCost(a, CostBombEnemyTargets, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		for _, ttype := range tradeTreatyTypes {
			if allies := w.alliesOf(d, ttype); len(allies) > 0 {
				partner := allies[0]
				w.BreakTreaty(d, partner, ttype)
				d.Events = append(d.Events, fmt.Sprintf("Saboteurs severed your %s with %s.", ttype, partner.Name))
				return fmt.Sprintf("You severed %s's %s with %s.", d.Name, ttype, partner.Name), nil
			}
		}
		return fmt.Sprintf("%s has no trade routes to sever.", d.Name), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy attempt to sever your trade routes.")
	return "The operation failed and your agent was lost.", nil
}

// UndermineInvestments trims a quarter off the principal (and matching
// return) of each of d's pending bank investments. On failure the agent is
// lost.
func (w *World) UndermineInvestments(a, d *Empire) (string, error) {
	if err := w.covertCost(a, CostBombEnemyTargets, true); err != nil {
		return "", err
	}
	if w.covertSuccess(a, d) {
		if len(d.Investments) == 0 {
			return fmt.Sprintf("%s has no investments to undermine.", d.Name), nil
		}
		var lost int
		for i := range d.Investments {
			cut := d.Investments[i].Amount / 4
			d.Investments[i].Amount -= cut
			d.Investments[i].Return -= cut
			lost += cut
		}
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs undermined your investments — %d gold in principal lost.", lost))
		return fmt.Sprintf("You undermined %s's investments: %d gold lost.", d.Name, lost), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy attempt to undermine your investments.")
	return "The operation failed and your agent was lost.", nil
}

// R5-Slappenheimer tuning. In BRE's S3-Sabre only 3 of the 11 dial settings
// (1, 2, 3) did anything and the rest fizzled; the manual never said which
// number did what. IB keeps the unpredictability but makes it honest: the dial
// (0-10) is a bluff that changes nothing — every launch is the same random
// gamble. Only about SlappenheimerEffectHits launches in SlappenheimerEffectRange
// (3 in 10) deliver a payload; the rest fizzle. The sysop's None handling mode
// disables the weapon entirely (gated in the menu).
const (
	SlappenheimerEffectHits    = 3   // landing launches per SlappenheimerEffectRange...
	SlappenheimerEffectRange   = 10  // ...i.e. a 3-in-10 chance to deliver a payload
	SlappenheimerBaseDamagePct = 5   // a landed hit always removes at least this %
	SlappenheimerDamageSpread  = 26  // random % headroom on top of the base (5-30% total)
	SlappenheimerBackfireScale = 200 // target Troopers / this = backfire chance (percent)
	SlappenheimerMultiHitOdds  = 10  // 1-in-this a hit strafes several assets at once
)

// slappenheimerResource pairs a strikeable field with its display name. BRE hid
// which asset each effect hit, so IB picks its own spread of targets.
type slappenheimerResource struct {
	name string
	val  *int
}

func slappenheimerResources(e *Empire) []slappenheimerResource {
	return []slappenheimerResource{
		{"Troopers", &e.Troopers},
		{"Jets", &e.Jets},
		{"Turrets", &e.Turrets},
		{"Tanks", &e.Tanks},
		{"Bombers", &e.Bombers},
		{"Carriers", &e.Carriers},
		{"Agents", &e.Agents},
		{"Gold", &e.Gold},
		{"Food", &e.Food},
	}
}

// slappenheimerDamage applies a landed R5-Slappenheimer hit to e and returns a
// human-readable list of what was destroyed (empty if the roll removed
// nothing). Each hit removes a random 5-30% of one asset; usually a single
// asset, but occasionally the missile strafes several at once — BRE's
// "extremely devastating" outcome. Land is one of the targets, but must be
// removed through the RegionMix (whose Total must always equal e.Land) rather
// than by touching e.Land directly.
func (w *World) slappenheimerDamage(e *Empire) string {
	res := slappenheimerResources(e)
	landIdx := len(res) // one past the plain-int assets: Land
	hits := 1
	if w.rng.Intn(SlappenheimerMultiHitOdds) == 0 {
		hits = 2 + w.rng.Intn(3) // 2-4 assets at once
	}
	seen := make(map[int]bool, hits)
	var parts []string
	for k := 0; k < hits; k++ {
		i := w.rng.Intn(landIdx + 1)
		if seen[i] {
			continue
		}
		seen[i] = true
		pct := SlappenheimerBaseDamagePct + w.rng.Intn(SlappenheimerDamageSpread)
		if i == landIdx {
			lost := e.Land * pct / 100
			if lost <= 0 {
				continue
			}
			e.Regions.remove(lost)
			e.syncLand()
			parts = append(parts, fmt.Sprintf("%d Regions", lost))
			continue
		}
		lost := *res[i].val * pct / 100
		if lost <= 0 {
			continue
		}
		*res[i].val -= lost
		parts = append(parts, fmt.Sprintf("%d %s", lost, res[i].name))
	}
	return strings.Join(parts, ", ")
}

// SlappenheimerStrike fires an R5-Slappenheimer at d. The missile can be shot
// down by the target's SDI, fizzle harmlessly, or land a random payload on a
// spread of the target's assets; a heavily-garrisoned target can turn it back
// on the attacker. The dial the player sets under User Select handling is a
// bluff and has no effect here, so it is not passed in. The agent is only lost
// when the covert approach itself is foiled — not on an SDI shoot-down, a
// fizzle, or a backfire.
func (w *World) SlappenheimerStrike(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if !w.covertSuccess(a, d) {
		a.Agents--
		d.Events = append(d.Events, "Your security foiled an enemy R5-Slappenheimer strike.")
		return "The operation failed and your agent was lost.", nil
	}
	if w.rng.Intn(100) < d.SDI {
		return fmt.Sprintf("%s's SDI intercepted your R5-Slappenheimer.", d.Name), nil
	}
	if w.rng.Intn(SlappenheimerEffectRange) >= SlappenheimerEffectHits {
		return "The R5-Slappenheimer fizzled and did no damage.", nil
	}
	// The more Troopers the target garrisons, the likelier the missile turns
	// back on the attacker.
	if w.rng.Intn(100) < d.Troopers/SlappenheimerBackfireScale {
		if hit := w.slappenheimerDamage(a); hit != "" {
			return "The R5-Slappenheimer backfired! You lost " + hit + ".", nil
		}
		return "The R5-Slappenheimer backfired, but the damage was negligible.", nil
	}
	hit := w.slappenheimerDamage(d)
	if hit == "" {
		return fmt.Sprintf("Your R5-Slappenheimer reached %s but did negligible damage.", d.Name), nil
	}
	d.Events = append(d.Events, "An R5-Slappenheimer struck your empire — lost "+hit+".")
	return fmt.Sprintf("Your R5-Slappenheimer hit %s: %s destroyed.", d.Name, hit), nil
}
