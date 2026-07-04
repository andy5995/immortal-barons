package game

import "fmt"

// covertSuccess reports whether a covert op by `a` against `d` succeeds:
// the more agents the attacker has relative to the defender, the likelier.
func (w *World) covertSuccess(a, d *Empire) bool {
	total := a.Agents + d.Agents
	if total == 0 {
		return false
	}
	return w.rng.Intn(total) < a.Agents
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

// CauseDissensions lowers d's popular support, weakening its economy and its
// troopers. On failure the agent is lost.
func (w *World) CauseDissensions(a, d *Empire) (string, error) {
	if a.Agents < 1 {
		return "", ErrNoAgents
	}
	if w.covertSuccess(a, d) {
		d.adjustSupport(-15)
		d.Events = append(d.Events, "Agitators stirred unrest — your popular support fell.")
		return fmt.Sprintf("You stirred dissent in %s, lowering its popular support.", d.Name), nil
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
