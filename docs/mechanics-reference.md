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
| Trooper | 1 | 1 | Cheap. Eats a lot of food. Hurt by terrorist ops. Helps defend vs. "Sabre" attacks. |
| Jet | 2 | **0** | Offense only. High upkeep. Needs carriers (1 carrier moves 100 jets). SDI cuts jet strength 25–30%. Targeted by the "Bomb Airbases" op. |
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
  loser's regions.
- **Nuclear attack** — turns enemy regions into waste (high cost).
- **Chemical attack** — damages fewer regions but kills a lot of people
  (and troopers).
- **Biological attack** — hurts people and troopers, but not land.
- **Attack pirates** — raid pirates to gain military equipment and land.
  Retaliating after a pirate hits you gains more regions (about 8–16+)
  than striking first (about 1–5), so it is usually better to wait.
  Military parked in the Trading Market is safe from pirate raids.
- **Group vs. individual** (interplanetary) — a solo strike returns double;
  a group attack shares the returns.

Per-day caps (config): individual 4, group 4, terrorist 25, bombing 4.
"Lost" attacking forces return after 3 days (config).

## Covert operations

Success depends on how many agents you have compared to the target: more
agents relative to the enemy means a higher success rate. Keeping many
agents on hand also *defends* you against incoming terrorist ops.

- **Send spy** — read the enemy's defensive strength.
- **Bomb intelligence** — kill enemy agents, so your later ops land more
  easily. Best used first.
- **Demoralize** — lower enemy military morale; they fight worse and, if
  low enough, units desert. The most-used op.
- **Cause dissensions** — lower popular support; weakens troopers.
- **Bomb airbases** — destroy grounded jets (if the enemy's jets are home).
- **Stir emigrations** — population starts leaving; damages support.
- **Spread propaganda** — sharply reduce popular support.
- **Bomb food stores** — destroy an empire's food reserve (can trigger a
  death spiral).
- **Bomb HQ** — weaken the enemy's HQ, reducing their tank effectiveness.
- **Spy on relations** — reveal the enemy's treaties.
- **Expose enemy ops** — ~24 hours of near-total immunity to incoming
  covert operations.

Which op to use against which unit: troopers → demoralize / dissensions /
bomb food; jets → bomb airbases; turrets → demoralize; tanks → demoralize
/ bomb HQ. **Alliances:** a Terrorist-Prevention treaty adds half an
ally's agents to your defense; an Intelligence alliance adds half their
agents to your offense.

Two more local covert ops from the game manual: **Set Up** — trick two
other empires into believing they declared war on each other (voids their
treaty; useful against defense pacts); and **Bribery** — bribe an enemy
agent to learn the opponent's covert tactics, which sets up "Expose Enemy
Ops."

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
  disabled and you must *build* instead.

Waste regions (from enemy weapons) can be cleaned for less than the cost
of new land.

**Population and tax** are a major income engine. A *low* tax rate (2–3%)
drives fast population growth; late game, tax on a huge population becomes
the main income. Set tax to 0% for a few turns to spike growth, then buy
**urban** regions so people don't leave when you raise tax back to ~7–9%.
Growing population needs enough **agricultural** regions to stay fed.

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

Tax rate, bank interest, and investment rates are configurable (a real
league ran tax 85%, interest 75%).

### Concrete economy numbers (from strategy guides)

- **Bank interest: about 1% per turn** on gold held in the bank *while you
  are playing* (investments tie money up until your next login and are
  less useful for this). Our clone currently uses 5% per turn — this
  should be ~1%.
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

## How Immortal Barons differs right now

Our current build is a simplified single-player game. Against this
reference it is missing:

- Turrets (defense-only unit) and a proper offense/defense split in combat
- Correct unit values (jet = offense only, tank ≈ 4 troopers)
- Region types (we model land as one flat resource)
- Nuclear / chemical / biological / pirate attacks (stubbed)
- Covert operations, diplomacy, trading, messages (stubbed)
- Multiple turns per day and protection turns
- A food market and a land market with moving prices (we use fixed prices)
- Correct bank interest (~1% per turn, with the ~1.6-billion interest cap)
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
