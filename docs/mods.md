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

This first version can change seven figures for each of the six military units:

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

To see every value a mod can change, as the game sets it without mods, run:

```
immortal-barons -print-base-mod
```

It prints them in the form of a `values.toml`. Copy the sections you want into
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

`values.toml` holds the changes, one section per unit. List only what changes:

```toml
[units.tank]
build_cost = 250  # the standard build cost is 500
upkeep = 0.3
price_low = 500
price_high = 900
```

Both files are [TOML](https://toml.io/). Lines starting with `#` are comments.

The game reads a mod strictly. A key it does not know, a unit it does not know,
or any file in the folder other than `mod.toml`, `values.toml` and
`pirates.star` is an error.

## Scripts

A mod may also hold `pirates.star`, a script that decides pirate raids. It is
written in [Starlark](https://github.com/bazelbuild/starlark), a small
language that looks like Python. When two mods in the list have one, the later
mod's script is used.

The script must define `raid(realm, factions)`. Each turn a realm plays, after
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

A script cannot read files, the clock, or other scripts (`load` is not
available). `while` loops and recursion are not allowed, and neither are
`if` and `for` at the top level of the file: put them inside a function. Each call to `raid`
may run for at most 1,000,000 steps, and so may the top level of the file
when it loads.

If the script fails in a turn, because of an error, by running out of steps,
or by returning a raid the game does not allow, that realm's turn uses the
normal pirate raid instead. The game writes the reason to its error output
once each time the game runs, naming the mod.

A script is as trusted as the rest of the mod folder. It runs inside the game
itself, and nothing limits how much memory it uses, so install only scripts
you have read.

`-check-mod` loads the script and checks that it defines `raid` with two
parameters. It cannot check what `raid` returns, because that depends on the
game. Watch the error output once the game has played some turns.

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
