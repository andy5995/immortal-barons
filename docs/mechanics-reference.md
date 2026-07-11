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

## Attack types

- **Regular attack** — direct assault; the winner takes some of the
  loser's regions. Exact figures from BRE's `attack.hlp`:
  - **Normal Attack:** capture **20%** of the opponent's regions; both sides
    fight until they suffer **15%** losses, then retreat.
  - **Quick Strike:** fight at **110%** strength (surprise) but capture only
    **50%** of a normal attack (10% of regions); both retreat at **8%** losses.
  - **Extended Battle:** fight at **85%** strength (fatigue) but capture up to
    **125%** of normal (25% of regions); both accept **20%** losses.

  The clone implements the Normal Attack (20% capture, symmetric 15% losses);
  Quick Strike / Extended Battle are not yet offered.
- **Nuclear attack** — turns enemy regions into waste (high cost).
- **Chemical attack** — damages fewer regions but kills a lot of people
  (and troopers).
- **Biological attack** — hurts people and troopers, but not land.
- **Attack pirates** — the nine pirate factions are living bands, not a
  fixed difficulty ladder: their strength is random (any faction can be the
  strongest). Pirates raid players at random, carrying off a share of the
  victim's **troopers, jets, turrets, tanks, agents, and gold** — but never
  bombers or carriers, and never the victim's regions; the game grants a
  raiding pirate new regions instead, so a pirate that just raided is fatter.
  A single raid takes at most **24,999** of any one thing (BRE.EXE constant is
  25,000). Beating a pirate reclaims ~a fifth of its hoard per hit, so it
  takes several hits to fully recover your goods.

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

The Covert Operations menu's item order and labels are confirmed from
BRE.OVR's string table (#73):

- **Send Spy** — read the enemy's defensive strength.
- **Stir Revolts** — spread propaganda that sharply lowers popular support.
- **Set Up** — trick d and one of its Full Defense Alliance partners into
  believing the other declared war, voiding the alliance between them
  (useful against a defense pact protecting a target).
- **Support Dissensions** — agitate d's own troopers into fleeing (~10%
  trooper loss).
- **Demoralize Forces** — lower enemy military morale; they fight worse
  and, if low enough, units desert.
- **Spy on Relations** — reveal the enemy's treaties.
- **Bomb Enemy Targets** — a submenu (see below).
- **Bribery** — bribe an enemy agent inside d, so d's future covert ops
  against you auto-fail.
- **Expose Enemy Ops** — per BRE.OVR ("Bribed Agent will expose enemy
  operations for 24 Hours"), a temporary shield against *all* incoming
  covert ops. IB models the 24 hours as one game-day
  (`ExposeOpsShieldDays`).
- **Visit Bank**.

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
- **Technology** — long-term efficiency: cheaper units, lower maintenance,
  higher tax income.
- **Industrial** — produces military goods; vital when buying arms is
  disabled and you must *build* instead. Also yields gold.

Waste regions (from enemy weapons) can be cleaned for less than the cost
of new land.

**Region gold income (BRE-verified — disassembly of BRE.OVR, offsets
0x342C0–0x34A4E).** Each gold region yields, per turn,
`perRegion = yield×Rate/100 + Base`, times its region count, where `yield` is a
per-(game-day) factor in the band 1.0–1.5 (the exact BRE distribution lives in
an unmapped helper segment; IB reconstructs it as this tunable band):

| Region | Rate | Base | Notes |
|--------|-----:|-----:|-------|
| Mountain (ore) | 400 | 3,550 | smallest swing → most stable |
| Coastal (tourism) | 1,000 | 3,750 | × support factor `0.10 + 0.90·(Support/100)` — floor ~375/region at 0% support, never zero |
| Desert (solar) | 2,000 | 3,000 | widest swing |
| Industrial | 100 | 2,500 | × industry-efficiency modifier |
| River (hydro) | 100 | 5,000 | highest base; ~10% "bad-year" turns halve it |

**Urban and Technology produce no direct gold** (BRE-verified): Urban is
population housing, Technology is maintenance reduction. Food output: River ×20,
Agricultural ×5 per region. These income numbers, the caps (2B money / 1.599B
interest), the pirate caps table, and the net-worth weights are BRE-scale;
**unit prices, the tax per-capita coefficient, and the yield band are IB's own
reconstructions** anchored to this scale (BRE computes prices/maintenance inline
— not stored as constants). All tunables live in `internal/game/balance.go`.

**Population and tax** are a major income engine. A *low* tax rate (2–3%)
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
- **Food market:** you buy and sell food against a shared market whose
  price moves with supply. Guide values: sell for ~6 coins/unit, buy back
  for ~3 coins/unit. A new empire starts with about 30,000 food.
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

The bank has three actions plus two views: **Deposit Funds** (savings),
**Withdraw Funds**, **Investments**, **List Investments / Loans**, and
**View Bank Rates**.

- **Savings** earn the *Bank/Savings Interest Rate* per turn on gold in the
  bank (about 1%/turn; see the caps above).
- **Investments** are term deposits (like bonds): you choose an **amount**
  and a **number of days** (there is a **minimum term**), the gold is
  **locked**, and it **matures on a future date**, returning principal plus
  interest at the current **Investment Rate**. Before confirming, the bank
  shows "Returns expected to be approximately N." The list view shows
  columns: Date / Investments / Loans Due.
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
4. Covert operations
5. Bank
6. Spending (buy military and regions)
7. Attacks
8. Trading
9. Interplanetary operations (multi-BBS games only)
10. Messages
11. System menu (tax rate, industrial output, skip menus)

If a player is cut off mid-turn, they resume where they left off.

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

Treaty types: tariff trade, protective trade, free trade (spreads morale
between realms), terrorist prevention, intelligence alliance, technology
agreement, full defense alliance (local games only), and declaration of
war (breaks treaties without causing internal unrest).

## Trading

Local and interplanetary markets let empires specialize. Players set prices
on a general market. Interplanetary trades auto-accept. Carriers may be
needed to move traded goods. Teamwork and trade are described as the main
path to winning large InterBBS games.

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
- Player mail and a planetary bulletin
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
