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
Offsets are **relative to the record base**, which sits **0x20 before** the
`ShortString` name the file-offset map above uses as its origin.

The two maps agree where they overlap: the region block is `name + 118` above
and `base + 0x96` (150) here, and 150 − 32 = 118.

```
empire record (les di,[0x28d8])
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
  +0x331 int32  land still available to BUY — the Daily Land Creation allowance.
                PER-EMPIRE, not a planet-wide pool: 0x12D30 bounds a region
                purchase against it and 0x12EF9 subtracts the number bought
                (pointer + sub32 helper 0c03:0fe3). Exhausting it produces
                "No land is available at this time."

config record (les di,[0x28b4])
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
0851:0288   int32 -> "1,234,567"        0fd0:178a   real compare
056d:1a07   technology factor: 1 + (cap-1)*(1 - exp(-level[sel]/(regions+1)))
0fd0:193e   Ln          0fd0:19e7   Exp          0fd0:1774   square
056d:0ec6   sum of the nine region counts (total regions)
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
