# FTN Transport with `barons-ftn`

`barons-ftn` is the boundary between Immortal Barons and an FTN mail system.
The game reads and writes its own private packet directories. The helper wraps
those packets for transport, chooses the next hop, and unwraps them after a
mailer session. Neither side has to pretend that a BBS inbound or a BinkP
outbox is an ordinary game directory.

This guide covers the operational details. For league concepts, roster `HOST`
lines, and packet authentication, begin with [Inter-BBS Leagues](inter-bbs.md).
The exact ZIP manifest and game-packet fields are in the
[developer packet-format reference](dev/ibbs-packet-format.md).

## The layers and their directories

There are three independent formats:

1. A **game packet** is one signed JSON `.brp` document. Its contents name its
   author, final destination, sequence, league, and operations.
2. A **transport bundle** is a ZIP file containing one or more unchanged game
   packets and transport-only routing metadata. Its temporary FTN name has the
   form `NNNNCCCC.BRP`.
3. A **stored-message envelope** is an optional `.msg` file telling a tosser to
   send one transport bundle as a file attach. Obox and direct BSO links do not
   create this envelope.

The same `.BRP` extension on the first two is intentional, but every file the
new helper publishes onto FTN is a ZIP transport bundle, never a raw game
packet. Its 8.3 physical name is only an alias. Packet ZIP members use the
canonical IB filename derived from their contents, and `--in` derives that name
again rather than trusting any external filename. As a receive-only migration
aid, `--in` can still recognize a raw JSON packet produced by an older helper.

Keep each owner in its own directory:

| Directory | Owner | Healthy contents |
|---|---|---|
| `bbs.cfg` `Inbound` | Immortal Barons | Unwrapped JSON packets waiting for `-planetary` |
| `bbs.cfg` `Outbound` | Immortal Barons | Complete JSON packets waiting for `--out` |
| `data/ftn-spool` | `barons-ftn` | Usually empty; journals appear while a handoff is incomplete |
| `ftn.cfg` `AttachDir` | connector/tosser | `NNNNCCCC.BRP` bundles waiting to be sent |
| `ftn.cfg` `NetmailDir` | connector/tosser | Outgoing game-owned `.msg` envelopes |
| `ftn.cfg` `InboundDir` | mailer/connector | Newly received bundles waiting for `--in` |
| an obox | connector/mailer | Bundles queued for the peer owning that outbox |
| a BSO directory | tosser/mailer/connector | `.?lo`, `.?ut`, `.bsy`, and point subdirectories |

Do not point `bbs.cfg` `Inbound` or `Outbound` directly at a BBS inbound,
filebox, obox, or BSO directory. The separation is what prevents the game,
helper, and mailer from reading or deleting the same file concurrently.

## Commands and safe event order

The helper has two modes:

```text
barons-ftn --in  -data /srv/ib/data
barons-ftn --out -data /srv/ib/data
```

With neither mode, `--out` is used for compatibility with old scheduled
commands. Supplying both is an error. `-data` defaults to `./data`, relative to
the process's working directory. It can be omitted only when the BBS scheduler
starts the command in the Immortal Barons installation directory.

The complete exchange order is:

1. Let the mailer finish its inbound session.
2. Run `barons-ftn --in` to validate and unwrap received bundles.
3. Run `immortal-barons -planetary` to apply local game packets and create new
   replies, scores, and broadcasts.
4. Run `barons-ftn --out` to claim and bundle that fixed outbound snapshot.
5. Run the tosser when using `.msg` attach links, then let the mailer send its
   obox or BSO queues.

For an hourly Unix event:

```sh
/opt/ib/barons-ftn --in  -data /srv/ib/data
/opt/ib/immortal-barons -planetary -data /srv/ib/data
/opt/ib/barons-ftn --out -data /srv/ib/data
/opt/bbs/bin/sbbsecho
/opt/bbs/bin/binkp-poll
```

There is no FTN-wide inbound semaphore. Run `--in` from a mailer post-session
event or after the receive command returns. The helper validates a complete ZIP
and all member digests before publishing anything, but that validation cannot
prove that an unrelated mailer is no longer writing the source file.

## `ftn.cfg` reference

`ftn.cfg` lives in the directory selected by `-data`. Keywords ignore case.
Unknown keywords are ignored for forward compatibility. Relative filesystem
paths are resolved beneath the data directory.

### Inbound settings

```ini
InboundDir        /var/spool/binkp/inbound
InboundNetmailDir /var/spool/binkp/inbound
OboxMeshFanout    Yes
```

- `InboundDir` is required by `--in` and names received attachments and raw
  obox/BSO bundles.
- `InboundNetmailDir` names received `.msg` envelopes. It defaults to
  `InboundDir` and normally stays the same.
- `OboxMeshFanout` defaults to `Yes`. It controls only an unaddressed broadcast
  received without an attach envelope. See [Mesh warning](#mesh-warning).

### Stored-message attach settings

```ini
NetmailDir /sbbs/fido/netmail
AttachDir  /sbbs/fido/ib-attach
Binkley    Yes
SubjectPath Absolute
```

- `NetmailDir` is where outgoing `.msg` envelopes are created. It is required
  when any peer uses `Attach`, including the compatibility default.
- `AttachDir` holds outgoing bundles for Attach and BSO links. If omitted, the
  helper uses `data/ftn-spool/attach`.
- `Binkley Yes` prefixes an attach subject with `^`; `No` writes `FLAGS KFS`.
  Both request deletion of the attachment after a successful send.
- `SubjectPath Absolute` writes the full attachment path. `Basename` writes
  only `NNNNCCCC.BRP`. Any other value is used as a literal path prefix.
  The stored-message Subject has 71 usable bytes, or 70 with the `^` prefix.

#### Keeping attach subjects short

The IB installation and data directories may be arbitrarily deep if
`AttachDir` is a short absolute path. For example:

```ini
AttachDir /var/spool/ib-attach
SubjectPath Absolute
```

This writes a Subject such as
`/var/spool/ib-attach/7PRK0001.BRP`; the installation path is not included.
A relative `AttachDir` is resolved beneath the data directory and therefore
does not provide this workaround. Create the attachment directory with
ownership and permissions that allow both `barons-ftn` and the mailer to use
it.

Do not use `/tmp` or another automatically cleaned directory for this purpose.
An attachment may remain queued across a reboot or cleanup interval, and
removing it leaves the mailer with a dangling `.msg` envelope. Use a short,
persistent spool directory instead.

`SubjectPath Basename` shortens the Subject further, to only
`7PRK0001.BRP`, but it is correct only when the mailer is independently
configured to search the same directory named by `AttachDir`. `barons-ftn`
cannot infer or configure that mailer search path. BSO flow files and oboxes do
not use a Type-2 Subject, so the 71-byte limit applies only to `Attach` links.

### Per-peer links

Each `Link` identifies a directly connected IBBS roster node:

```ini
Link 2 Attach
Link 3 Obox /var/spool/binkp/outbox/node3
Link 4 BSO /var/spool/ftn/outbound.309 Hold
```

The modes are:

- `Attach` creates a game-owned `.msg` in `NetmailDir` addressed to that next
  hop. It takes no per-link directory.
- `Obox` atomically publishes the bundle in the named peer-specific directory.
  The mailer must treat a final-name appearance as a complete file.
- `BSO` takes the destination's `.bsy`, then either merges into a compatible
  Barons bundle already named by that flavour's flow file or publishes a new
  bundle in `AttachDir` and adds a delete-after-send flow entry. The directory
  must be the exact root for that address's zone; `barons-ftn` does not guess
  domain-to-zone mappings.

BSO flavours are `Immediate`, `Continuous` (also accepted as `Crash`),
`Direct`, `Normal`, and `Hold`; `Normal` is the default. A point address uses
the standard `<net><node>.pnt/<point>.?lo` layout automatically.

If a destination has no `Link`, it uses `Attach`. Thus an old `ftn.cfg` with no
transport links continues to send every next hop through the existing `.msg`
chain. This fallback applies to addressed routing. Once any `Link` is present,
unaddressed mesh fanout uses only the explicitly listed peers; otherwise the
helper would invent graph edges and defeat a ring or partial mesh. List every
direct fanout neighbour, including one that uses `Attach`. A hub may freely mix
all three modes.

Paths in a `Link` line must not contain spaces. `NetmailDir`, `AttachDir`, and
the inbound directory settings consume the rest of their line and may contain
spaces when the operating system permits them.

## Configuration examples

### Two boards using direct oboxes

On node 1:

<!-- test-ftn-config -->
```ini
InboundDir /mystic/echomail/in
Link 2 Obox /mystic/filebox/ib_z99n1n2
```

On node 2, reverse the peer and directory:

<!-- test-ftn-config -->
```ini
InboundDir /mystic/echomail/in
Link 1 Obox /mystic/filebox/ib_z99n1n1
```

The game itself uses private paths:

```ini
Inbound  inbound
Outbound outbound
```

### Routed star with mixed links

The roster makes node 1 the hub:

```text
1 HOST 2 3 4
Hub BBS
777:10/1
...
```

The hub can use a different local handoff for every child:

```ini
InboundDir /srv/ftn/inbound
InboundNetmailDir /srv/ftn/inbound
NetmailDir /sbbs/fido/netmail
AttachDir /srv/ib/attach
Binkley Yes

Link 2 Attach
Link 3 Obox /srv/binkd/obox/node3
Link 4 BSO /srv/binkd/outbound Normal
```

A packet from node 2 to node 4 arrives inside node 2's attach. Hub `--in`
unwraps it, leaves its signed JSON bytes untouched, and publishes a new
transport bundle through node 4's BSO flow. It does not wait for the hub's next
planetary run.

### Synchronet stored-message chain

```ini
InboundDir /sbbs/fido/inbound
InboundNetmailDir /sbbs/fido/inbound
NetmailDir /sbbs/fido/netmail
AttachDir /sbbs/fido/ib-attach
Binkley Yes
SubjectPath Absolute
Link 1 Attach
```

The outbound chain is:

```text
barons-ftn --out -> .msg + NNNNCCCC.BRP -> SBBSecho -> BSO -> BinkIT
```

The receiving tosser normally leaves both the bundle and its stored-message
envelope in the configured inbound. `barons-ftn --in` verifies that the message
has the file-attach attribute, identifies Immortal Barons, names exactly one
attachment inside `InboundDir`, comes from a roster address, and is addressed
to this board. Only after all packets are delivered or forwarded does it delete
both files. Other netmail is never removed.

### Direct BSO/FLO handoff

<!-- test-ftn-config -->
```ini
InboundDir /var/spool/binkd/in
AttachDir /var/spool/ib/attach
Link 3 BSO /var/spool/binkd/outbound Normal
```

For peer `1:229/300`, the helper uses `00e5012c.bsy` and `00e5012c.flo` in the
configured BSO directory. The flow line begins with `^` and contains the full
attachment pathname, asking the mailer to delete it after success.

While it owns `00e5012c.bsy`, a later run may add its new snapshot to an
existing compatible Barons bundle referenced by that flow file. It atomically
replaces the ZIP wrapper at the same pathname; every signed packet member is
copied byte-for-byte. Exact-member deduplication makes a replay after a crash
idempotent. Entries without `^`, files outside `AttachDir`, non-Barons files,
other leagues or transmitters, and full bundles are not modified.

For point `1:229/300.4`, the paths are:

```text
00e5012c.pnt/00000004.bsy
00e5012c.pnt/00000004.flo
```

If `.bsy` already exists, that peer is normally reported busy and its durable
spool transaction remains pending. The invocation does not poll or sleep: other
peers continue, and the next scheduled `--in` or `--out` resumes pending
transactions before claiming new work. The narrowly scoped exception is an old
semaphore carrying `barons-ftn`'s own PID marker; recovery is described below.

## Bundling, names, and recovery

Every `--out` run takes one fixed snapshot under the same `game.lock` used by
Immortal Barons. Packets written after that claim wait for the next run. All
packets in the snapshot which share a next hop go into one ZIP bundle.

The FTN alias is `NNNNCCCC.BRP`:

- `NNNN` is a four-character base-36 namespace derived from this league and
  the transmitting hop's node number. League `0` has its own namespace, so an
  otherwise valid board whose league number is unset can still exchange mail.
- `CCCC` is a persistent four-character base-36 counter.
- The counter advances for every physical handoff, including each broadcast
  copy, and does not reset with a new game season.

The counter is reserved before publication, so a crash may skip a value but
cannot reuse it. Wrap is reported loudly. An existing alias is never
overwritten; the allocator advances until it finds a free one.

Attach and obox bundles are immutable after publication. Obox reuse may be
possible with a particular mailer's documented claim/rename protocol, but no
common obox lock is assumed here; it remains disabled pending interoperability
and race testing.

BSO is the exception. FTS-5005 gives the destination one `.bsy` covering its
outbound files. After acquiring that semaphore, `barons-ftn` may safely rebuild
a compatible bundle already advertised in the selected flow file. It releases
`.bsy` only after the replacement is durable.

FTS-5005 permits a `.bsy` to contain one line of PID information. A semaphore
created here contains `barons-ftn pid=<number>`, and the process also locks its
first-byte range on Windows, or takes a whole-file `flock` on Unix, for the
complete BSO update. A later helper removes that semaphore as stale only when
all three checks agree: the marker is exactly ours, the ownership lock can be
acquired non-blockingly, and the file is at least five minutes old. It then
retries the standard exclusive `.bsy` creation. Thus a live helper remains
protected even if its semaphore's timestamp is old, while a crash becomes
recoverable without guessing from age alone.

An empty, malformed, young, locked, or foreign-marked `.bsy` is simply busy.
`barons-ftn` never applies its five-minute policy to a mailer or tosser's
semaphore; the mailer's own FTS-5005 age/restart mechanism remains responsible
for those. A legacy empty semaphore left by an older `barons-ftn` is likewise
indistinguishable and follows the mailer's policy. Never manually clear `.bsy`
files merely because a peer is slow or offline.

Claimed packets and progress journals live under `data/ftn-spool`. A target is
marked complete only after its bundle and `.msg`, obox placement, or BSO flow
entry are durable. A restart uses the same alias and bytes, recognizes an
already-created attach message, and completes only unfinished targets.

Inbound rejection is per packet member, not per bundle. A wrong-league packet,
unknown destination, or routing cycle is recorded in the receipt while valid
members are still delivered or forwarded. After those valid members finish,
the complete original transport wrapper moves to `ftn-spool/bad` so the rejected
routing context remains available for diagnosis; it is not retried on every
later `--in` run. A local canonical-name collision is different: the receipt
and source stay pending because the operator must decide which bytes are valid.

All `barons-ftn` processes—both directions—hold `barons-ftn.lock`. Movement
between the connector spool and the private game directories also holds
`game.lock`. The lock order is always connector first, game second. Atomic
renames and exclusive file creation remain additional protections.

On local delivery, an existing canonical filename with identical bytes is
logged as a duplicate, not rewritten, and not counted as a new delivery. If
that canonical name already belongs to different bytes, the helper reports a
collision and retains its receipt and source rather than overwriting either file
or inventing a noncanonical name.

## Routing and broadcasts

The JSON packet names its final destination. A transport bundle is addressed
only to the next FTN hop. At a hub, `--in` reads enough JSON to choose the next
hop but copies the original JSON bytes into the new bundle without changing or
re-signing them. The actual node route and broadcast coverage live in the ZIP
manifest and are discarded before local game delivery. The final route node is
the transmitting hop, and the route length supplies the hop count, so neither
fact is stored twice.

A routed league with `HOST` lines is the normal and recommended arrangement.
The game creates addressed broadcast copies before signing, and the connector
routes each copy normally.

### Mesh warning

An old-style unaddressed broadcast has no final node. When it arrives over
Obox or BSO and `OboxMeshFanout Yes`, `--in` delivers it locally and sends it to
every configured peer in neither its `route` nor its `covered` list. Before a
sender publishes sibling copies, it puts every durably scheduled recipient in
the common `covered` list. This prevents those recipients from reflexively
cross-sending the same broadcast.

These lists limit amplification; they do not make distributed fanout exactly
once. In particular, a simple cycle of four or more nodes can have independently
scheduled branches meet after both are already in flight, producing duplicates.
The hop limit prevents an infinite loop, and canonical-name plus exact-byte
duplicate detection lets the inbound converge safely.

If the topology is a true mesh and the source already reaches every board, set:

```ini
OboxMeshFanout No
```

Then `--in` delivers an unaddressed broadcast locally and stops. The source
transport is responsible for putting one copy on every required direct link.
Do not use this switch to disguise a physical star whose roster claims to be a
mesh; describe that star with `HOST` lines instead.

## Troubleshooting by file location

| Where files accumulate | Meaning | Action |
|---|---|---|
| game `Outbound` | `--out` did not run or cannot take `game.lock` | Run `barons-ftn --out`; read its error |
| `ftn-spool/out` | At least one target is busy or failed | Read the warning; inspect that peer's `.bsy`, path, or netmail directory |
| `AttachDir` with no `.msg`/flow | Attach or BSO queue publication failed | Check subject length, `NetmailDir`, BSO directory, and permissions |
| `NetmailDir` `.msg` | The tosser has not packed outgoing netmail | Run/check the tosser and allow file attaches |
| BSO `.?lo` | The mailer has not successfully sent the referenced bundle | Check peer address, password, route, and `.bsy` |
| peer obox | The mailer has not sent or acknowledged the file | Check the peer session and outbox mapping |
| transport `InboundDir` | `--in` did not run, ran before receive completion, or rejected the wrapper | Run it after the session and read warnings |
| `ftn-spool/in` | Local publication or transit handoff is incomplete | Correct the named target; the next `--in` resumes it |
| game `Inbound` | `-planetary` has not applied the unwrapped packets | Run `immortal-barons -planetary` |
| `ftn-spool/bad` | An outbound packet was malformed/unroutable, or an inbound bundle contained a rejected member | Preserve it for diagnosis; correct the producing board, route, league, or roster |

One bad packet or busy peer does not stop unrelated destinations. Do not delete
spool journals to make a warning disappear: they are the record that prevents
partial work from being forgotten or blindly repeated.

## Upgrade order

ZIP bundles require `barons-ftn --in`; an older game cannot parse the ZIP as a
JSON packet. Upgrade receivers first:

1. Install the new helper on every board.
2. Configure `InboundDir` and schedule `barons-ftn --in` after receive sessions.
3. Verify that legacy raw `.brp` traffic is still delivered to the game.
4. Configure the per-peer `Link` modes.
5. Enable the new bundled `--out` path on senders.

Because `--in` accepts legacy raw JSON packets, steps 1–3 can be completed
without coordinating an exact cutover minute.
