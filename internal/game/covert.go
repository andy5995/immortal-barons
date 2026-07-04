package game

import (
	"fmt"
	"strings"
)

// covertSuccess reports whether a covert op by `a` against `d` succeeds:
// the more agents the attacker has relative to the defender, the likelier.
func (w *World) covertSuccess(a, d *Empire) bool {
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

// Sabotage (Special Operations) eliminates ~10% of d's troopers on success;
// on failure the agent is lost. Covert ops are secret, so the victim event
// does not name the attacker.
func (w *World) Sabotage(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		lost := d.Troopers / 10
		d.Troopers -= lost
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs struck your army — %d troopers lost.", lost))
		return fmt.Sprintf("Your agents sabotaged %s: %d troopers eliminated.", d.Name, lost), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy sabotage attempt.")
	return "The operation failed and your agent was lost.", nil
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

// BombIntelligence kills a share of d's agents, softening the target for your
// later covert ops. Best used first. On failure the agent is lost.
func (w *World) BombIntelligence(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		lost := d.Agents / 4
		d.Agents -= lost
		d.Events = append(d.Events, fmt.Sprintf("Enemy operatives struck your intelligence — %d agents lost.", lost))
		return fmt.Sprintf("You crippled %s's intelligence: %d agents eliminated.", d.Name, lost), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy intelligence strike.")
	return "The operation failed and your agent was lost.", nil
}

// StirRevolts spreads propaganda that lowers d's popular support (rioting and
// revolt), weakening its economy and its troopers. The manual's name for this
// op; not to be confused with "Support Dissensions", which is the
// troopers-flee sabotage op (our Special Operations). On failure the agent is
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

// BombAirbases destroys a share of d's grounded jets. On failure the agent is
// lost.
func (w *World) BombAirbases(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		lost := d.Jets / 4
		d.Jets -= lost
		d.Events = append(d.Events, fmt.Sprintf("Saboteurs hit your airbases — %d jets destroyed.", lost))
		return fmt.Sprintf("You bombed %s's airbases: %d jets destroyed.", d.Name, lost), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy airbase attack.")
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

// BombHQ weakens d's HeadQuarters, reducing its tank effectiveness. On failure
// the agent is lost.
func (w *World) BombHQ(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		before := d.HQ
		d.HQ -= 20
		if d.HQ < 0 {
			d.HQ = 0
		}
		d.Events = append(d.Events, "Saboteurs damaged your HeadQuarters.")
		return fmt.Sprintf("You damaged %s's HeadQuarters (%d%% -> %d%%).", d.Name, before, d.HQ), nil
	}
	a.Agents--
	d.Events = append(d.Events, "Your security foiled an enemy strike on your HeadQuarters.")
	return "The operation failed and your agent was lost.", nil
}
