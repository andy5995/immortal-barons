# bbs.cfg Reference

This page is for the BBS operator (the sysop). It lists every setting in
`bbs.cfg`, with the values each one takes and what happens when you leave it
out. The guides linked from each entry explain how the settings work together.

## The file

`bbs.cfg` is a plain text file in the data directory (the `-data` option,
`./data` by default). It holds the settings that belong to this board and this
machine: the board's name in a league, its directories, and a few choices each
sysop makes alone. The league's rules are in `config.json` instead. The game
never rewrites `bbs.cfg`, so a reset or a Coordinator's broadcast cannot change
it. With no file at all, every setting has its default.

The rules for a line:

- One setting per line: the keyword, then one or more spaces or tabs, then the
  value. The value is the rest of the line, so it may contain spaces, as a
  Windows path or a board name can.
- Keywords ignore case.
- A keyword with no value is ignored, and the default stays in place.
- Blank lines are skipped. A line that starts with `#` or `;` is a comment. A
  comment must be on its own line: text after a value is part of the value.
- A path that is not absolute is read as a path inside the data directory, not
  inside the directory the BBS starts the door from.
- A setting given twice takes the last value. `IncomingFileDir` is the
  exception: every line counts. `Link` and the per-node form of `GameOutbound`
  are given once per node, and for each node the last line counts.
- A keyword the game does not know is ignored. The inter-BBS commands print a
  warning for it and suggest the nearest keyword: `-maint` and `-full` on a
  league board, `-planetary`, `-ftn-status`, `-league-check`, `-league-config`,
  `-league-reset` and `-league-routes`.
- The settings older releases used, `Inbound`, `Outbound` and the two-field
  `Link <node> <dir>`, stop the inter-BBS commands on a league board. They
  print the lines to use instead. `-full` only skips the league exchange and
  lets the caller play. An `ftn.cfg` file left in the data directory has the
  same effect.

What a value that cannot be used does depends on the setting. For the settings
above the FTN transport sections, the line is ignored and the default stays in
place; the inter-BBS commands warn about a `Lottery` or `PirateNews` value that
is not a switch. For the FTN transport settings, the transport does not run
until the line is fixed: packets stay in `GameOutbound`, the run reports a
fault, and what is already in `GameInbound` is still applied.

### Switches

A switch takes `yes`, `true`, `on` or `1` to turn it on, and `no`, `false`,
`off` or `0` to turn it off, in any case.

### Example

A league board whose mailer is BinkIT on
[Synchronet](https://www.synchro.net/):

```
BoardID          Avalon
BBSName          Avalon BBS
LeagueNumber     900
GameInbound      inbound
GameOutbound     outbound
BulletinDir      bull
Lottery          yes
PirateNews       yes
OnFault          ntfy publish mybbs "$IB_FAULTS"

IncomingFileDir  /sbbs/fido/inbound
Link 1 BSO /sbbs/fido/outbound Normal
```

## Identity and league

### `BoardID`

This board's name. In a league it must be the same as the name the Coordinator
put in `ibnodes.dat`, character for character.

**Values:** any text.

**Default:** `local`.

More: [Joining a league](inter-bbs.md#joining-a-league-member-boards).

### `LeagueNumber`

The number of the league this board plays in. The Coordinator chooses it. A
packet marked with another league's number is ignored.

**Values:** a whole number from 1 to 999. Any other value is ignored without a
warning, as if the line were missing.

**Default:** none. A league board then refuses to run `-maint`, `-planetary`
and `-full`, and says which line to add. A board that plays alone does not need
it.

More: [Joining a league](inter-bbs.md#joining-a-league-member-boards).

## The game's packet directories

### `GameInbound`

The directory where packets from other boards wait for the planetary step. A
bundle of packets found here is unwrapped as well.

**Values:** a directory.

**Default:** `inbound`, inside the data directory.

More: [How packets move](inter-bbs.md#how-packets-move-you-choose-the-schedule).

### `GameOutbound`

The directory where the game writes packets for the other boards.

**Values:**

- `GameOutbound DIR` — the directory for every board with no line of its own.
- `GameOutbound N DIR` — the directory for packets whose next hop is node `N`
  (1 to 999). Give one line for each neighbor. Only a board that forwards
  packets for others needs this form.

**Default:** `outbound`, inside the data directory, and no per-node lines.

More: [A board that hosts others](inter-bbs.md#a-board-that-hosts-others).

## Bulletin files

### `BulletinDir`

The directory where the game writes its bulletin files: scores, news and
rankings, as `.ans`, `.txt` and HTML.

**Values:** a directory.

**Default:** none. When the line is missing or has no value, no bulletin files
are written.

More: [Bulletin Files](bulletins.md#turning-them-on).

### `BBSName`

What the bulletin web pages call this board.

**Values:** any text.

**Default:** the `BoardID`.

More: [The bbs.cfg lines](bulletins.md#the-bbscfg-lines).

### `BoardURL`

The address of this board's own website. The web pages link the board's name
to it.

**Values:** a URL.

**Default:** none. The board's name is shown as plain text.

More: [The bbs.cfg lines](bulletins.md#the-bbscfg-lines).

### `BulletinURL`

The address at which your web server serves `BulletinDir`. It fills the
`{{pageurl}}` word in the page header.

**Values:** a URL.

**Default:** none. `{{pageurl}}` is left empty.

More: [The three files you own](bulletins.md#the-three-files-you-own).

## Lottery and news

### `Lottery`

Whether this board offers the Queen's lottery.

**Values:** a [switch](#switches).

**Default:** `yes`.

More: [Your board's own settings](inter-bbs.md#your-boards-own-settings-bbscfg).

### `PirateNews`

Whether pirate raids are reported in the planet news. With `no`, raids still
happen and the raider still gets a report.

**Values:** a [switch](#switches).

**Default:** `yes`.

More: [Your board's own settings](inter-bbs.md#your-boards-own-settings-bbscfg).

## Fault alarm

### `OnFault`

A command to run after a `-maint`, `-planetary` or `-full` run that meets a
league fault it has not reported before.

**Values:** one command line. On Unix it runs with `sh -c`, on Windows with
`cmd /c`, so a pipeline works. It gets these environment variables:

- `IB_FAULTS` — the faults, separated by ` | `. On Unix they are also the first
  argument.
- `IB_BOARD` — the `BoardID`.
- `IB_DATA` — the data directory.

The command is stopped after 30 seconds. If it fails, the run prints the error
and goes on.

**Default:** none.

More: [Being told when the league stops
moving](inter-bbs.md#being-told-when-the-league-stops-moving).

## FTN transport: receiving

The FTN transport settings are read only on a league board. A board whose
packets travel another way leaves them all out.

### `IncomingFileDir`

A directory where the mailer puts the files it receives. The transport
unwraps the league's packets from it.

**Values:** a directory. Give one line for each directory the mailer delivers
into. The transport reads only the files in the directory itself, so a
subdirectory the mailer uses needs its own line.

**Default:** none. Only `GameInbound` is read.

More: [Inbound settings](ftn-transport.md#inbound-settings).

### `IncomingNetmailDir`

The directory where the tosser leaves received `.msg` netmail. The transport
looks there for the messages that announce attached packets.

**Values:** a directory.

**Default:** every `IncomingFileDir`.

More: [Inbound settings](ftn-transport.md#inbound-settings).

### `OboxMeshFanout`

Whether a broadcast with no address is sent on to the other peers. It applies
only to a broadcast that arrives as a plain file, over an `Obox` or `BSO` link,
not as a netmail attachment.

**Values:** a [switch](#switches).

**Default:** `yes`.

More: [Mesh warning](ftn-transport.md#mesh-warning).

## FTN transport: sending

### `OutgoingNetmailDir`

The directory where the game writes outgoing `.msg` netmail for the tosser to
pack. Each message has one file attached.

**Values:** a directory. It must exist.

**Default:** none. It is required when any peer uses an `Attach` link,
including a peer with no `Link` line. With neither this nor a `Link` line,
packets stay in `GameOutbound`.

More: [Stored-message attach
settings](ftn-transport.md#stored-message-attach-settings).

### `AttachDir`

The directory where the transport keeps outgoing files for `Attach` and `BSO`
links until the mailer sends them.

**Values:** a directory. A short absolute path keeps the netmail subject
short.

**Default:** `att`, inside the data directory.

More: [Keeping attach subjects
short](ftn-transport.md#keeping-attach-subjects-short).

### `Mailer`

Which mailer carries the netmail. It decides how the netmail asks the mailer to
delete the attachment after sending, or turns netmail off.

**Values:** the original game's seven names, in any case. Only two of them
change what the game writes:

| Value | What the game writes | Use it for |
| --- | --- | --- |
| `Binkley` | the attachment path starts with `^` | BinkIT ([Synchronet](https://www.synchro.net/)'s mailer) and other Binkley-style setups |
| `FrontDoor`, `DBridge`, `InterMail`, `DBridgeOld`, `Other` | a `FLAGS KFS` line in the message | any other mailer; all five do the same thing |
| `None` | no netmail at all, as in the original | a board with no netmail directory. With no `Link` line, packets stay in `GameOutbound` for your own script to move; a peer the transport delivers to needs an `Obox` or `BSO` link |

Any other value, or `None` together with a `Link ... Attach` line, is refused,
and the transport does not run until the line is fixed.

**Default:** the same as `Other`.

More: [Stored-message attach
settings](ftn-transport.md#stored-message-attach-settings).

### `SubjectPath`

How the attachment's path is written in the netmail subject. It applies only to
`Attach` links.

**Values:**

- `Absolute` — the full path.
- `Basename` — the file name alone. Use it only when the mailer looks for
  attachments in `AttachDir` by itself.
- anything else — used as the directory part, with the file name added to it.
  The separator is `\` when this value contains one, otherwise `/`.

**Default:** `Absolute`.

More: [Keeping attach subjects
short](ftn-transport.md#keeping-attach-subjects-short).

### `Bundled`

Whether the transport sends its peers ZIP bundles or plain packets. A `Raw` or
`Bundled` word on a `Link` line overrides it for that peer.

**Values:** a [switch](#switches).

**Default:** `no`. Every peer gets plain packets.

More: [Plain packets for boards that cannot read a
bundle](ftn-transport.md#plain-packets-for-boards-that-cannot-read-a-bundle).

## FTN transport: per-peer links

### `Link`

How the transport hands packets to one directly connected board.

**Values:** `Link N MODE`, where `N` is the peer's node number (1 to 999) and
`MODE` is one of:

- `Attach` — a netmail message in `OutgoingNetmailDir` with the file attached.
  It takes no directory.
- `Obox DIR` — the file goes into the peer's own outbox directory.
- `BSO DIR [FLAVOR]` — the file goes into a Binkley-style outbound. `DIR`
  is the outbound for the peer's zone. `FLAVOR` is `Immediate`, `Continuous`
  (or `Crash`), `Direct`, `Normal` or `Hold`; the default is `Normal`.

`Raw` or `Bundled` may end any `Link` line. It overrides `Bundled` for this peer.

A directory in a `Link` line must not contain spaces. Give one line for each
peer.

**Default:** a peer with no `Link` line uses `Attach`. Once any `Link` line is
present, a broadcast with no address is sent on only to the peers listed.

More: [Per-peer links](ftn-transport.md#per-peer-links).
