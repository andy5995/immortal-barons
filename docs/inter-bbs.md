# Inter-BBS Leagues

This guide is for a sysop connecting their board to other boards, so barons on
different systems share one planet. It assumes the game already runs on your
board — [Door Setup](door-setup.md) covers that.

Read the first two sections in order. The rest is reference you can come back
to, and the two worked examples at the end are complete setups you can follow.

## How a league works

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
    immortal-barons -ibbs-reset -board-id "Your Board" \
      -inbound /path/to/ftn/inbound \
      -outbound /path/to/filebox \
      -data /path/to/data
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
    immortal-barons -ibbs-reset \
      -import-bbs-cfg /path/to/bre/BBS.CFG \
      -data /path/to/data
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
    from ones that only claim to be yours. See "Signing packets" below. You can
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
"Joining a league" above, answers a different question: "is this really a league
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
copies each packet to all of them. That is the default until your Coordinator
says otherwise. A large league routes instead — see "Routing" above.

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

What changes is that arranging delivery is now yours. Point a transport at the
outbound directory: a file box entry if you already run a mailer, a timed event
if you do not. Or run `barons-ftn`, which gives you the original's handoff back.

The rest of the mapping:

| Barren Realms Elite | Immortal Barons |
|---|---|
| `BBS.CFG`, seven lines by position | `bbs.cfg`, one keyword per line |
| Line 1, sysop name | nothing — the wrapper is from `Immortal Barons` |
| Line 2, BBS name | `BoardID` |
| Line 3, node address | `ibnodes.dat`, where the helper reads it |
| Line 4, incoming files | `Inbound` |
| Line 5, netmail directory | `ftn.cfg`'s `NetmailDir` |
| Line 6, league number | `LeagueNumber` |
| Line 7, mailer | `ftn.cfg`'s `Binkley`, and only two ways rather than seven |
| `\OUTBOUND`, fixed | `Outbound`, and you choose the path |
| `ROUTE.CFG` | the roster's `HOST` entries, plus `Link` lines |
| `BRNODES.DAT` | `ibnodes.dat` |
| `BRE PLANETARY` | `immortal-barons -planetary` |

The keywords are there because a file read by position gives no warning when a
line is missing: every value below the gap moves up one and is read as the wrong
setting.

## Optional FTN handoff

`barons-ftn` turns the transport-neutral `.brp` files into ordinary FTN
file-attach netmail. The game does not run it automatically. A typical event or
door-exit chain is:

```
immortal-barons -planetary -data /path/to/data
barons-ftn -data /path/to/data
```

**Writing the netmail is not sending it.** `barons-ftn` leaves a `.msg` in the
netmail directory and stops. What carries it is whatever already carries your
netmail. On [Synchronet](https://www.synchro.net/) that is two more steps:
SBBSecho packs the message and its attachment into the outbound, then the
mailer sends it on its next outbound session. An incoming poll from the other
board sends nothing.

```
sbbsecho                          # pack the .msg and its attachment
jsexec -c ctrl exec/binkit.js     # poll out; an inbound poll sends nothing
```

Both usually run from your BBS's timed events already, so there is often
nothing to add. Knowing the shape helps when nothing arrives. A `.msg` left in
the netmail directory means the game did its part, and the mail system has not
run. That is a different problem from a `.msg` that never appeared.

Create `ftn.cfg` in that data directory:

```
NetmailDir /sbbs/fido/netmail
Binkley    No
```

- **NetmailDir** is the directory where your BBS or mailer watches for Type-2
  `.msg` netmail. A relative path is read relative to the game data directory.
  Not every BBS has one. [Mystic](https://www.mysticbbs.com/) keeps its own
  message bases and watches no such directory, so the helper has nothing to
  hand a packet to; use a file box or another transport there. On Synchronet,
  read the path from `scfg` → Networks → FidoNet EchoMail and NetMail →
  **NetMail Directory**. Check **Allow File Attachments** on that screen too.
  With it off, the wrapper is written and then ignored.
- **Binkley** is `Yes` when the mailer uses Binkley-style file attaches,
  including BinkIT, the Binkley-style mailer shipped with Synchronet, and `No`
  for a non-Binkley mailer. Synchronet can be used with several mailers, so the
  BBS package itself does not decide this switch. Omitting the setting is the
  same as `No`.
  Binkley-style handling gets a `^` before the attached path in the subject;
  non-Binkley handling gets a `FLAGS KFS` control line. Both get the private,
  local, file-attach, and kill-sent header attributes.
- **AttachDir** and **SubjectPath** are optional and control the attachment
  pathname. They are described under [Attachment pathnames](#attachment-pathnames)
  below. Omitting both keeps the behaviour of every earlier release.

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

### Attachment pathnames

Two separate things decide the attachment: where the file is written, and how
that file is spelled in the `.msg` subject. They are configured separately
because the spelling is resolved by the mailer, and only the operator knows the
mailer's working directory and attachment search path.

By default the helper moves a claimed packet into the `fido` child of the
`Outbound` or `Link` directory it came from, and puts that absolute pathname in
the subject. Both defaults are what every release before these settings existed
did, and an `ftn.cfg` with neither key behaves exactly that way.

```
AttachDir   /sbbs/fido/attach
SubjectPath Basename
```

- **AttachDir** puts every claimed packet in one directory instead of the
  per-outbound `fido` child, whatever outbound or link it came from. A relative
  path is read relative to the game data directory. Keep it on the same
  filesystem as the outbound directories: the helper claims a packet by
  renaming it there.
- **SubjectPath** chooses the spelling:
    - `Absolute`, or the key omitted, writes the full pathname. Safe with any
      mailer, and the most expensive in subject bytes.
    - `Basename` writes the filename alone, for a mailer configured to search
      an attachment directory. Point that search at `AttachDir`.
    - Anything else is a prefix. It is written in front of the filename exactly
      as configured, and the mailer resolves it against its own working
      directory — the helper never resolves it and never checks that it exists.
      A prefix written with backslashes (`C:\sbbs\ibout`) keeps them, for a
      mailer on a different kind of system.

Whatever the spelling, it must name a file the mailer can find, because the
mailer deletes the attachment after sending it.

The subject holds 71 bytes, or 70 with the Binkley `^`, for the whole spelling.
Before moving any packet, the helper checks every subject the whole run would
create, including broadcast suffixes, and exits without moving anything if one
will not fit. The error names the setting to change. When fewer than 8 bytes
are left, it warns on standard error while still queueing the mail.

**Which spelling to choose depends on your mailer.** `Absolute` spends the whole
directory out of the 71 bytes: `/sbbs/ibout/fido/` is 17, and the longest packet
name plus a broadcast suffix and Binkley's `^` can take 45 more, leaving 8.
`Basename` spends nothing on directories, but it needs a mailer that searches an
attachment directory. **Synchronet is not one** — SBBSecho reads the directory
out of the subject and reports the file as not found when it is missing — so a
Synchronet board keeps `Absolute` and a short data directory. A prefix is the
middle ground where the mailer resolves it against its own working directory.

This budget applies only when `barons-ftn` carries files through Type-2 `.msg`
netmail. A shared directory, sync tool, `scp`, or another transport that does
not put the pathname in an FTN subject has no such limit.

The game writes and closes each complete, optionally signed packet under a
non-`.brp` temporary name, then atomically renames it to its final `.brp` name.
The helper therefore needs no game lock and never scans a partial packet.
Concurrent helpers serialize through their own `barons-ftn.lock`, which the
game never takes; only the helper that moves a source into the attachment
directory creates its `.msg` or broadcast set. A malformed packet remains in the outbound directory;
a message-creation failure is moved back there for a later run. This uses
ordinary rename, exclusive file creation, and real copies—hard-link support is
not required.

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
immortal-barons -ibbs-reset -board-id "Bravo BBS" \
  -inbound ~/b-mystic/echomail/in \
  -outbound ~/b-mystic/filebox/iblocal_z1n1n1 \
  -data /path/to/member/data
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
cd ~/a-mystic
./mis poll 1:1/2
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
immortal-barons -planetary -data /path/to/data
cd /path/to/bbs
./mis poll 1:1/2
```

The game step reads and writes packet files; the mail step carries them. The
game never moves a file between boards, and your mailer knows nothing about the
game, so both have to run. The order matters: polling before the game has
written its outbox sends the packets from the run before.

Run it from your BBS's own event scheduler if it has one. If it has none, use
whatever schedules jobs on your system: `cron` or a systemd timer on Unix, Task
Scheduler on Windows. How often is up to you. Every exchange is a round trip, and the Travel Times screen in the game
reports how long your players actually wait. A league whose boards poll each hour
plays very differently from one that polls at 3am.

**You may already have some of this.** Daily maintenance runs the inter-BBS
step itself when inter-BBS play is on. Maintenance also runs on its own at the
first login of a new day, so a board with callers already reads and writes
packets once a day with nothing scheduled. What the timer adds is doing it more
often than daily, and the poll — which the game cannot do for you at all.

## Example: Synchronet

The steps under "Joining a league (member boards)" are the game's side. This
section is Synchronet's side — where each setting lives, and what carries the
packets. Do it in the order below. It assumes SBBSecho and BinkIT are already
running your other mail.

Paths below are the Linux defaults. On Windows they are `C:\SBBS\...`, the two
programs are `immortal-barons.exe` and `barons-ftn.exe`, and the script in the
last step is a `.bat` file.

### Your league address

In `scfg` → Networks → FidoNet EchoMail and NetMail:

- **System Addresses** — add the address the Coordinator assigned you. Leave
  your existing addresses alone.
- **NetMail Directory** — note the path. It goes in `ftn.cfg` below.
- **Allow File Attachments** — `Yes`. With it off the netmail is written and
  then ignored.

### A domain for the league

In `echocfg` → **Domains**, add one:

```
Name    xleague
Zones   777
```

The other three fields can stay empty. The domain keeps the league's zone from
being read as part of a network you already carry, and gives it its own
outbound tree if you set **Outbound Root**. Any name will do; use the same one
everywhere below.

### The Coordinator as a linked node

In `echocfg` → **Linked Nodes**, add the Coordinator's address with the domain
on it — `777:777/1@xleague`. Set **Session Password** to the one they gave
you. **Host** and **Port** are one level down, under **BinkP Settings...**,
along with **Poll** — which only calls when you have nothing waiting to send,
since pending files call out on their own.

Prove the link before you configure the game:

```
jsexec -c /sbbs/ctrl /sbbs/exec/binkit.js -l 777:777/1@xleague
```

Check the host it dials and that authentication succeeds.

Once the game side is set up too, `immortal-barons -league-check` should come
back with no FAIL lines.

### Point `barons-ftn` at the netmail directory

In the game's data directory, `ftn.cfg`:

```
NetmailDir /sbbs/fido/netmail
Binkley    Yes
```

`Binkley Yes` is what BinkIT expects. Leave `SubjectPath` alone: SBBSecho
takes the attachment's directory from the netmail subject, so the full
pathname has to be there. That pathname gets 70 bytes — see [Attachment
pathnames](#attachment-pathnames) — which is another reason to keep the game's
data directory short.

### The four steps, in order

```
immortal-barons -planetary -data /sbbs/xtrn/imb/data
barons-ftn -data /sbbs/xtrn/imb/data
sbbsecho
jsexec -c /sbbs/ctrl /sbbs/exec/binkit.js
```

The game reads and writes `.brp` files. `barons-ftn` wraps each one in a
`.msg`. SBBSecho packs it into the BSO/FLO outbound. BinkIT sends it. Wherever
a file stops is the step that did not run.

`sbbsecho` takes its ctrl directory from `SBBSCTRL`, not from an `.ini` path on
the command line, so set that variable in the script if your board is not at
`/sbbs`. Judge a run by whether the outbound emptied, not by BinkIT's last
line: a session that transferred everything can still end on a complaint about
files pending acknowledgement.

Put those four lines in a shell script and give it a lock, so the door's
clean-up and the timed event cannot run it at once:

```
#!/bin/sh
exec 9>/sbbs/xtrn/imb/data/planetary.lock
flock -n 9 || exit 0
...the four commands...
```

Then set it as the door's **Clean-up Command Line** in `scfg` → External
Programs → Online Programs, and add a timed event that runs it every 15
minutes or so. It carries your other networks' mail too, since SBBSecho and
BinkIT are system-wide.

On Windows there is no `flock`; run the batch file from the timed event alone
and leave the clean-up command line empty.
