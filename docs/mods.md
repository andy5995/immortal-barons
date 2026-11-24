# Mods

This page is for the BBS operator (the sysop). Players do not need to read it.

A mod changes some of the numbers the game plays by. Without mods, the game
plays the standard Immortal Barons rules. A game with a mod is a
different game, and the Game Setup screen lists every mod in play so players
can see that.

Mods work on a stand-alone board only for now. A board in an inter-BBS league
refuses them, because every board in a league must play the same rules and
nothing yet sends a mod to the other boards.

## What a mod can change

A mod can change seven figures for each of the six military units, the
HeadQuarters, and the figures of each region type. The units first:

| Key | Meaning | Limits |
| --- | --- | --- |
| `build_cost` | Production points an Industrial region spends to build one unit (not the purchase price) | 1 to 1,000,000 |
| `networth` | Net worth of one unit, in gold (up to 3 decimal places) | 0 to 10,000 |
| `upkeep` | Gold to maintain one unit per turn at the Medium Maintenance Costs setting (up to 1 decimal place) | 0 to 1,000 |
| `offense` | Strength of one unit when it attacks another realm on this planet, in troopers (up to 2 decimal places). For a tank, its strength with no HeadQuarters | 0 to 100; 0 for `turret` and `carrier` |
| `defense` | Strength of one unit when its realm is attacked from this planet, in troopers (up to 2 decimal places). For a tank, its strength with no HeadQuarters | 0 to 100 |
| `hq_bonus` | How much the unit gains for each point of HeadQuarters completion, in troopers (up to 2 decimal places), when its `offense` or `defense` is 3.5. A unit with another strength gains in the same proportion, so a finished HeadQuarters multiplies its strength by `1 + 100 x hq_bonus / 3.5`: 4.5/3.5 at the tank's 0.01, 13.5/3.5 at 0.1. Without mods only tanks have one | 0 to 0.1 |
| `price_low`, `price_high` | The range a realm's price for the unit moves inside | 1 to 1,000,000, at least 100 apart |
| `price_step` | How far that price can move in one turn | 1 to the width of the range |

The units are `trooper`, `jet`, `turret`, `bomber`, `tank` and `carrier`.

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
each unit's `hq_bonus`, above. Every key is optional:

| Key | Meaning | Limits |
| --- | --- | --- |
| `build_start` | Completion percent a newly bought HeadQuarters begins at | 1 to 100 |
| `build_per_turn` | Percent it gains each turn until it reaches 100 | 1 to 100 |
| `battle_loss_min` | Completion percent a HeadQuarters loses when its realm loses a defense | 0 to 100 |
| `battle_loss_jitter` | A random 0 up to one less than this is added to that loss | 1 to 100 |
| `price_base` | Gold a HeadQuarters costs a realm that has played no turns | 1 to 1,000,000,000 |
| `price_per_turn` | Gold the price rises with each turn the realm has played | 0 to 1,000,000 |
| `price_jitter` | A random 0 up to one less than this is added to the price. 0 makes the price exact | 0 to 1,000,000 |
| `price_cap` | The price never rises past this, less its own random draw | `price_base` to 1,000,000,000 |
| `price_cap_jitter` | A random 0 up to one less than this is taken off the cap | 0 to one less than `price_cap` |

For example, a HeadQuarters that is cheap and quick to build:

```toml
[hq]
price_base = 1000
price_per_turn = 10
build_per_turn = 10
```

What interplanetary play does to a HeadQuarters is not changed by a mod: the
interplanetary battle's own tank figures, the share a winning invasion takes,
and what a Sabotage HQ agent takes. Those only happen in a league, where mods
are not allowed.

### Regions

Each region type has its own `[regions.<id>]` section in `values.toml`. The
types are `coastal`, `mountain`, `desert`, `river`, `agricultural`, `urban`,
`industrial`, `technology` and `waste`. Every key is optional, and every type
takes every key: the base game has only rivers fish and only Industrial regions
pay idle gold, but a mod may give a desert food or an urban region gold, and the
game works it out the same way. All figures are whole numbers.

| Key | Meaning | Limits |
| --- | --- | --- |
| `gold_base`, `gold_rate` | Gold one region pays each turn: `gold_base` plus a random 0 up to one less than `gold_rate`, drawn once a turn for the whole type | 0 to 1,000,000 |
| `gold_share` | Percent of that gold paid out as gold (a river's other 25 percent is fish) | 0 to 100 |
| `support_share` | Percent of the gold that follows popular support, as coastal regions' does. The rest is paid in full | 0 to 100 |
| `idle_gold_base`, `idle_gold_rate` | Gold one region pays, as Industrial regions do, on the part of the realm's production not allocated to units: `idle_gold_base` plus a random 0 up to one less than `idle_gold_rate` when none is allocated, and that share of it otherwise | 0 to 1,000,000 |
| `food_base`, `food_rate` | Food one region grows each turn: `food_base`, raised by food technology, plus a random 0 up to one less than `food_rate` | 0 to 100,000 |
| `food_share` | Percent of that food that is produced | 0 to 100 |
| `pop_cap` | The type's weight in the population capacity | 0 to 10,000 |
| `upkeep` | Gold one region costs to keep each turn, before technology and the Maintenance Costs setting | 0 to 100,000 |
| `unit_points` | Production a region adds to the realm's unit pool | 0 to 100,000 |
| `production_boost` | The unit pool rises by this many times the type's share of all the realm's land; the whole boost is capped at 150 percent of normal | 0 to 100 |
| `research` | The type's weight in the realm's research count, which never passes the realm's land | 0 to 100 |

Gold is `(gold_base + draw) x gold_share percent`, scaled by popular support for
the `support_share` part of it, then raised by gold technology. Idle gold is not
raised by technology.

Waste is ruined land: it may change only `upkeep`, `gold_share` and
`food_share`, and every other key must stay 0. A type that pays gold or food a
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

To see every value a mod can change, as the game sets it without mods, run:

```
immortal-barons -print-base-mod
```

It prints them, the `[hq]` and `[regions.<id>]` sections included, in the form of a `values.toml`. Copy the sections you want into
your mod and change them.

## Making a mod

A mod is a folder under `mods` in the data directory. The folder's name is the
mod's name. It may use lowercase letters, digits, `-` and `_`.

```
data/
  mods/
    cheap-tanks/
      mod.toml
      values.toml
```

`mod.toml` describes the mod. `name`, `version` and `mod_api` are required:

```toml
name = "cheap-tanks"
version = "1.0"
mod_api = 1                 # the mod format this mod was written for
label = "Cheap Tanks"       # shown on the Game Setup screen; blank shows the name
description = "Tanks cost less to buy, build and keep."
```

`version` is the mod's own version, for its author to change as they like.
`mod_api` is the version of the mod format the game reads; see
[Mod format versions](#mod-format-versions).

`values.toml` holds the changes, one section per unit, plus `[hq]` and one
section per region type. List only what changes:

```toml
[units.tank]
build_cost = 250  # the standard build cost is 500
upkeep = 0.3
price_low = 500
price_high = 900
```

Both files are [TOML](https://toml.io/). Lines starting with `#` are comments.

The game reads a mod strictly. A key it does not know, a unit it does not know,
or any file in the folder other than `mod.toml`, `values.toml`, `pirates.star`
and the region scripts in a `regions` folder is an error.

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

`realm` is the realm that is playing. Its fields are `name`, `regions`,
`gold`, `troopers`, `jets`, `turrets`, `tanks` and `agents`, plus `market`,
which holds the `troopers`, `jets`, `turrets`, `tanks` and `agents` the realm
has for sale on the Trading Market.

`factions` is a list of the pirate factions. Each has `slot` (its number,
starting at 0), `name`, `regions`, `gold`, and the `troopers`, `jets`,
`turrets`, `tanks` and `agents` it holds.

`raid` returns a list of raids, or `[]` for none:

```python
def raid(realm, factions):
    if realm.jets < 10000 or rand(5) != 0:
        return []
    return [{"faction": 1, "take": "jets", "market": False}]
```

`take` is one of `troopers`, `jets`, `turrets`, `tanks`, `gold` and `agents`.
`market` is optional: `True` takes from the realm's Trading Market listing
instead of what it holds. Gold is not sold there, so it cannot be taken that
way. A realm may get up to 10 raids in one turn. Each raid takes as much as a
normal raid would, and grants the faction regions the same way.

Two built-in functions are added to Starlark's own:

- `rand(n)` returns a whole number from 0 to n - 1, drawn from the game's own
  random numbers.
- `day()` returns the game day.

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

`realm` is the realm whose land is asked about. Its fields are `name`, `day`
(the game day), `land`, `people`, `support`, `tax`, `turns_played`, and
`regions`, which holds how many regions of each type it has, by the type's
name: `realm.regions.mountain`.

`row` holds the type's figures as every realm plays them, the base game's with
`values.toml` applied, by the keys in the table under [Regions](#regions):
`row.food_base`.

`region` returns the figures it changes for this realm, or `{}` to change
nothing. A key it leaves out keeps its value in `row`:

```python
def region(realm, row):
    if realm.turns_played < 100:
        return {}
    return {"food_base": row.food_base + 150, "food_rate": 100}
```

The answers must stay inside the limits in the table, and a script for waste
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

`-check-mod` loads each script and checks that it defines `raid` or `region`
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

A plain name is looked up in the `mods` folder of the data directory. You can
also give the path to any mod folder, such as `-check-mod ~/my-mods/cheap-tanks`.

This checks the mod on its own, with the same limits a reset uses, and lists
every value it changes:

```
Cheap Tanks 1.0 (029923dee3137351): OK
Tanks cost less to buy, build and keep.
Changes from the base game:
  tank.build_cost: 500 -> 250
  tank.upkeep: 0.6 -> 0.3
  tank.price_low: 1125 -> 500
  tank.price_high: 2250 -> 900
```

## Turning mods on

Mods change only when a new game starts. Turn them on in the settings editor
that `-reset` opens, on its Mods page, or list them in `config.json` under
`Mods` and run `-reset-from-config`:

```json
"Mods": ["cheap-tanks"]
```

When two mods change the same value, the one later in the list wins. The reset
checks the combined values against the limits, so two mods that each pass
`-check-mod` can still be refused together.

`-reset` starts from the default settings, which turn no mods on, so pick
them again on the Mods page each time. `-reset-from-config` uses the list in
`config.json` as it stands. The Mods page refuses a mod that will not load,
alone or with the mods already on, and the reset prints the mods it applied.

A game keeps the mods it started with until the next reset:

- Changing `Mods` in `config.json` affects only the next `-reset-from-config`.
- The game saves a copy of each mod when it starts. If you edit or delete a
  mod's folder during a game, the game keeps the copy and writes a warning to
  its error output (stderr) each time it starts. Under a BBS, that output goes
  to the sysop's log.
- `-reload-mods` is the one exception. It puts the mods `config.json` lists,
  as their folders are now, into the game in progress, and prints what changed.
  It is for testing (see below).

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

`-reload-mods` does not work on a league board, because a league board cannot
use mods.
