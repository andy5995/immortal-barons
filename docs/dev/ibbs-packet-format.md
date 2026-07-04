# Inter-BBS Packet & Node-List Formats (developer reference)

This is the on-disk contract for Immortal Barons' inter-BBS ("interplanetary")
play. It is a developer reference, not a player/sysop doc — the sysop guide
lives in the translatable help content (`internal/help/content/interbbs/sysop.md`).

The clone is an independent reimplementation and does **not** use BRE's binary
packet format. It defines its own JSON packets. The node list reuses BRE's
plain-text `BRNODES.DAT` layout (under the clone's own filename `ibnodes.dat`).

## Transport model (Option A, file-drop)

The game only reads and writes packet files in two directories, set per board in
`config.json`:

- `InboundDir` — packets from other boards arrive here.
- `OutboundDir` — the game writes packets for other boards here.

Moving files between boards is external to the game (a mailer, a sync tool, scp,
a shared mount). `RunPlanetary` (`barons-door -planetary`, also folded into
`-maint` when `IBBS` is on) reads and applies inbound packets, launches due
group attacks, exports this board's scores, and writes the outbox.

## Packet files (`*.brp`)

Each packet is one JSON file. Filename:
`<from>-to-<to>-<date>-<n>.brp`, where `<to>` is `all` for a broadcast. A
transport that fans out must copy a broadcast to every other board's inbound.

The JSON is `game.Packet`:

```json
{
  "FromBoard": "AlphaBBS",
  "ToBoard": "",                 // "" = broadcast to the whole league
  "Date": "2026-07-04",          // ISO; used for observed travel-time display
  "Scores":  [ RemoteScore ],    // score share (feeds IP scores / attack targets)
  "Attacks": [ RemoteAttack ],   // strikes landing on ToBoard
  "Results": [ AttackResult ]    // outcomes returning to the origin
}
```

Component types:

```json
RemoteScore   { "Empire": "Asgard", "NetWorth": 1281, "Land": 100 }

RemoteAttack  { "ID": 1, "FromBoard": "AlphaBBS", "TargetEmpire": "Victim",
                "Offense": 150000, "Contributors": [ Contribution ] }
                // TargetEmpire "" = whole planet (strongest defender)

Contribution  { "Owner": "andy", "Offense": 100000 }   // for splitting spoils

AttackResult  { "ID": 1, "TargetBoard": "BravoBBS", "TargetEmpire": "Victim",
                "LandTaken": 12, "Won": true }
```

Processing (`World.ApplyPacket`): a packet addressed to this board (or a
broadcast) is applied — scores import into `RemoteBoards`; attacks resolve
against the named empire (offense vs defense, a capped land bite) and produce an
`AttackResult` returned to the origin; incoming results post to the planetary
bulletin. A packet addressed to a different board is left in place for the
transport to route onward.

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
