# Mod Reference

This page is written by the game's code: every key, limit and script value
below is read from the tables the game itself uses. It is for the sysop or
author who writes a mod. [Mods](mods.md) explains how a mod works; this page
lists what it may say.

## mod.toml

| Key | Meaning | Needed |
| --- | --- | --- |
| `name` | The mod's name. It must match the folder's name, which may use lowercase letters, digits, `-` and `_`, and must start with a letter or digit. `config.json` lists a mod by it. | required |
| `version` | The mod's own version, for its author to change as they like. | required |
| `mod_api` | The version of the mod format the mod was written for. This game reads `mod_api = 1`; see [Mod format versions](mods.md#mod-format-versions). | required |
| `license` | How others may use and share the mod, as an [SPDX identifier](https://spdx.org/licenses/). `-check-mod` prints it. | required |
| `label` | The name the Game Setup screen shows. Blank shows `name`. | optional |
| `description` | A line saying what the mod does. | optional |
| `options` | The [`[options.<id>]`](#options) tables the sysop may set at a reset. | optional |

### Options

Each `[options.<id>]` table takes these fields. See [Options](mods.md#options).

| Field | Meaning | Needed |
| --- | --- | --- |
| `type` | `number` or `bool`. | required |
| `label` | The short name shown in the settings editors, `-check-mod` and Game Setup. | required |
| `description` | A line saying what the option changes. | optional |
| `default` | The value used when the sysop makes no choice. A number is a whole number. A bool is `true` or `false`; `0` and `1` are refused. | required |
| `min` | Lowest value a number option may take. | required for a number, refused for a bool |
| `max` | Highest value a number option may take. | required for a number, refused for a bool |

## values.toml

These are the tables a `values.toml` may hold. Any other table or key is an error.

| Table | Meaning |
| --- | --- |
| `units` | One `[units.<id>]` table per unit. List only what changes. |
| `hq` | The one `[hq]` table. |
| `regions` | One `[regions.<id>]` table per region type. |
| `when` | `[when.<option>.units.<id>]`, `[when.<option>.hq]` and `[when.<option>.regions.<id>]`: the same tables, applied only when the bool option `<option>` is true. Read after the plain values, so they override them. |

### Units

The units are `trooper`, `jet`, `turret`, `bomber`, `tank`, `carrier`. Each `[units.<id>]` table takes these keys.

| Key | Meaning | Limits |
| --- | --- | --- |
| `build_cost` | Production points an Industrial region spends to build one unit. This is not the purchase price. | 1 to 1,000,000 |
| `networth` | Net worth of one unit, in gold, with up to 3 decimal places. | 0 to 10,000 |
| `upkeep` | Gold to maintain one unit per turn at the Medium Maintenance Costs setting, with up to 1 decimal place. None charges nothing, Low a quarter of it and High four times it. | 0 to 1,000 |
| `offense` | Strength of one unit when it attacks another realm on this planet, in troopers, with up to 2 decimal places. For a tank, its strength with no HeadQuarters. | 0 to 100; 0 for `turret` and `carrier` |
| `defense` | Strength of one unit when its realm is attacked from this planet, in troopers, with up to 2 decimal places. For a tank, its strength with no HeadQuarters. | 0 to 100 |
| `hq_bonus` | How much the unit gains for each point of HeadQuarters completion, in troopers, with up to 2 decimal places, when its `offense` or `defense` is 3.5. A unit with another strength gains in the same proportion, so a finished HeadQuarters multiplies its strength by `1 + 100 x hq_bonus / 3.5`: 4.5/3.5 at the tank's 0.01, 13.5/3.5 at 0.1. Without mods only tanks have one. | 0 to 0.1 |
| `price_low` | Bottom of the range a realm's price for the unit moves inside. | 1 to 1,000,000, at least 100 below `price_high` |
| `price_high` | Top of the range a realm's price for the unit moves inside. | 1 to 1,000,000, at least 100 above `price_low` |
| `price_step` | How far the price can move in one turn. | 1 to the width of the range |

### HeadQuarters

The `[hq]` table takes these keys.

| Key | Meaning | Limits |
| --- | --- | --- |
| `build_start` | Completion percent a newly bought HeadQuarters begins at. | 1 to 100 |
| `build_per_turn` | Percent a HeadQuarters gains each turn until it reaches 100. | 1 to 100 |
| `battle_loss_min` | Completion percent a HeadQuarters loses when its realm loses a defense. | 0 to 100 |
| `battle_loss_jitter` | A random 0 up to one less than this is added to that loss. | 1 to 100 |
| `price_base` | Gold a HeadQuarters costs a realm that has played no turns. | 1 to 1,000,000,000 |
| `price_per_turn` | Gold the price rises with each turn the realm has played. | 0 to 1,000,000 |
| `price_jitter` | A random 0 up to one less than this is added to the price. 0 makes the price exact. | 0 to 1,000,000 |
| `price_cap` | The price never rises past this, less its own random draw. At least `price_base`. | `price_base` to 1,000,000,000 |
| `price_cap_jitter` | A random 0 up to one less than this is taken off the cap. Less than `price_cap`. | 0 to one less than `price_cap` |

### Regions

The region types are `coastal`, `mountain`, `desert`, `river`, `agricultural`, `urban`, `industrial`, `technology`, `waste`. Each `[regions.<id>]` table takes these keys, all whole numbers. Every type takes every key. Waste is ruined land: it may change only the keys marked in the last column, and every other key must stay 0.

| Key | Meaning | Limits | Waste |
| --- | --- | --- | --- |
| `gold_base` | Gold one region pays each turn, before the random part (`gold_rate`). | 0 to 1,000,000 | must be 0 |
| `gold_rate` | A random 0 up to one less than this is added to `gold_base`, drawn once a turn for the whole type. | 0 to 1,000,000 | must be 0 |
| `gold_share` | Percent of that gold paid out as gold. A river's other 25 percent is fish. | 0 to 100 | may change |
| `support_share` | Percent of the gold that follows popular support, as coastal regions' does. The rest is paid in full. | 0 to 100 | must be 0 |
| `idle_gold_base` | Gold one region pays, as Industrial regions do, on the part of the realm's production not allocated to units, when none is allocated. | 0 to 1,000,000 | must be 0 |
| `idle_gold_rate` | A random 0 up to one less than this is added to `idle_gold_base`. When some production is allocated, the idle share of the total is paid. | 0 to 1,000,000 | must be 0 |
| `food_base` | Food one region grows each turn, before the random part. Food technology raises it. | 0 to 100,000 | must be 0 |
| `food_rate` | A random 0 up to one less than this is added to `food_base`. | 0 to 100,000 | must be 0 |
| `food_share` | Percent of that food that is produced. | 0 to 100 | may change |
| `pop_cap` | The type's weight in the realm's population capacity. | 0 to 10,000 | must be 0 |
| `upkeep` | Gold one region costs to keep each turn, before technology and the Maintenance Costs setting. | 0 to 100,000 | may change |
| `unit_points` | Production a region adds to the realm's unit pool. | 0 to 100,000 | must be 0 |
| `production_boost` | The unit pool rises by this many times the type's share of all the realm's land. The whole boost is capped at 150 percent of normal. | 0 to 100 | must be 0 |
| `research` | The type's weight in the realm's research count, which never passes the realm's land. | 0 to 100 | must be 0 |

