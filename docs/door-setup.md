# Door Setup — Running Immortal Barons

This guide is for the BBS operator (the sysop). It explains how to set up the
game on your board, and how to join an inter-BBS league. Players do not need to
read this.

## What you need

The game is one program: `immortal-barons`. It runs as a native door under BBS
software on any platform Go supports.

The game keeps all its files in one data directory (default `./data`). Point
the `-data` option at it.

Run `immortal-barons -help` to see all the command-line options. The
[Command Reference](command-reference.md) explains every option in one place.

## First-time setup

You must create the game once before anyone can play. Run `-reset` to open the
Configuration Editor (starting from built-in defaults), choose your settings, and
seed the world. Until this is done, the game reports "no game found" and refuses
to start, so a caller never lands in an empty, unplayable world.

```
immortal-barons -reset -data /path/to/data
```

To create the game without the editor — using the current `config.json` as-is —
use `-reset-from-config` instead.

This opens the **Configuration Editor**: a menu of every game setting (turns per
day, protection turns, land and market settings, interest and investment rates,
tax and region limits, costs and attack settings, number of AI barons, game
length, and more). Change what you like, then press `S` to save `config.json`
and start a fresh game, or `Q` to cancel. On a brand-new install there is
nothing to clear, so this just writes your config and seeds the starting world.
See "Starting a fresh game" below — it is the same command.

## Registering the door

Set up the game as an external program (a "door") in your BBS software.

- The game reads **DOOR32.SYS** (the dropfile most BBS software writes) or
  **DOOR.SYS**. Point the game at it with `-dropfile`, or let it search the
  current directory.
- The caller's handle from the dropfile becomes the name of their realm.

A typical command line (use the full path to `immortal-barons` only if it is not
installed on your `PATH`):

```
/path/to/immortal-barons -dropfile /path/to/DOOR32.SYS -data /path/to/data
```

### Example: Mystic BBS

In Mystic's menu editor, add a command of type **D3 (Exec DOOR32 program)** and
set its **Data** field to the command line below. Mystic writes the dropfile
into each node's own temporary directory and gives you `%P` for that directory
(with a trailing slash), so the same line works for every node:

```
/path/to/immortal-barons -dropfile %Pdoor32.sys -data /path/to/game-data
```

Two things to note. Mystic writes the file name in lower case (`door32.sys`),
which matters on Linux because file names there are case-sensitive. And the
game's data directory should be a separate directory from the BBS's own files.

Before the first caller connects, seed the game:

```
/path/to/immortal-barons -reset -data /path/to/game-data
```

This opens the settings editor (see "First-time setup" above); when you save
with `S`, it creates the data directory if it does not exist and writes
`config.json` and the starting world into it.

### How the game talks to the caller

The game connects to the caller in one of two ways, chosen automatically:

- **On Linux, macOS, and the BSDs**, it reads and writes the caller through
  standard input and output. Synchronet (in its EX_STDIO mode) and Mystic hand
  the connection to the door this way, so this is the normal case. You do not
  need to set anything up for it.
- **On Windows**, it attaches directly to the caller's socket. The socket
  handle is on line 2 of `DOOR32.SYS` (a Windows "winsock" handle). This is how
  a Windows door normally works.

Serial (FOSSIL) doors are not supported. Configure your BBS for a socket or
stdio door.

Note: a sysop has run the Windows socket path with a live caller and reports
that it works. If you run the game as a Windows door, please still report how it
goes.

### Character set

The game sends CP437 by default — the character set traditional BBS terminals
expect. A door does not auto-detect the character set (only `-local` does), so it
uses that default unless you pass `-utf8` or `-cp437`. Set the option that fits
your board (non-English languages need `-utf8`).

If your board serves callers on different character sets, and your BBS software
can tell a program the caller's character set (for example, by setting a shell
variable when it invokes a door), you can wrap the door in a small script that
passes `-utf8` or `-cp437` to match each caller.

See the [Character Set guide](https://andy5995.github.io/immortal-barons/charset/)
for the options and how to test each one.

## Daily maintenance

Maintenance advances the game one day for every day that has passed. It lets the
AI barons take their turns, runs pirate raids, and refreshes each player's turns.

**You usually do not need to schedule this.** Maintenance runs on its own the
first time a player logs in on a new day. If several days passed with no play,
the next login catches up all of them at once. A board that is not played every
day needs no nightly event.

Run it by hand only if you want the game to move forward while no one is playing:

```
immortal-barons -maint -data /path/to/data
```

This is useful for a league board, where the AI barons, pirates, and inter-BBS
packets should keep to a set schedule even on days when no local player logs in.
For a solo or local game, you can skip it.

## Starting a fresh game (reset)

There are two ways to start the game over. Both clear all empires (players
re-create their realm the next time they log in) and re-seed the AI barons on a
fresh day one. Neither picks a winner. Each one saves the old world to
`world.json.bak` in the data directory first, so you can restore it if you reset
by mistake.

**Reset and choose the settings:**

```
immortal-barons -reset -data /path/to/data
```

This opens the **Configuration Editor** starting from the built-in defaults, so
it also resets `config.json` to those defaults. Adjust any settings, then press `S`
to save and start the fresh game, or `Q` to cancel (which leaves the game
untouched). Because it writes a clean default `config.json`, you can also use
this to produce a config file to copy and reuse.

**Reset and keep your current settings:**

```
immortal-barons -reset-from-config -data /path/to/data
```

This starts the fresh game using the `config.json` you already have. It does not
open the editor and does not change your settings. Use it when you are happy with
the current settings and only want to clear the world.

### The config file is portable

All the game settings live in `config.json` in the data directory. This file can be
copied. You can back it up to another place, and you can copy it into any data
directory to reuse the same settings there. To carry your settings to a new install
or a fresh game, copy your saved `config.json` into the data directory, then run
`-reset-from-config`.

## Adding AI barons to a running game

To drop more AI opponents into a game that is already going, without resetting:

```
immortal-barons -add-ai N -data /path/to/data
```

This adds N new AI barons and exits. It leaves existing players and AI alone.
(To set the AI count for a brand-new game instead, use the Configuration Editor
via `-reset`.)

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
immortal-barons -planetary -data /path/to/data
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
immortal-barons -league-config -data /path/to/data
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
