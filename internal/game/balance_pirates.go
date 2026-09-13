package game

// balance_pirates.go — the pirate raid's numbers (#211). They were declared in
// pirates.go beside the code that reads them; the rule is that a tunable lives
// in a data file named for its subject, so somebody retuning raids has one place
// to look. Every BRE provenance comment moved with its constant: that comment is
// the fidelity evidence, not decoration, and most of this set is binary-verified
// and therefore not free to retune (see AGENTS.md).

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
	// Seeds for a new game. BINARY-VERIFIED, read from the seeding loop itself
	// (`ovr_0571fe_entry_010a`, BRE.OVR 0x57320-0x57402: nine iterations, one
	// Random per field, closed by `cmp word [bp-0x2],0x9`). Found with a write
	// watchpoint during a live `BRE RESET`, after a static search for the table's
	// writers came up empty -- the reset composes the records and then writes
	// them to GAME.TMP, so nothing else ever references the address.
	//
	// This mattered because IB seeded NOTHING, so a new league's factions held
	// nothing to steal back and raiding paid zero until players had been robbed
	// for days. The original's factions are worth attacking from the first turn.
	PirateSeedTroopersRoll = 7_000 // mov ax,0x1b58
	PirateSeedJetsRoll     = 4_000 // mov ax,0x0fa0
	PirateSeedTurretsRoll  = 7_000 // mov ax,0x1b58
	PirateSeedTanksRoll    = 9_000 // mov ax,0x2328
	PirateSeedRegionsRoll  = 75    // mov ax,0x004b
	// Agents alone carry a floor: `call Random(1000)` then `add ax,0x32`, so the
	// band is 50..1049 rather than 0..999. Fitting 54 samples read that as a flat
	// Random(1050) -- close on the mean, wrong at both ends.
	PirateSeedAgentsRoll = 1_000
	PirateSeedAgentsBase = 50
	// Gold is 500,000 plus the PRODUCT of two Random(300) draws
	// (`mul dx` at +0x1ed, then `add ax,0xa120` / `adc dx,0x7`). A product of two
	// uniforms is right-skewed, which is why 54 samples would not fit a flat roll
	// -- median 14,073 against a mean of 18,891 -- and why the fitted constant was
	// the wrong SHAPE even where it matched the average. Range 500,000..589,401.
	PirateSeedGoldBase = 500_000
	PirateSeedGoldRoll = 300 // ...drawn twice and multiplied

	PirateCapGold     = 600_000_000 // binary (0x23c34600)
	PirateCapRegions  = 300         // binary (0x12c, clamped at the end of every raid)
	PirateCapAgents   = 200_000     // binary (0x30d40)
	PirateCapTroopers = 300_000     // binary (0x493e0)
	PirateCapJets     = 400_000     // binary (0x61a80)
	PirateCapTurrets  = 400_000     // binary (0x61a80)
	PirateCapTanks    = 200_000     // binary (0x30d40)
)
