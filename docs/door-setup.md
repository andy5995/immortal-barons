# Door Setup — Running Immortal Barons

This guide is for the BBS operator (the sysop). It explains how to set up the
game on your board. Players do not need to read this.

To connect your board to others, see [Inter-BBS Leagues](inter-bbs.md).

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
  [**DORINFO1.DEF**](https://wiki.synchro.net/ref:dorinfo1.def). The
  [Synchronet](https://www.synchro.net/) wiki keeps an
  [index of drop file formats](https://wiki.synchro.net/ref:files#drop_files).
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

In [Mystic](https://www.mysticbbs.com/)'s menu editor, add a command of type
**D3 (Exec DOOR32 program)** and set its **Data** field to the command line
below. Mystic writes the dropfile into each node's own temporary directory and
gives you `%P` for that directory (with a trailing slash), so the same line
works for every node:

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

### Example: Synchronet

The release archive carries an `install-xtrn.ini`, so Synchronet can set the
door up for you. Unpack the archive under Synchronet's `xtrn` directory and
point the installer at it:

```
jsexec install-xtrn /sbbs/xtrn/imb
```

It adds the door itself and offers a nightly maintenance event, which you can
decline. The start-up directory is taken from where the file sits, so nothing
in the command line depends on a path you have to type. Two things it cannot
do for you: run `-set-dropfile` and create the game with `-reset`, as in
"First-time setup" above. The door refuses to start until both are done.

The rest of this section is the same setup entered by hand.

Add an external program in `scfg` → External Programs → Online Programs, and
set its **Start-up Directory** to the game's own directory.

Synchronet changes to the **Start-up Directory** before it runs the door, and
the game looks for `data` beside its own binary, so neither path needs to be
spelled out:

```
immortal-barons%. -dropfile %f
```

Keeping the line short this way also stays clear of Synchronet's command-line
limit, which is 100 characters and is applied silently. That relies on the
start-up directory staying set — clear it and the game looks for its data
somewhere else and reports no game found.

`%f` is the drop file Synchronet has just written for this caller: the full
path, in whichever directory and letter case it used. Nothing in the line
depends on the drop file type or on how you answered **Lowercase Filename**,
which matters on Linux, where file names are case-sensitive. `%.` appends the
platform's executable extension, so the same line works on Windows.

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

## Bulletins

Players read your bulletins from **(8) Game Bulletins** on the opening menu.
Each bulletin is one file, in a folder under the data directory:

```
data/bull/local     your own
data/bull/league    the League Coordinator's, if this board is in a league
```

Create the folders if they are not there yet. A bulletin is a `.txt` file or an
ANSI `.ans` file, up to 64 KB.

The **first line of the file is the title** the menu shows, so put the title on
line one and the text below it. Files are listed in file-name order. Name them
`10-news.txt`, `20-rules.txt` and so on to set the order yourself.

The menu shows the league's bulletins first, under the heading "Galactic", then
your own. It reads the folders each time a player opens it, so a file you copy
in is there for the next player.

When you add or change a bulletin, the game puts a line in the day's news with
its title.

In a league, `data/bull/league` belongs to the League Coordinator. The game
fills that folder for you on each `-planetary` run, and replaces whatever is in
it, so put your own bulletins in `data/bull/local`. See
[League bulletins](inter-bbs.md#league-bulletins).

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
machine does not drag the old board's directories along with it. That file also
carries the one rule the league does not set for you: `Lottery yes` or
`Lottery no` decides whether this board offers the Queen's lottery, and it
defaults to yes.

## Adding AI barons to a running game

To drop more AI opponents into a game that is already going, without resetting:

```
immortal-barons -add-ai N -data /path/to/data
```

This adds N new AI barons and exits. It leaves existing players and AI alone.
(To set the AI count for a brand-new game instead, use the Configuration Editor
via `-reset`.) A board in a league game has no AI barons, and this command
adds none.

## Editing a player

```
immortal-barons -players -data /path/to/data
```

This is the original's `VIEW` command. It lists every realm a caller owns. Pick
one by its Id letter — the same letter the game uses for it everywhere else —
then choose **D**elete realm, **P**layer name, **R**ealm name, or **Q**uit.

The player name is the one that matters most. The game finds a realm by the
player's BBS account name. If a player renames their account on your board, the
game no longer knows them at their next login and offers them a new realm. Their
old realm sits unplayed until the game removes it for being idle. Set the player
name here to the new one and the realm stays theirs.

Deleting asks you to confirm and cannot be undone. The caller may build a fresh
realm the next time they log in.

You can run this while the board is up. Each edit takes the same lock a caller's
turn takes, and only for as long as the edit. The questions are asked outside
the lock, so nobody is kept waiting while you decide.
