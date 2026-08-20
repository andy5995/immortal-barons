# FTN standards reference

The FidoNet Technology Network formats this project has to produce or read,
and the documents that define them. This file is a reference to the standards
themselves: nothing here describes Immortal Barons. For how the game uses any
of it, see [`ibbs-packet-format.md`](ibbs-packet-format.md).

FidoNet grew up outside the IETF, so **none of this is specified by an RFC**.
Its standards body is the **FTSC** (FidoNet Technical Standards Committee),
which publishes numbered documents in several series:

| series | meaning |
|---|---|
| `FTS-nnnn` | an adopted standard |
| `FSC-nnnn` | a proposal, not adopted; some are widely implemented anyway |
| `FRL-nnnn` | a reference library document |

## Why no copy is kept here

FTSC documents carry their own licences, and FTS-0001's forbids distribution
"as part of or packaged with any product or other sales transaction for which
any fee is charged". A project under a permissive licence grants everyone the
right to sell copies, so bundling the document would make the combined work
undistributable under the terms the project advertises. The documents are also
signed with their authors' postal and email addresses, which do not belong in
a third party's repository.

Facts about a format are not copyrightable; the prose describing them is. So
this file states the formats in its own words and links to the originals.

## FTS-0001 — A Basic FidoNet(r) Technical Standard

Revision 016, 30-Sep-95.
<https://nsrc.org/networks/fidonet/standards/fts-0001.016>

One document defining **four separate things**, which are routinely confused
with one another:

1. **Message Text** — the body: 8-bit, `CR` (0x0D) as the line separator, no
   `LF`. Lines beginning with `^A` (SOH, 0x01) are control lines, called
   *kludges*, and are not shown to the reader.
2. **Stored Message** — a `.msg` file on disk, described below.
3. **Packed Message** — the same message as it appears inside a packet, with a
   different, shorter header.
4. **Packet** — the bundle a mailer transfers, whose header is the **Type-2**
   format. An extension called *Type-2+* is in general use.
5. The original **transfer protocol** (XModem over dial-up). This part is
   obsolete and not implementable over sockets; it is the reason FTS-0001 is
   sometimes described as a bad document, which is a judgement on this section
   and not on the formats above.

**"Type-2" names the packet header.** A stored message has no type number, so
"Type-2 `.msg`" is a misnomer; the accurate term is *stored message*.

### Stored-message header

190 bytes, then the NUL-terminated text. Multi-byte fields are little-endian.
Strings are NUL-padded within their fixed field.

| offset | size | field |
|---:|---:|---|
| 0 | 36 | `fromUserName` |
| 36 | 36 | `toUserName` |
| 72 | **72** | `subject` |
| 144 | 20 | `dateTime` |
| 164 | 2 | `timesRead` |
| 166 | 2 | `destNode` |
| 168 | 2 | `origNode` |
| 170 | 2 | `cost` |
| 172 | 2 | `origNet` |
| 174 | 2 | `destNet` |
| 176 | 2 | `destZone` |
| 178 | 2 | `origZone` |
| 180 | 2 | `destPoint` |
| 182 | 2 | `origPoint` |
| 184 | 2 | `replyTo` |
| 186 | 2 | `attr` |
| 188 | 2 | `nextReply` |

`dateTime` is 20 bytes holding `DD MMM YY  HH:MM:SS` — 19 characters, note the
**two spaces** before the time — plus a terminating NUL.

The file is named `<number>.msg`, numbered from 1 within a message directory.

**The 72-byte `subject` field is where the file-attach path budget comes from.**
A NUL terminator leaves 71 usable bytes. Under Binkley-style attach semantics
the path is prefixed with `^`, leaving 70. This is a property of the header
field, not of any transport.

### Attribute word (`attr`)

Bit flags. The ones that bear on file attaches:

| bit | name |
|---|---|
| 0x0001 | Private |
| 0x0010 | FileAttached |
| 0x0080 | KillSent |
| 0x0100 | Local |

**The attach itself is `0x0010`.** `^` and `^AFLAGS KFS` signal something
else — *delete the file once it has been sent* — and they are alternatives
rather than both: a Binkley-style `^` prefix on the subject path, or the
control line. Do not read either as the attach, and do not confuse them with
`0x0080` KillSent, which deletes the message rather than the file.

### Zone and point fields

The header's zone and point words exist, but much software ignores them, which
is why 4-D addresses are carried redundantly in kludge lines (below). Writing
both is the safe course.

## Kludge lines

Each begins with `^A` and ends with `CR`.

| kludge | defined by | form |
|---|---|---|
| `INTL` | FTS-4001 | `^AINTL <destination> <origin>` — destination first |
| `FMPT` | FTS-4001 | `^AFMPT <origin point>`, only when non-zero |
| `TOPT` | FTS-4001 | `^ATOPT <destination point>`, only when non-zero |
| `MSGID` | FTS-0009 | `^AMSGID: <origin address> <serial>` |
| `REPLY` | FTS-0009 | `^AREPLY: <the MSGID being answered>` |
| `Via` | FTS-4009 | a routing audit trail |
| tear line, origin line, `SEEN-BY`, `PATH` | FTS-0004 | echomail only |

`^APID:` identifies the producing software and is in general use; it comes from
an FSC proposal whose number is not recorded here because it was not verified.

FTS-4001 exists because there is otherwise nowhere in the header a 4-D address
can be reliably carried.

## Addressing

`zone:net/node.point@domain`. Point and domain are omitted when they are zero
or implied. Zones partition the network; a net is a group within a zone; a
point hangs off a node.

## Finding the documents

FTSC publishes the full series; <https://nsrc.org/networks/fidonet/standards/>
mirrors it, and a filename carries its revision (`fts-0001.016`). Cite the
revision when the detail could differ between them.
