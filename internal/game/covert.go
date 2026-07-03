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
