# Gathering from a two-board BRE InterBBS league

Everything single-board lives in `SKILL.md` and the other references. This file
covers the case that needs two BRE installs talking to each other:
interplanetary attacks, recon exchange, the coordinator's broadcasts, and
anything about packets going missing.

## Read the tickets first

A previous session already drove a live two-board BRE 0.988 league (coordinator
plus member, direct-read, no mailer) and compared its `PLANETARY` exchange
against Immortal Barons' own packet. **The findings are in GitHub issues #60
through #65**, not here. #60 is the tracking issue and lists what BRE exchanges
that IB does not — read it before repeating any of that work.

## The two boards are already on this machine

Checked 2026-08-01. Both are installed and paired; neither needs setting up
again.

| | path | node | `ROUTE.CFG` |
| --- | --- | --- | --- |
| Board 1 | `~/.dosemu/drive_c/games/bre-dos` | 1:20/100, "the Graveyard Shift" | `ROUTE * 2` |
| Board 2 | `~/.dosemu/drive_c/games/bre-mis` | 1:20/101, "Test Planet Two" | `ROUTE * 1` |

Both list both nodes in `BRNODES.DAT`. Board 1 is the one every other part of
this skill refers to, and the one with the play data described elsewhere.

## Setting up a run

From Andy, and matching what is on disk:

- **Two directories, each with its own BRE install** and its own data files.
- **Unique node numbers**, listed in both boards' `BRNODES.DAT`.
- **Reset both games** with **Turns of Protection 0** and **Turns per Day 20**.
  Protection 0 makes realms attackable straight away; 20 turns a day means a
  whole exchange fits in one sitting instead of spanning game days.

Everything about driving a single board — the tmux/dosemu2 harness, key pacing,
the clock-tamper trap, the clean-quit rule — applies unchanged to each board.
Run them as two tmux sessions.

## Transport

There is no mailer. The transport is a manual file move between the two boards'
outbound and inbound directories, which is the whole point: it makes the packet
schedule something you control.

That control is what lets you observe **packet loss**. Withholding a packet
instead of delivering it is the way to see what BRE's "Days before 'lost' forces
returned" setting actually does to a detachment whose result never comes home
(issue #96) — a question the disassembly did not answer, because nothing in
either binary increases the away-force counts through an ordinary write.

## What a packet looks like

Board 2's `outbound/` still holds `2.msg` from the earlier run:

```
BRE System            <- from
BRE System            <- to
^C:\GAMES\BRE-MIS\OUTBOUND\999B0201.001    <- subject
25 Jul 26  17:59:59
INTL 1:20/100 1:20/101
```

So the envelope is a stock FidoNet `.MSG` and the game data rides in a **separate
attached file**, named `999B<from><to>.<seq>` on the evidence of that one sample.

**The attachment itself is gone.** Only the envelope survives, so reading what
BRE actually puts on the wire means generating a fresh one. Keep the next
attachment: copy it out of `outbound/` before delivering it, since processing on
the far side consumes it.

Note that IB deliberately does **not** copy this wire format — it uses its own
JSON packets, a clean-room choice recorded in #60. The value of reading BRE's
attachment is learning *what* it exchanges, not how it frames it.

## `BRE PLANETARY` and the packet files

**Daily maintenance is NOT the packet pass.** Launching `BRE` on a new day runs
Daily Maintenance and writes nothing outbound. League traffic is a separate run,
`BRE PLANETARY` ("Checking Daily Maintenance / … / Processing Incoming Data / …
/ Processing Outbound Data / Checking for Any Lost Attacks / Planetary
Maintenance Complete"), which is what IB's `-planetary` mirrors. An outbound
directory that stays empty while you wait for scores or an attack almost always
means this pass never ran.

**A copied board routes to ITSELF.** The stock `ROUTE.CFG` is `ROUTE * 2`, so a
copy made into node 2 sends everything to node 2, and `BRE PLANETARY` completes
and exports nothing, with no error. Node 1 routes `* 2`, node 2 `* 1`.

**A working export**, in the board's `OUTBOUND`:

```
999b0102.002     the data packet: league 999, game b (BRE), node 01 -> 02, sequence .002
brnodes.999      the node list, redistributed with it
```

**BRE consumes a received packet from the directory `bbs.cfg` line 4 names**
("Your Front End Incoming FILE Directory"; line 5 is "Your NetMail Directory",
per `bre.doc`). A packet dropped anywhere else sits there untouched. A successful
ingest produces the reply packet and an FTN netmail attach:

```
OUTBOUND/999b0201.003    the reply, node 02 -> 01
INBOUND/2.msg            the .MSG netmail attach
```

So BRE moves league traffic as ordinary FTN netmail with a file attach (#117).
Round trip: A `BRE PLANETARY` → put A's outbound packets where B's line 4 points
→ B `BRE PLANETARY` → the same back.

## Local InterBBS: the documented ring, and the case-collision that breaks it

`docs/bre.doc` has a **"Local InterBBS Setup"** section for exactly the
several-games-on-one-machine case. Read it before improvising a transport:

- Each game's **inbound files directory** (bbs.cfg **line 4**, "Front End
  Incoming FILE Directory") points at the **previous game's OUTBOUND** — a ring.
  For two boards: A reads B's OUTBOUND, B reads A's OUTBOUND.
- BRE always **writes** to `<its own dir>\OUTBOUND\`; line 4 only says where to
  READ. Line 5 is the **netmail** dir, where the Binkley `.MSG` wrappers land —
  seeing outgoing `.msg` files there is line 5 working, NOT the dirs being
  swapped.
- `ROUTE.CFG`: game 1 `ROUTE * 2`, game 2 `ROUTE * 1` (last routes back to #1).
- **No file transfer step exists or is needed.** Run `BRE PLANETARY` on each
  board in order; the docs say running the cycle **twice** gives immediate
  results. `BRE INBOUND` runs only the inbound half, which is the fast way to
  test ingestion.

**The landmine that cost a whole session (2026-08-11):** the boards had BOTH an
`OUTBOUND` and an empty lowercase `outbound` directory. dosemu's case-insensitive
lookup resolved the DOS path `C:\GAMES\BRE-B\OUTBOUND` to the **empty
lowercase one**, so BRE scanned a directory that never held a packet and
reported nothing at all — no error, no "Unknown Node", just a silent no-op,
while `DIR` from the DOS prompt happily showed the file in the *other* directory.

**Never diagnose a "BRE ignores my file" problem by guessing at filename masks.
Watch what it actually opens:**

```
inotifywait -m -r -e open,access,create,delete <bre-dirs> > /tmp/io.log &
# ...run BRE INBOUND...
grep -i outbound /tmp/io.log
```

That named the wrong directory in one run, after several dead-end hours spent
base-solving the overlay to find the search mask. Reach for inotify the moment
BRE appears to ignore a file. Success looks like:

```
    Processing Incoming Data from Node 2
     Compressed: 334       Decompressed: 2488      %: 86.6%
```

and on the sending side:

```
     Outbound mail for Test Planet Two - Node 2 created.
```

## The turn-played gate on InterPlanetary Ops

Five InterPlanetary Ops items refuse until the caller
has played a turn this entry, and every one of them SAYS so — *"You must play at
least one turn per entry in the game to access this option."* This paragraph
said the refusal was usually silent until the dispatch was read for #162; it is
not, and "the item did nothing and said nothing" is therefore evidence AGAINST
this gate, not for it. Which five, and where each is tested, is in
`docs/mechanics-reference.md`.
