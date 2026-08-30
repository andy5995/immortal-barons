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
  +0x00  String[30]  the CALLER'S BBS ACCOUNT NAME — the person, not the realm.
                A live save has "SELBY" here against a realm named "duh", and a
                second record "WRAPTEST" against "Wraptest", the BBS's own
                upper-cased login beside the name its owner typed. Nothing on
                any screen prints it; the dupe file DATA\DUPE.BR carries it
                across boards for duplicate-user checking, and the League
                Coordinator's PLAYERLIST command line dumps that file to
                PLAYERS.LST. Its length byte doubles as the slot-occupied test
                the pickers use (`cmp byte [es:di-0xf6d],0`).
  +0x1f  String[30]  the REALM name, and the one every player-visible screen
                uses: See Scores, the coordinator vote's roster, the
                interplanetary Player Information screen, and the recon packet's
                own `PlanetInfo.Names`, which `build_recon_record` fills from
                here. So a realm's owner never crosses to another planet.
  +0x3e  String[30]  a third name slot, empty in every record examined.
  +0x62  int32  population, in MILLIONS. Two independent reads agree: the food
                routine at BRE.OVR 0x37418 multiplies it by 1.5 for "Your People
                Need N", and the chemical and biological strikes print "<N>
                million civilians were killed!" beside it after taking a
                percentage of it. Both missiles also multiply it into their
                price, so every per-head price in this binary is per million.
  +0x5d  int32  slot-in-use serial. -1 in every unoccupied slot; a positive,
                game-lifetime-increasing number in an occupied one (921 / 932 /
                933 across three saves, the second realm exactly one above the
                first). Read or tested `> 0` at 83 sites — target pickers, net
                worth, the roster, the coordinator vote, the Technology
                Agreement partner loop — always through the indexed
                other-empire pointer, never written anywhere. It has to be
                tested everywhere because BRE initialises every unused slot
                with the STARTING TEMPLATE, so an empty record still carries a
                plausible region mix.
  +0x6a  int32  bank balance
                (+0x66 is NOT gold on hand: the turn routine zeroes it every
                 turn and it reads 0 in the save. The maintenance routine loads
                 gold from it into a local, so it is a per-turn working field.)
  +0x6e  int32  food in store. The production routine adds each turn's yield
                here; the allocation routine draws feeding out of it.
  +0x72  int32  gold earned this turn  (see below)
  +0x76 .. +0x8a  six int32 unit counts, in the order the food routine reads
                them: Trooper +0x76, Bomber +0x7a, Jet +0x7e, Turret +0x82,
                Tank +0x86, Carrier +0x8a. Trooper is measured (see the market
                escrow below); the rest are paired with their escrow slots by
                the armed-forces food routine, which reads each type's two
                fields back to back.
  +0x5d  int32  the slot's player id, and the FREE-SLOT MARKER. The record
                initializer (056d:0d21) writes -1 here; the daily purge skips
                any slot whose value is not > 0, so this — not the name's
                ShortString length byte — is what says "empty". The deletion
                routine passes it as the key to the message, trade-offer and
                report cleanups, which is what identifies it as the player id
                (a BBS user number: inferred from that use, not proven).
  +0x62  int32  population. Seeded 100 by the initializer, and the field the
                chemical and biological strikes subtract from before printing
                a millions-of-civilians line.
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
  +0x130 .. +0x161   the DIPLOMATIC RELATION row: 25 words, one per empire
                     letter A..Y. Values are -1 Enemy, 0 None, and 1..7 for the
                     seven pacts in Diplomacy-menu order (1 Tariff Trade
                     Agreement, 2 Protective Trade, 3 Free Trade Agreement,
                     4 Terrorist Prevention, 5 Intelligence Alliance,
                     6 Technology Agreement, 7 Full Defense Alliance); 8 and 9
                     are menu items (Declaration Of War, View Treaties) that
                     share the same name table and are never stored here.
                     VERIFIED: `break_diplomatic_treaty` writes `xor ax,ax` to
                     both parties' rows (`BRE.OVR 0x1a8f0`, `0x1a912`), so a
                     Declaration Of War leaves 0 behind rather than 8, and a
                     sweep of every write site at both relation displacements
                     found no site that stores 8 at all. This matters to any
                     code testing the row against a range: BRE's own battle
                     report loop admits `> 5`, which reaches 6 and 7 only.

                     BRE indexes it by the RAW ASCII letter, so every access
                     reads `[es:di + 2*letter + 0xae]` and the displacement
                     collides with the Mountain count above — see the
                     bre-gather skill's disassembly reference. Both rows are
                     written: forming a pact writes the current player's, and
                     break_diplomatic_treaty clears the partner's through the
                     all-empires array as well.

                     Only ELEVEN sites in the whole overlay touch it, all via
                     the current-player pointer, which is itself the finding that
                     a Full Defense Alliance cannot reach across planets: no
                     interplanetary code path can read a relation row at all.
  +0x130 .. +0x161   the diplomatic relation with each of the 25 slots, one
                     int16 per slot. For the enum see the fuller entry above —
                     8 is NOT among the stored values. Indexed by the raw ASCII
                     letter, so the code's displacement carries a folded
                     `base - 2*'A'` and a hit only means "relation" when a
                     `shl ax,1` precedes it. Each realm holds its OWN row, so a
                     pair's relation is stored twice and both copies have to be
                     maintained together.
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
  +0x2b9 .. +0x2bb   three penalty bytes, zeroed at turn start (BRE.EXE 0x62c4)
                     and spent during the end-of-turn step:
                       +0x2b9  subtracted from military morale (BRE.OVR 0xc1a1)
                       +0x2ba  subtracted from popular support (0xcf41)
                       +0x2bb  civil-war severity, in percent (0xc59a): halves
                               popular support and destroys that percentage of
                               every military unit type. The food-allocation
                               routine writes all three; an anti-crack check
                               writes 50 into the third.
  +0x211 .. +0x231   the Trading Market "For Sale" escrow, one int32 per slot in
                     the market screen's own order: Trooper, Jet, Turret,
                     Bomber, Food, (unused key 6), Agent, Tank, Carrier.
                     MEASURED: listing 73 of 100 troopers put 73 at +0x211 and
                     left 27 at +0x76. Net worth sums each of these with its
                     home counterpart, and the pirate raid's category ladder
                     reads them on five of its sixteen faces — so escrowed
                     military is NOT hidden from pirates.
  +0x26f int32  agents held (the market's escrowed agents are at +0x229)
  +0x28a real48 the day the realm was last played, as a date serial. The daily
                purge truncates it, adds the configured DeletionDays (a word at
                DS:0x6f99, default 7) and deletes the slot when the sum falls
                before today (a Real48 at DS:0x8606).
  +0x33f int32  SDI program funding, in WHOLE THOUSANDS of gold. The strength
                routine (BRE.EXE 056d:1139) reads it, multiplies by 1000 and
                divides by 10 x (totalRegions + 1) before the square root; the
                screen prints it followed by a literal ",000".
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

## Trade offer record (one field mapped)

A pending trade offer is a 0x97-byte record with its own integrity word at
`+0x93`/`+0x95`, checksummed over the whole 0x97 bytes by the same routine the
empire records use. Only one field of it has been read so far:

```
  +0x60  byte   the SENDER's turns-remaining-today at the moment of sending.
                Written by create_trade_offer (BRE.OVR 0x260CD, unit offset
                0x238B); read by process_trade_offer (0x24D6B, unit offset
                0x05B1), which compares it against the RECIPIENT's own +0x285
                and leaves the offer pending while the recipient has more turns
                left than the sender did. 0xFF is a sentinel meaning no gate —
                which path writes it is NOT established.
```

The rest of the record is unmapped. See `docs/mechanics-reference.md` for what
the field means in play.

## Deleting an empire record

Every deletion in the game funnels through one routine, `BRE.OVR 0x0079c1`. It
has exactly two callers — the sysop's Delete Empire in `manage_players`, and the
daily-maintenance purge at `BRE.OVR 0x007ed2` — which is why a crushed or
abdicated realm is still there for the rest of the game day and gone the next.
The purge loops the 25 slots, skips any whose `+0x5d` is not > 0, and marks a
slot for deletion when it has no regions (`total_regions` < 1), no population
(`+0x62` < 1), has gone unplayed past DeletionDays, or was never played and the
game has been running more than three days. The slot currently in play is exempt.

The routine itself clears the player's messages, trade offers and reports by
`+0x5d`; calls `BRE.OVR 0x050d74`, which loops the slots and zeroes the relation
at `+0x130` in BOTH directions (the deleted realm's row toward each rival and
each rival's row toward it); and finishes with `056d:0d21`, which `FillChar`s the
whole 1069-byte record to zero and seeds the new-realm defaults — `+0x5d` = -1,
`+0x62` = 100 population, `+0x76` = 100 troopers, `+0x8e`/`+0x92` = 100 morale
and support. The attack resolver never deletes anything: a realm crushed at
`BRE.OVR 0xef90` just hands over its land and surviving units.

`0x050d74`'s other caller is `confirm_end_game`, which wipes the whole relation
table at the end of a season.

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
0fd0:1841   real square root
0c03:12e1   min(int32,int32)            0c03:129b   max(int32,int32)
056d:0ec6   sum of the nine region counts (total regions)
056d:18f0   ruin N regions: calls the proportional remover (056d:11f1) across the
            nine counts, then adds the same N to Waste at +0xb6. Both the
            nuclear and the chemical strike reach it; the biological one does
            not, which is why a plague leaves the land alone.
056d:0d21   blank an empire record and seed the new-realm defaults
056d:1139   SDI strength %: clamp(0..100, trunc(sqrt(funding[+0x33f] * 1000
            / (totalRegions + 1) / 10)))
0fd0:1792   real -> int, TRUNCATES
0fd0:179a   real -> int, ROUNDS      <- the two are easy to confuse; which one a
                                        routine uses changes results by one unit
```

## Integrity check — direct edits are rejected

Each record ends with an **integrity dword at `+0x429`**, which BRE recomputes
and compares at load (the verifier sits at BRE.EXE file offsets
`0x84D5`–`0x87C4`, after the "GAME.DAT <empire> corrupt" string). A record whose
field is edited without that value being recomputed is silently DISCARDED — the
caller is prompted to name a new realm, with no "tampered" message. The dword at
`+0x33b` is a second such guard: it mirrors gold (`+0x66`), and the pair has to
agree.

**So prefer in-game changes for staging a test** — buying regions over a few
turns, or cloning a state BRE itself produced (see the snapshot workflow). A
local, unpublished helper can reseal a record for a scenario that cannot be
reached in play; it is deliberately not described here, and nothing about it is
needed to understand BRE or to build the clone.

The same load pass migrates one field: `+0x329`, when positive, is divided by
1000 into the SDI funding field (`+0x33f`) and cleared.

**The record ARRAY starts at file offset 2489** (= BRE's slot letter `A`; the
two 1069-byte blocks before it are header/template area, not slots). The old
note here that named file offsets 2521/3590 as the first name fields was
counting from the wrong base; BRE's own target-picker letters settle it.

Always back up `data/game.dat` (and `planet.bre`) before any experiment.
