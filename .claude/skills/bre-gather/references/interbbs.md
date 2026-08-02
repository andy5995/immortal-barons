# Gathering from a two-board BRE InterBBS league

Everything single-board lives in `SKILL.md`. This file covers the case that needs
two BRE installs talking to each other: interplanetary attacks, recon exchange,
the coordinator's broadcasts, and anything about packets going missing.

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
