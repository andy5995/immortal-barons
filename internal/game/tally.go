package game

// Tally counts what the mechanics did over a run, for a simulation of a mod
// (internal/modsim). It is optional and never saved: World.Tally is nil in
// normal play, every site below does nothing then, and none of them draws from
// the RNG or changes a result.
type Tally struct {
	// Battles are same-planet attacks fought; Wins those the attacker won, and
	// RegionsTaken the regions a win captured, a mod's capture included.
	Battles, Wins, RegionsTaken int64
	// FromDefender and FromAttacker are the military units a mod's capture took
	// from each side of a battle, in MilitaryGoods order.
	FromDefender, FromAttacker [numCaptureGoods]int64
	// Killed is what a battle's mod effects destroyed, in AttackKillGoods order.
	Killed [numKillGoods]int64
	// Raids and RaidTook are the pirate raids by spoil, and what each took.
	Raids    map[PirateSpoil]int
	RaidTook map[PirateSpoil]int64
	// Scripts: how often the game asked a mod's script and how often it failed
	// and the game used the built-in rules instead. Keyed "pirates" or a
	// region type's ID.
	ScriptCalls, ScriptFallbacks map[string]int
}

// NewTally is an empty tally.
func NewTally() *Tally {
	return &Tally{
		Raids: map[PirateSpoil]int{}, RaidTook: map[PirateSpoil]int64{},
		ScriptCalls: map[string]int{}, ScriptFallbacks: map[string]int{},
	}
}

func (t *Tally) script(name string, err error) {
	if t == nil {
		return
	}
	t.ScriptCalls[name]++
	if err != nil {
		t.ScriptFallbacks[name]++
	}
}

func (t *Tally) battle(won bool, regions int) {
	if t == nil {
		return
	}
	t.Battles++
	if won {
		t.Wins++
		t.RegionsTaken += int64(regions)
	}
}

// effects records one conventional set's result: what it took from the
// defender (target) and from the attacker, and what it destroyed.
func (t *Tally) effects(target, attacker AttackLoss) {
	if t == nil {
		return
	}
	for i := range target.Taken {
		t.FromDefender[i] += int64(target.Taken[i])
		t.FromAttacker[i] += int64(attacker.Taken[i])
	}
	for i := range t.Killed {
		t.Killed[i] += int64(target.Units[i]) + int64(attacker.Units[i])
	}
}

func (t *Tally) raid(s PirateSpoil, took int64) {
	if t == nil {
		return
	}
	t.Raids[s]++
	t.RaidTook[s] += took
}
