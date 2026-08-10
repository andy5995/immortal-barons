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
| Tank | **3–5** | **3–5** | Best all-round. Low upkeep, high buy cost. Strength scales with **HQ** (3 at 0%, 4 at 50%, 5 at 100%) and with morale. Helps defend vs. chemical missiles. The guide's flat "4" is the HQ-50 value — see HeadQuarters below. |
| Bomber | 0 | 0 | Carries bombs / special-ops; destroys enemy *grounded* jets when sent in an attack. |
| Carrier | 0 | 0 | Support: moves jets to battle and goods for trade. |
| HeadQuarters | — | — | Raises tank effectiveness; enemies bomb it to weaken your tanks. |

So combat must track a separate **offense score** and **defense score**,
not one shared value. Jets add only offense; turrets add only defense.

### HeadQuarters (BRE-verified — disassembly + `breins.txt`)

HQ is an `int32` at empire record `+0x26b`, holding percent complete.

- **Buying it sets the field to 5** (`BRE.OVR 0x12C8C`), after refusing when it
  is already above 0. Confirmed live: bre-3 prints "You have started work on your
  HeadQuarters." and the buy menu's `# Owned` column then reads 5. That column
  *is* the completion percent — it reads 100 on a finished HQ.
- **It advances +5 at end of turn while it sits in 1…99, then clamps to [0,100]**
  (`0xD010`). The buying turn's own end-of-turn advance counts, so it finishes
  19 turns after the purchase.
- **A tank's strength is `1.5 + HQ/100`** where a trooper is `0.5` and a
  jet/turret is `1.0` (`0x40241`, `0x4043D` — the float constants decode exactly
  to 1.5, 100.0, 0.5, 175.0, 2.0). In whole troopers that is **3 at HQ 0, 4 at
  50, 5 at 100**. `breins.txt` calls a tank "about the equivalent of four
  Troopers" — the HQ-50 value, which is how the manual and the binary reconcile.
  IB used 4 rising to 8 until 2026-07-30.
- The same expression scales the whole sum by `0.5 + morale/175` and divides by
  2. IB's morale curve (0.5 → 1.0, versus BRE's 0.5 → 1.07) is left as is: the
  constant ÷2 and the small top-end difference cancel between attacker and
  defender.
- Bombers are excluded from the sum and accumulated separately, matching
  `breins.txt` ("no offensive or defensive strength").
- **The price rises with the empire's lifetime turn count.** The military units
  walk around the centre of a fixed band; a HeadQuarters only ever climbs (5,039
  … 12,649 across the captures). Covert agents price the same way — see the
  covert-agent entry under the price walk below. `BRE.OVR 0x128BA`:

      price = min(5000 + 75 x turnsPlayed + Random(300),
                  100000 - Random(1000))

  The turn counter is its own `int32` at record `+0x281` — not the Score, which
  merely happens to track it at a flat 213/turn. Checked against 163 distinct
  captured prices over seven captures and two empires: 161 fall inside the
  300-wide window and are spread flat across it; the two strays are one turn's
  worth out, from a day-boundary off-by-one in the score-to-turns derivation used
  to check them. So a HeadQuarters rewards committing early, and the cap stops a
  very old realm being priced out. IB mirrors this with `Empire.TurnsPlayed` and
  `World.HQPrice` (`HQPrice*` in `balance.go`); it charged a flat 5,104 until
  2026-07-30.

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

Maintenance per unit per turn, at the sysop's low / medium / high setting. The
**Medium column is measured**, not taken from the guide: a controlled capture
held one unit type at a time with no Technology reduction and read
"Your Armed Forces Require" each turn —

| held | charged | rate |
|------|---------|------|
| 100 / 50 troopers | 40 / 20 | 0.40 |
| 20 / 60 jets | 24 / 72 | 1.20 |
| 20 / 60 turrets | 18 / 54 | 0.90 |
| 20 / 40 tanks | 12 / 24 | 0.60 |
| 20 bombers | 26 | 1.30 |
| 20 carriers | 2 | 0.10 |

The guide is right on every row **except troopers**, where it prints 0.60 against
a measured 0.40 (seen in three separate games). Trust the measurements, not the
table.

| Unit | Low | Medium | High |
|------|-----|--------|------|
| Trooper | 0.10 | ~~0.60~~ **0.40 measured** | 1.60 |
| Jet | 0.30 | 1.20 | 4.80 |
| Turret | 0.255 | 0.90 | 3.60 |
| Bomber | 0.32 | 1.30 | 5.20 |
| Tank | 0.15 | 0.60 | 2.40 |
| Carrier | 0.025 | 0.10 | 0.40 |

Net-worth value contributed per unit — **binary-verified** 2026-08-01, read out of
`056d:0F43` (`BRE.EXE 0x8F53`), the function the See Scores screen calls for its
Net Worth column:

| Item | Value | Item | Value |
|------|-------|------|-------|
| Trooper | 0.250 | Tank | 1.250 |
| Jet | 0.325 | Bomber | 3.000 |
| Turret | 0.425 | Carrier | 1.000 |
| Agent | 0.500 | Region | 12.50 |

Every weight matches what IB already had from the strategy guide, to the digit.
Bombers and carriers are integer multiplies (3 and 1); the rest are Turbo Pascal
6-byte reals, and the region term multiplies the same nine-region sum
(`056d:0EC6`) the Territory column uses. Three further details:

- **There is no debt subtraction.** IB subtracted `Debt/100` until 2026-08-01 and
  no longer does. This slightly raises the loan ceiling, which is a multiple of
  net worth.
- **Units away from home still count.** Each unit term adds a second count from a
  parallel array at record `+0x211` (troopers, jets, turrets, bombers, …, agents,
  tanks, carriers, 4 bytes apart) before applying the weight, so a realm with a
  strike in flight does not look poorer for it. **Not implemented in IB** (#96):
  an inter-BBS detachment is subtracted outright by `commitForce` and restored
  when the result packet returns, so net worth dips for the round trip. What
  fills BRE's array is **not read** — only the routine that scales it down with
  losses (`BRE.OVR 0xC358`) and this one, which reads it.
- **A dead realm is worth 0** rather than a computed figure. IB does this on the
  scores screen rather than inside `NetWorth`, because IB's combat math reads
  `NetWorth` too (capture density) and a zero there would change battles rather
  than a display.

A vestigial `+0x125` is added at the end. It is read here and **nowhere else in
either binary, and never written**, so it is always zero.

*Unreconciled:* pairing the See Scores figure 4,526,733 in
`cap/eots-covert-agents.cap` with the unit counts on screen around it gives about
464,000, roughly a factor of ten out. The disassembly is not in doubt — the
weights are literal constants — so the mismatch is in how that screen's snapshot
lines up with those counts (it refreshes daily, and its "Territory" column does
not equal the Spending menu's regions-owned). Worth settling before treating any
absolute net-worth figure as calibrated.

### Maintenance payment flow (BRE-verified)

Captured live from BRE (fresh realm, Auto-Pay Maintenance off). With Auto-Pay on
and enough gold on hand, all maintenance is paid silently. Otherwise the manual
flow runs in this order:

1. **Visit the bank? (y/N)** — opens the bank so a baron short on hand can
   withdraw savings to cover upkeep.
2. **Armed-forces upkeep**, then **region maintenance** — each a "how much will
   you give?" prompt. The prompt's max is the amount **required** (you cannot
   overpay); if you can't afford it, the max is your gold.
3. **Crown tax** — a per-turn tax to the Queen Royale (a non-player NPC monarch);
   its prompt max is your available gold. The gold is not destroyed: it goes into
   a planet-wide purse the Queen refunds out of (see **The Queen Royale's tax
   refund** below).
   The amount is **binary-verified**:
   `crownTax = trunc(goldEarnedThisTurn × PlanetaryTaxRate / 1000)`, where the
   rate is a sysop field in tenths of a percent (default 50 = **5% of the turn's
   gross gold income**, editor maximum 200). The base is that turn's six income
   lines only — not gold on hand, bank, net worth, score, or unit value. It
   reproduces 28 of 28 live charges exactly.

   **The industrial-gold line is in the base — capture-confirmed**, not only read
   from the binary. It matters because a baron who sells manufactured units
   instead of taking industrial gold pays no crown tax on the proceeds: a unit
   sale is a spending-menu transaction, not income. At a 10% rate that is a 10%
   edge to produce-and-sell before any other consideration.

   **Underpaying is allowed** and costs **popular support**, applied at day
   rollover, by
   `trunc((1 − (paid+1)/(required+1)) × 15)` — a ceiling of 15 that the +1s stop
   it from ever reaching (paying nothing costs 14). No land loss, no unit loss,
   and the debt is not carried forward.

   **Live-confirmed:** paying 0 dropped support 100→86, then 100→86, 86→72 and
   72→58 across four turns of a captured game — exactly 14 each time, matching
   the formula with no fitting. (The prompt's low number is a suggestion, not a
   floor; typing 0 is accepted.)

   For calibration, BRE's three shortfall penalties: forces ×40 against
   **morale**, regions ×50 against support, crown tax ×15 against support.

   **IB implements this**, including the deferral: the penalty accumulates during
   maintenance and lands at turn rollover, so the drop surfaces on the next turn's
   display as it does in the original. One deliberate divergence — the rate is
   stored as a whole percent (default 5, maximum 20) rather than BRE's tenths, so
   the config editor and the stored value use one unit rather than two.

   One original bug not copied: BRE's *region* shortfall computes
   `1 − ratio×50` rather than `(1 − ratio)×50`, which goes negative for any
   shortfall under 98% and would *raise* support. The forces and tax branches use
   the correct ordering; IB follows those.
   ### The Queen Royale's tax refund (#93)

   The crown tax is a redistribution, not a sink. Every gold **actually paid**
   (not the sum demanded) is banked in a planet-wide purse — an `int32` in the
   config record at `+0x24`, seeded 100,000 at game init — and the Queen hands a
   share of it back. **Fully binary-verified** (`BRE.OVR 0x18280`, called once
   from `BRE.EXE 0x61dd`; the feed is at `BRE.OVR 0x2FAF1`):

   ```
   rate    = 0.02, or 0.07 once purse > 100,000,000
   payout  = trunc(purse × rate)          capped at 1,000,000 while protected
   purse   = trunc(purse × (1 − rate))
   ```

   **When it fires.** The routine holds no `Random()`, so it pays whenever it is
   called, and it has one caller: the "since your last play" recap, behind two
   tests — the turn-stage counter (`+0x2b8`) is zero and `turnsRemaining ≥
   turnsPerDay`. That is **no turn part-way through and none taken today**: the
   player's first play of a game day. It is not a random event. The routine
   called immediately after it is the **lottery**, which confirms the
   first-play-of-the-day event block the lottery entry below predicts.

   **The cap is a newcomer guard.** It applies only when a per-empire predicate
   holds; that predicate (`056d:19b5`) compares the realm's lifetime turn counter
   (record `+0x281`) against the config's **Turns Of Protection** (`+0x38`) — it
   is BRE's own "is this realm still under New Realm Protection?" test, the same
   one that prints "Our empire is in protection, my lord." on a covert op. So a
   newcomer to a mature planet is held to 1,000,000 while an established realm
   takes the full share. Captured payouts run 354 to 14,000,000, with eighteen at
   the cap.

   **Because the purse is read fresh each time**, the first baron to play on a
   given day takes the largest cut and everyone after them draws on what is left.

   **IB implements this**, constants in `balance.go`. One divergence: IB pays
   exactly 1,000,000 where BRE usually pays 999,999. BRE caps by substituting
   `1000000 / purse` for the rate and multiplying back by the purse; the round
   trip through a six-byte real loses the last unit. That is its float format,
   not a rule.

   IB expresses the trigger as a per-day flag on the empire (`RefundTaken`,
   cleared with the other daily counters) rather than re-deriving BRE's two
   tests, since it has no turn-stage counter to test — `TurnProgress` is a set of
   named flags, not a 0–20 counter. The two agree on every path traced through
   the original.

4. Conditional: SDI maintenance (with SDI), waste-region decontamination (with
   waste regions), then the popular-support and military-morale boosts (shown
   only below 100). Support/morale are *requested* (optional), not required.
5. **Reconsider gate** — underpaying any *required* cost warns of disastrous
   results and offers to reconsider. Yes **restarts the whole sequence from the
   bank prompt**; No proceeds, with desertion/revolt for the shortfall.

Prompt colors (from a color capture): text plain white; the required and
suggested amounts bright cyan, the max dark cyan, the `(…; …)` parens bright blue.

IB implements steps 1, 2, 3, the SDI and support/morale parts of 4, and 5, with
the required-capped prompts and these colors. Waste-region decontamination is not
built. The SDI prompt is `Your SDI Program requires N gold.`, asked after the
region maintenance and before the crown tax (live capture).

**Method — the Auto-Pay line is a better probe than the prompts.** With Auto-Pay
Maintenance on, BRE collapses the whole sequence into one `N Gold paid.` line.
That single number is *more* useful than the itemised prompts, because every
component has to reconcile against it at once:

```
Gold paid = RegionUpkeepPerRegion × regions      (constant for a given realm)
          + perUnitMaint × units held
          + trunc(turn income × PlanetaryTaxRate / 1000)
```

Two unknowns can be separated by playing turns where one of them moves. This is
how the industrial-gold question above was settled: over ten captured turns,
assuming industry **is** taxed leaves a region-maintenance figure of exactly
913.000 per region on every turn across three different region counts (4,417 →
4,917 → 5,417), while assuming it is **not** leaves a figure that drifts with
income (974.8, 975.0, 981.3, 991.6…). A constant that survives a changing
denominator is the signal; see also the standing warning about checking whether
the denominator moved before explaining any per-unit figure.

The same decomposition recovered per-unit maintenance as a by-product: on the
turret-producing turns the per-region figure sits at 920.375 rather than 913, and
the gap is 44,392 turrets × 0.9 — matching the Medium maintenance table.

Both figures were later confirmed **directly**, without the decomposition, by a
capture taken with Auto-Pay off: the itemised `N gold is required to maintain
your regions` line divides exactly by 913 at four empire sizes from 15 regions
(13,695 gold) to 6,837 (6,242,181), across two separate games and every region
mix — so the rate is flat, with no dependence on type or size. On the same
capture a turret-only army was charged exactly `trunc(0.9 × turrets)` at four
sizes up to 219,032 turrets (197,128 gold), confirming the Medium table's
figures are literal gold rather than a ratio.

Region upkeep is the dominant drain: that 6,837-region empire owed 6.2M on land
against 197k on a 219,032-strong army. IB charges both figures as of the
constants in `balance.go` (`RegionUpkeepPerLand`, `Maint*Tenths`); before that it
charged 2 gold per region and ten times BRE's per-unit rate, which made expansion
effectively free and armies disproportionately expensive.

IB's config field **Max Tax Rate** is *not* BRE's Planetary Tax Rate, despite
having started life as a misreading of it. BRE caps nothing — its own prompt
offers `New Tax Rate [0-100]` regardless of that config value. IB keeps a player
cap as a deliberate divergence (ceiling `MaxPlayerTaxRate`, 50), separate from
the crown tax rate (`PlanetaryTaxRate`).

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
  × loser regions × density factor)`, scaled by Attack Rewards. The **density
  factor** is IB-original (no BRE-verified formula): the attacker's
  net-worth-per-region over the defender's, clamped to `[CaptureDensityMin,
  CaptureDensityMax]` = 50–200% (`CaptureDensityBase` 100% at equal density). A
  defender whose net worth is spread thin over its land (cheap, lightly-held
  regions) bleeds up to 2× the base; a denser, developed realm as little as half.
  Public strategy guides describe the *tactic* of preying on high-region,
  low-net-worth targets, not a number, so IB supplies its own — expect tuning.
  The base rate comes from a **live region-count sweep**
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
  pirate raids are not limited by it. An **individual interplanetary attack**
  (BRE's "Indiv. Attack Force", #62) draws on the same allowance: one baron
  striking one named baron on another planet, leaving at once rather than
  assembling like a group attack. Group attacks, terrorist ops and bombing ops
  have their own separate per-day allowances.
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

**Clingy Annihilator** (the clone's equivalent of BRE's *Gooie Kablooie*) — the
ultimate weapon, aimed at an entire enemy planet rather than one empire, and one
per planet at a time. IB implements the original's lifecycle (#16): begin
construction against a named planet → any baron funds it a million gold at a time
→ complete → awaiting launch → in flight → arrival. Its creator may dismantle it
instead, and the gold is not refunded.

The **funding cost is binary-verified** from BRE.OVR's overlay unit at 0x27441
(the routine at 0x277A0-0x27950), in millions of gold:

    cost  = round(targetPlanetLand x 0.0044743) + 100     { land floored at 1000 }
    cost  = min(cost, 5000)
    ratio = targetPlanetLand / ourPlanetLand
    cost  = cost x 2.0   if ratio > 4
            cost x 1.5   if ratio > 2
            cost x 1.2   if ratio > 1

so the weapon is priced against how much bigger the target planet is than yours.
The constants are `Clingy Annihilator*` in `balance.go`.

The rest is **IB's reconstruction**, following the original's prompts rather than
its code: two days in flight, 10% of each realm's regions on arrival scaled by how
much of the weapon survived, and interception by **jets only** (the original is
explicit that nothing else can reach it) at one percent knocked off per 250 jets,
spent whether they connect or not. SDI reduces the damage.

**That reconstruction is known to be wrong in two ways** — it needs a live game,
or the original's code, before it can be called done:

- **The weapon should not detonate once and vanish.** The original's own
  instructions describe a siege: 10% of the planet's regions instantly on
  arrival, "every day of it's existance after the first day, another 5% ... up
  to a max of 5 days at which time it will self-destruct", with jets battling it
  the whole time it sits there. IB has no post-arrival phase at all, so the
  weapon costs a planet a tenth of its land instead of up to a third, and the
  cooperative defence the original is built around never happens. (#112)
- **SDI should not blunt it.** The original says jets are "the only way to
  destroy this thing", and the doomsday weapon is not among the three things SDI
  is documented to work on. IB subtracts the defender's SDI anyway, which
  contradicts the interception note in the line above. (#111)

The target planet is told about it — while it is under construction and again in
flight, with the arrival time in hours — which is what makes interception
possible (#63). BRE does the same, and its own strings carry the wording:
"Gooie Kablooie destined for our planet is under construction at ...",
"Gooie Kablooie arrives from ... in N Hours."

**SDI Defense** — a funded anti-missile/anti-jet shield. The original names
three separate ceilings: it destroys **up to 50%** of incoming missiles, and
reduces attacking **jets by up to 30%** and **bombers by up to 20%**
(`game/breins.txt`). See "The SDI program" below for how it is funded, and
"What the shield actually does is not verified" for how much of that IB has.

Per-day caps (config): individual 4, group 4, terrorist 25, bombing 4.

**Gold costs on these menus are a gap.** BRE prices a terrorist op in the
InterPlanetary Operations menu's own cost column, and prices four of the Special
Operations entries on theirs (figures in `docs/dev/bre-screens.md`). IB charges
gold for neither: the `TerrorCosts` setting is editable, broadcast and displayed,
but nothing consumes it. The terror figure grew over the capture and three
readings do not establish what it scales with.
"Days before 'lost' forces returned" (`Config.LostForcesDays`, default 3) is an
**inter-BBS** setting, not a local-combat one. A strike sent to another board is
away for the whole packet round trip, and packets go missing; the setting gives a
detachment back to its owner when no result has come home in that many days. IB
implements this (#96): `World.InFlight` records every strike that leaves, a
returning result clears it, and `ReturnLostForces` — run from the planetary step
after inbound packets are applied — hands back anything that has waited too long
and posts news. 0 turns the recovery off.

## Covert operations

Success depends on how many agents you have compared to the target: more
agents relative to the enemy means a higher success rate. Keeping many
agents on hand also *defends* you against incoming terrorist ops.

**What the binary actually does — Send Spy (`BRE.OVR 0x4BA48`, reached through
overlay stub `0x4DC:0x3E`).** Read 2026-08-01. `a` is the attacker (the current
player, the global empire letter at `DS:0x28DC`); `d` is the chosen target;
`kind` is a difficulty divisor the caller passes, and Send Spy passes 1:

    if bribed[d][a] > threshold then
        if Random(10) <> 0 then { caught; fail }        { 90% auto-fail }
    r := Random(10)
    if r = 0 then success                               { 10% auto-succeed }
    if r = 10 then { caught; fail }                     { dead code: Random(10) is 0..9 }
    A := covertStrength(a, 0) div kind
    B := covertStrength(a, 1)
    if allied[a][d] = 1 then A := A * 2
    while A > 32767 or B > 32767 do begin A := A div 2; B := B div 2 end
    if Random(A + B) < A then begin score[a] := score[a] + Random(30); success end
    else { caught; fail }

    covertStrength(e, mode):
        total := e.Agents
        for x := 'A' to 'Y' do
            if e.treaty[x] = 4 and mode <> 0 then total := total + x.Agents * 0.5
            if e.treaty[x] = 5 and mode = 0  then total := total + x.Agents * 0.4
        if total > 1e9 then total := 1e9
        result := trunc(total)

**IB reproduces this for Send Spy** (`spySuccess` in `internal/game/covert.go`),
defect included, on Andy's call. The effect ops keep `covertSuccess` — its
attacker-against-defender roll — because BRE does not resolve them through this
routine at all, so extending the defect to them would invent a bug the original
does not have. Both paths now share `covertStrength` and so pick up the verified
40/50 ally shares. What differs between the two:

- **The defender's agents never enter the roll.** Both `covertStrength` calls
  pass `a`; the bytes are identical (`8A 46 10`) at `0x4BAE7` and `0x4BB03`, only
  the mode byte changes. With no alliances `A = agents div kind` and
  `B = agents`, the agent count cancels, and Send Spy is a flat
  `0.1 + 0.9 x 1/(1+1)` = **55%** whatever either side holds. The same function
  is called correctly elsewhere (`0x4AA5E` passes a different empire with mode 1),
  so the Send Spy site looks like a copy-paste bug in the original rather than a
  design.
- **The two alliance weights are not equal**: 0.5 on the defending side, 0.4 on
  the attacking one. IB lent half in both directions until this was read. Which
  treaty number is which is inferred from the documented effect (an Intelligence
  Alliance helps the attacker, a Terrorist Prevention treaty the defender), not
  read from a name table.
- **A Terrorist-Prevention treaty raises `B`**, which lowers the holder's *own*
  spy success — a direct consequence of the defect above.
- **One roll in ten succeeds before any of this**, and a bribed attacker still
  slips through one time in ten instead of failing outright. A fourth branch
  (`r = 10`) is dead code: `Random(10)` returns 0..9.

Two more things read here, **not** reflected in IB: an effect op **spends an
agent up front** (`BRE.OVR 0x17957` decrements agents before resolving) where IB
charges it only on failure, and it records itself in a per-op byte at record
`+0xFD + op`, which is how "Limit one try per turn!" is enforced. How the effect
ops resolve is still unread.

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
- **Desert** — solar income; swings widely (3,000–5,000 per region — the
  widest band of any region).
- **River** — the highest and steadiest gold of any region (hydroelectric), and
  it grows a little food alongside it every turn. In BRE a river does one or the
  other; IB pays both (see Rivers below).
- **Agriculture** — grows food; food self-sufficiency.
- **Urban** — more population, tax revenue, and trade capacity.
- **Technology** — long-term efficiency. Produces no direct gold. See the full
  mechanic below; IB currently models it as a single accumulating `TechFactor`
  percent, which is the right *shape* but not the original's structure.
- **Industrial** — produces military goods; vital when buying arms is
  disabled and you must *build* instead. Also yields gold.

Waste regions (from enemy weapons) can be cleaned for less than the cost
of new land.

**Region gold income (BRE-verified — disassembly of BRE.OVR, offsets
0x342C0–0x34A4E).** Each gold region yields, per turn,
`perRegion = Base + Random(Rate)` — a uniform integer draw over the **whole**
of `[0, Rate)` — times its region count. So Base is the floor and Rate is the
full width of the swing. Each region type draws **independently**; there is no
planet-wide year factor (in one live turn desert drew near the top of its band
while agriculture drew zero).

The draw is over every integer in the range, not a coarse percentage: one live
ore turn came out at Base + **279**, which is not a multiple of Rate/100.

Live-verified across four region types in two separate games, expressed as a
share of Rate: desert 2.6–99.8%, tourism 0.4–99.8%, ore 0.5–99.8%, hydro
2.0–99.0%. Two earlier readings were wrong and are recorded here so they are not
re-derived: a reconstructed 1.0–1.5 multiplier ran high, and a later 0.30–0.80
band came from 12 turns — the sample size at which any uniform looks narrow.

| Region | Rate | Base | Notes |
|--------|-----:|-----:|-------|
| Mountain (ore) | 400 | 3,550 | smallest swing → most stable |
| Coastal (tourism) | 1,000 | 3,750 | × support factor `0.1 + 0.9·(Support/100)` — floor 375/region at 0% support, never zero. **Binary-verified:** the three constants decode to exactly 100.0, 0.9 and 0.1 in the coastal-only float sequence, so this is read from the original rather than fitted. An earlier headless sweep (#31) fitted `0.099 + 0.901·(Support/100)` — that was fit noise around the round values |
| Desert (solar) | 2,000 | 3,000 | widest swing |
| River (hydro) | 100 | 5,000 | **Binary-scale-verified** — 44 captured figures all divided exactly by the river count, 5,002–5,099. IB pays `(100 − RiverFishShare)%` of this as gold and the rest as food (#29) |

**Industrial** regions run **two separate pools**, both binary-verified against
BRE.OVR:

- **Gold** — `yield×55/100 + 2,500` per region, paid on the **unallocated %**
  only (so allocating everything to units earns no industrial gold).
- **Units** — a flat **2,100** points per region, unaffected by the yield draw,
  which is why unit output is identical turn to turn while gold varies.
  Costs: trooper 100 / jet 140 / turret 150 / tank 500 / bomber 1,500 /
  carrier 1,750. Specialize: ×1.25 to the chosen unit, ×0.85 to the rest.

The unit pool is multiplied by the **Mountain boost**,
`min(1.5, 1 + 3 × Mountain / TotalRegions)` — mountains raise manufacturing by
up to half, but the boost is a *share* of the realm, so it dilutes as an empire
expands elsewhere. BRE keeps the whole chain in floating point and rounds once at
the end.

## Technology (binary-verified)

Read from the original binary and confirmed against live play. **IB implements
this**, including the 15 slots, the random distribution, and the freeze.

**It is fifteen counters, not one level.** The empire record holds 15 research
slots. Only **six** do anything; the other nine are pure dilution, so 9 of every
15 research points are wasted.

**Research per turn**, run just before the income phase:

```
if techRegions <= 0:  nothing happens at all
points = 4 × round( (techRegions² / totalRegions)^0.75 )
  + for each ally who also holds Technology:
        round( (min(myTech, allyTech)² / totalRegions)^0.75 )     [not ×4]
each point:  slot[random 0..14] += 1
```

So research is **quadratic in Technology regions and only inverse-linear in
realm size** — it rewards a large tech block in a small realm.

**Effect of a level**, per slot:

```
factor = 1 + (cap − 1) × (1 − exp( −level / (totalRegions + 1) ))
```

| effect | cap | applied as |
|---|---:|---|
| food decay | 5.0 | `100/factor` |
| food production | 2.0 | `100×factor` |
| SDI funding | 2.0 | `100/factor` |
| gold income | 1.5 | `100×factor` |
| population tax | 1.5 | `100×factor` |
| military strength | 1.4 | `100×factor` |
| maintenance costs | 1.4 | `100/factor` |
| unit production | 1.35 | `100×factor` |

Food decay has by far the largest ceiling, which is why it moves first and
furthest — at low levels Technology is effectively a spoilage technology.

**There is no decay.** The binary has exactly one write site and it only
increments. Selling Technology regions makes research stop; the level itself
**freezes permanently**. Confirmed live: a realm that dropped to zero Technology
regions held identical advisor percentages across four further turns.

**But the benefit dilutes as the realm grows**, since total regions is the
denominator — which is what the in-game note means by "larger empires need more
advanced technology to maintain the same efficiency." Confirmed live: with the
level frozen at zero Technology regions, buying 500 regions moved food decay from
72% to 89%, matching the formula's prediction exactly.

Together those two properties mean a player can bank research cheaply while
small, then liquidate the regions and keep the benefit — but cannot expand
afterwards without giving most of it back. **IB keeps this behaviour
deliberately**: the exploit is self-limiting, because a realm that stays small to
preserve its technology pays for that in income, military and land.

**Technology Agreement (#11).** A partner adds an unmultiplied research
contribution, bounded by whichever side holds *less* Technology. So the pact
accelerates a realm that is already researching and does nothing for one holding
no Technology at all — IB previously let a tech-less realm inherit a partner's
level, which the original does not.

**Urban and Technology produce no direct gold** (BRE-verified): Urban is
population housing, Technology is an efficiency multiplier (see the Technology
region above). Food output: `Agricultural × 300` grown, then raised by the
Technology factor (#20); rivers add a share of their yield as food every turn,
see the Rivers section. These income numbers, the caps (2B money / 1.599B
interest) and the pirate caps table are BRE-scale, and the net-worth weights are
binary-verified; **the tax per-capita coefficient and the yield band are IB's own
reconstructions** anchored to this scale. All tunables live in `internal/game/balance.go`.

**Per-turn price walk (#30), binary-verified.** Every empire stores its own price
for each of the six military units (`Empire.Prices`) and steps it once per turn
(`World.stepPrices`, from `PlayTurn`). BRE's walk, at `BRE.OVR 0x12633`:

    up, down := 1, 1
    if price < mid  then down := Random(3) + 1
    if price > mid  then up   := Random(3) + 1
    price := price + Random(step) div up
    price := price - Random(step) div down
    if price < lo   then price := lo + Random(60)
    if price > hi   then price := hi - Random(100)

`mid` is the midpoint of `[lo, hi]`. Whichever side of it a price sits on, the
move that would carry the price further out is divided by 1..3 while the move
back is taken in full, so the walk is **mean-reverting** and prices sit near the
centre of a band they are free to roam.

The three tables are arrays of six words in `BRE.EXE`'s data segment (`DS:0x508`
step, `DS:0x514` low, `DS:0x520` high):

| Unit | Low | High | Mid | Step |
| --- | ---: | ---: | ---: | ---: |
| Troopers | 200 | 350 | 275 | 25 |
| Jets | 250 | 400 | 325 | 25 |
| Turrets | 300 | 475 | 387 | 25 |
| Bombers | 2,500 | 5,000 | 3,750 | 125 |
| Tanks | 1,125 | 2,250 | 1,687 | 75 |
| Carriers | 4,750 | 6,000 | 5,375 | 125 |

Checked against `cap/eots-covert-agents.cap`, 30 consecutive turns: every buy
price is inside its band, the observed spread stays close to mid (troopers
226–305, carriers 5,257–5,457), and no turn-to-turn move reaches the unit's step.

BRE seeds a new empire's prices deliberately outside the bands (`BRE.EXE 0x8DEC`:
troopers 1,000, jets 15,000, turrets 1,400, bombers 2,000, tanks 6,000, carriers
18,000), so the first walk step snaps each one to a band edge. IB seeds at the
floor instead, which lands in the same place for bombers and one clamp lower for
the rest; it washes out within a few turns either way. The same initializer
confirms the starting realm (3 Coastal, 2 Agricultural, 5 Desert, 5 Mountain, 100
troopers, morale and support 100) and sets each of the six Set-Industries
allocations to 15%.

The stored value is what the Spending menu shows and what a buy/sell charges
within the turn (shown == charged; buy and sell route through the same accessor,
sell = buy/3 truncated), and it persists across days via the save. Steps are
deterministic (keyed per empire and turn, the same `GameDay`/`TurnsLeft` basis
region yields use) so play is reproducible and concurrency-safe. The walk is
per-empire, AI empires included — each is keyed on its own name.

**Covert agents do not walk.** Their price ratchets with the empire's lifetime
turn count instead, the same shape as a HeadQuarters and read off the same
counter (`BRE.OVR 0x1293E`, record `+0x281`), with no cap:

    price = 450 + 20 x turnsPlayed + Random(300)

So a realm 100 turns old pays about 2,450–2,749 per agent where a new one pays
450–749, and the price never comes back down. Pinned by the same capture:
solving each turn's agent *and* HeadQuarters price for a shared turn count leaves
exactly one integer feasible per turn, and it advances by one every turn, day
boundary included. Constants are `AgentPrice*` in `balance.go`; **agents sell at
a flat 100** (`SellAgentPrice`, the literal at `BRE.OVR 0x16AEB`), not buy/3.

**Regions do not walk either** — their price rises purely with holdings
(`917 + owned×33`), which BRE held exact every turn.

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
BRE starts every type at 15%, leaving 10% to fall through to industrial gold.
**IB spends the whole pool on units**: jets 16%, carriers 2%, turrets and tanks
21% each, troopers and bombers 20%. Because a type's output is `pct/cost`, a
flat split builds one carrier per 12.5 jets while a carrier lifts 100 — IB's
share makes the two rates meet exactly (`DefaultProdPct` and friends in
`balance.go`).
A common money tip: set industry to 100% carriers and *sell* the carriers
— more profitable than producing gold directly. Mountain regions boost
industrial output (see the region table for the formula) and are the most
war-stable.

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
popular support / improve military morale"). Underpaying maintenance lowers
them. Low popular support cuts Coastal income and, at the extreme, brings riots.
Low military morale scales down combat effectiveness, and below a threshold
troops desert each turn.

**The support boost — binary-verified (BRE.OVR 0x2F4C4 and 0x2F740):**

```
deficit = min(100 - Support, 15)
cost    = deficit × (3 × People + 500)          # People in millions
points  = deficit × (given + 1) / (cost + 1)    # truncated
maximum payable = cost × 3 / 2
```

Reproduced exactly by two live prompts: 216,366 gold at 23,874M people and
218,139 at 24,071M, both three points short, each restoring exactly 3 points.
Note the deficit **charged for** is capped at 15, but the award is a plain ratio
of what you paid — so **overpaying by half buys 22 points, not 15**. That is the
original's behaviour, not an IB addition. The `+1` on each side is the same shape
the crown-tax penalty uses.

**Military morale's** request and cap (`MoralePerBoostGold`,
`MaxMoraleBoostPerTurn` in `internal/game/payments.go`) are **not** verified —
they remain reconstructed placeholders, tunable.

**Riots and emigration — verified against a BRE.OVR disassembly (HIGH confidence):**

- **Riot trigger + chance:** each turn a riot fires iff `tax > 10` **and**
  `tax*tax >= Random(10000)` — i.e. **riot probability = tax² / 10000**
  (quadratic, not linear). Samples: tax 15 → 2.25%, 20 → 4%, 30 → 9%,
  50 → 25%, 71 → ~50%, 100 → 100%.
- **Riot effect:** each riot removes **`People div 15`** (~6.67%) of the
  population and docks **`tax div 3`** popular support. (An earlier reading had
  the `tax div 3` term cancelling population growth — wrong; it is the support
  penalty, confirmed by live capture below.)
- **Support drifts with the tax rate every turn, riot or not:**

  ```
  Support = clamp(Support - riotPenalty - (tax - 30) / 10, 0, 100)
  if Support < 10:              Morale -= (10 - Support)
  if tax < 10 and Support < 85: Support += (10 - tax)
  ```

  Integer division truncates toward zero, so a rate **below 30 recovers support
  for free** (+1/turn at tax 12–29) and one above 40 bleeds it. This is why a
  low-tax realm sits pinned at 100. **Live confirmation:** at tax 12 a riot took
  support 100 → 97 on two separate turns — exactly `100 − 12/3 − (12−30)/10`.
  Both turns also show that riots at a *low* rate are real, just rare: 12² /
  10000 = 1.4% a turn.
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

The clone implements all of the above (`internal/game/turn.go`): the trigger and
chance, the `People div 15` loss, the `tax div 3` support hit, the per-turn tax
drift, the low-support morale drain, and the low-tax buy-back. IB's earlier
invented model — a drift toward `100 − (tax−15)×3` plus a free 5-point boost for
a "well-run realm" — has been removed; the tax drift covers free recovery, as it
does in the original.

Tax rate, bank interest, and investment rates are configurable (a real
league ran tax 85%, interest 75%).

### Concrete economy numbers (from strategy guides)

- **Bank interest: about 1% per turn** on gold held in the bank *while you
  are playing* (investments tie money up until your next login and are
  less useful for this). The clone's Interest Rate knob is anchored so its
  default (50) yields ~1%/turn, matching this.
- **Interest cap: 1,599,999,999.** Gold above this does not earn interest.
  At the cap, interest is roughly 25–35 million per turn.
- **Absolute money cap: 2,000,000,000 in BRE.** You cannot hold more than 2
  billion coins at once (in the bank or on hand) — a separate, higher ceiling
  than the interest cap.

  **IB makes it a sysop knob, defaulting to BRE's figure.**
  `Config.MoneyCapBillions` (Configuration Editor: "Money Cap (billions)") is
  the cap in whole billions, read through `World.MoneyCap()`. It defaults to
  `MoneyCapMinBillions` = 2, which reproduces BRE, and may be raised to
  `MoneyCapMaxBillions` = 999. Gold credited above whatever it is set to is
  still discarded — the knob moves the ceiling, it does not remove it.

  BRE's own 2 billion is the largest a 32-bit signed integer holds, so it reads
  as a machine limit rather than a design choice. IB's money fields are `int64`,
  so the ceiling is now a game rule and behaves the same on a 32-bit door as on
  a 64-bit one. The knob is in whole billions so the editor's field fits an
  `int` on a 32-bit build, and 999 is the widest figure the abbreviated display
  renders in three digits before the point.

  Deposits and withdrawals are unbounded up to the cap — nothing gates the bank
  per turn, so a per-action limit there only cost keystrokes. What IB does keep
  at 2 billion is **one investment** (`MaxInvestment`); the number of
  investments is not limited.
- **A bank at the money cap pays its interest into gold in hand** rather than
  having it clamped away (`processEconomy`). The cap limits what one purse
  holds; a full purse is no reason to destroy the earnings. Gold in hand carries
  the same cap, so a baron whose hand is also full still loses the overflow.
  **Unverified against BRE** — IB chooses this because the alternative silently
  deletes money the player earned.
- **Gold destroyed by the cap raises an event**, naming the amount and its
  source ("a matured investment", "this turn's income", "the trading market").
  Every path that pays gold in runs through `World.creditGold`, which is what
  holds gold in hand at the cap and files the notice; `Withdraw` is the
  deliberate exception, since it draws only what fits and leaves the remainder
  banked. BRE reports nothing here — it just stops counting.
- **Figures of a billion or more are displayed abbreviated**, as a fixed
  4-decimal form with a capital B: 1,000,000,000 renders `1.0000B`,
  1,847,392,104 renders `1.8473B`, and `MoneyCapMax` renders `999.0000B`. The
  fraction is truncated, never rounded, so a figure just under the next
  billion never reads as having reached it. Below a billion nothing changes —
  full digits with the locale thousands separator. The rule lives in one place
  (`internal/numfmt`, which the engine and the menu layer both call) and applies
  to any figure, not only gold, so a unit count that runs into the billions
  abbreviates the same way. German and Russian take
  the comma as the decimal mark.
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
- **Rivers — IB pays gold *and* food every turn (DELIBERATE DIVERGENCE, #29).**
  In BRE each turn an empire's rivers do EITHER hydropower (gold) OR fishing,
  never both — strictly exclusive across 63/63 captured turns. **IB pays both
  every turn instead**, splitting a river's yield by `RiverFishShare` (30%): 70%
  of the hydropower gold plus 30% of a full fishing haul, every turn.

  The split is **expectation-preserving** — it is BRE's average, not a buff or a
  nerf — so no rebalancing follows from it. What it removes is the variance. At
  24 rivers the swing is ~121,000 gold present or absent; a player who commits to
  rivers at scale faces millions of gold appearing and vanishing with no way to
  plan around it, and a food source that shows up 30% of the time is close to
  useless for covering consumption. One constant drives both halves so they
  cannot drift apart when tuned.

  **Live measurements (63 turns across five captures):** rivers fished on **19 of
  63 turns, 30%** — the source of `RiverFishShare`. Hydropower gold is a clean
  `5,000 + Random(100)` per region (every one of 44 captured figures divided
  exactly by the river count, 5,002–5,099). Fishing yield per river is
  `110 + [0, ~20)` — Base 110 is firm (the Civilian advisor quotes it as the
  minimum) but the Rate is not pinned. **IB uses a flat `RiverFishFood = 124`**,
  which sits inside that range, so the remaining defect is the flatness rather
  than the magnitude.

  This does **not** make rivers a food region: 30% of 124 is ~37 food per region
  per turn against an Agricultural region's 300, about an eighth. It is a steady
  garnish, not a substitute for farmland.

  **Superseded (was recorded here as a caveat on 2026-07-23):** an earlier reading
  of "0 fishing across ~240 river-turns" was a counting error, not a finding — the
  captures are `\r`-separated, so `grep -c` reported one line where six fishing
  turns were present, and the fishing line is announced separately from the
  food-grown line. Fishing was happening all along. Count these captures with
  `grep -oc`.

  **Fishing is NOT gated on food need**, which that error had suggested. A
  controlled test — the same empire driven through a food deficit and then a
  surplus — fished on 4 of 9 short turns versus 3 of 11 surplus turns (Fisher
  exact, p = 0.64). See issue #67.
- **Agricultural output is a draw, not a flat rate (BRE-verified).** Per region
  per turn it is `FoodAgriBase (300) + Random(FoodAgriRate (5))` — one roll per
  turn shared by every Agricultural region, exactly like the gold regions. Across
  six captures and nine region counts (2 … 194) every printed total divided
  exactly by its region count, and the per-region figure landed on 300, 301, 302,
  303 and 304 — all five, so the width is 5 and the floor is 300. (IB paid a flat
  300 until 2026-07-30, i.e. always the bottom of the band.)
- **Food growth is a *turn-start* credit (matches BRE).** This turn's food yield
  (Agricultural draw + river fishing) is added to the granary at the **start**
  of the turn — alongside military production and gold income, exactly what the
  start-of-turn income report announces (`World.GrowFood`). So the player can
  **sell or spend this turn's growth the same turn**. (Earlier IB deferred the
  growth to the end-of-turn economy step, where it arrived after the food market
  and was always subject to spoilage — corrected 2026-07-20 after driving BRE.)
- **Food consumption:** each turn the population eats
  `People × PeopleFoodPerThousand (75) / 1000` and the army eats
  `Troopers / ArmyFoodDivisor (200)` — about **1 food per 200 troopers**.
  Live-verified twice: 42,259 troopers → 211, and a realm billed 36 food for
  7,212 troopers. `breins.txt` gives troopers "the added need for food, as
  compared to other units", which is comparative, not exclusive: **troopers are
  not the only unit that eats** (see the turret/tank rate below). A large
  standing army is still nearly food-free, as in BRE — food pressure comes from
  population, not the army. (Both constants in `balance.go`.
  Fixed 2026-07-23: the army was previously billed 1 food/trooper, ~200× too heavy.)

  **The two terms are truncated separately, then summed** — the total is
  `trunc(population food) + trunc(army food)`, not one accumulator truncated
  once. Four turns of a combined `N units of Food consumed` line discriminate
  the models (25,865M + 49,840 turrets billed 38,801, where a single
  accumulator gives 38,802); the same holds at 219,032 and 278,857 turrets.
  Summing the two terms before truncating reads one unit high whenever both
  have a fractional part.

  The **people** rate is BRE's, not a reconstruction, once the population scales
  are lined up. BRE charges exactly `1.5` food per million people — nine samples
  across five captures, from 100M to 34,600M, all exact under truncation
  (4,081M → 6,121, i.e. `trunc(6121.5)`). IB's population counter runs 20× BRE's
  displayed millions (a fresh realm is 2,000 to BRE's 100M), and `75/1000` is
  `1.5/20`, so the two charge the same food for the same realm. An earlier note
  here claimed IB's rate was lighter; it is identical.

  **Turrets and tanks each eat 1 food per 10,000 units, and IB does not charge
  them.** The divisor is **10,000 exactly**, not a fit. Ten army-food prompts
  from a turret-only empire (99,382 up to 816,657 turrets) bracket it to
  9,966.5 … 10,082.2, and a tank-only empire drove it shut: with production
  frozen and the count sold to an exact figure, **30,000 tanks billed 3 and
  29,999 billed 2**, which allows only 9,999.67 … 10,000. The same pair proves
  **truncation, not rounding** (2.9999 → 2). Fifteen further tank readings from
  10,194 to 56,197 are all consistent.

  Tanks and turrets therefore carry the same weight, so the rate is a property
  of the army, not of the unit type. An earlier note here recorded tanks and
  jets as eating **nothing**; that test added 1,000 jets and 533 tanks, both of
  which truncate to zero at this rate, so it had no power to detect them.
  **Jets, bombers and carriers remain untested — not known-free.** Covert agents
  are bounded below 1 food per 300 agents (a 50-agent purchase moved nothing),
  which rules out the trooper rate but not the turret/tank one.

  The charge stays 0.02% of a realm's food bill, which is why IB's omission has
  never shown up in play.
- **Food spoilage:** **5% of the food remaining after growth and consumption**
  spoils each turn — `floor(0.05 × food)` — with **no floor** below which nothing
  spoils, reduced by Technology regions. **Re-verified by driving the original
  (2026-07-20):** spoilage matched `floor(5% × food-after-grow-and-consume)` to
  the unit at three stocks (1,452→72, 2,668→133, 0→0). **Re-verified again at
  scale (2026-07-30)** over every capture: 62 of 63 turns matched to the unit,
  once food sold or bought at the market between the stock line and the spoilage
  line is counted. Truncation, not rounding (29,759 → 1,487, from 1,487.95). Because growth is credited
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
- **Land market:** you may buy at most **500 regions per turn**, and the
  per-region price rises as you own more (about 1,100 coins/region when you hold
  only 2). Land is also **finite**: see Daily Land Creation below.
- **Daily Land Creation — a PER-EMPIRE allowance (BINARY-VERIFIED).** A realm may
  only buy land its own allowance covers, and each purchase draws that allowance
  down; every daily maintenance grants `Config.LandPerDay` more. When it is used
  up the original answers *"No land is available at this time."*

  It is **not** a shared planet-wide pool. `BRE.OVR 0x12D30` bounds a purchase
  against a field at `+0x331` on the **empire** record (the same record that
  holds gold and popular support), and `0x12EF9` subtracts the number bought
  through the sub-32 helper. IB modelled it as one contested planet pool first;
  the disassembly disproved that, and the difference matters — a shared pool
  makes realms race each other for land, a per-realm allowance does not.

  IB had the config fields (`LandPerDay`, `InitialMarketLand`) since before this:
  editable in the Configuration Editor, shown on the Game Setup screen, and
  broadcast over inter-BBS — but **nothing read them**, so land was effectively
  infinite. That is why a beaten realm could rebuild faster than any war could
  take land from it; a 60-day bot game reached 267,000 regions with not one realm
  ever conquered. A new realm opens with `InitialMarketLand + LandPerDay` so it
  can expand on its first day.

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

Turns per day: 10 (config; BRE's own default is 8). New players get protection
turns at the start (config: 15). A turn walks through a sequence of menus:

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

**Mail arrives at the head of a turn, unasked.** In BRE the recap runs straight
into the inbox: each message is drawn in its box and read on one keypress
(`[R]eply / [D]elete / [I]gnore / [Q]uit`), and an empty inbox prints *"You have
no messages."* in that same spot. There is no count line and no "read them now?"
gate — IB had both and dropped them. Two deliberate differences remain:

- **IB re-checks each turn, BRE once per play.** Another node can deliver mail
  while a session is in progress (#3), so a later turn opens the reader again;
  it stays silent on an empty inbox so the line is not repeated all day.
- **Enter does nothing at the message prompt.** Every other prompt in the game
  treats Enter as the default, which here would skip an unread message for a
  player holding Enter through the pre-turn stops. Only R/D/I/Q act.

Box geometry, measured from a live capture: a 76-column top rule carrying the
date and time, and a short 41-column rule under the From/To headers (it does not
span the box). Colors are in `docs/dev/bre-screens.md`.

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
- **Send Message can address all allies.** BRE's picker takes one realm or `Z`
  for all of them; IB adds `*`, which writes to every realm the sender holds a
  standing treaty with, of any type (Enemy is a relation, not a treaty, so an
  enemy is never included). The key is offered only while they hold one.
- **Diplomacy is on the opening menu as well as System.** BRE lists it under
  System alone.
- **Buy Regions closes itself when the gold runs out.** A purchase that leaves
  the player unable to afford another region says so and returns to the Spending
  menu; BRE asks again, so a player who has just spent their last gold reads
  "You can afford 0 regions" and quits by hand. Reaching the day's region cap
  does not close the screen — a capped player may still want its Advisors
  entry.
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

- **Group Attack** — commit real forces, not gold. Each baron sends troopers,
  jets, tanks and bombers (deducted from their army); the pooled detachments are
  the strike's offense, valued by the combat table (trooper 1, jet 2, tank 4,
  bomber `GroupAttackBomberOffense`). Survivors return to each contributor,
  reduced by `GroupAttackLossPct` — including when the strike is refused, so
  attacking a realm that turns out to be protected still costs a slice of the
  force. **Indiv. Attack Force** commits the same four types (#62).
- **Protection crosses the league.** A scores packet marks each realm still
  under New Realm Protection, and the attack and terror target lists leave those
  realms out — matching the local attack list, which hides them too. The target
  board still refuses an arriving strike itself, since the flag can go stale
  while the strike is in transit. Spying is not blocked by protection, so the
  spy target list shows every realm.
- **Terrorist Ops** — a force-destroying strike (not intel). Commit agents; the
  op is queued and resolves on the target board's next packet run. Each agent is
  one hit that removes ~1/`TerrorUnitLossDenom` (7, from BRE's disassembled 6/7
  ratio) of one randomly chosen unit type. New Realm Protection blocks it.
- **Spy** — send an agent to a remote baron; intel lands in the planet-wide Spy
  Database. Reached from the Spy Database screen.
- **SDI** — puts gold into the program, capped at `SDIMax` (50%, per BRE's "up
  to 50%" missile interception). See "The SDI program" below.

### The SDI program

The shield is a pot of gold rather than a level bought outright. Its screen
shows four figures — Total Funding, Yearly Maintenance, Funding / Region,
Current SDI Strength — then the turn's allowance and the funding prompt.

Two of its rules are **capture-verified**, fitting all seventeen consecutive SDI
screens in a live league game exactly, from an empty program to just over seven
million gold (`docs/dev/bre-screens.md`):

- **Upkeep** = 4% of total funding, billed every turn even though the screen
  calls it yearly.
- **Spending allowance** = 20% of total funding, never less than 250,000, and
  refilled every turn. Gold goes in only in whole thousands, which the screen
  says outright.

Two things are **not** established:

- **How funding converts to strength.** The captured game's region count moved
  under the figures too much to read a curve off seventeen points, and the
  strategy guides say the percentage scales with region count, which the screen's
  own "Funding / Region" line hints at. IB keeps its own `SDIStep` until the
  original's rule is disassembled, so its strength curve is NOT a fidelity claim.
- **What underpaying the upkeep does.** The captured player always paid in full.
  IB scales the program back to what was funded; without some consequence the
  upkeep would be optional and an unmaintained shield would defend as well as a
  maintained one.

The original printed `Funding / Region: 0,000 Gold` at every funding level,
including seven million. That is unexplained and most likely a defect in it, so
IB shows the figure the label describes.

The original's own instructions confirm what the screen's per-region line hints
at: "The more territory you control, the larger your program will need to be as
well." So the strength curve is a function of funding AND land, which is why a
funding-only table cannot give it.

#### What the shield actually does is not verified

The funding side above came off a live capture and is exact. **The combat side
did not.** It arrived with the feature, carries no provenance, and does not
match the original's documented figures — treat every number in this paragraph
as IB's own until someone reads the original's code:

- IB scales missile damage by the defender's SDI percentage. The original
  describes *destroying* up to half of incoming missiles, which is interception,
  not a discount — and one strike is one missile, where the two models disagree
  most. (#113)
- IB applies the SDI percentage to jet effectiveness and has no bomber term. The
  original caps jets at 30% and bombers at 20%, separately. (#113)
- IB already contains two different models: an R5-Slappenheimer is intercepted
  on a roll against the SDI percentage, while nuclear, chemical and biological
  damage is scaled by it. Nothing justifies the split. (#113)
- IB reduces Clingy Annihilator damage by the defender's SDI. It should not: the
  original says jets are "the only way to destroy this thing", and the doomsday
  weapon is not among the three things SDI is documented to work on. (#111)

**Picking a planet is one prompt everywhere.** BRE asks
`Enter Planet Name or Number (? for list)` on every screen that needs a planet —
the Spy Database's viewer and the IP Messages screens both, from the live
capture — and `?` draws the red-ruled "List of Planets" table of number, planet
name and location. The number is the planet's **roster** number (`ibnodes.dat`,
BRE's BRNODES.DAT), so a planet keeps the same number on every screen; a board
heard from over a packet but absent from the roster is numbered on after it.
The list includes the board you are calling from.

IB routes every planet pick through it (`pickPlanetNamed`): Create Group Attack,
Indiv. Attack Force, the spy and terrorist targeting, the Clingy Annihilator, and
IP Messages. A name must be typed in full (case-insensitively) — whether BRE
accepts an abbreviation was never observed, and guessing would risk sending to
the wrong planet. An IB divergence: a message addressed to your OWN board is
delivered locally rather than queued as a packet, which would otherwise leave
for a transport with nowhere to take it.

### Travel Times (binary-verified)

BRE's "Average Turn Around Times to All BBSes" is a measurement of the
*transport*, kept per node in `DATA\TIMES.BR` (255 six-byte reals, indexed by
node number, in days). Read from the overlay routines at `BRE.OVR`
0x445B0-0x44770 (the loader/saver and the `TIME_CHECK` handler) and 0x23D70 (the
display):

- **The probe.** A record of `{fromNode, toNode, sentStamp}` is sent to every
  other active node. The receiving board's only job is to send it back
  **unchanged**, with the two node bytes swapped; the stamp is never rewritten,
  so what the originator finally measures is a full round trip.
- **The average**, folded in when the echo comes home:
  `avg = (avg + 2*elapsed) / 3` — weighted two-to-one toward the newest sample,
  so a transport that changes speed is reflected within a few exchanges rather
  than being diluted by every trip ever made. A board with `avg == 0` has never
  completed one and reads "No Data".
- **The display** quantizes before printing: under two days it shows
  `round(days*240)/10` as `N.NN hours`; at two days or over,
  `round(days*100)/100` as `N.NN days`. Colors are BRE's — bright green for
  hours, cyan for days, red for "No Data" — and each row is the planet name in a
  30-column field with the figure straight after, between two 75-column inset
  rules.

**IB adds two smaller units** (`TravelSecondsCutoff`, `TravelMinutesCutoff`): a
round trip under a minute reads in seconds, under an hour in minutes, and a real
measurement is never rounded down to zero. BRE never needed them — a league
polling a few times a day is measured in hours — but a modern always-on link
answers in seconds, where BRE's format prints `0.00 hours` and reads as "never
measured". The stored figure and the averaging are unchanged; this is the
display only, and it is a deliberate readability divergence.

IB implements the mechanic as described. Constants: `TravelAvgNewWeight`,
`TravelAvgDenom`, `TravelHoursCutoff` in `balance.go`. The probes ride along with
whatever else the inter-BBS run is sending, once per game day
(`World.PingTravelTimes`); the stamp is RFC3339, so boards in different time
zones measure the same interval. What BRE keys off a configurable day interval,
IB fixes at one day.

### IP Messages

Interplanetary mail is addressed to a **planet**, not to a baron: it lands in
the mailbox of every realm on the receiving side. The menu (live capture) is
Single Planet, Select Planets, All Planets, Allied Planets, Planet Coordinator,
Quit, drawn as a 25-column cyan box. Naming a planet shows "Our current
relations with X" first; the text is then written in the same 20-line editor
local mail uses.

Two addresses are narrower than a planet. **Planet Coordinator** reaches the
receiving planet's elected Coordinator alone — BRE heads such a message
`Message To  : Coordinator`. And a **reply** may be sent to its author alone;
see below.

**Allied Planets** writes to every planet the board's own diplomacy chart calls
Allied (see Planetary diplomacy, below) — the one thing that chart drives rather
than describes. A message to a planet reaches every living realm there,
computer barons included, which is the same reach the local "send to all" has;
a coordinator message with no Coordinator elected posts a news line instead.

**Replying is BRE's own.** It ships a SECOND message reader for interplanetary
mail (`DATA\MSG.BRF`, strings at `BRE.OVR` 0x1F94C), separate from the local one
(`DATA\MSGS.DAT`, 0x1DC0E). It heads the box `Message From: <realm> on <planet>`,
offers the same R/D/I/Q, and asks **`Public Reply?`** — yes sends the answer to
the whole of the author's planet, no to the author alone. It then offers
`Quote Message?` with a first and last line to quote, and quotes under a
`Quote From <realm> Of <planet>` header.

IB implements the addressing (`IPMessage.ToEmpire` carries an author-only reply,
`Message.FromBoard` the route home), the "Public Reply?" prompt, both headers,
and the quote (below). One divergence: IB has a **single mail reader** that
names the planet when a message came from one, where BRE keeps a separate
interplanetary reader. The screens read the same; the duplication is not worth
reproducing.

### Planetary diplomacy

Each board keeps a chart of where it stands with every other planet: **Enemy**,
**None**, **Peace** or **Allied**. It is shown by InterPlanetary Ops → Diplomacy
List, and beside every planet a player names ("Our current relations with X").

It is **not** a treaty system, and BRE says so on the screen that edits it
(`BRE.OVR` 0x23425): the chart "is *not* official… None of the info in this
chart is official. (ie, none of this is forced nor reported to the other
planets.)" So:

- The value binds nothing. Calling a planet an Enemy does not stop a treaty,
  a trade or an attack, in either direction.
- It never rides a packet. Two boards can hold contradictory views of the same
  pair and neither will ever learn of the other's.
- Only the elected **BBS Coordinator** may change it, on a Diplomacy
  Modification screen: pick a planet, then answer `Change status to War, None,
  Peace, or Ally?`. *War* files Enemy and *Ally* files Allied — the prompt's
  words and the display words are two different tables in the original
  (`BRE.OVR` 0x23530 and `BRE.EXE` 0x158b5).

One thing reads it rather than displaying it: **Allied Planets** on the IP
Messages menu addresses the planets it calls Allied.

IB implements all of this. Two things are not established from the original: the
color it prints **Enemy** in (the capture only ever showed the other three, so
IB infers bright red), and which hotkey on BRE's eight-item Coordinator Ops menu
opens the screen (IB uses `D`). A new season clears the chart, since the
Coordinator who filed it is cleared with every other empire.

### Quoting a reply

Both of BRE's readers offer the same quote, and IB's one reader does it for
local and interplanetary mail alike (captured live, `cap/eots-ibbs.cap`):

- **`Quote Message? (Y/n)`** — default **Yes**.
- **`First Line to Quote:`** and **`Last Line to Quote:`**, in the message's own
  line numbers. The bounds are forgiving, not strict: the captured reply answered
  20 for a message one line long and was quoted without complaint. In IB a line
  past the end meets the same clamp-and-confirm every over-max entry does (#9) —
  the figure is corrected to the message's length on screen and a **second Enter**
  commits it, so an over-range answer costs one extra keypress rather than an
  error.
- The chosen lines open the editor **already in it**, each prefixed `> ` under a
  `> Quote From <realm>` header (`> Quote From <realm> Of <planet>` on the
  interplanetary side). They are ordinary editor lines from there on — numbered,
  counted against the 20-line limit, and cleared by `/C` with everything else.
  BRE numbers them in blue where a line being typed is green, and IB does the
  same.
- A line that was **already** quoted carries over like any other, nesting as
  `> > `. Choosing the range is what keeps a long exchange from growing without
  bound — IB used to drop previously-quoted lines instead, which it no longer
  needs to do now that the range is asked for.

IB's own additions to the range prompts are the `(suggested; max)` hint every
other numeric prompt carries — BRE pre-fills the field with the value instead,
which IB's line reader cannot do — and no quoting at all when the message being
answered is empty.

## Diplomacy

**One relation per pair (BRE-faithful, #88).** Two empires hold exactly ONE
relation at a time, so forming a pact **replaces** whatever stood before —
trading a Full Defense Alliance for a Tariff Trade Agreement gives up the
alliance. IB previously kept a list and let realms stack every benefit at once.
The relation enum, decoded from BRE.OVR's value→name dispatch (table at 0x1C0E3):

| value | relation | | value | relation |
|---:|---|---|---:|---|
| −1 | Enemy | | 4 | Terrorist Prevention |
| 0 | None | | 5 | Intelligence Alliance |
| 1 | Tariff Trade Agreement | | 6 | Technology Agreement |
| 2 | Protective Trade | | 7 | Full Defense Alliance |
| 3 | Free Trade Agreement | | 8 | Declaration Of War |

This is also the independent confirmation that **6 is Technology Agreement**, the
value the Technology Agreement research bonus keys on. Menu index equals relation
value for 1–8, so the Diplomacy menu's numbering *is* the enum.

**Declaration Of War is the formal way to end an agreement, not a separate war
system.** BRE's own instructions: *"This is used to break an agreement with
another empire without causing internal troubles in your realm. The treaty is not
officially broken until the other realm is notified."* Two consequences, both
implemented:

- **Declaring war costs nothing at home** and leaves the pair at **Enemy**
  (`World.DeclareWar`), mailing the other realm.
- **Breaking a pact any other way does cost you.** Attacking a realm you hold an
  agreement with breaches it: the pair drops to Enemy and the breaker loses
  `TreatyBreachSupportPenalty` popular support (`World.breachTreaty`, called from
  `Attack`). The original does not publish the size of that penalty, so the
  figure is IB's own.

**Known simplification:** the original says a treaty "is not officially broken
until the other realm is notified", implying the old pact still binds until the
message lands. IB breaks it immediately and mails the notice in the same step;
that delay window is not modelled.

A save written before this held stacked relations; `EnsureTreaties` collapses
each pair to the last one recorded on load.

**The proposer is told how the offer was answered.** A pending offer sits on the
target realm until it plays, so the reply is asynchronous — it lands on the
proposer's "since your last play" log whenever the other side gets around to
answering. **Captured live from a league game (2026-07):** seven replies over
about three and a half hours, each one line, in BRE's wording —
`X accepted your Full Defense Alliance proposal.` /
`X rejected your Full Defense Alliance proposal.` (note *rejected*, not
"declined"). IB files the same two lines (`notifyProposer` in
`internal/game/diplomacy.go`), for AI answers as well as human ones.

**IB shows the proposals you sent; BRE shows them nowhere (#92, DELIBERATE
DIVERGENCE).** A proposal is stored on the *recipient*, so in BRE the sender's
View Treaties still reads `None` for that realm — indistinguishable from one they
never contacted. IB derives the sender's list by scanning (`ProposalsFrom`) and
prints it as an "Awaiting a reply" block under the Relations table; no new state
is stored, so the two views cannot drift. Safe because your own outgoing offers
are information you already hold — nothing about another realm is revealed.

Two rules go with it. A proposal **does not expire**: it stands until the target
accepts, rejects, or is eliminated. And a **new proposal to the same realm
replaces the pending one**, for the same reason a pair holds one relation at a
time (#88) — only one can ever be agreed, so leaving both live would let a realm
accept a pact the proposer had already thought better of. Re-sending the
*identical* type is still a no-op and does not mail them twice.

The recap itself is styled as BRE styles it: each entry sits under its own
76-column rule carrying a 1-based counter and the real date and time the thing
happened (`eventRule` in `internal/menu/gameflow.go`; layout and colors in
`docs/dev/bre-screens.md`). Events are therefore stamped when they are filed
(`game.Event`), not when they are read. Entries carried over from a save written
before the stamp existed draw an unbroken rule with no time.

Seven treaty types are proposed / accepted / broken through the Diplomacy menu,
and each carries a gameplay effect (#11 wired the last two):

- **Full Defense Alliance** — blocks attacks between the two realms; when either
  is attacked, its ally **sends 30% of its mobile forces** — troopers, tanks, and
  agents only (*not* jets/turrets/bombers/carriers) — to reinforce the defender in
  that battle (`AreAllied`, `AllianceStrength`). **Verified live against BRE
  (2026-07):** an ally holding 10,903 tanks / 97 troopers contributed exactly
  3,271 tanks / 29 troopers = 30.0% (Attack Menu → Alliance Strength screen), and
  a losing attacker's report opens "The empire's allies send N Troopers and M
  Tanks." BRE in-game text: "the most balanced and powerful alliance… puts forth a
  large amount of all of your forces in defending an ally in need. NOTE: effective
  only in Local Games." **IB implements this** (`allyDefenseBoost` / `AllyDefenders`
  in `internal/game/diplomacy.go`, `AllyDefenseContribPct = 30` in `balance.go`):
  when a realm is attacked, each Full Defense Alliance partner adds 30% of its
  troopers + tanks to the defender's battle power (valued as the ally's own
  `Defense()` weighs them — tanks 3–5 troopers by HQ, morale- and tech-scaled; turrets
  stay home, agents are covert); the attacker's battle report notes the
  reinforcements, and the committed detachment bleeds at the defender's casualty
  rate (`bleedAllies`). The Alliance Strength screen (`allianceStrength`) shows
  each ally's sent troopers/tanks/agents. See the `bre-binary-verified-math`
  memory.
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
  BRE's "Depositing trading market money" step). **There is NO commission**
  (`MarketCommissionPct = 0`), measured 2026-07-30 (#43): a 200-trooper listing
  at 1,000 cost the buyer exactly 200,000 and put exactly 200,000 in the seller's
  bank. The bank read 201,250 the next day because it accrues interest —
  confirmed by watching it reach 205,048 with no further deposit, which is
  `201,250 x 1.00625^3` to the coin. Full pass-through, verified from both ends.
- **Bank interest is per TURN, not per day:** 0.625% a turn under a 5.0% daily
  rate at 8 turns/day, compounding — the daily rate divided by turns per day.
  This was a by-product of the commission test and it **confirms IB's existing
  formula** (`Bank x InterestRate / (1000 x TurnsPerDay)`, `processEconomy`),
  which until now rested on the config help text alone.
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

Now matching this reference (as of v0.0.4):

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
- Bank interest ~1% per turn, with the interest cap and the money cap
- Nuclear / chemical / biological strikes and pirate raids
- Clingy Annihilator, with the original's fund-build-launch-intercept lifecycle,
  and SDI defense (a flat percentage damage-reducer)
- Covert agents with spying and sabotage (success scales with agent count)
- Player mail — a BRE-style per-message reader (Reply / Delete / Ignore /
  Quit), where Ignore keeps a message for next time (it can be ignored
  indefinitely) and only Delete removes it; Reply quotes a chosen line range of
  the original — plus a
  planetary bulletin
- Multiple turns per day, new-realm protection, and daily maintenance
- A rising land-market price (expansion is self-limiting)
- Region types, with the reference Rate/Base pairs, and the food market
- Diplomacy: treaty offers, the one-agreement-at-a-time rule, Declaration Of
  War, and the `-*Relations*-` roster
- The covert menu, including Spy on Relations, the Spy Database, bribery,
  expose enemy ops, and the Bomb Enemy Targets submenu
- The InterBBS (interplanetary) layer: file-drop packets, group and individual
  attacks, spying another planet, a Clingy Annihilator aimed across planets, and
  league orders signed by the coordinator
- IP Messages: mail addressed to a planet, to its Coordinator, or back to one
  author, written in the same editor local mail uses
- Travel Times: the average round trip to each planet, measured by a probe the
  far board echoes back
- A daily Planetary Master, crowned as `LastMaster` when a season ends

Still missing against the reference:

- A league season that ends on a schedule. A Planetary Master is crowned each
  day and the Coordinator can start a new season with `-league-reset`, but
  nothing times one.
- Negotiated empire-to-empire trade deals carrying goods with demands (see the
  trading section above; `Send Trade Deal` sends gold only)
- The Quick-Strike and Extended-Battle attack variants
- Civil-war collapse
- BRE's finer interplanetary news subtypes

A few Diplomacy and Covert menu items are recorded but inert, pending fuller
subsystems. Each is flagged where it is described above.

### Deliberately not implemented

- **Lottery.** BRE offers a ticket at the start of a turn
  (`Would you like to buy a lottery ticket? (Y/n)`), takes six letters, draws six,
  and pays winnings into the bank — one observed draw picked `ABCDEF` against
  `OZDQEF` and paid 500,000. No ticket cost was shown or charged. The match rule
  is unresolved: that draw scores 2 by position (E, F) or 3 by set intersection
  (D, E, F), and both fit the payout. **Not planned** — it is a pure
  random-gold faucet with no decision content. Revisit only if players ask for it,
  in which case the match rule needs pinning down first with a shuffled-letter
  ticket.

  The ticket is offered **once per day per empire**, not on every entry into the
  game — re-entering the same day brings no second offer. It shares that gating
  with the Queen Royale tax refund (#93), and the disassembly settles it: the
  recap calls the refund and the lottery back to back, both behind the same
  `turnsRemaining ≥ turnsPerDay` test (`BRE.EXE 0x61dd`). So the two are one
  **first-play-of-the-day event block**. The refund is built (see its entry
  above); a lottery would hang off the same hook rather than be a random event.

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

## Inter-BBS packet integrity (#53)

BRE guarded its league traffic with CRCs, duplicate detection and a binary
`COORD.KEY` that authenticated the League Coordinator; its own error strings name
the threats ("Invalid CRC in incoming Gooie Kablooie from BBS #", "Duplicate or
Late Attack Return Recieved - Packet Deleted", "Illegal Route Found from BBS #").
IB implements the two that still bite. Both hold across a relay: a forwarded
packet is passed on byte for byte, so the Coordinator's signature and the
origin's sequence number are still the ones the destination checks.

- **Coordinator authentication.** The Coordinator's board holds an ed25519
  private key (`coord.key`, created with `-gen-coord-key`); every board holds the
  public half (`coord.pub`, installed with `-coord-key`). Everything that
  dictates to another board — the league ruleset, the roster, a league-wide reset
  — is signed, and a board that cannot verify a signature refuses the order
  rather than trusting it. A shared secret could not do this job: every member
  would hold it and could sign with it.
- **Anti-replay.** Each outbound packet carries a sequence number. A board tracks
  the highest it has seen from each sender and a set of packets already applied,
  and drops anything it has processed before or that arrives with a stale number.
  Without it a saved packet could be dropped back in to pay a strike's results
  out twice, or re-run a reset.

## Inter-BBS packet routing (#106)

BRE routes a league as a tree rather than a mesh, and the Coordinator owns the
shape of it. The routing is written into the roster itself, on each board's first
line — the node number, `HOST`, then the numbers it forwards for (`2 HOST 3 5 7
8`) — and `docs/bre.doc` tells members to ask their Coordinator whether they need
routing of their own at all, "in which case you may skip this step". A board may
override the answer with its own `ROUTE.CFG`, whose rules the manual says
"override anything else assumed by the BRNODES.DAT file".

IB reads the same HOST lines out of `ibnodes.dat` and the same rules out of
`ibroute.cfg` (`ROUTE <dest|*> <via>`, last match wins). What this buys is the
reason BRE did it: a leaf board configures one link, to its uplink, whatever the
size of the league, and a board joining costs one sysop an edit instead of all of
them. `-league-routes` prints the resulting table, as BRE's `BRE TEST` does.

Divergences, all forced by the transport rather than chosen:

- **BRE's ROUTE.CFG also sets a FidoNet mailer's send priority** (`CRASH`,
  `HOLD`, `NORMAL`). IB reads and ignores those: a packet is a file in a
  directory, and what the transport does with it is configured in the transport.
- **A broadcast is addressed per planet before it is written**, one file each,
  because only the game knows the tree and the transport cannot fan out along
  it. An unrouted league still writes the single broadcast, and its transport
  fans that out as before — a league only changes behaviour once its Coordinator
  writes a HOST line, so no existing board is affected.
- **A hop count caps forwarding** (`MaxPacketHops`). A cycle typed into a roster
  is one sysop's mistake that every board obeys, and nothing else would notice
  it. BRE's "Illegal Route Found from BBS #" says it had the same problem.

**What this does not do**, and BRE could not do either: stop a sysop inventing
their own board's figures. Scores and strikes are self-reported and unsigned;
signing them would imply a guarantee about the *numbers* that nothing can give.
What is implemented stops one board dictating to the others, and stops a packet
landing twice.

**Origin is a separate question, and is open (#118).** Whether a packet really
came from the board it names is not the same as whether that board's figures are
honest, and the second being unfixable does not make the first so. Today nothing
checks the sender: a packet written into an inbound directory is applied on the
strength of a name it carries. Every transport a league actually uses makes
that check itself — a mailer authenticates the session behind a netmail, a hub
binds an upload to an account — but IB neither requires nor reads the result, and
the plain file drop it documents as the default supplies nothing. Sysop answers
in discussion #117 are what surfaced this.
