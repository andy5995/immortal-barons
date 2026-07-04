# Inter-BBS (Interplanetary) Play — Design Spec (DRAFT)

**Status:** draft for Andy's review. The transport choice (§3) is the one
decision that needs Andy before implementation; everything else follows from
it. Drafted from the original BRE data — `docs/{brnodes,bbs,route}.sam` and
BRE.DOC's "Interplanetary Operations" section — cross-referenced with the
existing code.

This spec covers the 6 remaining menu stubs, all of which are inter-BBS
features: Create/Join Group Attack, Terrorist Ops (interplanetary special
ops), Travel Times, Spy Database, and Modify League Diplomacy.

## 1. What BRE actually does (verified from source)

BRE's inter-BBS is **asynchronous store-and-forward**. Each BBS ("planet") is
a node in a **league**. Play is local; between-planet actions are queued into
**outbound packets**, moved to other boards by an external transport, and
processed on the remote board's next maintenance run. Results return in later
packets. The latency is a game mechanic — group attacks have a departure time,
and "Travel Times" reports the round-trip delay.

Config files (formats fully specified by the `.sam` samples):

- **BRNODES.DAT** — the league node list. Per node: number, planet name,
  FidoNet net/node address, city, state, country. Maintained by the League
  Coordinator and shared to all members.
- **BBS.CFG** — 7 lines: sysop name, planet name (in-game identity), node
  address, inbound-file dir, netmail dir, **league number (1–999)**, mailer
  type (FrontDoor / Binkley / D'Bridge / Intermail / … / NONE).
- **ROUTE.CFG** — optional routing overrides: `ROUTE <from> <to>`, `CRASH`,
  `HOLD`, `NORMAL` (`*` = all nodes).

Maintenance command **`BRE PLANETARY`** reads inbound packets and writes
outbound ones; it can run several times a day. `INBOUND` / `OUTBOUND` are
sub-steps; `LASTPACKET` tracks the last packet processed per board.

Gameplay semantics:

- **Create Group Attack** — build a force aimed at a whole planet or a single
  barony; set a departure time; teammates on your planet may **join** until it
  leaves; returns split by contribution.
- **Join Group Attack** — add forces to a waiting party.
- **Indiv. Attack Force** — solo strike on one remote empire; 2× returns,
  single target only, no whole-planet option.
- **Terrorist / Interplanetary Special Ops** — bomb enemy food/trade markets,
  undermine investments, bomb trade routes, Nuclear/Chemical/S3-Sabre, Spy Guy.
- **Spy Database** — spy reports on remote empires (incoming group attacks,
  Gooie Kablooies) stored planet-wide, readable by all locals.
- **Travel Times** — approximate round-trip latency per node.
- **Modify League Diplomacy** — a Coordinator function to set league-level
  diplomacy stance between planets.

## 2. What already exists in the code

- `game.Config`: `BoardID`, `IBBS`, `GameLength`; `InterBBSEnabled()` gates the
  interplanetary menus (`ibbsHidden` in the menu tree — all 6 stubs are hidden
  in single-board play today).
- `game.RemoteBoard{BoardID, Date, Scores []RemoteScore}` and
  `World.RemoteBoards` + `ImportBoard()` — a score-sharing packet already
  round-trips (tested in `ibbs_test.go`). This is the seed of the packet model.
- `internal/door` parses dropfiles (per-caller identity, node number).

So the transport *concept* (import/export a board snapshot) is proven; what's
missing is the richer packet types (attacks, ops, spy reports) and the async
processing flow.

## 3. THE decision for Andy: transport

The clone is an **independent reimplementation** — it will not interoperate
with real BRE nodes, so it does **not** need BRE's binary packet format or
FidoNet framing. It only needs to preserve the **semantics** (async
store-and-forward, latency, league membership). Two options:

**Option A — Transport-agnostic file-drop (recommended).** The game reads
inbound packet files from a configured `inbound/` dir and writes outbound
packet files to `outbound/<board>/`, in the clone's own format (JSON). *How*
those files move between boards is the operator's concern — a sync tool, scp,
a shared mount, or Synchronet's own FidoNet if they want it. This mirrors
BRE's model exactly (the mailer moved the files; the game just read/wrote
dirs), needs **no FidoNet dependency**, and is trivially testable in-memory
(write packet from World A, read into World B). It also extends the existing
`RemoteBoard` import cleanly.

**Option B — FidoNet-compatible.** Emit real FidoNet `.pkt`/NetMail so a
Synchronet FidoNet setup carries it. Closer to the original's plumbing, but
adds a heavy legacy dependency and setup burden for something the clone doesn't
need for correctness. Not recommended unless interop with a FidoNet mesh is a
goal.

The rest of this spec assumes **Option A**.

## 4. Packet model (Option A)

A packet is a JSON file carrying a typed list of items from one board to
another (or broadcast to the league):

```
type Packet struct {
    FromBoard string
    ToBoard   string   // "" = broadcast to whole league
    Date      string   // ISO; used for travel-time / ordering
    Scores    []RemoteScore   // existing score share (kept)
    Attacks   []RemoteAttack  // group/individual strikes landing on ToBoard
    Ops       []RemoteOp      // interplanetary special/covert ops
    SpyReports[]SpyReport     // for the Spy Database
    Diplo     []LeagueDiplo   // coordinator league-diplomacy updates
}
```

Each type names the target empire by `(BoardID, EmpireName)`. Processing on
the receiving board applies the effect during maintenance and may enqueue a
**result** packet back to the origin (battle returns, spy findings).

## 5. Feature designs (all unit-testable with in-memory multi-World fixtures)

- **Group Attack** — a `PendingGroupAttack{Target, DepartDay, Contributors
  map[owner]forces}` lives on the World until `DepartDay`; Join adds to
  `Contributors`; at departure, maintenance serializes it into an outbound
  `RemoteAttack`. The receiving board resolves it against the target empire
  (reuse `combat.go`), splits returns by contribution, enqueues a result.
- **Indiv. Attack Force** — same path, single contributor, 2× returns.
- **Terrorist / IP Special Ops** — outbound `RemoteOp`; receiving board applies
  the effect (reuse `covert.go` / `specials.go` resolution) and enqueues a
  result. Spy Guy produces `SpyReport`s.
- **Spy Database** — accumulate received `SpyReport`s in
  `World.SpyDatabase []SpyReport`; the menu just lists them.
- **Travel Times** — computed from the node list + last-packet dates
  (`LASTPACKET` equivalent already implied by `RemoteBoard.Date`); pure display.
- **Modify League Diplomacy** — Coordinator-only; edits league diplo state,
  broadcast via `LeagueDiplo` packet items.

## 6. Config / league membership

Add to `game.Config` (or a sibling league config, parsed from a `BBS.CFG`-style
file): `LeagueNumber int`, `InboundDir`, `OutboundDir`, and a parsed node list
(`[]LeagueNode{Number, PlanetName, Address, City, State, Country}` from a
`BRNODES`-style file). Keep it small; the coordinator/sysop config screen
(already stubbed) grows to edit these.

## 7. Testing strategy (no live Synchronet needed to build/verify)

Unit + integration tests run **two or more `World`s in-process**, passing
`Packet`s between them via an in-memory transport, asserting: a group attack
launched on World A lands on the right empire in World B; returns split
correctly; spy reports populate the database; travel-time display is correct.
Live Synchronet is only for final real-world validation of the *file
movement*, not the game logic.

## 8. Open questions for Andy

1. **Transport: Option A (file-drop, recommended) or B (FidoNet)?** — blocks
   implementation.
2. Scope: build all 6 at once, or start with Group Attack (the core) and layer
   the rest?
3. Do we replicate Gooie Kablooie / SDI here too (currently separate stubs), or
   keep this spec to the 6 menu stubs?
