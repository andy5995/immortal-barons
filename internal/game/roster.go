package game

import (
	"fmt"
	"strings"
	"time"
)

// roster.go — the planet's set of realms: seeding the AI barons, admitting a
// caller, and taking a realm off the board again.

// AI baron names are built from a modifier x noun matrix rather than a fixed
// list, so a game can field far more distinct barons and the lineup varies
// game to game. The two single-word pools combine as "<modifier> <noun>"
// (e.g. "Crimson Horde"); ~24 x ~24 gives hundreds of gritty combinations.
var (
	aiNameModifiers = []string{
		"Crimson", "Iron", "Ashen", "Obsidian", "Void", "Rust", "Cinder", "Grim",
		"Ember", "Dread", "Storm", "Dust", "Salt", "Blood", "Frost", "Bone",
		"Shadow", "Molten", "Scarlet", "Onyx", "Thorn", "Gale", "Coal", "Ashfall",
	}
	aiNameNouns = []string{
		"Horde", "Dominion", "Clan", "Reavers", "Kings", "Pact", "Legion",
		"Marauders", "Barons", "Host", "Vanguard", "Coven", "Coil", "Wardens",
		"Raiders", "Syndicate", "Covenant", "Brood", "Vultures", "Jackals",
		"Corsairs", "Warband", "Conclave", "Sovereigns",
	}
)

// newAIName returns a "<modifier> <noun>" name not present in used, or ok=false
// only when every combination is taken. It tries random combinations first (via
// w.nameRng, so it is deterministic per seed and varied game to game), then
// falls back to a deterministic scan so the "pool exhausted" answer is exact.
func (w *World) newAIName(used map[string]bool) (string, bool) {
	for tries := 0; tries < 200; tries++ {
		name := aiNameModifiers[w.nameRng.Intn(len(aiNameModifiers))] + " " + aiNameNouns[w.nameRng.Intn(len(aiNameNouns))]
		if !used[strings.ToLower(name)] {
			return name, true
		}
	}
	for _, m := range aiNameModifiers {
		for _, n := range aiNameNouns {
			name := m + " " + n
			if !used[strings.ToLower(name)] {
				return name, true
			}
		}
	}
	return "", false
}

// addAIEmpire appends one AI empire (Owner "") with the standard AI starting
// setup and returns it, or nil when the planet has no free slot. Shared by
// seedAIEmpires and AddAIEmpires.
func (w *World) addAIEmpire(name string) *Empire {
	slot := w.freeSlot()
	if slot == 0 {
		return nil
	}
	e := newEmpire(name, "", w.Config, w.GameDay)
	e.Slot = slot
	e.Jets = 5
	e.Turrets = 40
	// Spread personalities evenly across the AI pool (#36) so a game gets a mix
	// of diplomats, balanced realms, and aggressors rather than all alike.
	e.AIProfile = aiProfiles[len(w.AIEmpires())%len(aiProfiles)]
	// Economic skill is rolled randomly (not cycled), so the sharp/dull split
	// varies game to game rather than being a fixed pattern.
	if w.rng.Intn(2) == 0 {
		e.AISkill = AISkillDull
	} else {
		e.AISkill = AISkillSharp
	}
	w.Empires = append(w.Empires, e)
	return e
}

// seedAIEmpires appends Config.AICount AI empires to the world.
func (w *World) seedAIEmpires() {
	w.AddAIEmpires(w.Config.AICount)
}

// AddAIEmpires injects up to n new AI barons into the live world, generating
// matrix names not already used by any existing empire (dead or alive,
// case-insensitive). It returns the count actually added, which is less than n
// when the planet runs out of slots or every name combination is taken.
//
// Barons and callers share the planet's PlanetSlots realms, and barons are
// seeded at reset, so they take their slots first and Max Players Per BBS
// bounds the callers within what is left.
//
// A league board gets none, ever, and adds none later: an inter-BBS game is
// played between the boards' human realms, and a computer baron would take a
// share of the planet's standing in a league it cannot be held accountable in.
// The guard is here rather than only in the editor so no path — reset, seeding,
// or a later injection — can slip one in.
func (w *World) AddAIEmpires(n int) int {
	if w.Config.IBBS {
		return 0
	}
	used := make(map[string]bool, len(w.Empires))
	for _, e := range w.Empires {
		used[strings.ToLower(e.Name)] = true
	}
	added := 0
	for added < n {
		name, ok := w.newAIName(used)
		if !ok {
			break
		}
		if w.addAIEmpire(name) == nil {
			break // the planet is full
		}
		used[strings.ToLower(name)] = true
		added++
	}
	return added
}

func newEmpire(name, owner string, cfg Config, day int) *Empire {
	// BRE's starting realm (verified from a live BRE new-empire screen): 15
	// regions — 2 Agricultural, 5 Desert, 5 Mountain, 3 Coastal — with 100
	// troopers, 1000 food, full morale, and no other units.
	regions := RegionMix{Agricultural: StartAgricultural, Desert: StartDesert, Mountain: StartMountain, Coastal: StartCoastal}
	return &Empire{
		Name: name, Owner: owner, Alive: true,
		Gold: StartGold, Food: StartFood, Land: regions.Total(), People: StartPeople,
		Regions:  regions,
		Troopers: StartTroopers, Tax: StartTax, Support: 100, Morale: 100,
		TurnsLeft: cfg.TurnsPerDay, Protection: cfg.ProtectionTurns,
		CreatedDay: day,
		// A new realm opens with one day's land allowance already granted, plus
		// whatever the sysop seeded the market with, so it can expand on day one
		// before its first Daily Land Creation arrives.
		LandAvailable: cfg.InitialMarketLand + cfg.LandPerDay,
		// The full pool goes to units, split so carrier output matches jet lift
		// (see DefaultProdTroopersPct); BRE defaults to 90% units, 10% gold.
		ProdTroopers: DefaultProdTroopersPct, ProdJets: DefaultProdJetsPct, ProdTurrets: DefaultProdTurretsPct,
		ProdBombers: DefaultProdBombersPct, ProdTanks: DefaultProdTanksPct, ProdCarriers: DefaultProdCarriersPct,
		ProdGold:        DefaultProdGoldPct,
		ProdInitialized: true, // so a player's later all-zero setting isn't overwritten
		Prefs:           DefaultPrefs(),
	}
}

// HumanCount is the number of human (caller-owned) empires in the world.
func (w *World) HumanCount() int {
	n := 0
	for _, e := range w.Empires {
		if e.Owner != "" {
			n++
		}
	}
	return n
}

// BoardFull reports whether no new caller may enroll — either the planet has no
// free slot at all, or the sysop's Max Players Per BBS cap is reached.
//
// The slot check is the one that cannot be configured away. Max Players Per BBS
// counts HUMANS only, so it never saw the computer barons sharing the same
// roster, and 0 ("no cap of my own") left nothing bounding the set the pickers
// letter — either way a board could hold more realms than 25 and the surplus was
// addressable by nobody (#144). 0 now means "up to the planet's slots", which is
// the most it could ever have meant.
func (w *World) BoardFull() bool {
	if w.PlanetFull() {
		return true
	}
	return w.Config.MaxPlayers > 0 && w.HumanCount() >= w.Config.MaxPlayers
}

// AddHuman creates and registers a human empire keyed by handle, in the lowest
// free slot. It returns nil when the planet is full; callers check BoardFull
// under the same lock that does the insert, so this is the backstop rather than
// the refusal a player sees.
func (w *World) AddHuman(handle, realm string) *Empire {
	slot := w.freeSlot()
	if slot == 0 {
		return nil
	}
	e := newEmpire(realm, strings.ToLower(strings.TrimSpace(handle)), w.Config, w.GameDay)
	e.Slot = slot
	w.Empires = append(w.Empires, e)
	return e
}

// RealmNameTaken reports whether an empire already uses this realm name
// (case-insensitive). Call it under the world lock together with AddHuman so a
// concurrent onboarding cannot slip a duplicate name in between the check and
// the insert.
func (w *World) RealmNameTaken(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, e := range w.Empires {
		// A renamed realm holds its old name too: inbound packets still address it
		// (see RenameEmpire), so nobody else may take delivery of that name.
		if strings.ToLower(e.Name) == n {
			return true
		}
		for _, old := range e.PriorRealmNames() {
			if strings.ToLower(old) == n {
				return true
			}
		}
	}
	return false
}

// PriorRealmNames returns every name this realm used to answer to and still
// takes delivery on — the one a player's rename moved it off (FormerName) and
// any a sysop's did (PriorNames). Callers that resolve a name written before a
// rename, or that hold a name against re-use, walk this rather than either
// field, so neither kind of rename can be missed.
func (e *Empire) PriorRealmNames() []string {
	if e.FormerName == "" {
		return e.PriorNames
	}
	return append([]string{e.FormerName}, e.PriorNames...)
}

// dropEmpires removes every empire `gone` selects and forgets what the world
// still holds on its behalf. EVERY removal path goes through here, because two
// kinds of state outlive the record.
//
// w.Treaties rows and pending TreatyOffers are keyed by realm NAME: a name freed
// by one removal can be claimed by the next caller to onboard, and the new realm
// would then hold its predecessor's alliances and Enemy standing. The rows stay
// invisible in the meantime — alliesOf and TreatyPartners skip the non-living —
// so the leak only surfaces when the name comes back.
//
// The trading market is keyed by realm name too, and leaks the same way: the
// goods a dead realm had escrowed there, and the sale gold it had not been paid,
// would go to whoever claims the name next.
//
// The departing realm's SLOT is freed here too, and the next realm founded may
// take it — nothing waits for a reset. That is safe only because everything a
// realm leaves behind keys on its NAME, never on its slot: treaties, pending
// offers and market escrow are forgotten below, and mail and events name realms
// in prose. The one trace of a slot that outlives its holder is the letter a
// sent message recorded in its "To" field, which is a label written at send
// time and not a reference anything follows.
//
// BRE deletes an empire the same way, and more completely: its delete path
// (BRE.OVR 0x0079c1) clears the slot's messages, trade offers and reports by the
// player id, zeroes the pair's relation in BOTH directions for every other realm
// (0x050d74), and blanks the whole record before seeding a new realm into it.
func (w *World) dropEmpires(gone func(*Empire) bool) {
	kept := w.Empires[:0]
	var forget []*Empire
	for _, e := range w.Empires {
		if gone(e) {
			forget = append(forget, e)
			continue
		}
		kept = append(kept, e)
	}
	w.Empires = kept
	for _, e := range forget {
		w.forgetRelations(e.Name)
		w.forgetMarketPosition(e.Name)
		w.forfeitPendingDeals(e)
	}
}

// Kill marks e eliminated. The husk is kept rather than deleted so the
// next-day rebuild rule holds: removeDeadHusks collects it once GameDay has
// passed DiedDay, which is what stops an owner re-onboarding the same day.
//
// Every death goes through here — conquest, a Gooie Kablooie, starvation,
// and abdication — so whatever a future death has to do besides setting these
// two fields is added in one place. Three of the four call sites were in
// internal/game and the fourth in the abdication screen, which is how a rule
// ends up being enforced by a renderer.
func (w *World) Kill(e *Empire) {
	e.Alive = false
	e.DiedDay = w.GameDay
}

// WaiveProtection ends e's new-realm protection at the end of the current turn.
//
// It drops to a single turn rather than 0 on purpose: the realm stays protected
// for the rest of this turn — still unable to attack or be attacked — and the
// turn-end tick in PlayTurn clears it. Setting 0 here would let ending
// protection double as a free same-turn attack.
func (w *World) WaiveProtection(e *Empire) {
	if e.Protection > 1 {
		e.Protection = 1
	}
}

// RemoveEmpire deletes e from the world (Abdicate). The empire is gone
// entirely; the caller gets a fresh realm on their next visit.
func (w *World) RemoveEmpire(e *Empire) {
	w.dropEmpires(func(x *Empire) bool { return x == e })
}

// removeDeadHusks deletes eliminated empires whose death is in the past
// (GameDay > DiedDay), keeping husks that died today so the owner cannot
// re-onboard on the same day. AI barons are removed too; they never rebuild.
func (w *World) removeDeadHusks() {
	w.dropEmpires(func(e *Empire) bool { return !e.Alive && w.GameDay > e.DiedDay })
}

// removeUnplayedEmpires erases a human realm whose owner created it and then
// never took a single turn. It is removed outright rather than left as a husk,
// so the same caller can build a fresh realm normally the next time they show
// up — nothing is held against them.
//
// Governed by Config.IdleDaysRemove: 0 disables realm removal entirely.
//
// Only human realms are eligible: AI barons are driven by aiPlay and always have
// LastPlayed set, and an empty Owner is what marks a baron as AI.
//
// The timing works because onboarding runs AFTER maintenance in the login flow
// (internal/play): a realm created today survives to its first turn, and it is
// the NEXT day's maintenance that collects it if nothing was ever played. For
// the same reason this must only run on a day that actually advances, never on
// the already-ran-today path, or a second login would erase a realm the player
// had not had a chance to play yet.
func (w *World) removeUnplayedEmpires() {
	// One switch governs whether this board reaps realms at all: a sysop who sets
	// Config.IdleDaysRemove to 0 ("never remove") keeps abandoned AND never-played
	// realms alike.
	if w.Config.IdleDaysRemove <= 0 {
		return
	}
	w.dropEmpires(func(e *Empire) bool {
		return e.Owner != "" && e.LastPlayed == "" && w.GameDay > e.CreatedDay
	})
}

// removeIdleEmpires removes a human realm nobody has played for
// Config.IdleDaysRemove days, so an abandoned realm does not sit in the
// rankings forever or grow the world file without bound (#83). 0 means never
// remove, matching how GameLength already treats 0 as endless.
//
// The realm is removed outright, like a never-played one, so the same caller may
// build fresh whenever they return. Its land is NOT returned to anyone's Daily
// Land Creation allowance: allowances are per-empire, so there is no shared pool
// for it to go back to.
//
// AI barons are exempt — they are played by aiPlay every day by definition, and
// an empty Owner is what marks a baron as AI.
// `today` is the REAL date maintenance is running for, not the game clock: a
// human's LastPlayed is stamped with the real date they played, so "unplayed for
// N days" is measured in real days away from the board. The two can differ,
// since the game clock advances at most one day per real day.
func (w *World) removeIdleEmpires(today string) {
	days := w.Config.IdleDaysRemove
	if days <= 0 {
		return
	}
	cutoff, err := time.Parse("2006-01-02", today)
	if err != nil {
		return // unparseable clock: remove nobody rather than guess
	}
	limit := cutoff.AddDate(0, 0, -days).Format("2006-01-02")
	w.dropEmpires(func(e *Empire) bool {
		if e.Owner != "" && e.LastPlayed != "" && e.LastPlayed < limit {
			w.postNews(fmt.Sprintf("%s has been abandoned by its ruler and fades from the map.", e.Name))
			return true
		}
		return false
	})
}

// FindByOwner returns the realm belonging to a caller's BBS handle, or nil.
//
// An empty handle matches NOBODY, though every computer baron stores one: the
// empty handle is the AI marker, not an identity, so a lookup on it used to
// return an arbitrary baron and hand the caller its realm. Callers that pass a
// handle straight from outside — a dropfile alias, an inter-BBS packet, a
// market key — get nil instead of a stranger's empire.
func (w *World) FindByOwner(handle string) *Empire {
	h := strings.ToLower(strings.TrimSpace(handle))
	if h == "" {
		return nil
	}
	for _, e := range w.Empires {
		if e.Owner == h {
			return e
		}
	}
	return nil
}

func (w *World) AIEmpires() []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if e.Owner == "" && e.Alive {
			r = append(r, e)
		}
	}
	return r
}

func (w *World) Targets(attacker *Empire) []*Empire {
	var r []*Empire
	for _, e := range w.Empires {
		if e != attacker && e.Alive && e.Protection == 0 && !w.AreAllied(attacker, e) {
			r = append(r, e)
		}
	}
	return r
}
