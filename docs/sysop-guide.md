# Sysop Guide — Running Immortal Barons

This guide is for the BBS operator (the sysop). It explains how to set up the
game on your board, and how to join an inter-BBS league. Players do not need to
read this.

## What you need

The game is one program: `barons-door`. It runs as a native door under modern
BBS software such as Synchronet or Mystic. You do not need DOSBox or DOSEMU.

The game keeps all its files in one data directory (default `./data`). Point
the `-data` option at it.

## First-time setup

Run the setup once to create the config file:

```
barons-door -setup -data /path/to/data
```

This opens the **Configuration Editor**: a menu of every game rule (turns per
day, protection turns, land and market settings, interest and investment rates,
tax and region limits, costs and attack settings, number of AI barons, game
length, and more). Change what you like, then press `0` to save `config.json`,
or `Q` to cancel. `-setup` only writes the config; it does not create or change
any game world (the world is created when your first caller logs in).

There is also a **`-reset`** command described later. It opens the *same*
Configuration Editor, but after you save it also **starts a fresh game** (clears
all empires and re-seeds the AI). Use `-setup` when you first install the game,
and `-reset` when you want to reconfigure and start over.

## Registering the door

Set up the game as an external program (a "door") in your BBS software. Your BBS
writes a dropfile when a caller runs the door. The game reads it to learn who
is playing and how much time they have.

- The game reads **DOOR32.SYS** (both Synchronet and Mystic write it) or
  **DOOR.SYS**. Point the game at it with `-dropfile`, or let it search the
  current directory.
- The caller's handle from the dropfile becomes the name of their realm.

A typical command line:

```
barons-door -dropfile /path/to/DOOR32.SYS -data /path/to/data
```

## Daily maintenance

Run maintenance once a day (a nightly event is the usual place):

```
barons-door -maint -data /path/to/data
```

This advances the game one day for every day that passed, lets the AI barons
take their turns, and refreshes each player's turns.

## Starting a fresh game (reset)

To reconfigure and start the game over, run:

```
barons-door -reset -data /path/to/data
```

This opens the same **Configuration Editor** as `-setup` (adjust any rules, then
`0` to save or `Q` to cancel). After you save, it clears all empires (players
re-create their realm the next time they log in) and re-seeds the AI barons on a
fresh day one. It does not pick a winner. The old world is saved to
`world.json.bak` in the data directory first, so you can restore it if you reset
by mistake. Cancelling with `Q` leaves the game untouched.

## Inter-BBS (league) play

A league is a group of boards whose players compete against each other. To join
one, turn on inter-BBS play and give your board a name.

Edit `config.json` and set:

- `"IBBS": true` — turn on the inter-BBS menus.
- `"BoardID"` — a short unique name for your board (your "planet").
- `"InboundDir"` — the directory where packets from other boards arrive.
- `"OutboundDir"` — the directory where the game writes packets for other
  boards.

Ask your League Coordinator for the node list and add it to your data directory
as **`ibnodes.dat`** (see the format below).

## How packets move (you choose the schedule)

The game never moves files between boards. It only reads and writes packet
files in your inbound and outbound directories. Moving the files between boards
is your job, and you choose how often it happens.

The inter-BBS step is:

```
barons-door -planetary -data /path/to/data
```

It reads every packet in your inbound directory, applies it, and writes new
packets to your outbound directory. It also runs automatically inside `-maint`
when inter-BBS play is on.

A common setup:

1. A caller plays the game.
2. After the caller exits, or on a schedule you pick, run `-planetary`.
3. Your transport script copies each file from your outbound directory to the
   inbound directory of every other board (over FidoNet, a sync tool, scp, a
   shared mount — whatever you use).
4. The next `-planetary` run on each board reads and applies those files.

Run it as often as you like. More often means shorter travel times between
planets. The in-game "Travel Times" screen shows players how recently packets
have arrived, so they know how fast operations move.

## The node list: `ibnodes.dat`

The node list names every board in the league. It uses the same simple layout
as the original BRE `BRNODES.DAT`. Each board is six lines, and a blank line
separates boards:

```
1
Avalon
363/277
Orlando
FL
USA

2
Pier 7
106/477
Houston
TX
USA
```

The six lines are: node number, board (planet) name, network address, city,
state or province, and country. Board number 1 is the League Coordinator.

## League-wide rules (Coordinator only)

The League Coordinator sets the rules that must match across the whole league.
These are all the fields marked with a star in the Configuration Editor: turns
per day, protection turns, game length, land market and daily land, interest
and investment rates, tax and region and player limits, buy-military mode, and
the cost, damage, and reward levels. Set them in the Coordinator's own
`config.json`, then broadcast them to every board:

```
barons-door -league-config -data /path/to/data
```

This writes a settings packet to your outbound directory. Each member board
adopts the settings on its next `-planetary` run. Member boards accept these
settings only from the Coordinator's board (node 1), so no one else can change
the league rules. Only the Coordinator's board may send this packet.

## The Coordinator

There are two different "coordinator" ideas, and they are not the same thing:

- **League Coordinator** — the sysop of board number 1. This person keeps the
  node list and hands it out to the other boards.
- **BBS Coordinator** — a *player* your board elects. Players vote in the
  System menu, and the player with the most votes gets the Coordinator menu.
  Votes can change at any time.
