# BRE's `RESOURCE.DAT`: the per-install settings file

`RESOURCE.DAT` is Barren Realms Elite's own per-installation configuration file,
in the directory holding `BRE.EXE`. It is plain text, one `Keyword value` per
line, `;` for comments, and booleans may be written `TRUE`/`FALSE`, `ON`/`OFF`,
`YES`/`NO`, `T`/`F` or `1`/`0`. It is the original's equivalent of IB's
`bbs.cfg`: a sysop's own file that no other board and no league officer writes.

`docs/bre.doc` documents 38 keywords. **The binary reads 56, of which 21 are
undocumented** — and three of the documented ones are read by nothing at all.
The undocumented remainder is where several whole mechanics hide: the lottery,
the monarch's name, and six switches that remove features from the game
outright.

## How the list was recovered

Every setting is read through one of three resident helpers, called with the
keyword as a Pascal `ShortString` loaded immediately before the call:

| Helper | Type |
|---|---|
| `0d61:0025` | number |
| `0d61:002a` | boolean |
| `0d61:002f` | string |

So the complete set is the set of call sites. The catalog
(`docs/dev/bre-v0988-disassembly.json`) lists every caller of the three, which
is what makes the sweep exhaustive rather than a grep: `load_display_name_settings`
(39 sites), `exe_025d_proc_0080` (8), `parse_doorfile` (4), `configure_local_game`
(4), `load_pirate_settings` (2), `initialize_fossil_port` (2),
`exe_09bd_proc_0808` (2), `initialize_game_runtime` (1), `ovr_00ad05_proc_0004`
(1), `exe_0a42_proc_002d` (1) and `select_bios_console_io` (1, reaching the
boolean reader directly rather than through the resident stub). Each routine's
keywords then come from `find-string --function <name> ""`. No segment base is
guessed anywhere in this.

Two call sites resist it: the number and the boolean read in
`exe_09bd_proc_0808` reference no identifier-shaped string in their own unit, so
those two keywords are unknown. Everything below is the 56 that resolved.

Most of them sit in one table of consecutive ShortStrings at `BRE.OVR 0x5680f`,
read by `load_display_name_settings` (`0x056a03`) — the routine's name in the
catalog understates it; it loads the feature switches too.

## The undocumented settings

### Feature switches — each removes a feature from the game

Six booleans, default on. Every one is tested twice: once where the main menu is
built (`show_game_settings`, `BRE.OVR 0x013753`), which skips registering the
item and its hotkey, and again at the feature's own entry point, which returns
before doing anything. A switched-off feature is therefore gone rather than
hidden.

| Keyword | Global | Menu test | Feature test |
|---|---|---|---|
| `DIPLOMACY` | `0x6d4e` | `0x13ce2` | `run_diplomacy_menu` returns at entry (`0x1c80f`) |
| `MESSAGES` | `0x6d4f` | `0x13dae` | `send_local_message` returns at entry (`0x1d853`) |
| `TRADING` | `0x6d50` | `0x13e4d` | inside `run_trading_market_menu` (`0x26e78`) |
| `TRADINGMARKET` | `0x6d51` | `0x13e54` | `run_trading_market_menu` (`0x26e7f`, `0x26eb6`) and `draw_trading_market` (`0x3d0d6`) |
| `COVERTOPS` | `0x6d52` | `0x13cb5` | `enter_covert_operations_menu` returns at entry (`0x179e9`) |
| `MACROS` | `0x6d53` | `0x13ea7` | **none** |
| `LOTTERY` | `0x76c0` | — | `run_lottery` returns at entry (`0x01861f`) |

`MACROS` is the exception worth knowing: the byte is written by the loader and
read in exactly one place, the menu builder. Switching it off hides **Write
Macros** from the menu and nothing else — the Ctrl-key expansion of macros
already recorded goes on working.

`LOTTERY` has no menu item to hide; its own routine is the only reader. The
mechanic it gates is specified in `docs/mechanics-reference.md`.

### `LEADER` — the crown's name

A string, up to 30 characters, held at `0x6d92` and defaulting to the literal
`Queen Royale` (`BRE.OVR 0x564de`). It is read by the tax refund
(`queen_refund 0x18364`), the crown-tax payment prompt
(`allocate_turn_budget 0x2f9cc`), the random-event resolver (`0xe6d6`) and the
news writers (`0x4ec31`, `0x4f0a0`) — so the sysop renames the monarch
everywhere the game speaks of them.

### `PIRATENAMEn` / `PIRATECOLORn` — the nine pirate factions

The keyword is built at run time by appending the faction digit to the base
string, and the value indexes an array: names are 31 bytes apart from `DS:0x1293`,
colours one byte apart from `DS:0x13c9` (`BRE.OVR 0x5726c`, `0x572c7`). So each
of the nine factions can be renamed and recoloured per board. IB reached the
same place from the other direction: its factions carry IB-original names
because the original's cannot be copied.

### The rest

| Keyword | Type | What it is |
|---|---|---|
| `FOODMARKETNAME` | string | renames the Food Market on screen |
| `BANKNAME` | string | renames the Bank |
| `IPPIRATENEWS` | boolean | interplanetary pirate news (`write_local_attack_news 0x4e53c`) |
| `LOCALPIRATENEWS` | boolean | local pirate news (`0x4e54a`) |
| `MINOUTBOUNDTIME` | number | read by `run_interbbs_maintenance` (`0xa2ce`) |
| `STRICTPROCESSING` | boolean | gates `process_incoming_interbbs_data` (`0x3be2b`) |
| `EXTERNALSCORESANSI` | string | path |
| `EXTERNALSCORESASCII` | string | path |
| `EXTERNALSCORESNAME` | string | name |
| `LOCKBAUDRATE` | number | a second keyword beside the documented `LockedBaud`; `parse_doorfile` reads both |
| `TopScoresAmt` | number | how many rows the scores file carries, `0x795b`; the documented `BBSAmt`/`PlayerAmt` do not exist |

The purpose column is traced from the code for the news and InterBBS entries.
For the names and score paths it is read off the keyword and its helper type,
not from following the value — treat those as likely rather than proven.

## Documented but absent

Three keywords in `bre.doc` have **no matching string anywhere in `BRE.EXE` or
`BRE.OVR`** at v0.988, so nothing can be reading them:

- `NoTimeCheck` — documented as switching off the time-limit check
- `BBSAmt`, `PlayerAmt` — documented beside the scores-file placeholders, of
  which only `TopScoresAmt` exists

Either they were dropped and the manual never caught up, or they were renamed.
`whatsnew.doc` is a changelog and may describe a build that is not this one, so
do not treat the manual as evidence that a setting works.

## What this means for IB

`LOTTERY` is implemented, as `Lottery` in `bbs.cfg` (`docs/mechanics-reference.md`).

The other six feature switches have **no IB counterpart**. `World.VisitCovert`,
`VisitTrading` and `VisitMessage` look like three of them and are not: they are
per-player preferences on the Preferences menu, deciding whether a stage is
offered during that player's turn, and a player can turn them back on. A sysop
switch that removes Diplomacy, Messages, Trading, the Trading Market, Covert
Operations or Write Macros from the board would be a new setting, and belongs in
`bbs.cfg` beside `Lottery` for the same reason: the original leaves it to each
installation, so a league's boards may differ.

Renaming the monarch (`LEADER`) has no IB counterpart either. IB says "Queen
Royale" throughout, in code and in translated catalogs.

`PURGENETMAIL` is documented rather than hidden, so it is absent from the lists
above, but it is worth reading for what it says about the original's design. It
is a boolean, default off, that has the game sweep its own leftover netmail out
of the netmail directory during `BRE INBOUND` and `BRE PLANETARY` — the
envelopes other boards' copies of the game left behind, which the manual frames
as saving the sysop from deleting that mail by hand.

The original therefore wrote file-attach netmail like `barons-ftn` does, and
shipped the receiving board a way to clean up after it. IB has no counterpart,
which is #223. Line 7 of the original's own `bbs.cfg` carries the other half of
the same thought: a mailer setting of `NONE` writes no `.MSG` at all, for the
boards the manual says were running the game with no mail system.

Both are from `docs/bre.doc`. Neither has been confirmed against the binary.

## Every keyword the binary reads

Undocumented ones are marked. Read via the string, number or boolean lookup as
shown.

**`load_display_name_settings` (`BRE.OVR 0x056a03`), table at `0x5680f`:**
`FOODMARKETNAME`*, `BANKNAME`*, `DIPLOMACY`*, `MESSAGES`*, `TRADING`*,
`TRADINGMARKET`*, `COVERTOPS`*, `MACROS`*, `ANSISCORES`, `ASCIISCORES`,
`TODAYNEWSANSI`, `YESTERDAYNEWSANSI`, `TODAYNEWSASCII`, `YESTERDAYNEWSASCII`,
`IPPIRATENEWS`*, `LOCALPIRATENEWS`*, `DELETIONDAYS`, `MINOUTBOUNDTIME`*,
`PURGENETMAIL`, `STRICTPROCESSING`*, `LEADER`*, `LOTTERY`*,
`EXTERNALSCORESANSI`*, `EXTERNALSCORESASCII`*, `EXTERNALSCORESNAME`*,
`RECONUPDATE`, `OLDESTRECON`, `TIMINGCHECK`, then the scores-file placeholders
`BBSScore`, `BBSWorth`, `BBSLand`, `BBSWorthLand`, `PlayerScore`, `PlayerWorth`,
`PlayerLand`, `PlayerWorthLand`, `ANSIExtension`, `ASCIIExtension`,
`TopScoresAmt`*.

**Elsewhere:** `GAMENAME`, `VGA`, `STATUS.BACKGROUND`, `STATUS.TITLE`,
`STATUS.TOPINFO`, `STATUS.BOTTOMINFO`, `STATUS.LINE`, `STATUS.EXTRA`
(`exe_025d_proc_0080`); `MULTITASKER` (`exe_0a42_proc_002d`); `FOSSIL`, `Com`
(`initialize_fossil_port`); `LOCAL`, `LOCKEDBAUD` (`configure_local_game`);
`LOCKBAUDRATE`* (`parse_doorfile`); `PIRATENAME`*, `PIRATECOLOR`*
(`load_pirate_settings`); `BIOS` (`select_bios_console_io`).

Twenty-one marked, 56 in all, plus the two unresolved sites named above.
