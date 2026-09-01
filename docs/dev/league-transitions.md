# League transitions

**Status: proposal. None of this is built.** It records a design put to the
project in
[#230](https://github.com/andy5995/immortal-barons/issues/230#issuecomment-5445820553)
after the v0.0.8 transport change, so the reasoning survives outside a comment
thread. Where it describes what IB does today, that is marked and sourced; the
rest is a target, and the numbers, names and file formats in it are not fixed.

## Protocol bumps: drain first

**DECIDED 2026-08-31 (#229), and the one part of this file that is settled
policy rather than proposal.**

When `game.Protocol` moves, a league does not upgrade board by board. It closes
the game, lets every board finish sending what it already has queued, and only
then switches to the new release together.

The reason is how a held packet comes back. `SpeaksOurProtocol` is exact
equality, and a packet whose protocol a build does not speak is held and released
only when the READER itself comes to speak the number the packet already carries.
That happens when the reader upgrades and never when the sender does — so in a
staggered upgrade the board that moves **first** holds everything still arriving
from the boards behind it, and nothing ever releases it. Its `held/` files
survive; what they carried does not reach the game. The board that moves **last**
is unaffected, because its backlog becomes readable the moment it moves. The cost
lands on whoever moves first, which is the wrong way round.

Draining the queues before anyone upgrades means no packet is ever in flight
across the boundary, so the asymmetry has nothing to bite on.

#229 proposed the other fix — a table of older protocols the newer build could
read, with a migration per entry. The policy removes the need for it, and the
issue was closed unbuilt. The distinction it drew is still worth keeping in mind
when authoring a bump, because it says which kind of change is even convertible:
an **additive** field is (the old packet leaves it unset and its signature still
verifies under the older shape), while **renaming or changing the meaning of a
signed field** is not (the origin signature was taken over the old rendering, so
converting the packet destroys the proof of who sent it).

This is narrower than the Coordinated rollout proposed below, and it does not
depend on any of it: it needs no `MinBoardVersion` gate, no signed cutover
commit, and no new tooling. It is what a league should do today.

**v0.0.9 is the first release this applies to** — `Protocol` moved to 2 for
`RemoteScore.FormerName` (#235) and the S3-Sabre's dial. Its release notes
should say so.

## The problem it addresses

A release can break a league in two different ways, and only one of them is
loud.

The **transport container** is the loud one. A board on v0.0.7 aborts its whole
inbound run on the first ZIP bundle it meets, so a single board turning bundling
on early stops another board's mail entirely. v0.0.8 answers this by emitting
plain packets unless a sysop sets `Bundled Yes`, which makes the safe state the
default — but it is still a default, and the dangerous setting is one line of
`ftn.cfg` away with nothing checking whether the league can read it.

The **quiet** one is gameplay. An interplanetary attack is resolved on the
target board, so two boards running different combat math give different answers
to the same attack and nothing announces it. The wire is byte-identical, so no
protocol check fires.

The proposal's claim is that both are the same problem — a league-wide change
applied board by board — and that the league already has a signed instrument for
saying "we have all moved": the Coordinator's `MinBoardVersion`.

## Release classes

Every release is classified, and the class states what a league has to do.

| Class | Policy |
|---|---|
| Rolling | No gameplay or packet-semantic difference. Boards upgrade independently. |
| Coordinated | Play freezes league-wide while boards upgrade; the game continues afterwards. |
| Reset | The release begins a new season and rejects everything from the previous generation. |

v0.0.8 would be Coordinated: it changed both interplanetary combat resolution
and the FTN container.

## Tie bundling to the signed minimum version

The core of the proposal. Bundled output stops being a per-link opinion and
becomes a consequence of what the Coordinator has committed to:

- while `MinBoardVersion` is blank or below the release that introduced bundles,
  `barons-ftn --out` emits raw packets, whatever `ftn.cfg` says;
- once the Coordinator raises it, bundled output becomes available on its own;
- `Raw` stays as an explicit per-link override, for recovering one broken peer.

The property this buys: **omitting a step costs only coalescing.** It cannot
disable an old board. The Coordinator's signed league-config packet is the
cutover commit — before it, both versions understand the wire; after it, every
acknowledged board understands bundles.

**The premise of that last clause does not hold, and the proposal needs
reworking around it (andy5995, 2026-08-31).** "Every acknowledged board
understands bundles" assumes a board's game version tells you whether it can
unwrap one. It does not: unwrapping is `barons-ftn -in`, the helper is optional,
and a board reading `.brp` straight out of its mailer's directory is a supported
setup that `docs/inter-bbs.md` documents. **Some boards will never run the
helper**, whatever version of the game they are on — so raising
`MinBoardVersion` can never be sufficient authority to start bundling toward
them, and a design that flips the default on that signal would break exactly the
boards #230 is about.

What survives is the direction of the gate, not its input. Any automatic flip
has to key on *whether the peer runs the helper*, which is a property of the
LINK and not of the league — which is what `ftn.cfg`'s per-link `Raw`/`Bundled`
already records. So the honest rule may simply be the one in force today:
**bundling is per-link, indefinitely, and the sysop must know their peer.** That
is less satisfying than a signed cutover, but it is true of the deployments that
exist.

Worth stating plainly what is given up by never bundling toward such a peer,
since it is smaller than this section implies: the routing manifest (a bundle
carries per-packet `Route` and `Covered`, so a forwarding board skips peers a
broadcast has already reached; a raw packet rebuilds `Route` from its own
`FromNode` and knows nothing about coverage), the loop check that reads the same
`Route`, BSO coalescing (`appendBSOBundle` merges into a bundle already queued
under the peer's `.bsy`), and fewer files. None of it is gameplay — every packet
still arrives, applies, and is replay-checked. In a **star** topology routed
through the Coordinator the manifest buys almost nothing, because there is only
one path; it earns its keep in a mesh.

`--status` should say which state it is in, in a line a sysop can paste:

```text
Compatibility output: raw; league minimum is below v0.0.8
Bundled output enabled; league minimum is v0.0.8
```

`-league-check` on the Coordinator should report the transition as a checklist
rather than leaving the sysop to compare `BBSINFO.LST` by eye:

```text
UPGRADE  v0.0.8: 5 of 6 boards confirmed
WAITING  Foobar BBS: last reported v0.0.7
OUTPUT   Raw compatibility mode; bundles cannot be emitted yet
NEXT     Run -bbsinfo after Foobar's confirmation
```

A dedicated `-league-upgrade-status <version>` would make it first-class and
refuse the commit while any roster node is old or unknown.

## Rollout of a Coordinated release

1. The Coordinator announces a UTC freeze window.
2. At the freeze, boards stop new logins. Callers already in finish the
   transaction they are in and are returned to the BBS.
3. Scheduled transport keeps running — inbound, planetary processing, outbound,
   tosser, mailer — so pre-freeze traffic settles.
4. Each sysop installs the release, separates the mailer and game directories,
   and schedules `barons-ftn --in`.
5. Each sysop runs `barons-ftn --in`, `barons-ftn --status`,
   `immortal-barons -league-check` and `immortal-barons -version`, and reports
   the exact version and a clean check.
6. With output still raw, the league runs one complete exchange. The Coordinator
   verifies every roster board reports the new version and has confirmed `--in`
   is installed.
7. The Coordinator raises `MinBoardVersion` and broadcasts the league config.
   That signed packet is what permits bundled output.
8. Play stays frozen for one further exchange. Check `--status`,
   `LASTPACKET.LST`, `BBSINFO.LST` and travel-time replies.
9. The Coordinator announces a UTC reopening time. A node reopens only after it
   has received the commit, is running the new version, and that time has
   passed.

**The window ending must never reopen a node by itself.** Any failure leaves the
league frozen, which is recoverable; a league half-reopened on divergent
versions is not.

## Late packets

The signed commit is the boundary. A packet is judged by when it *arrives*, not
by when it was sent, and the transport container is not evidence of version: a
delayed raw packet from a new board is valid, and the signed inner `Version` and
`Protocol` decide.

| Arrives after the commit | Action |
|---|---|
| Accepted protocol, version at or above the minimum | Apply normally, raw or bundled alike |
| Accepted protocol, version below the minimum or absent | Do not apply; archive the exact bytes; notify the origin and the Coordinator |
| Protocol newer than this board speaks | Hold; this board missed an upgrade; notify its sysop and the Coordinator |
| Protocol older than the committed transition | Archive as obsolete; notify the origin and the Coordinator — do not hold forever |
| From the wrong pre-reset generation | Reject before application and archive |

Rejected packets belong in a bounded `rejected/version` area carrying sender,
sequence, stated version and transition ID. They are **not** queued for later
application: the sender regenerates anything that mattered under a new sequence
once it has upgraded.

## Reset releases and the season problem

A reset does not currently protect the new season from delayed packets. An
ordinary packet carries no season, and `ResetForNewSeason` deliberately
preserves Outbox and Transit, so a packet from the old generation can arrive
after the reset and be applied as though it belonged.

The durable fix is a signed league-generation value on every packet, rejected
before application when it differs. Until that exists, a reset has to either
prove and clear every old inbound, outbound, helper and mailer queue before
reopening, or take a new League Number for the new season so late packets are
unambiguously foreign. The second is harder to misuse; consuming league numbers
for it is the cost.

## What IB does today

Grounded in the current tree, so the gap is visible:

- **Bundling is a config posture, not a league state.** `ftn.cfg` takes a
  board-wide `Bundled` and per-link `Raw`/`Bundled` modifiers; `rawFor` resolves
  them, defaulting to raw (`internal/ftn/config.go`). Nothing consults
  `MinBoardVersion`.
- **`MinBoardVersion` gates packets, and only packets.** A packet from a board
  below the minimum is refused by `bounceVersion`, which returns a payload-free
  `Notice` to the origin and files a sysop note (`internal/game/ibbs.go:502`).
  IB refuses only the offending board rather than stopping the whole league,
  which is a deliberate divergence recorded in `docs/mechanics-reference.md`.
- **The rejected bytes are not kept**, and the Coordinator is not told. The
  origin gets the notice; nothing else records that it happened.
- **There is no release class.** Nothing in the game or the docs distinguishes a
  release a league may take one board at a time from one it may not, and
  `docs/dev/releasing.md` decides `Protocol` and `MinBoardVersion` per release
  by hand.
- **Freeze is manual.** No command stops logins league-wide or holds a node shut
  until a commit arrives.

## Open questions

- Which release the bundling gate keys on. Hard-coding "0.0.8" in the emission
  path dates the code; a capability recorded in the league config does not, but
  it is a wire field and so a protocol question.
- Whether a board should refuse to emit bundles when it cannot verify the
  Coordinator's key, which is where every league starts.
- What a bounded `rejected/version` area costs to keep, and who prunes it.
- Whether the generation value can ride as an omitempty field — a board that
  does not know it must still see byte-identical bytes for signature checks, so
  the answer decides whether it is a `Protocol` bump.
