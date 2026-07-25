# BRE mechanics reference

This document describes how the original *Barren Realms Elite* (BRE) and
its predecessor *Solar Realms Elite* (SRE) played. It is a reference for
building Immortal Barons faithfully. It records game *rules and mechanics*
only (which are not covered by copyright). No original code, text, or art
is copied.

Sources are listed at the bottom. Numbers marked "config" are sysop
settings from a real league, so they are defaults, not fixed rules.

## Military units

### Combat values (exact, from the Wennagel guide)

| Unit | Offense | Defense | Notes |
|------|---------|---------|-------|
| Trooper | 1 | 1 | Cheap. Eats a lot of food. Hurt by terrorist ops. A large garrison makes an enemy R5-Slappenheimer likelier to backfire. |
| Jet | 2 | **0** | Offense only. High upkeep. Needs carriers (1 carrier moves 100 jets). SDI cuts jet strength 25–30%. |
| Turret | **0** | 2 | Defense only — the defensive **counterpart to jets**: it shoots down attacking jets (and blows up tanks / kills troops). Also helps intercept nuclear missiles. Cannot be destroyed by terrorist ops. |
| Tank | 4 | 4 | Best all-round. Low upkeep, high buy cost. Strength scales with HQ and morale. Helps defend vs. chemical missiles. |
| Bomber | 0 | 0 | Carries bombs / special-ops; destroys enemy *grounded* jets when sent in an attack. |
| Carrier | 0 | 0 | Support: moves jets to battle and goods for trade. |
| HeadQuarters | — | — | Raises tank effectiveness; enemies bomb it to weaken your tanks. |

So combat must track a separate **offense score** and **defense score**,
not one shared value. Jets add only offense; turrets add only defense.

Terminology note: some BRE guides call the defensive unit a **"Missile
Base"** rather than a "Turret" (same defensive role: it shoots down jets,
destroys tanks, and kills troops). **Commanders** are a support unit —
you need roughly **1 commander per 50 troops** (100 per 5,000) to keep
troops effective.

Guides sometimes mix in Solar Realms Elite's space theme. The unit map is:
region↔planet, trooper↔soldier, jet↔fighter, tank↔heavy cruiser,
turret↔defense station, commander↔general, agent↔covert agent. Same roles,
different names.

### Costs, maintenance, and net-worth values

Rough buy-cost ratio: about **7 troopers = 1 tank**, and about **6 jets =
1 tank**. So a tank has the offense of 4 troopers but costs ~7 — troopers
are cheaper short-term, tanks win long-term (lower upkeep).

Maintenance per unit per turn, at the sysop's low / medium / high setting:

| Unit | Low | Medium | High |
|------|-----|--------|------|
| Trooper | 0.10 | 0.60 | 1.60 |
| Jet | 0.30 | 1.20 | 4.80 |
| Turret | 0.255 | 0.90 | 3.60 |
| Bomber | 0.32 | 1.30 | 5.20 |
| Tank | 0.15 | 0.60 | 2.40 |
| Carrier | 0.025 | 0.10 | 0.40 |

Net-worth value contributed per unit (from the guide's net-worth table):

| Item | Value | Item | Value |
|------|-------|------|-------|
| Trooper | 0.250 | Tank | 1.250 |
| Jet | 0.325 | Bomber | 3.000 |
| Turret | 0.425 | Carrier | 1.000 |
| Agent | 0.500 | Region | 12.50 |

### Maintenance payment flow (BRE-verified)

Captured live from BRE (fresh realm, Auto-Pay Maintenance off). With Auto-Pay on
and enough gold on hand, all maintenance is paid silently. Otherwise the manual
flow runs in this order:

1. **Visit the bank? (y/N)** — opens the bank so a baron short on hand can
   withdraw savings to cover upkeep.
2. **Armed-forces upkeep**, then **region maintenance** — each a "how much will
   you give?" prompt. The prompt's max is the amount **required** (you cannot
   overpay); if you can't afford it, the max is your gold.
3. **Crown tax** — a per-turn tax to the Queen Royale (a non-player NPC monarch)
   that is a pure sink (no recipient); its prompt max is your available gold.
   *(Not built in IB yet — issue #52; the amount formula needs the disassembly.)*
4. Conditional: SDI maintenance (with SDI), waste-region decontamination (with
   waste regions), then the popular-support and military-morale boosts (shown
   only below 100). Support/morale are *requested* (optional), not required.
5. **Reconsider gate** — underpaying any *required* cost warns of disastrous
   results and offers to reconsider. Yes **restarts the whole sequence from the
   bank prompt**; No proceeds, with desertion/revolt for the shortfall.

Prompt colors (from a color capture): text plain white; the required and
suggested amounts bright cyan, the max dark cyan, the `(…; …)` parens bright blue.

IB implements steps 1, 2, the support/morale part of 4, and 5, with the
required-capped prompts and these colors. SDI/waste maintenance and the crown tax
(step 3) are not built (crown tax tracked in #52).

## Score (distinct from Net Worth)

The scores board shows **Score** and **Net Worth** as separate columns. Net
Worth is the wealth snapshot above. **Score is a cumulative, earned metric**,
verified from live BRE play: it starts at **0** and grows by the empire's
**net worth measured at the start of the day**, awarded **once per turn played**
— flat within a day, regardless of mid-day growth. Measured: a standard new
realm (NW 212.5) scored a flat **+213 every turn**; 8 turns = **1704** (matching
another 8-turn realm exactly). BRE's exact per-win attack bonus and the
day-rollover behaviour are not recoverable from the binary (overlay-blocked).

IB implements: `Empire.Score` (seeded 0) `+= ScorePerTurn` (a flat **213**) each
turn played. All measured BRE empires were standard starts (net worth 212, award
213), so whether the award tracks net worth or is a size-independent constant was
never distinguishable — IB awards a **flat constant**, so Score measures turns
played, not realm size. **IB additions (not in BRE):** a riot and food spoilage
each subtract `ScorePerTurn/10` from Score (tunable in `balance.go`; Score never
goes below 0). BRE's exact attack-scoring bonus is unrecoverable from the binary,
so IB uses its own combat-score model (below).

**Combat score (IB's own).** A battle's Score award scales with the forces used
up in it (units both sides lose = `battle`):

- Attacker wins: attacker `+= battle / CombatScoreDivisor`; the defender loses
  `CombatLoserPenaltyPct%` of that (a bit less than the winner gained).
- Defender repels the attack: the defender gains `DefenseWinBonusPct%` of an
  attacker's award (a successful defense is worth more); the losing attacker
  loses `CombatLoserPenaltyPct%` of that.
- Raids on a **pirate faction** move Score only a little — win or lose,
  `faction strength / PirateScoreDivisor` (scaled by the fight's real size, not
  the army you bring, so it can't be farmed; far below an empire battle).

All four constants live in `balance.go`; Score never drops below 0.

## Attack types

- **Regular attack** — direct assault; the winner takes some of the
  loser's regions. `attack.hlp` documents the Normal attack as "both sides fight
  to 15% losses"; a **live BRE test (2026-07-21, two samples)** showed losses are
  actually **asymmetric — the LOSER bleeds ~20%, the WINNER ~8%** (scaled by the
  Attack Damage config). IB now uses `RegularAttackLoserLossPct` (20) /
  `RegularAttackWinnerLossPct` (8), deciding the winner first.
  - **Quick Strike / Extended Battle** (`attack.hlp`): these are **IBBS
    group-attack** variants, not local — a live local Regular Attack offers no
    variant menu — so IB correctly does not offer them locally (verify against
    the group-attack docs).

  Region **capture** follows `max(RegularAttackCaptureFloor, RegularAttackCapturePct%
  × loser regions)`, scaled by Attack Rewards. A **live region-count sweep**
  (2026-07-21, Attack Rewards = Medium, five points 30–574 regions) gave a clean
  formula: **a ~15-region floor** below ~150 regions, **~10% above** — and it is
  **independent of the strength ratio** (verified 1.3×–4×). So a small realm loses
  a big share (a 100-region defender loses 15 = 15%), a large one ~10% (a 574-region
  defender loses 57). `attack.hlp`'s "20%" is likely the *High*-rewards value;
  Medium is ~10%. (An earlier "scales with the strength ratio" reading was an
  artifact of comparing victims of different *sizes* across resets — the count
  sweep corrected it.) The **loser only loses regions if it is the defender** — a
  losing attacker never loses land. In BRE the **winner then picks which region
  *types*** to take (a region picker); IB does the same on the human path (#58) —
  the winning attacker chooses the captured composition, decoupled from the
  proportional mix the loser bleeds, while the AI and group attacks auto-allocate.
  Like BRE, the attacker first picks a **committed force** — how many
  Troopers/Jets/Tanks/Bombers to send (jets usable only up to `Carriers × 100`).
  Only the committed units add offense, and only they take the loss; held-back
  units stay home and unhurt.
  The win **captures regions and takes no gold** — BRE's Regular Attack rewards
  land, not money (*"a successful assault brings you extra regions"*, `breins.txt`;
  gold-to-bank is the separate pirate-raid path). Either way a
  large empire is ground down over many attacks, while a small one can lose its
  last regions in one, which eliminates it (and the conqueror absorbs its
  surviving military).

  A player (human or AI) may launch at most `Config.MaxIndividualAttacks`
  regular attacks per **day** (BRE's "Maximum Individual Attacks Per Day",
  default **3**; `0` = unlimited). The count resets at daily maintenance and is
  shared across all of the day's turns, so it caps total aggression, not
  attacks-per-turn. Only conventional/regular attacks count — WMD strikes and
  pirate raids are not limited by it.
- **Nuclear attack** — turns enemy regions into waste (high cost).
- **Chemical attack** — damages fewer regions but kills a lot of people
  (and troopers).
- **Biological attack** — hurts people and troopers, but not land.
- **Attack pirates** — the nine pirate factions are living bands, not a
  fixed difficulty ladder: their strength is random (any faction can be the
  strongest). Their **names are IB-original** (BRE's coined names are its own
  creative work). Pirates raid players at random: IB rolls a **20%** chance per
  turn that an empire is raided, plus a further **5%** chance of a *second* raid
  by a different faction the same turn — about 1 turn in 5, matching the felt
  frequency in BRE (the exact BRE rate is not measured). A raid carries off a
  share of the victim's **troopers, jets, turrets, tanks, agents, and gold** —
  but never bombers or carriers, and never the victim's regions; the game grants
  a raiding pirate new regions instead, so a pirate that just raided is fatter.
  A single raid takes at most **24,999** of any one thing (BRE.EXE constant is
  25,000). Beating a pirate reclaims ~a fifth of its hoard per hit, so it
  takes several hits to fully recover your goods; **a winning raid that seizes
  pirate-held land opens the same region-type picker a Regular Attack uses
  (#21)**, while a raid on a landless band yields only gold and military.

  Hard caps on what a faction can hold (✓ = verified against the BRE.EXE caps
  table at 0x14ede — `5000,25000,50000,80000,80000,100000,100000,200000,600000`):
  tanks **50,000** ✓, troopers/jets/turrets **100,000** ✓ each, agents
  **65,000** ✓ (BRE.EXE ×3), regions **100** (play-data estimate), gold **600,000** ✓ — the
  earlier "300,000 assumed" is absent from the binary; 600,000 is in the table.
  Military parked in the Trading Market is safe from pirate raids.
- **Group vs. individual** (interplanetary) — a solo strike returns double;
  a group attack shares the returns.

**Doomer Kaboomer** (the clone's equivalent of BRE's *Gooie Kablooie*) — the
ultimate weapon, aimed at an entire enemy planet/BBS rather than one empire. It is community-funded (all members of
the attacking board contribute; funding scales with the target's size) and
only one can exist at a time. After funding it takes a few days to build,
then launches. On arrival it destroys 10% of *all* regions on the target
planet, then 5% more per day. After 5 days it exhausts itself — so the
defenders must send jets to shoot it down before then. It can also be
dismantled by its owner. The original's turn log and prompts confirm the
lifecycle: begin construction → fund (millions of gold) → complete →
awaiting launch → inbound processing.

**SDI Defense** — a funded anti-missile/anti-jet shield (spend up to ~2
billion; percent-complete scales with your region count, and Technology
regions lower its upkeep). When complete it destroys about half of incoming
missiles and cuts attacking jets' effectiveness by ~25–30%.

Per-day caps (config): individual 4, group 4, terrorist 25, bombing 4.
"Lost" attacking forces return after 3 days (config).

## Covert operations

Success depends on how many agents you have compared to the target: more
agents relative to the enemy means a higher success rate. Keeping many
agents on hand also *defends* you against incoming terrorist ops.

**Each op charges a gold fee up front** (on top of the agent risk), shown as
a cost column on the menu. The fees below are live-sampled from BRE's default
(medium) game setup on 2026-07-21 — other BRE setups scale them, so IB keeps
them as tunable `Cost*` constants in `balance.go`. A failed op still risks
losing the agent, but an op you cannot afford does nothing (charges neither
gold nor agent). The menu footer shows `You have <gold> gold and <N> agents.`

The menu's item order, labels, numeric hotkeys (1-9), and per-op costs are
confirmed from BRE (BRE.OVR string table plus a live capture, #73):

- **(1) Send Spy** — read the enemy's full status. Cost `CostSendSpy` (5,000).
- **(2) Stir Revolts** — propaganda that sharply lowers popular support.
  Cost `CostStirRevolts` (25,000).
- **(3) Set Up** — trick d and one of its Full Defense Alliance partners into
  believing the other declared war, voiding the alliance between them
  (useful against a defense pact protecting a target). Cost `CostSetUp`
  (50,000).
- **(4) Support Dissensions** — agitate d's own troopers into fleeing (~10%
  trooper loss). Cost `CostSupportDissensions` (80,000).
- **(5) Demoralize Forces** — lower enemy military morale; they fight worse
  and, if low enough, units desert. Cost `CostDemoralizeForces` (80,000).
- **(6) Spy on Relations** — reveal the enemy's treaties. Cost
  `CostSpyOnRelations` (100,000).
- **(7) Bomb Enemy Targets** — a submenu (see below). Cost
  `CostBombEnemyTargets` (100,000) per variant.
- **(8) Bribery** — bribe an enemy agent inside d, so d's future covert ops
  against you auto-fail. Cost `CostBribery` (200,000).
- **(9) Expose Enemy Ops** — per BRE.OVR ("Bribed Agent will expose enemy
  operations for 24 Hours"), a temporary shield against *all* incoming
  covert ops. IB models the 24 hours as one game-day
  (`ExposeOpsShieldDays`). Cost `CostExposeEnemyOps` (600,000).
- **(V) Visit Bank**.

**One effect op per turn (#54).** BRE caps *effect* covert ops at one per turn
("Limit one try per turn!"): the first effect op works, and any second effect op
of *any* type is refused that turn — verified live (Stir Revolts, then Set Up,
was refused). The two *info* ops — **Send Spy** and **Spy on Relations** — are
exempt and unlimited. IB enforces this: `covertCost(..., capped)` sets
`TurnProgress.CovertOpUsed` on the first effect op and returns
`ErrCovertCapReached` for the rest until the flag clears at turn start; the info
ops pass `capped=false`.

**Bomb Enemy Targets** submenu (BRE.OVR: "All missiles and bombs require
500 Bombers to deliver their payloads" — `BombingBombersRequired` gates
every item below):

- **Bomb Food Market** — destroy an empire's food reserve (can trigger a
  death spiral).
- **Bomb Trading Market** — raid a quarter of the target's gold.
- **Bomb Trade Routes** — sever one of the target's standing trade
  treaties.
- **Undermine Investments** — trim a quarter off the principal of the
  target's pending bank investments.
- **Nuclear Assault** / **Chemical Bombing** — reuses the WAR menu's
  `NuclearStrike`/`ChemicalStrike`.
- **R5-Slappenheimer** (the clone's rename of BRE's *S3-Sabre*, avoiding the
  original's coined name) — a variable-return missile. A disassembly of BRE's
  `SABREHIT` showed only 3 of the 11 dial settings (1, 2, 3) did anything and
  the manual never said which number did what, so from the player's seat the
  result was random; the target's SDI could intercept it and a heavily
  garrisoned target could turn it back on the attacker. IB keeps that feel but
  makes it honest: the **dial is a bluff** — the player still sets it 0–10 under
  User Select handling, but it changes nothing (every launch is the same random
  gamble). The `None` handling mode disables the weapon (gated in the menu);
  `User Select`/`Random`/`Constant` all enable it and differ only in the (inert)
  dial. The target's SDI (`d.SDI`%) can intercept, and only about 3 launches in
  10 (`SlappenheimerEffectHits`/`SlappenheimerEffectRange`) land a payload — the
  rest fizzle. A landed hit removes a random 5–30 %
  (`SlappenheimerBaseDamagePct` + `rng.Intn(SlappenheimerDamageSpread)`) of one
  asset, and ~1-in-`SlappenheimerMultiHitOdds` strafes several at once (BRE's
  "extremely devastating" outcome). Backfire is a continuous probability scaled
  by the target's Troopers (`d.Troopers / SlappenheimerBackfireScale`), applying
  the same damage to the attacker. BRE hid which field each effect hit, so IB
  picks its own spread (Troopers, Jets, Turrets, Tanks, Bombers, Carriers,
  Agents, Gold, Food, and Land — Land removed through the RegionMix so its Total
  stays equal to `Land`). The agent is lost only when the covert approach itself
  is foiled — not on an interception, a fizzle, or a backfire.

**Alliances:** a Terrorist-Prevention treaty adds half an ally's agents to
your defense; an Intelligence alliance adds half their agents to your
offense.

## Economy and regions

Land is bought from a market (config: 3000 starting units, 4000–5000 new
units created per day). There are eight region types, each with a
different economic role:

- **Coastal** — highest tourism income, but it collapses with low public
  support (under ~300/region at 0% support; about equal to mountains at
  37% support).
- **Mountain** — lowest income of the money regions, but the most stable
  (never fails); also boosts industrial output.
- **Desert** — solar income; swings widely (~1,900–3,000 per region).
- **River** — highest income on a cash turn (hydroelectric), but every so
  often it produces food instead — never rely on it for either alone.
- **Agriculture** — grows food; food self-sufficiency.
- **Urban** — more population, tax revenue, and trade capacity.
- **Technology** — long-term efficiency, applied as the `TechFactor` percent. It
  is **cumulative** (BRE-verified via live play): a per-empire `TechLevel`
  accumulates each turn Technology regions are held (gain ∝ tech-share², ceiling
  ∝ tech-share, hard cap `TechFactorCap`), so the bonus **ramps up over turns**
  and is not the instantaneous share. `TechFactor` raises army strength, all gold
  income (incl. tax), and food-region output by `tf`%; lowers upkeep on units
  **and regions** (#20) and food spoilage to `100−tf`%. Produces no direct gold.
  Constants (`TechGainDiv`, `TechCeilMul`, `TechFactorCap`) are IB's own tunable
  reconstruction; see the `bre-binary-verified-math` notes for the live data.
- **Industrial** — produces military goods; vital when buying arms is
  disabled and you must *build* instead. Also yields gold.

Waste regions (from enemy weapons) can be cleaned for less than the cost
of new land.

**Region gold income (BRE-verified — disassembly of BRE.OVR, offsets
0x342C0–0x34A4E).** Each gold region yields, per turn,
`perRegion = yield×Rate/100 + Base`, times its region count, where `yield` is a
per-(game-day), planet-wide factor in the band **0.30–0.80** (live-calibrated:
mountain, coastal, and river income all reconcile with the disassembled Bases at
this band — the earlier 1.0–1.5 reconstruction ran ~15–30% high):

| Region | Rate | Base | Notes |
|--------|-----:|-----:|-------|
| Mountain (ore) | 400 | 3,550 | smallest swing → most stable |
| Coastal (tourism) | 1,000 | 3,750 | × support factor `0.10 + 0.90·(Support/100)` — floor ~375/region at 0% support, never zero. **Live-verified (#31):** a headless BRE sweep of Tourism income across Support 0→100 (coastal count held at 3) fits `factor = 0.099 + 0.901·(Support/100)`, matching this curve to ~1% on both floor and slope |
| Desert (solar) | 2,000 | 3,000 | widest swing |
| River (hydro) | 100 | 5,000 | highest base; a river fishes instead some turns (#29) |

**Industrial** regions don't use a fixed rate/base — each is one shared capacity
pool (~2,600 gold-valued points/region, live-verified). The production
percentages buy units (costs: trooper 100 / jet 140 / turret 150 / tank 500 /
bomber 1,500 / carrier 1,750); the **unallocated %** pays out as gold 1:1.
Specialize: +25% to the chosen unit, −15% to the rest.

**Urban and Technology produce no direct gold** (BRE-verified): Urban is
population housing, Technology is an efficiency multiplier (see the Technology
region above). Food output: `Agricultural × 300` grown, then raised by the
Technology factor (#20); rivers fish `× 124` on a fishing turn (else hydropower
gold), see the Rivers section. These income numbers, the caps (2B money / 1.599B
interest), the pirate caps table, and the net-worth weights are BRE-scale;
**unit prices, the tax per-capita coefficient, and the yield band are IB's own
reconstructions** anchored to this scale (BRE computes prices/maintenance inline
— not stored as constants). All tunables live in `internal/game/balance.go`.

**Per-turn price walk (#30).** Each unit's buy price follows a *persistent random
walk*, per empire: every empire stores its own current prices (`Empire.Prices`) and
steps them once per turn (`World.stepPrices`, from `PlayTurn`). A step moves a price
up or down by up to `PriceWalkStep*`% of its base (cheap units 3%, agents 8%) and is
clamped to ±`PriceWalkBandPct`% (30%) of the base, so prices drift like BRE but can't
run away. The stored value is what the Spending menu shows and what a buy/sell charges
within the turn (shown == charged; buy and sell route through the same accessor, sell =
buy/3, agents flat), and it persists across days via the save. Steps are deterministic
(keyed per empire and turn, same `GameDay`/`TurnsLeft` basis as river fishing) so play
is reproducible and concurrency-safe. This matches a live 14-turn sample (2026-07-15):
each price drifted across turns and days, and the walk is per-empire — a fresh empire
created the same day a veteran had drifted saw prices back at base. AI empires walk
independently too (each keyed on its own name). **Regions do not walk** — their price
rises purely with holdings (`917 + owned×33`), which BRE held exact every turn.

**Population and tax** are a major income engine. The per-capita coefficient
(`TaxGoldPerCapita`) is calibrated so a new realm's first-turn taxes (~5,100)
match BRE's income report (~5,183) — a minor share of income, with region
income dominating, as in BRE. A *low* tax rate (2–3%)
drives fast population growth; late game, tax on a huge population becomes
the main income. Set tax to 0% for a few turns to spike growth, then buy
**urban** regions so people don't leave when you raise tax back to ~7–9%.
Growing population needs enough **agricultural** regions to stay fed.

**Population growth model — BRE's shape, the clone's own tuning.** A BRE.OVR
disassembly (HIGH confidence on shape) shows BRE grows population **logistically
toward a carrying capacity**, not at a flat birth rate:
`capacity = (Σ region·weight + Support·90 + base) · taxFactor(tax)`, then
`growth = min(capacity − People, People/2)` plus a small jitter (population is
stored in *millions*). Capacity is dominated by **popular support**; **tax**
lowers growth twice (a decreasing `taxFactor` multiplier *and* by dragging
support down); **urban** is one of the weighted region types. That `People/2`
term is a **50%/turn ceiling** — with high support a realm sits far below
capacity and pins to it, which is BRE's characteristic explosive growth. The
exact float `taxFactor(tax)` curve is the one value the disassembly could not
recover (a relocated Turbo-Pascal FP routine).

The clone keeps the **self-limiting logistic shape** but replaces BRE's runaway
ceiling with moderate rates (a deliberate balance choice, not a fidelity gap).
In `internal/game/turn.go`: `popCapacity = Land·20 + Urban·60 + Agricultural·20
+ Support·30`, growth closes headroom at `~1/12` per turn, clamped to
**±8%/turn** (`PopGrowthCapPct`), and positive growth needs food. Support is the
main lever, matching BRE's intent. Selling urban/agricultural land or losing
support lowers capacity, so population then **drifts down gradually** toward the
new capacity — the clone's answer to BRE's separate instant ~1M-per-urban
housing loss on a land sell (one code path instead of two, and it makes the
"no misrule emigration" result below correct by construction: being over
capacity *is* the attrition). All weights are tunable constants.

**Capacity is decoupled from food, as in BRE.** Carrying capacity is
support-driven; food is a separate gate (positive growth needs stored food, and
a food shortage starves population back). So a high-support, low-agriculture
realm can sit at a food *surplus* while its population climbs toward a capacity
whose food need will outrun production. This is the intended balance — you
manage agriculture (or the food market) to match a support-driven population,
not the other way round. The Civilian advisor warns ahead of that wall: when the
realm is fed now but its food need *at full population* would exceed production,
it flags "our people are still growing… add agricultural regions before then"
(issue #35).

**Industrial production:** industrial regions output military units. You
set production percentages across trooper/jet/turret/tank/bomber/carrier.
A common money tip: set industry to 100% carriers and *sell* the carriers
— more profitable than producing gold directly. Mountain regions boost
industrial output and are the most war-stable.

Guide region mixes for orientation: early game leans money regions
(coastal + river, ~1:1, plus ~5–10% agricultural); a war build leans
industrial + mountain (~40/40); a mature economy runs roughly a 4:3:3
agricultural : urban : technology ratio. Protection at game start is long
(about 20–70 turns), and a "day" is about 10–15 turns.

Other economy pieces: military maintenance, food consumption, land
maintenance, tax payments, spending on public support/morale, and the food
market (limited supply — relying on it is risky). Banking offers
interest-earning savings and loans; investment rates move over time.

**Popular support and military morale** are 0–100 stats. Each turn's payment
stage prompts for both when they are below 100 ("N gold is requested to boost
popular support / improve military morale"); a single turn's payment only
raises each by a capped amount, so full recovery takes several turns.
Underpaying maintenance lowers them (and taxes erode support). Low popular
support cuts Coastal income and, at the extreme, brings riots. Low military
morale scales down combat effectiveness, and below a threshold troops desert
each turn. The exact request/boost/decay constants are not in BRE's released
files (the empire record layout was deliberately withheld); the values in
`internal/game/payments.go` are reconstructed placeholders, tunable.

**Riots and emigration — verified against a BRE.OVR disassembly (HIGH confidence):**

- **Riot trigger + chance:** each turn a riot fires iff `tax > 10` **and**
  `tax*tax >= Random(10000)` — i.e. **riot probability = tax² / 10000**
  (quadratic, not linear). Samples: tax 15 → 2.25%, 20 → 4%, 30 → 9%,
  50 → 25%, 71 → ~50%, 100 → 100%.
- **Riot effect:** each riot removes **`People div 15`** (~6.67%) of the
  population, and cancels that turn's population growth (a suppression
  accumulator bumped by `tax div 3`). (Recovered by identifying the 32-bit
  divide/subtract runtime helpers across 138 call sites.)
- **Emigration is NOT a gameplay mechanic.** BRE's tiered civil-revolt /
  "most of your empire has left your rule" / troops-fleeing system is gated on
  a severity byte whose *only* nonzero setter is BRE's **crack/registration
  check** — it fires only when a *pirated* copy is detected. In a registered
  BRE, misrule-driven emigration never happens; misrule attrition is **riots
  (tax) and starvation (food) only**. So the clone models no misrule emigration.
- **Random population swings ARE real, though** — separate from the above.
  BRE's sysop-editable random events (`events.dat` `^GAINPEOPLE` / `^LOSEPEOPLE`:
  "people flee to your empire", "aliens drop off N million", "killed in bungee
  accidents", …) make population appear "from nowhere" and vanish "into
  oblivion" — never a transfer to/from another realm. The clone implements
  these as random per-empire events (`internal/game/events_random.go`,
  `eventPeople`).

The clone implements the verified riot trigger/chance and the `People div 15`
loss (`internal/game/turn.go`); the Support hit on a riot is the clone's own
addition, not from BRE. The growth-cancel (`tax div 3`) is not yet modeled.

Tax rate, bank interest, and investment rates are configurable (a real
league ran tax 85%, interest 75%).

### Concrete economy numbers (from strategy guides)

- **Bank interest: about 1% per turn** on gold held in the bank *while you
  are playing* (investments tie money up until your next login and are
  less useful for this). The clone's Interest Rate knob is anchored so its
  default (50) yields ~1%/turn, matching this.
- **Interest cap: 1,599,999,999.** Gold above this does not earn interest.
  At the cap, interest is roughly 25–35 million per turn.
- **Absolute money cap: 2,000,000,000.** You cannot hold more than 2 billion
  coins at once (in the bank or on hand) — a separate, higher ceiling than
  the interest cap.
- **Food market (issue #19):** food is bought and sold against a **shared
  planet-wide pool** that starts each day at `FoodMarketDailySupply` (1,000,000
  units, from BRE's live "~1,001,452 available today"). Buying depletes the pool
  and is capped to what remains; selling replenishes it. The sysop's **Food
  Unlimited** toggle (Config Editor; default off/limited) removes the cap. Prices
  **vary daily** within `buy ∈ [FoodBuyPriceMin, 3×FoodBuyPriceMin]` with
  `sell = buy/3` — BRE's own [20,60]/[7,20] band (IB runs BRE-native economy
  scale; `FoodBuyPriceMin = 20` in `balance.go`).
- **Food production:** `Agricultural × FoodPerAgri (300)` per turn, calibrated to
  live BRE (97 Agri → 29,197; 16 Agri → 4,864, both no River).
- **Rivers — hydropower *or* fishing (issue #29, live-verified):** each turn an
  empire's rivers do EITHER hydropower (gold, as usual) OR fishing, chosen by a
  per-turn coin-flip (`RiverFishChance`, ~50/50 in live BRE). On a fishing turn
  the rivers yield `River × RiverFishFood (124)` food and **no** river gold that
  turn; on a hydropower turn, gold and no river food. (Constants in `balance.go`;
  the exact chance/rates are tunable, live-sampled.) **Caveat (2026-07-23):** a
  10-turn color capture of a *food-surplus* BRE empire showed its 24 rivers running
  hydropower on **every** turn (0 fishing across ~240 river-turns) — inconsistent
  with an unconditional ~50/50 flip. This suggests BRE fishing may be **much rarer**
  than 50%, or **conditional on a food shortage** (this empire never needed food).
  `RiverFishChance = 50` is therefore unconfirmed and likely too high; the
  disassembly or a food-short capture would settle it.
- **Food growth is a *turn-start* credit (matches BRE).** This turn's food yield
  (`Agricultural × 300` + river fishing) is added to the granary at the **start**
  of the turn — alongside military production and gold income, exactly what the
  start-of-turn income report announces (`World.GrowFood`). So the player can
  **sell or spend this turn's growth the same turn**. (Earlier IB deferred the
  growth to the end-of-turn economy step, where it arrived after the food market
  and was always subject to spoilage — corrected 2026-07-20 after driving BRE.)
- **Food consumption:** each turn the population eats
  `People × PeopleFoodPerThousand (75) / 1000` and the army eats
  `(Troopers + Jets×2 + Tanks×2) / ArmyFoodDivisor (200)` — about **1 food per 200
  troops**, with jets and tanks counting double. The army rate is **live-verified**
  against BRE (2026-07 IBBS capture: 42,259 troopers → "Armed Forces Require 211").
  So a large standing army is nearly food-free, as in BRE — food pressure comes from
  population, not the army. (Both constants in `balance.go`. The **people** rate is
  IB's own reconstruction at IB's population scale; it lands lighter than BRE's
  ~1.5 food per million people, so population food is somewhat easier than BRE.
  Fixed 2026-07-23: the army was previously billed 1 food/trooper, ~200× too heavy.)
- **Food spoilage:** **5% of the food remaining after growth and consumption**
  spoils each turn — `floor(0.05 × food)` — with **no floor** below which nothing
  spoils, reduced by Technology regions. **Re-verified by driving the original
  (2026-07-20):** spoilage matched `floor(5% × food-after-grow-and-consume)` to
  the unit at three stocks (1,452→72, 2,668→133, 0→0). Because growth is credited
  at turn start (above), selling the surplus down to next-turn consumption drains
  the granary to ~0 after feeding, yielding **zero spoilage** — BRE's "sell excess
  → no decay" behavior. (`FoodSpoilPct` in `balance.go`. An earlier 2026-07-11
  disassembly read hypothesized a ~1,000-unit floor + decay of the excess; the
  live driving disproved it — a floor would give 22/83, not 72/133.)
- **Feeding & food shortfall:** each turn the realm consumes food; a **feed stage**
  (BRE's Payment→Food-Market slot) warns when short, and with **Auto-Feed** on the
  Food Market opens automatically so the player can buy food. Going underfed hurts:
  **popular support and military morale drop and people emigrate, scaled to how
  much of the turn's food need went unmet** (`FoodShortfallSupportDrop` = 70
  support points, `FoodShortfallMoraleDrop` = 80 morale points — hungry troops
  demoralize faster than the public — and `FoodShortfallEmigrationPct` = 10% of
  population, all at 100% unfed). IB's own reconstruction (BRE publishes no rate),
  calibrated to a live BRE point: ~73% short dropped support ~50 points in one
  turn. breins.txt confirms the direction: *"Without food, morale and public
  support will [decline]."*
- **Land market:** starts with a pool of about 5,000 regions; you may buy
  at most **500 regions per turn**; the per-region price rises as you own
  more (about 1,100 coins/region when you hold only 2).
- **Loans** from the bank scale with how many regions you own.
- **Trade deals** can send gold from one player to another (used by teams
  to stack bank interest across many players).
- **Starting force** (one league): 50 soldiers and 2 commanders, plus 2
  regions. Unit terms seen in guides also include tanks, "commanders," and
  "missile bases."
- **Starting food:** about 30,000 tons; a new empire uses only ~1,000 per
  turn early on.
- **Tax cap:** you can raise tax to about **39%** before you start losing
  popular support (people leave). Low tax (2–5%) maximizes population
  growth.
- **Emperor gifts:** a very small realm (e.g. one left at 2 regions)
  receives daily gifts — stolen coins from the Emperor or free military
  (e.g. thousands of tanks). This matches the "Gold for being" award text
  in the original binary. It enables a "quit and idle" fast-start exploit.
- **Investment** is a separate, higher-return option than bank interest,
  but it ties money up until a set time (e.g. ~24 hours). Bank interest
  (~1%/turn) applies to money on hand *while you play*.
- **Registered-only:** in the original, bank interest, loans, and
  investments only work in the registered (paid) version. Not a constraint
  for our clone, but it explains why some guides note "registered only."
- **Technology regions** give gradual, game-long improvements (no discrete
  "discoveries"): lower maintenance on regions/military/SDI and rising
  economic and military efficiency over time.

### Banking and investments (from the binary strings)

The bank menu (BRE "Crazy Gold Bank", IB "Goldie Luck's Bank"): **Cash Relief /
Loans**, **Deposit Funds**, **Withdraw Funds**, **Investments**, **List
Investments / Loans**, and **View Bank Rates**.

- **Savings** earn the *Bank Interest Rate* on gold in the bank. BRE-faithful
  (config-help verified): the knob is "interest the bank gives in **10 days**", so
  `InterestRate/10` is the **daily** rate (config 50 → **5.0%/day**, shown in View
  Bank Rates), credited "at the end of each turn" — so per turn it is the daily
  rate spread across `TurnsPerDay` turns: `Bank × InterestRate / (1000 ×
  TurnsPerDay)`. (This replaced IB's old flat ~1%/turn, which compounded to a much
  higher effective daily rate.)
- **Investments** are term deposits (like bonds): you choose an **amount**
  and a **number of days** — **2 to 10 days** (`MinInvestDays`=2, `MaxInvestDays`=10,
  live-BRE-verified: the bank prints "There is now a 2 day minimum on
  investments." and prompts "…invest for? (2; 10)") — the gold is **locked**, and
  it **matures on a future date**, returning principal plus interest at the
  current **Investment Rate**, **compounded daily** (live-verified: 1000 for 2
  days at 5%/day returns 1102 = 1000·1.05²). Before confirming, the bank shows
  "Returns expected to be approximately N gold. Accept? (Y/n)"; on accept it
  reports "Investment will be returned on MM/DD/YYYY." The list view shows
  columns: Date / Investments / Loans Due.
- **Cash Relief / Loans** (#40) — term-based borrowing (`internal/game/loan.go`).
  You choose a **repayment term** of **1–10 days** (`LoanMinDays`/`LoanMaxDays`),
  the bank shows the **rate** ("The loan rate will be X% per day, totalling Y%
  overall") and a **ceiling** ("We will provide up to N gold"), then you borrow
  up to it and **owe the compounded total on the due date** ("You owe N gold in D
  Days."). Loan math is **live-BRE-verified**: daily rate = `8.0 + 0.2·days` %
  (`LoanBaseRateTenths`/`LoanRatePerDayTenths`; 2d→8.4, 5d→9.0, 10d→10.0),
  compounded daily (1000@2d=1175, 616@5d=947, 500@10d=1296). The **ceiling
  formula is IB-reconstructed** (`LoanCeilingMultiple` × net worth less
  outstanding — BRE's exact formula is unverified, the gathered points were
  confounded by growing debt). At the due date `matureLoans` deducts the amount
  owed from gold then bank; an unpaid loan **defaults** — the shortfall rolls into
  open-ended **Debt** grown by `LoanDefaultPenaltyPct` (25%) and support drops.
  Defaulted **Debt** still grows `DebtGrowthPct`/turn and is repaid from the same
  Cash Relief screen.
- **The Investment Rate floats** — each daily maintenance updates it:
  - Supply/demand: heavy investing pushes rates **down**; weak investing
    pushes them **up**.
  - The bank nudges rates by 0.5% to stop them collapsing or skyrocketing.
  - Random events: the Queen occasionally raises/lowers investment rates by
    ~1% (inflation flavor).
  - The sysop configures a **Standard** and a **Steady** investment rate.
- **Loans**: you borrow gold at a stated loan interest rate ("The loan rate
  will be N% interest overall"); loans appear in the list with a due date.
- **Undermine Investments** is a covert op that damages a rival's pending
  investments.

Longer terms and higher rates mean bigger returns, but the gold is locked
until maturity — the strategy guides exploit this heavily (invest in
staggered tranches so one matures each day).

## New-player start

A new player sees `Welcome to <board>` / `Barren Realms Elite`, then the
new-player intro text (`NEWPLAY.TXT`), then the naming step titled "Name
Your Empire" with the prompt `Name your Realm:`. A name must have at least
3 letters/numbers and must not match another player; otherwise: "Your
empire name is invalid…". The player is then asked "Would you like
Instructions?" (`BREINS.TXT`). Our clone reproduces the naming prompt and
its validation rule.

## Turn structure

Turns per day: 15 (config). New players get protection turns at the start
(config: 60). A turn walks through a sequence of menus:

1. Diplomacy (first turn only)
2. Status screen
3. Payment / food market
4. Covert operations (shown only when the step is enabled in Preferences and
   the player holds at least one covert agent — a fresh realm starts with none)
5. Bank
6. Spending (buy military and regions)
7. Attacks
8. Trading
9. Interplanetary operations (multi-BBS games only)
10. Messages
11. System menu (tax rate, industrial output, skip menus)

If a player is cut off mid-turn, they resume where they left off.

## Menu fidelity vs BRE

IB's menus match BRE's layout, item order, and hotkeys where practical (#17 menu
audit, verified live with `tmux capture-pane -e`). A few places diverge on
purpose:

- **Number formatting is locale-aware.** Prices and totals use the selected
  language's thousands separator (comma for English, `.` for German, space for
  Russian; see `groupSep`), so a column won't always read exactly like BRE's
  English, no-separator figures.
- **Menu label/column widths flex for translations.** German/Russian labels differ
  in length from English, so column widths can't be pinned to BRE's exact English
  spacing.
- **The Sell menu omits HeadQuarters.** BRE lists it and refuses the sale ("you
  cannot sell your headquarters!"); IB simply doesn't offer an option that always
  fails.
- **Set Industries / Change Production keeps the current % on Enter.** BRE walks
  each unit with a suggested value of **0** (pressing Enter zeroes it), so a stray
  Enter wipes the allocation; IB suggests the unit's **current** % instead, so Enter
  leaves it unchanged. Both cap the running total at 100% via a shrinking max (the
  remainder becomes gold). The order and per-unit prompt otherwise match BRE.
- **About lives in the Help browser** (keyed `A`), not on the Game/System menus,
  so `I` stays BRE's InterBBS Scores key.
- **Menu items carry a 2-space left margin** where BRE's start at column 0 — a
  minor deliberate layout choice, kept because it applies uniformly across every
  menu and reads cleanly in a terminal.
- Otherwise the region-type order (Buy Regions), the unit rows, the Price/Owned
  column colors, and the hotkey layout follow BRE.

## Elimination and restart

An empire is destroyed when its people or land reach zero (from an attack, a
weapon, or starvation). A destroyed empire is removed from the game — its owner
cannot keep playing it. As in BRE, the player builds a **new realm the next
day**: on a later day's login they are prompted to name a new realm (it need not
be the old name) and start fresh under the same handle. Voluntary abdication
works the same way.

## Interplanetary operations (IB implementation)

InterBBS ops run over file-drop packets. IB matches BRE's player-facing model
(verified against a disassembly of the original binary):

- **Group Attack** — commit real forces, not gold. Each baron sends troopers
  (deducted from their army); the pooled troopers are the strike's offense
  (1 each). BRE also lets you send jets/tanks/bombers and returns survivors;
  IB's v1 commits troopers only.
- **Terrorist Ops** — a force-destroying strike (not intel). Commit agents; the
  op is queued and resolves on the target board's next packet run. Each agent is
  one hit that removes ~1/`TerrorUnitLossDenom` (7, from BRE's disassembled 6/7
  ratio) of one randomly chosen unit type. New Realm Protection blocks it.
- **Spy** — send an agent to a remote baron; intel lands in the planet-wide Spy
  Database. Reached from the Spy Database screen.
- **SDI** — funds whole per-point steps of `SDIStep` gold up to `SDIMax` (50%,
  per BRE's "up to 50%" missile interception).

## Diplomacy

Seven treaty types are proposed / accepted / broken through the Diplomacy menu,
and each carries a gameplay effect (#11 wired the last two):

- **Full Defense Alliance** — blocks attacks between the two realms and combines
  their offense and defense (`AreAllied`, `AllianceStrength`).
- **Tariff Trade Agreement** / **Free Trade Agreement** — per-turn trade income
  scaled by population; Free earns more than Tariff (`tradeIncome`).
- **Intelligence Alliance** / **Terrorist Prevention** — lend half an ally's
  agents to your covert offense / defense (`covert.go`).
- **Technology Agreement** — a tech-sharing pact (BRE: "gain some of the
  technological advances of its partner"). A higher-tech partner raises your
  `TechLevel` ceiling to `TechAgreementCapPct` (60%) of their level, and you catch
  up `1/TechAgreementGainDiv` (1/20) of the gap each turn — so even a realm with
  little Technology of its own gains from a strong partner (`techAgreementCeiling`,
  `advanceTech`).
- **Protective Trade** — guards the two realms' trade (BRE: "preventing bombing
  of trade deals"): a partner cannot bomb the other's trade routes or trading
  market (`BombTradeRoutes` / `BombTradingMarket` refuse the op, no agent lost).
  BRE also makes trade deals "cheaper to send and maintain" — deferred until IB's
  trade deals carry costs (#17 Phase 2).

Declaration of War breaks treaties without causing internal unrest. The two
newly-wired treaties' magnitudes are IB tunables — BRE's manual gives the intent,
not the numbers.

## Trading

Local and interplanetary markets let empires specialize. Teamwork and trade are
described as the main path to winning large InterBBS games.

**General Trading Market (#17, built).** Reached via `System → Trading → Trading
Market`. Any empire can list goods for other empires to buy:

- **Goods:** troopers, jets, turrets, bombers, food, agents, tanks, carriers
  (units + food + agents; not regions or HQ). Verified live against BRE.
- **Listing escrows the goods:** setting a For Sale quantity moves that many out
  of your inventory into the market (Owned drops); `Change setup` to a lower
  quantity or 0 returns them. Columns: `Your Prices · Owned · For Sale · Total
  For Sale` (Total = the planet-wide pool).
- **Buying:** pick a good, choose a selling empire from the live listings (`Id ·
  Empire · For Sale · Price`), and buy. You cannot buy your own listing. The
  buyer pays the full price immediately (verified live — no markup); the seller's
  proceeds are deposited at **day-end maintenance** (`settleMarketProceeds` —
  BRE's "Depositing trading market money" step), minus a `MarketCommissionPct`
  tunable (default 0 — BRE's real cut was too noisy to isolate live).
- **Protection-gated:** a realm under new-realm protection cannot use the market.
- **Escrowed goods are safe from pirates and attacks** — an intended BRE strategy
  (community guide: park military to evade pirates). But listing does **not** dodge
  your own economy: escrowed **military units still cost maintenance** and escrowed
  **food still spoils**. `Bomb Trading Market` (covert) destroys a share
  (`BombMarketLossPct`) of a target's listed goods and pending proceeds.

Negotiated empire-to-empire trade deals carrying goods with demands (BRE's other
trading half) are not built yet; `Send Trade Deal` still sends gold only.
Interplanetary trades and carrier-moved goods remain future work.

## News files (what BRE broadcasts)

BRE keeps two planet-wide news feeds, populated from template files with random
flavor variants and placeholders `%F` (from/attacker), `%T` (target), `%N`
(empire), `%P` (planet), `%C` (captured amount):

- **Planetary news** (`game/news.dat`) — written when the event happens, seen
  by everyone on the planet. Categories: `NORMALWIN` / `NORMALLOSS` (regular
  attacks), `TOTALWIN` (an empire is destroyed), `NUKE` / `CHEM` / `BIO`
  (WMD strikes), `PIRATEWIN` / `PIRATELOSS` (pirate raids), `CIVILWAR`
  (collapse from poor leadership), `RIOTS` (high-tax unrest).
- **Interplanetary news** (`game/ipnews.dat`) — inter-BBS attacks: individual /
  group-on-single / group-on-whole-BBS, each with a WIN and LOSS variant, plus
  the returning-strike (`IP-RET-*`) versions and `IP-RET-KILL`.
- **Interplanetary report** (`game/ipreport.dat`) — the *private* per-player
  result of covert/special IP ops (bombing, terrorist ops, special operations,
  Sabre), not planet-wide news.

The clone broadcasts most of these to its planetary bulletin (original wording,
not BRE's verbatim lines): regular-attack wins/losses/conquests (NORMALWIN /
NORMALLOSS / TOTALWIN), nuclear/chemical/biological strikes (NUKE / CHEM / BIO),
pirate-raid outcomes (PIRATEWIN / PIRATELOSS), and tax riots (RIOTS). IP strike
results post to the bulletin too. Not yet generated: CIVILWAR (the clone has no
civil-war collapse mechanic) and BRE's finer-grained interplanetary news
categories (individual vs. group vs. whole-BBS, attack vs. return).

**Keep news prose translatable.** News lines go through the PO catalogs, so each
line must be a *whole sentence* chosen by event category, with only clearly
delimited placeholder substitutions (the `%F`/`%T`/`%N`/`%P`/`%C` style). Do not
build a line by concatenating fragments, and do not interpolate a name or number
mid-clause in a way that assumes English word order or agreement — German case
and Russian gender/number make a translator rewrite the whole sentence, and a
fragment-glued line can't be reordered at all. The safest unit for a translator
is one self-contained sentence per variant with the substitutions at the edges.
Prefer more flavor *variants* (separate whole sentences) over grammatically
clever single lines.

## How Immortal Barons differs right now

Now matching this reference (as of v0.0.1):

- Offense/defense split in combat, with the correct unit values
  (trooper 1/1, jet 2/0, turret 0/2, tank 4/4)
- Turrets (defense-only) and carriers (jets can only attack if carried)
- Bomber airfield strikes: a regular attack sends the attacker's bombers to
  destroy the defender's grounded jets first, resisted by turrets (anti-air)
  and SDI
- Interactive maintenance stage at turn start: pay armed-forces upkeep and
  region maintenance ("how much will you give?"), with underpayment causing
  desertion / revolts, plus an optional popular-support boost. Auto-Pay
  Maintenance pays it silently when affordable. (SDI upkeep, waste
  decontamination, and military morale are not yet modelled.)
- Reference net-worth values and per-unit maintenance
- Bank interest ~1% per turn, with the interest cap and 2-billion money cap
- Nuclear / chemical / biological strikes and pirate raids
- Doomer Kaboomer and SDI defense (v1 simplification: the Doomer Kaboomer is an
  instant planet-wide strike rather than the original's multi-day build/decay,
  and SDI is a flat percentage damage-reducer)
- Covert agents with spying and sabotage (success scales with agent count)
- Player mail — a BRE-style per-message reader (Reply / Delete / Ignore /
  Quit), where Ignore keeps a message for next time (it can be ignored
  indefinitely) and only Delete removes it; Reply quotes the original — plus a
  planetary bulletin
- Multiple turns per day, new-realm protection, and daily maintenance
- A rising land-market price (expansion is self-limiting)

Still missing against the reference:

- Region types (we still model land as one flat resource) and a food market
- Diplomacy (treaties) and trading between empires
- The remaining covert ops (Spy on Relations, Spy Database, Bribery) pending
  the diplomacy/database subsystems
- Leagues that end and reset with a Planetary Master
- The InterBBS (interplanetary) layer

These are the slices that would move it closer to the real game.

## Sources

- Barren Realms Elite gameplay overview — The Realm of Serion BBS:
  <https://serionbbs.com/bre-info>
- Barren Realms Elite setup and config — Synchronet wiki:
  <http://wiki.synchro.net/howto:door:bre>
- Solar Realms Elite documentation (predecessor, same lineage), Amit Patel:
  <http://www-cs-students.stanford.edu/~amitp/Articles/SRE-Documentation.html>
- Barren Realms Elite — Break Into Chat BBS wiki:
  <https://breakintochat.com/wiki/Barren_Realms_Elite>
- Barren Realms Elite manual-style overview — GameBanshee (units, regions,
  covert ops, treaties, and attack descriptions):
  <https://www.gamebanshee.com/bbs/guides/barrenrealmselite.php>
- Strategy guides (GameFAQs): Barren Realms Elite FAQs —
  <https://gamefaqs.gamespot.com/bbs/574618-barren-realms-elite/faqs>
  - "Strategies and Hints" by Robert Wennagel (unit combat/maintenance/
    net-worth tables, region strategy, covert ops): faqs/1572
  - Team Guide by Anonymous (team roles; bank-interest and food-market
    money strategy): faqs/1573
  - "Hints and Tips" by Crystal Palace / CPalace (start-up gifts,
    commanders, tax cap, pirate retaliation, trade-deal tactics): faqs/1571
  - "Cash-On-Wheels FAQ" by Anonymous (interest/money caps, region income
    figures, investment cycling, pirate evasion via the Trading Market)
