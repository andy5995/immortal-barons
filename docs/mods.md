# Mods

This page is for the BBS operator (the sysop). Players do not need to read it.

A mod changes some of the numbers the game plays by. Without mods, the game
plays the standard Immortal Barons rules. A game with a mod is a
different game, and the Game Setup screen lists every mod in play and its
option values, so players can see that.

On a stand-alone board, you choose mods for the next reset. In a
[league](#in-a-league), the Coordinator chooses them and `-league-reset` sends
them to every board. Member boards play the copy in that order and do not need
mod folders of their own. A mod with scripts plays only on a board whose sysop
has set `AcceptLeagueScripts Yes` in `bbs.cfg`; see
[Command-line options and settings](#command-line-options-and-settings).

A complete [example mod](examples/mods/example/) uses most of what a mod can
do, with comments in each file.

## Command-line options and settings

- `-init-mod <dir>`: create an empty mod, with a `mod.toml` and a
  `values.toml`, then exit. `<dir>` is a name, which is created in the `mods`
  folder of the data directory, or a path. It refuses a folder that already
  exists. See [Making a mod](#making-a-mod).
- `-check-mod <dir>`: check one mod on its own, with the limits a reset uses,
  print what it changes, then exit. `<dir>` is a name in the `mods` folder or a
  path. See [Checking a mod](#checking-a-mod).
- `-sim-mod <dir>`: play the mod against the base game in throwaway worlds of
  computer barons and print how they differ, then exit. It takes `-sim-days`,
  `-sim-seeds`, `-sim-realms`, `-sim-csv <file>` and `-mod-option <id>=<value>`.
  See [Testing how a mod plays](#testing-how-a-mod-plays).
- `-print-base-mod`: print every value a mod can change, as the game sets it
  without mods, in the form of a `values.toml`, then exit. The `[hq]`,
  `[combat]`, `[regions.<id>]` and `[attacks.<kind>.<side>]` sections are included. Copy
  the sections you want into your mod and change them.
- `-reset`: start a new game after the settings editor. The editor starts from
  the default settings, which turn no mods on, so pick them again on its Mods
  page each time. See [Turning mods on](#turning-mods-on).
- `-reset-from-config`: start a new game with the mods that `config.json` lists
  under `Mods`, with their `ModOptions`. See [Turning mods on](#turning-mods-on).
- `-reload-mods`: put the mods `config.json` lists, as their folders are now,
  into the game in progress, and print what changed. It is for testing on a
  stand-alone board; a league board refuses it. See
  [Testing a mod](#testing-a-mod).
- `-league-reset <YYYY-MM-DD>`: the Coordinator resets every board in the
  league, and the order carries the mods in the Coordinator's `config.json`.
  See [In a league](#in-a-league).
- `AcceptLeagueScripts Yes` in `bbs.cfg`: lets a board play a league mod that
  carries scripts. The default is `no`. See [In a league](#in-a-league) and
  [`AcceptLeagueScripts`](bbs-cfg.md#acceptleaguescripts).

The [Command Reference](command-reference.md) lists these with every other
option.

## What a mod can change

A mod can change the figures of each of the six military units, the
HeadQuarters, what the Attack Rewards and Attack Damage levels mean, and the
figures of each region type. The
[Mod Reference](mod-reference.md) lists every key with what it means and its
limits. The units are `trooper`, `jet`, `turret`, `bomber`, `tank` and
`carrier`; their keys are under
[Units](mod-reference.md#units).

Without mods, a trooper is 1 on attack and defense, a jet 2 on attack, a
turret 2 on defense and a tank 3.5 on both; bombers and carriers are 0. Rules
that belong to a unit stay in force under a mod: only jets that carriers can
move go into an attack, every unit the realm holds defends (jets included,
whether or not a carrier could move them), and a HeadQuarters raises the
strength of each unit with an `hq_bonus`, which without mods is the tank alone,
to 4.5/3.5 of its `offense` and `defense` when finished. Technology and morale scale the
total as usual. Pirate raids and the help an alliance sends use their own
figures, which `values.toml` does not change.

The Maintenance Costs setting still applies to a mod's `upkeep`: None charges
nothing, Low a quarter of it and High four times it.

### HeadQuarters

The HeadQuarters is one building per realm, so its figures are one `[hq]`
section in `values.toml`, not one per unit. The strength it adds to units is
each unit's `hq_bonus`. Every key is optional; see
[HeadQuarters](mod-reference.md#headquarters) for the keys.

For example, a HeadQuarters that is cheap and quick to build:

```toml
[hq]
price_base = 1000
price_per_turn = 10
build_per_turn = 10
```

What interplanetary play does to a HeadQuarters is not changed by a mod: the
interplanetary battle's own tank figures, the share a winning invasion takes,
and what a Sabotage HQ agent takes. League mods use those same figures.

### Combat

The sysop chooses an Attack Rewards level and an Attack Damage level, each
None, Low, Medium or High. The `[combat]` section in `values.toml` redefines
what each level means, and the sysop still chooses the level. Every key is
optional; see [Combat](mod-reference.md#combat) for the keys and limits.

- `capture_none`, `capture_low`, `capture_medium` and `capture_high` are the
  percent of the defender's regions a winning same-planet attack takes, by
  Attack Rewards. `ip_capture_*` are the same for an interplanetary attack,
  which has its own ladder in the base game.
- `retreat_*` are the percent of its own force a side loses in a same-planet
  battle before it breaks off, by Attack Damage.
- `capture_floor` is the fewest regions a winning same-planet attack takes.
- `score_win` and `score_crush` are the score for each region captured, in an
  ordinary win and in a total conquest.

Each level has its own key, so a mod that wants "5 points more than the base"
writes each base value plus 5; `-print-base-mod` shows the base values. For
example, a game where a High reward takes a third of a realm and a win pays
more:

```toml
[combat]
capture_high = 33
score_win = 100
```

What an interplanetary battle loses stays the attack type's own rate, rescaled
by the Attack Damage level, and an interplanetary attack's minimum capture
stays 10 regions. A mod does not change either.

### Regions

Each region type has its own `[regions.<id>]` section in `values.toml`. The
types are `coastal`, `mountain`, `desert`, `river`, `agricultural`, `urban`,
`industrial`, `technology` and `waste`. Every key is optional, and every type
takes every key: the base game has only rivers fish and only Industrial regions
pay idle gold, but a mod may give a desert food or an urban region gold, and the
game works it out the same way. All figures are whole numbers. The keys and
their limits are under [Regions](mod-reference.md#regions).

Gold is `(gold_base + draw) x gold_share percent`, scaled by popular support for
the `support_share` part of it, then raised by gold technology. Idle gold is not
raised by technology.

Waste is ruined land: it may change only the keys the reference marks, and
every other key must stay 0. A type that pays gold or food a
mod gave it gets its own line in the turn report, and the advisors and the
turn report name whichever types the mod has boosting production, doing
research or building units.

For example, deserts that can be farmed a little, and cheaper land to keep:

```toml
[regions.desert]
food_base = 150
food_rate = 100

[regions.agricultural]
upkeep = 600
```

To see every value a mod can change, as the game sets it without mods, use
[`-print-base-mod`](#command-line-options-and-settings).

### Attack effects

What a missile does is set by a table of effects, one for each kind of attack:
`nuclear`, `chemical`, `biological`, `conventional.won`, `conventional.lost`,
`ip.attack.won`, `ip.attack.lost`, `ip.nuclear`, `ip.chemical` and one for each
part of the S3-Sabre's dial: `sabre.intelligence`, `sabre.people`,
`sabre.military_bases`, `sabre.airbases`, `sabre.regions` and `sabre.food`.
Each kind has two tables, except the missiles described below.
`[attacks.<kind>.target]` is what the attack does to the realm it hits.
`[attacks.<kind>.attacker]` is what it does back to the realm that fired it.
The base game has nothing in the attacker tables.

The battle of a same-planet attack does not change here: its losses, its capture
of regions and its score are as before, unless the [`[combat]`](#combat)
section changes them. The two `conventional` kinds add to it,
`conventional.won` when the attacker wins and `conventional.lost` when the
attacker is repelled. They are empty in the base game. The sections are written
as `[attacks.conventional.won.target]` and so on. Regions that
`conventional.won` captures count as captured by the battle: they are in its
report, its score and the regions the winner types. A realm left with no
people, or with no land by anything but a won same-planet attack, falls at the
next daily check and not at once.

The `ip.attack.won` and `ip.attack.lost` kinds add to an interplanetary attack
of any type, Group Attack included, that fought a battle. The resolver's battle,
capture and losses do not change. The `target` table runs on the defending board
after the battle, and a `capture` there is carried home with the battle's own
regions and split among the contributors. Each contributor's report gives its
own share of what the table took, as it does for the battle's kills. The
`attacker` table runs on the attacker's board when the result comes home, on
each realm of that board that contributed, as if it had attacked alone. It
cannot `capture` regions, because the target is on another board. Its unit
capture is the exception, and runs on the defending board (see below).

The `ip.nuclear`, `ip.chemical` and `sabre` kinds are missiles that arrive from
another board, and they act on the board that receives them. Only their
`target` table is used: the attacker is on another board, so a non-zero
`attacker` table or a `capture` on these kinds is an error. What stays built in
is whether a missile gets through (the misfire, SDI and garrison rolls), where
the Sabre's dial lands, its backfire and the score. Every board in a league
plays the same mods.

Apart from those limits, any kind may use any effect, except that only the four battle kinds (`conventional.won`, `conventional.lost`, `ip.attack.won` and `ip.attack.lost`) can capture units. The effects are `waste`, `destroy` and
`capture` (a percent of the regions), `people`, one `kill_<unit>` for each
unit, `food`, `morale`, `support` and `hq`. In the attacker table, `capture`
gives the target that share of the attacker's regions. Captured regions arrive
untyped for a player, who chooses the types at the start of the next turn, and
as coastal regions for an AI baron.

### Capturing units

`capture_<unit>` takes a percent of that military unit (`capture_trooper`,
`capture_jet`, `capture_turret`, `capture_bomber`, `capture_tank`,
`capture_carrier`), and `capture_random` takes a percent of all the surviving
units together, drawn at random across the types. A unit left unset is never
captured. In a `target` table the attacker takes the units. In an `attacker`
table the target takes them, so a defender who repels an attack can keep some
of the attacker's tanks:

```toml
[attacks.conventional.lost.attacker]
capture_tank = 10
```

The percent is of the units still alive after the kills in the same table, so a
unit a `kill_<unit>` destroyed cannot also be captured. What the percent is of:

- From an attacker, the survivors of the force it sent. Units it held back are
  safe, and an attacker table cannot capture turrets or carriers, which an
  attack never sends.
- From a defender, the survivors among the units that stood in the defense:
  troopers, jets, turrets and tanks. `capture_from_stock = 1` in a `target`
  table widens that to the defender's whole remaining stock, bombers and
  carriers included. It is not allowed in an `attacker` table.
- From an ally that sent troops to the defense, the same percent of the part of
  its detachment that survived, never its stock.

A kind may capture in both tables. The `target` table runs first. On an
interplanetary win or a repelled strike, the units the `target` table captures
come home with the result and are shared among the contributors like regions.
An `attacker` table's capture on an interplanetary kind is made on the
defending board: the arriving force's survivors lose the units before they go
home, and the defending realms keep them.

A percent is a whole number, or `[pct, up, down]` for a percent that varies:
`pct + Random(up) - Random(down)`. The base nuclear strike is `waste = [7, 3, 3]`.
`morale` and `support` are the part that is kept, as `[num, den]`. `[3, 4]`
keeps three quarters, and `[1, 1]` keeps all of it. `hq` is points of
HeadQuarters completion, not percent. The limits are under
[Attacks](mod-reference.md#attacks).

A strike's report names what it took. If a mod sets any effect the usual
report is built around to zero, the report lists what was taken instead.

For example, a nuclear strike that also destroys a fifth of the turrets and
leaves a little fallout on the realm that fired it:

```toml
[attacks.nuclear.target]
kill_turret = 20

[attacks.nuclear.attacker]
people = [1, 2, 0]
```

## Making a mod

A mod is a folder under `mods` in the data directory. The folder's name is the
mod's name. It may use lowercase letters, digits, `-` and `_`, and must start
with a letter or digit.

[`-init-mod`](#command-line-options-and-settings) creates a new, empty mod to
start from:

```
immortal-barons -init-mod cheap-tanks
```

It writes a `mod.toml` with the name, a first version, the current `mod_api`
and the MIT license, and a `values.toml` that changes nothing yet. Add the
license's text as a `LICENSE` file before you share the mod.

```
data/
  mods/
    cheap-tanks/
      mod.toml
      values.toml
```

`mod.toml` describes the mod. `name`, `version`, `mod_api` and `license` are
required:

```toml
name = "cheap-tanks"
version = "1.0"
mod_api = 1                 # the mod format this mod was written for
license = "CC-BY-4.0"       # an SPDX identifier, https://spdx.org/licenses/
label = "Cheap Tanks"       # shown on the Game Setup screen; blank shows the name
description = "Tanks cost less to buy, build and keep."
```

`version` is the mod's own version, for its author to change as they like.
`license` says how others may use and share the mod; [`-check-mod`](#checking-a-mod) prints it.

Put the license's full text in the mod folder too, as `LICENSE`, `LICENSE.txt`,
`LICENSE.md` or `COPYING`. Immortal Barons is under the MIT license, so choose
a license that is compatible with it. It must also let others copy and share
the mod: a league's Coordinator sends the mod, license file included, to every
board in the league.
`mod_api` is the version of the mod format the game reads; see
[Mod format versions](#mod-format-versions).

`values.toml` holds the changes, one section per unit, plus `[hq]`, `[combat]`, one
section per region type and the `[attacks.<kind>.<side>]` sections. List only
what changes:

```toml
[units.tank]
build_cost = 250  # the standard build cost is 500
upkeep = 0.3
price_low = 500
price_high = 900
```

Both files are [TOML](https://toml.io/). Lines starting with `#` are comments.

The game reads a mod strictly. A key it does not know, a unit it does not know,
or any file in the folder other than `mod.toml`, `values.toml`, `pirates.star`,
the region scripts in a `regions` folder and a license file is an error.

## Options

A mod can let the sysop choose a few values when a game starts. Put the choices
in `[options.<id>]` tables in `mod.toml`. [`-init-mod`](#command-line-options-and-settings)
writes a commented example. The fields of a table are under
[Options](mod-reference.md#options).

For example:

```toml
[options.tank_cost]
type = "number"
label = "Tank cost"
description = "Production points to build one tank"
default = 500
min = 1
max = 1000

[options.fast_start]
type = "bool"
label = "Fast start"
description = "Use the cheap starting values"
default = false
```

A number option can stand in for a figure in `values.toml`:

```toml
[units.tank]
build_cost = { option = "tank_cost" }
```

A bool option selects a section whose name begins with `when`:

```toml
[when.fast_start.units.tank]
build_cost = 400
```

That section applies only when the bool is True. It is read after the plain
values, so it overrides them. A section whose bool is False is still checked
against the limits, so the mod is refused now rather than when the option is
turned on. A script reads either kind with `option("<id>")`:
a number is a whole number, and a bool is `True` or `False`. A chosen number
still has to pass the figure's normal limit in the [Mod Reference](mod-reference.md).

The sysop makes the choices on the Mods page of either settings editor, or in
`config.json` under `ModOptions`. A game keeps its choices until the next
reset, and a league reset carries them to every board.

## Scripts

A mod may also hold scripts: `pirates.star`, which decides pirate raids, and a
`regions` folder with a script for each region type the mod changes, which
decides what that type does realm by realm. They are written in
[Starlark](https://github.com/bazelbuild/starlark), a small language that looks
like Python.

Scripts follow the same rule as `values.toml`: when two mods in the list have
the same script, the later mod's is used and the earlier one's does not run.
The settings editor's mod list names each script that does not run this way.
Which mods work together, and in what order, is for the people who make them
to say; the game only applies them in the order they are listed.

### Pirate raids

`pirates.star` must define `raid(realm, factions)`. Each turn a realm plays, after
the usual checks (pirates are on, the realm is out of protection), the game
calls `raid` instead of rolling for a raid itself.

`realm` is the realm that is playing, and `factions` is a list of the pirate
factions. The fields of each are under
[`raid`](mod-reference.md#pirate-script).

`raid` returns a list of raids, or `[]` for none:

```python
def raid(realm, factions):
    if realm.jets < 10000 or rand(5) != 0:
        return []
    return [{"faction": 1, "take": "jets", "market": False}]
```

`take` and `market` are described in the reference. A realm may get up to 10
raids in one turn. Each raid takes as much as a normal raid would, and grants
the faction regions the same way. The built-in functions `rand`, `day` and
`option` are listed under
[Built-in functions](mod-reference.md#built-in-functions).

If the script fails in a turn, because of an error, by running out of steps,
or by returning a raid the game does not allow, that realm's turn uses the
normal pirate raid instead. The game writes the reason to its error output,
naming the mod: once for each different reason, up to five, each time the
game runs.

### Region figures

A script for one region type goes in the mod's `regions` folder, named for the
type: `regions/desert.star`, `regions/coastal.star`, and so on for any of the
nine types listed under [Regions](#regions). A mod has a script only for the
types it changes:

```
data/
  mods/
    farmland/
      mod.toml
      regions/
        desert.star
```

Each script must define `region(realm, row)`. It changes the type's figures
for one realm at a time, so a realm's land can do more or less depending on
the realm: a bonus for small realms, deserts that farm only once a realm has
played a hundred turns.

`realm` is the realm whose land is asked about, and `row` holds the type's
figures as every realm plays them, the base game's with `values.toml` applied,
by the keys of [Regions](mod-reference.md#regions): `row.food_base`. Their
fields are under
[`region`](mod-reference.md#region-script).

`region` returns the figures it changes for this realm, or `{}` to change
nothing. A key it leaves out keeps its value in `row`:

```python
def region(realm, row):
    if realm.turns_played < 100:
        return {}
    return {"food_base": row.food_base + 150, "food_rate": 100}
```

The answers must stay inside the [limits](mod-reference.md#regions), and a script for waste
may change only the keys waste may change in `values.toml`.

The game asks for a realm's figures whenever it needs them: for the turn, but
also for each screen that shows a projection, such as the income a realm can
expect. So a region script has no `rand()` or `day()`: it must give the same
answer each time it is asked the same question, or a screen would show one
figure and the turn pay another. Gold is left out of `realm` for the same
reason, since it changes during a turn.

The other fields of `realm` change between turns, so a projection holds for
the realm as it stands. A script that reads `turns_played`, `people` or
`support` can pay next turn a figure this turn's screen did not show: in the
example above, the screen on turn 99 shows the deserts' figures without the
bonus, and turn 100 pays them with it.

If a script fails, because of an error, by running out of steps, or by
returning a figure outside its limits, that realm plays the type's figures
from `row` unchanged. The game writes the reason to its error output, naming
the mod and the script: once for each different reason, up to five, each time
the game runs.

### What every script may do

A script cannot read files, the clock, or other scripts (`load` is not
available). `while` loops and recursion are not allowed, and neither are
`if` and `for` at the top level of the file: put them inside a function. Each
call to `raid` or `region` may run for at most 1,000,000 steps, and so may the
top level of the file when it loads.

A script keeps nothing from one call to the next. Values set at the top level
of the file are frozen once it has loaded: `raid` and `region` can read a list
or dict made there, but changing it is an error.

A script is as trusted as the rest of the mod folder. It runs inside the game
itself, and nothing limits how much memory it uses, so install only scripts
you have read.

[`-check-mod`](#checking-a-mod) loads each script and checks that it defines `raid` or `region`
with two parameters. It cannot check what they return, because that depends on
the game. Watch the error output once the game has played some turns.

## Mod format versions

`mod_api` in `mod.toml` says which version of the mod format a mod was written
for. The mod format is the folder's layout, the keys `values.toml` may set and
what each one means, and what a script is given and may return. This version
of the game reads `mod_api = 1`.

When a later version of the game changes the format in a way that could make a
mod that worked before fail to load, or load and play differently, the number
goes up and the ChangeLog says what changed. The game then refuses a mod
written for another version before the game starts, and names the mod. A mod
written for a newer version than the game reads is refused the same way.

Adding something new, a key `values.toml` may set or a field a script may
read, does not raise the number, so a mod keeps working until a change
actually affects it.

## Checking a mod

```
immortal-barons -check-mod cheap-tanks
```

[`-check-mod`](#command-line-options-and-settings) takes a name or the path to
any mod folder, such as `-check-mod ~/my-mods/cheap-tanks`. It lists
every value the mod changes. It also lists the options and shows any choice from
`config.json` that differs from the default:

```
Cheap Tanks 1.0 (029923dee3137351): OK
Tanks cost less to buy, build and keep.
License: CC-BY-4.0
Options:
  Tank cost: number, default 500, from 1 to 1000
  Fast start: bool, default False
Options changed from their defaults:
  Tank cost: 500 -> 250
Changes from the base game:
  tank.build_cost: 500 -> 250
  tank.upkeep: 0.6 -> 0.3
  tank.price_low: 1125 -> 500
  tank.price_high: 2250 -> 900
```

## Turning mods on

Mods change only when a new game starts. On a stand-alone board, turn them on
in the settings editor that [`-reset`](#command-line-options-and-settings) opens,
on its Mods page, or list them in `config.json` under `Mods` and run
[`-reset-from-config`](#command-line-options-and-settings):

```json
"Mods": ["cheap-tanks"],
"ModOptions": {"cheap-tanks": {"tank_cost": 250}}
```

`ModOptions` is by mod name, then option ID. Leave out an ID to use its
default. The Mods page has an options row for the mods in play, so the settings
editors set the same values. `-check-mod` shows the choices it reads from
`config.json`.

When two mods change the same value, the one later in the list wins. The reset
checks the combined values against the limits, so two mods that each pass
`-check-mod` can still be refused together.

`-reset-from-config` uses the list in `config.json` as it stands. The Mods page refuses a mod that will not load,
alone or with the mods already on, and the reset prints the mods it applied.

A game keeps the mods it started with until the next reset:

- Changing `Mods` or `ModOptions` in `config.json` affects only the next
  `-reset-from-config`.
- The game saves the chosen option values with each mod when it starts.
- The game saves a copy of each mod when it starts. If you edit or delete a
  mod's folder during a game, the game keeps the copy and writes a warning to
  its error output (stderr) each time it starts. Under a BBS, that output goes
  to the sysop's log.
- [`-reload-mods`](#command-line-options-and-settings) is the one exception on
  a stand-alone board. It is for testing (see [Testing a mod](#testing-a-mod)).
  A league board refuses it; see [In a league](#in-a-league).

## In a league

The Coordinator chooses the mods for every board. Put the names in the
Coordinator's `config.json` under `Mods`, then run
[`-league-reset <date>`](#command-line-options-and-settings). The
signed reset order carries every file of each mod and the Coordinator's option
choices, so member boards do not need their own copies. The command prints the
mods it is carrying.

Member boards leave `Mods` out of their own `config.json`. On their next
`-planetary` run, they check the files against the hashes in the order and play
those mods. The mods must load together and pass the same limits as on a
stand-alone reset. A board that cannot load them does not reset. Its
`-planetary` run prints the reason and records it in `planetary.log`. The
board keeps the order and tries it again on each later `-planetary` run. Once
the board can play the mods, that order starts the new game, with its original
date and mods. The Coordinator does not need to reset the league again.

Every packet a board sends carries a fingerprint of the league's rules, with
each mod's hash and chosen option values included. If someone edits a board's
copy of a mod in `world.json`, the fingerprint changes and the other boards
hold its packets.

Scripts are another choice. A mod that carries `pirates.star` or a script in
`regions` runs only on a board whose sysop sets
[`AcceptLeagueScripts Yes`](#command-line-options-and-settings) in
`bbs.cfg`. The setting defaults to `no`. A board with the default refuses a
scripted reset and keeps the order. It resets at its first planetary run after
its sysop turns the setting on. If the board moves to another league first, the
order is discarded. Scripts run on that machine with no memory limit, so read
them before you allow them.

## Testing a mod

Test a mod in a game of its own, in a separate data directory, so that nothing
in a real game changes. Start the game once:

```
immortal-barons -data ./mod-test -reset-from-config
```

`./mod-test/config.json` lists the mod under `Mods`, and the mod's folder is in
`./mod-test/mods`. Play it with `-local`:

```
immortal-barons -data ./mod-test -local
```

After you change the mod, load the change into the same game:

```
immortal-barons -data ./mod-test -reload-mods
```

It prints each value that changed, and the next turn plays by them. The game
keeps everything else: the realms, the land, and the market prices set when the
game started. A mod that will not load is refused, and the game keeps the
values it had. Start a new game with `-reset-from-config` when you want to see
the mod from the start.

A league board refuses [`-reload-mods`](#command-line-options-and-settings)
because its mods come from the Coordinator's reset order.

## Testing how a mod plays

```
immortal-barons -sim-mod cheap-tanks
```

[`-sim-mod`](#command-line-options-and-settings) plays the mod and the base
game side by side in throwaway worlds of computer barons, with the same seeds
and settings, and prints how they differ. It runs in memory: it reads the mod's folder
(for a bare name, it looks in the `mods` folder of the data directory), reads no
game and writes nothing except the CSV file you ask for. It takes a name or a
path, like `-check-mod`.

Each world plays `-sim-days` days (60) with `-sim-realms` barons (8). It runs
`-sim-seeds` seeds (8), because one run is one storyline and a property such as
"the leader runs away with the game" shows only across many. The report reads
the worlds at five points over the run and gives, for each figure, the mean
across the seeds for the base game and the mod, and the difference. A second
table gives the lowest and highest seed on the last day. The figures are the
realms still alive and the number eliminated, the leader's and the median
realm's net worth and regions, the ratio of the two net worths, gold in hand
and banked, and the total of each unit. `-sim-csv <file>` writes every figure,
spread included, as CSV for a spreadsheet, after `#` comment lines giving the
mod, its version and hash, the run's settings and every option's value. The
limits are 1000 seeds, 3650 days and 25 realms, the planet's slots.

Set an option for the run with `-mod-option <id>=<value>`, as often as needed.
A bool takes `true`, `false`, `on`, `off`, `yes` or `no`, and a number takes digits; an id
the mod does not declare, a value of the wrong type or a repeated id is an
error. The report header lists every option's value, defaults included. The
option flag works only with `-sim-mod`. To compare values of one option, run the
command once per value and give each its own `-sim-csv`.

A script that fails is reported under the table, and the game falls back to the
built-in rules where it failed, as in a real game. The `ip.*` and `sabre.*`
attack kinds need more than one board, so they are not simulated; the report
lists them when the mod sets any.

The barons play by their own code and do not adapt to a mod. The report shows
what a change does mechanically, such as a cheap unit compounding a winner's
lead, and not how a person would use it. It is a first check before
play-testing, not a replacement for it.
