# Command Reference

This page lists every command-line option for Immortal Barons in one place. It
covers the game program (`immortal-barons`) and the optional FTN transport
helper (`barons-ftn`).

Run `immortal-barons -help` to see the same options in your terminal. This page
and the `-help` output use the same groups.

## The game program: `immortal-barons`

Run it like this:

```
immortal-barons [options]
```

With no options, it runs as a BBS door: it looks for the drop file your BBS
writes (the format you chose with `-set-dropfile`) in the current folder and
plays over the BBS connection. If that format is BBSDEV.DRP, it reads the
`BBSDEV_DRP` environment variable first. The options below change that.

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

### Terminal (output)

These options choose how the game draws its screens. See
[Character Set](charset.md) for the full explanation of the first three.

- **`-utf8`** — Force UTF-8 output. Needed for non-English languages. With
  `-local`, the game already detects UTF-8 from your locale, so you rarely need
  this.
- **`-cp437`** — Force CP437 output (the classic BBS character set). This is the
  door default. Use it to override the `-local` locale detection.
- **`-ascii`** — Force plain 7-bit ASCII output, for a terminal that reads
  neither CP437 nor UTF-8. Box rules become `-`, `|` and `+`, shaded blocks
  become `#`, and an accented letter loses its accent. Nothing can be
  mis-decoded, at the cost of the artwork. Only English and German are offered
  in this mode; a language that would come out as question marks is not.
- **`-no-ansi`** — Send plain text with no color and no cursor control, as a
  terminal that cannot render escape sequences receives. Lists that normally
  use a moving highlight are numbered instead. The door does this on its own
  when the caller's BBS reports no ANSI support, so this option is for testing
  that path on a terminal that could render it.

Use only one of `-utf8`, `-cp437` and `-ascii`. `-no-ansi` is separate and
combines with any of them: the character set and whether escapes render are
two different questions about a terminal.

All four also apply to `-reset`. Because the full-screen Configuration Editor
is drawn with escape sequences and cannot be drawn without them, `-no-ansi`
opens the plain line-by-line editor instead — the same one a terminal that
cannot render ANSI gets.

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
  world is saved first. This also rewrites `config.json`. It sets up a
  stand-alone board, so the editor leaves out the league settings; use
  `-ibbs-reset` for a board that joins a league.
- **`-reset-from-config`** — Start a new game using the current `config.json`,
  without opening the editor. It clears all empires and rebuilds the world. The
  old world is saved first.
- **`-add-ai N`** — Add N computer barons to the running game, then exit.
- **`-players`** — List the players and edit one of them, then exit. This is the
  original's `VIEW` command. Pick a realm by its Id letter, then choose
  **D**elete realm, **P**layer name, **R**ealm name, or **Q**uit. Deleting asks
  you to confirm and cannot be undone. Change the player name when someone has
  renamed their account on the board: the game finds a realm by that name, so
  otherwise the game does not know them at their next login. Each edit takes the
  same lock a caller's turn takes, so it is safe to run while the board is up.
- **`-maint`** — Run the daily maintenance step, then exit. Run this once a day
  (for example, from a nightly scheduled task).

### Testing and balance

These are development tools. A board never needs them, and they advance, expose
or override game state.

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
    collapses to one realm too quickly.

    **It plays real turns and saves the result.** Two things guard against
    running it by mistake: it asks before starting, and the default answer is
    no; and it refuses outright on a game that has any human realm, since
    advancing someone else's realm is not something a warning can undo.

- **`-dupe-check on`** / **`-dupe-check off`** — Force Dupe Checking on or off
  **for this run only**. Dupe Checking is the league rule that locks a baron out
  here when they are found playing on another board in the league.

    **It changes no setting.** `config.json` is not written, and neither is
    anything else, so the run cannot leave the league rule altered behind it —
    the next command sees the saved setting again. This is what makes it a
    testing switch rather than a way to configure the game; to change the rule
    for real, use the Configuration Editor.

    It is a modifier, not a mode: it rides whatever else you asked for, so
    `immortal-barons -local -dupe-check off` plays a local turn with the rule
    lifted. `off` lets a baron the league had shut out reach the game; `on`
    applies the rule even on a board whose saved setting has it off. Either way
    the record of who was locked survives untouched.

### Inter-BBS

These options are for games that link several BBSes together (a "league"). See
[Inter-BBS Leagues](inter-bbs.md) for how league play works.

- **`-ibbs-reset`** — Start a new game as a board in a league. The same as
  `-reset`, except the settings editor also asks the league settings (board
  name, packet directories, and the interplanetary rules), and it creates the
  packet directories.
- **`-board-id NAME`**, **`-inbound DIR`**, **`-outbound DIR`** — Settings for
  `-ibbs-reset`. Giving `-board-id` skips the settings editor, so a member board
  is set up in one command. Use this when the League Coordinator sets the rules:
  they arrive in the Coordinator's next broadcast and replace whatever this board
  starts with. `-inbound` and `-outbound` default to `inbound` and `outbound`
  inside the data directory. The game does not write `bbs.cfg`: the reset ends by
  printing that file, filled in from these flags, for you to save yourself. It is
  plain text and yours alone — nothing in the game ever rewrites it.
- **`-import-bbs-cfg PATH`** — Take this board's name, incoming-files directory
  and league number from an original Barren Realms Elite `BBS.CFG`, for
  `-ibbs-reset`. Use it when converting a league you already run, so you do not
  retype what that file already says. It prints what it read. `-board-id` and
  `-inbound` override it, and naming the board in the file skips the settings
  editor just as `-board-id` does. It prints the `bbs.cfg` to save, the same as
  the flags do.
- **`-planetary`** — Run the inter-BBS step, then exit: read incoming packets,
  run the group attacks, and write outgoing packets.
- **`-full`** — Run the full cycle, then exit: read inbound packets, play a
  turn, and write outbound packets. This is the same as running `-planetary`,
  then the door (or `-local`), then `-planetary` again, but in one step. It
  requires either `-local` (with `-name` to identify the player) or a BBS drop
  file in the working directory. Use `-detailed` alongside it to see each
  packet as it is read and written.
- **`-detailed`** — Show each packet as it is read and written. This is a
  modifier, not a mode: it takes effect when used with `-full` or `-planetary`.
  Without one of those, it is ignored.
- **`-league-config`** — Send this board's league settings to the whole league,
  then exit. Only the league coordinator (node #1) uses this.
- **`-league-check`** — Check this board's league setup — the roster, the board
  name, the packet directories and the keys — and report everything that is
  wrong at once, then exit. Run it after joining a league, and whenever a
  transport run complains. It exits non-zero when anything failed, so an event
  can run it.
- **`-league-routes`** — Print which board each planet's packets are handed to,
  and the directory they are written in, then exit. Use it to check a roster the
  coordinator has just sent.
- **`-gen-coord-key`** — Create this league's coordinator key, then exit. Only
  the coordinator runs this, once. It prints a line to give every other board.
  The private half is written to `coord.key` in the data folder; keep it secret,
  and copy it if you ever hand coordinatorship to another sysop.
- **`-coord-key KEY`** — Record the coordinator's public key on this board, then
  exit. Every board in the league needs this. Without it, a board cannot check
  that league orders really came from the coordinator, and will refuse them.
- **`-gen-board-key`** — Create this board's packet-signing key, then exit.
  Every board in the league runs this once. It prints a line to send to the
  league coordinator, who puts it in the roster; once it is there, other boards
  can tell a packet from this board apart from one that only claims to be. The
  private half is written to `board.key` in the data folder — anyone holding it
  can send packets as this board.
- **`-league-reset DATE`** — Start a new season across the whole league on DATE,
  then exit. Only the coordinator uses this. It resets this board and sends a
  signed order for the other boards to reset on their next `-planetary` run.
- **`-lastpacket`** — Write `LASTPACKET.LST`, then exit: when a packet from each
  other board was last processed here. Use it to find a board that has gone
  quiet.
- **`-bbsinfo`** — Write `BBSINFO.LST`, then exit: every board, when it was last
  heard from, and the game version it runs. A board below a version the
  coordinator requires is marked.
- **`-playerlist`** — Write `PLAYERLIST.LST`, then exit: every realm on every
  board. Only the league coordinator (node #1) may write this one.

The three `.LST` reports are written into the data directory, and are built from
what packets have already told this board — none of them changes the game.

### Info

- **`-version`** — Print the version, then exit.
- **`-help`** — Print the grouped list of options, then exit.

## The FTN helper: `barons-ftn`

`barons-ftn` moves packets between the game's private directories and an FTN
mailer. Run inbound after a receive session, planetary processing next, and
outbound before the tosser/mailer sends:

```
barons-ftn --in -data /path/to/data
immortal-barons -planetary -data /path/to/data
barons-ftn --out -data /path/to/data
```

It reads `bbs.cfg`, `ibnodes.dat`, and the FTN-only `ftn.cfg`.
`--out` takes a fixed snapshot and creates one 8.3-named ZIP handoff per next
hop, using stored-message attach, direct obox, or BSO/FLO as configured. Attach
and obox handoffs are immutable; while holding the peer's `.bsy`, BSO may merge
the snapshot into a compatible bundle already advertised in its flow file.
`--in` validates received bundles and game-owned attach envelopes, publishes
local packets, and immediately forwards transit packets. Concurrent helpers
and game processes serialize through their shared locking contract.

- **`-in` / `--in`** — Receive, unwrap, and route inbound FTN bundles.
- **`-out` / `--out`** — Bundle and hand off outbound game packets. This is the
  default when neither direction is supplied.
- **`-status` / `--status`** — Report what each spool is still holding, for
  whom, for how long, and why, and change nothing. A file count answers none of
  those: a snapshot is kept whole until every target in it publishes, so it also
  holds bundles for peers that already went out.
- **`-data DIR`** — Folder holding the game data and `ftn.cfg`; default
  `./data`, relative to the scheduler's working directory.
- **`-version`** — Print the helper and game version, then exit.
- **`-help`** — Print the options, then exit.

See [FTN Transport with `barons-ftn`](ftn-transport.md) for the complete
configuration reference, examples, scheduling, recovery, and troubleshooting.
