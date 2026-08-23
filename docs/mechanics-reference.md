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
| Jet | 2 | **0** | Offense only. High upkeep. Needs carriers (1 carrier moves 100 jets). An enemy SDI cuts jet strength, but only on an interplanetary strike — see "SDI Defense". |
| Turret | **0** | 2 | Defense only — the defensive **counterpart to jets**: it shoots down attacking jets (and blows up tanks / kills troops). Also helps intercept nuclear missiles. Cannot be destroyed by terrorist ops. |
| Tank | **3–5** | **3–5** | Best all-round. Low upkeep, high buy cost. Strength scales with **HQ** (3 at 0%, 4 at 50%, 5 at 100%) and with morale. The guide's flat "4" is the HQ-50 value — see HeadQuarters below. (`whatsnew.doc` claims tanks help defend against chemical missiles; the shipped v0.988 routine never reads the tank count — see the chemical attack below.) |
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
  2 — that is the **interplanetary invasion** resolver (`0x402D6`,`0x404D2`),
  and it is a *different* morale curve from the one a same-planet attack uses
  (`morale × 0.6 + 50`, `0x0F37B`). IB implements the same-planet curve
  (`moraleFactor`) on both paths; the two differ by at most 3% at full morale and
  cancel between attacker and defender.
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

   BRE's five shortfall penalties all share this shape and differ only in scale
   and target — **binary-verified**, and IB implements every one:

   | Shortfall | Scale | Stat | Address |
   |---|---:|---|---|
   | Armed forces | 40 | military morale | `BRE.OVR 0x2F077` |
   | Regions | 50 | popular support | `BRE.OVR 0x2F196` |
   | Crown tax | 15 | popular support | `BRE.OVR 0x2FAF1` |
   | Food, the people's share | 40 | popular support | `BRE.OVR 0x38104` |
   | Food, the army's share | 40 | military morale | `BRE.OVR 0x382E9` |

   The armed-forces branch touches morale **only** — not support — and the region
   branch support only. Two of them additionally file a **civil war** (below).

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
   called immediately after it is the **lottery**, the other half of the same
   first-play-of-the-day event block (its own entry follows).

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

   ### The Queen's lottery

   The other half of the same first-play block, settled immediately after the
   refund and from the same routine's caller (`BRE.OVR 0x018610`, called from
   `BRE.EXE 0x038a2`). **Fully binary-verified.**

   A ticket costs **5,000 gold**, taken the moment the offer is accepted and
   **never shown on screen**. The offer is skipped entirely for a realm holding
   less than the price, and the block runs once a game day, so refusing it — or
   being too poor for it — means no second offer until tomorrow.

   The player types **six letters**, A–Z only, one keypress each with no Enter to
   finish; **Enter takes a random letter** for that position, so a held key still
   produces a playable ticket. Six letters are then drawn, each uniform over the
   26, and scored by **set intersection with multiplicity**: a drawn letter
   matches any not-yet-used letter on the ticket wherever it sits and consumes
   it, so one `A` on the ticket scores at most one drawn `A`. Position is
   irrelevant — the original's scan blanks the ticket slot it matched.

   | Matches | Prize |
   |---:|---:|
   | 1 | 2,500 |
   | 2 | 10,000 |
   | 3 | 500,000 |
   | 4 | 1,000,000 |
   | 5 | 4,000,000 |
   | 6 | 10,000,000 |

   One match is still a 2,500-gold loss. The six-letter prize is `0x00989680` =
   ten million; the hundred million that circulates among players is not in the
   binary. Winnings go to the **bank**, not to gold in hand.

   Whether the lottery runs at all is the **BBS sysop's** choice, not the League
   Coordinator's: the switch is the `LOTTERY` boolean in the per-install
   `RESOURCE.DAT`, default on, so two boards in one league may differ.

   **IB implements this**, constants in `balance.go`, the switch as `Lottery` in
   `bbs.cfg` — IB's own per-board file, which no broadcast rewrites. Two
   divergences. Where the prize would carry the bank past the money cap BRE pays
   **nothing at all**; IB banks what fits and pays the rest into gold in hand
   through `creditGold`, which reports what the ceiling ate rather than deleting
   a win it has just announced. And IB draws an unmatched letter in bright red
   rather than the original's dark red, which sits at 2:1 against black, and
   states the match count in words so colour is not what tells a win from a miss.

4. Conditional: SDI maintenance (with SDI), waste-region decontamination (with
   waste regions), then the popular-support and military-morale boosts (shown
   only below 100). Support/morale are *requested* (optional), not required.
5. **Reconsider gate** — underpaying any *required* cost warns of disastrous
   results and offers to reconsider. Yes **restarts the whole sequence from the
   bank prompt**; No proceeds, with desertion/revolt for the shortfall.

Prompt colors (from a color capture): text plain white; the required and
suggested amounts bright cyan, the max dark cyan, the `(…; …)` parens bright blue.

IB implements steps 1, 2, 3, all of 4, and 5, with the required-capped prompts
and these colors. The SDI prompt is `Your SDI Program requires N gold.`, asked after the
region maintenance and before the crown tax (live capture).

**Auto-Pay Maintenance bypasses itself, per turn, on more than affordability
— BINARY-VERIFIED.** The silent one-line total only fires when Auto-Pay is on
*and* gold covers the bill *and* popular support is at its 100 cap *and*
military morale is at its 100 cap *and* the realm holds no Waste regions. Any
one of those failing falls through to the full manual sequence above for that
turn — including the optional support/morale boost prompts and the
waste-decontamination offer — even though the Auto-Pay *preference* itself is
never touched. It is a per-turn bypass, re-checked fresh every turn, not a
timed or persistent switch-off: once support and morale are back at 100 and
any Waste is cleared, the very next turn collapses to the silent total again.

Read from `BRE.OVR` procedure `allocate_turn_budget` (0x02eebb), the routine
`run_player_turn` (`BRE.EXE` flat 0x36e1) calls once per turn. Its gate, at
flat 0x3b12-0x3b6d: `real_compare` the gold on hand against the summed total
due, then `cmp word [es:di+0x94],0` / `cmp word [es:di+0x92],0x64` (support's
int32 at record `+0x92`, required == 100), `cmp word [es:di+0x90],0` /
`cmp word [es:di+0x8e],0x64` (morale's int32 at `+0x8e`, required == 100),
`or ax,[es:di+0xb6],[es:di+0xb8]` (the Waste count at `+0xb6`, required == 0),
then `cmp byte [es:di+0x339],0` (the Auto-Pay preference flag). Any failure
jumps to the block that calls `allocate_turn_budget` — the manual sequence;
passing all five skips that call and prints the collapsed total instead. The
`+0x339` preference byte is only ever read here, never written, which is why
this is a bypass rather than the checkbox itself flipping off.

IB implements this in `paymentStage` (`internal/menu/gameflow.go`): the same
five conditions gate the silent branch, reusing the empire's existing
`Support`, `Morale`, and `Regions.Waste` fields (0-100 support/morale is
already a structural range in the code, so no new `balance.go` constant was
needed for the 100 threshold).

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

**What the binary says, and what it does not.** The award is **computed, not
stored**: neither 213 as a 32-bit integer nor 213.0 as a Turbo Pascal Real48
appears anywhere in `BRE.EXE` or `BRE.OVR`. The Score field itself is a **Real48
at empire record +0x28A** (an earlier note called it +0x28E, which is only its
last word), and it is written from **exactly two sites in the whole program**,
both in `BRE.EXE` — neither of them a per-turn overlay stage. What the original
computes the award *from* is still not recovered.

Two readings survive every measurement: a size-independent constant, and the
realm's net worth captured at creation. They cannot be told apart from play,
because every empire BRE can create has the same start and so the same 213.

**IB implements** `Empire.Score` (seeded 0) `+= ScorePerTurn` each turn played,
with `ScorePerTurn` **derived from the starting setup** — `(StartRegions ×
NetWorthLand + StartTroopers × NetWorthTrooper + 500) / 1000`, which is 213 —
rather than written down as a literal. That reproduces BRE exactly for every
empire BRE can create, and keeps the award in step with the start if `balance.go`
retunes it, which is the one case where the two readings diverge. Nothing in the
economy takes Score away: riots and food spoilage do **not** touch it. BRE's
exact attack-scoring bonus is unrecoverable from the binary, so IB uses its own
combat-score model (below).

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
  loser's regions. Losses are asymmetric, and the asymmetry is an outcome of the
  strength ratio rather than a pair of rates: see "The regular attack's
  casualties and capture" under the combat section, which supersedes the earlier
  reconstruction from two live samples.
  - **Quick Strike / Extended Battle** (`attack.hlp`): these belong to the
    **IBBS individual attack**, not to local combat and not to group attacks. A
    live local Regular Attack offers no variant menu, and `breins.txt` gives a
    group attack no type choice either. A **disassembly of BRE.OVR's IBBS attack
    unit** settles it: the three-item menu is drawn inside `Indiv. Attack Force`.
    See "Individual attack variants" below.

  Region **capture** follows `min(loser regions, max(RegularAttackCaptureFloor,
  the Attack Rewards share × loser regions × density factor))`. The **density
  factor** is IB-original, and the binary now proves it is an addition rather
  than a reconstruction — BRE reads the defender's region count and the level
  constant and nothing else. It is the attacker's
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
- **Nuclear attack** — turns enemy regions into waste. BINARY-VERIFIED
  (`BRE.OVR` `launch_nuclear_attack`, unit `ovr_00e809` +0x225e):

  ```
  cost    = min(targetRegions * 3543, 50,000,000)
  percent = 7 + Random(3) - Random(3)          -- a 5-9% band, centred on 7
  ruined  = targetRegions * percent / 100      -- truncated
  ```

  The ruined count is taken out of the target's mix proportionally — *including*
  any waste it already holds, so a realm that is already half-ruined absorbs part
  of the next strike with land that was ruined anyway — and the same number is
  added back as Waste. **No land changes hands, nobody dies, and no realm can be
  eliminated by a nuclear strike.** Against a small realm the truncation can
  round the damage to zero.

  **The strike is not intercepted.** Nothing in the routine reads the target's
  SDI or turret count; its only reads of the target record are the realm name and
  the region total. SDI's "up to 50% of incoming missiles" (breins.txt) is the
  interplanetary path, and `whatsnew.doc`'s much older "Missile Bases now help
  defend against incoming Nuclear Missiles" does not survive into v0.988's local
  routine. IB previously scaled the damage by `(100 - SDI)`; that was IB's own
  invention and is gone.

  The price is quoted only after a target is named — all three missile screens
  reach one shared arms-dealer routine (`offer_black_market_weapon`, +0x206a)
  that prints the figure and takes a yes before deducting anything — so IB asks
  for the target first, then quotes. **That routine settles the bill against
  gold in hand PLUS the bank**: it compares the price against the sum, then takes
  whatever gold on hand cannot cover straight out of the bank. IB used to refuse
  a sale a banked fortune could have paid for; all three missiles now draw the
  same way.

  The attacker also gains **`Random(900)` Score**. Empire field +0x286 is the
  Score: it is written by eight aggressive actions and read by the empire status
  screen, the scores table's middle column, and the recon record. The award is a
  flat draw, not a share of the damage, so a strike that ruins nothing still
  pays.

  **The sibling awards, recovered in the same pass (for #103):**

  | Action                          | Score award              |
  |---------------------------------|--------------------------|
  | Nuclear strike                  | `Random(900)`            |
  | Chemical strike                 | `Random(700)`            |
  | Biological strike               | `Random(400)`            |
  | Pirate raid                     | `Random(300) + 100`      |
  | Send spy                        | `Random(30)`             |
  | Regular attack (two branches)   | a battle figure × 192, × 82 |
  | Returning interplanetary attack | a battle figure × 233    |

  The two regular-attack awards sit behind a league config byte (config record
  +0x3d8) and need their own pass before IB adopts them.
- **Chemical attack** — the nuclear missile's little brother: it ruins a third
  as much land, and gasses the people, the morale and the popular support on top.
  BINARY-VERIFIED (`BRE.OVR` `launch_chemical_attack`, unit `ovr_00e809` +0x25f0):

  ```
  cost    = min(targetPopulation * 94 + targetRegions * 2037, 50,000,000)
  percent = 3 + Random(3) - Random(3)          -- a 1-5% band, centred on 3
  ruined  = targetRegions * percent / 100      -- truncated, then made Waste
  killed  = targetPopulation * 20 / 100        -- a FLAT fifth, no roll at all
  morale  = round(morale  * 3/4)               -- record +0x8e
  support = round(support * 2/3)               -- record +0x92
  ```

  `targetPopulation` is in the original's unit of **one million**, so IB divides
  its own head count by `PopBREUnitScale` before pricing the shot. The damage
  percentages need no such conversion, being unit-free.

  The land damage goes through the very same region-to-waste helper the nuclear
  strike uses (`056d:18f0` — remove the count proportionally across the nine
  region fields, then add it to Waste at +0xb6), which is what `whatsnew.doc`
  records as "Local Nuclear & Chemical Missiles … now makes regions into WASTE
  regions instead of destroying them". **IB used to destroy the land outright**,
  and to kill troopers, and to scale everything by `(100 - SDI)` — none of the
  three is in the routine.

  **Not intercepted either.** The routine's only reads of the target record are
  the realm name, the region total, the population, morale and support: no SDI,
  no turrets, and no tanks. `whatsnew.doc`'s "Tanks now help defend against
  incoming Chemical Missiles" is an entry from before v0.988 and does not survive
  into the shipped routine, exactly as its nuclear/missile-base twin does not.

  Attacker sees the ruined-region count and the civilian toll, in that order;
  the victim's log gets both plus the famine that follows. Score: `Random(700)`.
- **Biological attack** — kills people and troopers, halves military morale, and
  touches no land at all. BINARY-VERIFIED (`BRE.OVR` `launch_biological_attack`,
  unit `ovr_00e809` +0x2ac6):

  ```
  cost      = min(targetTroopers * 23 + targetPopulation * 434
                  + targetRegions * 1237, 50,000,000)
  killed    = targetPopulation * (10 + Random(4) - Random(2)) / 100   -- 9-13%
  troopers -= targetTroopers  * (15 + Random(6) - Random(4)) / 100    -- 12-20%
  morale    = morale / 2                       -- an integer divide, truncating
  support   = round(support * 2/3)
  ```

  Same population unit as the chemical missile. The routine never reaches the
  region-to-waste helper, so the land is untouched — including its waste.

  The **Score lands on the sale**, before any damage is rolled, so a plague that
  kills nobody still pays its `Random(400)`.

  Attacker sees the trooper toll and then the civilian one — the reverse of the
  chemical report's order, which leads with land.

  **None of the three can eliminate a realm.** The nuclear and chemical strikes
  leave every region on the target's books as waste, and a percentage of a
  population never reaches zero. IB used to declare a realm "utterly conquered"
  on a chemical or biological strike; that was IB's own and is gone.
- **Attack pirates** — the nine pirate factions are living raiders, not a
  fixed difficulty ladder: their strength is random (any faction can be the
  strongest). Their **names are IB-original** (BRE's coined names are its own
  creative work). Pirates raid players at random: IB rolls a **20%** chance per
  turn that an empire is raided, plus a further **5%** chance of a *second* raid
  by a different faction the same turn — about 1 turn in 5, matching the felt
  frequency in BRE (the exact BRE rate is not measured). A raid carries off a
  share of **one** of the victim's holdings, drawn at random — never bombers or
  carriers, and never the victim's regions; the game grants a raiding pirate new
  regions instead, so a pirate that just raided is fatter. The draw is BINARY-
  VERIFIED (`BRE.OVR` at `0x35e30`: one `Random(16)` feeding an eleven-way ladder
  that selects a single victim field and a single name from the six-entry table
  after `" has captured "`): **troopers, jets, turrets, tanks 3/16 each, gold and
  agents 2/16 each**. Thirty-five raid notices across five captures never name
  two things, which is this draw and not a display limit. **Faces 11-15 read the
  victim's Trading Market listing instead of its inventory** — see the escrow
  entry below.

  **How often**: the roll is `Random(20) <= min(6, regions/1200 + 2)`
  (`0x35db5`), so exposure RISES WITH THE REALM — 3-in-20 for a small realm up
  to a 7-in-20 ceiling at 4,800 regions. Afterwards, landed or not, a 1-in-10
  roll re-runs the whole routine with a freshly picked faction (`0x363ba`, a
  recursive call), so a turn can carry several raids. IB's old flat 20% plus a
  5% "second raid by a different faction" was a guess; neither shape is in the
  binary.

  How much it takes is BINARY-VERIFIED too (`BRE.OVR` at `0x35f66`: a 32-bit
  divide by `0x21`, then a min against `0x5dc0 + Random(0x3e8)`):
  **`min(holdings / 33, 24000 + Random(1000))`** — about 3%, with a *jittered*
  cap. Six trooper notices in the captures reproduce `holdings / 33` exactly
  (15 of 510, 16 of 528, 90 of 2,976, 88 of 2,909, 1,058 of 34,933, 1,551 of
  51,187), and the three largest takes seen — 24,048 and 24,546 turrets,
  24,415 tanks — land inside the 24,000–24,999 band rather than on one ceiling.
  The earlier reconstructed "5%, capped at 24,999" was about two-thirds too
  harsh and read the jitter's top as a flat cap.

  One capture detail unexplained: **Gold** takes are tiny (38, 58, 258, 8,581)
  against treasuries in the millions, so the field a raid draws gold from is not
  the treasury at the moment shown. The likely reading is that the raid resolves
  early in the turn, before the turn's income is credited, so it takes 1/33 of
  whatever was left in hand — but that is inference, not read from the binary.
  **A faction's army is nothing but stolen goods.** BRE keeps no strength stat for
  a faction: its defense is computed live from its loot as
  **`tanks + turrets/2 + troopers/3`** (`BRE.OVR` `0x3671b`), and the only
  writes to a faction's record anywhere in the overlay are the raid-steal path
  and the raid-resolution path — nothing seeds one. A new game therefore opens
  with nine empty factions. That is not a contradiction: **a raid on a player is
  not a battle**, so an empty faction robs you exactly as well as a fat one and
  arms itself from the proceeds. Faction strength is consulted only when *you*
  attack *them*.

  A player's raid commits troopers/jets/tanks at **`troopers/2 + jets +
  tanks*2`** and wins on strictly greater. **Both sides take casualties either
  way**, computed before the win/loss test: the attacker loses
  `Random(5)+2` percent of what it committed, the faction `Random(10)+4` percent
  of what it holds. A winning raid is therefore never free (#119).

  A win hands back **1/3** of the faction's gold, regions, agents and troopers
  and **1/4** of its jets, turrets and tanks. Three consecutive raids on one
  band in `cap/kde3-01.cap` confirm it: gold 3391→2261→1507 and agents
  1460→973→649 decay by exactly 2/3, turrets 8516→6387→4790 by exactly 3/4,
  while troopers, jets and tanks fall faster because they also pay battle
  losses — those three are the types the faction commits to the fight, so they
  are docked twice and turrets are not. So
  one raid recovers a third of what a faction holds, two recover 5/9, three 70% —
  which is why one attack sometimes suffices and two or three usually do.
  **A winning raid that seizes pirate-held land opens the same region-type
  picker a Regular Attack uses (#21)**, while a raid on a landless faction yields
  only gold and military.

  Hard caps on what a faction can hold, read from the clamp sites themselves
  (`BRE.OVR` `0x3629c` and `0x36a59`, each a min against a literal, applied at
  the end of every raid and again after a player beats the faction): troopers
  **300,000**, jets **400,000**, turrets **400,000**, tanks **200,000**, agents
  **200,000**, regions **300**, gold **600,000,000**. A raid also grants the
  faction `Random(25)` regions. This supersedes the earlier reading of the
  BRE.EXE table at `0x14ede` — that table is some other set of limits.
  Military parked in the Trading Market is safe from pirate raids.
- **Group vs. individual** (interplanetary) — a solo strike returns double;
  a group attack shares the returns.

**Clingy Annihilator** (the clone's equivalent of BRE's *Gooie Kablooie*) — the
ultimate weapon, aimed at an entire enemy planet rather than one empire, and one
per planet at a time. IB implements the original's lifecycle (#16): begin
construction against a named planet → any baron funds it a million gold at a time
→ complete → awaiting launch → in flight → arrival. It can be dismantled before
it flies and the gold is not refunded.

**Who may call it off is the elected BBS Coordinator**, from item 1 of the
Coordinator menu — the original's *Dismantle Gooie*. Standing a strike down is a
diplomatic lever rather than a change of mind: the planet makes peace and calls
off the weapon still aimed at its new ally, over the builder's head (#45, built
2026-08-18). Two divergences remain, both **#114**: IB still lets the baron who
started it dismantle it from the weapon's own desk, and still asks them whether
to launch, where the original launches by itself once the funding completes.

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
spent whether they connect or not. The SDI does not touch it: the original's SDI
percentage is read in four routines and this is not one of them (see "SDI
Defense"), which matches its instructions saying jets are the only answer. IB scaled the
damage by the defender's SDI until 2026-08-14; that was IB's own and is gone
(#111).

**That reconstruction is still wrong in one known way** — it needs a live game,
or the original's code, before it can be called done:

- **The weapon should not detonate once and vanish.** The original's own
  instructions describe a siege: 10% of the planet's regions instantly on
  arrival, "every day of it's existance after the first day, another 5% ... up
  to a max of 5 days at which time it will self-destruct", with jets battling it
  the whole time it sits there. IB has no post-arrival phase at all, so the
  weapon costs a planet a tenth of its land instead of up to a third, and the
  cooperative defence the original is built around never happens. (#112)

The target planet is told when the weapon **launches**, with the arrival time in
hours, which is what makes interception possible (#63). **It is not told while
the weapon is being built** — that is what a SpyGuy is for. Both of BRE's
construction strings, "Gooie Kablooie destined for our planet is under
construction at ..." and "Gooie Kablooie arrives from ... in N Hours.", belong
to `show_gooie_arrival_time`, whose only caller is the SPY_GUY packet receiver,
and its funding and dismantle reports are gated on the target's spy counter as
well. IB broadcast the whole build to its target for free until 2026-08-18,
which left a watcher with nothing to report that his planet did not already
know.

**SDI Defense** — a funded anti-missile/anti-jet shield. The original names
three separate ceilings: it destroys **up to 50%** of incoming missiles, and
reduces attacking **jets by up to 30%** and **bombers by up to 20%**
(`game/breins.txt`). All three are BINARY-VERIFIED, along with what the shield
does NOT reach — see "The SDI program" below for the funding, the strength curve,
and the reach.

Per-day caps (config): individual 4, group 4, terrorist 25, bombing 4.

**The three special-attack switches remove their operations from the menus.**
`game/reset.hlp` describes Bombing Operations, Missile Operations and Gooie
Kablooies (IB's Clingy Annihilator) as classes of special attack a sysop may
disable outright, so IB hides rather than refuses — `byKey` skips a hidden item,
so the hotkey goes with the label:

| Setting | What it hides |
| --- | --- |
| `BombingOps` | Bomb Enemy Targets on the Covert menu, and Bomb Food Market / Bomb Trading Market / Bomb Trade Routes / Undermine Investments on Special Operations |
| `MissileOps` | Nuclear / Chemical / Biological Attack on the Attack menu, and Nuclear Assault / Chemical Bombing / R5-Slappenheimer on Special Operations |
| `ClingyAnnihilator` | Clingy Annihilator Ops on the InterPlanetary menu |

**IB's reading, not a capture:** BRE words both as
inter-BBS settings about attacks sent to another board, and no capture of either
one disabled exists, so which menus the original strips is unverified — but a
switch the player is told about on Game Setup has to do something.

### Local Attacks, Local Attack Scoring, Dupe Checking

Three more inter-BBS-only settings, all from BRE's Configuration Editor page two
and documented in `game/reset.hlp`. Each is off-league inert: IB checks
`Config.IBBS` first, because BRE scopes all three to interplanetary games.

- **Local Attacks** (default **Enabled**; the original's help "highly
  recommends" leaving it on, "forcing BBS's to use teamwork to remove
  troublemakers"). Disabled, barons on one board may not attack each other. The
  Attack Menu then collapses to the pirate and alliance entries — **captured
  live**, `docs/dev/bre-screens.md` "Attack Menu (InterBBS, local attacks OFF)" —
  so Regular, Nuclear, Chemical and Biological all go, hotkeys with them. IB
  binds its AI barons by the same switch.
- **Local Attack Scoring** (default **Disabled**, which is how BRE ships it and
  what its help recommends: "so that users cannot attack each other just to
  build up score"). Disabled, a local battle moves neither side's score;
  `whatsnew.doc` records the change as "score is no longer given for winning
  local attacks". Whether BRE also suppresses the LOSER's penalty is unverified;
  IB suppresses both, so the pair cannot be used to grind a rival down.
- **Dupe Checking** (default **Enabled**). BRE looks "for users on your system
  that may be playing on other BBSes and temporarily lock them out of the game
  (until they delete one of their players)". IB does the same from the scores
  packet, and **deliberately diverges on how**: BRE compares handles, IB compares
  a 64-bit hash of the normalized handle (`RemoteScore.OwnerHash`), so a packet
  that lands on every sysop's board carries nobody's handle. A locked baron is
  refused at login with the board that reported them; the lock lifts when that
  board stops listing them, or when the Coordinator turns the switch off —
  `World.DupeLocked` reads the switch at the gate rather than clearing state.

All three ride the `LeagueConfig` broadcast, since a league has to agree on them.

**Which settings the Coordinator broadcasts follows one rule: anything that
changes how the local game plays has to be the same on every planet, or the
season is not a fair one.** Only a board's identity, its file paths, and its
session policy (the idle-caller timeout and its warning count) stay local; so
does the AI count, which a league board never uses. Note this is a WIDER set
than the fields the Configuration Editor stars — the star means "an inter-BBS
option", which a rule like the tax cap or the pirate factions is not, though both
still have to match across the league. `TestEveryGameRuleIsBroadcast` holds the
line: a field added to `Config` must be either in `LeagueConfig` or in
`perBoardConfigFields` with the reason it is not a rule. It found three on the
wrong side when it was written — idle-realm removal, the money cap, and
unlimited food, each of which would have let two planets play a different
game.

**A terrorist op costs `(opsToday + 63) × total regions` gold — BINARY-VERIFIED.**
The InterPlanetary Operations menu prices the op in its own cost column, and the
price scales with the launcher's **region count** and **daily op count**, rising
as a realm buys land or launches more ops. At opsToday ≤ 1 the per-region cost
is 64 (confirmed against four captures); each additional op adds 1 per region,
up to 163 at the cap of 100. Terrorism Costs scales the result:

| Regions | × 64 | Price on the menu |
| --- | --- | --- |
| 8,466 | 541,824 | 541,824 |
| 8,471 | 542,144 | 542,144 |
| 8,474 | 542,336 | 542,336 |
| 8,484 | 542,976 | 542,976 |

Four readings, four exact matches, from one session as the realm's land grew
(opsToday = 0 throughout, so the per-region cost held at 64).
The chain: `launch_terrorist_operation` (`BRE.OVR 0x2afbf`) passes empire field
`+0x276` to the pricing routine at `0x2aca8`, which clamps it with `max_i32(…,1)`
then `min_i32(…,100)`, calls `total_regions` (`056d:0ec6`), and combines them
through Real48. The result is compared against gold before the op is allowed.

The `+0x276` term is the per-day counter — written at `0x864f`, incremented at
`0x2b603` after a launch, read by both the menu and the launcher, and separately
tested against the configured cap at `[0x1040]`. The pricing routine (`ovr_02aca8_entry_0000`)
clamps it with `max_i32(…,1)` then `min_i32(…,100)`, adds 63, and multiplies
by `total_regions`:

	capped   := clamp(terrorOpsToday, 1, 100)
	base     := (capped + 63) × totalRegions
	goldCost := base × configMult

For opsToday ≤ 1 the per-region cost is 64 (matching the four captures above);
each subsequent op raises it by 1, up to 163 at the cap of 100. BINARY-VERIFIED.

IB charges it: `World.TerrorOpGoldCost` applies the same formula, quoted before
the confirm and taken in `SendTerror`. The four Special Operations entries
price themselves on their own menu — see "Interplanetary Special Operations"
below.

### The two cost levels (Attack Costs, Terrorism Costs) — BINARY-VERIFIED

Both are inter-BBS-only settings: Attack Costs scales what an interplanetary
strike charges, Terrorism Costs what a terrorist op charges. **Neither uses the
generic Level ladder** the other presets use (0 / 50 / 100 / 200):

| Level | Multiplier |
| --- | --- |
| None | 0% |
| Low | 20% |
| Medium | 100% |
| High | 300% |

Read out of `BRE.OVR` two independent ways that agree exactly. The attack site
(`0x2bbc2`, config byte `0x182`) branches on the level and divides the price by
Real48 `5.0` or multiplies it by `3.0`; the terrorist pricing routine (`0x2ad9f`,
config byte `0x184`) writes the same spread as literal percents 100 / 0 / 20 /
300, and a sibling site (`0x2ad1a`) repeats the ÷5 / ×3 form on longints. BRE
encodes the byte Medium 0, None 1, Low 2, High 3, which is what ties each figure
to its level; the direction also matches `game/reset.hlp`, which says a High
Attack Costs setting "will make attacking more difficult".

The figures live in `balance.go` as `CostLevel*Pct` and reach the two knobs
through `Level.CostPercent()`. `Level.Percent()` stays for Maintenance / Region /
Attack Damage / Attack Rewards; Trade Deal Costs has a third ladder of its own,
below.

### Maintenance Costs — BINARY-VERIFIED, and wider than it looked

| Level | Effect on upkeep |
| --- | --- |
| None | 0 |
| Low | ÷ 4 |
| Medium | unchanged |
| High | × 4 |

Read at `BRE.OVR 0x2E836` inside `calculate_military_maintenance`, and again at
`0x2E948` inside `calculate_region_maintenance` — the same two charges IB scales.
Both switch on config byte **+0x180** and divide by Real48 `4.0` or multiply by
it.

**Low is a quarter and High is four times**, so between the two ends the same
army costs SIXTEEN times as much to keep. IB applied the generic ladder's half
and double until this was read, which understated the knob badly at both ends.
`Level.MaintCostScaled` now carries it.

### Trade Deal Costs — BINARY-VERIFIED, and a ladder of its own

| Level | Effect on the per-day transit rate |
| --- | --- |
| None | 0 |
| Low | ÷ 6 |
| Medium | unchanged |
| High | × 3 |

Read at `BRE.OVR 0x5158F`, inside the trade-deal cost routine: it switches on
config byte **+0x186** (BRE's own encoding, Medium 0 / None 1 / Low 2 / High 3)
and either leaves the figure alone, zeroes it, divides by Real48 `6.0`, or
multiplies by `3.0`. The two runtime calls it uses are the same pair the
attack-cost site at `0x2BBC2` uses with `5.0` and `3.0`, which is what pins
divide against multiply — that site was verified independently, so this reading
rests on a case already checked rather than on a fresh guess.

**Note the Low arm divides by SIX**, where the attack pair divides by five and
the generic presets halve. Three knobs, three ladders. IB assumed this one
matched the generic ladder until the routine was read; `Level.TradeCostScaled`
now applies the original's arithmetic, and the test pins the figures as golden
literals.

### Region Cost Change — BINARY-VERIFIED, and a big-realm surcharge

It does not scale the price. Config byte **+0x185**, read at `BRE.OVR 0x3019C`,
selects a value and the routine then does:

```
flag  = (regions >= 300) ? 1 : 0        -- cmp against 0x12C
climb = 33 + flag x value               -- mul [bp-0x6], add 0x21
```

| Level | Value added | Climb below 300 regions | Climb at 300+ |
| --- | --- | --- | --- |
| None | 0 | 33 | 33 |
| Low | 15 | 33 | 48 |
| Medium | 35 | 33 | 68 |
| High | 55 | 33 | 88 |

So the knob is **inert until a realm passes 300 regions**, then steep — and it
moves the per-region CLIMB, which the region count then multiplies, so the effect
on the price of the next region grows with the realm.

This also settles where IB's `LandPerRegion = 33` came from: it is the binary's
own `add ax,0x21`, not an approximation. Live sampling read a flat 33 because
every realm sampled was under the threshold, so the knob had never engaged — and
33 was right all along, for the realms that were measured.

IB used to multiply the whole price by a percentage of the level, which is the
wrong shape and taxes small realms the original leaves alone.

### All five cost knobs, side by side (#56)

Every one is now read from the binary, and **no two share a ladder** — which is
the point worth carrying away, because IB had assumed a shared one and was wrong
about three of them.

| Knob | Config byte | None | Low | Medium | High | Shape |
| --- | --- | --- | --- | --- | --- | --- |
| Maintenance Costs | +0x180 | 0 | ÷4 | ×1 | ×4 | scales upkeep |
| Region Cost Change | +0x185 | +0 | +15 | +35 | +55 | adds to the climb, only at 300+ regions |
| Trade Deal Costs | +0x186 | 0 | ÷6 | ×1 | ×3 | scales the transit rate |
| Attack Costs | +0x182 | 0 | ÷5 | ×1 | ×3 | scales an interplanetary strike |
| Terrorism Costs | +0x184 | 0 | ÷5 | ×1 | ×3 | scales a terrorist op |

Trade Deal Costs was the one reaching nothing at all: stored, shown on Game Setup
and broadcast to the league while no gameplay read it, so a sysop could set it to
None or High and no number moved. A Protective Trade pact discounts what that
setting leaves, not the raw rate.

The **Covert Operations fees** (`Cost*`) are a different case and #56 should not
be read as covering them. There is no covert cost knob among the five, and none
of those fees is a literal in either binary in 32-bit or Real48 form, so the
figures IB sampled cannot be treated as one point on a ladder. Scaling them is
**unevidenced**, not merely unbuilt; sampling a second BRE setup live would
settle it.

"Days before 'lost' forces returned" (`Config.LostForcesDays`, default 3) is an
**inter-BBS** setting, not a local-combat one. A strike sent to another board is
away for the whole packet round trip, and packets go missing; the setting gives a
detachment back to its owner when no result has come home in that many days. IB
implements this (#96): `World.InFlight` records every strike that leaves, a
returning result clears it, and `ReturnLostForces` — run from the planetary step
after inbound packets are applied — hands back anything that has waited too long
and posts news. 0 turns the recovery off.

## Covert operations

Neither realm's agent count changes an operation's odds. That is not IB's
choice — it is what the original's one covert roll computes, and the routine
below is where it comes from. An agent is what an operation SPENDS (one per try,
lost when the try fails); what moves the odds is the operation's own difficulty,
a bribed agent inside the target, an Expose Enemy Ops shield, and the two agent-
lending treaties.

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

**IB resolves EVERY local covert operation through this routine** (`covertRoll`
in `internal/game/covert.go`), defect included, on Andy's call. It is BRE's only
local covert roll: the menu queues each effect op as a type-7 record
(`BRE.OVR 0x04CA06`) and the resolver at `0x04BE9F` drains the queue and calls
this routine once per op — twice for Set Up, once against each court. Send Spy
and Spy on Relations skip the queue and reach it through `report_spy_result`
(`BRE.OVR 0x016D67`). There is no second roll anywhere for the local menu, which
is what retired IB's own attacker-against-defender `covertSuccess`. Verified
2026-08-16 by walking all seven resolver call sites and both info-op sites.

What that means in play:

- **The defender's agents never enter the roll.** This is the startling one, and
  it is deliberate in IB because it is what the original does — a future reader
  will take it for a bug, so: both `covertStrength` calls pass `a`; the bytes are
  identical (`8A 46 10`) at `0x4BAE7` and `0x4BB03`, only the mode byte changes.
  With no alliances `A = agents div kind` and `B = agents`, the agent count
  cancels, and an easy op is a flat `0.1 + 0.9 x 1/(1+1)` = **55%** whatever
  either side holds. The same function is called correctly elsewhere (`0x4AA5E`
  passes a different empire with mode 1), so the site looks like a copy-paste bug
  in the original rather than a design.

  **Consequence: agents defend against nothing on the local menu.** Stockpiling
  them buys staying power against the one-per-failure loss and the attacking side
  of the roll once an Intelligence Alliance is in play, and nothing else. The
  only defences that exist are Expose Enemy Ops and a Terrorist Prevention treaty
  — and the latter, per the entry below, works backwards.
- **The divisor is integer division, so a thin realm is worse off than the
  cancellation suggests.** At 1 agent, `A = 1 div 2 = 0` and a Demoralize Forces
  can only land on the flat one-in-ten. The cancellation to a fixed rate holds
  only once the count is large against the divisor.
- **The two alliance weights are not equal**: 0.5 on the defending side, 0.4 on
  the attacking one. IB lent half in both directions until this was read. Which
  treaty number is which is inferred from the documented effect (an Intelligence
  Alliance helps the attacker, a Terrorist Prevention treaty the defender), not
  read from a name table.
- **A Terrorist-Prevention treaty raises `B`**, which lowers the holder's *own*
  spy success — a direct consequence of the defect above.
- **One roll in ten succeeds before any of this** is weighed. A fourth branch
  (`r = 10`) is dead code: `Random(10)` returns 0..9.
- **The first guard is the Expose Enemy Ops shield, not a bribe.** IB read the
  bribed-agent flag here until 2026-08-16; the Real48 slot the routine compares
  against the clock is the shield's expiry, and the bribe flag is read further
  down as a doubling of `A`. See the two entries below.

**The per-op difficulty divisor** (`kind`) is 1 for Send Spy, Spy on Relations,
Stir Revolts, Set Up and Support Dissensions; 2 for Demoralize Forces and Bomb
Enemy Targets; 3 for Bribery. Read from the seven resolver call sites at
`BRE.OVR 0x04BFD4`, `0x04C0A2`, `0x04C0CA`, `0x04C16E`, `0x04C277`, `0x04C353`
and `0x04C6F5`, plus `report_spy_result` at `0x016D8A`. With no treaties in play
that gives 55% / 40% / 32.5%. IB's `CovertDifficulty*` constants carry them.

**An effect op spends an agent up front** (`BRE.OVR 0x17957` decrements agents as
the record is queued) and hands one back on success (`0x04C69D` and its five
siblings), so the net cost is one agent per failure. IB does both now, and the
distinction is visible rather than cosmetic: between sending an agent and hearing
back, a realm really is one agent short.

**"Limit one try per turn!" is per OPERATION.** The menu indexes a per-digit byte
at record `+0xFD + digit` — read at `BRE.OVR 0x017AE0`, set at `0x017C4F` — so a
turn holds one try of each item, not one item overall. Digits 1 and 6 (the info
ops) skip the check and never set the byte; digit 9 (Expose Enemy Ops) is
dispatched before either. IB matches this with `TurnProgress.CovertOpsUsed`,
keyed by operation.

**Bribery is an OFFENSIVE holding.** The flag lives in the ATTACKER's record
indexed by the target (`BRE.OVR 0x04BA48` at `+0x15D`, set by the resolver at
`0x04CD67`), and a set flag doubles `A` — the attacker's own numerator. It buys
the bribed realm nothing. IB read the same flag backwards, as a shield making
that realm's ops against you fail, until it was checked; it now doubles the
briber's odds (`CovertBribeOffenseMultiplier`). The menu still refuses a second
bribe inside a realm you already hold one in, before charging (`0x01790A`).

**Expose Enemy Ops shields against ONE realm** — one of the realms you already
hold a bribed agent inside, chosen from a list of exactly those
(`bribe_enemy_agents`, `BRE.OVR 0x01701B`, which walks the per-letter bribe array
and lists every entry that is set). It writes `now + 1.0` days into a per-pair
Real48 slot, charges the fee, spends no agent and takes no per-turn slot, and
blocks 9 attempts in 10 from that realm — the tenth lands normally. IB matches
all of that; `Empire.ExposedFrom` holds the per-realm expiry.

> **Not reproduced — an off-by-one in the original.** `bribe_enemy_agents` reads
> the player's chosen letter into `[bp-2]` (the read is at `BRE.OVR 0x0171C7`,
> and `[bp-2]` is what the routine echoes back and tests for RETURN), then writes
> the expiry into the slot indexed by `[bp-1]` (`BRE.OVR 0x0172D4`) — the listing
> loop's counter, which the loop leaves at `'Y'`, the 25th and last realm slot.
> The Pascal source almost certainly said `c` where it meant `ch`. So BRE's
> shield does not land on the realm the player picked: it lands on slot 25, which
> is a real realm on a full board and an empty record on every other, where the
> 600,000 buys nothing.
>
> Re-examined 2026-08-17, weighing three ways to render it. **Reproducing the
> index** is out: a realm's letter in BRE is its permanent index into a fixed
> 25-entry array, while `World.Empires` is unbounded and IB's letters are
> assigned per screen by list position, so `Empires[24]` usually exists and the
> misfire would hit an arbitrary innocent realm instead of missing harmlessly.
> **Reproducing the outcome** — take the 600,000 and do nothing — matches what a
> player on a half-full board experiences, but it is wrong on a full one, and it
> would strand the rest of the routine: the per-pair expiry, the nine-in-ten
> block and the bribed-realms-only picker are all binary-verified, all read by
> `covertRoll`, and nothing else in IB writes that expiry, so the verified half
> of the mechanic would become dead code behind a dead menu item. **IB therefore
> keeps the intended behaviour** — the shield lands on the realm you picked —
> which diverges on one instruction's index and reproduces everything else the
> routine does. The artifact is recorded here instead of built.
>
> One thing behind this is unverified: `bribe_enemy_agents` is the only writer of
> the expiry array found, but the binary was not swept exhaustively for others.
> If another writer exists, the shield is a live mechanic in BRE and the case for
> the intended behaviour is stronger still.

**Each op charges a gold fee up front** (on top of the agent risk), shown as
a cost column on the menu. The fees below are live-sampled from BRE's default
(medium) game setup on 2026-07-21 — other BRE setups scale them, so IB keeps
them as tunable `Cost*` constants in `balance.go`. A failed op still risks
losing the agent, but an op you cannot afford does nothing (charges neither
gold nor agent). The menu footer shows `You have <gold> gold and <N> agents.`

The menu's item order, labels, numeric hotkeys (1-9), and per-op costs are
confirmed from BRE (BRE.OVR string table plus a live capture, #73):

- **(1) Send Spy** — read the enemy's full status. Cost `CostSendSpy` (5,000).
- **(2) Stir Revolts** — propaganda that lowers popular support by
  `Random(4)+5` POINTS, floored at 5. Cost `CostStirRevolts` (25,000).
- **(3) Set Up** — trick d and a second realm into believing the other declared
  war, voiding every treaty between them (useful against a defense pact
  protecting a target). BRE rolls the covert attempt against EACH of the two.
  Cost `CostSetUp` (50,000).
- **(4) Support Dissensions** — agitate d's own troopers into fleeing;
  `Random(10)+10-Random(10)` percent of them go, so 1-19% around a mean of a
  tenth. Cost `CostSupportDissensions` (80,000).
- **(5) Demoralize Forces** — lower enemy military morale by `Random(5)+5`
  POINTS, floored at 5; they fight worse and, if low enough, units desert.
  Cost `CostDemoralizeForces` (80,000).
- **(6) Spy on Relations** — reveal the enemy's treaties. Cost
  `CostSpyOnRelations` (100,000), which IB charges and BRE only advertises — see
  the deliberate divergence below.
- **(7) Bomb Enemy Targets** — ONE flat terror-bombing op, not a submenu (see
  below). Cost `CostBombEnemyTargets` (100,000).
- **(8) Bribery** — buy an agent inside d over to your side, doubling your own
  odds on every later op against d. BRE refuses at the menu, before charging
  anything, when a bribed agent is already in place. Cost `CostBribery`
  (200,000).
- **(9) Expose Enemy Ops** — per BRE.OVR ("Bribed Agent will expose enemy
  operations for 24 Hours"), a one-day shield against ONE realm you already
  hold a bribed agent inside, blocking 9 of its attempts in 10. Spends no agent
  and takes no per-turn slot, so it can be run repeatedly. IB models the 24
  hours as one game-day (`ExposeOpsShieldDays`). Cost `CostExposeEnemyOps`
  (600,000).
- **(V) Visit Bank**.

**Who the target is told about (BINARY-VERIFIED).** The target learns which
realm came after it only when an agent is **caught**. An operation that succeeds
is reported to the target with no attribution at all. Three independent paths in
the original agree, which is why this is treated as the rule and not one screen's
wording:

- the local **Send Spy** / **Spy on Relations** report (`BRE.OVR 0x016d67`)
  files an event on the target naming the caller's realm on the caught branch,
  and files *nothing* on the target when the spy gets away;
- a received **interplanetary agent packet** (`BRE.OVR 0x04a96b`) resolves each
  agent in turn, counts the successes, and files a source-naming line — realm
  plus the sending planet in parentheses, count = agents caught — only when that
  count is short of the number sent. The per-operation lines the target sees for
  the agents that *did* get through carry no source;
- a received **interplanetary bombing run** (`BRE.OVR 0x04a09a`) fails two rolls
  in three, and on failure files the one line in its template that names the
  source; the four success lines name nobody.

IB follows this in `internal/game/covert.go`: every foiled op goes through
`covertFoiled`, which names the attacking realm, and every success event stays
anonymous. The Expose Enemy Ops guard in `covertRoll` routes to the same branch,
so **an exposed realm hands you its name nine times in ten** — which is most of
what the shield buys.

Two paths deliberately sit outside the rule, both because the original puts them
there:

- an **R5-Slappenheimer that arrives from another planet** names the firing
  realm and its planet on impact. BRE reports an incoming sabre as a missile
  strike, alongside nuclear and chemical, rather than as an agent operation. The
  *local* R5-Slappenheimer stays anonymous on a hit — BRE resolves a same-planet
  strike through a report template (`GAME\COVERT.DAT`) that is not shipped with
  0.988 and cannot be read, so only the half with evidence attributes the hit.
- IB's **interplanetary bombing ops** (`applySpecialOp` in `ibbs_special.go`)
  name the sending realm and board on success, where BRE names nobody. This is a
  standing IB divergence, not an oversight: everything else that crosses in a
  packet — attacks, nuclear and chemical strikes — is board-attributed, and the
  packet is signed anyway.

### The local resolver, read 2026-08-16

The six EFFECT ops are queued by the menu ("Covert Agent Sent out") and resolved
out of daily maintenance by **`BRE.OVR 0x04be9f`** — catalogued
`run_ai_covert_operations`, which is a **misnomer**: it walks the queued 19-byte
covert records of every player and is the only resolver the local menu reaches.
It dispatches on the MENU DIGIT, with cases for 2, 3, 4, 5, 7 and 8 only. Digits
1, 6 and 9 are absent because they resolve inside the menu itself
(`run_covert_operations_menu`, `0x017469`): '9' branches at `0xAA3` without
queuing, and '1' and '6' resolve immediately through `report_spy_result` at
`0xB4A` and are exempt from the once-per-turn flag.

**IB queues the six effect ops the same way** (`World.CovertQueue`,
`internal/game/covert_queue.go`), read 2026-08-16 and built the same day. The
menu path takes the fee, the agent and the once-per-turn slot and files a record;
`DailyMaintenance` drains the queue — before `aiPlay`, so an AI's operations wait
a day exactly as a player's do — and files each result on the ATTACKER's event
recap, which is where an asynchronous player reads it. Where the evidence sits:

- **The record.** `commit_agent` (`BRE.OVR 0x01793C`) sets the per-digit byte at
  `+0xFD + digit`, decrements agents at `+0x26F`, then passes a 19-byte buffer to
  the queue writer at `0x04CA06`, which copies `0x13` bytes and appends it as a
  **type 7** record. Its fields, filled at `0x0175A6-0x0175D3`: `+0x00` the
  attacker's 32-bit player ID (record `+0x5D`), `+0x04` the target's, `+0x0C` the
  attacker's realm letter, `+0x0D` the target's, `+0x0F` the menu digit as a
  32-bit. Digit '3' calls a picker first and stores the second court beside the
  target. IB's `QueuedCovertOp` mirrors this, carrying names in place of the two
  IDs and the two letters.
- **It is persisted.** The list helpers (`0490:0025` append, `0490:002A` delete)
  share an overlay unit with `open_and_validate_game_data`, and BRE's maintenance
  is a separate program run (`BRE PLANETARY`), so the record must outlive the
  door session that made it. IB persists `CovertQueue` in `world.json`.
- **The player sees no result at the menu**, only "Covert Agent Sent out". The
  resolver files report lines to BOTH sides from `GAME\COVERT.DAT` — the
  `COVERT_SENDER_HIT` / `COVERT_SENDER_FAIL` categories are the attacker's half.
  That file is not shipped with 0.988, so **the wording is unknown** and IB
  writes its own.
- **A realm that dies in between is not struck.** The resolver re-reads the
  target's stored ID out of the empire record indexed by `+0x0D` and compares it
  with `+0x04`; a mismatch jumps clear of the whole resolution
  (`0x04BF3D-0x04BF6A`, branching to `0x04BF5C` and out). Nothing is refunded. IB
  has no fixed 25-letter roster to re-check against, so it checks the realm is
  still alive, refunds nothing, and files a line telling the attacker why —
  otherwise the fee would vanish unexplained.
- **BRE's own manual corroborates the split**, singling Send Spy out with "This
  operation takes place immediately" and telling the other operations no such
  thing.

**Consequence for play, and it is a large one:** a covert operation cannot soften
a realm you attack in the same turn. IB resolved every operation on the spot
until this landed, which made covert a tactical move; in the original it is a
strategic one, planned a day ahead. The AI's pre-battle demoralize (`aiWageWar`)
was removed with the same change — it could no longer affect the battle it was
paid for, and `aiCovertOps` already makes that judgement a day earlier.

**A whole family of spec figures was sourced from the WRONG resolver.** The
inter-BBS packet path, `resolve_received_covert_operation` (`BRE.OVR 0x04a68e`),
uses a **different op enumeration**, so an address inside it says nothing about
what a local menu digit does. Everything corrected below came from that mistake.

| Menu digit / op | Local resolver site | What it actually does | What this spec claimed | Corrected |
|---|---|---|---|---|
| 2 Stir Revolts | `0x04C00C` | `support -= Random(4)+5`, then floor 5 | `support × 11/13` from `0x4AE61` | yes — `0x4AE61` is inside the inter-BBS resolver |
| 3 Set Up | `0x04C084` | rolls the attempt against BOTH named realms, then zeroes the relation slot in each direction | one roll, and only a Full Defense Alliance broken | yes |
| 4 Support Dissensions | `0x04C178` | `troopers -= Trunc(troopers × (Random(10)+10−Random(10)) / 100)` | "~10% trooper loss" | yes — a flat tenth was IB's own |
| 5 Demoralize Forces | `0x04C2BD` | `morale -= Random(5)+5`, then floor 5 | `morale × 6/7` from `0x4AC91` | yes — `0x4AC91` is inside the inter-BBS resolver |
| 7 Bomb Enemy Targets | `0x04C37D` | `Random(6)+1` picks one of six holdings; it loses `Trunc(held × pct / 100)` (table below) | a seven-item lettered submenu of player-chosen variants | yes |
| 8 Bribery | `0x04C6D1` | sets the bribed-agent flag for the target on the ATTACKER's record, where the roll reads it as a doubling of the ATTACKER's numerator | a shield making the bribed realm's ops against you fail | yes — the flag is offensive, not defensive |

Two figures the same reading corrected on the way past:

- **The floor of 5 on morale and support is not the computer's.** `0x4C02F` and
  `0x4C2E0` sit in this resolver, which runs every player's queued op, so every
  successful Stir Revolts and Demoralize Forces stops at 5. IB applied it only
  after an AI op (`aiCovertFloor`), which has been removed in favour of the
  universal `CovertStatFloor`.
- **A successful op hands the attacker an agent back** (`agents += 1` on the
  attacker's record in every case). Since the menu spends one agent when the op
  is queued, that is the same net result as IB's "lose the agent only on
  failure" and needs no change.

**The Bomb Enemy Targets table** (`0x04C37D` rolls `Random(6)+1`; each slot then
takes `base + Random(spread)` percent of the named holding):

| Pick | Holding (record field) | Percent | Site |
|---:|---|---|---|
| 1 | population (`+0x62`) | `Random(10)+5` → 5-14% | `0x04C39E` |
| 2 | troopers (`+0x76`) | `Random(5)+5` → 5-9% | `0x04C427` |
| 3 | agents (`+0x26F`) | `Random(5)+5` → 5-9% | `0x04C4B0` |
| 4 | tanks (`+0x86`) | `Random(3)+5` → 5-7% | `0x04C539` |
| 5 | jets (`+0x7E`) | `Random(3)+5` → 5-7% | `0x04C5C2` |
| 6 | food in store (`+0x6E`) | `Random(70)+20` → 20-89% | `0x04C64B` |

Picks 1-5 truncate; the food slot calls the rounding helper instead
(`0x04C6C6`). Nothing else is touched, and the player chooses neither the target
holding nor the percentage.

**The per-op cost is a runtime table, not a constant in the file.** The menu
reads a 32-bit fee at `DS:0x57A + keycode×4` — `DS:0x63E` for '1' through
`DS:0x65E` for '9' — which is why neither binary contains 5,000 or 600,000
anywhere. So **BRE's local bombing fee cannot be read out of the binary**: IB's
100,000 stays, being the default-setup figure sampled live on 2026-07-21.

Two smaller things read on 2026-08-16, both settled on 2026-08-17 from a second
reading of the menu:

- **DELIBERATE DIVERGENCE — Spy on Relations charges what its menu says.**
  BRE's item 6 advertises 100,000 (`cap/covert-menu-20260817.cap`,
  `cap/kd3-01.cap`) and takes 5,000: `report_spy_result` serves both info ops and
  subtracts `DS:0x63E`, the slot-'1' Send Spy fee, whichever one called it
  (`BRE.OVR 0x016E73`, the tail of the routine). The menu's own affordability
  gate still measures the advertised 100,000, so what the original does is hold
  you to a price it never charges. **IB charges the advertised 100,000 on
  purpose** (`CostSpyOnRelations`), which makes the op twenty times dearer than
  in the original and shifts covert economics accordingly — Andy's call, made
  knowing that. Do not "correct" this back to 5,000;
  `TestSpyOnRelationsChargesTheFeeItAdvertises` locks it.
- **Expose Enemy Ops IS affordability-checked, in the menu.** The earlier note
  here read the fee subtraction inside `bribe_enemy_agents` (`BRE.OVR 0x0175D9`),
  which does no checking, and concluded gold could go negative. It cannot: the
  menu compares the pressed key's fee against the caller's gold before it
  dispatches anything — `BRE.OVR 0x01775C`, printing "Sorry!  You cannot afford
  that!" — and digit 9 is dispatched after that gate, at `0x0177BD`. IB's refusal
  (`ErrCantAfford`) already matched; nothing changed.

**A realm under New Realm Protection cannot run the effect ops or Expose Enemy
Ops** — the menu tests the CALLER's own shield right before the fee gate, and
refuses (`BRE.OVR 0x017716`, refusal string loaded at `0x01772E`). The predicate
is `056d:19b5`, the same lifetime-turns-against-Turns-Of-Protection test the
Queen's refund cap uses. Digits 1 and 6, the info ops, jump past it (`0x01770B` and
`0x017714`), so a sheltered realm may still spy.

**IB matches this.** `covertOp` refuses the seven effect ops — including Expose
Enemy Ops, which does not share the target picker — while `covertInfoOp` lets
Send Spy and Spy on Relations through. The gate sits ahead of the fee, so a
refused operation costs no gold, no agent and no per-turn slot, and it carries
its own wording: the attack menus refuse because the TARGET is shielded, this
refuses because the caller is.

An earlier reading of this section claimed IB gated nothing here. That was
wrong — the six ops routed through `localAttack` were gated all along, with the
attack wording. The real divergences were that the two info ops were gated too,
and that Expose Enemy Ops and the interplanetary Send SpyGuy were not.

The resolver also files a report line to **both** sides
from `GAME\COVERT.DAT` — categories `COVERT_TARGET_HIT`, `COVERT_TARGET_FAIL`,
`COVERT_SENDER_HIT`, `COVERT_SENDER_FAIL`, `BOMB_TARGET_HIT`, `BOMB_SENDER_HIT`,
`COVERT_MISC`. Its formatter (`0x04bc05`) substitutes the sending realm's name,
the target's, a third realm's, and an amount, so the sender's name *is* available
to the target-side lines. The file itself ships with neither the 0.988 tree nor
the `BREDATA.EXE` payload, so the wording — and therefore whether those lines use
it — is unknown. IB applies the caught-agent rule above to these ops on the
strength of the three paths that can be read, the local spy among them.

**One try of EACH effect op per turn (#54).** "Limit one try per turn!" is keyed
per OPERATION, not per turn overall. The menu computes `digit - '0'`, indexes the
per-digit byte array at record `+0xFD`, and refuses only when THAT digit's byte
is already set (`BRE.OVR 0x017AE0`, set at `0x017C4F`). So a turn holds one Stir
Revolts, one Set Up, one Support Dissensions, and so on. The two *info* ops —
**Send Spy** and **Spy on Relations** — skip the check outright (`0x017AC9`
tests for digits '1' and '6') and never set a byte, and **Expose Enemy Ops** is
dispatched at `0x017AA3` before the check is reached. IB matches this:
`covertCost(..., capped)` records the operation in `TurnProgress.CovertOpsUsed`
and returns `ErrCovertCapReached` only for a repeat of that same operation; the
info ops pass `capped=false` and Expose Enemy Ops does not go through
`covertCost` at all.

> An earlier note here recorded the opposite from live play — Stir Revolts
> followed by Set Up, refused. That could not be reconciled with the code on a
> re-read (2026-08-16), and the code is unambiguous, so the live note is treated
> as a misattributed refusal (a spent agent or an unaffordable fee would give the
> same "nothing happened") rather than evidence. Re-testing it live would settle
> what that session actually hit.

**Bomb Enemy Targets is ONE op, and the bombing submenu is interplanetary.**
Settled from the LOCAL resolver, which is the direct evidence (#159): the
digit-7 case in `run_ai_covert_operations` rolls `Random(6)` at `0x04C3B4` and
dispatches on the result through six branches, each taking a band off one field
— the table above. Six sub-targets chosen by the die, no menu, and no reading of
the eight-item string table anywhere on the path. The ownership argument below
points the same way and is kept because it settles who the eight items belong
TO, but it is not what settles the local op.

The eight-item bombing table at `BRE.OVR 170011` is read by
`run_bombing_operations_menu` (`0x029EA9`) alone, and that procedure's only
caller is `run_interbbs_menu` — so Bomb Food Market, Bomb Trading Market, Bomb
Trade Routes, Undermine Investments, Nuclear Assault, Chemical Bombing, S3-Sabre
and Send SpyGuy belong to the InterPlanetary **Special Operations** menu and to
no other. The 500-Bomber requirement is that menu's too: the gate and the 500
Bombers each launch consumes are both inside it (`+0x1015`, `+0x1146`), and the
local Covert menu tests no bomber count anywhere. BRE's own manual agrees on the
local op — "your intelligence agency will randomly bomb targets".

IB offered a seven-item lettered submenu here (`B/M/R/U/N/C/S`) and charged
100,000 per variant. That was IB's own construction, built by reading the shared
string table as if the local menu took its first seven entries, and it has been
replaced by BRE's single op with the six-slot table above. The interplanetary
Special Operations menu is unchanged and keeps all eight items.

The **R5-Slappenheimer** (the clone's rename of BRE's *S3-Sabre*, avoiding the
original's coined name) is therefore an interplanetary weapon only, and the
sysop's `R5-Slappenheimer Handling` setting now gates it there. A disassembly of
BRE's `SABREHIT` showed only 3 of the 11 dial settings (1, 2, 3) did anything and
the manual never said which number did what, so from the player's seat the result
was random; the target's SDI could intercept it and a heavily garrisoned target
could turn it back on the attacker. IB keeps that feel but makes it honest: the
**dial is a bluff** — the player still sets it 0-10 under User Select handling,
but it changes nothing. The `None` handling mode disables the weapon (gated in
the menu); `User Select`/`Random`/`Constant` all enable it and differ only in the
(inert) dial. The target's SDI intercepts on `Random(100) <= SDI/2` —
BINARY-VERIFIED from the arriving-strike routine (`BRE.OVR ovr_0450a9 +0x481`),
which is where breins.txt's "up to 50% of incoming missiles" comes from and why a
full shield stops half of them rather than halving one. The comparison is
inclusive, so an unshielded realm still turns one shot in a hundred aside. Only
about 3 launches in 10 (`SlappenheimerEffectHits`/`SlappenheimerEffectRange`)
land a payload — the rest fizzle. A landed hit removes a random 5-30 %
(`SlappenheimerBaseDamagePct` + `rng.Intn(SlappenheimerDamageSpread)`) of one
asset, and ~1-in-`SlappenheimerMultiHitOdds` strafes several at once (BRE's
"extremely devastating" outcome). Backfire is a continuous probability scaled by
the target's Troopers (`d.Troopers / SlappenheimerBackfireScale`). BRE hid which
field each effect hit, so IB picks its own spread (Troopers, Jets, Turrets,
Tanks, Bombers, Carriers, Agents, Gold, Food, and Land — Land removed through the
RegionMix so its Total stays equal to `Land`).

The other interplanetary Special Operations, unchanged by this:

- **Bomb Food Market** — destroy a planet's food-market supply.
- **Bomb Trading Market** — destroy a share of what is listed on a planet's
  trading market, and the pending proceeds with it.
- **Bomb Trade Routes** — wreck the goods riding in the planet's pending trade
  deals: two strikes in three come to nothing, each deal has one chance in three
  of being hit, and a deal that is hit keeps 5-9% of every good in it. A deal
  whose own two parties hold Protective Trade is spared (see that pact under
  Diplomacy for the BRE rule and its addresses).
- **Undermine Investments** — trim a quarter off the principal of a planet's
  pending bank investments.
- **Nuclear Assault** / **Chemical Bombing** — the WAR menu's strikes, aimed
  across planets.

**Alliances:** a Terrorist Prevention treaty adds **half** (50%) of an ally's
agents to your covert defense; an Intelligence Alliance adds **two-fifths**
(40%) of theirs to your covert offense. The two shares are not equal.
BINARY-VERIFIED: both are Real48 literals thirty bytes apart in one routine,
`covert_resolution` (`BRE.OVR 0x04cab7`) — relation 4 with the defense flag
multiplies by `0.5` (`800000000000`), relation 5 with the offense flag by `0.4`
(`7fcdcccccc4c`). BRE caps the accumulated total at 1e9; IB does not, which is
unreachable at any real agent count.

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

**Waste** is the ninth region type, not destroyed land. Nuclear and chemical
strikes convert productive regions into it; it counts toward territory, costs the
same region upkeep as anything else, produces nothing, and cannot be bought or
sold. BINARY-VERIFIED (`BRE.OVR` `ovr_02e6b2` +0x458 and +0x4b1), decontamination
is offered in the maintenance sequence — after the SDI upkeep, before the
support and morale boosts — and is optional:

```
allowance = min(max(waste / 5, 10), waste)        -- a fifth per turn, floor 10
perRegion = regionPrice / (2 * technologyFactor(2.0, slot 0))  -- the FOOD factor
cost      = allowance * perRegion
```

The technology factor is the **food** one, not the maintenance one: the routine
passes `(2.0, slot 0)`, the pair the agricultural yield uses, where region and
military maintenance pass `(1.4, slot 3)` and SDI upkeep passes `(2.0, slot 1)`.
Reading those four sibling call sites together is what pins it — and it
independently confirms IB's whole slot/cap table. So an unteched realm cleans at
half the price of new land and a fully teched one at a quarter.

The original's `whatsnew.doc` records the per-turn share being raised from 10% to
20%, which the disassembled `/ 5` confirms. Underpaying is the original's rule
too, not IB's: it divides the gold given by the full bill and scales the count by
the result, so part of the bill cleans part of the pile.

**Cleaned land has no type of its own.** The original moves it to a pool of
unallocated regions (empire field +0xba, an int32 that sits just past the nine
region counts) and asks the owner to name the types, through the same
`[N Regions left]` / `How many <Type> regions?` screen that shares out conquered
land — the screen IB already implements as the #58 capture picker. Conquest,
pirate spoils, a returning interplanetary attack and decontamination all feed
that one pool.

`total_regions` sums the nine region counts and **excludes** the pool, so
unallocated land is not territory until it is placed: it does not count toward a
nuclear strike's damage base, technology dilution, or the scores table's
Territory column. IB's deferred-capture model already matches this.

**Divergence:** IB has no persisted pool. It runs the picker at the moment the
land is cleaned (or captured), and drops any remainder into Coastal if the player
quits the picker early; the original keeps the pool on the empire record and
re-offers it. Because the gold for decontamination is already spent, IB restores
the cleaned land as Coastal *before* asking, and the picker reclaims it — so a
session that drops between the payment and the question costs the player the
choice of type, never the land.

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

**Technology Agreement (#11) — fully verified** (`BRE.OVR`
`process_economic_production`, unit `ovr_033b64`; own research at `+0x34b`, the
partner loop `+0x3c2`..`+0x4cb`). A partner adds an unmultiplied research
contribution, bounded by whichever side holds *fewer Technology regions*:

```
per partner:  round( (min(myTech, partnerTech)^2 / myTotalRegions)^0.75 )
```

So the pact accelerates a realm that is already researching and does nothing for
one holding no Technology at all — IB previously let a tech-less realm inherit a
partner's level, which the original does not.

The two details left open when the mechanic was first written up are settled:

- **What the `min` keeps** — both operands are **Technology region counts**
  (record field `+0xb2`, the eighth of the nine region counts): the researcher's
  own, read through the current-empire pointer, and the partner's, read from its
  record. The denominator is the **researcher's** total regions, not the
  partner's, and the term does not get the ×4. IB's `advanceTech` already did
  exactly this.
- **The second condition on the partner** — the loop skips a partner whose
  `+0x5d` is not positive. That is the slot's in-use marker, not a treaty or
  activity flag. In a live `game.dat` every one of the 24 unoccupied slots reads
  `-1` there while the one occupied slot reads a positive serial, and three saves
  from different games give 921, 932 and 933 — a counter that advances as realms
  are created (the second realm's value is the first's plus one). The binary
  reads or tests the field at 83 sites, always through the indexed
  other-empire pointer and always as `> 0`, and writes it nowhere. IB's
  `alliesOf` filter on `Alive` is the same gate.

The loop walks realms `A`..`Y` testing the **researcher's own** relation row
(`+0xae + 2×index`) for value **6**, and skips the researcher's own slot; the
treaty enum is the one recorded under Diplomacy above. The exponent is computed
as `exp(ln(x) × 3 / 4)` and rounded half away from zero, and the own-research
term's ×4 is an integer multiply on the rounded result — so the two terms round
separately before they are summed and spent on random slots.

**Urban and Technology produce no direct gold** (BRE-verified): Urban is
population housing, Technology is an efficiency multiplier (see the Technology
region above). Food output is covered under the food
section: an Agricultural draw raised by the Technology factor (#20), plus a
share of every river's yield. These income numbers, the caps (2B money / 1.599B
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

**Regular attack strength — BINARY-VERIFIED.** Read out of the attack routine
(`BRE.OVR` `0xF130`–`0xF3A0`), which builds both sides the same way:

```
attacker = troopers x 0.5 + jets    x 1 + tanks x (1.75 + HQ/200)
defender = troopers x 0.5 + turrets x 1 + tanks x (1.75 + HQ/200)
           x (morale x 0.6 + 50)/100 x techFactor(military, cap 1.4)
```

Only the ratios matter, since both sides are scaled identically, so IB doubles
them into whole troopers: **1 : 2 : 3.5..4.5**. Notes:

- **A finished HeadQuarters adds one trooper's worth per tank**, not two — the
  HQ term spans 1.75 to 2.25, i.e. half a tank across the whole range. IB's
  earlier 3..5 curve read *breins.txt*'s "a tank is about the equivalent of four
  Troopers" as a base when it is the HQ-50 mid-point.
- **Full morale fights ABOVE par**, at 110%, and a broken army at 50%. IB had
  derived the slope from the floor (`100 - 50`), capping effectiveness at 100.
- **Technology multiplies combat strength**, applied inside this computation via
  the tech routine at `056d:1a07` with slot 5 and cap 1.4. This is why an attack
  figure taken from a capture cannot be used as a constant unless the realm's
  technology factors are known — see the `bre-gather` skill.
- The empire record's military block is `+0x76` troopers, `+0x7a` **bombers**,
  `+0x7e` jets, `+0x82` turrets, `+0x86` tanks, `+0x8a` carriers, `+0x8e`
  morale, and HeadQuarters sits apart at `+0x26b` (all confirmed against the
  Spending menu's key dispatch at `0x163d0`).
- A separate pass scans every empire for treaty type **7** (Full Defense
  Alliance) and contributes **30**, which matches IB's `AllyDefenseContribPct`.

**A lost defence damages the HeadQuarters**: `Random(3)+5` points off, clamped
at zero (`0xFFA2`, subtracted via `0c03:0fe3`). So a HeadQuarters is not
permanent — a realm under repeated attack loses the tank bonus it spent 20 turns
building. IB had no HQ damage at all.

**A total victory takes the loser's whole military.** The "crushed the enemy
completely" path transfers all six unit types — troopers, jets, turrets, tanks,
bombers and carriers (`0x101E4`-`0x10294`) — which is what "you also get all the
remains of your opponent's military" means literally.

**Allied defenders contribute at their own morale**, on a different curve from
the main one: `allyMorale/2 + 25`, so 25% at broken morale and 75% at full
(`0xF5D7`).

**The regular attack's casualties and capture — BINARY-VERIFIED.** Both were
reconstructed from play until 2026-08-14; they are now read out of the driver
(`BRE.OVR` `0xEF90`) and the resolver it calls (`0xE81F`).

Two sysop knobs drive it, and each has its own table — neither is the generic
`Level.Percent` multiplier applied to a Medium baseline, which is how IB had
modelled them:

| Level | Attack Rewards: share of the defender's regions | Attack Damage: share a side loses before it retreats |
|---|---|---|
| None | 0% | 1% |
| Low | 5% | 10% |
| Medium | 10% | 20% |
| High | **25%** | **30%** |

The rewards table is loaded at `0xFFF9`–`0x10062` (default `0.10`, then a switch
on the config byte at `+0x183`); the damage table at `0xF84F`–`0xF8B8` on the
byte at `+0x181`, where the constants are what *survives* — `0.99`, `0.90`,
`0.80`, `0.70` — so the losses are their complements. Note **None still costs 1%**,
not nothing. `attack.hlp`'s flat "15%" and "20%" describe the interplanetary
variants, not this table.

Capture is `min(defenderRegions, max(15, round(defenderRegions × share)))` —
the floor of **15** is pushed at `0x1009f` and the defender's own region count
into the min at `0x100c3`. Capture depends on nothing else: not on the strength
ratio, and not on any measure of how developed the land is.

**Casualties are an outcome of the fight, not a rate.** The resolver runs a
round at a time:

```
each round, while both sides are above their retreat threshold:
    defender is hit with probability  attackerStrength / (attackerStrength + defenderStrength),
                          or failing that on a flat 5% chance
    attacker is hit the same way, on the defender's share, against the updated strengths
    a hit multiplies that side by 0.99 and takes 1 more off the top
loss reported = 1 - remaining/initial, or 100% for a side driven to nothing
```

So the side that breaks off has lost exactly the retreat share, and the other
has lost only what the strength ratio cost it: an overwhelming attacker is
almost never the side being hit and comes home nearly intact, while an evenly
matched one pays nearly as much as the loser. IB previously handed the winner a
flat 8% and the loser a flat 20% — about right for an even match, and wrong
everywhere else. It also rolled a ±20% jitter over each side's strength before
the fight; that is gone, because the variance belongs inside the battle.

**IB divergence, now confirmed as one:** IB scales the capture by a net-worth
*density* factor (softer, thinly-held land falls faster). The binary reads the
defender's region count and the level constant and nothing else, so this is
IB's own addition rather than an unverified reconstruction of something BRE
does. It is kept deliberately; see `CaptureDensityBase` in `balance.go`.

**Population / migration — BINARY-VERIFIED.** Read out of BRE's end-of-turn
routine (`BRE.OVR` `0xD08A`–`0xD3CC`). This supersedes an earlier partial
reading of the same routine, which had support as an *additive* `Support·90`
term and could not recover the tax curve at all; both are resolved below. IB
previously replaced the whole thing with its own logistic tuning
(`Land·20 + Urban·60 + Agricultural·20 + Support·30`, `1/12` approach, ±8%/turn)
— that is gone.

Carrying capacity:

```
capacity = Σ(regions × weight) × support/90 × 10/max(3, tax) + 50
```

with per-region weights **Coastal 7 · River 10 · Agricultural 8 · Desert 4 ·
Industrial 9 · Urban 102 · Mountain 8 · Technology 7 · Waste 0**. Raw land does
not house anyone — the *mix* does, and **Urban outweighs every other type by more
than ten to one**, so a realm that wants people buys Urban. Support and tax are
**multiplicative**, not additive: 90% support is neutral and a 10% tax rate is
neutral, so a realm at 45% support taxing at 20% carries a quarter of what its
land could otherwise hold. The `+50` is a floor every realm gets regardless of
holdings.

**Waste is zero, read from the binary rather than assumed.** The routine loads
eight region counts — `+0x96` through `+0xb2`, one per weight above — and stops;
the ninth count, Waste at `+0xb6`, is never read (eight loads, seven adds). Ruined
land therefore houses nobody, which is where a nuclear strike gets its bite: the
target keeps every region and goes on paying upkeep on it, but the people those
regions held drain away as the population settles to a capacity that no longer
counts them.

Each turn the population moves toward that capacity:

```
move   = (capacity − People) × (Random(5)+5) / 100     -- 5-9% of the GAP
if move < 0:  move = move × sqrt(tax) / 2              -- leaving is faster, and taxes drive it
move  += Random(capacity/100) − Random(capacity/300)   -- churn, either way
if tax > 50:  move -= abs(move × 0.25)                 -- punitive rate, on top
if move > People/2:  move = People/2                   -- 50%/turn ceiling
People += move  (floored at 0)
```

**Food does not gate migration.** The routine reads the region counts, the
population, support and tax, and never touches the food field at `+0x221`; a
shortfall costs support, morale and, past the threshold, a civil war, and
nothing more. IB suppressed a turn's growth whenever the granary was empty — a
leftover of its own pre-binary logistic model (`6ace5fd`) that outlived the
rewrite — so a realm that fed its people in full at the maintenance prompts,
spending its food down to zero, lost the growth it had paid for. Removed
2026-08-18.

Because the movement is a share of the **gap** rather than of the population, a
realm far below capacity fills quickly and one near it barely stirs. The
`People/2` ceiling is BRE's characteristic explosive growth. Above a 50% tax
rate the realm settles *below* its capacity rather than at it — measured at
about 75-81% of capacity after 40 turns.

**The `sqrt` is verified, not assumed.** The decline branch calls a Turbo Pascal
System-unit real routine (`fd0:1841`) on the tax rate, which the call shape alone
does not distinguish from `Ln`. Resolved by reading the routine itself in the
resident image at `BRE.EXE` `0x13e81`: it seeds its guess by halving the real's
biased exponent (`add cl,0x80` / `sar cl,1` / `add cl,0x80`), sets a 2⁻²⁰
tolerance (`sub al,0x14`), and loops divide → add → halve (`dec al` on the
exponent byte) until convergence — Newton-Raphson for a square root. `Ln` would
carry a `ln(2)` polynomial and would neither halve an exponent nor divide in a
loop. The same routine backs the combat-odds code at `0x4a81c`, which is
therefore also `sqrt`.

**Unit conversion — millions to people.** BRE stores population as a **16-bit
count of millions** (record `+0x62`; the status screen prints "Population: 101
Million" and migration reports "gained N million people"). IB counts people
directly, and **twenty of IB's to one of BRE's** — so the weights and the `+50`
above, all in BRE's unit, are multiplied by 20 (`PopBREUnitScale`) to give a
capacity in IB's.

The factor is pinned by the two games starting the same realm. BRE's new realm —
2 Agricultural, 5 Desert, 5 Mountain, 3 Coastal, 100 troopers, 1000 food, 100%
support, 15% tax, which is IB's starting mix exactly — reads "Population: 100
Million" against a capacity of 121, so it opens just *under* capacity and grows.
IB starts that realm at 2000 people. `TaxGoldPerCapita` already carries the same
factor: BRE's new realm earns 5183 gold at 15% tax, about 345 per BRE unit,
which is IB's 17 × 20.

Leaving the conversion out was a real defect, not a theoretical one: it put the
starting realm sixteen times over its own capacity, and a new baron who changed
nothing lost about 300 people on their first turn.

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

**Popular support and military morale** are 0–100 stats, held as `int32`s on the
empire record at `+0x92` and `+0x8e`. Each turn's payment stage prompts for both
when they are below 100 ("N gold is requested to boost popular support / improve
military morale").

**Both must sit at exactly 100 for the silent Auto-Pay Maintenance branch to
run** (`BRE.EXE` flat `0x3b12`–`0x3b6d`, alongside "gold ≥ due" and "no waste").
So anything that nudges either off 100 puts the baron back through the manual
payment sequence — which is why the two carry more weight in the original than a
status-screen figure suggests. IB mirrors the gate (`internal/menu/gameflow.go`,
`paymentStage`).

Every input and effect below was read out of the binary by taking the **complete
access list** for the two fields: 62 sites in `BRE.OVR` and 4 in `BRE.EXE`.

**What raises them**

| Input | Effect | Address |
|---|---|---|
| Founding a realm | both set to 100 | `BRE.EXE 0x8D99` / `0x8DA7` |
| Paying the boost | see the formulas below | `BRE.OVR 0x2F740` / `0x2F91E` |
| Tax under 10% while support < 85 | support `+ (10 − tax)` | `BRE.OVR 0xCE97` |
| Tax under 30% | support `− (tax−30)/10`, i.e. a gain | `BRE.OVR 0xCE97` |

**What lowers them**

| Input | Effect | Address |
|---|---|---|
| Underpaying forces / regions / crown tax / food | the five shortfall penalties tabled above | — |
| A tax riot | support `− tax/3` | `BRE.OVR 0xCE97` |
| Support under 10 | morale `− (10 − support)` | `BRE.OVR 0xCF9C` |
| Civil war | support halved | `BRE.OVR 0xC5C8` |
| Breaking a treaty by attacking | both `× 3/4` (integer) | `BRE.OVR 0x1A881` |
| Chemical strike on you | morale `× 3/4` rounded, support `× 2/3` rounded | `BRE.OVR 0x110AE`, `0x11109` |
| Biological strike on you | morale halved, support `× 2/3` rounded | `BRE.OVR 0x115FE`, `0x11645` |
| Demoralize Forces against you (local) | morale `− (Random(5)+5)`, floor 5 | `BRE.OVR 0x4C2BD` |
| Stir Revolts against you (local) | support `− (Random(4)+5)`, floor 5 | `BRE.OVR 0x4C00C` |
| Demoralize Forces arriving in a packet | morale `× 6/7` | `BRE.OVR 0x4AC91` |
| Stir Revolts arriving in a packet | support `× 11/13` | `BRE.OVR 0x4AE61` |
| A Free Trade Agreement partner in worse shape | drags you toward them (NOT built in IB) | `BRE.OVR 0x99BF` |

Both are clamped to `[0, 100]` where they are applied. **Every** covert op — not
only an AI's — floors its victim at **5** of either stat (`BRE.OVR 0x4C02F`,
`0x4C2E0`, both inside the resolver that runs every player's queued op). The two
packet-path rows are recorded for completeness: IB has no received-covert-op path
yet, so nothing reads those two ratios today.

**What they do**

- Low popular support cuts **Coastal income** (`0.1 + 0.9 × support/100`) and
  **population capacity** (`× support/90`), and below **35** it puts a riot line
  in the planet news at 1-in-20 a turn (`BRE.OVR 0xD5AD`) — cosmetic, unlike the
  tax riot.
- Military morale scales **combat effectiveness** (`morale × 0.6 + 50`, so a
  full-morale army fights at 110%) and drives **desertion**, below.
- Both are shown on the status screen (`BRE.OVR 0x1969A`, `0x19B11`) and ride the
  inter-BBS score packet (`BRE.OVR 0x4AB5A`).

**Desertion — binary-verified (`BRE.OVR 0xC1F9`–`0xC2D5`).** Once per turn BRE
draws a percentage from a band chosen by morale:

| Morale | Rate |
|---|---|
| 0–9 | `22 + Random(7) − Random(17)` |
| 10–19 | `17 + Random(5) − Random(12)` |
| 20–29 | `10 + Random(3) − Random(8)` |
| 30–39 | `5 + Random(2) − Random(5)` |
| 40+ | none |

A rate of zero or less costs nothing, which is why the milder bands often pass
without a loss. Each of **Troopers, Jets and Tanks** then loses `count div 100 ×
rate`, independently, with a 1-in-4 chance of being spared; the same is taken
from anything of that type escrowed on the Trading Market. **Turrets, bombers and
carriers never desert** — the routine loads three unit types and stops.

**There is NO free morale recovery.** Nothing anywhere in the binary adds to
morale except the boost the baron pays for. IB used to drift it back 4 points a
turn; that is removed.

**No auto-pay for the boost, deliberately (#39).** The prompt appears every turn
support sits under 100 and there is no preference to pay it automatically. That
is not a missing convenience: BRE's silent Auto-Pay Maintenance branch is gated
on popular support and military morale BOTH reading exactly 100, so a realm that
has slipped is routed through the manual sequence precisely so the boost is put
in front of the player. A toggle that paid it unattended would remove the
decision the original built the gate to force.

**The support boost — binary-verified (BRE.OVR 0x2F4C4 and 0x2F740):**

```
deficit = min(100 - Support, 15)
cost    = deficit × (3 × People + 500)          # People in millions
points  = deficit × (given + 1) / (cost + 1)    # truncated
maximum payable = cost × 3 / 2
```

Reproduced exactly by two live prompts: 216,366 gold at 23,874M people and
218,139 at 24,071M, both three points short, each restoring exactly 3 points.
The `3 × People` term is per BRE unit, so IB divides its own count by
`PopBREUnitScale` before applying it (see the population section) — without that
the crown charges twenty times the price.
Note the deficit **charged for** is capped at 15, but the award is a plain ratio
of what you paid — so **overpaying by half buys 22 points, not 15**. That is the
original's behaviour, not an IB addition. The `+1` on each side is the same shape
the crown-tax penalty uses.

**The morale boost — binary-verified (BRE.OVR 0x2F6BA, 0x2F6CA, 0x2F82C and
0x2F91E):** the same shape, priced off the ARMY rather than the population.

```
deficit = min(100 - Morale, 15)
cost    = deficit × (0.10·Troopers + 0.05·Jets + 0.10·Turrets + 0.15·Tanks) + 500
points  = deficit × (given + 1) / (cost + 1)          # truncated
maximum payable = cost × 3 / 2
```

Three differences from the support boost worth noticing, all read from the code
rather than inferred. **Bombers and carriers are not priced at all** — the
routine loads four unit counts and stops. Units escrowed on the **Trading
Market** are counted, exactly as they are for maintenance. And the flat 500 sits
*outside* the multiply here, where the support boost has it inside
(`deficit × (3·People + 500)`), so a realm with no army pays a flat 500 whatever
its deficit. The cost is capped at 2,000,000,000 (`BRE.OVR 0x2F7E5`).

IB's earlier placeholder charged a flat 100 gold a point up to 20 points a turn.

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
- **Emigration is NOT a gameplay mechanic.** Misrule attrition is **riots (tax)
  and the civil war (food, land upkeep) only** — no realm ever loses people
  merely for being badly run, and none loses people to starvation either.
  BRE's tiered "most of your empire has left your rule" / troops-fleeing
  *messages* sit in the block the **crack/registration check** reaches
  (`BRE.OVR 0xC4F3`, behind three global flags), so they are seen only when a
  pirated copy is detected.

  **Correction to an earlier reading.** That check was previously recorded here
  as the *only* nonzero setter of the civil-war severity byte (`+0x2bb`). It is
  not: taking the byte's whole access list finds three setters, one of which is
  the crack check (a flat 50) and two of which are ordinary play — see below.
  The earlier note was made from the two sites nearest the question rather than
  from the full list, which is exactly the failure mode
  `.claude/skills/bre-gather/references/disassembly.md` warns about.
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

### Civil war — binary-verified, IMPLEMENTED

BRE keeps a **civil-war severity percentage** on the empire record at `+0x2bb`,
files into it during the turn, and spends it in the civil-unrest routine
(`BRE.OVR 0xC59A`). It is not a separate subsystem so much as the severe end of
the two shortfalls that can light it:

- **Famine** — the people got under **65%** of their food need:
  `severity += round((1 − r) × 30)` (`BRE.OVR 0x381E5`).
- **Unpaid land upkeep** — under **90%** of region maintenance was paid:
  the same `round((1 − r) × 30)` (`BRE.OVR 0x2F23C`). A far easier trigger than
  famine, and letting land upkeep slide is the fastest way to tear a realm apart.
- A third setter exists — the crack/registration check, a flat 50. Not a
  gameplay input.

When it fires, at severity `S` percent:

- popular support is **halved** (`BRE.OVR 0xC5C8`);
- `S`% of the realm's **regions** are destroyed, removed proportionally across
  the nine types, and returned to the planet-wide land pool the game sells new
  land from (config record `+0x20`);
- `S`% of **every one of the six unit types** is destroyed — held *and* escrowed
  on the Trading Market (`BRE.OVR 0xC663`), so a listing is no shelter.

**IB implements all of this** (`internal/game/morale.go`, `resolveCivilWar`),
with one divergence: IB has no planet-wide land pool — its Daily Land Creation
allowance is per-empire — so the destroyed regions are simply gone rather than
resold.

Tax rate, bank interest, and investment rates are configurable (a real
league ran tax 85%, interest 75%).

### Concrete economy numbers (from strategy guides)

**Treat every figure in this section as unverified.** The public player guides
are not tied to a version, so a number in one may describe a build other than the
0.988 this clone is matched against. They are useful for finding what a mechanic
*is*; the figure itself needs the binary or a live game behind it before anything
relies on it.

- **Bank interest: about 1% per turn** on gold held in the bank *while you
  are playing* (investments tie money up until your next login and are
  less useful for this). This is *not* what IB does: its Interest Rate knob is
  read the way BRE's own config help describes it — the return over ten days, so
  the default 50 is 5.0%/day, credited across the day's turns — which comes out
  well under 1% on a turn.
- **An interest cap of 1,599,999,999 is CLAIMED and REJECTED** — gold above it
  earning no interest, and roughly 25–35 million per turn at the ceiling. Nothing
  supports the figure: neither it nor a round 1.6 billion is a 32-bit constant
  anywhere in `BRE.EXE` or `BRE.OVR`, and no capture shows the interest
  flattening. IB carried it as `InterestCap` until v0.0.4 and no longer does —
  the whole balance earns, and the money cap is the bank's only ceiling. Do not
  put it back without evidence that is not a guide.
- **Absolute money cap: 2,000,000,000 — CONFIRMED BY PLAY of the original.** It
  binds three things there: gold in hand, gold in savings, and what may be
  invested in a day. Unlike the interest cap it survives the check that killed
  that one, and the absence of a literal in either binary (32-bit or Real48) is
  consistent rather than damning — a bound tested against a Turbo Pascal constant
  needs no constant of its own.

  IB implements the first two as a **sysop knob** defaulting to that figure, and
  the third differently: it caps **one investment** at 2 billion but does not add
  up a day's investing (`MaxInvestment`). That divergence is deliberate and stays.

  **IB makes it a sysop knob, defaulting to BRE's figure.**
  `Config.MoneyCapBillions` (Configuration Editor: "Money Cap (billions)") is
  the cap in whole billions, read through `World.MoneyCap()`. It defaults to
  `MoneyCapMinBillions` = 2, BRE's own figure, and may be raised to
  `MoneyCapMaxBillions` = 999. Gold credited above whatever it is set to is
  still discarded — the knob moves the ceiling, it does not remove it.

  Two billion is NOT the largest a 32-bit signed integer holds (that is
  2,147,483,647), so it is a rule the original chose rather than the machine
  limit it is usually presented as. IB's money fields are `int64`,
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
- **Food production:** `Agricultural × (300 + Random(5))` per turn, calibrated to
  live BRE (97 Agri → 29,197; 16 Agri → 4,864, both no River) and read from the
  binary (below).
- **Rivers — IB pays gold *and* food every turn (DELIBERATE DIVERGENCE, #29).**
  In BRE each turn an empire's rivers do EITHER hydropower (gold) OR fishing,
  never both — strictly exclusive across 63/63 captured turns, and the
  production routine says why: it rolls `Random(4)` and fishes only on a zero.
  **IB pays both every turn instead**, splitting a river's yield by
  `RiverFishShare`: 75% of the hydropower gold plus 25% of a fishing haul, every
  turn.

  The split is **expectation-preserving** — it is BRE's average, not a buff or a
  nerf — so no rebalancing follows from it. What it removes is the variance. At
  24 rivers the swing is ~121,000 gold present or absent; a player who commits to
  rivers at scale faces millions of gold appearing and vanishing with no way to
  plan around it, and a food source that shows up a quarter of the time is close
  to useless for covering consumption. One constant drives both halves so they
  cannot drift apart when tuned, which is why the share IS the fishing chance.

  **Live measurements (63 turns across five captures):** rivers fished on **19 of
  63 turns, 30%**, which was the original source of `RiverFishShare` — a
  reasonable read of 19/63 but wide enough to hold the true 25%, and the binary
  settles it. Hydropower gold is a clean `5,000 + Random(100)` per region (every
  one of 44 captured figures divided exactly by the river count, 5,002–5,099).

  This does **not** make rivers a food region: a quarter of ~120 is ~30 food per
  region per turn against an Agricultural region's 300, about a tenth. It is a
  steady garnish, not a substitute for farmland.
- **Both food yields are binary-verified, and technology raises the base only.**
  `BRE.OVR 0x33b64` returns `300 × technologyFactor(2.0, slot 0)` — the **food**
  factor — and `0x33ba6` returns `110 × ` the same factor. Their shared caller in
  the production routine then adds the per-turn draw (`Random(5)` for
  Agricultural, `Random(20)` for a fishing river) and multiplies by the region
  count. So the draw is *not* scaled by technology, and the Agricultural
  per-region figures of exactly 300 … 304 seen across six captures are the
  untech'd band. IB paid a flat `RiverFishFood = 124` with no draw and no
  technology; both are now `RiverFishFood (110) + Random(RiverFishRate (20))`
  raised on the base, matching the Agricultural shape.

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
- **Agricultural output is a draw, not a flat rate.** Per region per turn it is
  `FoodAgriBase (300) + Random(FoodAgriRate (5))` — one roll per turn shared by
  every Agricultural region, exactly like the gold regions. Across six captures
  and nine region counts (2 … 194) every printed total divided exactly by its
  region count, and the per-region figure landed on 300, 301, 302, 303 and 304 —
  all five, so the width is 5 and the floor is 300; the binary read below says
  the same. (IB paid a flat 300 until 2026-07-30, i.e. always the bottom.)
- **Food growth is a *turn-start* credit (matches BRE).** This turn's food yield
  (Agricultural draw + river fishing) is added to the realm's food at the **start**
  of the turn — alongside military production and gold income, exactly what the
  start-of-turn income report announces (`World.GrowFood`). So the player can
  **sell or spend this turn's growth the same turn**. (Earlier IB deferred the
  growth to the end-of-turn economy step, where it arrived after the food market
  and was always subject to spoilage — corrected 2026-07-20 after driving BRE.)
- **Food consumption is TWO obligations, not one (#91).** BRE bills the people
  and the armed forces from two separate routines, prompts for them one after
  the other, and truncates each on its own:

  ```
  people = trunc(populationInMillions × 1.5)
  forces = trunc((troopers × 0.5 + everyOtherMilitaryUnit × 0.01) × 0.01)
  ```

  Both are read straight out of the binary — `BRE.OVR 0x37418` (people) and
  `0x37459` (armed forces), the two need routines in the food overlay unit,
  called back to back by the allocation routine at `0x37fdf` that prints
  `Your People Need N units of food` and then `Your Armed Forces Require N units
  of food`. The prompts default to as much of the stored food as the obligation
  asks for, and if either goes unmet BRE warns that the decision *may lead to
  DISASTEROUS results* and offers to reconsider, which restarts both prompts.

  **The people** eat 1.5 per million, exactly. Nine live samples from 100M to
  34,600M are all exact under truncation (4,081M → 6,121, i.e.
  `trunc(6121.5)`). IB counts people directly, `PopBREUnitScale` (20) to BRE's
  million, so the conversion runs through that constant —
  `FoodPerBREPopUnitTenths` in `balance.go`.

  **Every military unit type eats**, which BRE's own changelog states outright:
  *"All military units now require food to survive"* (`docs/whatsnew.doc`,
  0.97). The armed-forces routine sums **twelve** terms as one real and
  truncates once — six unit types, each counted both where it is held
  (record `+0x76 … +0x8a`) and where it sits escrowed on the Trading Market
  (`+0x211 … +0x231`), so listing an army for sale no more dodges its rations
  than it dodges its maintenance. Food (`+0x221`) and agents (`+0x229`) are
  **not** among the twelve, so neither eats — which settles the covert-agent
  question that live play could only bound.

  Troopers carry weight `0.5` and everything else `0.01`, and the whole sum is
  then multiplied by `0.01`: **1 food per 200 troopers, 1 per 10,000 of
  everything else**. Both rates are corroborated in play — 42,259 troopers →
  211 and 7,212 → 36 for the trooper rate; **30,000 tanks bill 3 and 29,999 bill
  2** for the other, which allows only 9,999.67 … 10,000 and proves truncation
  over rounding. `breins.txt` gives troopers "the added need for food, as
  compared to other units", which is comparative and matches the 50× weight.

  **The two obligations are truncated separately, then summed.** Four turns of a
  combined `N units of Food consumed` line discriminate the models (25,865M +
  49,840 turrets billed 38,801, where one accumulator gives 38,802); the same
  holds at 219,032 and 278,857 turrets. Summing before truncating reads one unit
  high whenever both terms have a fractional part.

  Food pressure still comes from population, not the army: the non-trooper
  charge is ~0.02% of a realm's bill, which is why IB's earlier troopers-only
  formula survived so long. The measurement that appeared to clear jets and
  tanks added 1,000 jets and 533 tanks — 0.15 food between them, so it had no
  power to detect either.
- **Food spoilage:** **5% of the food remaining after growth and consumption**
  spoils each turn — `floor(0.05 × food)` — reduced by Technology regions, and
  **only once the stock passes 1,000**. **Verified by driving the original
  (2026-07-20):** spoilage matched `floor(5% × food-after-grow-and-consume)` to
  the unit at three stocks (1,452→72, 2,668→133, 0→0), and **again at scale
  (2026-07-30)** over every capture: 62 of 63 turns matched to the unit, once
  food sold or bought at the market between the stock line and the spoilage line
  is counted. Truncation, not rounding (29,759 → 1,487, from 1,487.95).

  The 1,000 floor is **binary-verified** — BRE's decay block (`BRE.OVR 0xd8ef`)
  sums the stored and market-listed food and jumps past the whole step unless
  the total exceeds 1,000, which is why a fresh realm's starting food never
  rots. A 2026-07-11 read had hypothesised the floor **and** that only the
  excess decays; the live driving disproved the second half (excess-only gives
  22/83, not 72/133) and the first half was discarded with it. Below the floor
  nothing rots; above it, the whole stock is charged. IB spoiled every stock
  down to the last unit until this was corrected.

  Because growth is credited at turn start (above), selling the surplus down to
  next-turn consumption drains the food after feeding, yielding **zero
  spoilage** — BRE's "sell excess → no decay" behavior. (`FoodSpoilPct` and
  `FoodSpoilFloor` in `balance.go`.)
- **Feeding & food shortfall:** each turn the realm consumes food; a **feed stage**
  (BRE's Payment→Food-Market slot) warns when short, and with **Auto-Feed** on the
  Food Market opens automatically so the player can buy food, then asks the two
  obligations in turn. Going underfed hurts: **popular support and military morale
  drop and people emigrate, scaled to how much of the turn's food need went unmet**
  (`FoodShortfallSupportDrop` = 70 support points, `FoodShortfallMoraleDrop` = 80
  morale points — hungry troops demoralize faster than the public — and
  `FoodShortfallEmigrationPct` = 10% of population, all at 100% unfed). IB's own
  reconstruction, calibrated to a live BRE point: ~73% short dropped support ~50
  points in one turn. breins.txt confirms the direction: *"Without food, morale
  and public support will [decline]."*

  **BRE's own penalties are now read, and IB implements them.**
  The allocation routine files three byte-sized penalties on the empire record,
  all applied and cleared during the end-of-turn step. With `r` the fraction of
  an obligation that was actually given (BRE computes it as `(given+1)/(need+1)`):

  | shortfall | penalty | applied at |
  | --- | --- | --- |
  | armed forces | military morale `-= trunc((1−r) × 40)` | `BRE.OVR 0xc1a1`, clamped to 0…100 |
  | people | popular support `-= trunc((1−r) × 40)` | `BRE.OVR 0xcf41`, clamped to 0…100 |
  | people, and only when `r < 0.65` | **civil war** of severity `round((1−r) × 30)` percent | `BRE.OVR 0xc59a` |

  A civil war **halves popular support and destroys that percentage of every
  military unit type**, and it is the same routine an anti-crack punishment
  fires at severity 50. So BRE's starvation has no emigration at all: the people
  do not leave, the army collapses.

  **IB matches this** (`internal/game/morale.go`, `resolveCivilWar`): the two
  shortfalls are billed apart at BRE's rates, a famine under two thirds of the
  people's need triggers the civil war, and starvation no longer drives anyone
  out of the realm. The rates and the collapse landed together, which was the
  point — either alone leaves starvation weaker than both games intend.
  Food Market opens automatically so the player can buy food. Going underfed hurts:
  **BINARY-VERIFIED (`BRE.OVR 0x38104` / `0x381E5` / `0x382E9`).** BRE bills the
  people's need and the army's need **separately** and scores each on the usual
  `(paid+1)/(due+1)` ratio: the people's shortfall costs `trunc((1 − r) × 40)`
  **popular support**, the army's costs the same in **military morale**, and a
  people's ratio under 65% lights a **civil war** (above). IB feeds the people
  first and the army from what is left, matching the order BRE prompts in.

  **Nobody emigrates.** BRE has no starvation attrition at all — the three
  penalty bytes are the whole of it. IB's own 70/80/10 reconstruction, which also
  drove 10% of the population out, is removed. breins.txt agrees on direction:
  *"Without food, morale and public support will [decline]"* — and says nothing
  about people leaving.
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

  **IB holds the rate in tenths of a percent per day** (`World.InvestRate`),
  which is the unit BRE works in throughout: the Standard Investment Rate knob
  states the return over ten days, so its default 35 is 3.5%/day; the
  Investments screen quotes two decimals ("5.00%") and View Bank Rates one
  ("5.0%"); and the half-point nudge above cannot be expressed in whole
  percents. The floating rate is bounded by the same range BRE allows the knob,
  **3.5% to 10.0% per day** (`MinInvestRate`/`MaxInvestRate`) — a live game whose
  knob sat at the default was observed at 5.0%/day, so the rate drifts well above
  its setting but not without limit. IB held it as a whole percent until
  v0.0.4, in a 1–25%/day band whose ceiling compounded a ten-day term into a
  ninefold return; a save from before the change is converted on load
  (`EnsureInvestRate` — the old band tops out below the new floor, so the two
  units cannot be confused) and clamped into the band.
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
Instructions?" (`BREINS.TXT`). Our clone reproduces the naming prompt, and
**deliberately loosens the rule**: it asks for three VISIBLE characters
rather than three letters or numbers, so a name drawn entirely from CP437's
block and line-drawing glyphs is accepted (#151). Control characters are
refused outright — an escape in a realm name would move the cursor on every
screen that lists it.

## Realm slots: the planet holds 25, and a letter is a realm's name

**A planet holds exactly 25 realms, and each holds one permanent slot.** BRE
keeps its empires in a fixed 25-entry array and uses the letter that indexes it
as the realm's public identity: slot 1 is `A` and slot 25 is `Y`, with `Z`
reserved for "All" in the pickers. Three independent readings agree — the save
record's diplomatic relation row is 25 int16s, "one per empire letter A..Y"
(`docs/dev/bre-save-format.md`); each record carries an in-use marker (`+0x5d`,
-1 when free) rather than the array being packed; and its own config help says
"the normal maximum is 25" (`game/reset.hlp`). The rosters print the gaps: a
captured See Scores board runs `(A)`, `(B)`, `(E)`, and a captured Relations
table `[A]` then `[F]` (`docs/dev/bre-screens.md`).

IB matches all of it.

- **The slot is assigned once and never moves.** `Empire.Slot` is 1..25, and
  every screen that shows an `Id` or takes a selection letter — See Scores, the
  attack picker, the recipient picker, `-*Relations*-`, the letters a message
  records in `Message To  :` — reads `'A' + Slot - 1`. A realm keeps its letter
  however many neighbours die, are pruned, or join, and the See Scores board
  letters by slot rather than by rank, so its rows are not in letter order.
- **Creation is bounded by the slots, not by a separate count.** The lowest free
  slot is taken under the same world lock that checks for one, so two BBS nodes
  onboarding at once cannot both claim it. A caller arriving at a full planet is
  refused with a message and no realm is made. Before this (#144) nothing
  bounded the roster: the pickers stopped at 25 entries while the world kept
  growing, so a realm past the 25th could act on everyone and be acted on by
  nobody, in silence.
- **Callers and computer barons share the 25.** They live in one roster and one
  set of pickers, so barons take slots that callers then cannot have. Barons are
  seeded at reset, so they take theirs first.
- **Max Players Per BBS is a cap on CALLERS within that.** A sysop may seat fewer
  than the planet allows; 0 means "as many as the slots left", not "unlimited" —
  it never bounded the roster the pickers letter, which is half of what made #144
  possible on a default board.
- **A freed slot is reusable at once, and its next holder inherits nothing.**
  BRE does the same — delete the realm holding C and the next realm founded is
  lettered C — so this matches rather than merely being IB's own call. Observed in
  play; the original's purge loop is in the disassembly but its re-seeding side
  was never found there, which is why this was recorded as undetermined until it
  was simply asked about.
  Every removal path goes through one function, which forgets the departing
  realm's treaties, pending offers and market escrow. All of that keys on the
  realm's NAME, never on its slot, so nothing follows the slot to its next
  occupant. The one trace that outlives a realm is the letter an already-sent
  message recorded in its `To` field: a label written at send time, not a
  reference anything follows.
- **Slots are per-planet and never leave it.** Nothing in an inter-BBS packet
  carries one. An interplanetary message names its target realm, and the letter
  is stamped locally when the message is delivered; the inter-BBS scores board
  numbers its rows positionally, since two realms on different planets
  legitimately hold the same letter.
- **A world saved before slots existed gets them on load**, in the saved order —
  which is the order the letters used to come from, so a live board's players
  keep the letters they know. The pass is idempotent, so a reload never
  renumbers anyone twice. A world that had grown past 25 keeps its callers ahead
  of its barons and drops whatever is left over, which was addressable by nobody
  in any case.

### Preferences belong to the player, not to the board

The seven Preferences toggles — the three Visit-menu skips, Enter-exits-BUY,
end-of-turn deposit, Auto-Pay Maintenance and Auto-Feed — are one player's own
settings, held on `Empire.Prefs`.

BRE keeps them the same way, and the disassembly says so outright rather than
the manual: the Auto-Pay flag is read as `cmp byte [es:di+0x339],0` with `es:di`
on the empire record (see "Auto-Pay Maintenance bypasses itself" above), so the
preference is a field inside the player, at record offset `+0x339`.

IB held all seven on the `World` until this was corrected, which made them the
board's: every caller on a BBS shared one set, and each player's choices
overwrote the last player's. The world-level fields survive as migration input
only — `World.EnsurePrefs`, run from `store.repair` on every load, copies them
onto any realm that has not got its own yet, so a board that upgrades mid-season
keeps the settings it was playing with.

Defaults for a new realm are IB's, not BRE's: BRE opens with the three Visit
menus and the two buy/deposit toggles on and the two automations off, so an
untouched realm walks through every optional menu and answers the same
maintenance and food prompts by hand each turn. IB starts with the walk-through
menus off and the automations on.

### Renaming a realm (IB's own; BRE has none)

A realm may be renamed **once**, from the Preferences menu, and never again.
The item is listed from the start but refuses while the realm is under New
Realm Protection — a realm nobody may touch should not also be able to shed the
name its rivals know it by — and disappears from the menu once the rename is
spent. `Empire.FormerName` holds the previous name and is what makes it
once-only. The planet is told in the news.

The realm name is an identity key as well as a label, so `World.RenameEmpire`
rewrites every reference in the same transaction: treaties (re-sorted, since
the pair is held in canonical order), the covert queue, market listings and
market proceeds, treaty offers, barter offers, local mail senders, bribed-agent
lists, `ExposedFrom`, and the Planetary Master fields. Prose already written —
events and past news — is left alone: it records what was said at the time.

What cannot be rewritten is what has already left the board, and two things
cover it:

- **Committed interplanetary forces find their way home regardless.**
  `InFlightStrike` and `GroupAttack` record their contributors by OWNER HANDLE,
  not by realm name.
- **A packet addressed to the old name still lands.** `remoteTarget` and the
  inbound trade path resolve through `FindByNameOrFormer`, so an attack,
  message, recon request, special op or market bid sent before the rename
  reaches the realm. The old name is also held against re-use for as long as
  the realm lives (`RealmNameTaken`), so no second realm can take delivery of
  it.

Other planets see the new name from this board's next score export
(`ImportBoard` replaces a board's snapshot wholesale, so no ghost row is left),
which is the same packet round trip every other cross-board fact takes.

### Sysop edits to a player (`-players`, BRE's `VIEW`)

The original's `VIEW` command lists the players and, for one of them, changes
the caller's user name, changes the empire's name, or deletes the empire behind
a confirmation. IB's `-players` does the same three (#161). It is a command-line
mode, as BRE's is: sysop work, and two of the three are things no player may do
to themselves. Each edit is one locked transaction — lock, re-read, apply,
save — and the prompts run outside the lock, so it is safe to run on a live
board without holding a caller's turn up.

**The interface follows `manage_players` (`BRE.OVR` 0x405a).** BRE browses one
realm per screen: the up and down arrows step through the roster, a letter A-Y
jumps to that realm, ESC leaves, and RETURN opens an action prompt reading
`Delete Empire, Player Name, Empire Name, or Quit?` on the keys D, P, E and Q.
A delete asks `Are you sure?` and takes only Y; a rejected name draws
`Invalid Empire Name.`; an empty roster is said in words rather than drawn as
an empty table.

IB keeps the realm LETTER as what a realm is picked by, the four keys, and
their order. Two deliberate divergences: the arrow-key browse is not
reproduced, since this mode reads a pipe as readily as a terminal and prints
the roster as a table; and BRE's `E` for Empire Name is `R`, because a realm is
what IB calls the thing on every other screen.

- **Change the realm name** — `World.SysopRenameEmpire`, the same rewrite the
  player's rename performs, without the once-only limit or the protection bar.
  The old name goes to `Empire.PriorNames` rather than `FormerName`, so a name
  changed FOR a player does not spend the rename that is theirs to make. Both
  fields take delivery of packets addressed to an old name and both are held
  against re-use; `PriorRealmNames` is what every lookup walks.
- **Change the caller name** — `World.RenameOwner` re-keys the realm to a
  different BBS handle and repoints everything stored under the old one: group
  attack contributors, strikes and bids away on another planet, the Clingy
  Annihilator's builder, and every realm's Coordinator vote. The incoming
  weapon's builder is NOT touched — that handle belongs to another board.
  Nothing needs holding for a packet in the air, unlike a realm rename: every
  interplanetary answer is matched to the local record it belongs to by id, and
  the handle is read off that record rather than off the packet.
- **Delete the realm** — `World.RemoveEmpire`, the path Abdicate takes, which
  already forgets the realm's treaties and market position. The caller may
  build fresh at their next login.

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

**Sending: the recipient picker addresses a LIST, not one realm.** BRE's
`(A-Y,Z=All,?=List) Send to:` prompt toggles: each letter adds or removes that
realm, `Z` toggles the whole `A`..`Y` range at once, `?` prints the roster and
re-draws the prompt, and **RETURN closes the list** (empty = cancel). Anything
else is ignored silently, the caller's own letter and dead slots included. One
composition then goes to every selected realm, and every copy carries the same
`Message To  :` letters in letter order — which is what the boxed capture's
`Message To  : ABCE` is. Verified against the selection routine at `BRE.OVR`
0x1b65e and its per-letter toggle at 0x1b575; layout and colours are in
`docs/dev/bre-screens.md`. Two consequences worth stating outright:

- **`Z` does not send.** It only marks everyone; the message is not composed
  until RETURN. IB used to treat `Z` as a one-key "all" that jumped straight to
  the editor.
- **A picker letter is the realm's world SLOT, not its row number**, so a dead
  realm or your own leaves a gap in the lettering. That is the same letter
  `Message To  :` records, and the same one the `-*Relations*-` roster shows.

Editor: 20 lines under a 68-column ruler, `/S` save, `/A` abort, `/C` clear.

**Not yet built (verified, from the binary).** BRE's *local* reader answers `R`
with a three-way destination prompt — public reply, author only, or select
destinations — keyed `P`/`A`/`S` (`BRE.OVR` unit 0x1d75d: the tests sit at
+0xe02, +0xeb7 and +0xf13). `P` addresses every living realm that was on the
original message's recipient list except the reader; `S` opens the same
multi-select picker as Send Message. IB replies to the author alone, and asks its
two-way `Public Reply?` only for interplanetary mail.

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
- **The picker can address all allies.** On top of BRE's letters and `Z`, IB
  adds `*`, which toggles every realm the sender holds a standing treaty with, of
  any type (Enemy is a relation, not a treaty, so an enemy is never included).
  The key is offered only while they hold one, and it marks the allies on the
  same list — RETURN still closes it, so letters can be added or removed after.
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

**BRE deletes the record, and it does it at daily maintenance — never in the
attack.** Read from a disassembly of the original: the attack resolver
(`BRE.OVR 0xef90`) that crushes a realm only moves its land and surviving
military to the winner and files the news; it leaves the loser's record in place,
holding zero regions. The record is collected by the daily-maintenance sweep
(`BRE.OVR 0x007ed2`), which walks every slot and deletes it when **any** of these
holds: it has no regions left; it has no population left; nobody has played it
for the configured idle limit (`DeletionDays`, default 7, per `docs/bre.doc`); or
it was created and never played and the game is more than three days old. The
slot in play is exempt. Two things follow. A crushed realm stays visible for the
rest of the game day, which is why the loser's neighbours can still see what
happened — IB's one-day husk (`removeDeadHusks`) is the same window. And
abdication has to reach the player "the next day" because it also goes through
this sweep: the deletion routine has exactly two callers, that sweep and the
sysop's Delete Empire.

**Deleting a realm clears its diplomacy, both ways.** BRE's deletion routine
(`BRE.OVR 0x0079c1`) discards the player's messages, trade offers and reports,
then calls a helper (`BRE.OVR 0x050d74`) that walks every other slot and zeroes
the relation **in both directions** — the dead realm's row toward each rival and
each rival's row toward it — before blanking the record and marking the slot free
(the per-record player id is set to -1; that field, not the realm name, is what
BRE tests for an empty slot). IB does the same in `forgetRelations`, called from
`dropEmpires` for every realm any removal path drops: `w.Treaties` rows and
pending `TreatyOffers` are keyed by realm NAME, and a freed name can be claimed
by the next caller to onboard, so a leftover row would hand a reborn realm its
predecessor's alliances and Enemy standing.

**It clears the realm's Trading Market position too.** IB's listings and unpaid
sale proceeds are keyed by realm NAME, like the treaty rows, so a freed name
would hand the next caller to claim it the dead realm's escrowed goods and
unsettled gold. `forgetMarketPosition`, called from `dropEmpires` beside
`forgetRelations`, destroys both — the goods are gone with the realm that owned
them and the gold is paid to nobody, matching the full wipe BRE's delete path
performs on the slot's trade offers.

The market was keyed by the owner HANDLE until v0.0.4, which every computer
baron carries empty (the emptiness is what marks a baron as AI), so the whole
pool shared one position per good: each baron's listing overwrote the last and
the escrowed goods behind the overwritten row were destroyed, and all their sale
gold settled to whichever baron the handle lookup returned first. A world saved
under the old key migrates on load (`EnsureMarket`): a handle that still names a
realm keeps its position, the shared computer-baron row goes to the first living
baron, and a handle with no realm left forfeits its position the way
`forgetMarketPosition` already treats one.

## Interplanetary operations (IB implementation)

InterBBS ops run over file-drop packets. IB matches BRE's player-facing model
(verified against a disassembly of the original binary):

- **Group Attack** — commit real forces, not gold. Each baron sends troopers,
  jets, tanks and bombers (deducted from their army); the pooled detachments are
  the strike's offense, valued by the combat table (trooper 1, jet 2, tank 4,
  bomber `GroupAttackBomberOffense`). Survivors return to each contributor,
  reduced by `GroupAttackLossPct` — including when the strike is refused, so
  attacking a realm that turns out to be protected still costs a slice of the
  force. A group attack gets **no type choice** (confirmed by a live capture —
  its picker goes straight from the target to the force prompts), so it fights
  on the Normal Attack's terms. Its departure delay is set in **hours, floor 12
  and ceiling 120** — BINARY-VERIFIED (BRE.OVR 0x2c38a/0x2c391 push the bounds,
  0x2c3ce/0x2c3d2 re-check the answer, 0x2c40b divides it by 24 and 0x2c41c adds
  the quotient to the clock). BRE therefore stores an **instant** the force
  leaves at, not a game day, and IB does the same (`GroupAttack.DepartAt`): the
  planetary step runs several times a day, so a 12-hour wait really does leave
  before a day-long one, and a strike can be timed to land before the target's
  next turn (#124). Its whole-planet option is still a row in a numbered list
  rather than BRE's `(O)ne Dominion or (A)ll?` keypress, recorded in
  `docs/dev/bre-screens.md` as an open divergence. **Indiv. Attack Force** commits the same four types
  (#62), picks a type (see below), and carries off
  `IndividualAttackReturnsPct` (200%) of what a group attack of the same weight
  would — BRE states the trade in both of its own docs: "You get twice as many
  returns if you send the attack yourself, but you can't attack an entire
  planet" (`game/breins.txt`, `docs/bre.doc`).
- **Individual attack variants.** An individual strike is pressed one of three
  ways. Every figure is fidelity contract, not a playtest knob; the capture
  percentages are relative to a Normal Attack's take, which is how BRE's own
  in-game help (`game/attack.hlp`) states them.

  | Type | Attacker strength | Land taken | Both sides retreat at |
  | --- | --- | --- | --- |
  | Quick Strike | 120% | 50% of normal | 8% losses |
  | Normal Attack | 100% | 100% | 15% losses |
  | Extended Battle | 85% | 125% of normal | 20% losses |

  **Land and losses come from `attack.hlp`; the strength column comes from the
  code, because the two disagree.** BRE's invasion resolver
  (BRE.OVR 0x4055a-0x405a8) switches on the arriving type byte and scales the
  incoming force by a Real48 constant: Quick loads **1.2**, Extended loads 0.85,
  Normal is left alone. The help advertises the quick strike at "110%". The
  disassembly is authoritative for exact constants, so IB applies 120% and the
  in-game help says 120%.

  **Attack Damage rescales the retreat share.** The 8/15/20% above is the
  Medium reading. The same resolver then reads the sysop's Attack Damage byte
  (+0x181, the one the local resolver reads at 0xF85E) and does something
  different with it than the local table: **None** flattens survival to 0.99
  (1% losses whatever the type), **Low** halves the type's loss, **Medium**
  leaves it, **High** doubles it. `Level.InterplanetaryLossPct`.

  The strength multiplier is applied to the offense that leaves the sending
  board; the capture and casualty rates are applied by the **target** board on
  arrival, so the type travels in the packet. Its wire codes are BRE's own —
  **Quick=0, Normal=1, Extended=2** — read from a disassembly of the IBBS attack
  unit in `BRE.OVR`, which presets the type to Normal Attack before drawing the
  menu and writes 0 for `2` and 2 for `3`.

  The menu itself was **captured live end-to-end** (docs/dev/bre-screens.md):
  a 21-column red box with a `(?) Help` item, and **Enter takes Quit**, the
  ordinary menu default. (An earlier reading of the disassembly's `bp-0xa`
  preset suggested Enter picked Normal Attack; the capture disproved it — that
  preset is only the variable's initial value.) The quit key prints BRE's
  `Attack aborted.` Launching also **costs `IndividualAttackGoldPerUnit`
  (1) gold per unit sent** — "This attack will cost 100 gold." for 100 troopers,
  verified for troopers only — and BRE confirms with `Send this Attack? (Y/n)`
  before it goes. Its force prompts ask about **every** unit type, including
  ones held at zero, each defaulting to 0.
- **The round trip.** An interplanetary strike is four steps, not one, and each
  is on a different board's schedule (#107).
  1. **It leaves.** An individual strike goes out on the sending board's next
     planetary run; a group attack waits out its hours first. Either way the
     committed units leave the baron's army at once and the strike is recorded
     on `World.InFlight`, which is what makes step 4 possible.
  2. **It lands.** The target board resolves it on its own planetary run
     (`resolveRemoteAttack`): protection is re-checked *on arrival*, since it
     can lapse in transit; both sides then spend the type's retreat share of
     what they brought — the **defender bleeds units too**, by unit type,
     through the local battle's own `loseForces` — and a winner takes land.
  3. **The answer comes home.** The result carries the survivors per
     contributor, what the strike destroyed (`Enemy`, a `UnitLoss`), the land
     taken, and a **verdict**: BRE's returning report prints one of SUCCESS /
     FAILURE / NOT FOUND / PROTECTED (BRE.OVR strings at 0x04123c-0x041256),
     which `AttackResult.Outcome` carries. A strike that found no such realm is
     not the same as one that was beaten, and the baron is told which.
  4. **The baron reads it and places the land.** Each contributor gets a
     **private event report** — the attack type, the target realm and planet,
     the verdict, what was lost and what came home by unit type, and what it
     destroyed. That is BRE's own report structure, read off its returning-
     attack routine (BRE.OVR 0x04136c); IB's wording is its own. Captured land
     is **parked** on `Empire.PendingRegions` and the **region picker** — the
     same one a Regular Attack opens (#58) — runs at the start of the baron's
     next turn, because the result arrives while they are not in a session. A
     group attack's land is split between contributors in proportion to the
     offense each committed, with the remainder to the largest.

  **A late or duplicate return is discarded.** If nothing is waiting on the
  result's ID, the lost-forces timer has already given the army back (or this is
  a second copy of the packet), and paying the survivors out again would double
  it. BRE refuses the same case in the same place — "Duplicate or Late Attack
  Return Recieved - Packet Deleted" (BRE.OVR 0x04115f) — and `reset.hlp`'s
  "Days for Lost Attacks" entry states the rule in prose: a late return is
  processed as though the assault was never made.

  **News.** BRE keeps six returning-strike lines (`game/ipnews.dat`'s
  `IP-RET-INDIV`, `-GROUPSINGLE`, `-GROUPBBS`, each with a win and a loss form,
  plus `IP-RET-KILL`), because who to congratulate differs: a solo baron is
  named, a group strike is the planet's doing, and a whole-planet raid names no
  enemy realm. IB distinguishes individual from group, win from loss, in its own
  wording; the finer subtypes are still open.
  ones held at zero, each defaulting to 0. The sysop's **Attack Costs** level
  scales that price and BRE clamps the result at `AttackCostCap`
  (200,000,000 gold); see "The two cost levels" below.
- **Protection crosses the league.** A scores packet marks each realm still
  under New Realm Protection, and the attack and terror target lists leave those
  realms out — matching the local attack list, which hides them too. The target
  board still refuses an arriving strike itself, since the flag can go stale
  while the strike is in transit. A protected realm is still a legal SPY target,
  so the spy target list shows every realm.
- **The caller's own shield gates this menu too.** The InterPlanetary menu tests
  it on the same predicate the covert menu uses, at `BRE.OVR 0x020F88`, for
  digits 2, 3, 4, 6, 8 and 9 — Terrorist Ops, Send Trade Deal, Create Group
  Attack, Indiv. Attack Force, Special Operations and Gooie Kablooie. Digits 1,
  5 and 7 (View IPScores, Join Group Attack, Send Message) jump past it. IB gates
  the same items except Join Group Attack, which it still refuses; whether the
  original really lets a sheltered realm join someone else's group attack, or
  refuses it further in, is untested here.
- **Terrorist Ops — the nine operations differ, BINARY-VERIFIED (#166).** Commit
  agents; the op is queued and resolves on the target board's next packet run,
  where `resolve_received_covert_operation` (`BRE.OVR` 0x04a96b) dispatches on the
  operation byte the packet carried. New Realm Protection blocks the lot. What
  each one does to the target, per agent that lands:

  | # | Operation | Field | Effect | Branch |
  |---|-----------|-------|--------|--------|
  | 1 | Send Spy | — | takes intelligence, costs the target nothing | 0x3F0 |
  | 2 | Bomb Intelligence Agencies | agents `+0x26F` | −(2 + `Random(3)`)% | 0x56A |
  | 3 | Demoralize Forces | morale `+0x8E` | × 6/7 | 0x5E9 |
  | 4 | Cause Dissensions | troopers `+0x76` | −(2 + `Random(3)`)% | 0x63C |
  | 5 | Bomb Air Bases | jets `+0x7E` | −(3 + `Random(5)`)% | 0x6BB |
  | 6 | Stir Emigrations | population `+0x62` | −(4 + `Random(7)`)% | 0x73A |
  | 7 | Spread Propaganda | support `+0x92` | × 11/13 | 0x7B9 |
  | 8 | Bomb Food Storages | food `+0x6E` | −`Random(30)`% | 0x80C |
  | 9 | Sabotage HQ | HeadQuarters `+0x26B` | −15 points, flat | 0x87C |

  Turbo Pascal's `Random(n)` returns 0..n-1, so Bomb Intelligence takes 2, 3 or
  4 percent and Bomb Food Storages can take nothing at all. The percentage bands
  are `Random(n)/100 + base` computed in Real48 and multiplied by the field; the
  two ratio operations and the flat HQ hit have no roll in them. The operation
  names come from the report templates in `game/ipreport.dat`, whose eight
  damaging entries are in this order.

  **Each agent is rolled for — BINARY-VERIFIED.** `calculate_combat_odds`
  (`BRE.OVR` 0x04a7a9) is called once per committed agent, and only a winner
  reaches the branch above. It takes one roll in a hundred as an automatic
  success, then one in twenty as an automatic failure, then weighs the two
  covert pools: it takes the ROUNDED SQUARE ROOT of each, gives the defender
  half again its own, and succeeds on `Random(a + d*3/2) < a`. Roots mean
  quadrupling your agents only doubles your edge, and evenly matched realms land
  about two agents in five. Both figures are halved together until the total
  fits 30,000, which guards the original's 16-bit `Random`.

  The attacker's pool cannot be measured on the defending board, so it TRAVELS:
  the launcher calls the covert-pool routine for itself and writes the figure
  into the record the packet carries (`launch_terrorist_operation` 0x02B201, at
  record `+0x0D`), which is what 0x04a7a9 weighs against the local pool. IB does
  the same through `RemoteTerror.Strength`; a packet without it lands every
  agent, as every packet did before this.

  IB also reports a batch the way the original does — one line carrying the count rather than one line
  per agent, which is what `ipreport.dat`'s `MULTI_`/`SINGLE_` template pair is
  for ("... %N times!"). Until this landed, IB ignored the operation and
  destroyed random units whichever of the nine was sent; a packet from a board
  that predates the dispatch still gets that blanket effect
  (`TerrorUnitLossDenom`), since it names no operation.
- **Send SpyGuy — BINARY-VERIFIED, and not a covert agent at all.** IB keeps the
  original's name here, deliberately (Andy, 2026-08-18), where it renamed Gooie
  Kablooie and S3-Sabre: players coming from the original look for this one by
  name. Do not "correct" it under the distinctive-names rule. He is a
  watcher posted on a PLANET, bought with gold rather than an agent, and he
  gathers no intelligence. Read out of `run_bombing_operations_menu`
  (BRE.OVR 0x029ea9), `ovr_044225_entry_0000` and `run_daily_maintenance`:

  - **Price**: the SENDER's own total regions × `SpyGuyGoldPerRegion` (300),
    **per day** — "A SpyGuy will cost N gold per day." — charged for the whole
    stay at once. The same shape as Terrorist Ops, which is total regions × 64.
  - **Stay**: the caller is asked "How many days would you like him to remain?",
    defaulting to 3, bounded by `SpyGuyMaxDays` (15) and by what the gold
    covers. The 15 is corroborated by `whatsnew.doc` — "Extended SpyGuy to
    function for up to 15 days (previously, max was 10)".
  - **The watched board holds the counter**, one byte per foreign board, and
    keeps the LONGER of a stay already running and a newly paid one. Daily
    maintenance decrements every counter ("Processing SpyGuys"); at zero he is
    gone. **Nobody is told, on either side** — there is no discovery and no
    execution. ("found and executed" belongs to `investigate_traitors`, and
    "was found spying on you" to the LOCAL Send Spy.)
  - **What he reports**: a group attack assembled against his planet (or a realm
    on it), with the hours until it leaves, and a Gooie under construction,
    funded, or dismantled. `breins.txt`: "he(she) will report information on all
    incoming group attacks and Gooie Kablooies allowing realms to prepare for
    the incoming onslaughts." On arrival he is caught up on both at once
    (`show_gooie_arrival_time`, `estimate_attack_arrival`).
  - **Reports are PLANET NEWS on the paying planet**, not mail: every report
    goes out through `append_news_record`, which writes a `NEWS_DATA` packet.

  IB implements all of it. **What IB had before was its own invention** — a
  per-baron Send Recon that spent an agent, warned the target, and filed figures
  in the Spy Database. That errand is gone; the Spy Database is now filled the
  way the original fills it, by covert operations reporting the state they found
  their target in (`resolve_received_covert_operation` → `write_spy_report` →
  "Information added to Global Spy Data Bank").
- **Special Operations** — the cross-planet bombing and missile set; see below.
- **SDI** — puts gold into the program; the strength it buys is capped at
  `SDIMax` (100%, the original's own clamp). See "The SDI program" below.

### Interplanetary Special Operations

The Special Operations submenu (InterPlanetary Ops → 8) runs the local Bomb
Enemy Targets set against a baron on another planet, plus Send SpyGuy. The
original resolves them by **writing a packet** rather than touching the target:
its handler (`run_bombing_operations_menu`, `BRE.OVR` 0x029ea9) calls
`write_special_operation_packet`, so the strike lands when the packet does. IB
does the same, on the machinery that already carries terror ops.

The menu is numbered 1-8 with no Help item (live capture):

| # | Operation | Price |
|---|---|---|
| 1 | Bomb Food Market | 10,000,000 |
| 2 | Bomb Trading Market | 25,000,000 |
| 3 | Bomb Trade Routes | 25,000,000 |
| 4 | Undermine Investments | 75,000,000 |
| 5 | Nuclear Assault | quoted |
| 6 | Chemical Bombing | quoted |
| 7 | R5-Slappenheimer | quoted |
| 8 | Send SpyGuy | quotes its own, after the target is named |

The four prices are the original's own, off the menu's price column
(`docs/dev/bre-screens.md`), and are fidelity constants in `balance.go`.

**Items 1-4 are aimed at the PLANET, items 5-7 at a named baron.** What the four
bombing ops wreck belongs to the whole planet — its food market, its trading
market, the trade deals in transit across it, the investments in its bank — so
they carry no realm, and one realm's New Realm Protection has nothing to refuse.
The three missiles ruin land and kill people, so they must name the realm that
owns them, and protection stops them as it stops any strike. The original's menu
handler splits the same way: keys `1`-`4` all branch to ONE shared handler
differing only by an index into a price table, while `5`, `6` and `7` each have
their own (`BRE.OVR` 0x029ea9, dispatch at 0x105a-0x124c).

**The effects are the local ops' effects**, called through the same helpers, so
a retune lands on both menus: food halved, a share of the market position and
its pending proceeds destroyed, the goods stripped out of the trade deals a
strike reaches, a quarter off each investment's principal and matching return,
and the three missiles' own damage. Every op needs the 500 Bombers the original requires of anything on this
menu, answers to the sysop's Bombing Ops / Missile Ops switches, counts against
the daily bombing allowance, and is stopped by New Realm Protection.

**Three IB decisions**, none of them established from the original:

- **The three missiles are priced off the LAUNCHER's land**
  (`IPMissileGoldPerRegion`), scaled by Terrorism Costs. The local versions
  price off the target's size, which is exactly what a board cannot know about a
  realm on another planet, and the original's menu shows no price for them. This
  one is a playtest knob.
- **A backfiring R5-Slappenheimer is reported home and applied there.** The roll
  happens where the target lives, but the realm it hurts is on the board that
  fired, so the damage lands when the answer arrives.
- **A lost packet returns nothing**, because a Special Operation commits no
  forces and no agents — only gold, already spent on launching it. The sender is
  told the strike was never heard of again rather than left waiting.

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

**Strength is BINARY-VERIFIED** (`BRE.EXE` resident `056d:1139`, the routine every
reader of the percentage calls):

```
strength% = trunc(sqrt(funding / (10 x (totalRegions + 1))))     clamped 0..100
```

The original stores the pot in **whole thousands** and computes
`thousands * 1000 / (regions+1) / 10` before the square root, which is the same
figure. It reproduces all sixteen captured screens exactly at that game's **8,321
regions** — the Terrorist Ops price on the surrounding menu is `regions × 64` (base per-region cost at opsToday=0),
which pins the count — so land is a divisor of the shield and not just of its
cost. Two consequences worth stating: taking land thins your own shield, and each
further percent costs more than the last (the curve is quadratic in the
percentage).

That also explains the screen's `Funding / Region: 0,000 Gold` line, previously
recorded here as an unexplained defect. It divides the stored thousands by the
region count and prints a literal `,000`, so any realm funding under 1,000 gold
per region reads as zero. The label is honest; the granularity is coarse. IB
prints the gold figure the label describes — a deliberate divergence.

One thing is still **not** established:

- **What underpaying the upkeep does.** The captured player always paid in full.
  IB scales the program back to what was funded; without some consequence the
  upkeep would be optional and an unmaintained shield would defend as well as a
  maintained one.

#### What the shield reaches — BINARY-VERIFIED

Every reader of the percentage calls the same resident routine, so its call list
is the whole of the mechanic. There are **seven call instructions in four
routines** — the earlier note here said "four call sites", which conflated the
two counts:

| where | site | what it does |
|---|---|---|
| `show_empire_status` | `BRE.OVR 0x19b48`, `0x19b7f` | tests `> 0` to decide whether to print the line, then formats the number |
| `run_interbbs_menu` (SDI Program screen) | `0x212f3`, `0x215df` | prints the strength on entry and again after gold is added |
| arriving interplanetary attack (`ovr_03f4a0`) | `+0xed6` | inside the A..Y loop that builds the planet-wide land-weighted average |
| arriving interplanetary attack (`ovr_03f4a0`) | `+0x10aa` | the named defender's own shield, applied to the incoming force |
| arriving S3-Sabre (`ovr_0450a9`) | `+0x481` | the interception roll |

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
- An R5-Slappenheimer is intercepted on a roll against the SDI percentage. That
  is now the only local weapon SDI touches: the nuclear, chemical and biological
  routines were all read and none of the three consults it, so the damage
  discount IB used to apply is gone (#103). Whether SDI should touch the
  R5-Slappenheimer either is still unread. (#113)
- IB reduces Clingy Annihilator damage by the defender's SDI. It should not: the
  original says jets are "the only way to destroy this thing", and the doomsday
  weapon is not among the three things SDI is documented to work on. (#111)
- **Jets** on an arriving strike fight at `1 - SDI x 0.3/100`, **bombers** at
  `1 - SDI x 0.2/100` (truncated). Linear in the percentage; the published "up to
  30% / 20%" are what they reach at SDI 100.
- **Missiles** are *intercepted*, not discounted: `Random(100) <= SDI x 0.5`
  stops the S3-Sabre outright. Hence "up to 50%".
- **A GROUP attack faces no shield at all.** The whole-planet path averages every
  living realm's SDI by land and then divides by 100 a second time
  (`ovr_03f4a0 +0xf18`); no average can reach 100, so the truncated result is
  always zero. Only a strike aimed at a named baron meets a shield. IB follows
  this rather than correcting it — it is what the original does, and the
  alternative reading would be a guess.
- **Nothing local is touched.** Not a neighbour's attack, not a nuclear, chemical
  or biological missile, and not the Clingy Annihilator (#111). IB applied
  `(100 - SDI)` in four such places until 2026-08-14; all four were IB's own
  invention and are gone.

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
computer barons included, which is the same reach the local "send to all" has.

**Arriving mail posts no news, on either planet.** BRE's inbound handler
(`process_interbbs_message_packet`) writes `DATA\MSG.BRF` and touches no news
file, and no news or report template carries a message category. IB filed a
planet news line for each arrival — naming the sender of a planet-wide message,
and reporting one addressed to the Coordinator or to a realm that had since
died — which put private mail in front of the whole planet. Removed 2026-08-18
(#146). A message to a Coordinator on a planet that has elected none, or to a
realm that has died, is now simply not delivered.

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

- **In BRE the value binds nothing.** Calling a planet an Enemy does not stop a
  treaty, a trade or an attack, in either direction.
- **IB gives it two jobs**, both IB's own: "Allied Planets" addresses a message
  to every planet marked Allied, and — since interplanetary trading (#47) — a
  baron may only bid at a planet's market while this board marks it **Allied**.
  `SendTradeBid` refuses otherwise, and `resolveRemoteTradeBid` re-checks on
  arrival, so an alliance dropped while a bid is in transit sends the gold back.
  Since the chart never rides a packet, each side gates on its OWN view: the
  buyer's board must call the seller's planet Allied to send, and the seller's
  board must call the buyer's Allied to fill. Proven live 2026-08-15 between two
  Mystic boards.
- It never rides a packet. Two boards can hold contradictory views of the same
  pair and neither will ever learn of the other's.
- Only the elected **BBS Coordinator** may change it, on a Diplomacy
  Modification screen: pick a planet, then answer `Change status to War, None,
  Peace, or Ally?`. *War* files Enemy and *Ally* files Allied — the prompt's
  words and the display words are two different tables in the original
  (`BRE.OVR` 0x23530 and `BRE.EXE` 0x158b5).

One thing reads it rather than displaying it: **Allied Planets** on the IP
Messages menu addresses the planets it calls Allied.

### Electing the BBS Coordinator — BINARY-VERIFIED

The Coordinator is elected by the planet's own barons, from the System Menu's
**Coordinator Vote** (`5`), and the vote may be changed at any time.

- **A baron still under new-realm protection cannot vote.** BRE draws the menu
  item only after the protection predicate clears the caller
  (`show_game_settings`, the guard at `BRE.OVR` unit `ovr_013753` +0x082f).
  IB matches this.
- **Every realm holding a slot is a candidate — protected realms included, and
  yourself.** The ballot is the shared realm picker
  (`choose_target_empire`, `BRE.OVR` 0x01aa99), which builds its key set from
  one test, the slot's player id > 0. Its two extra filters — the target's net
  worth must be positive, and the target must not be protected — ride an
  argument the vote passes as 0, and the exclude-yourself branch rides another
  it passes as 1. **IB excluded protected realms until 2026-08-18**, which in a
  young game left a voter with only their own realm on the ballot (#149).
- The office goes to the living realm with the most votes; IB breaks a tie by
  net worth, which the original does not state.

**Every turn opens by telling you where you stand.** The moment Play Game is
chosen, before the "since your last play" recap and only in an InterBBS game,
BRE prints one of two lines (`run_door_session`, `BRE.EXE 013a:0cf7`, behind the
InterBBS flag at `0x6a9a`; it compares the elected Coordinator's id against the
caller's own):

- to the Coordinator, that they hold the office;
- to everyone else, the realm their vote is currently for, and that the vote can
  be changed from the System menu.

The office and the realm name are the bright segment of the line, the rest the
body colour. A baron who has not voted is named **"No one"**: the formatter
(`format_no_recipient`, `BRE.OVR 0x0176f`) falls back to that literal when the
stored vote is not a realm letter A-Y, and takes the same path when the realm it
names has zero net worth — so a choice that has since died reads the same as
never having chosen. The line is one sentence in every case.

The notice does not test protection, so a realm still under protection is told
where to change a vote it cannot yet cast — the menu builder having just left
that item out (above). **IB copies both halves**, contradiction included: the
line is printed to everyone, and the item appears when protection ends.

IB implements all of this. One thing is not established from the original: the
color it prints **Enemy** in (the capture only ever showed the other three, so
IB infers bright red). The hotkey is settled — BRE's Coordinator Ops menu has
**four** items keyed `1`-`4` and this screen is `2` (see the table in
`docs/dev/bre-screens.md`); the "eight-item menu, IB uses `D`" this file used to
record came from the same misread string that section corrects. A new season clears the chart, since the
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

- **Declaring war costs a quarter of both popular support and military morale**
  and leaves the pair at **Enemy** (`World.DeclareWar`), mailing the other realm.
- **Attacking a partner outright costs nothing.** It still breaches the pact —
  the pair drops to Enemy (`World.breachTreaty`, called from `Attack`) — but the
  breaker pays no price at home. The crown charges for the declaration, not for
  the betrayal.

**Every Diplomacy action that addresses a realm takes a LIST.** The Diplomacy
menu calls the same toggling picker Send Message uses — the selection routine at
`BRE.OVR` 0x1b65e, reached from two sites inside its diplomacy menu (0x1c800
+0x08e7 and +0x0a79) — so one action proposes a pact to, or declares war on, as
many realms as are marked. Letters toggle, `Z` marks the whole `A`..`Y` range,
`?` lists, RETURN closes and an empty list cancels; the full behaviour is under
"Sending: the recipient picker addresses a LIST" above. One thing differs here,
and BRE passes it to the routine as a flag: `?` lists the `-*Relations*-` table
rather than the score table. IB's rules on top of that:

- **Marking exactly one realm is the negotiation proper** — it proposes the pact,
  or accepts that realm's matching offer.
- **Selecting the pact a realm ALREADY holds with you changes nothing
  (DELIBERATE DIVERGENCE).** IB reports that the agreement stands and returns to
  the Diplomacy menu. BRE files the proposal again regardless: its treaty items
  (1–7) read no relation before sending — verified in `BRE.OVR` at 0x1C800, where
  the send loop at +0x0995 goes straight to `"<pact> proposed to <realm>"` with no
  test of the relation row, while the Declaration Of War loop at +0x0ADA is the
  only one that reads it. IB used to offer to BREAK the standing pact here, which
  is neither BRE's behaviour nor safe — it put the one destructive diplomatic act
  behind the same key as the constructive one. BRE's break-with-penalty prompt
  ("Are you sure you wish break your agreement?", `BRE.OVR` 0x1A838) belongs to
  the shared target picker used by attacks, covert ops and trading, not to
  diplomacy.
- **Marking several sends one proposal each**, skipping any realm that already
  holds that pact, and asks the covering message once for the whole batch.
- **The covering message is optional, rides on the offer, and is mailed
  nowhere.** Attaching one is a separate step in the diplomacy menu
  (`prompt_diplomatic_message_attachment`, `BRE.OVR` 0x1C5BC), and the target
  reads it inside the offer itself: the proposal line is followed by a
  `Message attached:` block (`attach_message_to_diplomatic_proposal`, 0x1CCD9)
  before the Regions/Net Worth/Score line. Captured live in `cap/kd3-01.cap`,
  where the next proposal on the same screen carries no message and goes
  straight to its figures. A proposal puts nothing in the mailbox at all. IB
  mailed a generated "proposes a pact" line on every proposal, so the target was
  told twice what the offer prompt had just asked them.

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

Two rules go with it. A proposal **does not expire** — BINARY-VERIFIED, and the
answer to #95: it stands until the target accepts, rejects, or is eliminated.
`process_diplomatic_proposal` (`BRE.OVR` 0x1CF73) walks the same record list a
trade deal uses and selects proposals by type, then goes straight to the
identity checks; the timestamp compare that lapses a trade deal has no
counterpart there, and the proposal record carries no day count to compare. And a **new proposal to the same realm
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
  is attacked, its ally **sends 30% of its troopers and tanks** (*not*
  jets/turrets/bombers/carriers, and **not agents**) to reinforce the defender in
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
  rate (`bleedAllies`), which also **tells each partner what it lost and in whose
  defence** — BRE files that line in the same loop iteration as the deduction
  (`BRE.OVR 0x00ef90`, the aid loop in `resolve_regular_attack`), and without it
  a player's units disappear from a battle they were never told about.
  See the `bre-binary-verified-math` memory.

  **Who is told is wider than who helps, and the line is filed unconditionally.**
  BINARY-VERIFIED. The report loop's guard is `cmp word [es:di-0xebf],0x5 / jg`
  (`BRE.OVR 0x10545`) — every relation ABOVE 5 — while the detachment share is
  written only at `cmp ax,0x7 / jnz` (`0xf541`), equality with Full Defense
  Alliance. So a **Technology Agreement** partner receives a battle notice
  reading zero and zero, having sent nothing and lost nothing. The 236
  instructions between guard and deduction contain no branch of any kind, and the
  recap filer itself (`04ef:002f`) is 25 instructions with no conditional jump —
  there is no zero-total test and no dedup, so a partner that sent nothing is
  told so. IB matches both behaviours (`battleNotified`).

  **A Declaration Of War does NOT qualify, though relation 8 would pass the
  guard.** The value is never stored: `break_diplomatic_treaty` writes
  `xor ax,ax` to both relation rows (`0x1a8f0`, `0x1a912`), leaving 0 behind, so
  8 exists only as a display string. This settles the contradiction in
  `docs/dev/bre-save-format.md`, whose `+0x130` entry appears twice — once
  correctly saying 8 and 9 are menu items never stored, once wrongly listing 8 as
  a stored enum value.

  **The Alliance Strength screen has three figure columns and TWO treaties feed
  them.** BINARY-VERIFIED at `BRE.OVR 0x01177a` (`send_defensive_aid`), which
  sets two independent shares from the pair's relation: `0x32` (50) against
  relation 4, **Terrorist Prevention**, applied to **agents**; `0x1e` (30)
  against relation 7, **Full Defense Alliance**, applied to **troopers and
  tanks**. The attack resolver's aid loop reads troopers and tanks only, and the
  attacker's in-battle line names only those two. A pair holds one relation at a
  time, so a partner appears on exactly one of the two lists. IB credited an
  alliance partner 30% of its agents as well until 2026-08-16, lending help the
  original never lends; `AllyDefenders` now splits the rows by treaty.
  **The "only in Local Games" note is a rule, not a caveat, and IB honours it**:
  the relation lives in one planet's empire records and never rides a packet, so
  an arriving interplanetary strike meets the target's own `Defense()` and
  nothing more — `resolveRemoteAttack` does not consult `allyDefenseBoost`, and
  `TestFullDefenseAllianceDoesNotDefendAgainstInterplanetaryStrikes` fails if it
  ever starts to.
- **Tariff Trade Agreement** / **Free Trade Agreement** — per-turn trade income.
  BINARY-VERIFIED (BRE.OVR 0x03416b Tariff, 0x0341f0 Free Trade, both in
  `process_economic_production`): each partner pays
  `min(myPopulation, partnerPopulation) x rate`, with the rate assembled inline
  from both realms' New Realm Protection flags —
  `6 - 3*protectedSelf - 2*protectedPartner` for a Tariff,
  `11 - 5*protectedSelf - 5*protectedPartner` for Free Trade. Paying on the
  SMALLER population is what stops a pact with a giant from being free money, and
  the protection cuts stop a sheltered newcomer farming one.
  `TariffTradeGoldPerHead` / `FreeTradeGoldPerHead` and their cuts in
  `balance.go`, applied by `tradeIncome`. The rates are **gold per head of IB's
  own `People` count** — BRE counts population in millions and IB counts people,
  and IB applies BRE's population-side figures to its own unit unchanged, as it
  already does for the carrying-capacity weights. (IB previously paid
  `People/40` and `People/20` off the holder's OWN population, roughly a
  twelfth of the original and rounding to nothing at starting scale.)
- **Intelligence Alliance** / **Terrorist Prevention** — lend an ally's agents to
  your covert offense / defense, at **40%** and **50%** respectively — the shares
  differ (`covert.go`, `CovertAllyOffensePct` / `CovertAllyDefensePct`).
  BINARY-VERIFIED in `covert_resolution` (`BRE.OVR 0x04cab7`); see the Covert
  Operations section for the two constants.
- **Technology Agreement** — a tech-sharing pact (BRE: "gain some of the
  technological advances of its partner"). Each partner adds an unmultiplied
  research term bounded by whichever side holds fewer Technology regions, so it
  accelerates a realm that is already researching and does nothing for one
  holding none (`advanceTech`; full derivation under Technology above).
- **Protective Trade** — guards a trade deal from bombing (BRE: "preventing
  bombing of trade deals"), and the guard belongs to the DEAL, not to the
  attacker. BINARY-VERIFIED (BRE.OVR 0x051077, the routine that
  `resolve_received_bombing` at 0x04a09a calls for a received op type 3, Bomb
  Trade Routes): a deal is skipped when the deal's own sender and recipient hold
  Protective Trade with each other — whoever fired the strike. The deal record's
  fields +8 and +9 are the sender and recipient letters (confirmed against
  `create_trade_offer`, BRE.OVR 0x024961+0x1e97), and relations are mutual, so
  reading one row settles both. The attacker's own relations are never read.

  Three further facts about that routine. The check runs at RESOLUTION on the
  receiving board, after the attacker has paid and the packet has flown, so
  nothing is refused, nothing is refunded, and no per-deal message exists. A
  separate `random(3)` per deal lets one deal in three escape whatever the
  relation. And another `random(3)` at the top of `resolve_received_bombing`
  voids the whole strike two times in three. A deal that is not spared loses
  `trunc(qty x (random(5)+5) / 100)` — 91-95% — of each of its nine goods
  quantities.

  **IB follows all of it** (`bombRoutesLands`, `bombRoutesEffect`,
  `bombDealBasket`; the three rolls are `BombRoutesLandOdds`,
  `BombRoutesDealHitOdds` and `BombRoutesKeptPctMin`/`Spread` in `balance.go`).
  A strike wrecks the goods in pending `TradeDeal`s rather than severing any
  standing agreement, and the guard reads the deal's own two parties, so holding
  Protective Trade with the realm you are bombing buys you nothing and a deal
  between a guarded pair survives whoever fires. Since only one relation can
  stand between a pair (#88), the pact that guards a deal is also the pact that
  makes it cheap to send. IB used to REFUSE the op outright — no fee, no agent
  spent — when the ATTACKER held Protective Trade with the target; that was IB's
  own invention and has been removed.

  **Bomb Trading Market reads no relation at all.** Op type 2 loops every living
  realm and scales each one's market quantities; nothing in that branch compares
  a relation, so Protective Trade shields no market. IB's market guard has been
  removed to match.

  **Bomb Trade Routes is interplanetary-only in BRE, and now in IB.** Its menu
  (`run_bombing_operations_menu`, BRE.OVR 0x029ea9) is reached only from
  `run_interbbs_menu`. BRE's LOCAL Covert Operations item 7 is a different op
  that rolls `random(6)+1` and damages one of six quantities, comparing no
  relation — which is what IB's local item 7 now is. Only the interplanetary
  menu offers the trade-route strike.

  **What deals a strike reaches is IB's own call.** BRE's op is planet-wide, so
  it never had to say which deals a strike aimed at ONE realm covers. IB takes
  every deal that realm is a party to, on either side of it — a realm's trade
  routes run both ways, and counting only the deals sent TO it would let a realm
  dodge the op by never accepting one. The interplanetary planet-wide variant
  keeps BRE's scope: every deal on the planet, with one landing roll for the
  whole strike rather than one per realm.

  Protective Trade also makes trade deals **cheaper to send**: the per-day transit rate is
  divided by `ProtectiveTradeCostDivisor` (3) before the span is chosen, so a
  guarded deal costs 33,333 a day instead of 100,000
  (`TradeDealGoldPerDayBetween`). BINARY-VERIFIED — BRE.OVR 0x0268bc compares the
  recipient's relation against 2 (Protective Trade) and divides the per-day cost
  by three; the manual's "and maintain" has no separate charge behind it, because
  the one up-front `days x rate` payment is the whole cost.

**Declaration Of War** is the menu's formal way to end an agreement, and it is
expensive. BINARY-VERIFIED (BRE.OVR 0x01a838, `break_diplomatic_treaty`): once
confirmed, popular support (record `+0x92`) and military morale (`+0x8e`) are each
divided by four and multiplied by three — a quarter off both — and only then are
the relation rows on *both* empires cleared. The screen's own message speaks of
revolts and of morale dropping severely. IB matches (`DeclareWarKeepNumerator` /
`DeclareWarKeepDenominator`, `DeclareWar`), charging only when a real pact stood,
as BRE does: the option is offered at all only when the relation is a treaty.

Two manual statements about it are **wrong about the shipped game**, and are
recorded here so they are not "fixed" back in:

- "without causing internal troubles in your realm" — the code charges the two
  quarters above. A disassembly outranks the docs, so IB follows the code.
- "The treaty is not officially broken until the other realm is notified" — BRE
  clears both rows in the same routine that prompts, with nothing waiting on the
  message. There is no delayed-break window, and IB models none.

Attacking a partner outright (`breachTreaty`) costs the breaker nothing, which
is the original's behaviour: no attack path reads the relation, so there is
nowhere for a penalty to be charged. IB charged 10 popular support here until
the declaration routine was read, on the reasoning that a pact you can walk out
of for free is not a pact. That priced the two exits the wrong way round. The
original's asymmetry is deliberate — a declaration is a public act with a public
cost, and a betrayal is punished by the other players rather than by the crown.

The two newly-wired treaties' magnitudes are IB tunables — BRE's manual gives the
intent, not the numbers.

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
- **Protection-gated:** a realm under new-realm protection cannot trade at all.
  BRE's own reset help defines the setting as the turns for which a new empire
  "is unable to attack, trade, and be attacked", and its changelog adds that
  trade deals cannot be received under protection. IB refuses every path that
  moves goods between realms: listing on or buying from the Trading Market
  (`SetMarketListing`, `BuyFromMarket`), trade deals in both directions
  (`SendTradeDeal` checks the sender and the target), and interplanetary bids
  (`SendTradeBid`). Implemented 2026-08-15; this entry had stated the rule while
  only attacks were actually gated.
- **Escrowed goods are safe from attacks, but NOT from pirates.** The community
  guide's "park military to evade pirates" is wrong about pirates, and IB now
  follows the original: five of the sixteen faces of the raid's category ladder
  read the Trading Market listing rather than the inventory (one per unit type —
  gold is not traded). MEASURED by driving BRE: listing 73 of 100 troopers wrote
  73 to record `+0x211`, the field bucket 11 reads, and left 27 at `+0x76`.
  Escrowing therefore does not hide military; it moves about a third of the raid
  risk onto the listing. Listing also does **not** dodge
  your own economy: escrowed **military units still cost maintenance** and escrowed
  **food still spoils**. `Bomb Trading Market` (covert) destroys a share
  (`BombMarketLossPct`) of a target's listed goods and pending proceeds.

Negotiated empire-to-empire trade deals carrying goods with demands (BRE's other
trading half) are built: `SendTradeDeal` takes a full basket each way, escrows
what is offered, consumes a transport carrier and charges the per-day transit
fee (#17). Interplanetary trading is built too, as its own type (`IPTradeBid`,
a buy order that travels to another planet and is filled there); carrier-moved
goods remain future work.

**A deal cannot reach anyone earlier in their day than it left yours —
BINARY-VERIFIED.** `create_trade_offer` stamps the offer record with the
SENDER's turns-remaining-today at the moment of sending (`BRE.OVR` 0x260CD,
unit offset 0x238B, into record byte `+0x60`) and prints the same figure back
to them as the turn it will arrive on (0x2451, rendered as
`TurnsPerDay - turnsRemaining + 1`). `process_trade_offer` then compares that
stamp against the RECIPIENT's own turns-remaining (0x24D6B, unit offset 0x05B1)
and, when the recipient still has more turns left than the sender did, leaves
the record pending — it is neither shown nor deleted, only re-stamped with its
integrity word. Because both halves read the same counter, a deal sent on the
sender's third turn of the day is waiting from the recipient's third turn
onward, that day and every later day it survives to. A byte of `0xFF` is a
sentinel for no gate at all; which path writes it is NOT established.

IB mirrors this with `TradeDeal.ArrivesOnTurn`, holding the sender's turn of
the day counted from 1 — the figure BRE prints — so that turns-per-day cancels
out of the comparison and an unstamped deal from an older save keeps arriving
at once. The sender is told the arrival turn in IB's own wording.

Andy's suggestion that the stagger exists to keep a recipient under the
2-billion gold cap is NOT what the code does: the branch reads only
turns-remaining and turns-per-day, no gold figure appears in it, and a
recipient reaching their sixth turn meets everything sent on turns one to six
at once — `cap/eots-ibbs-01.cap` shows two deals accepted back to back. Lower
cap exposure is a side effect. (The claim circulates because that capture also
carries a *player's* mail message asserting it; that is folklore, not BRE
output.)

**Neither the offer nor an answer is mail.** BRE carries
" accepted your trade deal." and " rejected your trade deal." side by side in
`process_trade_offer` (`BRE.OVR` 0x24D6B), each written to the other realm's
record, so the answer is waiting on the proposer's recap whenever they next
play — the same shape as a treaty answer (`notifyProposer`). IB filed only the
acceptance, and as mail: a decline returned the escrow silently, leaving the
proposer to notice their own stock coming back. The offer itself puts nothing
in the mailbox either; the recipient meets it at turn start, as with a treaty
proposal.

**A deal lasts exactly the span it was sent for, and only an acceptance ever
moves the goods — BINARY-VERIFIED.** The days prompt is not a pricing dial: BRE
adds the day count to the clock and stores the result on the offer record
(`create_trade_offer` 0x2256), and the turn-start sweep compares that stamp
against the clock, deleting the record and telling the sender its trade fleet
got no response (`process_trade_offer` 0x24E5). The prompt refuses fewer than
two days (0x21B2), offers ten as its default (0x1F7D), and has no ceiling —
what the sender can pay for is the only limit, where IB used to cap the span at
five days.

Nothing gives the escrow back. The rejection branch files the notice and zeroes
the 151-byte record (0xDC4) with no goods moving, the lapse branch does the same
(0x511), and the branch for a target that can no longer be found does the same
again (0x562) — every routine that moves goods sits inside the accept branch.
So sending a deal is a bet: the goods leave when it is sent, and they come back
only if the offer is taken. That settles #174 — the escrow is forfeit when the
recipient dies, and the sender is told, which is what IB now does on all three
paths. IB previously returned the goods on a decline.

**A pending deal is put again on every entry, and out of turns is asked
nothing.** `run_player_turn` calls `process_trade_offer` behind a
turns-remaining test (`BRE.EXE` 0x3842) and gates the CALL on nothing further —
the per-deal arrival test above lives inside the routine, not at this call site
— so ignoring a deal only defers it to the next entry that has a turn to play — the
second question in #175. The recap entries and the mailbox are on the other side
of that test (0x385F) and are shown either way, before "Sorry, you have used all
of your turns today." (0x3F8D); IB returned at that message and showed neither.
Both of #175's questions are now answered above.

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
pirate-raid outcomes (PIRATEWIN / PIRATELOSS), tax riots (RIOTS), and civil-war
collapse (CIVILWAR — `postCivilWarNews`).

An interplanetary strike is reported on **both** planets (#108): the defending
board posts what the raid took and cost, or that it was repelled, and the
attacker's board posts its return. An INDIVIDUAL strike is named on both sides —
the packet carries `RemoteAttack.FromEmpire`, so the defending planet reads
"Ironhold of Alpha" and not merely "Alpha" — while a GROUP attack is anonymous
on both, since it is the planet's doing and naming one of several contributors
would be a lie. A packet from a board too old to carry the field falls back to
the planet name, which is all those boards could ever say.

Not yet generated: BRE's finer-grained interplanetary news categories
(individual vs. group vs. whole-BBS, attack vs. return) as separate `ipnews.dat`
subtypes.

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

## Game Bulletins

Bulletins are the sysop's noticeboard, reached from **(8) Game Bulletins** on
the opening menu. BRE keeps them all in one `game/bulletin.lst`, each section
opened by `^NAME` and closed by `^END`. **IB uses one file per bulletin
instead** — a deliberate divergence, because a league's bulletins have to travel
between boards as files anyway, and ANSI artwork does not survive being pasted
into a list file.

They live under the data directory:

    data/bull/league   the League Coordinator's, shown as "Galactic"
    data/bull/local    this board's own

A bulletin is a `.txt` or an `.ans` file, at most 64 KB, and its **title is the
first line of the file** with any ANSI escapes removed; a first line carrying no
letters or digits (a row of block art, an empty line) leaves the file name to
name it. `.ans` files are decoded from CP437, so artwork authored in PabloDraw
or Moebius renders on a CP437 door and on a UTF-8 terminal alike. Files are
listed in file-name order, which is what a `10-`/`20-` prefix is for.

The menu numbers the galactic bulletins from 1 and carries straight on into the
local ones, so a player picks by number without minding which list a bulletin
came from. A group with nothing in it draws no heading.

**A bulletin added or edited is planet news**, naming the bulletin, so nobody
has to open the menu to find out whether anything moved. A withdrawn one is not
news. The first pass on a board upgrading from a version without bulletins is
silent — the files already sitting there are the baseline, not a day's worth of
news.

### League distribution

`bull/league` belongs to the League Coordinator. The Coordinator's board reads
it off disk on every planetary run and broadcasts the **complete set**, signed
with the Coordinator key like the ruleset and the roster; every other board
writes what arrives into its own `bull/league`, creating the directory if it
has none, and removes anything the league no longer carries. A board that joins
late or misses a packet is brought level by the next broadcast rather than
needing a replay, and an empty set is how the last bulletin is withdrawn from
the league.

A member board's own edits to `bull/league` are neither news nor kept: the next
broadcast puts the directory back. A file deleted there by hand is restored the
same way. A board in no league has no Coordinator filling the directory, so
whatever its sysop puts there is treated as its own and does file news.

An incoming bulletin's file name is checked before it is used to build a path —
no separators, no leading dot, `.txt` or `.ans` only — and an oversized file is
refused at both ends. An unsigned set, or one from a board that is not node #1,
is refused like a forged ruleset.

## How Immortal Barons differs right now

Now matching this reference (as of v0.0.4):

- Offense/defense split in combat, with the correct unit values
  (trooper 1/1, jet 2/0, turret 0/2, tank 4/4)
- Turrets (defense-only) and carriers (jets can only attack if carried)
- Bomber airfield strikes: a regular attack sends the attacker's bombers to
  destroy the defender's grounded jets first, resisted by turrets (anti-air)
- Interactive maintenance stage at turn start: pay armed-forces upkeep and
  region maintenance ("how much will you give?"), with underpayment causing
  desertion / revolts, plus optional popular-support and military-morale boosts.
  Auto-Pay Maintenance pays it silently when affordable — and, as in the
  original, only while support and morale both read exactly 100.
- Reference net-worth values and per-unit maintenance
- Bank interest ~1% per turn, with the interest cap and the money cap
- Nuclear / chemical / biological strikes and pirate raids
- Clingy Annihilator, with the original's fund-build-launch-intercept lifecycle
- SDI defense: a funded shield whose strength is its gold spread over the land it
  covers, blunting incoming interplanetary jets and bombers and intercepting
  incoming missiles
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
- BRE's finer interplanetary news subtypes

A few Diplomacy and Covert menu items are recorded but inert, pending fuller
subsystems. Each is flagged where it is described above.

### Screen output that deliberately diverges

`docs/dev/bre-screens.md` is the catalog, each note beside the screen it belongs
to. These two are recent enough to be worth naming here as well, since both
replace something the original does line for line and both will look like bugs
to anyone checking IB against a capture:

- **Manufacturing is a list, not six sentences.** BRE ends each line with "were
  manufactured by Industrial Zones."; IB says that once as a heading and lists
  the units under it, one per line, with no line for a type that built none.
- **Daily maintenance names IB's own tasks.** The shape is BRE's — a marked
  header, an indented line per task as it is carried out, a bright closing
  line — but the tasks are IB's, in IB's words, and only the ones with something
  to do are printed. Half of BRE's list belongs to file formats IB does not
  have.

Neither is an oversight to correct back.

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

IB reads the same HOST lines out of `ibnodes.dat`. What this buys is the
reason BRE did it: a leaf board configures one link, to its uplink, whatever the
size of the league, and a board joining costs one sysop an edit instead of all of
them. `-league-routes` prints the resulting table, as BRE's `BRE TEST` does.

Divergences, all forced by the transport rather than chosen:

- **IB does not read `ROUTE.CFG`.** It did until v0.0.7. Three of that file's
  four keywords (`CRASH`, `HOLD`, `NORMAL`) set a FidoNet mailer's send
  priority, which with a file drop is configured in the transport rather than
  the game; the fourth left a board able to contradict the Coordinator's roster
  silently, in a file no other board can see. BRE's own sample says a league
  whose Coordinator keeps routing in the roster needs no such file, so the
  roster is IB's single routing table.
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

**Origin is checked separately, and is (#118).** Whether a packet really came
from the board it names is not the same as whether that board's figures are
honest, and the second being unfixable does not make the first so. Before this,
nothing checked the sender: a packet written into an inbound directory was
applied on the strength of a name it carried, which was enough to hand a realm
an army (a result packet's survivors) or take its regions (an attack packet's
offense figure). Sysop answers in discussion #117 are what surfaced it.

Every board now holds its own ed25519 key in `board.key` (`-gen-board-key`) and
signs every outbound packet over its whole contents, with two fields excluded:
the signature itself, and `Hops`, which each forwarding hub legitimately
increments. The Coordinator's own signature IS covered, so a genuine order
cannot be lifted out of one packet and grafted into another.

The matching public key travels in the league roster, on an optional seventh
line of each node block. That is what makes the distribution work without a new
mechanism: the roster is already signed by the Coordinator key each sysop
records once by hand, so a board never has to trust a key handed to it by the
board the key belongs to. One out-of-band step anchors every key after it.

The two key pairs answer different questions and are not interchangeable:

| | `coord.key` / `coord.pub` | `board.key` + roster `PublicKey` |
| --- | --- | --- |
| Answers | May this board dictate league rules? | Did this packet come from the board it names? |
| Held by | Coordinator (private), everyone (public) | Every board (private), roster (public) |
| Distributed | Once, by hand | Inside the signed roster |
| Covers | Ruleset, roster, season reset | The whole packet |

**A roster entry with no key is applied unsigned**, which is every league until
its Coordinator publishes keys. Refusing would break a working league on upgrade
rather than securing it, so the check reports "cannot check" and "failed the
check" as different things. That transition state is the remaining gap and it
closes as rosters gain keys; a board whose roster key IS set can no longer be
impersonated.

**Still not addressed**: withholding. A signature proves where a packet came
from and says nothing about one that never arrives, and a missing packet looks
the same as a board that did not play.

## Inter-BBS packet staleness (#104)

Nothing in a packet said which game it belonged to, so a packet already sitting
in the inbound directory when a board resets — most often its own leftovers,
written by a peer before the reset and never read — was applied to the fresh
world exactly as if it were current: a strike from a realm that no longer
exists, scores for empires already wiped, mail addressed to barons who are
gone.

`World.Epoch` is this board's own generation counter, starting at 1 and
advancing on every full reset — a stand-alone `-reset` as well as a
league-wide one. Every outbound packet is stamped with the sender's Epoch at
write time. `ApplyPacket` discards a packet whole when its Epoch is older than
this board's current one, before anything in it is applied. A packet with no
Epoch at all — one written before this existed — is trusted rather than
rejected.

## Inter-BBS board identity (#105)

BRE keys a board on the roster's node number (`BRNODES.DAT`), not its name;
IB's packets originally carried only `FromBoard`/`ToBoard` name strings. A name
can collide between two sysops, and a board that renames itself becomes a
stranger to a league still holding its old name.

Packets now also carry `FromNode`/`ToNode`, the roster numbers of the two
ends, stamped from the sender's own roster. Wherever identity actually matters
— `VerifyBoardOrigin`'s key lookup, the Coordinator check (`fromCoordinator`,
node #1), and whether an inbound packet is addressed here
(`World.AddressedToMe`) — the node number is checked first and the board name
is the fallback, for a packet or a roster that predates node identity. The
name stays the display label everywhere else (`RemoteBoard`, the planet list).
