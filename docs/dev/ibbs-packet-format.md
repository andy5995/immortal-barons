# Inter-BBS Packet & Node-List Formats (developer reference)

The sysop's side of inter-BBS ("interplanetary") play is `docs/inter-bbs.md`.

The clone is an independent reimplementation and does **not** use BRE's binary
packet format. It defines its own JSON packets. The node list reuses BRE's
plain-text `BRNODES.DAT` layout (under the clone's own filename `ibnodes.dat`).

## Transport model (Option A, file-drop)

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

Moving files between boards is external to the game (a mailer, a sync tool, scp,
a shared mount). `RunPlanetary` (`immortal-barons -planetary`, also folded into
`-maint` when `IBBS` is on) reads and applies inbound packets, launches due
group attacks, exports this board's scores, and writes the outbox.

`barons-ftn` is the optional FTN adapter. It remains outside the game process:
it reads the existing board and roster files, renames each outbound
`.brp` into that directory's `fido/` child, then creates an FTS-0001 Type-2
file-attach `.msg` with exclusive creation. An unaddressed mesh broadcast is
fanned out with real copies to one distinct attachment and message per other
roster node; an addressed packet is sent to its routed next hop. No hard-link
support is required. Its only settings are in `ftn.cfg`. `WriteOutbox` writes
complete JSON, including the board signature when configured, under a
non-`.brp` temporary name and atomically publishes it by renaming it, so the
adapter needs no game lock. Concurrent adapter processes serialize through
their own `barons-ftn.lock`, which the game never takes.

## Packet files (`*.brp`)

Each packet is one JSON file. Modern packet filenames are
`[L<nnn>-]<from-node>-<sequence>-<to-node>.brp`; the three identity numbers use
base 36 to keep the name short enough for an absolute pathname in an FTN Type-2
subject. That budget is why a roster node number is capped at 999 (#180): every
extra digit lengthens every filename the board sends, and boards have been
measured running with three bytes of subject to spare. The sequence has a fixed width so a directory scan sees each sender's
packets in order. For example, `L042-2-000000000001z-3.brp` is league 42,
origin node 2, sequence 71, final destination node 3. A zero destination is a
broadcast. The `L<nnn>` prefix is present when the league number is set. A
modern packet at league 0 gets a short digest of its origin board before those
three numbers, preventing equal node numbers in two leagues sharing a directory
from colliding. A legacy packet without a stable origin node and sequence gets
a deterministic 128-bit content digest instead. A transport that fans out must
copy a broadcast to every other board's inbound. `barons-ftn` leaves the first
broadcast attachment's name unchanged and appends a zero-based, base-36 copy
index directly to the stem for each additional recipient (`packet0.brp`,
`packet1.brp`, ...). The dense index is transport-only: sparse roster node
numbers do not lengthen the attachment name, and packet identity remains in the
JSON rather than the copy suffix.

**The extension is matched without regard to case** (#179). FTN transport hands
files over in upper case routinely — 8.3-era software and several mailers do it,
and a league carried over FidoNet can meet one at any hop — so a `.BRP` delivered
into an inbound directory is read like any other packet. An exact match against
the lowercase name left the file sitting there unread, and unreported: not
applied, not refused, not counted as skipped.

The short name leaves room for FTN `.msg` transports, whose Subject must carry
the attachment pathname in at most 71 bytes. That byte budget is an FTN
constraint, not a restriction on the packet format or on other transports; it
comes from the stored-message header described in
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
did not, and whatever order #178 settles on diverges from nothing the original
decided. Note that the fixed-width sequence above is justified by scan order; if #178 removes that dependence, the fixed width keeps its other job (a short, collision-free name) and loses that one.

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
                "Protected": true, "OwnerHash": "3f6a1c09b2d84e57" }
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
`OutboundDirs`, and `Lottery` — live here rather than in `config.json`, and are marked
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
```

`Lottery` is the only rule in the file, and the exception
`TestEveryGameRuleIsBroadcast` names: BRE keeps the same switch in the
per-install `RESOURCE.DAT`, so it is each sysop's, not the league's. It takes
yes/no, on/off or true/false, and an unreadable value leaves the default (on)
alone.

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
