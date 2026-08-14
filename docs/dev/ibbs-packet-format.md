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
- `OutboundDirs` — per-neighbour override of `OutboundDir`, keyed by roster node
  number (`Config.OutboundLink`). Only a board that HOSTs others needs any.

They are stored in `bbs.cfg`, not `config.json` — see below.

Both are resolved against `DataDir` unless absolute (`Config.Inbound()` /
`Config.Outbound()`) — a door is launched from whatever working directory the
BBS chooses, so a CWD-relative path lands somewhere different on every call.

Moving files between boards is external to the game (a mailer, a sync tool, scp,
a shared mount). `RunPlanetary` (`immortal-barons -planetary`, also folded into
`-maint` when `IBBS` is on) reads and applies inbound packets, launches due
group attacks, exports this board's scores, and writes the outbox.

## Packet files (`*.brp`)

Each packet is one JSON file. Filename:
`[L<nnn>-]<from>-to-<to>-<date>-<seq>-<n>.brp`, where `<to>` is `all` for a
broadcast and the `L<nnn>` prefix is the league number when one is set. A
transport that fans out must copy a broadcast to every other board's inbound.

`<to>` is the packet's FINAL destination, not the board the file is handed to.
Where the file is written is the routing decision (`World.NextHop`); the name
records who it is ultimately for, which is what makes a directory of packets
readable to a sysop chasing one.

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
  "Annihilator": AnnihilatorStatus,          // a doomsday weapon aimed at ToBoard (#63)
  "TimeChecks": [ TimeCheck ],      // round-trip probes, out and echoed back
  "IPMessages": [ IPMessage ],      // interplanetary mail for ToBoard's barons
  "LeagueConfig": LeagueConfig,     // coordinator's ruleset (signed)
  "LeagueNodes": [ LeagueNode ],    // coordinator's roster (signed, #64)
  "Reset": LeagueReset,             // coordinator's new-season order (signed, #65)
  "Seq": 7,                         // per-sender sequence, for replay detection (#53)
  "Signature": "base64",            // ed25519 over the coordinator-authored parts
  "League": 42,                     // league number; a board in two leagues ignores the other's
  "Hops": 0,                        // boards that have forwarded this; capped by MaxPacketHops
  "Epoch": 3,                       // sender's World.Epoch, so a packet a reset has outlived is
                                     // recognised as stale rather than applied (#104). 0 = sender
                                     // predates the field, trusted rather than rejected.
  "FromNode": 2,                    // sender's roster node number, preferred over FromBoard for
  "ToNode": 1                       // identity (auth, addressing, the Coordinator check, #105).
                                     // 0 = unaddressed (ToNode) or no roster yet (FromNode);
                                     // FromBoard/ToBoard are the fallback either way.
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
                "LandTaken": 12, "Won": true, "Kind": "Normal Attack",
                "Survivors": [ Contribution ],
                "Outcome": "success",
                "Enemy": { "Troopers": 900, "Turrets": 150, "Tanks": 40, "Jets": 0 } }
                // Outcome is the verdict the origin reports to the baron:
                // "success" / "failure" / "notfound" / "protected". Absent in a
                // packet written before it existed, which reads as Won deciding
                // between success and failure — how that packet was resolved.
                // Enemy is what the strike destroyed, by unit type; absent
                // likewise, and an absent one reports nothing destroyed rather
                // than guessing.

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

A packet addressed to a specific board (`ToBoard`/`ToNode`) is matched by
`ToNode` first when both this board's own roster number and the packet's are
known, falling back to `ToBoard` otherwise (`World.AddressedToMe`) — the same
preference `VerifyBoardOrigin` and the Coordinator check give `FromNode` over
`FromBoard` (#105). A packet whose `Epoch` is older than this board's current
`World.Epoch` is discarded before anything else runs: it was written for a
game this board has since wiped by resetting (#104).

Processing (`World.ApplyPacket`): a packet addressed to this board (or a
broadcast) is applied — scores import into `RemoteBoards`; attacks and terror
ops resolve and produce results returned to the origin; incoming results give
each contributor their survivors, a private report and their share of the
captured land, and post a line to the planetary bulletin — unless nothing is
waiting on that result's ID, in which case the whole result is discarded rather
than paid out (the lost-forces timer has already returned the army, or this is a
duplicate); recon requests are answered from live figures; IP
messages are delivered to the mailboxes they name; and a time-check naming this
board is echoed back untouched, while one of our own coming home is folded into
`World.TravelTimes`. In a routed league (`World.Routed` — the roster carries HOST
lines, or this board has route rules) a packet addressed to a different board is
taken from the inbound directory and queued on `World.Transit`, to be written out
again on the link for its next hop (#106). In an unrouted league it is left
alone: the transport there copies every packet to every board, so it is one the
addressee already has. It is forwarded byte for byte apart from `Hops`:
its `Seq` and `Signature` belong to the board that wrote it, so a hub that
re-stamped one would be vouching for another board's orders. A packet carrying a
different league's number is left alone.

A reply packet is written whenever it carries anything at all
(`Packet.HasPayload`) — an answer that is only an echoed probe, or only recon
reports, still has to go out.

## Node list: `ibnodes.dat`

Plain text, one board per six-line block, blank line between blocks (BRE's
`BRNODES.DAT` layout). Loaded at startup into `World.LeagueNodes`.

```
1              node number (1 = League Coordinator), optionally "1 HOST 2 4"
Avalon         board / planet name
363/277        network address
Orlando        city
FL             state / province
USA            country
```

The first line may carry BRE's HOST routing: the node's own number, `HOST`, then
the numbers it forwards for. The roster is the league's routing table, and it is
signed and broadcast by the Coordinator (#64), so every board gets the tree
without any sysop editing anything.

Nothing below applies until a roster carries a HOST line or a board has an
`ibroute.cfg`. Until then a league is a mesh and the transport fans packets out,
which is what every existing board does.

## Routing overrides: `ibroute.cfg`

Optional, in the data directory (`store.ParseRouteFile`, BRE's `ROUTE.CFG`).
`ROUTE <dest|*> <via>`, `;` comments, last matching rule wins. Overrides the HOST
tree. BRE's `CRASH` / `HOLD` / `NORMAL` lines set a FidoNet mailer's send
priority and are read and ignored — with a file drop that is the transport's
business, not the game's.

## Board config: `bbs.cfg`

The per-board settings — `BoardID`, `LeagueNumber`, `InboundDir`, `OutboundDir`,
`OutboundDirs` — live here rather than in `config.json`, and are marked
`json:"-"` on `game.Config` so they cannot land in both. `config.json` is
rewritten by a Coordinator's ruleset broadcast, which is no place for settings
that describe one board's own machine.

Keyword per line, `store.LoadBoardConfig` / `SaveBoardConfig`, comments with `#`
or `;`, keywords matched case-insensitively, unknown keywords ignored:

```
BoardID       Avalon
LeagueNumber  900
Inbound       /home/bbs/ftn/in
Outbound      /home/bbs/filebox/uplink
Link 3        /home/bbs/filebox/node3
```

Not BRE's positional seven lines (sysop, planet, address, inbound, netmail dir,
league, mailer). Positional cannot express `Link` at all, and a blank field
shifts every field after it — which is what most of BRE's own InterBBS
troubleshooting section is about. IB stores no FTN address or mailer name
because it addresses nothing and writes no netmail.

`store.ParseBoardConfig` reads BRE's own positional format, wired to
`-ibbs-reset -import-bbs-cfg PATH` for a sysop converting a league they already
run. It takes the planet name, the incoming-files directory and the league
number. The sysop name, FTN address and mailer have no counterpart here, and the
netmail directory is deliberately not read as `OutboundDir` — BRE puts `.MSG`
files there, while IB's outbound holds the packets themselves.

The path is explicit rather than a scan of the data directory: `BBS.CFG` and
`bbs.cfg` are the same filename on macOS and Windows, so a scan would find the
board's own config and parse it positionally.

**Migration.** A board set up before the split has these in `config.json` and
nowhere else, so `LoadConfig` reads them back from the raw JSON before applying
`bbs.cfg` over the top; the next `SaveConfig` writes `bbs.cfg` and drops them
from `config.json`. A sysop who opens neither file sees nothing happen.

## Code map

- `internal/game/ibbs.go` — packet types, group attacks, resolution, scores.
- `internal/store/ibbs.go` — `WriteOutbox`, `ReadInbound`, `RunPlanetary`.
- `internal/game/ibbs_route.go` — `NextHop`, `ForwardPacket`, the HOST tree.
- `internal/store/league.go` — `ParseNodeList`, `ParseBoardConfig`.
- `internal/store/route.go` — `ParseRouteFile`.
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
Gooie-Kablooie (Clingy Annihilator) status; scores/news; coordinator config +
reset. IB currently carries scores, group attacks, terror ops, results, and the
`LeagueConfig` ruleset broadcast; the recon exchange, individual interplanetary
attacks, cross-board Clingy Annihilator status, node-list broadcast, and league-wide reset
are the open gaps under #60.

**Config field set.** BRE's coordinator config editor (the `LeagueConfig`
analogue) marks league-wide fields with `*` = "InterBBS Setting Only": Attack /
Terrorist Costs, Individual / Group / Terrorist Attacks per day, Bombings per
day, Days for Lost Attacks, Gooie Kablooies, Bombing / Missile Operations, Local
Attacks, Local Attack Scoring, Dupe Checking — alongside the non-`*` general
settings (turns/day, protection, land, interest, tax, region caps, maintenance /
trade-deal / region costs, attack damage / rewards).
