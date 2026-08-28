# Inter-BBS Troubleshooting

This is the guide for a league that has stopped moving. It is written for two
readers: a sysop whose board has gone quiet, and the League Coordinator, who has
a few checks and a few problems nobody else has. [Inter-BBS
Leagues](inter-bbs.md) covers the setup this assumes you already have.

Work in this order. Each step tells you whether the fault is in your half or
somewhere else, which is the question worth answering first.

1. `immortal-barons -league-check` — is this board's own setup sound?
2. The last `-planetary` run report — did packets arrive, and what happened to
   them?
3. `planetary.log` — what did earlier runs complain about?
4. Your mailer or file transport — did anything reach the inbound directory at
   all? With `barons-ftn`, `--status` answers this without changing anything.

## Step 1: `-league-check`

```
immortal-barons -league-check -data /path/to/data
```

It reports every problem it can find at once rather than stopping at the first,
and exits non-zero when anything failed, so a timed event can run it and tell
you only when something is wrong.

The lines it prints, and what a FAIL on each one means:

| Check | A FAIL means |
|---|---|
| Inter-BBS play | this board plays alone; `-ibbs-reset` joins a league |
| League roster | `ibnodes.dat` is missing, unreadable, or lists no boards |
| Roster entries | a line in the roster is malformed; the message names which |
| Board name | not set, or not the name of any board on the roster |
| League number | outside the 1-999 the packet filenames can carry |
| Inbound directory | it does not exist, or cannot be read |
| Outbound directory | it does not exist, or cannot be written to |
| Coordinator key | not recorded, or the file is not a key — league orders will be refused |
| Board signing key | `board.key` is there but unreadable, so packets go out unsigned |

A board with no signing key at all is **ok**, not a failure: signing is optional,
and a league where nobody has run `-gen-board-key` works. What is not optional
is the Coordinator's public key on every member board — without it a board
cannot check that league orders came from the Coordinator, and refuses them.

When `barons-ftn` is in use, `-league-check` also reports the transport's own
backlog, because by the time anyone asks why a board went quiet the run that
failed is long gone. Those lines are the same ones `barons-ftn --status` prints;
see [Using `barons-ftn`](#using-barons-ftn-to-see-where-a-packet-stopped) below.

## Step 2: read the run report

Every `-planetary` run prints what it did. The counters are the fastest
diagnosis in the game, so it is worth knowing what each one is telling you.

**Applied** is the only number that means the league is working. The rest say a
packet arrived and did not become part of your game.

A run that skipped anything names each reason:

| The report says | What happened | What to do |
|---|---|---|
| refused, not matching the sender's key | the packet claimed to be from a board whose roster key it does not match | see [Refused packets](#refused-packets) |
| could not be read at all | corrupt, truncated, or not a game packet | see [Quarantined packets](#quarantined-packets) |
| left in place, too new to trust as complete | the file is under five minutes old and may still be mid-transfer | nothing; the next run picks it up |
| held for a protocol this build does not read | the two boards are on different releases | see [Held packets](#held-packets) |
| already seen | a duplicate or a replay of a packet already applied | nothing; this is the replay guard working |
| for another league | its league number is not yours | nothing, if you share an inbound directory with another league. Otherwise check `LeagueNumber` in `bbs.cfg` on both boards |
| mesh copy | a copy addressed to somebody else, in a mesh setup | nothing |

Three more lines appear when they apply. **Passed N packets on** is this board
forwarding for a neighbour, which is routing working. **Released N held
packets** means an upgrade here freed a backlog. **The League Coordinator's
roster replaced this board's copy** means an order arrived and was accepted —
the one line that tells a member board its Coordinator link is alive.

`-detailed` alongside `-planetary` shows each packet as it is read and written,
when a counter alone is not enough.

## Step 3: the game's log

Every `-planetary` run prints its transport faults when it finishes, and writes
the same lines to `planetary.log` in your data directory, each one stamped with
the date and time. Most sysops run the step from a scheduler, which throws
printed output away, so the file is usually the only copy you have. It keeps the
last 500 lines.

These are the faults it records:

- **A packet was destroyed.** Either no board of that name is on your roster, or
  it was passed from board to board 25 times and never reached anyone. The
  second one means a `HOST` line points back the way the packet came, so the
  route is a circle.
- **Nothing can be sent to a board.** Same cause, found before anything is
  written: that board is not on the roster, so this board has no route to it.
  Anything addressed there is discarded here.
- **A packet was refused.** The line says why. A packet that did not match the
  sending board's key, a board running an older release than the league
  requires, or Coordinator orders that failed one of the six checks — those are
  the three, and the wording names which one.
- **Another board refused ours**, quoting that board's own reason.
- **Packets from a board are being held.** See below.
- **A packet could not be read and was quarantined.** See "Quarantined
  packets" below.
- **The order a contested batch applied in**, whenever more than one board's
  packets arrived in the same run. See [When two boards disagree about what
  happened](#when-two-boards-disagree-about-what-happened).

## Refused packets

A refusal is the one skip that always means something is wrong somewhere. There
are three kinds, and the log line says which.

**The sending board's key did not match.** The roster carries a public key for
that board, and the packet was not signed by the matching private half. Either
the sending board regenerated `board.key` without giving the Coordinator the new
line, or the roster entry was mistyped, or the packet is not from that board at
all. The fix is on the sending board and at the Coordinator, not here.

A roster entry with **no key at all** is a different outcome: it is applied
unchecked. "Cannot check" and "failed the check" are deliberately kept apart —
every league starts with no keys, and a league that never adds them still works.

**The board runs a release older than the league requires.** The Coordinator has
set a minimum version, and that board is below it. The fix is an upgrade on the
sending board.

**Coordinator orders failed their check.** Six situations refuse them, and three
of the six are fixed on the sending board rather than yours. In the order they
are tested:

| The reason | Where the fix is |
|---|---|
| this board is the League Coordinator and takes orders from no one | nowhere — a Coordinator does not take its own orders back |
| it came from node N, and only node 1 may issue orders | the sending board is not the Coordinator, or its roster node number is wrong |
| this board's roster names no node 1 | your roster is missing the Coordinator's entry |
| this board's Coordinator is X | you and the sender disagree about who the Coordinator is |
| no Coordinator key is recorded here | yours: run `-coord-key` with the key the Coordinator gave you |
| the sending board did not sign it | the Coordinator has no `coord.key`, so its orders go out unsigned |
| the signature did not match the key recorded here | the Coordinator's key changed and you still hold the old one |

## Held packets

A packet whose format this build cannot read is moved to the `held` folder in
your data directory instead of being refused. Refusing would destroy a roster
update, mail, or a returning strike that will be perfectly good once both boards
run the same release.

Every planetary run looks in that folder again and applies whatever it can now
read. Whether that ever happens depends on which side of the upgrade you are
on, and the difference matters:

- **You are behind.** The other board upgraded first, so its packets state a
  format newer than yours. Upgrading releases them: your next planetary run
  reads the folder, finds them readable, and applies the backlog with nobody
  doing anything. Leave the files alone.
- **You are ahead.** You upgraded first, so packets from boards still on the
  old release state a format older than yours. Those are held too, and your
  build will never speak that older format again — so they stay held even after
  the other boards upgrade. What they contained is not lost from disk, but it
  will not reach your game on its own.

The second case is the one to plan around, because the cost falls on whoever
upgrades first, which is the opposite of what you would expect. When a release
changes the packet format, the guide for that release says so. Agree a window
with your Coordinator and upgrade close together, so nothing spends long in
flight between two boards that disagree.

## Quarantined packets

A file that cannot even be parsed — corrupt JSON, a truncated transfer, or a
foreign file dropped in the wrong directory — is moved to the `bad` folder
in your data directory instead of blocking the rest of the batch. Unlike
`held`, nothing here is expected to become readable later on its own:
planetary runs never look in `bad` again, and a repaired copy has to be
dropped back into your inbound directory by hand.

A file young enough that it might still be mid-transfer — a mailer that
writes straight to the final name rather than a temp-then-rename dance — is
left alone for five minutes before it is ever quarantined, so a run that
happens to land mid-write does not permanently lose a packet that would
have applied cleanly on the next one.

A second file quarantined under the same name (a mailer retrying a bad
transfer, say) is kept as its own numbered copy rather than overwriting the
first. If a neighbour's transport keeps redelivering one broken file
without limit, `bad` stops accepting new copies of it past roughly a
thousand and the planetary run's log says so — clearing the folder of
copies you have already looked at is a sysop task; nothing does it for you.

## Nothing in the log, and still nothing arriving

Then the packet never reached your inbound directory, and the fault is in the
half you own. Three things to check:

Your **Inbound Dir** setting has to name the directory your mailer really writes
to. If it names a different one, the game reads an empty directory and reports
nothing, because an empty inbound is also what a quiet day looks like.

Your mailer has to be running and linked. "Step 5 — prove the link" in the
league guide polls each board from the other and reads the result properly; a
session that connects and transfers nothing is the answer you want.

If you hand packets to FidoNet, writing the netmail is not sending it. See
["Writing the netmail is not sending it"](inter-bbs.md#before-bundled-transport)
— a `.msg` still sitting in your netmail directory means the game did its part.

The in-game **Travel Times** screen is where players see this first. A planet
whose round trip stops moving is the same fault, seen from the other end.

## Using `barons-ftn` to see where a packet stopped

When the transport is `barons-ftn`, it will tell you where in the chain a packet
is sitting. This matters because the game and the transport keep separate
directories on purpose: a packet the game has written is not a packet the mailer
has been given, and neither of those is a packet that has been sent.

### `--status` changes nothing

```
barons-ftn --status -data /path/to/data
```

Reach for this first: it reads the spool journals and prints what is unfinished,
for whom, for how long and why, without moving anything. `-league-check` prints
the same report.

Two of its answers decide who acts. An inbound receipt held by a **canonical-name
collision** is waiting on *your* decision and will wait forever without it; one
held **in transit for another board** is waiting on that peer, and names that
peer's last error. An **unreadable journal** is the one line here that is a
fault — nothing will ever retry it.

Read the counts carefully rather than as a backlog: ["What a healthy spool looks
like"](ftn-transport.md#what-a-healthy-spool-looks-like) explains why a growing
number of waiting snapshots is often a working transport with one unreachable
peer.

### What a run tells you

`--out` bundles and hands off; `--in` receives, unwraps and routes. Both print
warnings to standard error and a summary to standard output, and both act, so
they are not the command to reach for while you are still working out what is
wrong.

- `Queued <packet> for <next hop> (<address>) as <message>` names the file the
  transport handed over and to whom. That is the point where the packet stops
  being the game's problem and starts being your mailer's: if a queue line
  appeared and the far board never heard, the fault is downstream of the game.
- `N queued; M snapshot(s) still waiting on K peer(s)` on `--out` is the line
  worth logging. An empty system and a stalled one both queue nothing, and this
  is the only place the difference shows.
- `No outbound packets.` means the game wrote nothing to hand over — so the
  question is whether `-planetary` ran, not whether the transport works.
- `Delivered N packet(s) to the game.` on `--in` is the count that should be
  matched by the next `-planetary` run's **Applied**. If `--in` delivers and
  `-planetary` applies nothing, the packets are in the game's inbound and were
  refused, held, or quarantined — read the run report and the log, above.

Run the three in the order the game expects, and the counts line up end to end:

```
barons-ftn --in -data /path/to/data      # deliver what the mailer brought
immortal-barons -planetary -data /path/to/data
barons-ftn --out -data /path/to/data     # hand over what the game wrote
```

A `barons-ftn` error is printed with the setting that fixes it, so the message
is worth reading in full rather than grepping for the first line — a refused
subject length runs past 700 bytes of explanation.

[FTN Transport with `barons-ftn`](ftn-transport.md) has the table of where files
accumulate and what each location means, which is the next step when `--status`
says a peer is waiting and you need to know on what.

## Finding the board that went quiet

Three reports answer "who has stopped talking to us", built from what packets
have already told this board. None of them changes the game, and each writes a
`.LST` file into the data directory ([Command Reference](command-reference.md)
has the detail):

- `-lastpacket` — when each board was last heard from. A date that has stopped is
  the board to chase.
- `-bbsinfo` — the same, plus the release each board runs, marking any below a
  version the Coordinator requires. This is where a version refusal is confirmed
  from your side.
- `-playerlist` — every realm on every board. Coordinator only.

A board missing from these entirely was never heard from at all, which points at
the roster or the routing rather than at that board.

## For the League Coordinator

Everything above applies to the Coordinator's own board too. These are the
problems that are only yours.

**Your orders are being refused.** The refusing board's log says which of the
six checks failed, so the first move is to ask for that line rather than to
guess — three of the six are fixed at your end and three at theirs, and the
wording tells you which. The common one in a young league is a board that never
ran `-coord-key`.

**Check the routing before you wonder why nothing arrives.** `-league-routes`
prints which board each planet's packets are handed to and the directory they
are written in. Run it on any board after sending a new roster. A roster with no
`HOST` lines is a legitimate setup — every board links to every other, and one
unaddressed broadcast file is written that your transport has to copy to
everybody — but it is not what a routed league looks like, and the command says
which of the two you have.

**A circle destroys packets.** When a `HOST` line points back the way a packet
came, the packet is passed between boards 25 times and then destroyed, with a
log line on whichever board finally gave up. `-league-routes` on each board is
how the loop is found.

**Your roster is the league's roster.** It travels with your orders, and a
member board applies it under the same check as the ruleset. A board that never
accepted your key never accepted your roster either, so it is still routing by
whatever it had — which is a slow, quiet failure rather than an error anyone
sees.

**A new season.** `-league-reset DATE` resets your board and sends a signed
order for the others to reset on their next `-planetary` run. A board that
refuses your orders will not reset, and will then be a board playing a different
season from everyone else. Confirm with `-bbsinfo` that every board is being
heard from *before* you send it.

**Version floors.** If you require a minimum release, a board below it has every
packet refused, and its sysop sees only that their packets bounce. `BBSINFO.LST`
marks those boards; tell them what the floor is rather than letting them work it
out from refusals.

## When two boards disagree about what happened

A trade bid, a land claim, or a group attack that both sides remember
differently is a question about ordering, not about transport. When more than
one board's packets arrive in the same run, the log records the order the
contested batch applied in. That line is the answer, and it is on the board
where the two packets met — which is not necessarily either of the boards
arguing.

`-detailed` on the next run shows the same thing as it happens.
