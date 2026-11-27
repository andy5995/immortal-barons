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
| `prefs` | The [`[prefs.<id>]`](#player-preferences) tables, yes/no choices each player makes for their own realm and the mod's scripts read. At most 8 per mod and 25 across the mods in a game. | optional |

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

### Player preferences

Each `[prefs.<id>]` table takes these fields. See [Player preferences](mods.md#player-preferences).

| Field | Meaning | Needed |
| --- | --- | --- |
| `label` | The name on the player's Preferences menu, at most 40 characters. It is shown as written, not translated. | required |
| `description` | A line saying what the preference changes, shown under the Preferences menu. At most 200 characters. | optional |
| `default` | What a player who has made no choice gets: `true` or `false`. | required |

## values.toml

These are the tables a `values.toml` may hold. Any other table or key is an error.

| Table | Meaning |
| --- | --- |
| `units` | One `[units.<id>]` table per unit. List only what changes. |
| `hq` | The one `[hq]` table. |
| `combat` | The one `[combat]` table. |
| `regions` | One `[regions.<id>]` table per region type. |
| `attacks` | One `[attacks.<kind>.target]` and one `[attacks.<kind>.attacker]` table per kind of attack. |
| `when` | `[when.<option>.units.<id>]`, `[when.<option>.hq]`, `[when.<option>.combat]`, `[when.<option>.regions.<id>]` and `[when.<option>.attacks.<kind>.<side>]`: the same tables, applied only when the bool option `<option>` is true. Read after the plain values, so they override them. |

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

### Combat

The `[combat]` table redefines what the sysop's Attack Rewards and Attack Damage levels mean, and what a winning same-planet attack pays. The sysop still chooses the level. Every key is a whole number. `capture_*` and `ip_capture_*` are percents of the defender's regions by Attack Rewards level, for a same-planet and an interplanetary attack. An interplanetary share is then scaled by the attack's type and, for an individual strike, doubled. `retreat_*` are percents of a side's own force it loses before it breaks off a same-planet battle, by Attack Damage level; an interplanetary battle's loss stays the attack type's own rate rescaled by the level.

| Key | Meaning | Limits |
| --- | --- | --- |
| `capture_none` | Percent of the defender's regions a winning same-planet attack takes when Attack Rewards is None. | 0 to 100 |
| `capture_low` | The same at Attack Rewards Low. | 0 to 100 |
| `capture_medium` | The same at Attack Rewards Medium. | 0 to 100 |
| `capture_high` | The same at Attack Rewards High. | 0 to 100 |
| `ip_capture_none` | Percent of each defending realm's regions a winning interplanetary attack takes when Attack Rewards is None. It is then scaled by the attack's type. | 0 to 100 |
| `ip_capture_low` | The same at Attack Rewards Low. | 0 to 100 |
| `ip_capture_medium` | The same at Attack Rewards Medium. | 0 to 100 |
| `ip_capture_high` | The same at Attack Rewards High. | 0 to 100 |
| `retreat_none` | Percent of its own force a side loses in a same-planet battle before it breaks off, when Attack Damage is None. | 1 to 100 |
| `retreat_low` | The same at Attack Damage Low. | 1 to 100 |
| `retreat_medium` | The same at Attack Damage Medium. | 1 to 100 |
| `retreat_high` | The same at Attack Damage High. | 1 to 100 |
| `capture_floor` | The fewest regions a winning same-planet attack takes, up to all the defender holds. | 0 to 1,000,000 |
| `score_win` | Score the attacker gains for each region an ordinary same-planet win captures. | 0 to 100,000 |
| `score_crush` | The same when the capture takes every region the defender had. | 0 to 100,000 |

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

### Attacks

The kinds of attack are `nuclear`, `chemical`, `biological`, `conventional.won`, `conventional.lost`, `ip.attack.won`, `ip.attack.lost`, `ip.nuclear`, `ip.chemical`, `sabre.intelligence`, `sabre.people`, `sabre.military_bases`, `sabre.airbases`, `sabre.regions`, `sabre.food`. Each kind has two tables, except the missile kinds below. `[attacks.<kind>.target]` is what the attack does to the realm it hits. `[attacks.<kind>.attacker]` is what it does back to the realm that launched it, and is empty in the base game. The target's effects are applied first. Each table takes these keys, in the order they are applied; list only what changes. The two `conventional` kinds do not replace the same-planet attack's battle, its capture or its losses, whose figures are set in `[combat]`: they add to it, `conventional.won` when the attacker wins and `conventional.lost` when it is repelled, and are empty in the base game. The `ip.attack.won` and `ip.attack.lost` kinds add to an interplanetary attack of any type, Group Attack included, that fought a battle; the resolver's battle, capture and losses are not set here; `[combat]` changes the capture share. The `target` table runs on the defending board after the battle, and a `capture_regions` there is carried home with the battle's own. The `attacker` table runs on the attacker's board when the result comes home, on each realm of that board that contributed, as if it had attacked alone; it cannot `capture_regions`. The one part of an `ip.attack` `attacker` table that runs on the defending board is its unit capture, since that is where the arriving force is. The `ip.nuclear`, `ip.chemical` and `sabre.*` kinds are missiles that arrive from another board and act on the receiving board, so only their `target` table is used: a non-zero `attacker` table or a `capture_regions` on them is an error.

A key marked as a percent takes a whole number, a flat percent, or `[pct, up, down]`: the percent drawn as `pct + Random(up) - Random(down)` and held to 0 to 100. A draw is made only when `up` or `down` is above 0. Every `pct`, `up` and `down` is 0 to 100. `morale` and `support` take `[num, den]`, with `den` 1 to 1,000 and `num` 0 to `den`. In the attacker's table, `capture_regions` gives the target that share of the attacker's regions. The `capture_<unit>` and `capture_random` keys give the other side that share of the military units still alive after the battle's kills, and only the four battle kinds (`conventional.won`, `conventional.lost`, `ip.attack.won`, `ip.attack.lost`) may set them or `capture_from_stock`. An attack sends no turrets or carriers, so an `attacker` table cannot capture them. On an interplanetary win the captured units travel home with the result and are shared among the contributors like the regions.

| Key | Meaning | Form |
| --- | --- | --- |
| `waste` | Percent of the realm's regions turned to waste. The regions stay on its books but earn nothing until cleaned. | percent |
| `destroy` | Percent of the realm's regions destroyed outright. | percent |
| `capture_regions` | Percent of the realm's regions taken by the other side, as land it types at the start of its next turn. | percent |
| `people` | Percent of the realm's people killed. | percent |
| `kill_trooper` | Percent of the realm's Troopers killed. | percent |
| `kill_jet` | Percent of the realm's Jets killed. | percent |
| `kill_turret` | Percent of the realm's Turrets killed. | percent |
| `kill_bomber` | Percent of the realm's Bombers killed. | percent |
| `kill_tank` | Percent of the realm's Tanks killed. | percent |
| `kill_carrier` | Percent of the realm's Carriers killed. | percent |
| `kill_agent` | Percent of the realm's Agents killed. | percent |
| `capture_trooper` | Percent of the realm's Troopers still alive after the battle and the kills that the other side takes. Only a battle can capture units. In a `target` table the defender's base is the units that stood in the defense and, for the troops an ally sent, what survived of that detachment; in an `attacker` table it is the survivors of the force the attacker sent. | percent |
| `capture_jet` | Percent of the realm's Jets still alive after the battle and the kills that the other side takes. Only a battle can capture units. In a `target` table the defender's base is the units that stood in the defense and, for the troops an ally sent, what survived of that detachment; in an `attacker` table it is the survivors of the force the attacker sent. | percent |
| `capture_turret` | Percent of the realm's Turrets still alive after the battle and the kills that the other side takes. Only a battle can capture units. In a `target` table the defender's base is the units that stood in the defense and, for the troops an ally sent, what survived of that detachment; in an `attacker` table it is the survivors of the force the attacker sent. | percent |
| `capture_bomber` | Percent of the realm's Bombers still alive after the battle and the kills that the other side takes. Only a battle can capture units. In a `target` table the defender's base is the units that stood in the defense and, for the troops an ally sent, what survived of that detachment; in an `attacker` table it is the survivors of the force the attacker sent. | percent |
| `capture_tank` | Percent of the realm's Tanks still alive after the battle and the kills that the other side takes. Only a battle can capture units. In a `target` table the defender's base is the units that stood in the defense and, for the troops an ally sent, what survived of that detachment; in an `attacker` table it is the survivors of the force the attacker sent. | percent |
| `capture_carrier` | Percent of the realm's Carriers still alive after the battle and the kills that the other side takes. Only a battle can capture units. In a `target` table the defender's base is the units that stood in the defense and, for the troops an ally sent, what survived of that detachment; in an `attacker` table it is the survivors of the force the attacker sent. | percent |
| `capture_random` | Percent of all the surviving units together, drawn at random across the unit types, that the other side takes. The base is the same as for `capture_<unit>`, after those are taken. | percent |
| `capture_from_stock` | In a `target` table, 1 widens the base of a defender's unit capture from the units that stood in the defense to its whole remaining stock of each unit (including bombers and carriers). Not allowed in an `attacker` table. | 0 or 1 |
| `food` | Percent of the realm's food destroyed. | percent |
| `morale` | The part of the realm's morale that is kept, as `[num, den]`. `[1, 1]` leaves it alone. | `[num, den]` |
| `support` | The part of the realm's popular support that is kept, as `[num, den]`. `[1, 1]` leaves it alone. | `[num, den]` |
| `hq` | HeadQuarters completion points lost, taken off the realm's percent complete and never below 0. | points |

## Scripts

A script is a [Starlark](https://github.com/bazelbuild/starlark) file. The rules every script follows are under [Scripts](mods.md#scripts). Each entry point is called with the arguments below and must return the value described.

### Pirate script

`pirates.star` defines `raid(realm, factions)`. It is called once per realm per turn, after the usual checks. `realm` is a read-only struct:

| Field | Type | Meaning |
| --- | --- | --- |
| `name` | text | The realm's name. |
| `regions` | number | How many regions the realm has. |
| `gold` | number | The realm's gold in hand. |
| `troopers`, `jets`, `turrets`, `tanks`, `agents` | number | How many of each unit the realm holds. |
| `pref` | function | `realm.pref(id)` is `True` or `False`: the player's answer to a preference the mod declares, or its default when they have made none. An id the mod does not declare is an error. |
| `market` | struct | What the realm has for sale on the Trading Market, as a struct with the same unit names: `realm.market.jets`. |

`factions` is a frozen list with one struct per pirate faction, in slot order:

| Field | Type | Meaning |
| --- | --- | --- |
| `slot` | number | The faction's number, starting at 0. A raid names a faction by it. |
| `name` | text | The faction's name. |
| `regions` | number | How many regions the faction has. |
| `gold` | number | The faction's gold. |
| `troopers`, `jets`, `turrets`, `tanks`, `agents` | number | How many of each unit the faction holds. |

`raid` returns a list of at most 10 raids, or `[]` for none. Each raid is a dict:

| Key | Type | Meaning |
| --- | --- | --- |
| `faction` | number | The `slot` of the faction that raids. Required. |
| `take` | text | What the raid takes: one of `troopers`, `jets`, `turrets`, `tanks`, `gold`, `agents`. Required. |
| `market` | bool | `True` takes from the realm's Trading Market listing instead of what it holds. Optional. Gold is not sold there, so `gold` cannot be taken that way. |

### Region script

`regions/<type>.star` defines `region(realm, row)`. It is called for one realm at a time, whenever the game needs the figures of the region type the file is named for. `realm` is a read-only struct:

| Field | Type | Meaning |
| --- | --- | --- |
| `name` | text | The realm's name. |
| `day` | number | The game day. |
| `land` | number | The realm's total land. |
| `regions` | struct | How many regions of each type the realm has, by the type's ID: `realm.regions.mountain`. The IDs are `coastal`, `mountain`, `desert`, `river`, `agricultural`, `urban`, `industrial`, `technology`, `waste`. |
| `people` | number | The realm's population. |
| `support` | number | The realm's popular support. |
| `tax` | number | The realm's tax rate, in percent. |
| `turns_played` | number | How many turns the realm has played. |
| `pref` | function | `realm.pref(id)` is `True` or `False`: the player's answer to a preference the mod declares, or its default when they have made none. An id the mod does not declare is an error. |

`row` is a read-only struct of the type's figures:

| Field | Type | Meaning |
| --- | --- | --- |
| `gold_base`, `gold_rate`, `gold_share`, `support_share`, `idle_gold_base`, `idle_gold_rate`, `food_base`, `food_rate`, `food_share`, `pop_cap`, `upkeep`, `unit_points`, `production_boost`, `research` | number | The type's figures as every realm plays them: the base game's with `values.toml` applied. Each is described under [Regions](#regions). |

`region` returns a dict of the figures it changes, with the keys of `row`, or `{}` to change nothing. A key it leaves out keeps its value in `row`. Every figure must stay inside its limit above, and a script for waste may change only the keys waste may change.

### Built-in functions

These are added to Starlark's own. A region script has no `rand()` or `day()`: it must give the same answer every time it is asked the same question.

| Function | Meaning | Available to |
| --- | --- | --- |
| `rand(n)` | A whole number from 0 to n - 1, drawn from the game's own random numbers. `n` must be at least 1. | pirates |
| `day()` | The game day. | pirates |
| `option(id)` | The value the sysop chose for an option the mod declares. A number is a whole number; a bool is `True` or `False`. | pirates and regions |

