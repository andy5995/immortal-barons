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
