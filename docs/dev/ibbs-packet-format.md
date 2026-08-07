# Inter-BBS Packet & Node-List Formats (developer reference)

This is the on-disk contract for Immortal Barons' inter-BBS ("interplanetary")
play. It is a developer reference, not a player/sysop doc — the door setup guide
is `docs/door-setup.md`.

The clone is an independent reimplementation and does **not** use BRE's binary
packet format. It defines its own JSON packets. The node list reuses BRE's
plain-text `BRNODES.DAT` layout (under the clone's own filename `ibnodes.dat`).

## Transport model (Option A, file-drop)

The game only reads and writes packet files in two directories, set per board in
the Configuration Editor and stored in `config.json`:

- `InboundDir` — packets from other boards arrive here.
- `OutboundDir` — the game writes packets for other boards here.

Both are resolved against `DataDir` unless absolute (`Config.Inbound()` /
`Config.Outbound()`) — a door is launched from whatever working directory the
BBS chooses, so a CWD-relative path lands somewhere different on every call.

Moving files between boards is external to the game (a mailer, a sync tool, scp,
a shared mount). `RunPlanetary` (`immortal-barons -planetary`, also folded into
`-maint` when `IBBS` is on) reads and applies inbound packets, launches due
group attacks, exports this board's scores, and writes the outbox.

## Packet files (`*.brp`)

Each packet is one JSON file. Filename:
`<from>-to-<to>-<date>-<n>.brp`, where `<to>` is `all` for a broadcast. A
transport that fans out must copy a broadcast to every other board's inbound.

The JSON is `game.Packet`:

Every field is optional; one packet carries whatever the run had to send. The
authority is `game.Packet` — this is a reader's map of it, not a second
definition.

```json
{
  "FromBoard": "AlphaBBS",
  "ToBoard": "",                    // "" = broadcast to the whole league
  "Date": "2026-07-04",             // ISO game date the packet was written
  "Scores":  [ RemoteScore ],       // score share (feeds IP scores / attack targets)
  "Attacks": [ RemoteAttack ],      // strikes landing on ToBoard
  "Terrors": [ RemoteTerror ],      // terrorist ops landing on ToBoard
  "Results": [ AttackResult ],      // outcomes returning to the origin
  "Recon":   [ ReconRequest ],      // scouting asked of ToBoard (#61)
  "ReconReports": [ SpyReport ],    // answers coming back to the origin (#61)
  "Doomer":  DoomerStatus,          // a doomsday weapon aimed at ToBoard (#63)
  "TimeChecks": [ TimeCheck ],      // round-trip probes, out and echoed back
  "IPMessages": [ IPMessage ],      // interplanetary mail for ToBoard's barons
  "LeagueConfig": LeagueConfig,     // coordinator's ruleset (signed)
  "LeagueNodes": [ LeagueNode ],    // coordinator's roster (signed, #64)
  "Reset": LeagueReset,             // coordinator's new-season order (signed, #65)
  "Seq": 7,                         // per-sender sequence, for replay detection (#53)
  "Signature": "base64"             // ed25519 over the coordinator-authored parts
}
```

Component types:

```json
RemoteScore   { "Empire": "Asgard", "NetWorth": 1281, "Land": 100, "Score": 940,
                "Protected": true }
                // Protected = still under New Realm Protection, so the boards
                // that read this leave it off their target lists. Absent in a
                // packet written before the field existed, which reads as
                // unprotected. Advisory: the target board still refuses an
                // arriving strike on its own authority.

RemoteAttack  { "ID": 1, "FromBoard": "AlphaBBS", "TargetEmpire": "Victim",
                "Offense": 150000, "Contributors": [ Contribution ] }
                // TargetEmpire "" = whole planet (strongest defender)

Contribution  { "Owner": "andy", "Offense": 100000 }   // for splitting spoils

AttackResult  { "ID": 1, "TargetBoard": "BravoBBS", "TargetEmpire": "Victim",
                "LandTaken": 12, "Won": true }

TimeCheck     { "From": "AlphaBBS", "To": "BravoBBS",
                "Sent": "2026-07-04T18:02:11+10:00" }
                // From is the only board that reads the elapsed time; To echoes
                // the record back UNCHANGED. RFC3339, so the two clocks may sit
                // in different zones.

IPMessage     { "FromBoard": "AlphaBBS", "FromEmpire": "Asgard",
                "ToCoordinator": false, "ToEmpire": "",
                "When": "07/04/2026  18:02:11", "Body": "..." }
                // neither To* set = every realm on ToBoard reads it
```

Processing (`World.ApplyPacket`): a packet addressed to this board (or a
broadcast) is applied — scores import into `RemoteBoards`; attacks and terror
ops resolve and produce results returned to the origin; incoming results post to
the planetary bulletin; recon requests are answered from live figures; IP
messages are delivered to the mailboxes they name; and a time-check naming this
board is echoed back untouched, while one of our own coming home is folded into
`World.TravelTimes`. A packet addressed to a different board is left in place for
the transport to route onward.

A reply packet is written whenever it carries anything at all
(`Packet.HasPayload`) — an answer that is only an echoed probe, or only recon
reports, still has to go out.

## Node list: `ibnodes.dat`

Plain text, one board per six-line block, blank line between blocks (BRE's
`BRNODES.DAT` layout). Loaded at startup into `World.LeagueNodes`.

```
1              node number (1 = League Coordinator)
Avalon         board / planet name
363/277        network address
Orlando        city
FL             state / province
USA            country
```

## Board config: `BBS.CFG`

Optional, seven lines (`store.ParseBoardConfig`): sysop name, planet name, node
address, inbound-file dir, netmail dir, league number, mailer. The clone's
own `config.json` (`BoardID`, `IBBS`, `InboundDir`, `OutboundDir`) is the
authoritative runtime config; the BBS.CFG parser exists for importing a board's
identity from an existing BRE-style setup.

## Code map

- `internal/game/ibbs.go` — packet types, group attacks, resolution, scores.
- `internal/store/ibbs.go` — `WriteOutbox`, `ReadInbound`, `RunPlanetary`.
- `internal/store/league.go` — `ParseNodeList`, `ParseBoardConfig`.
- `scripts/ibbs-smoke.sh` — end-to-end 3-board exchange with the real binary.

## Verified against real BRE (2026-07-22)

The clone's inter-BBS model was cross-checked by running a two-board **BRE
0.988** InterBBS league (coordinator + member) locally under dosemu2 and
comparing its exchange against IB's Option A packet. Findings and the remaining
gaps are tracked in #60.

**Local two-board setup (no mailer).** BRE's own "Local InterBBS Setup" runs
several boards on one machine with no front-end mailer: each board's *inbound*
directory points directly at the *other* board's `\OUTBOUND`, and a `ROUTE.CFG`
forms a circle (`ROUTE * 2` on board 1, `ROUTE * 1` on board 2). `BBS.CFG` line
4 is the inbound-file dir, line 5 the outbound/netmail dir. This works because
the boards share a disk. The clone's `InboundDir`/`OutboundDir` are the direct
analogue.

**Exchange commands.** BRE runs maintenance from the command line:
`BRE PLANETARY` (read inbound, then write outbound — the equivalent of
`immortal-barons -planetary`), split into `BRE INBOUND` (read + route) and
`BRE OUTBOUND` (write). A league-wide reset by the coordinator propagates to
members: a member's next `PLANETARY` wipes and rebuilds its world from the
coordinator's reset packet.

**Transport format (what IB deliberately does *not* copy).** BRE moves files as
FidoNet FTS-0001 netmail (`N.msg`, from/to user "BRE System", subject = the
attached file's absolute path, file-attach attribute, `INTL` kludge) carrying a
compressed binary data packet named `<league>b<from><to>.<seq>` (BRE's own
`fd`-escape packer, ~90% ratio) plus a broadcast `brnodes.<league>` node list.
IB uses plain JSON `.brp` instead — an intentional clean-room simplification.
The transport differs; the *contents* are what fidelity is judged on.

**Packet contents (BRE's PLANETARY stages).** Local recon info; global recon
requests; routing data; node list; group attacks; individual IP-attack info;
Gooie-Kablooie (Doomer Kaboomer) status; scores/news; coordinator config +
reset. IB currently carries scores, group attacks, terror ops, results, and the
`LeagueConfig` ruleset broadcast; the recon exchange, individual interplanetary
attacks, cross-board Doomer status, node-list broadcast, and league-wide reset
are the open gaps under #60.

**Config field set.** BRE's coordinator config editor (the `LeagueConfig`
analogue) marks league-wide fields with `*` = "InterBBS Setting Only": Attack /
Terrorist Costs, Individual / Group / Terrorist Attacks per day, Bombings per
day, Days for Lost Attacks, Gooie Kablooies, Bombing / Missile Operations, Local
Attacks, Local Attack Scoring, Dupe Checking — alongside the non-`*` general
settings (turns/day, protection, land, interest, tax, region caps, maintenance /
trade-deal / region costs, attack damage / rewards).
