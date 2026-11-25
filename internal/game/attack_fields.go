package game

import (
	"fmt"
)

// Limits for an attack's effects. A percent is of a realm's holdings, so it
// stops at 100; a jitter is the bound of a Random(n) drawn against a percent;
// morale and support are kept as Num/Den, and Den is bounded so the product
// with a figure stays inside an int on the 32-bit door builds.
const (
	ModAttackPctMax    = 100
	ModAttackJitterMax = 100
	ModAttackHQMax     = 100
	ModAttackDenMax    = 1000
)

// The keys the attack table's Exact hooks are looked up by, spelled once.
const (
	keyPeople = "people"
	keyFood   = "food"
)

// killKey is the key of the kill effect on g.
func killKey(g *Good) string { return "kill_" + g.ID }

// AttackField is one key of an effect set as a mod names it: the key in a
// values.toml [attacks.<kind>.<side>] section, and how to reach its figure.
// Exactly one of Roll and Fraction is set. Validation and the mod reader both go
// through AttackFields, so a key is named and bounded in one place.
type AttackField struct {
	Key      string
	Roll     func(s *EffectSet) *Roll
	Fraction func(s *EffectSet) *Fraction
	// Points marks a Roll read as points, not percent (hq).
	Points bool
	// Desc is one sentence saying what the figure means; the generated mod
	// reference prints it.
	Desc string
}

// AttackFields are the keys of an effect set, in the order they are applied.
var AttackFields = func() []AttackField {
	f := []AttackField{
		{Key: "waste", Roll: func(s *EffectSet) *Roll { return &s.Waste },
			Desc: "Percent of the realm's regions turned to waste. The regions stay on its books but earn nothing until cleaned."},
		{Key: "destroy", Roll: func(s *EffectSet) *Roll { return &s.Destroy },
			Desc: "Percent of the realm's regions destroyed outright."},
		{Key: "capture", Roll: func(s *EffectSet) *Roll { return &s.Capture },
			Desc: "Percent of the realm's regions taken by the other side, as land it types at the start of its next turn."},
		{Key: keyPeople, Roll: func(s *EffectSet) *Roll { return &s.People },
			Desc: "Percent of the realm's people killed."},
	}
	for i, g := range AttackKillGoods {
		f = append(f, AttackField{Key: killKey(g), Roll: func(s *EffectSet) *Roll { return &s.Kill[i] },
			Desc: fmt.Sprintf("Percent of the realm's %s killed.", g.Plural)})
	}
	return append(f,
		AttackField{Key: keyFood, Roll: func(s *EffectSet) *Roll { return &s.Food },
			Desc: "Percent of the realm's food destroyed."},
		AttackField{Key: "morale", Fraction: func(s *EffectSet) *Fraction { return &s.Morale },
			Desc: "The part of the realm's morale that is kept, as `[num, den]`. `[1, 1]` leaves it alone."},
		AttackField{Key: "support", Fraction: func(s *EffectSet) *Fraction { return &s.Support },
			Desc: "The part of the realm's popular support that is kept, as `[num, den]`. `[1, 1]` leaves it alone."},
		AttackField{Key: "hq", Roll: func(s *EffectSet) *Roll { return &s.HQ }, Points: true,
			Desc: "HeadQuarters completion points lost, taken off the realm's percent complete and never below 0."},
	)
}()

// AttackSides are the two sets of an attack, by the name a mod's section uses.
var AttackSides = []string{"target", "attacker"}

// Side is the set named by one of AttackSides, or nil.
func (fx *AttackEffects) Side(name string) *EffectSet {
	switch name {
	case "target":
		return &fx.Target
	case "attacker":
		return &fx.Attacker
	}
	return nil
}

// AttackFieldByKey is the field named key, or nil.
func AttackFieldByKey(key string) *AttackField {
	for i := range AttackFields {
		if AttackFields[i].Key == key {
			return &AttackFields[i]
		}
	}
	return nil
}

// validateAttacks reports each attack figure outside its limits through bad.
func (r *Rules) validateAttacks(bad func(format string, a ...any)) {
	for _, kind := range EffectKinds() {
		fx, ok := r.Attacks[kind]
		if !ok {
			bad("attacks.%s: missing", kind)
			continue
		}
		row := attackKindOf(kind)
		if row.RemoteTarget {
			if fx.Attacker != noEffect() {
				bad("attacks.%s.attacker: this missile comes from another board, so it has no attacker effects", kind)
			}
			if !fx.Target.Capture.IsZero() {
				bad("attacks.%s.target.capture: the attacker is on another board and cannot receive land", kind)
			}
		}
		if row.AttackerAway && !fx.Attacker.Capture.IsZero() {
			bad("attacks.%s.attacker.capture: the target is on another board and cannot receive land", kind)
		}
		for _, side := range AttackSides {
			set := *fx.Side(side)
			for _, f := range AttackFields {
				where := fmt.Sprintf("attacks.%s.%s.%s", kind, side, f.Key)
				if f.Roll != nil {
					v := *f.Roll(&set)
					limit := ModAttackPctMax
					if f.Points {
						limit = ModAttackHQMax
					}
					switch {
					case v.Pct < 0 || v.Pct > limit:
						bad("%s: percent %d is outside 0 to %d", where, v.Pct, limit)
					case v.Up < 0 || v.Up > ModAttackJitterMax || v.Down < 0 || v.Down > ModAttackJitterMax:
						bad("%s: up %d and down %d must each be 0 to %d", where, v.Up, v.Down, ModAttackJitterMax)
					}
					continue
				}
				v := *f.Fraction(&set)
				switch {
				case v.Den < 1 || v.Den > ModAttackDenMax:
					bad("%s: denominator %d is outside 1 to %d", where, v.Den, ModAttackDenMax)
				case v.Num < 0 || v.Num > v.Den:
					bad("%s: numerator %d is outside 0 to the denominator (%d)", where, v.Num, v.Den)
				}
			}
		}
	}
}
