# Character Set (CP437 or UTF-8)

The game can send its screens in two character sets: **CP437** or **UTF-8**.
This page explains the difference, which one to use, and how to choose.

## Quick answer

- **Playing locally** (`-local`): the game follows your terminal. It uses UTF-8
  when your system locale is UTF-8 (common today) and CP437 otherwise. You
  usually do not need any option.
- **Running as a door**: the game sends CP437 by default. Add `-utf8` if your
  callers use UTF-8 terminals.
- **The web browser version** always uses UTF-8.

## The two character sets

- **CP437** is the classic BBS and DOS character set. Traditional terminals such
  as SyncTERM and NetRunner expect it. The box-drawing lines and block shapes in
  the game's screens come from CP437.
- **UTF-8** is the modern standard. Most terminals and all web browsers use it
  today.

The two are not the same at the byte level. If the game sends one and the
terminal expects the other, the screen looks wrong: garbled boxes and stray
symbols instead of clean lines.

## The options

- `-utf8` — force UTF-8 output.
- `-cp437` — force CP437 output.
- Neither — auto-detect for local play (from your locale); CP437 for a door.

You cannot use both at once.

## Playing locally

When you run `-local`, the game reads your locale (`LC_ALL`, `LC_CTYPE`, then
`LANG`). If it names UTF-8, the game uses UTF-8; otherwise it uses CP437. So on a
normal desktop terminal it just works.

To see CP437 on a UTF-8 system, run with `-cp437` (or set a non-UTF-8 locale,
for example `LANG=C`).

Note: the Linux text console (a raw virtual terminal, not a terminal window) is
UTF-8. Use `-utf8` there if the game does not pick it — a bare console login
sometimes has no locale set.

## Running as a door

How the door's character set reaches the caller depends on your BBS software.

**Synchronet (its default mode)** detects the caller's character set when they
connect and translates the door's CP437 output to it. So keep the default
(CP437), and Synchronet handles every caller — CP437 and UTF-8 alike. Do not add
`-utf8`, and do not turn on Synchronet's "Untranslated" mode for the door.

**Mystic, or any pass-through setup** (including Synchronet's "Untranslated"
mode), sends the door's output to the caller unchanged. Then the character set
the door sends must match the caller's terminal:

- CP437 terminals (SyncTERM and most classic clients) — use the default.
- UTF-8 terminals (for example, telnet from a modern terminal window) — add
  `-utf8`.

In a pass-through setup, one door command line sends one character set. If your
board serves both kinds of caller, pick the character set most of them use, or
set up two door entries (one with `-utf8`, one without) and let callers choose.

## Languages

Non-English languages need UTF-8, because CP437 can only show English. In CP437
mode the game stays in English and language selection is turned off. Run with
`-utf8` to use another language.

## Testing a character set

- To see CP437 the way a classic caller would: connect with SyncTERM, or wrap
  your connection with `luit -encoding CP437`.
- To see UTF-8: use any modern terminal set to UTF-8.
