# Character Set (CP437, UTF-8 or ASCII)

The game can send its screens in three character sets: **CP437**, **UTF-8** or
plain **ASCII**. This page explains the difference, which one to use, and how to
choose.

## Quick answer

- **By default** the game sends CP437 (what traditional BBS terminals expect).
  Use `-utf8` for UTF-8, `-cp437` to force CP437, or `-ascii` if neither looks
  right.
- **`-local`** auto-detects from your locale — UTF-8 when your locale is UTF-8
  (common today) — so local play usually needs no option.

## The three character sets

- **CP437** is the classic BBS and DOS character set. Traditional terminals such
  as SyncTERM and NetRunner expect it. The box-drawing lines and block shapes in
  the game's screens come from CP437.
- **UTF-8** is the modern standard. Most terminals use it today.
- **ASCII** is the plain 7-bit set every terminal agrees on. The game
  substitutes look-alikes for what ASCII lacks: box rules become `-`, `|` and
  `+`, shaded blocks become `#`, and an accented letter loses its accent. The
  artwork is plainer, but nothing can be mis-decoded.

CP437 and UTF-8 are not the same at the byte level. If the game sends one and
the terminal expects the other, the screen looks wrong: garbled boxes and stray
symbols instead of clean lines. ASCII is the way out when you cannot tell which
one a caller has, or when their terminal is set to something else entirely
(Latin-1, say, which is a common default in modern telnet and ssh clients).

## The options

- `-utf8` — force UTF-8 output.
- `-cp437` — force CP437 output.
- `-ascii` — force plain ASCII output.
- None of them — the game sends CP437, unless you use `-local`, which
  auto-detects from your locale (see below).

You can use only one at a time. A separate option, `-no-ansi`, controls whether
the game sends colour and cursor control at all; the two questions are
independent, and a terminal can need both. See the
[Command Reference](command-reference.md) for the full list of command-line
options.

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

The game ships translations in other languages. Which ones you can pick depends
on the character set.

- **UTF-8** — every language is available. UTF-8 can show any character.
- **CP437** — a language is available only if all of its characters exist in
  CP437. German works, for example, because CP437 has the letters ä, ö, ü, and ß.

Languages that use characters CP437 does not have are not offered in CP437 mode.
This includes Russian and other Cyrillic languages; Czech, Polish, and similar
languages (their accented letters); and Chinese, Japanese, and Korean. To use one
of those, run with `-utf8`.

- **ASCII** — the same rule, more strictly. A language whose letters merely lose
  their accents is still offered, since it stays readable; one that would come
  out as question marks is not.

The game works this out for itself. It checks each translation, and if even one
character in it cannot be shown in CP437, that language is left off the CP437
list — so the screen never shows broken characters. Nothing is hard-coded: if a
translation later adds a character outside CP437, that language drops from the
CP437 list on its own.

In CP437 or ASCII mode the language menu (under Preferences) lists only the
languages that character set can display. If a language was already set in a
UTF-8 session and it does not fit, the session falls back to English rather than
showing broken characters.

## Testing a character set

- To see CP437 the way a classic caller would: connect with SyncTERM, or wrap
  your connection with `luit -encoding CP437`.
- To see UTF-8: use any modern terminal set to UTF-8.
- To see ASCII: run with `-ascii` on any terminal. It looks the same everywhere,
  which is the point of it.
