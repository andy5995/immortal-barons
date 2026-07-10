# Character Set (CP437 or UTF-8)

The game can send its screens in two character sets: **CP437** or **UTF-8**.
This page explains the difference, which one to use, and how to choose.

## Quick answer

- **By default** the game sends CP437 (what traditional BBS terminals expect).
  Use `-utf8` for UTF-8, or `-cp437` to force CP437.
- **`-local`** auto-detects from your locale — UTF-8 when your locale is UTF-8
  (common today) — so local play usually needs no option.
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
- Neither — the game sends CP437, unless you use `-local`, which auto-detects
  from your locale (see below).

You cannot use both at once.

## Auto-detection

The game auto-detects the character set only with `-local`: it reads your
terminal's locale (`LC_ALL`, `LC_CTYPE`, then `LANG`) and uses UTF-8 when the
locale is UTF-8, so local play usually needs no option. To see CP437 on a UTF-8
system, run with `-cp437` (or set a non-UTF-8 locale, for example `LANG=C`).

Without `-local` — for example when the game runs as a door, serving a remote
caller — there is no local terminal to read, so the game uses the default
(CP437) unless you pass `-utf8` or `-cp437`.

Note: the Linux text console (a raw virtual terminal, not a terminal window) is
UTF-8. Use `-utf8` there if the game does not pick it — a bare console login
sometimes has no locale set.

If the game could better fit how you run it — for example, by detecting the
character set when it runs as a door — please
[open a feature request](https://github.com/andy5995/immortal-barons/issues).

## Languages

Non-English languages need UTF-8 output (`-utf8`). In CP437 mode the game shows
English only, and language selection is turned off. The caller must receive UTF-8
for the text to display correctly.

## Testing a character set

- To see CP437 the way a classic caller would: connect with SyncTERM, or wrap
  your connection with `luit -encoding CP437`.
- To see UTF-8: use any modern terminal set to UTF-8.
