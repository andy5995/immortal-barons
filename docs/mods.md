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
| `price_low`, `price_high` | The range a realm's price for the unit moves inside | 1 to 1,000,000, at least 100 apart |
| `price_step` | How far that price can move in one turn | 1 to the width of the range |

The units are `trooper`, `jet`, `turret`, `bomber`, `tank` and `carrier`.

Without mods, a trooper is 1 on attack and defense, a jet 2 on attack, a
turret 2 on defense and a tank 3.5 on both; bombers and carriers are 0. Rules
that belong to a unit stay in force under a mod: only jets that carriers can
move go into an attack, every unit the realm holds defends (jets included,
whether or not a carrier could move them), and a HeadQuarters
raises a tank's strength by the same proportion as without mods, up to 4.5/3.5
of its `offense` and `defense` when finished. Technology and morale scale the
total as usual. Pirate raids and the help an alliance sends use their own
figures, which `values.toml` does not change.

The Maintenance Costs setting still applies to a mod's `upkeep`: None charges
nothing, Low a quarter of it and High four times it.

### HeadQuarters

The HeadQuarters is one building per realm, so its figures are one `[hq]`
section in `values.toml`, not one per unit. Every key is optional:

| Key | Meaning | Limits |
| --- | --- | --- |
| `tank_bonus` | How much a tank gains for each point of HeadQuarters completion, in troopers (up to 2 decimal places), when a tank's `offense` or `defense` is at its base of 3.5. A tank with another weight gains in the same proportion, so a finished HeadQuarters raises every tank's strength by `100 x tank_bonus / 3.5` percent | 0 to 0.1 |
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
| `research` | The type's weight in the realm's research count | 0 to 100 |

Gold is `(gold_base + draw) x gold_share percent`, scaled by popular support for
the `support_share` part of it, then raised by gold technology. Idle gold is not
raised by technology.

Waste is ruined land: it may change only `upkeep`, `gold_share` and
`food_share`, and every other key must stay 0. A type that pays gold or food a
mod gave it gets its own line in the turn report.

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

`mod.toml` describes the mod. `name` and `version` are required:

```toml
name = "cheap-tanks"
version = "1.0"
label = "Cheap Tanks"       # shown on the Game Setup screen; blank shows the name
description = "Tanks cost less to buy, build and keep."
```

A mod that holds a script also needs `script_api` (see
[Script versions](#script-versions)).

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
and `regions.star` is an error.

## Scripts

A mod may also hold scripts: `pirates.star`, which decides pirate raids, and
`regions.star`, which decides what each realm's regions do. They are written
in [Starlark](https://github.com/bazelbuild/starlark), a small language that
looks like Python. When two mods in the list both have `pirates.star`, the
later mod's is used. Region scripts combine instead: see below.

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
normal pirate raid instead. The game writes the reason to its error output
once each time the game runs, naming the mod.

### Region figures

`regions.star` must define `regions(realm, rows)`. It changes the figures of
[Regions](#regions) for one realm at a time, so a realm's land can do more or
less depending on the realm: a bonus for small realms, deserts that farm only
once a realm has played a hundred turns.

`realm` is the realm whose land is asked about. Its fields are `name`, `day`
(the game day), `land`, `people`, `support`, `tax`, `turns_played`, and
`regions`, which holds how many regions of each type it has, by the type's
name: `realm.regions.desert`.

`rows` holds the figures every realm plays by, the base game's with your
`values.toml` sections applied, by the type's name: `rows["coastal"].gold_base`.

`regions` returns the figures it changes for this realm, by type and key, or
`{}` to change nothing. A type or key it leaves out keeps the value in `rows`:

```python
def regions(realm, rows):
    if realm.land >= 500:
        return {}
    coastal = rows["coastal"]
    return {"coastal": {"gold_base": coastal.gold_base * 2}}
```

The answers must stay inside the limits in the table above, and waste may still
change only the keys it may change in `values.toml`.

The game asks for a realm's figures whenever it needs them: for the turn, but
also for each screen that shows a projection, such as the income a realm can
expect. So a region script has no `rand()` or `day()`: it must give the same
answer each time it is asked the same question, or a screen would show one
figure and the turn pay another. Gold is left out of `realm` for the same
reason, since it changes during a turn.

When more than one mod in the list has `regions.star`, every one of them runs,
in the order the mods are listed. Each is given, as `rows`, the figures the one
before it returned, so a mod that makes deserts farmable and a mod that makes
coasts richer both take effect. Where two scripts change the same figure, the
later mod wins, as it does in `values.toml`.

If a script fails, because of an error, by running out of steps, or by
returning a figure outside its limits, its own changes are dropped for that
realm and the next script is given the `rows` it was given. The game writes
the reason to its error output once each time the game runs, naming the mod.

### Script versions

A mod that holds a script says in `mod.toml` which version of the script
interface it was written for:

```toml
script_api = 1
```

The script interface is what a script is given (`realm`, `rows`, `factions`
and their fields) and what it may return. This version of the game runs
version 1. When a later version of the game changes it in a way that could
break a script that worked before (a field renamed or removed, a function's
parameters changed), the version goes up and the ChangeLog says what changed.
The game then refuses a mod written for a version it does not run, before the
game starts, and says which mod and why, rather than letting the script fail
during play. A mod written for a newer version than the game runs is refused
the same way.

Adding something new, a field a script may read or a key it may return, does
not raise the version, so a mod keeps working until a change actually affects
it. A mod with no script does not need `script_api`.

### What every script may do

A script cannot read files, the clock, or other scripts (`load` is not
available). `while` loops and recursion are not allowed, and neither are
`if` and `for` at the top level of the file: put them inside a function. Each
call to `raid` or `regions` may run for at most 1,000,000 steps, and so may the
top level of the file when it loads.

A script is as trusted as the rest of the mod folder. It runs inside the game
itself, and nothing limits how much memory it uses, so install only scripts
you have read.

`-check-mod` loads each script and checks that it defines `raid` or `regions`
with two parameters. It cannot check what they return, because that depends on
the game. Watch the error output once the game has played some turns.

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
