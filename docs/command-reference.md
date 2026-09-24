# Command Reference

This page lists every command-line option for Immortal Barons in one place.

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
- **`-players`** — List the players and edit one of them, then exit. This is the
  original's `VIEW` command. Pick a realm by its Id letter, then choose
  **D**elete realm, **P**layer name, **R**ealm name, or **Q**uit. Deleting asks
  you to confirm and cannot be undone. Change the player name when someone has
  renamed their account on the board: the game finds a realm by that name, so
  otherwise the game does not know them at their next login. Each edit takes the
  same lock a caller's turn takes, so it is safe to run while the board is up.
- **`-maint`** — Run the daily maintenance step, then exit. On a board that
  plays alone it is not required: every login runs maintenance too, and catches
  up any days that passed with nobody playing. Running it once a day from a
  nightly scheduled task moves that work off the first caller's login, which is
  worth doing on a board with real traffic.

    **On a league board it is required.** It also runs the inter-BBS step, which
    a login does not, including the [FTN transport](ftn-transport.md) when
    `bbs.cfg` sets one up, and raises the same fault alarm as `-planetary`.
    Schedule it as often as you want packets to move.

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

    This is a development tool: it plays computer barons against each other to
    check game balance. Nothing a sysop can set creates computer barons, so on
    a board's own game it has nothing to play.

    **It plays real turns and saves the result.** Two things guard against
    running it by mistake: it asks before starting, and the default answer is
    no; and it refuses outright on a game that has any human realm, since
    advancing someone else's realm is not something a warning can undo.

- **`-dupe-check on`** / **`-dupe-check off`** — Force Dupe Checking on or off
  **for this run only**. Dupe Checking is the league rule that locks a baron out
  here when they are found playing on another board in the league.

    **It changes no setting.** The override is not saved anywhere, so the next
    command sees the saved setting again. To change the rule for real, use the Configuration
    Editor.

    It combines with any other option: `immortal-barons -local -dupe-check off`
    plays a local turn with the rule off. `off` lets a baron the league had
    locked out reach the game; `on` applies the rule even where the saved
    setting has it off. Either way, the record of who was locked out stays as
    it is.

### Inter-BBS

These options are for games that link several BBSes together (a "league"). See
[Inter-BBS Leagues](inter-bbs.md) for how league play works.

- **`-ibbs-reset`** — Start a new game as a board in a league. The same as
  `-reset`, except the settings editor also asks the league settings (board
  name, packet directories, and the interplanetary rules), and it creates the
  packet directories.
- **`-board-id NAME`**, **`-game-inbound DIR`**, **`-game-outbound DIR`** —
  Settings for `-ibbs-reset`. Giving `-board-id` skips the settings editor, so a
  member board is set up in one command. Use this when the League Coordinator
  sets the rules: they arrive in the Coordinator's next broadcast and replace
  whatever this board starts with. `-game-inbound` and `-game-outbound` default
  to `inbound` and `outbound` inside the data directory. The game does not write
  `bbs.cfg`: the reset ends by printing that file, filled in from these flags,
  for you to save yourself. It is plain text and yours alone — nothing in the
  game ever rewrites it.
- **`-import-bbs-cfg PATH`** — Take this board's name, league number, mailer,
  incoming files directory and netmail directory from an original Barren Realms
  Elite `BBS.CFG`, for `-ibbs-reset`. The last three become the FTN transport's
  `Mailer`, `IncomingFileDir` and `OutgoingNetmailDir` lines. Use it when
  converting a league you already run, so you do not retype what that file
  already says. It prints what it read. `-board-id` overrides the name, and
  naming the board in the file skips the settings editor just as `-board-id`
  does. It prints the `bbs.cfg` to save, the same as the flags do.
- **`-planetary`** — Run the inter-BBS step, then exit: take in what the mailer
  brought, read incoming packets, run the group attacks, write outgoing packets,
  and hand them to the mailer. The two mailer steps run only when `bbs.cfg` sets
  up the [FTN transport](ftn-transport.md). It can run as often as you like,
  including from the mailer's post-session event. A run that meets a fault it
  has not reported before, a failed handoff to the mailer included, exits
  non-zero and runs `bbs.cfg`'s `OnFault` command, so whatever runs this on a
  timer raises the alarm — see "Being told when the league stops moving" in the
  inter-BBS guide.
- **`-full`** — Run the full cycle, then exit: read inbound packets, play a
  turn, and write outbound packets. This is the same as running `-planetary`,
  then the door (or `-local`), then `-planetary` again, but in one step. The FTN
  transport runs before and after, as in `-planetary`, but never waits for
  another run that is already moving the mail. The play step is the one the
  same command runs without `-full`: `-local` plays in your terminal, and
  otherwise the door reads the drop file, from `-dropfile` or the working
  directory. Use `-detailed` alongside it to see each packet as it is read and
  written.

    `-full` is optional, and it never replaces the scheduled `-maint` a league
    board needs. What it adds is an exchange at the moment a caller enters, so
    news from other boards is as fresh as possible. The cost is a wait before
    the game starts. With a timer every 15 minutes, a board can leave it off.
- **`-detailed`** — Show each packet as it is read and written. This is a
  modifier, not a mode: it takes effect when used with `-full` or `-planetary`.
  Without one of those, it is ignored.
- **`-league-config`** — Send this board's league settings to the whole league,
  then exit. Only the league coordinator (node #1) uses this. A `-planetary` run
  on the coordinator's board sends them too, so a member that missed one
  broadcast catches up on its own; use this to push a change immediately.
- **`-league-check`** — Check this board's league setup — the roster, the board
  name, the packet directories and the keys — and report everything that is
  wrong at once, then exit. Run it after joining a league, and whenever a
  transport run complains. It exits non-zero when anything failed, so an event
  can run it.
- **`-league-routes`** — Print which board each planet's packets are handed to,
  and the directory they are written in, then exit. Use it to check a roster the
  coordinator has just sent.
- **`-ftn-status`** — Report what the FTN transport's spools are still holding,
  for whom, for how long, and why, and change nothing. Counting the files cannot
  tell you that: the spool keeps a batch until it has been handed to the mailer
  for every board it is for, so it can still hold files for boards whose copy
  already went out. See [FTN Transport](ftn-transport.md).
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
  coordinator requires is marked, and so is one playing by rules other than the
  coordinator's — as is this board itself, on a line under the table. Neither
  mark stops anything: they say what is true, and the fix is the coordinator's
  next broadcast or the sysop's own config.
- **`-playerlist`** — Write `PLAYERLIST.LST`, then exit: every realm on every
  board. A realm on this board is listed by its caller's BBS handle; a realm on
  another board by its name, because boards do not send handles to each other.
  Rows with the same letter in the Owner column belong to one caller, matched
  through the duplicate-checking owner hash, so the letters appear across boards
  only while Dupe Checking is on. A realm locked out by duplicate checking says
  which board locked it. Only the league coordinator (node #1) may write this
  one.

The three `.LST` reports are written into the data directory, and are built from
what packets have already told this board — none of them changes the game.

### Info

- **`-version`** — Print the version, then exit.
- **`-help`** — Print the grouped list of options, then exit.
