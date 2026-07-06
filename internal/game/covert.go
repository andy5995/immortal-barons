package game

import (
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

// SendSpy gathers military intel on d. Needs at least one agent. On failure
// the agent is caught (lost) and the victim is alerted.
func (w *World) SendSpy(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	a.ShieldedUntilDay = w.GameDay + ExposeOpsShieldDays
	return "Your agents expose enemy covert operations — you are shielded from incoming ops for the next day.", nil
}

// SpyOnRelations reveals every treaty d holds with other empires — useful
// pre-war intelligence on alliance networks and trade partners. On failure the
// agent is lost.
func (w *World) SpyOnRelations(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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

// BombTradingMarket raids d's gold reserves via its own trading market. On
// failure the agent is lost.
func (w *World) BombTradingMarket(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		lost := d.Gold / 4
		d.Gold -= lost
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs disrupted your trading market — %d gold lost.", lost))
		return fmt.Sprintf("You disrupted %s's trading market: %d gold lost.", d.Name, lost), nil
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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
	if a.Agents < 1 {
		return "", ErrNoAgents
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

// SabreBackfireTroopers is the Trooper count on the target above which an
// R5-Slappenheimer missile risks backfiring (BRE.OVR: "large quantities of Troopers
// on the target empire are known to cause the missile to backfire").
const SabreBackfireTroopers = 5000

// sabreDial picks the R5-Slappenheimer's random-return magnitude (BRE's "dial",
// 0-10) according to the sysop's Sabre Handling mode. This is a v1
// simplification of BRE's dial: the original's per-value effect table was
// never documented ("the instruction manual did not tell which number
// corresponds to which task"), so we scale a single Trooper-loss effect by
// the dial instead of reproducing an unknown effect table.
func (w *World) sabreDial() int {
	switch w.Config.SabreHandling {
	case SabreConstant:
		return 5
	case SabreNone:
		return 0
	default: // SabreRandom, SabreUserSelect
		return w.rng.Intn(11)
	}
}

// SabreStrike fires an R5-Slappenheimer missile at d, a variable-return weapon whose
// magnitude depends on sabreDial. A heavily-garrisoned target has a chance
// to backfire the missile onto the attacker instead. On failure (the covert
// roll, not the backfire) the agent is lost.
func (w *World) SabreStrike(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		dial := w.sabreDial()
		if d.Troopers > SabreBackfireTroopers && w.rng.Intn(2) == 0 {
			lost := a.Troopers / 10
			a.Troopers -= lost
			return fmt.Sprintf("The R5-Slappenheimer backfired! You lost %d troopers.", lost), nil
		}
		lost := d.Troopers * dial / 20
		d.Troopers -= lost
		d.Events = append(d.Events, fmt.Sprintf("An R5-Slappenheimer struck your forces — %d troopers lost.", lost))
		return fmt.Sprintf("Your R5-Slappenheimer hit %s: %d troopers eliminated (dial %d).", d.Name, lost, dial), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy R5-Slappenheimer strike.")
	return "The operation failed and your agent was lost.", nil
}
