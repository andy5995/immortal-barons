# BRE save-file format notes (`data/game.dat`)

Partial map of BRE v0.988's binary save, from differential diffing (make one
known in-game change, diff the file). Reference for understanding BRE, not for
IB's own save format (IB uses JSON). **Key finding: BRE validates record
integrity, so naive byte-edits are rejected** — see the warning below.

## Files

- **`data/game.dat`** (~29 KB) — the **empire records**. Fixed-size records
  (~1069 bytes apart: two empires' name fields sat at file offsets 2521 and
  3590). Each record's name is a Turbo Pascal `ShortString` (1 length byte +
  chars), e.g. `04 "Rome"`.
- **`data/planet.bre`** (~360 KB) — planet/region map data; holds no empire
  name strings.
- **`data/ids.dat`** — empty in this game.

## Region counts (mapped)

The eight region-type counts are **`int32` little-endian**, stored **contiguously
in Buy-Regions display order**:

`Coastal, River, Agricultural, Desert, Industrial, Urban, Mountain, Technology`

For the "Rome" record (name at file offset 3590) the block began at offset
**3708** (0xE7C) — i.e. name + 118 bytes. Confirmed: buying exactly 10 Coastal
changed the first int32 there from `16 00 00 00` (22) to `20 00 00 00` (32), and
nothing else in the block moved. Two `int32 = 100` fields sat just before the
block (0xE74/0xE78) — likely Popular Support / Morale, unconfirmed.

```
offset 0xE7C  Coastal   int32 LE
offset 0xE80  River      "
offset 0xE84  Agricultural
offset 0xE88  Desert
offset 0xE8C  Industrial
offset 0xE90  Urban
offset 0xE94  Mountain
offset 0xE98  Technology
```

## Runtime record layout (from the disassembly)

The offsets above came from differential diffing the file. These come from
reading `BRE.OVR`, where the empire record is reached through a far pointer
loaded with `les di,[0x28d8]` and the config record with `les di,[0x28b4]`.
Offsets are **relative to the record base**, and the base **IS the offset of the
`ShortString` name** — the length byte itself. (An earlier note here put the base
0x20 *before* the name, reconciling the two maps by arithmetic rather than by
measurement. A live check disproves it: with base = name, +0x281 read 2 after two
turns played, +0x8e read 100 against 100% morale on screen, and the region block
at +0x96 read `[3,0,2,5,0,0,5,0,0]` against a status screen showing 3 Coastal,
2 Agricultural, 5 Desert, 5 Mountains. With base = name-0x20 none of those line
up. The file-offset map above and this one therefore do NOT agree where they
overlap, and this one is the measured one.)

The two maps agree where they overlap: the region block is `name + 118` above
and `base + 0x96` (150) here, and 150 − 32 = 118.

```
empire record (les di,[0x28d8])
  +0x62  int32  population, in MILLIONS. Named by the string the chemical and
                biological strikes print beside it — "<N> million civilians were
                killed!" — after each subtracts a percentage of this field. Both
                also multiply it into their warhead's price, so every per-head
                price in this binary is per million.
  +0x6a  int32  bank balance
                (+0x66 is NOT gold on hand: the turn routine zeroes it every
                 turn and it reads 0 in the save. The maintenance routine loads
                 gold from it into a local, so it is a per-turn working field.)
  +0x72  int32  gold earned this turn  (see below)
  +0x8e  int32  military morale
  +0x92  int32  popular support
  +0x96  int32  region block starts (Coastal first, Buy-Regions order); the
                nine counts +0x96..+0xb6 are what "total regions" sums
  +0xa6  int32  Industrial count
  +0xae  int32  Mountain count
  +0xbe  int32  technology levels: FIFTEEN counters, +0xbe .. +0xf9, one per
                research slot. Only slots 0-5 do anything; 6-14 are pure
                dilution. Never decremented anywhere in the binary.
  +0x129 .. +0x12e   six bytes: Set-Industries allocation percentages, in
                     menu order (Troopers, Jets, Turrets, Bombers, Tanks,
                     Carriers)
  +0x281 int32  lifetime turns played. Drives the HeadQuarters price ratchet and,
                against config +0x38, whether the realm is still protected.
  +0x285 byte   turns remaining today. Reset to config +0x36 at rollover; zero is
                what produces "Sorry, you have used all of your turns today."
  +0x2b8 byte   stage counter for the turn in progress. The turn is a state
                machine: the loop head at BRE.EXE 0x6236 dispatches on this byte,
                each stage advances it, and at 20 (0x685f) the turn commits —
                +0x281 increments, +0x285 decrements, and this resets to 0
                (0x688e). It lives in the record, not on the stack, so an
                abandoned turn resumes where it stopped; the same job as IB's
                TurnProgress.
  +0x211 .. +0x231   the Trading Market "For Sale" escrow, one int32 per slot in
                     the market screen's own order: Trooper, Jet, Turret,
                     Bomber, Food, (unused key 6), Agent, Tank, Carrier.
                     MEASURED: listing 73 of 100 troopers put 73 at +0x211 and
                     left 27 at +0x76. Net worth sums each of these with its
                     home counterpart, and the pirate raid's category ladder
                     reads them on five of its sixteen faces — so escrowed
                     military is NOT hidden from pirates.
  +0x26f int32  agents held (the market's escrowed agents are at +0x229)
  +0x331 int32  land still available to BUY — the Daily Land Creation allowance.
                PER-EMPIRE, not a planet-wide pool: 0x12D30 bounds a region
                purchase against it and 0x12EF9 subtracts the number bought
                (pointer + sub32 helper 0c03:0fe3). Exhausting it produces
                "No land is available at this time."

config record (les di,[0x28b4])
  +0x1c  int32  Daily food pool, planet-wide (seeded 0xF4240 = 1,000,000 at
                game init; buying depletes it, selling replenishes it)
  +0x24  int32  Pool the Queen Royale tax refund is paid out of, planet-wide.
                Seeded 0x000186A0 = 100,000 at game init (0x44CE6, beside the
                food pool's 1,000,000). The refund routine (BRE.OVR 98944, just
                past its message string) reads it, pays min(V x rate, 1,000,000)
                to the empire via 0c03:0f10 into empire +0x66, then writes
                V x (1 - rate) back. rate = 0.02, or 0.07 once V > 100,000,000
                (the threshold is a literal cmp against 0x05F5E100; all three
                reals decode exactly: 0.02, +0.05, cap 1,000,000). The cap is
                applied by overwriting rate with 1,000,000/V, and is itself
                gated by the per-empire predicate at 056d:19b5 — when that is
                false the payout is uncapped. The uncapped branch is common in
                play: a census of all 47 refunds in cap/ found payouts of
                12,581,639 to 14,000,000 on two boards, alongside eighteen at the
                cap (1,000,000, or 999,999 where the rate substitution loses a
                unit).

                THE PREDICATE IS "is this empire still under New Realm
                Protection?" It reads config +0x38 (Turns Of Protection) and the
                empire's lifetime turn counter +0x281 and returns
                turnsPlayed < turnsOfProtection. Identified from its other call
                sites: at BRE.OVR 0x1771a the same call gates the covert message
                "Our empire is in protection, my lord." So the 1,000,000 ceiling
                is a newcomer guard — a fresh realm on a mature planet cannot
                open with a 14-million windfall.

                THE CALLER is BRE.EXE 0x61dd, the only one, inside the "Since
                your last play" recap. It runs when the turn-stage counter
                (+0x2b8) is zero AND turnsRemaining (+0x285) >= config +0x36
                (turns per day) — no turn part-way through and none taken today,
                i.e. the player's first play of a game day. The routine holds no
                Random(), so it always pays when reached. The call immediately
                after it is the lottery (its strings are at BRE.OVR 0x18531), so
                the two share one first-play-of-the-day event block.

                The pool is FED by the crown tax: the tax routine at 0x2FAF1 —
                the same one that charges the Queen Royale tax and computes the
                underpayment support penalty — adds the empire's tax figure
                straight into V. It banks the amount actually PAID, not the
                amount due: the value added ([bp-0x10], from 056d:01bf at
                0x2FA7C) is the same one the shortfall penalty divides by the
                amount due at 0x2FA94, which only makes the penalty positive if
                it is the paid figure.

                Play data confirms the seed and the writeback independently: a
                fresh game's first refund is 2,000 (2% of 100,000) and the next
                is 1,960 (2% of 98,000 = V x 0.98). Two refunds captured on a
                third board (an A-Net game server) fit the same 2% branch on a
                mature pool: 375,090 and 384,673 put V in the 18.7-19.3M range,
                well under both the 100M rate threshold and the cap, so no
                capture yet exercises the 0.07 branch or the cap gate. See
                issue #93.
  +0x36  int16  Turns per day
  +0x38  int16  Turns of Protection (compared against empire +0x281)
  +0x42  int16  Planetary Tax Rate, tenths of a percent (default 50 = 5.0%,
                editor maximum 200 = 20.0%)
```

**`+0x72` is a per-turn accumulator, not a running total.** It is zeroed at the
start of the income phase and added to at exactly six sites, one per income
line — population tax, ore, tourism, solar, industrial, hydro. Nothing else in
the binary writes it, so bank interest, food sales and plunder do not enter it.
This is the base for the crown tax (issue #52).

Runtime helpers worth recognising when reading this code:

```
0c03:0ed0   Random(n)                   0fd0:178e   int -> real
0c03:0f10   add32 through a pointer     0fd0:177a   real multiply
0fd0:0ecc   32-bit multiply             0fd0:1780   real divide
0fd0:0f09   32-bit divide               0fd0:1768   real add
0fd0:176e   real subtract
0851:0288   int32 -> "1,234,567"        0fd0:178a   real compare
056d:1a07   technology factor: 1 + (cap-1)*(1 - exp(-level[sel]/(regions+1)))
0fd0:193e   Ln          0fd0:19e7   Exp          0fd0:1774   square
056d:0ec6   sum of the nine region counts (total regions)
056d:18f0   ruin N regions: calls the proportional remover (056d:11f1) across the
            nine counts, then adds the same N to Waste at +0xb6. Both the
            nuclear and the chemical strike reach it; the biological one does
            not, which is why a plague leaves the land alone.
0fd0:1792   real -> int, TRUNCATES
0fd0:179a   real -> int, ROUNDS      <- the two are easy to confuse; which one a
                                        routine uses changes results by one unit
```

## ⚠ Integrity check — direct edits are rejected

Patching Coastal from 32 to 200 directly in `game.dat` (with BRE closed) and
relaunching made BRE **fail to find the empire** and prompt the caller to name a
new realm — the edited record was discarded, no explicit "tampered" message.
Restoring the pre-edit backup brought the empire back intact. So BRE stores a
per-record (or per-file) **integrity value** that a raw field edit invalidates.

**Consequence for test setup:** you cannot just poke region counts to stage a
scenario — the empire gets reset. To edit safely you would first have to locate
the checksum field and its algorithm (another differential-diff pass: change one
field in-game and find which *other* bytes move — those are the checksum), then
recompute it after each edit. Until that is done, **prefer in-game changes**
(e.g. buying regions over a few turns) for staging tests.

Always back up `data/game.dat` (and `planet.bre`) before any experiment.
