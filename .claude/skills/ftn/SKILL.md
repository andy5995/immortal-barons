---
name: ftn
description: Use when moving files or messages over FidoNet Technology Network transport — netmail, file attaches, packets, BSO/FLO outbound, binkp, nodelists, AKAs and domains — or when configuring or debugging a mailer or tosser (BinkIT, SBBSecho, BinkD, Irex, Mystic's own), or when a packet is written and never arrives.
---

# FTN transport

FTNs are store-and-forward: nothing is delivered live. An application writes a
file, a **tosser** queues it, a **mailer** carries it on a session, and the far
side unpacks it. Every stage is a separate program with its own config and its
own failure mode — which is why "the mailers connect" proves almost nothing.

The formats this project produces are restated in `docs/dev/ftn-standards.md`;
its use of them is in `docs/dev/ibbs-packet-format.md`. This file is the wider
transport picture and the traps.

Nothing here is an RFC. The standards body is the **FTSC**: `FTS-` adopted,
`FSC-` proposed (several are universally implemented anyway), `FRL-`
reference. Filenames carry a revision (`fts-0001.016`) — cite it when a detail
could differ. Mirror: <https://nsrc.org/networks/fidonet/standards/>.

| what | document |
|---|---|
| stored message, packets, attach | [FTS-0001.016](https://nsrc.org/networks/fidonet/standards/fts-0001.016) |
| MSGID | [FTS-0009.001](http://ftsc.org/docs/fts-0009.001) |
| binkp/1.0 | [FTS-1026.001](http://ftsc.org/docs/fts-1026.001) |
| nodelist | [FTS-5000.005](http://ftsc.org/docs/fts-5000.005) |
| Binkley Style Outbound | [FTS-5005.003](http://ftsc.org/docs/fts-5005.003) |

Read them from those links rather than vendoring them (see the last section).
The revisions above are the ones this file's claims were checked against.

## The moving parts

| role | does | examples |
|---|---|---|
| application | writes a message or file to hand off | a door game, a BBS |
| tosser | packs messages, writes the outbound queue | SBBSecho, Mystic's `mutil` |
| mailer | makes the session, transfers files | BinkIT, BinkD, Irex, Mystic's `mis` |

**Each stage leaves the file in a specific directory, so whichever directory
still holds it is the stage that failed.** Work application output → tosser
queue → outbound queue → far inbound, and stop at the first place it is
sitting.

## Triage: written, never arrived

Each stage leaves the file somewhere. Find the first place it is still sitting;
that is the stage that failed. Ask for listings, not opinions.

1. **Application output.** Is the file there, non-empty, readable by the BBS
   user? For a netmail attach, is a `.msg` in the directory the tosser actually
   reads, is **Allow File Attachments** on, and is `0x0010` set on the message?
2. **Did the tosser run at all?** Timestamped log, and did the `.msg`
   disappear? This is the most common answer, and it is the real cause behind
   most "we need a schedule" conclusions.
3. **Did the tosser abort?** Grep its log for `attachment file not found`. In
   SBBSecho that is fatal to the entire run (`sbbsecho.c:5941`), so one bad
   path presents as "nothing tosses".
4. **The outbound queue.** List the outbound root *and* any `outbound.<zonehex>`
   siblings, and glob all five flavours — `.?lo` and `.?ut`, not just `.flo`.
   Nothing there means the tosser never queued it. Files in the *main* outbound
   for a private zone means the domain is missing and both tosser and mailer
   fell back to it silently (`sbbsecho.c:164-172`, `binkit.js:170-175`). Check
   `.bsy` mtime for a stale lock. With outboxes enabled the *packet* goes to
   the node's outbox while the attach still writes a flow file, so look in both.
5. **The session.** Did the queue drain — did the `.flo` shrink? A successful
   session can queue nothing at all (see Debugging). Compare the AKA each side
   has configured for the other, and the password.
6. **Their inbound.** Secure and unsecure both. Then their tosser log.

## Netmail file attach — where the pain is

The attachment's **pathname travels in the Subject field**, which is 72 bytes
including its NUL: **71 for the path, 70 with the Binkley `^`**. A 1980s header
field is carrying a filesystem path, which is why a long install path breaks
FTN transport and a short one is worth choosing deliberately.

Two traps in that one field:

- **The separator is space *or* comma** (`FIDO_FILELIST_SEP " ,"`, FTS-0001), so
  the Subject is a *list*. A path containing a space silently becomes two
  filenames, neither of which exists. Only the first token's directory is kept;
  later tokens are resolved against it.
- **A bare filename is not searched for.** Verify any mailer resolves one before
  relying on it — SBBSecho does not: it takes the directory straight from the
  Subject, has no attachment search path, and `chdir`s to the ctrl directory at
  startup, so a bare name is looked for in ctrl and reported not found.

`^` and `FLAGS KFS` are **not** how an attach is signalled — attribute bit
`0x0010` is (FTS-0001.016). Both mean *delete the file after sending*, and both
are read by the **tosser**, not the mailer:

- `^` prefixed to the Subject path is a BinkleyTerm-era convention that **no
  FTSC document defines** — do not cite FTS-5005 for it. SBBSecho strips it in
  `write_flofile` (`sbbsecho.c:783-796`) and re-emits it as the flow-file `^`
  of FTS-5005.003 §3.1, which is a different field in a different file.
- `FLAGS KFS` in the body does the same job (`sbbsecho.c:5915-5941`).

Neither is `0x0080` KillSent, which deletes the *message*.

**A path SBBSecho cannot resolve is fatal to the whole run**: `write_flofile`
returning non-zero calls `bail(1)` (`sbbsecho.c:5941`). One bad attach leaves
every other message untossed, so the symptom is "the tosser is broken", not
"one attach missing".

**The path does not travel.** Before packing, SBBSecho rewrites the Subject to
the bare filename (`sbbsecho.c:5944`) and the receiver prepends its own inbound
directory. The 71-byte budget constrains your *local* path only — never chase a
path-length theory on the receiving board.

## BSO/FLO outbound

**FTS-5005**, the queue a mailer scans. Filenames are hex net and node, so
`00010001.flo` is `1:1/1` and a listing tells you which link is backed up.

| file | is |
|---|---|
| `<net><node>.?ut` | a packet |
| `<net><node>.?lo` | a control file, one line per file to send |
| `<net><node>.bsy` | the node lock — REQUIRED (§5.1) |
| `<net><node>.csy` | a same-system call lock — OPTIONAL (§5.2); `.hld` and `.try` also exist (§5.3, §5.4) |
| `<net><node>.pnt/` | subdirectory for a point destination |

**The flavour letter is the `?`, and there are five** (FTS-5005.003 §3.2). The
order below is the priority for deciding whether to *poll* — not the order a
mailer sends files, which is whatever it finds in the directory:

| flavour | packet | flow | means |
|---|---|---|---|
| Immediate | `iut` | `ilo` | poll now, ignoring every restriction |
| Continuous | `cut` | `clo` | poll honouring internal but not external restrictions (remote is CM) |
| Direct | `dut` | `dlo` | poll honouring all restrictions |
| Normal | `out` | `flo` | as Direct, but may be rerouted |
| Hold | `hut` | `hlo` | do not poll; hand over when *they* call |

Note the packet and the flow file take **different letters for Normal** —
`out` against `flo` — and that "Continuous" is what most software and sysops
call **crash**. Grepping only for `.flo` misses four flow files, five packet extensions and
`.req`. Checking the age of a `.bsy` is *recommended* (§5.1), not required, so a
crashed mailer can still wedge a node.

Hold is not "never sent": it suppresses the poll, and §3.2 requires the files
to go out when the remote initiates. When *you* initiate for some other reason
§3.2 leaves it implementation-dependent — BinkIT takes the `.hlo` along
(`binkit.js:231`).

Line prefixes in a flow file (FTS-5005.003 §3.1) say what to do with the file
*after* a successful send:

| | |
|---|---|
| `#` | truncate to zero length (used for archived mail) |
| `^` | delete |
| `~` | skip this line — how an already-processed line is marked |
| none | send, keep |

`-`, `!` and `@` are optional alternates; §3.1 says to recognise them on read
and not emit them.

**List the full path.** §3.1 gives two reasons, and the second is the one that
bites: a bare name starting with a directive character is read as a directive,
and a name with no path at all resolves against a default directory that
differs between programs.

**A non-primary zone gets its own outbound directory, suffixed with the zone in
hex** — zone 777 is `outbound.309`. The domain is what maps an address to its
outbound root, and a mailer that cannot map one **falls back to the main
outbound rather than failing**, so a missing domain misroutes in silence. That
is the single best reason a private network needs its own domain.

## Sessions and security

Not optional detail — a password mismatch is a leading cause of "connects fine,
nothing moves".

- A **session password** is per linked node, and both sides must agree.
- Files from an unauthenticated session land in the **unsecure inbound**. A
  file can arrive there and still never be acted on — but check the mechanism
  before blaming paths: SBBSecho *does* scan it (`sbbsecho.c:7102-7106`) and
  then silently ignores unauthenticated EchoMail from it
  (`sbbsecho.c:6291-6296`). A packet password is the other way this happens.
- **A poll collects** — binkp is bidirectional (FTS-1026.001 §2) and BinkIT
  hands the queue over from its *inbound* auth callback (`binkit.js:1008`). So
  "they poll us, so we need no schedule" is half right: what you still need is
  your own **tosser** run, or there is nothing queued to collect.
- **"Authenticated" is not the bar BinkIT applies.** A caller whose addresses
  are *not configured at all* is neither rejected nor authenticated, and gets
  files for every address it claims (`binkit.js:987-997`). Only a configured
  node presenting a **wrong** password is refused. Worth knowing before
  assuming a session was safe.

## Addressing

`zone:net/node.point@domain`; point and domain omitted when zero or implied. A
system's several addresses are its **AKAs**, and the AKA presented decides
which of them a session is authenticated for.

## Synchronet specifics

The config is split across two programs, which is the usual stumbling block:

- `scfg` → Networks → FidoNet EchoMail and NetMail — **System Addresses**,
  **NetMail Directory**, **Allow File Attachments**.
- `echocfg` — **Domains**, **Linked Nodes**, and per-node **BinkP Settings**
  (Host, Port, Poll). Looking for these in `scfg` is a dead end.
- `sbbsecho` and `jsexec` take the **ctrl directory** from `SBBSCTRL`, falling
  back to `/sbbs/ctrl` with a warning when it is unset (`getctrl.c:28-32`).
  An `.ini` argument sets sbbsecho's own config file, never the ctrl dir
  (`sbbsecho.c:6808-6820`) — so pointing it at an `.ini` in another tree still
  reads `/sbbs/ctrl`.
- `binkit.js`, when not running as a service (`binkit.js:1211`): no arguments
  scans the outbound and sends; `-p` also polls; `-P` polls only; `-l <addr>`
  polls that node *instead of* the outbound scan. `-p`/`-P` reach only nodes
  with **BinkP Poll** set (`binkit.js:1109`), so neither is "poll everything".

Mystic is reported to keep its own message bases and watch no netmail
directory, so an application cannot hand it a `.msg` and wants a file box
instead. Unverified against Mystic's own source or docs — confirm before
relying on it.

## Myths this file used to repeat

Each was believed, written down, and disproved. They are the errors most likely
to be repeated by someone reasoning from one board's behaviour.

| myth | truth |
|---|---|
| "An inbound poll sends nothing of yours" | binkp is bidirectional; BinkIT queues outbound files from its inbound auth callback (`binkit.js:1008`). A board that looked one-way had never run its tosser. |
| "`^` / `FLAGS KFS` signal the attach" | `0x0010` signals the attach. Those two mean delete-after-send, and the tosser reads them. |
| "Hold flavour is never sent" | It suppresses the *poll*; FTS-5005.003 §3.2 requires sending when the remote initiates. |
| "The attachment path travels to the far side" | SBBSecho rewrites the Subject to the bare filename before packing (`sbbsecho.c:5944`). |

## Debugging

Judge a session by **whether the queue drained**, not by the mailer's last
line: a transfer that completed can still end on a complaint about pending
acknowledgements. Then walk the stages above and find the first directory still
holding the file.

**If it is in none of them and never arrived, do not conclude the fault is
theirs.** FTS-5005.003 §3.1 requires software to ignore a flow-file line whose
file is missing and carry on, so a session can report success having queued
nothing at all. Confirm the `.flo` actually shrank before looking outward.

## The modern alternative

**TITH** ("This Isn't That Hard") is a Rust store-and-forward protocol for FTNs
with public-key auth and no arbitrary field limits, so the Subject budget above
does not exist in it. Its daemon is not yet a production mailer — direction,
not a build target.

## Do not vendor FTSC documents

FTS-0001's licence forbids distribution as part of anything sold for a fee,
which a permissive licence grants everyone the right to do, and the documents
carry their authors' postal addresses. Restate the facts and cite the number;
`docs/dev/ftn-standards.md` has the reasoning in full.

## Keep this skill current

**Correct this file yourself, without being asked, the moment a session proves
something in it wrong.** It is committed, so a correction is one commit. Say in
one line what changed, and fix `docs/dev/ftn-standards.md`,
`docs/dev/ibbs-packet-format.md` and `docs/inter-bbs.md` in the same change —
a wrong sentence here usually lives in one of them too.

Two rules, both learned the hard way on this file:

- **Name the layer.** Protocol, document, tosser, mailer, or one board's
  config. Most FTN facts are not protocol facts, and every error found here so
  far was a one-implementation behaviour written as a general rule.
- **Cite what proved it** — a document section, or a file and line. Prefer
  deleting a wrong line to hedging it; "usually" keeps a false claim in
  circulation.
