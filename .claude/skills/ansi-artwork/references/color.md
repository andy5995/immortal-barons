# Color reference

## The classic 16-color VGA text palette

The palette almost all ANSI art targets. `SGR fg` / `SGR bg` are the numbers you
put in an escape: `ESC[<fg>;<bg>m`. RGB is the canonical VGA hardware value
(intensity levels 0x00/0x55/0xAA/0xFF). Emulators approximate these differently,
so treat RGB as a guide, not gospel.

| # | Name | SGR fg | SGR bg | RGB |
|---|------|--------|--------|-----|
| 0 | Black | 30 | 40 | #000000 |
| 1 | Red | 31 | 41 | #AA0000 |
| 2 | Green | 32 | 42 | #00AA00 |
| 3 | Yellow / brown | 33 | 43 | #AA5500 |
| 4 | Blue | 34 | 44 | #0000AA |
| 5 | Magenta | 35 | 45 | #AA00AA |
| 6 | Cyan | 36 | 46 | #00AAAA |
| 7 | White (light gray) | 37 | 47 | #AAAAAA |
| 8 | Bright black (dark gray) | 90 | 100 | #555555 |
| 9 | Bright red | 91 | 101 | #FF5555 |
| 10 | Bright green | 92 | 102 | #55FF55 |
| 11 | Bright yellow | 93 | 103 | #FFFF55 |
| 12 | Bright blue | 94 | 104 | #5555FF |
| 13 | Bright magenta | 95 | 105 | #FF55FF |
| 14 | Bright cyan | 96 | 106 | #55FFFF |
| 15 | Bright white | 97 | 107 | #FFFFFF |

Color **3** is brown at normal intensity and yellow when bright — a VGA quirk.

## The escape (SGR) mechanics

- `ESC` is byte `0x1B`, written `\x1b` or `\033` in code, shown here as `ESC`.
- Set colors: `ESC[<codes>m`, e.g. `ESC[91;44m` = bright-red on blue.
- Reset everything: `ESC[0m`. Reset after every art block so color doesn't leak.
- Separate escapes also work: `ESC[91m` then `ESC[44m`.
- **Bold = bright (historically).** On DOS/BBS, `ESC[1m` set the foreground
  intensity bit, so `ESC[1;31m` rendered as bright red. Using the `90`–`97`
  bright codes directly is clearer on modern terminals.

## Why backgrounds were limited to 8 colors

Classic PC text mode packed each cell into one attribute byte: 4 bits fg
(including an intensity bit), 3 bits bg, and 1 bit **blink**. The background had
no intensity bit — its 4th bit was blink — so standard mode gives 16 foreground
colors but only **8 backgrounds**.

## iCE colors — getting 16 backgrounds

"iCE color" repurposes the blink bit as a background-intensity bit: 16 solid
backgrounds, no blinking. It is a client-side decision, signaled two ways:

1. **Static (SAUCE metadata):** the file's SAUCE record sets the non-blink flag
   (`TFlags` bit 0). Renderers like `ansilove -i` honor it.
2. **Runtime (SyncTERM private toggle):** `ESC[?33h` then `ESC[?35h` (combined
   `ESC[?33;35h`). *(Single-source; verify against your target client.)*

There is no in-band "blink-as-color" standard in the byte stream itself.

## 256-color and truecolor (modern terminals only)

- **256-color:** fg `ESC[38;5;<n>m`, bg `ESC[48;5;<n>m`. Indices 0–15 are the
  system palette, 16–231 a 6×6×6 cube (`n = 16 + 36·r + 6·g + b`), 232–255 a
  grayscale ramp. Cube channel levels are **nonlinear**: 0, 95, 135, 175, 215,
  255.
- **Truecolor:** fg `ESC[38;2;<r>;<g>;<b>m`, bg `ESC[48;2;<r>;<g>;<b>m`.
  Supported by xterm, VTE, Konsole, iTerm2, Alacritty, kitty, Windows Terminal —
  **not** macOS Terminal.app, and **not** classic BBS clients (SyncTERM /
  NetRunner are CP437 + 16-color).

**Rule of thumb:** for portable art and any BBS target, design in the 16 colors.
Reach for 256/truecolor only when you know the output is a modern terminal (SSH,
local CLI). Designing in 16 colors also just looks more like "ANSI art."
