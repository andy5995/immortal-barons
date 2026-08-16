# Door Setup — Running Immortal Barons

This guide is for the BBS operator (the sysop). It explains how to set up the
game on your board, and how to join an inter-BBS league. Players do not need to
read this.

## What you need

The game is one program: `immortal-barons`. It runs as a native door under BBS
software on any platform Go supports.

The game keeps all its files in one data directory (default `./data`). Point
the `-data` option at it.

**Keep that directory on a local disk, and run every node from the same
computer.** Several nodes can play at once, because the game locks the world
file for the moment of each change and the nodes take turns. That lock works
inside one computer only. If the directory sits on a network share, or nodes
run on two computers, two of them can save over each other. Nothing reports an
error — a turn simply goes missing.

Run `immortal-barons -help` to see all the command-line options. The
[Command Reference](command-reference.md) explains every option in one place.

## First-time setup

You must create the game once before anyone can play. Until you do, the game
reports "no game found" and refuses to start, so a caller never lands in an
empty, unplayable world.

```
immortal-barons -reset -data /path/to/data
```

This opens the **Configuration Editor**, starting from the built-in defaults: a
menu of every game setting (turns per day, protection turns, land and market
settings, interest and investment rates, tax and region limits, costs and attack
settings, number of AI barons, game length, and more). Change what you like,
then press `S` to save `config.json` and seed the world, or `Q` to cancel.

To skip the editor and use the current `config.json` as it stands, run
`-reset-from-config` instead.

These same two commands restart the game later on — see "Starting a fresh game".

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
  refuses to start. Only the door needs it: `-local` play does not read a
  drop file.
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

Mystic writes the file name in lower case (`door32.sys`), which matters on Linux
because file names there are case-sensitive. Keep the game's data directory
separate from the BBS's own files.

Before the first caller connects, seed the game as in "First-time setup" above,
pointing it at this board's data directory:

```
/path/to/immortal-barons -reset -data /path/to/game-data
```

Saving with `S` creates the data directory if it does not exist.

### How the game talks to the caller

The game connects to the caller in one of two ways, chosen automatically on
every platform. There is nothing to configure.

- If your BBS hands the door a **live standard input** — a terminal — the game
  reads and writes the caller through it. Mystic does this, and so does
  Synchronet when the door's I/O interception is turned on.
- Otherwise, if the drop file names a **socket**, the game attaches to that
  socket. A native Synchronet door with I/O interception off is set up this way:
  the connection arrives as a numbered handle (line 2 of `DOOR32.SYS`) and
  standard input is connected to nothing.

A live standard input wins: a BBS that hands one over means it to be used. The
drop file alone cannot decide this, because Mystic names a socket **and** gives
a terminal, so a door that simply believed the drop file would abandon a
connection that works.

**If a caller is dropped the instant the door starts**, check
`data/ib-door.log`. The `session i/o backend=` line says which path was taken.
A board that names a socket, has no terminal, and still fails is reporting a
socket the door could not attach to — worth sending to the issue tracker with
those two log lines.

Serial (FOSSIL) doors are not supported. Configure your BBS for a socket or
stdio door.

A sysop has run the Windows socket path with a live caller and reports that it
works. If you run the game as a Windows door, please still report how it goes.

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
first time a player logs in on a new day. Days with no play are skipped, and the
game picks up where it left off.

```
immortal-barons -maint -data /path/to/data
```

Run that from a nightly event if you want the game to keep moving on quiet days.
It matters most on a league board, where the AI barons, pirates and inter-BBS
packets should keep to a schedule even when no local player logs in.

## Starting a fresh game (reset)

The two commands that create the game also restart it:

- **`-reset`** opens the Configuration Editor from the built-in defaults, so it
  resets `config.json` to those defaults as well. `Q` cancels and leaves the
  game untouched. Because it writes a clean default config, this is also how you
  produce a `config.json` to copy and reuse.
- **`-reset-from-config`** keeps the `config.json` you already have, opens no
  editor, and only clears the world.

Both clear all empires (players re-create their realm the next time they log in)
and re-seed the AI barons on a fresh day one. Neither picks a winner. Each saves
the old world to `world.json.bak` in the data directory first, so you can
restore it if you reset by mistake.

### The config file is portable

The game's rules live in `config.json` in the data directory, and that file can be
copied. Back it up somewhere, or copy it into any data directory to reuse the same
settings there. To carry your settings to a new install or a fresh game, copy your
saved `config.json` into the data directory, then run `-reset-from-config`.

It holds the rules only. Anything naming this particular board — its name in a
league, its packet directories — is in `bbs.cfg`, so a config copied to another
machine does not drag the old board's directories along with it.

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

Two different roles are called "coordinator", and the rest of this guide keeps
them apart:

- **League Coordinator** — the sysop of board number 1. This person keeps the
  node list, hands it out to the other boards, and sets the league's rules.
- **BBS Coordinator** — a *player* your board elects. Players vote in the
  System menu, and the player with the most votes gets the Coordinator menu.
  Votes can change at any time.

Set the board up with `-ibbs-reset` instead of `-reset`:

```
immortal-barons -ibbs-reset -data /path/to/data
```

It works like `-reset` — the settings editor, then a fresh world — but the
editor also asks the league settings, which a stand-alone board is never shown.
Set these on the **Caps & Node** page:

- **Board ID** — a short unique name for your board (your "planet").
- **League Number** — the number your Coordinator picked for this league, 1 to
  999. Leave it at 0 if they have not given you one. It matters when one board
  plays in two leagues that share an inbound directory: each game ignores the
  other's packets, and marks its own files `L042-`. **Both leagues need a
  non-zero number** for that to work. A packet is only ignored when the two
  numbers are set and differ, so a league still on 0 reads the other league's
  packets and has its own read in turn.
- **League Name** — what the league calls itself, shown to players on Game
  Setup. The Coordinator sets it and it reaches every board with the rest of the
  ruleset, so you can leave it blank. Nothing routes by it; the League Number
  still does all the matching.
- **Inbound Dir** — the directory where packets from other boards arrive. This
  is usually your mailer's inbound directory, where it puts every file it
  receives.
- **Outbound Dir** — the directory where the game writes packets for other
  boards. Pick the directory whose **whole contents your mailer sends** to that
  link. Mailers often call this a *file box*.

  **It is usually not your mailer's main outbound directory.** That one holds
  the mailer's own queue: it sends the files a control file names, and it never
  looks for anything else. A game packet left there is not named anywhere, so it
  stays where it is and nothing reports an error. Your mailer's own
  documentation will say which directory it sends whole.

  To check it: run `-planetary`, look for a `.brp` file in the directory, then
  poll the other board. If the file goes, the directory is right. If it stays,
  it is a queue and you need the other one.

Board ID, League Number and the two directories are written to **`bbs.cfg`** in
your data directory, a plain text file you can edit instead of opening the
editor. See below. The League Name is not among them — it belongs to the league
rather than to your board, so it travels in the Coordinator's broadcast.

The two directories are relative to your data directory, so the defaults
`inbound` and `outbound` need no editing on most boards. Give a full path
instead if your transport drops packets somewhere else.

A reset creates them for you, and warns if either still holds packets from the
game it just cleared — the next `-planetary` run would apply those to the new
game, so delete them first.

On Windows a full path means one with a drive letter (`C:\bbs\inbound`) or a
UNC share (`\\server\bbs\inbound`). A path that only starts with a backslash,
like `\bbs\inbound`, is not a full path, and is read as being inside your data
directory.

Ask your League Coordinator for the node list and add it to your data directory
as **`ibnodes.dat`** (see the format below).

## Your board's own settings: `bbs.cfg`

Everything that describes *this* board rather than the game lives in `bbs.cfg`
in your data directory. It is plain text, one setting per line, and you can edit
it in anything:

```
BoardID       Avalon
LeagueNumber  900
Inbound       /home/bbs/ftn/in
Outbound      /home/bbs/filebox/uplink
```

Lines starting with `#` or `;` are comments, and the game writes a commented copy
for you. Keywords are matched whatever their capitalisation.

These settings sit apart from `config.json` because `config.json` holds the
league's rules, and those are overwritten when the Coordinator's settings packet
arrives. Nothing outside your board ever changes `bbs.cfg`.

A board that forwards packets for its neighbours adds a line per neighbour —
see "Routing" below.

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
3. Your transport carries each file from your outbound directory to the inbound
   directory of the board it is meant for (over FidoNet, a sync tool, scp, a
   shared mount — whatever you use).
4. The next `-planetary` run on that board reads and applies those files.

In a small league every board links to every other one, and your transport
copies each packet to all of them. That is the default, and nothing below
changes it until your Coordinator says otherwise. A large league routes instead
— see "Routing" below.

Run it as often as you like. More often means shorter travel times between
planets. The in-game "Travel Times" screen shows players how recently packets
have arrived, so they know how fast operations move.

### If you are coming from Barren Realms Elite

BRE hands each packet to your mailer. It writes the packet to its own
`\OUTBOUND` directory, then drops a `.msg` wrapper in your netmail directory so
the mailer knows to attach the file and send it. Four of `BBS.CFG`'s seven lines
serve that wrapper: the sysop name and node address that go inside it, the
netmail directory it is written to, and which mailer's flavour to use.

Immortal Barons normally stops one step earlier. It writes the packet and
leaves it there, so the game itself has no wrapper, netmail directory, or mailer
setting, and `bbs.cfg` has no line for any of the four. Watch for `.brp` files
in your outbound directory; that is the equivalent of seeing the `.msg` appear.

If your transport wants the original-style handoff, the separate `barons-ftn`
helper creates it. See [Optional FTN handoff](#optional-ftn-handoff) below.

The gain is that the game knows nothing about mail, which means:

- **A league needs no mailer at all.** Two boards on one machine sharing a
  directory is a working league, and so is a pair of boards syncing a folder
  between them.
- **Any transport works**, including ones written long after BRE was. You are
  not held to the four mailers BRE knows about, and there is no once-a-day limit
  for choosing the wrong one.
- **The wrapper is optional**: boards using another transport keep `.msg` files
  and mailer-specific settings entirely out of the path.

What you give up is that BRE arranged delivery for you. Here you point your own
transport at the outbound directory — a file box entry if you already run a
mailer, or a `cron` line if you do not.

The rest of the mapping:

| Barren Realms Elite | Immortal Barons |
|---|---|
| `BBS.CFG`, seven lines by position | `bbs.cfg`, one keyword per line |
| Line 2, BBS name | `BoardID` |
| Line 4, incoming files | `Inbound` |
| Line 6, league number | `LeagueNumber` |
| Lines 1, 3, 5, 7 | optional `barons-ftn` plus `ftn.cfg` |
| `\OUTBOUND`, fixed | `Outbound`, and you choose the path |
| `ROUTE.CFG` | the roster's `HOST` entries, plus `Link` lines |
| `BRNODES.DAT` | `ibnodes.dat` |
| `BRE PLANETARY` | `immortal-barons -planetary` |

The keywords are there because a file read by position gives no warning when a
line is missing: every value below the gap moves up one and is read as the wrong
setting.

## The node list: `ibnodes.dat`

The node list names every board in the league. It uses the same simple layout
as the original BRE `BRNODES.DAT`. Each board is six lines, and a blank line
separates boards:

```
1
Avalon
1:363/277
Orlando
FL
USA

2
Pier 7
1:106/477
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

A board may also carry a **seventh line**: its packet-signing key, described
under "Signing packets" below. The line is optional, and a roster written
without it is read exactly as before, so keys can be added to a league that is
already running, one board at a time.

```
2
Pier 7
106/477
Houston
TX
USA
4e1b9c07a3f25d81c6b40e9fa27d3358e1c0b7429dd6a85f3b1e64c0927af8d3
```

## Signing packets

A packet names the board it came from, but that name is only text in a file.
Signing lets the other boards check it.

Each board creates its own key once:

```
immortal-barons -gen-board-key -data /path/to/data
```

This writes the private half to `board.key` in your data directory and prints
the public half on a line of its own. Keep `board.key` secret: anyone holding it
can send packets as your board.

Send that key to your League Coordinator. They add it as the seventh line of
your board's roster entry, with the key on it and nothing else, then broadcast
the roster to everyone. A seventh line holding anything but the 64-character key
is treated as no key at all, which leaves that board unchecked rather than
reporting an error.

Once the roster carries a key for a board, every packet claiming to come from it
is checked, and one that does not match is refused with a news line naming the
board. Until then, packets from that board are applied unchecked. That is where
every league starts, and it is why adding keys is worth doing.

Do not create a second key on a board that already has one. Every packet it
sends will fail on the other boards until the Coordinator publishes the
replacement.

This key answers "which board sent this packet". The Coordinator's key, under
"Joining a league" below, answers a different question: "is this really a league
order". A league uses both.

## Routing: one link instead of nineteen

A league of twenty boards where every board links to every other one costs each
sysop nineteen links to configure, and one board joining costs all twenty an
edit. So the Coordinator can arrange the league as a tree instead, and every
board works out from the roster which neighbour to hand a packet to.

The Coordinator writes the tree into the node list, on each board's **first
line**: the board's own number, the word `HOST`, and the numbers it forwards
for. Board 2 collecting boards 3, 5, 7 and 8 begins its entry with:

```
2 HOST 3 5 7 8
```

For this shape —

```
                    1
              /           \
             2             4
          / | | \        / | \
         3  5 7  8      6  9  11
                              |
                             10
```

— the first lines across the whole roster are `1 HOST 2 4`, `2 HOST 3 5 7 8`,
`4 HOST 6 9 11`, `11 HOST 10`, and a bare number for everyone else. A board that
hosts nobody carries no routing information at all.

Board 3 then links only to board 2. Everything it sends, wherever it is
addressed, goes to board 2, and board 2 passes it on. When a board joins, the
Coordinator edits one roster and broadcasts it, and only the new board's uplink
touches its own setup.

Check what your board makes of the roster it has:

```
immortal-barons -league-routes -data /path/to/data
```

It prints every planet, the board your packets for it are handed to, and the
directory they are written in.

### A board that hosts others

A board forwarding for its neighbours has a separate link to each of them, and a
mailer usually wants each link's files in its own directory. Add a `Link` line
to `bbs.cfg` for each, giving the neighbour's node number and the directory:

```
Link 3  /home/bbs/filebox/league_node3
Link 5  /home/bbs/filebox/league_node5
```

Anything with no `Link` line of its own goes to **Outbound**, which is what a
board's link to its own uplink should be. A board that hosts nobody needs none
of this.

### Overriding the Coordinator's routing

A board that wants to send some traffic its own way can put an `ibroute.cfg`
file in its data directory. Each line is `ROUTE`, the board to send to, and the
board to send it through; `*` stands for every board, and sending a board
through itself restores a direct link. Lines starting with `;` are comments.

```
; everything through board 8, except board 5, which we reach directly
ROUTE * 8
ROUTE 5 5
```

The file overrides what the roster says. Later lines win over earlier ones, so
write the general rule first. A league whose Coordinator keeps the routing in the
roster needs no such file on any board.

## Optional FTN handoff

`barons-ftn` turns the transport-neutral `.brp` files into ordinary FTN
file-attach netmail. The game does not run it automatically. A typical event or
door-exit chain is:

```
immortal-barons -planetary -data /path/to/data
barons-ftn -data /path/to/data
```

Create `ftn.cfg` in that data directory:

```
NetmailDir /sbbs/fido/netmail
Binkley    No
```

- **NetmailDir** is the directory where your BBS or mailer watches for Type-2
  `.msg` netmail. A relative path is read relative to the game data directory.
- **Binkley** is `Yes` when the mailer uses Binkley-style file attaches,
  including BinkIT, the Binkley-style mailer shipped with Synchronet, and `No`
  for a non-Binkley mailer. Synchronet can be used with several mailers, so the
  BBS package itself does not decide this switch. Omitting the setting is the
  same as `No`.
  Binkley-style handling gets a `^` before the attached path in the subject;
  non-Binkley handling gets a `FLAGS KFS` control line. Both get the private,
  local, file-attach, and kill-sent header attributes.

The helper gets this board's address and every destination address from
`ibnodes.dat`. Use complete `zone:net/node` addresses there; a point may add
`.point`. The helper prefers the packet's stable destination node number, uses
the board name for packets from older versions, and applies `ibroute.cfg` and
the roster's `HOST` tree to choose the next hop. For the common arrangement
where every member sends through node 1:

```
; ibroute.cfg
ROUTE * 1
```

With a route configured, `-planetary` already writes one signed, addressed
packet per board. In an unrouted mesh it writes one unaddressed broadcast
instead; `barons-ftn` gives every other board its own attachment pathname and
`.msg`. It sends those copies directly to each board. Turning the already-signed
broadcast into routed, addressed packets here would change its signed bytes.

For each configured `Outbound` or `Link` directory, the helper moves a claimed
packet into its `fido` child and puts that absolute pathname in the `.msg`
subject. The pathname must fit the Type-2 subject field (71 bytes, or 70 with
the Binkley `^`); choose a short outbound path if necessary. The FTN software
removes the claimed file after sending it.

The helper takes the same cross-platform local file lock as the game before it
scans, so two door nodes cannot move the same source together or race a game
write. The game holds that lock while it signs and writes the outbox, so a
packet visible to the helper is complete and, when board signing is configured,
already signed. The move is the ownership claim; only the process that moves a
packet creates its `.msg` or broadcast set. A malformed packet or a
message-creation failure is moved back to the outbound directory for a later
run. This uses ordinary rename, exclusive file creation, and real copies—hard-
link support is not required.

## League-wide rules (Coordinator only)

The League Coordinator sets the rules that must match across the whole league.
These are all the fields marked with a star in the Configuration Editor: turns
per day, protection turns, game length, land market and daily land, interest
and investment rates, tax and region and player limits, buy-military mode, the
cost, damage, and reward levels, and the league policies — whether barons on one
board may attack each other, whether such a battle scores, and whether a caller
found playing on two boards is locked out. Set them in the Coordinator's own
`config.json`, then broadcast them to every board:

```
immortal-barons -league-config -data /path/to/data
```

This writes a settings packet to your outbound directory. Each member board
adopts the settings on its next `-planetary` run. A member accepts them only
when the packet comes from node 1 *and* carries the Coordinator's signature, so
no other board can change the league rules.

### Starting a new season

The Coordinator can restart the whole league on a chosen date:

```
immortal-barons -league-reset 2026-09-01 -data /path/to/data
```

The date is `YYYY-MM-DD`. This resets the Coordinator's own board straight away
and sends a signed order that each member carries out on its next `-planetary`
run. Every board keeps what identifies it in the league — its roster, its keys
and its packet history — and starts a fresh world, so nobody re-does the setup.

Nothing schedules this. Run it when you decide the season is over.

### League reports

Three commands write a report into the data directory. None of them changes the
game; each is built from what packets have already told this board.

| Command | Writes | Shows |
|---|---|---|
| `-lastpacket` | `LASTPACKET.LST` | when a packet from each board was last processed here — how you spot a board that has gone quiet |
| `-bbsinfo` | `BBSINFO.LST` | every board, when it was last heard from, and the version it runs |
| `-playerlist` | `PLAYERLIST.LST` | every realm on every board (Coordinator only) |

## Joining a league (member boards)

A member board's sysop does not set the league rules — the Coordinator does, and
they arrive over the wire. What a member has to do is get on the wire, then
create the game.

**Tell the League Coordinator what you want your board called** — that is your
choice, not theirs. They add it to the roster and give it a node number.

**Then you need five things back from them:**

| What | Why |
|---|---|
| Your node number, and your name exactly as they recorded it | The game matches boards by name, so the spelling and spacing in the roster are what your Board ID has to be. |
| The `ibnodes.dat` file | The league roster. Put it in your data directory. |
| The Coordinator's key | One line, from their `-gen-coord-key`. Without it your board refuses league orders. |
| The league number, if they use one | A number from 1 to 999. It only matters if your board plays in two leagues that share an inbound directory, but ask anyway — a packet stamped with the wrong one is ignored. |
| The mailer details for your uplink | Their address, host and port, and the session password. This is your BBS's business, not the game's. |

**All five arrive by hand — email, a message on their board, a download.** None
of it can come over the league's own link, because that link does not exist yet:
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

    This writes `bbs.cfg`. If your Coordinator gave you a league number, add it
    there — the command has no flag for it:

    ```
    LeagueNumber 900
    ```

    **Converting a board that already plays BRE?** Point the command at your old
    `BBS.CFG` instead and it takes the board name, incoming-files directory and
    league number from it, printing what it read:

    ```
    immortal-barons -ibbs-reset -import-bbs-cfg /path/to/bre/BBS.CFG -data /path/to/data
    ```

    It leaves behind the three lines the game has no use for — your name, your
    node address, and your mailer — and does **not** read your netmail directory
    as the outbound directory. BRE puts `.MSG` files there; the game writes
    packets, which are not the same thing.

4. **Create your board's own signing key**, and send the line it prints to the
    Coordinator:

    ```
    immortal-barons -gen-board-key -data /path/to/data
    ```

    They add it to your roster entry, so the other boards can tell your packets
    from ones that only claim to be yours. See "Signing packets" above. You can
    join a league without this and add it later.

5. **Run the inter-BBS step once:**

    ```
    immortal-barons -planetary -data /path/to/data
    ```

    Until the Coordinator's settings packet arrives, your board is running
    default rules. Turns per day and the rest will change when it lands, so do
    not be alarmed at the first `Game Setup` screen, and do not let players on
    before it.

6. **Put `-planetary` on a schedule**, and let your callers in.

Step 3 does not need the Coordinator's rules to have arrived, so 2 and 3 can
happen in either order — the keys are kept in their own files and a reset does
not disturb them. The Coordinator's key must be recorded before step 5, though,
or the settings packet is refused as unsigned.

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

### Step 6 — create the two games

The Coordinator's board sets the league rules, so it is the one that opens the
settings editor:

```
immortal-barons -ibbs-reset -data /path/to/coordinator/data
```

On the **Caps & Node** page, set the Board ID to the board's name, and the two
packet directories:

| | value |
|---|---|
| Inbound Dir | your BBS's FTN inbound, e.g. `~/a-mystic/echomail/in` |
| Outbound Dir | the filebox for the other board, from step 3 |

The member board takes no editor at all — its rules arrive from the Coordinator:

```
immortal-barons -ibbs-reset -board-id "Bravo BBS" -inbound ~/b-mystic/echomail/in -outbound ~/b-mystic/filebox/iblocal_z1n1n1 -data /path/to/member/data
```

Quote a board name that has spaces in it.

### Step 7 — the roster and the key

Write `ibnodes.dat` once and put a copy in **both** data directories. The names
must match the Board IDs exactly:

```
1
Alpha BBS
1:1/1
Local
XX
USA

2
Bravo BBS
1:1/2
Local
XX
USA
```

Then create the Coordinator's key on the first board, and record the public half
on the second:

```
immortal-barons -gen-coord-key -data /path/to/coordinator/data
immortal-barons -coord-key THE_KEY_IT_PRINTED -data /path/to/member/data
```

Without the key the member board refuses the Coordinator's orders, and the
league rules never arrive.

This example stops there, which is enough to play. To have each board sign its
own packets as well, run `-gen-board-key` on both and add the 64-character key
it prints as a seventh line in that board's roster entry. See "Signing packets"
above.

### Step 8 — the first exchange

From the Coordinator's board, broadcast the rules, carry them over, and apply
them:

```
immortal-barons -league-config -data /path/to/coordinator/data
cd ~/a-mystic && ./mis poll 1:1/2
immortal-barons -planetary -data /path/to/member/data
```

Watch the packet as it goes: it appears in the filebox, then in the other
board's inbound, then disappears as the game reads it. The member board's news
should then say **"The League Coordinator updated the league settings."** If it
says a packet "claimed to carry League Coordinator orders and was refused"
instead, the key in step 7 did not take.

### Step 9 — put it on a schedule

You should not have to run this by hand every day. Each board needs one job on
a timer, doing the two steps in order:

```
immortal-barons -planetary -data /path/to/data && cd /path/to/bbs && ./mis poll 1:1/2
```

The game step reads and writes packet files; the mail step carries them. The
game never moves a file between boards, and your mailer knows nothing about the
game, so both have to run. The order matters: polling before the game has
written its outbox sends the packets from the run before.

Run it from cron, or from your BBS's own event scheduler. How often is up to
you. Every exchange is a round trip, and the Travel Times screen in the game
reports how long your players actually wait. A league whose boards poll each hour
plays very differently from one that polls at 3am.

**You may already have some of this.** Daily maintenance runs the inter-BBS
step itself when inter-BBS play is on. Maintenance also runs on its own at the
first login of a new day, so a board with callers already reads and writes
packets once a day with nothing scheduled. What the timer adds is doing it more
often than daily, and the poll — which the game cannot do for you at all.
