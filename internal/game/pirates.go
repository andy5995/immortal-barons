package game

import (
	"fmt"
	"strings"
)

// PirateFactions are the nine raidable pirate factions. Their strength is
// randomized per game (NOT an easiest-to-hardest ladder), so any faction can
// turn out the strongest.
// The three generic words (Humans, Barbarians, Sharks) are kept; the rest are
// IB-original names, mostly adapted from Devonian sea life — fitting, since the
// kept "Sharks" are themselves Devonian survivors — plus Nightjackals. BRE's
// distinctive faction names are its own creative expression, so we don't reuse
// them (clean-room — see the bre-gather skill's license notes).
var PirateFactions = []string{
	"Humans", "Barbarians", "Nightjackals", "Sharks", "Dunkleoids",
	"Trilobarians", "Raptorians", "Gorgonoids", "Ammonians",
}

// PirateFaction is a living pirate faction. It raids empires — carrying off one
// kind of holding at a time and being granted new regions by the game (it does
// not steal the victim's regions) — and grows the longer it is ignored, since
// nothing shrinks it but a player attacking it. Pirates never take bombers or
// carriers.
type PirateFaction struct {
	Name string
	Land int // regions it holds (game-granted on raids; capturable)
	Gold int64
	// The loot IS the army. BRE keeps no strength stat for a faction — see Defense.
	LootTroopers int
	LootJets     int
	LootTurrets  int
	LootTanks    int
	LootAgents   int
	// Forces is the pre-0.0.5 strength scalar, kept only so an old save loads.
	// Nothing reads it; seedPirates no longer sets it.
	Forces int `json:",omitempty"`
}

// Defense is what a faction fields when a player raids it, computed live from
// the loot it is sitting on: BINARY-VERIFIED as tanks + turrets/2 + troopers/3
// (BRE.OVR 0x3671b). A faction has no strength of its own — it fights with what it
// stole, so draining its hoard is what actually weakens it, and a faction that
// has robbed nobody cannot defend itself at all.
//
// Note this does NOT gate raiding: a raid on a player is not a battle (the
// steal path never reads these fields), so day-one empty factions rob players and
// arm themselves from the proceeds.
func (p *PirateFaction) Defense() int {
	return p.LootTanks + p.LootTurrets/2 + p.LootTroopers/3
}

// Pirate tuning. Everything marked "binary" is read out of BRE.OVR and, where
// a capture could reach it, confirmed against one; the raid frequency is still
// reconstructed from play (see docs/mechanics-reference.md).
const (
	// How often a realm is raided. BINARY-VERIFIED (BRE.OVR 0x35db5): the roll
	// is Random(20) <= min(6, regions/1200 + 2), so the chance RISES WITH THE
	// REALM — 3-in-20 (15%) for a small one up to the 7-in-20 (35%) ceiling at
	// 4,800 regions. IB's flat 20% was a guess that happened to sit mid-band.
	PirateRaidChanceOutOf    = 20   // binary: Random(20)
	PirateRaidChanceBase     = 2    // binary: the +2 floor
	PirateRaidChanceCap      = 6    // binary: min against 6
	PirateRaidRegionsPerStep = 1200 // binary: one extra chance in 20 per 1,200 regions

	// After a raid attempt — landed or not — a 1-in-10 roll runs the whole
	// routine again, faction re-picked (BRE.OVR 0x363ba, a recursive call into
	// its own entry). So a turn can carry several raids, each less likely than
	// the last. IB's old model was a flat 5% chance of exactly one extra raid,
	// forced to be a DIFFERENT faction; neither is in the binary.
	PirateRaidRetryOutOf = 10 // binary: Random(10) == 0

	// Battle casualties, paid by BOTH sides whoever wins (BRE.OVR 0x367ad and
	// 0x368df, both above the win/loss compare). The attacker loses
	// Random(5)+2 percent of what it committed; the faction Random(10)+4
	// percent of what it holds. BINARY-VERIFIED, including the /100 divisor,
	// which decodes from the Turbo Pascal real at 0x87,0,0x4800.
	PirateAttackerLossMin    = 2  // binary
	PirateAttackerLossJitter = 5  // binary: Random(5) added to the minimum
	PirateDefenderLossMin    = 4  // binary
	PirateDefenderLossJitter = 10 // binary

	// What a winning raid hands back, as a divisor of the faction's holding.
	// BINARY-VERIFIED (BRE.OVR 0x36bd3 onward) and confirmed by three
	// consecutive raids on one faction in cap/kde3-01.cap.
	PirateReclaimDivMain   = 3 // binary: gold, regions, agents, troopers
	PirateReclaimDivArmour = 4 // binary: jets, turrets, tanks
	// What one raid carries off: have/33, held to 24000 + Random(1000). BINARY-
	// VERIFIED (BRE.OVR 0x35f66: a 32-bit divide by 0x21, then a min against
	// 0x5dc0 + Random(0x3e8)) and confirmed against captures — see pirateTake.
	// The earlier 5%/24999 pair was reconstructed and roughly two-thirds too
	// harsh; the 24,999 came from reading the jitter's top as a flat cap.
	PirateRaidTakeDivisor = 33     // binary
	PirateRaidCapBase     = 24_000 // binary
	PirateRaidCapJitter   = 1_000  // binary
	PirateRaidLandMax     = 25     // binary: Random(25) regions granted to a faction per raid

	// Hard caps on faction holdings, clamped at the end of every raid and again
	// after a player beats the faction. No bombers/carriers — a faction never
	// holds either. BINARY-VERIFIED from the clamp sites themselves (BRE.OVR
	// 0x3629c onward and 0x36a59 onward, each a min against a literal), which
	// supersedes the earlier reading of the BRE.EXE table at 0x14ede: that
	// table is some other set of limits, not these.
	PirateCapGold     = 600_000_000 // binary (0x23c34600)
	PirateCapRegions  = 300         // binary (0x12c, clamped at the end of every raid)
	PirateCapAgents   = 200_000     // binary (0x30d40)
	PirateCapTroopers = 300_000     // binary (0x493e0)
	PirateCapJets     = 400_000     // binary (0x61a80)
	PirateCapTurrets  = 400_000     // binary (0x61a80)
	PirateCapTanks    = 200_000     // binary (0x30d40)
)

// seedPirates creates the nine factions EMPTY. Nothing in the original seeds a
// faction with an army or with land: the only writes to its record anywhere in the
// overlay are the raid-steal path and the raid-resolution path, so a faction
// holds exactly what it has taken from players. A new game therefore opens with
// nine factions that can rob you from day one and cannot yet defend themselves.
func (w *World) seedPirates() {
	w.Pirates = make([]PirateFaction, len(PirateFactions))
	for i, name := range PirateFactions {
		w.Pirates[i] = PirateFaction{Name: name}
	}
}

// battleLoss is a percentage casualty, rounded as BRE rounds it.
func battleLoss(have, pct int) int { return have * pct / 100 }

// EnsurePirates seeds the factions after loading a save that predates them, and
// again if the sysop turns pirates back on. Turning them OFF leaves the factions'
// records alone rather than emptying them, so a faction keeps whatever it had
// stolen and the switch is reversible.
func (w *World) EnsurePirates() {
	if w.Config.Pirates && len(w.Pirates) == 0 {
		w.seedPirates()
	}
}

// pirateTake is how much of a holding of size `have` a single raid carries
// off: BRE's min(have div 33, 24000 + Random(1000)). Both halves are read from
// the binary and confirmed against captures — six trooper notices reproduce
// have/33 exactly, and the three largest takes seen (24,048 / 24,415 / 24,546)
// are the jittered cap rather than a flat ceiling.
func (w *World) pirateTake(have int64) int64 {
	t := have / PirateRaidTakeDivisor
	if cap := int64(PirateRaidCapBase + w.rng.Intn(PirateRaidCapJitter)); t > cap {
		t = cap
	}
	return t
}

// capAdd adds delta to cur, clamped to cap.
func capAdd[T number](cur, delta, cap T) T {
	cur += delta
	if cur > cap {
		cur = cap
	}
	return cur
}

// pirateRaidChance is how many of the twenty faces of BRE's die let a raid
// through: regions/1200 + 2, held to 6. A big realm is a bigger target — the
// figure is 3-in-20 for a realm of any modest size and tops out at 7-in-20.
func pirateRaidChance(e *Empire) int {
	k := e.Land/PirateRaidRegionsPerStep + PirateRaidChanceBase
	if k > PirateRaidChanceCap {
		k = PirateRaidChanceCap
	}
	return k
}

// maybePirateRaid gives one empire its per-turn exposure to pirates (#21 — BRE
// rolls this per turn, not once a day). New-realm protection wards it off.
// Called from PlayTurn so it applies uniformly to human and AI empires; a human
// victim's notice surfaces at the next turn's income (BRE's "since your last
// play" timing).
//
// BRE writes the retry as a recursive call to its own entry point, reached
// whether or not the attempt raided, so the loop below is that recursion
// flattened: roll, maybe raid, then a 1-in-10 roll to go round again with a
// freshly picked faction.
func (w *World) maybePirateRaid(e *Empire) {
	if !w.Config.Pirates || !e.Alive || e.Protection > 0 || len(w.Pirates) == 0 {
		return
	}
	for {
		faction := w.rng.Intn(len(w.Pirates))
		if w.rng.Intn(PirateRaidChanceOutOf) <= pirateRaidChance(e) {
			w.pirateRaidVictim(faction, e)
		}
		if w.rng.Intn(PirateRaidRetryOutOf) != 0 {
			return
		}
	}
}

// PirateSpoil names the one kind of thing a raid carried off, in the order
// BRE's own name table lists them (BRE.OVR, right after " has captured ").
type PirateSpoil int

const (
	SpoilTroopers PirateSpoil = iota
	SpoilJets
	SpoilTurrets
	SpoilTanks
	SpoilGold
	SpoilAgents
)

// pirateDraw is one face of BRE's sixteen-sided raid die: what the raid takes,
// and whether it comes out of the victim's own inventory or its Trading Market
// listing.
type pirateDraw struct {
	Spoil      PirateSpoil
	FromMarket bool
}

// pirateSpoilWeights is BRE's draw for what a raid takes: one Random(16) over
// an eleven-way ladder that lands on a single field (BRE.OVR at 0x35e30). Each
// unit type appears three times and gold and agents twice, so the four unit
// types are 3/16 each and gold and agents 2/16. This is why 35 raid notices
// across five captures never name two things: a raid takes ONE, not a slice of
// everything.
//
// **Faces 11-15 target the Trading Market listing, not the inventory** — the
// ladder's second field per unit type is the market's For Sale slot. Proven by
// driving BRE: listing 73 of 100 troopers wrote 73 to record +0x211 (the raid's
// bucket-11 field) and left 27 at +0x76 (bucket 0/1). So escrowing military
// does NOT hide it from pirates; it moves 5 of the 16 faces onto the escrow.
var pirateSpoilWeights = [16]pirateDraw{
	{Spoil: SpoilTroopers}, {Spoil: SpoilTroopers},
	{Spoil: SpoilJets}, {Spoil: SpoilJets},
	{Spoil: SpoilTurrets}, {Spoil: SpoilTurrets},
	{Spoil: SpoilTanks}, {Spoil: SpoilTanks},
	{Spoil: SpoilGold}, {Spoil: SpoilGold},
	{Spoil: SpoilAgents},
	{Spoil: SpoilTroopers, FromMarket: true},
	{Spoil: SpoilJets, FromMarket: true},
	{Spoil: SpoilTurrets, FromMarket: true},
	{Spoil: SpoilTanks, FromMarket: true},
	{Spoil: SpoilAgents, FromMarket: true},
}

// marketGood is the Trading Market good a spoil is listed under, or "" for gold
// (which is not traded).
func (s PirateSpoil) marketGood() string {
	switch s {
	case SpoilTroopers:
		return "Trooper"
	case SpoilJets:
		return "Jet"
	case SpoilTurrets:
		return "Turret"
	case SpoilTanks:
		return "Tank"
	case SpoilAgents:
		return "Agent"
	}
	return ""
}

// PirateHit is one raid notice: the faction, what it took, and how much. The
// front-end owns the wording and the faction's color, so the engine stores the
// parts rather than a sentence.
//
// Slot is the faction's index in World.Pirates. The colour BRE paints a faction
// belongs to its SLOT, not its name (PIRATECOLOR is indexed by faction number),
// so the front-end must not look the colour up by name — a world seeded under
// different faction names would find no match and fall back to plain body text.
type PirateHit struct {
	Faction string
	Slot    int
	Spoil   PirateSpoil
	Amount  int64
}

// pirateRaidVictim carries off a share of ONE of v's holdings into p's hoard
// and grows p; the game grants it new regions rather than taking v's. All
// holdings are clamped to their caps. Bombers and carriers are never a target —
// they are absent from BRE's name table.
func (w *World) pirateRaidVictim(slot int, v *Empire) {
	p := &w.Pirates[slot]
	draw := pirateSpoilWeights[w.rng.Intn(len(pirateSpoilWeights))]

	// A market face drains the victim's listing instead of its inventory. Gold
	// is not traded, so it has no market face.
	var src *int
	if draw.FromMarket {
		if l := w.marketListing(v.Name, draw.Spoil.marketGood()); l != nil {
			src = &l.Qty
		}
	} else {
		switch draw.Spoil {
		case SpoilTroopers:
			src = &v.Troopers
		case SpoilJets:
			src = &v.Jets
		case SpoilTurrets:
			src = &v.Turrets
		case SpoilTanks:
			src = &v.Tanks
		case SpoilAgents:
			src = &v.Agents
		}
	}

	var took int64
	if draw.Spoil == SpoilGold {
		took = w.pirateTake(v.Gold)
		v.Gold -= took
		p.Gold = capAdd(p.Gold, took, PirateCapGold)
	} else if src != nil {
		took = w.pirateTake(int64(*src))
		*src -= int(took)
		switch draw.Spoil {
		case SpoilTroopers:
			p.LootTroopers = capAdd(p.LootTroopers, int(took), PirateCapTroopers)
		case SpoilJets:
			p.LootJets = capAdd(p.LootJets, int(took), PirateCapJets)
		case SpoilTurrets:
			p.LootTurrets = capAdd(p.LootTurrets, int(took), PirateCapTurrets)
		case SpoilTanks:
			p.LootTanks = capAdd(p.LootTanks, int(took), PirateCapTanks)
		case SpoilAgents:
			p.LootAgents = capAdd(p.LootAgents, int(took), PirateCapAgents)
		}
	}
	p.Land = capAdd(p.Land, w.rng.Intn(PirateRaidLandMax), PirateCapRegions)

	// Only humans read the notice; the raid itself still hits AI victims.
	if v.Owner != "" && took > 0 {
		v.PirateHits = append(v.PirateHits, PirateHit{Faction: p.Name, Slot: slot, Spoil: draw.Spoil, Amount: took})
	}
}

// breTally renders a raid tally the way BRE writes one: capitalised unit names
// in a fixed order, comma separated, with "and" before the last. Zero entries
// are kept, so the same fields appear every time and a missing one means the
// unit does not take part rather than that none were lost.
//
//	You took 78000 Gold, 8 Regions, 525 Troopers, 481 Jets, 606 Turrets, and 175 Tanks.
func breTally(parts []string) string {
	switch len(parts) {
	case 0:
		return "nothing"
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}

// raidLoot is the "You took" tally. The order is BRE's, from the captured
// screen in docs/dev/bre-screens.md. Regions are omitted when the faction holds
// no land, which that capture records as BRE's own behaviour; Agents have no
// BRE counterpart and follow the units it does list.
func raidLoot(gold int64, regions, troopers, jets, turrets, tanks, agents int) string {
	parts := []string{fmt.Sprintf("%d Gold", gold)}
	if regions > 0 {
		parts = append(parts, fmt.Sprintf("%d Regions", regions))
	}
	parts = append(parts,
		fmt.Sprintf("%d Troopers", troopers),
		fmt.Sprintf("%d Jets", jets),
		fmt.Sprintf("%d Turrets", turrets),
		fmt.Sprintf("%d Tanks", tanks))
	if agents > 0 {
		parts = append(parts, fmt.Sprintf("%d Agents", agents))
	}
	return breTally(parts)
}

// raidWinLines and raidFailLines are the headlines a raid may draw, picked at
// random so a player raiding the same faction all day is not told the same
// sentence every time. The first of each is BRE's own, verbatim from BRE.OVR;
// the rest are IB's, kept to its register and near enough its length that no one
// line stands out as the "real" one — and short enough not to force a wrap.
var raidWinLines = []string{
	"Your efforts against %s have brought you success!",
	"Your forces broke the %s and came home loaded.",
	"The %s scattered before your assault.",
	"You overran the %s and took back what you could.",
	"The %s could not hold what they had taken.",
}

var raidFailLines = []string{
	"You could not successfully raid %s.",
	"Your raid on the %s came to nothing.",
	"The %s drove your forces back.",
	"Your attack on the %s was beaten off.",
	"The %s held against everything you sent.",
}

// raidWin renders a winning raid under an already-chosen headline (the caller
// owns the RNG). The losses line appears only when the raid actually cost
// something; see the note at the call site.
func raidWin(headline, loot string, troopers, jets, tanks int) string {
	report := fmt.Sprintf("%s\nYou took %s.", headline, loot)
	if troopers > 0 || jets > 0 || tanks > 0 {
		report += fmt.Sprintf("\nYou lost %s.", raidLosses(troopers, jets, tanks))
	}
	return report
}

// raidLosses is the "You lost" tally — the three types a raid may commit.
func raidLosses(troopers, jets, tanks int) string {
	return breTally([]string{
		fmt.Sprintf("%d Troopers", troopers),
		fmt.Sprintf("%d Jets", jets),
		fmt.Sprintf("%d Tanks", tanks),
	})
}

// RaidFaction resolves a player's attack on a pirate faction. The attacker
// commits troopers/jets/tanks (each clamped to what it owns); its strength is
// troopers/2 + jets + tanks*2 against the faction's Defense, and it wins on
// strictly greater. BINARY-VERIFIED shape (BRE.OVR 0x366e5 and 0x3671b).
//
// BOTH sides take casualties whichever way it goes, then a win hands back a
// third of the faction's gold, regions, agents and troopers and a quarter of
// its jets, turrets and tanks — so draining a fat faction takes several hits.
// The report names the loot; capturedLand is the number of regions won (0 on a
// loss or against a landless faction), which the caller lets the player allocate
// by type through the same picker a Regular Attack uses (#21). Reclaimed gold and
// military land in the attacker at once; only the land is deferred.
func (w *World) RaidFaction(a *Empire, faction, troopers, jets, tanks int) (report string, capturedLand int) {
	if faction < 0 || faction >= len(w.Pirates) {
		return "There are no pirates by that name.", 0
	}
	p := &w.Pirates[faction]
	startForces := p.Defense() // battle scale for Score, before any reclaim shrinks it

	troopers = clampInt(troopers, 0, a.Troopers)
	jets = clampInt(jets, 0, a.Jets)
	tanks = clampInt(tanks, 0, a.Tanks)

	offense := troopers/2 + jets + tanks*2
	defense := p.Defense()

	// Both sides pay, win or lose: BRE charges the attacker a slice of what it
	// committed and the faction a slice of what it holds BEFORE testing who won
	// (BRE.OVR 0x367ad-0x369ff, the losses are straight-line above the compare).
	tLost := battleLoss(troopers, w.rng.Intn(PirateAttackerLossJitter)+PirateAttackerLossMin)
	jLost := battleLoss(jets, w.rng.Intn(PirateAttackerLossJitter)+PirateAttackerLossMin)
	kLost := battleLoss(tanks, w.rng.Intn(PirateAttackerLossJitter)+PirateAttackerLossMin)
	a.Troopers -= tLost
	a.Jets -= jLost
	a.Tanks -= kLost
	p.LootTroopers -= battleLoss(p.LootTroopers, w.rng.Intn(PirateDefenderLossJitter)+PirateDefenderLossMin)
	p.LootJets -= battleLoss(p.LootJets, w.rng.Intn(PirateDefenderLossJitter)+PirateDefenderLossMin)
	p.LootTanks -= battleLoss(p.LootTanks, w.rng.Intn(PirateDefenderLossJitter)+PirateDefenderLossMin)

	if offense > defense {
		// A win hands back a third of the faction's gold, regions, agents and
		// troopers and a quarter of its jets, turrets and tanks (BRE.OVR
		// 0x36bd3 onward: f09 divides by 3 or 4 per field). Confirmed against
		// three consecutive raids on one faction in cap/kde3-01.cap, where the
		// untouched-by-combat fields decay by exactly 2/3 (gold, agents) and
		// exactly 3/4 (turrets).
		gotT := p.LootTroopers / PirateReclaimDivMain
		gotJ := p.LootJets / PirateReclaimDivArmour
		gotU := p.LootTurrets / PirateReclaimDivArmour
		gotK := p.LootTanks / PirateReclaimDivArmour
		gotA := p.LootAgents / PirateReclaimDivMain
		gotG := p.Gold / PirateReclaimDivMain
		gotLand := p.Land / PirateReclaimDivMain

		p.LootTroopers -= gotT
		p.LootJets -= gotJ
		p.LootTurrets -= gotU
		p.LootTanks -= gotK
		p.LootAgents -= gotA
		p.Gold -= gotG
		p.Land -= gotLand

		a.Troopers += gotT
		a.Jets += gotJ
		a.Turrets += gotU
		a.Tanks += gotK
		a.Agents += gotA
		w.creditGold(a, gotG, "reclaimed pirate loot")
		// The captured land is DEFERRED, not auto-added: the caller opens the
		// region-type picker so the player chooses the composition (#21). Reclaimed
		// gold/military above land immediately; only the regions wait.
		capturedLand = gotLand
		addScore(a, startForces/PirateScoreDivisor)
		w.postPirateNews(a, p.Name, true)
		// BRE's wording, verbatim from BRE.OVR ("Your efforts against ",
		// " have brought you success!", "You took ", "You lost "). Three short
		// lines rather than IB's old single sentence — though a full tally can
		// still pass 80 columns, because BRE abbreviates gold ("78k Gold") where
		// IB prints the figure, so the display still wraps it.
		//
		// The third line is the attacker's own casualties, which BRE prints on a
		// win too — a winning raid is not free there (#119).
		headline := fmt.Sprintf(raidWinLines[w.rng.Intn(len(raidWinLines))], p.Name)
		return raidWin(headline, raidLoot(gotG, gotLand, gotT, gotJ, gotU, gotK, gotA), tLost, jLost, kLost), capturedLand
	}

	addScore(a, -startForces/PirateScoreDivisor)
	w.postPirateNews(a, p.Name, false)
	return fmt.Sprintf("%s\nYou lost %s.",
		fmt.Sprintf(raidFailLines[w.rng.Intn(len(raidFailLines))], p.Name),
		raidLosses(tLost, jLost, kLost)), 0
}
