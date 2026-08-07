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

- Run `-set-dropfile` once to tell the game which drop file your BBS writes.
  The supported formats are **DOOR32.SYS**,
  [**DOOR.SYS**](https://wiki.synchro.net/ref:door.sys),
  [**PCBOARD.SYS**](https://wiki.synchro.net/ref:pcboard.sys), and
  [**DORINFO1.DEF**](https://wiki.synchro.net/ref:dorinfo1.def). The Synchronet
  wiki keeps an [index of drop file formats](https://wiki.synchro.net/ref:files#drop_files).
  DOOR32.SYS is the one most modern BBS software writes and the best choice when
  you have it. The setting is saved in `door.json` (separate from the game
  settings, so it is never changed by a reset). Until you set it, the door
  refuses to start. Only the door needs it: the web front-end and `-local` play
  do not read a drop file.
  - If you are updating from an earlier version, run `-set-dropfile` once after
    you install the update. Earlier versions did not have this setting.
- Point the game at the drop file with `-dropfile`, or let it search the current
  directory for the configured format.
- The caller's handle from the drop file becomes the name of their realm.

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

Maintenance moves the game forward one day. It lets the AI barons take their
turns, runs pirate raids, and refreshes each player's turns.

**You usually do not need to schedule this.** Maintenance runs on its own the
first time a player logs in on a new day. A board that is not played every day
needs no nightly event; days with no play are skipped, and the game picks up
where it left off. If you would rather the game keep moving on quiet days, run
it from a nightly event.

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
via `-reset`.) A board in a league game has no AI barons, and this command
adds none.

## Inter-BBS (league) play

A league is a group of boards whose players compete against each other.

Set the board up with `-ibbs-reset` instead of `-reset`:

```
immortal-barons -ibbs-reset -data /path/to/data
```

It works like `-reset` — the settings editor, then a fresh world — but the
editor also asks the league settings, which a stand-alone board is never shown.
Set these on the **Caps & Node** page:

- **Board ID** — a short unique name for your board (your "planet").
- **Inbound Dir** — the directory where packets from other boards arrive.
- **Outbound Dir** — the directory where the game writes packets for other
  boards.

The two directories are relative to your data directory, so the defaults
`inbound` and `outbound` need no editing on most boards. Give a full path
instead if your transport drops packets somewhere else. A reset creates them
for you, and warns if either still holds packets from the game it just
cleared — the next `-planetary` run would apply those to the new game, so
delete them first.

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

All six lines must have something on them. A blank line is what separates one
board from the next, so a board written with an empty field is read as a broken
entry and skipped. Only the first three matter to the game; put anything you
like in the last three.

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

## Joining a league (member boards)

A member board's sysop does not set the league rules — the Coordinator does, and
they arrive over the wire. What a member has to do is get on the wire, then
create the game.

**Ask the League Coordinator for four things:**

| What | Why |
|---|---|
| Your board's name and node number | They must match the node list exactly. The name is your board's Board ID. |
| The `ibnodes.dat` file | The league roster. Put it in your data directory. |
| The Coordinator's key | One line, from their `-gen-coord-key`. Without it your board refuses league orders. |
| The mailer details for your uplink | Their address, host and port, and the session password. This is your BBS's business, not the game's. |

**All four arrive by hand — email, a message on their board, a download.** None
of it can come over the league's own link, because none of the link exists yet:
the mailer details are what build it, and the key is what proves a packet came
from the Coordinator, so a key that arrived in a packet would prove nothing. It
is a public key, so there is no harm in it travelling in the clear; it lets your
board *check* the Coordinator's orders, not issue them.

After that first hand-off the roster keeps itself current: the Coordinator
broadcasts it, and a board updates its own copy when a signed roster arrives.
The key is a one-time exchange, unless the league changes Coordinator.

**Then, in this order:**

1. **Set up the mail link and test it.** Your BBS and your mailer, not the game.
   Do not go on until a poll connects.
2. **Put `ibnodes.dat` in your data directory**, and record the Coordinator's
   key:

    ```
    immortal-barons -coord-key THEIR_KEY -data /path/to/data
    ```

3. **Create the game.** One command, no settings editor — the rules are not
   yours to choose:

    ```
    immortal-barons -ibbs-reset -board-id "Your Board" -inbound /path/to/ftn/inbound -outbound /path/to/filebox -data /path/to/data
    ```

    Quote the board name if it has spaces. `-inbound` is where your mailer
    delivers incoming files; `-outbound` is the filebox for your uplink. Both
    may be left out, in which case the game uses `inbound` and `outbound` inside
    its data directory and you move the files yourself.

4. **Run the inter-BBS step once:**

    ```
    immortal-barons -planetary -data /path/to/data
    ```

    Until the Coordinator's settings packet arrives, your board is running
    default rules. Turns per day and the rest will change when it lands, so do
    not be alarmed at the first `Game Setup` screen, and do not let players on
    before it.

5. **Put `-planetary` on a schedule**, and let your callers in.

Step 3 does not need the Coordinator's rules to have arrived, so 2 and 3 can
happen in either order — but the key must be recorded before step 4, or the
settings packet is refused as unsigned.

## Example: a two-board league, step by step

This is a worked example of setting up a league from nothing. It uses two
boards on one machine, both running **Mystic BBS**, with Mystic's own binkp
mailer moving the packets. The addresses are made up: nothing here touches
FidoNet or any other real network.

Another BBS package, or another mailer, changes the details but not the shape.
The game only reads and writes files in two directories; everything else is
your BBS and your mailer.

The two boards in the example:

| | first board | second board |
|---|---|---|
| Install | `~/a-mystic` | `~/b-mystic` |
| Board ID (planet) | `AlphaBBS` | `BravoBBS` |
| Node number | 1 (League Coordinator) | 2 |
| Network address | `1:1/1` | `1:1/2` |
| binkp port | 24554 | 24555 |

### Step 1 — choose the names and addresses

Settle these before touching anything, because they appear in three places: the
game's Board ID, the game's node list, and your BBS's mail configuration. The
Board ID is fixed when you create the game, and it must match the node list
exactly.

Node 1 is the League Coordinator. The network addresses belong to your mailer,
not to the game — the game never opens a connection, so it never uses them. Two
boards on one machine need different binkp ports; 24554 is the standard one.

### Step 2 — give each board its own network address

In Mystic: `mystic -cfg`, then **Networking → Echomail Addresses**. Add an
address on each board:

| | first board | second board |
|---|---|---|
| Zone / Net / Node / Point | 1 / 1 / 1 / 0 | 1 / 1 / 2 / 0 |
| Domain | `iblocal` | `iblocal` |
| Primary | Yes | Yes |

The domain is the name of your network and must be the same on both boards. Add
your address on an empty row: the first row is a reserved placeholder, and
Mystic only lets you edit its description.

### Step 3 — point each board at the other

In Mystic: **Networking → Echomail Nodes**. The list starts empty; press `/`
for the command list, then **Insert**. Each board gets one entry, for the board
it connects to:

| Field | on the first board | on the second board |
|---|---|---|
| Address | `1:1/2` | `1:1/1` |
| Domain | `iblocal` | `iblocal` |
| Session type | BinkP | BinkP |
| binkp hostname | `127.0.0.1:24555` | `127.0.0.1:24554` |
| binkp password | the same on both | the same on both |
| Use filebox | Yes | Yes |
| Active | Yes | Yes |

Mystic spreads these over several pages: the address, domain and session type
are on page 1, and the hostname and password are on the BINKP page, which takes
the host and port together as `host:port`. Watch the title bar — it shows the
address and domain you have entered so far, so `0:0/0@` means page 1 is still
empty.

The port is the one this board **dials**, so it is the other board's port. The
password must match on both sides. Leave the archive, export, crash and size
settings alone: no echomail is flowing here, only files.

Set **Use Filebox** to Yes and let Mystic generate the default path. A filebox
is a directory whose contents your BBS hands to that node at the next session,
and it is how the game's packets travel. Mystic names it after the network and
the node's address, so the first board gets
`~/a-mystic/filebox/iblocal_z1n1n2` — its outbox for the second board — and the
second board gets `~/b-mystic/filebox/iblocal_z1n1n1`. Note both paths: a later
step points the game's outbound directory at them.

A larger league does not mean more of these. Boards route through the League
Coordinator, so a member board configures one link — its uplink — and mail for
every other board goes out over it.

### Step 4 — let each board answer

So far each board knows how to dial the other. Now each needs to listen. In
Mystic: **Servers → BINKP**.

| | first board | second board |
|---|---|---|
| Server | active | active |
| Port | 24554 | 24555 |

A board listens on its **own** port, which is the port the other board dials.
Then run Mystic's server on both boards (`./mis server`) and leave them
running. Each writes to `logs/mis.log` in its own install; check there that the
binkp server bound its port and did not report the address as already in use.

### Step 5 — prove the link

From the first board, poll the second:

```
./mis poll 1:1/2
```

Then the other way round, from the second board:

```
./mis poll 1:1/1
```

**Test both directions.** They use different settings, so one working says
nothing about the other.

`./mis poll LIST` shows the nodes it knows about. A session that connects and
finishes with nothing to transfer is the result you want: it means the address,
the port and the password all agree.

Read the log rather than the last line: "Polled 1 systems" is printed whether
the session worked or not. A wrong port reports **"Authorization failed"**,
which sounds like a password problem but is not — nothing answered, so there was
nothing to authorize. The line above it names the port it dialled, and that is
the one to check against the other board's binkp server.
