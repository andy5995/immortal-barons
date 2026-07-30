# Command Reference

This page lists every command-line option for Immortal Barons in one place. It
covers the game program (`immortal-barons`) and the web server
(`immortal-barons-web`).

Run `immortal-barons -help` to see the same options in your terminal. This page
and the `-help` output use the same groups.

## The game program: `immortal-barons`

Run it like this:

```
immortal-barons [options]
```

With no options, it runs as a BBS door: it looks for the drop file your BBS
writes (the format you chose with `-set-dropfile`) in the current folder and
plays over the BBS connection. The options below change that.

### Play

These options are for playing the game.

- **`-local`** — Play in your own terminal instead of as a BBS door. Good for
  testing or for a single player on the same machine.
- **`-name NAME`** — Set your player name. Only used with `-local`. Without it,
  the game uses your system login name.
- **`-dropfile PATH`** — Path to the BBS drop file. Your BBS software writes this
  file and tells the door where it is. The format is the one you set with
  `-set-dropfile` (see the door setup guide for the supported formats).
- **`-data DIR`** — The folder that holds the game data. The default is `./data`,
  which is **relative to the directory you run the command from**, not to where
  the program file is. Run the game from a different folder and it looks for
  `data` there. To avoid surprises, give a full path (for example
  `-data /home/bbs/immortal-barons/data`). All modes use this option to find the
  shared world.

### Character set (output)

These options choose how the game draws its screens. See
[Character Set](charset.md) for the full explanation.

- **`-utf8`** — Force UTF-8 output. Needed for non-English languages. With
  `-local`, the game already detects UTF-8 from your locale, so you rarely need
  this.
- **`-cp437`** — Force CP437 output (the classic BBS character set). This is the
  door default. Use it to override the `-local` locale detection.

### Sysop and game admin

These options are for the person who runs the game. Most of them do one job and
then exit.

- **`-set-dropfile`** — Choose which drop file format your BBS writes, save it,
  then exit. Run this once when you set the door up. The choice is stored in
  `door.json`, apart from the game settings, so a reset never changes it. Until
  it is set, the door refuses to start. See the door setup guide for the
  supported formats.
- **`-reset`** — Start a new game. First it opens the settings editor so you can
  change the rules, then it clears all empires and rebuilds the world. The old
  world is saved first. This also rewrites `config.json`.
- **`-reset-from-config`** — Start a new game using the current `config.json`,
  without opening the editor. It clears all empires and rebuilds the world. The
  old world is saved first.
- **`-add-ai N`** — Add N computer barons to the running game, then exit.
- **`-maint`** — Run the daily maintenance step, then exit. Run this once a day
  (for example, from a nightly scheduled task).
- **`-dump`** — Print the game world as JSON, then exit. The output is the world
  *after* the game loads it (old saves are migrated and missing fields filled
  in), so it can differ from the raw `world.json` file. Useful for scripts and
  for checking game balance.
- **`-spectate N`** — Play the game forward N days with no human players, then
  exit, printing a summary of each day and a final table of every realm. It does
  not reset the game first, so pair it with `-reset-from-config` for a clean run:

    ```
    immortal-barons -data ./sandbox -reset-from-config
    immortal-barons -data ./sandbox -spectate 30
    ```

    This is a balance-checking tool, not something a board needs to run. It
    exists so the computer barons can be watched playing against each other, to
    see whether they expand sensibly, whether wars happen, and whether the game
    collapses to one realm too quickly. **It plays real turns and saves the
    result**, so only use it on a game you are willing to advance — never on a
    live board.

### Inter-BBS

These options are for games that link several BBSes together (a "league"). See
[Door Setup](door-setup.md) for how inter-BBS play works.

- **`-planetary`** — Run the inter-BBS step, then exit: read incoming packets,
  run the group attacks, and write outgoing packets.
- **`-league-config`** — Send this board's league settings to the whole league,
  then exit. Only the league coordinator (node #1) uses this.
- **`-export FILE`** — Write this board's score packet to FILE, then exit.
- **`-import FILE`** — Read a score packet from FILE, then exit.

### Info

- **`-version`** — Print the version, then exit.
- **`-help`** — Print the grouped list of options, then exit.

## The web server: `immortal-barons-web`

The web server lets people play in a browser. It always uses UTF-8. See
[Web Server](webserver.md) for setup.

```
immortal-barons-web [options]
```

- **`-addr ADDRESS`** — The address to listen on. The default is `:8080`.
- **`-data DIR`** — The folder that holds the game data. The default is `./data`,
  relative to the directory you run the command from (see the note under the game
  program's `-data` above). Point it at the same folder the game program uses to
  share one world.
