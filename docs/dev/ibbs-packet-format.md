# Inter-BBS Packet & Node-List Formats (developer reference)

The sysop's side of inter-BBS ("interplanetary") play is `docs/inter-bbs.md`.

The clone is an independent reimplementation and does **not** use BRE's binary
packet format. It defines its own JSON packets. The node list reuses BRE's
plain-text `BRNODES.DAT` layout (under the clone's own filename `ibnodes.dat`).

## Transport model

The game only reads and writes packet files in two directories, set per board in
the Configuration Editor and stored in `bbs.cfg` (not `config.json` — see "Board
config" below):

- `InboundDir` — packets from other boards arrive here.
- `OutboundDir` — the game writes packets for other boards here.
- `OutboundDirs` — per-neighbour override of `OutboundDir`, keyed by roster node
  number (`Config.OutboundLink`). Only a board that HOSTs others needs any.

Both are resolved against `DataDir` unless absolute (`Config.Inbound()` /
`Config.Outbound()`) — a door is launched from whatever working directory the
BBS chooses, so a CWD-relative path lands somewhere different on every call.

Moving files between boards is external to the game. `RunPlanetary`
(`immortal-barons -planetary`, also folded into `-maint` when IBBS is on) reads
and applies inbound packets, launches due group attacks, exports this board's
scores, and writes the outbox.

### Generic file handoff contract (#191)

The final `.brp` name is the commit marker (the extension is matched without
regard to case). Its existence means the file is complete, closed, and ready
for its current owner to consume. A transport must never create a final `.brp`
and then fill it in place. That rule applies to a plain filebox copier, a sync
service, `scp`, and a shared or network filesystem just as it does to
`barons-ftn`.

The ownership states are:

| State | Owner | Contract |
|---|---|---|
| Non-`.brp` temporary file in game outbound | Game | Private work in progress; transports ignore it. |
| Final `.brp` in game outbound | Transport | The game has published complete bytes; a transport may claim them. |
| Non-`.brp` claimed or staged file | Transport | Private transport work; the game ignores it. |
| Final `.brp` in game inbound | Game | Published and immutable; the transport must not change or remove it. |

The game publishes outbound packets by creating a temporary file in the
destination directory, writing and closing it, then renaming it to `.brp` on
that same filesystem. A transport may therefore scan only final `.brp` names;
it does not need the game lock merely to avoid a partial read. If multiple
transport workers can consume the same outbound directory, they must serialize
with one another or atomically rename a source to a non-`.brp` claimed name in
that directory. Only the worker that acquired the claim may queue or remove
it. It must preserve the packet bytes exactly, and retain enough state to retry
or restore the claim after a delivery failure.

Inbound publication is the mirror image:

1. Stage the complete packet under a non-`.brp` name on the destination
   filesystem. Copying to a temporary file elsewhere and then falling back to
   a cross-filesystem copy into the final name does not satisfy the contract.
2. Finish and close the staged file. A transport promising crash-durable
   delivery should also sync the file before publication and the destination
   directory after the rename, where the platform supports it.
3. Serialize publishers for this inbound directory and check the intended
   final name. If it already contains identical bytes, the arrival is a
   duplicate and the staged copy may be discarded. If it contains different
   bytes, preserve the staged copy, report a collision, and never overwrite
   either file.
4. Atomically rename the staged file to its final `.brp` name. From that point
   the game owns it.

No lock shared with the game is required for this inbound handoff. A rename
that lands before `ReadInbound` takes its directory snapshot is processed in
that run; one that lands afterwards waits intact for the next run. The atomic
name transition makes both outcomes safe. A transport still needs its own lock
or an equivalent no-replace publication primitive to keep two of its workers
from racing through the collision check.

This contract requires a filesystem whose same-directory rename has atomic,
consistent visibility to every participating process. If a remote mount cannot
provide that guarantee, use a receiver-side process to stage and rename on a
local filesystem, or schedule transport and `-planetary` so they cannot
overlap. A sidecar marker or manifest does not repair weak visibility by
itself: it adds a second file whose ordering, atomic publication, collision,
and cleanup would need another protocol.

`ReadInbound` defensively leaves an unparseable file younger than five minutes
in place and retries it later. That grace period limits damage from a transport
that writes directly to the final name; it is a heuristic, not an alternative
readiness signal. An older incomplete file is quarantined to `bad/`.

`barons-ftn` is the optional bidirectional FTN adapter. The game's directories
remain private. `-out` claims a fixed snapshot under `game.lock`, groups its
packets by next hop, and publishes one FTN handoff per hop. Attach and obox
bundles are immutable; BSO bundles may be rebuilt at the same path while the
peer's `.bsy` is held. `-in`
validates a received bundle, publishes local packets under the same game lock,
and immediately re-bundles transit. All helper processes serialize through
`barons-ftn.lock`; the lock order is always helper then game. Durable journals
under `ftn-spool` make the handoff resumable.

### FTN transport bundle

The physical `NNNNCCCC.BRP` file is a ZIP archive. It is not a new game packet
and none of its metadata is covered by a board signature. It contains:

```text
manifest.json
packets/000000/L100-2-000000000001z-3.brp
packets/000001/L100-2-0000000000020-3.brp
...
```

The version-1 manifest is:

```json
{
  "format": 1,
  "delivery": "attach | direct",
  "entries": [
    {
      "route": [2, 1],
      "covered": [2, 1, 3, 4]
    }
  ]
}
```

Every ZIP member CRC must pass before any entry is published. Readers reject
duplicate members, a packet/routing-entry count mismatch, unsafe paths,
unsupported versions, more than 10,000 entries, and more than 256 MiB expanded
data. Each manifest entry supplies transport state for the packet member at the
same ZIP order position; it does not repeat the member name. A member is decoded as
`game.Packet` to validate its shape, derive its canonical filename, and choose a
route. Its raw bytes are copied unchanged when delivered or forwarded.

`delivery` records only the distinction the receiver needs. An `attach` bundle
waits for a matching validated `.msg`; a `direct` obox or BSO bundle is processed
without one. It does not redundantly record which direct queue carried it.
`route` is the actual node trace: its last node is the
transmitting hop, and its length supplies the hop count. `covered` is present on
an unaddressed broadcast and contains every node for which a branch has already
been durably scheduled. A receiver fans out only to nodes in neither set. For a
legacy raw packet, the unchanged inner packet's existing `Hops` value is added
to the route length; it is not duplicated in the manifest. These fields are
loop controls, not authentication; the game still verifies the inner packet
against the league roster.

Coverage prevents ordinary sibling copies from cross-sending, but it is not a
distributed exactly-once protocol. Independently scheduled branches in a simple
cycle of four or more nodes can still meet and produce duplicates. Canonical
filename and exact-byte comparison distinguish that case from a conflicting
packet using the same identity.

The alias namespace is four base-36 characters for `league*1000 + transmitting
node`, followed by four characters from a persistent counter. Both numbers are
limited to 1..999. The counter is reserved before publication, advances once
per physical copy, and warns on wrap. A receiver discards this alias and
re-derives each member's canonical name from the decoded JSON; all game identity
remains in the packet.

A leading JSON object instead of ZIP is accepted as one legacy entry, allowing
receivers to be upgraded before senders. This is receive-only compatibility,
not an FTN wire format: new senders always publish ZIP, even for one packet.
New bundled output requires the receiving `barons-ftn -in`; the game itself
still reads JSON only.

## Packet files (`*.brp`)

Each game packet is one JSON file. These names are private to the game and ZIP
members; FTN sees the independent 8.3 bundle alias above. Modern filenames are
`[L<nnn>-]<from-node>-<sequence>-<to-node>.brp`; the three identity numbers use
base 36. The sequence has a fixed width so a directory scan sees each sender's
packets in order. For example, `L042-2-000000000001z-3.brp` is league 42,
origin node 2, sequence 71, final destination node 3. A zero destination is a
broadcast. The `L<nnn>` prefix is present when the league number is set. A
modern packet at league 0 gets a short digest of its origin board before those
three numbers, preventing equal node numbers in two leagues sharing a directory
from colliding. A legacy packet without a stable origin node and sequence gets
a deterministic 128-bit content digest instead. Packet identity remains in the
JSON; neither this private name nor the transport alias is authoritative.

**The extension is matched without regard to case** (#179). FTN transport hands
files over in upper case routinely — 8.3-era software and several mailers do it,
and a league carried over FidoNet can meet one at any hop — so a `.BRP` delivered
into an inbound directory is read like any other packet. An exact match against
the lowercase name left the file sitting there unread, and unreported: not
applied, not refused, not counted as skipped.

The 8.3 bundle alias leaves room for FTN `.msg` transports, whose Subject must
carry the attachment pathname in at most 71 bytes. That byte budget is an FTN
constraint, not a restriction on the packet format; it comes from the
stored-message header described in
[`ftn-standards.md`](ftn-standards.md), which is where the FTN formats and the
standards defining them are written down.

The destination number is the packet's FINAL destination, not the board the
file is handed to. Where the file is written is the routing decision
(`World.NextHop`).

The JSON is `game.Packet`. Every field is optional; one packet carries whatever
the run had to send. `game.Packet` itself is the authority — this is a reader's
map of it, not a second definition.

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
  "TradeBids":  [ IPTradeBid ],     // buy orders landing on ToBoard's market (#47)
  "TradeFills": [ IPTradeFill ],    // their answers coming home (#47)
  "Market":  [ RemoteListing ],     // FromBoard's market, riding its scores (#47)
  "Version": "0.0.5",               // the sender's game version, for BBSINFO
  "LeagueConfig": LeagueConfig,     // coordinator's ruleset (signed)
  "LeagueNodes": [ LeagueNode ],    // coordinator's roster (signed, #64)
  "Reset": LeagueReset,             // coordinator's new-season order (signed, #65)
  "Seq": 7,                         // per-sender sequence, for replay detection (#53)
  "Signature": "base64",            // ed25519 over the coordinator-authored parts
  "BoardSig": "base64",             // ed25519 by the SENDING board over the whole packet, so
                                     // FromBoard is proven rather than claimed (#118).
                                     // boardSigningBytes zeroes exactly two fields first:
                                     // BoardSig itself and Hops, which every hub increments.
                                     // Signature IS covered, so a coordinator order cannot be
                                     // lifted out of one packet and grafted into another.
  "League": 42,                     // league number; a board in two leagues ignores the other's.
                                     // ReadInbound skips a packet only when the reader's own number
                                     // and the packet's are BOTH non-zero and differ, so 0 on either
                                     // side (an unnumbered league, or a packet predating the field)
                                     // is accepted. Two leagues sharing an inbound directory
                                     // therefore both need a number.
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

### Identity lives in the file, and BRE's lived in the name

The original derives a packet's origin from its **filename**. Its inbound
scanner (`BRE.OVR 0x03bd64`) is a wildcard directory walk, not a loop over the
roster: `FindFirst` on the mask, `while DosError = 0` (`0x1b68`) with `FindNext`
at the bottom, and inside it copies five characters starting at position 2 of
the name (`0x1b7d`), converts them to a number, bounds-checks that against the
node count at `[0x1264]`, indexes the node table by `node x 0x3e`, and deletes
the file when that entry is empty. The routine has a message for removing data
from an unknown node, which a roster loop could never reach and a name-keyed
scan reaches routinely.

IB reads origin and destination from the JSON instead (`FromNode`/`ToNode`
above), so a renamed packet still applies correctly and the name is free to be a
transport convenience. This is a deliberate divergence and the stronger design;
it is also what makes an 8.3 transport alias possible at all (#178).

**The original picks no ingestion ORDER.** `FindFirst`/`FindNext` return DOS
FAT directory-entry order — the slot each file happened to be written into, with
freed slots reused after deletions. Not alphabetical, not by node, not arrival
order. So IB's current sorted order is not fidelity: `os.ReadDir` sorts where DOS
did not, and the order #178 settles on (below) diverges from nothing the
original decided. The fixed-width sequence above is no longer justified by
scan order — #178 removed that dependence — but keeps its other job: a
short, collision-free name.

### Applying an inbound batch (#178)

`ReadInbound` stages every packet file in the directory before applying any
of them, rather than applying each one as `os.ReadDir` returns it. Base-36
encodes a packet's origin node near the front of its filename, so
alphabetical order gave the same origin first place in every batch for as
long as the roster stood — a fixed, permanent advantage on anything two
origins contest in the same run, such as a trade bid or a land claim.

Staged packets are grouped by `originKey` (`FromBoard` first, `FromNode`
as a fallback for a packet old enough to carry no board name at all — it
has to key the same way replay detection does, or one origin's own
packets can be split across two groups and reordered against each other).
Each origin's own packets stay in their existing `Seq` order within their
group: only the order *between* origins was ever the problem.

The Coordinator's group is identified by comparing a group's key against
the roster's actual Coordinator board — never by asking an individual
packet whether it *claims* to be from the Coordinator — so a forged
`FromNode: 1` buys an origin nothing. (A board with no roster loaded yet
falls back to trusting a self-declared `FromNode: 1`, the same trust
level `fromCoordinator` already uses to bootstrap — this narrows to a
one-time window before any roster exists and closes for good once one
does.)

Only the packets in that group that actually carry something only the
Coordinator may send (`LeagueConfig`, `LeagueNodes`, `Reset`, or
`Bulletins` — the same `CarriesCoordinatorOrders` check
`SignAsCoordinator`/`VerifyCoordinatorOrders` use, so there is one
definition of "league-wide state" instead of two) *and verify* against
this board's Coordinator public key are applied ahead of the rest of the
batch — not the whole group. `ExportNodeList` rebroadcasts the roster on
every planetary run of the Coordinator's board, so gating on the whole
group rather than the individual packets let an ordinary gameplay packet
(a trade bid, a land claim, a strike) riding in the same batch as that
rebroadcast inherit its priority for free on essentially every run — the
exact fixed advantage this feature exists to remove, just re-anchored
from filename order to "is the Coordinator's board". The split lands
after the *last* qualifying packet in the group's own `Seq` order, not
the first: cutting at the first would leave a *later* verified packet in
the group waiting for its shuffled turn, letting the rest of the batch
run one check behind whatever it just changed — the same failure the
carve-out exists to prevent, and cutting at the last also keeps the
whole applied-first prefix in the origin's own ascending `Seq` order, so
nothing in the deferred remainder can ever be mistaken for a replay of
what already applied. The deferred remainder, if any, takes its chances
in the shuffle exactly like any other group's packets, Coordinator's
board included when it has nothing signed and verified to offer at all.
The verification half matters because staging happens before any
signature is examined: without it, an origin could buy first-mover
priority simply by setting `LeagueNodes` on an unsigned packet, no
forged `FromNode`/`FromBoard` required.

This only does what it is meant to when the verified-orders packets
actually carry the lowest `Seq` in their group. `ExportNodeList`,
`ExportLeagueConfig`, and `ExportBulletins` all *prepend* their packet
to `Outbox` rather than appending: `StampOutbox` assigns `Seq` in
`Outbox` slice order, and the Coordinator's own player actions from
earlier in the day are already queued there by the time a scheduled
planetary run gets to these exports. Appending would give them the
*highest* `Seq` of the batch instead of the lowest, which would make the
split land after everything — the entire group, ordinary gameplay
included, exactly the bug this section starts by describing.

Every other group is applied in an order reshuffled every run, read from
`crypto/rand` and nothing derived from packet content — so no origin can
grind for a favorable position by crafting what it sends. The order
actually applied is written to the sysop's planetary log (not the
in-session report an interactive door caller sees) whenever a batch held
an actual choice between more than one origin, for auditability.

A packet that fails to parse as JSON is moved aside into `bad/`
(`BadDir`) instead of aborting the run — see "Quarantined packets" in
`docs/inter-bbs-troubleshooting.md` for the sysop-facing behavior (the grace period for
an in-flight transfer, the cap on same-named copies, and why nothing
empties the directory automatically).

### Interplanetary trading (#47): the compatibility rule

The three trading fields are **new in v0.0.5** and are the first change to this
format since boards began signing packets, so the compatibility rule matters.

**They are `omitempty`, and that is load-bearing.** The origin signature is taken
over the marshalled packet (`boardSigningBytes`), so a board too old to know a
field drops it on unmarshal and then computes a different signature. If these
fields were always emitted, EVERY packet would fail verification on an older
board. Omitting them when empty keeps every packet that carries no trading byte
identical across versions — which is every packet an old board could act on
anyway. `TestTradingFieldsAreOmittedWhenEmpty` holds that line.

What does break, unavoidably: a packet that actually carries trading will not
verify on a board older than v0.0.5. That is the honest cost of the feature, and
it degrades sensibly — the old board rejects the packet rather than
misinterpreting it, and its barons simply never see the Trading menu.

### The Coordinator payload: accepted shapes

The Coordinator's `Signature` covers a payload of its own, not the whole packet:
`FromBoard`, `Seq`, `LeagueConfig`, `LeagueNodes`, `Reset`, `Bulletins`, in that
order. `omitempty` cannot do for it what it does for the packet — the payload is
assembled from named fields, so adding one changes the bytes of every packet, and
a field left nil marshals as `null` rather than vanishing. Adding `Bulletins` did
exactly that: a Coordinator on the older build signed five fields while every
board built since verified six, and each refused the other's league orders
silently.

A receiver therefore verifies against every payload shape a released build signed,
newest first — today the six fields above, then the five without `Bulletins`. A
shorter shape is accepted only when the packet leaves every field beyond it empty:
a signature taken before `Bulletins` existed cannot have covered a bulletin set,
so accepting one for a packet that carries bulletins would apply content nothing
signed. Adding a field to the payload means appending its old length to
`payloadShapes` in `internal/game/ibbs_auth.go`.

### Upgrading a league across a protocol bump

**A league does not roll a protocol change through board by board.** It closes
the game, lets every board finish sending what it has queued, and only then
switches to the new release together. The policy, and why the hold behaviour
makes it necessary, are in
[`league-transitions.md`](league-transitions.md#protocol-bumps-drain-first).

### The Coordinator's version requirement

`LeagueConfig.MinBoardVersion` lets the Coordinator require a game version of
every board ("" = no requirement). A packet from a board below it is refused
whole, with a news line naming the board and the version it runs; `BBSINFO.LST`
marks the same board `(below vX.Y.Z)` so a Coordinator can see who is holding
the league up without waiting for a bounce.

A board that states NO version fails a set requirement. That is deliberate: it
predates boards saying so at all, which puts it below any version worth
requiring, and a board that cannot state its version cannot prove it meets the
bar.

**UNVERIFIED — how the original behaves.** It is *said* to stop the Coordinator
processing outbound traffic at all until the laggard upgrades. That comes from
recollection, not from the binary or the docs, and holding a whole league
hostage to one stale board is destructive enough that IB does not copy it on a
maybe: IB refuses only the offending board. Worth settling if anyone can read
the original's inter-BBS path.

Component types:

```json
IPTradeBid    { "ID": 12, "FromBoard": "AlphaBBS", "FromOwner": "khan",
                "FromEmpire": "Ironhold", "Seller": "Redlands", "Good": "Tank",
                "Qty": 40, "Price": 500 }
                // A BID, not a purchase. The buyer's gold left their hands when
                // this was queued; the receiving board fills it only if Seller
                // still offers Good at exactly Price, and refuses otherwise.
                // The ALLIANCE is judged on arrival too, not as the buyer saw it
                // — one broken while the packet was in transit closes the market
                // and the gold goes home, the same arrival-time rule an incoming
                // strike's New Realm Protection check follows.
IPTradeFill   { "ID": 12, "Filled": true, "Good": "Tank", "Qty": 40,
                "Gold": 0, "Reason": "" }
                // The answer. Gold is the refund for whatever did not fill (the
                // whole bid when Filled is false, the remainder on a partial
                // fill). Reason is the seller-side wording, so the buyer is told
                // why rather than just handed their money back.
RemoteListing { "Realm": "Redlands", "Good": "Tank", "Qty": 100, "Price": 500 }
                // One row of the sender's market. A SNAPSHOT: by the time a bid
                // against it lands, a packet round trip has passed and the
                // listing may be gone, smaller or repriced. That staleness is
                // the whole reason bids exist.
RemoteScore   { "Empire": "Asgard", "NetWorth": 1281, "Land": 100, "Score": 940,
                "Protected": true, "OwnerHash": "3f6a1c09b2d84e57",
                "FormerName": "Vanaheim" }
                // Protected = still under New Realm Protection, so the boards
                // that read this leave it off their target lists. Absent in a
                // packet written before the field existed, which reads as
                // unprotected. Advisory: the target board still refuses an
                // arriving strike on its own authority.
                //
                // OwnerHash feeds duplicate-user checking: the first 16 hex
                // characters of the SHA-256 of the caller's normalized BBS
                // handle. A HASH, not the handle — a scores packet lands on
                // every board in the league and is kept there, and no sysop
                // needs another board's user list to answer "is this the same
                // person". Present only while the sending board has Dupe
                // Checking on; absent otherwise and in older packets, and a
                // board that sends none releases the locks it had asserted.
                //
                // FormerName is the name this realm carried before its one
                // rename, so the receiving board can say the two rows are one
                // realm rather than a departure and an arrival. It rides in
                // every export for the rest of the realm's life, so the
                // RECEIVER bounds the news: it posts the rename only while the
                // snapshot it is replacing still held the old name. Introducing
                // it moved Protocol to 2 — it is signed, and once a realm has
                // renamed it is in every scores packet, so an older board would
                // re-marshal without it and fail the origin signature.

RemoteAttack  { "ID": 1, "FromBoard": "AlphaBBS", "TargetEmpire": "Victim",
                "Offense": 150000, "Contributors": [ Contribution ] }
                // TargetEmpire "" = whole planet (strongest defender)

Contribution  { "Owner": "andy", "Troopers": 90000, "Jets": 0, "Tanks": 1000,
                "Bombers": 0, "Tech": 12800 }
                // One baron's detachment. Tech is their Technology military
                // factor when they committed it, in 1/10000ths (10000 = x1,
                // 14000 = the 1.4 ceiling); the target board weighs the slot
                // by it, as the original's force record carries the same
                // value. Absent in a packet written before it existed, which
                // reads as x1.

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

Processing (`World.ApplyPacket`). A packet addressed to this board, or a
broadcast, is applied payload by payload:

- **Scores** import into `RemoteBoards`.
- **Attacks and terror ops** resolve, producing results returned to the origin.
- **Incoming results** give each contributor their survivors, a private report
  and their share of the captured land, and post a line to the planetary
  bulletin. If nothing is waiting on that result's ID the whole result is
  discarded rather than paid out — the lost-forces timer has already returned
  the army, or this is a duplicate.
- **Recon requests** are answered from live figures.
- **IP messages** are delivered to the mailboxes they name.
- **Time checks** naming this board are echoed back untouched; one of this
  board's own coming home is folded into `World.TravelTimes`.

A packet addressed to a *different* board depends on the league's shape:

- **Routed** (`World.Routed` — the roster carries HOST lines, or this board has
  route rules): the packet is taken from the inbound directory and queued on
  `World.Transit`, to be written out again on the link for its next hop (#106).
  It is forwarded byte for byte apart from `Hops`, because its `Seq` and
  `Signature` belong to the board that wrote it — a hub that re-stamped one
  would be vouching for another board's orders.
- **Unrouted:** the packet is left alone. The transport there copies every
  packet to every board, so the addressee already has it.

A packet carrying a different league's number is left alone.

A reply packet is written whenever it carries anything at all
(`Packet.HasPayload`) — an answer that is only an echoed probe, or only recon
reports, still has to go out.

## Node list: `ibnodes.dat`

Plain text, one board per six-line block, blank line between blocks (BRE's
`BRNODES.DAT` layout), plus an optional seventh line IB adds. Loaded at startup
into `World.LeagueNodes`.

```
1              node number, 1 to 999 (1 = League Coordinator), optionally "1 HOST 2 4"
Avalon         board / planet name
363/277        network address
Orlando        city
FL             state / province
USA            country
4e1b…8d3       OPTIONAL: that board's packet-signing public key (#118)
```

The seventh line is read by index, so a roster written without it parses
unchanged and one written with it is ignored by an older board. `BoardPublicKey`
hex-decodes the **whole trimmed line** and requires 32 bytes, so anything else
on it — a board name, a label — makes the entry decode to no key at all. That
reads as "this board has no key published", which applies its packets unchecked
rather than raising an error, so a malformed key line silently disables the
check it was meant to turn on. `-gen-board-key` prints the key alone for this
reason.

The first line may carry BRE's HOST routing: the node's own number, `HOST`, then
the numbers it forwards for. The roster is the league's routing table, and it is
signed and broadcast by the Coordinator (#64), so every board gets the tree
without any sysop editing anything.

Routing applies only once a roster carries a HOST line. Until then a league is a
mesh and the transport fans packets out, which is what every existing board
does.

BRE also let a board override the roster with its own `ROUTE.CFG`. IB read that
file until v0.0.7 and no longer does: BRE's own sample says a league whose
Coordinator keeps routing in the roster needs no such file, and three of the
file's four keywords (`CRASH`, `HOLD`, `NORMAL`) set a FidoNet mailer's send
priority, which is the transport's business rather than the game's. One routing
table, held by the Coordinator, is what remains.

## Board config: `bbs.cfg`

The per-board settings — `BoardID`, `LeagueNumber`, `InboundDir`, `OutboundDir`,
`OutboundDirs`, `Lottery`, and `PirateNews` — live here rather than in
`config.json`, and are marked
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
Lottery       yes
PirateNews    yes
```

`Lottery` and `PirateNews` are the only rules in the file, and the two
exceptions `TestEveryGameRuleIsBroadcast` names: BRE keeps both questions in the
per-install `RESOURCE.DAT`, so they are each sysop's, not the league's. They take
yes/no, on/off or true/false, and an unreadable value leaves the default (on)
alone. `PirateNews no` suppresses the news line a pirate raid posts and nothing
else — the raid, its loot, its losses and the raider's own report are unchanged.

Not BRE's positional seven lines (sysop, planet, address, inbound, netmail dir,
league, mailer). Positional cannot express `Link` at all, and a blank field
shifts every field after it — which is what most of BRE's own InterBBS
troubleshooting section is about. The game stores no mailer name or netmail
directory here. FTN addresses are already roster data in `ibnodes.dat`; the
optional `barons-ftn` adapter keeps its netmail directory and Binkley-mode
switch in the separate `ftn.cfg`.

`store.ParseBoardConfig` reads BRE's own positional format, wired to
`-ibbs-reset -import-bbs-cfg PATH` for a sysop converting a league they already
run. It takes the planet name, the incoming-files directory and the league
number. The sysop name, FTN address, netmail directory, and mailer are not
imported: the roster and optional `ftn.cfg` own those values, and BRE's netmail
directory must not become `OutboundDir` — BRE puts `.MSG` files there, while
IB's outbound holds the packets themselves.

The path is explicit rather than a scan of the data directory: `BBS.CFG` and
`bbs.cfg` are the same filename on macOS and Windows, so a scan would find the
board's own config and parse it positionally.

**Migration.** A board set up before the split has these in `config.json` and
nowhere else, so `LoadConfig` reads them back from the raw JSON before applying
`bbs.cfg` over the top; the next `SaveConfig` writes `bbs.cfg` and drops them
from `config.json`. A sysop who opens neither file sees nothing happen.

## How BRE tells another board something: `NEWS_DATA`

The original has one channel for "put this line in that planet's news", and it
is a packet type of its own. `append_news_record` (BRE.OVR 0x048a79) builds a
258-byte record — our board number, the destination board number, and a
255-byte line — looks up the `NEWS_DATA` type code and writes it out. Seven
routines use it: `create_group_attack`, `fund_gooie_kablooie`,
`launch_gooie_kablooie`, `dismantle_gooie_kablooie`, `estimate_attack_arrival`,
`show_gooie_arrival_time` and `report_suspected_cheating`. So every SpyGuy
report, and the warning that a weapon is on its way, reaches the far planet as
**planet news**, not as mail and not as a private notice.

IB carries the same thing as `Packet.News []string`, posted to `NewsToday` on
arrival. The watcher himself rides as `Packet.SpyGuys []SpyGuyDispatch`
(`{FromBoard, Days}`), which is BRE's three-byte `SPY_GUY` record — from board,
to board, days — in IB's own shape.

## Code map

- `internal/game/ibbs.go` — the packet itself: what it carries, how it is
  addressed, how an arriving one is applied. The mechanics that ride in it sit
  beside it as `ibbs_attack.go`, `ibbs_terror.go`, `ibbs_league.go`,
  `ibbs_spy.go`, `ibbs_annihilator.go` and the rest of the `ibbs_*.go` family.
- `internal/store/ibbs.go` — `WriteOutbox`, `ReadInbound`, `RunPlanetary`.
- `internal/game/ibbs_route.go` — `NextHop`, `ForwardPacket`, the HOST tree.
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
Gooie Kablooie status; scores/news; coordinator config + reset. IB currently carries scores, group attacks, terror ops, results, and the
`LeagueConfig` ruleset broadcast; the recon exchange, individual interplanetary
attacks, cross-board Gooie Kablooie status, node-list broadcast, and
league-wide reset are the open gaps under #60.

**Config field set.** BRE's coordinator config editor (the `LeagueConfig`
analogue) marks league-wide fields with `*` = "InterBBS Setting Only": Attack /
Terrorist Costs, Individual / Group / Terrorist Attacks per day, Bombings per
day, Days for Lost Attacks, Gooie Kablooies, Bombing / Missile Operations, Local
Attacks, Local Attack Scoring, Dupe Checking — alongside the non-`*` general
settings (turns/day, protection, land, interest, tax, region caps, maintenance /
trade-deal / region costs, attack damage / rewards).
