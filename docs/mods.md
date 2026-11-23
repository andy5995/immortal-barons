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

This first version can change five figures for each of the six military units:

| Key | Meaning | Limits |
| --- | --- | --- |
| `cost` | Production points to build one unit | 1 to 1,000,000 |
| `networth` | Net worth of one unit, in gold (up to 3 decimal places) | 0 to 10,000 |
| `upkeep` | Gold to maintain one unit per turn at the Medium Maintenance Costs setting (up to 1 decimal place) | 0 to 1,000 |
| `price_low`, `price_high` | The range a realm's price for the unit moves inside | 1 to 1,000,000, at least 100 apart |
| `price_step` | How far that price can move in one turn | 1 to the width of the range |

The units are `trooper`, `jet`, `turret`, `bomber`, `tank` and `carrier`.

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
description = "Tanks cost less to build and keep."
```

`values.toml` holds the changes, one section per unit. List only what changes:

```toml
[units.tank]
cost = 250      # the standard cost is 500
upkeep = 0.3
```

Both files are [TOML](https://toml.io/). Lines starting with `#` are comments.

The game reads a mod strictly. A key it does not know, a unit it does not know,
or any other file in the folder is an error.

## Checking a mod

```
immortal-barons -check-mod cheap-tanks
```

A plain name is looked up in the `mods` folder of the data directory. You can
also give the path to any mod folder, such as `-check-mod ~/my-mods/cheap-tanks`.

This checks the mod on its own, with the same limits a reset uses, and lists
every value it changes:

```
Cheap Tanks 1.0 (f9296d291178373f): OK
Tanks cost less to build and keep.
Changes from the base game:
  tank.cost: 500 -> 250
  tank.upkeep: 0.6 -> 0.3
```

## Turning mods on

Mods change only when a new game starts. List them in `config.json` under
`Mods`, then reset the game:

```json
"Mods": ["cheap-tanks"]
```

When two mods change the same value, the one later in the list wins. The reset
checks the combined values against the limits, so two mods that each pass
`-check-mod` can still be refused together.

`-reset` and `-reset-from-config` both use the list. The settings editor has no
page for mods yet, so `-reset` keeps the list from `config.json` instead of
clearing it. The reset prints the mods it applied.

A game keeps the mods it started with until the next reset:

- Changing `Mods` in `config.json` affects only the next reset.
- The game saves a copy of each mod when it starts. If you edit or delete a
  mod's folder during a game, the game keeps the copy and writes a warning to
  its error output (stderr) each time it starts. Under a BBS, that output goes
  to the sysop's log.
