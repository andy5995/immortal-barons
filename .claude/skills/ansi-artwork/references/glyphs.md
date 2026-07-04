# Glyph reference: CP437 ↔ Unicode

The drawing glyphs of ANSI art. Each row gives the classic **CP437 hex byte**,
the **Unicode code point** (what you emit in a UTF-8 file), the glyph, and its
use. Emit the Unicode code point for modern/UTF-8 terminals; the CP437 byte only
for legacy CP437 clients. (Mappings verified against unicode.org's CP437.TXT.)

## Blocks and shades — the core drawing glyphs

| CP437 | Unicode | Glyph | Name / use |
|-------|---------|-------|------------|
| 0xDB | U+2588 | █ | Full block (100% ink) |
| 0xB0 | U+2591 | ░ | Light shade (~25%) |
| 0xB1 | U+2592 | ▒ | Medium shade (~50%) |
| 0xB2 | U+2593 | ▓ | Dark shade (~75%) |
| 0xDF | U+2580 | ▀ | Upper half block (top = fg, bottom = bg) |
| 0xDC | U+2584 | ▄ | Lower half block (bottom = fg, top = bg) |
| 0xDD | U+258C | ▌ | Left half block |
| 0xDE | U+2590 | ▐ | Right half block |

Quarter/eighth blocks (Unicode, no single CP437 byte for most): `▘▝▖▗▚▞▙▟▛▜`
split a cell 2×2; `▁▂▃▄▅▆▇█` and `▏▎▍▌▋▊▉█` give 1/8 steps for bars/meters.
Note a cell still holds only one fg + one bg color regardless of glyph.

## Single-line box drawing

| CP437 | Unicode | Glyph | Name |
|-------|---------|-------|------|
| 0xC4 | U+2500 | ─ | Horizontal |
| 0xB3 | U+2502 | │ | Vertical |
| 0xDA | U+250C | ┌ | Top-left corner |
| 0xBF | U+2510 | ┐ | Top-right corner |
| 0xC0 | U+2514 | └ | Bottom-left corner |
| 0xD9 | U+2518 | ┘ | Bottom-right corner |
| 0xC3 | U+251C | ├ | Tee right |
| 0xB4 | U+2524 | ┤ | Tee left |
| 0xC2 | U+252C | ┬ | Tee down |
| 0xC1 | U+2534 | ┴ | Tee up |
| 0xC5 | U+253C | ┼ | Cross |

## Double-line box drawing

| CP437 | Unicode | Glyph | Name |
|-------|---------|-------|------|
| 0xCD | U+2550 | ═ | Horizontal |
| 0xBA | U+2551 | ║ | Vertical |
| 0xC9 | U+2554 | ╔ | Top-left |
| 0xBB | U+2557 | ╗ | Top-right |
| 0xC8 | U+255A | ╚ | Bottom-left |
| 0xBC | U+255D | ╝ | Bottom-right |
| 0xCC | U+2560 | ╠ | Tee right |
| 0xB9 | U+2563 | ╣ | Tee left |
| 0xCB | U+2566 | ╦ | Tee down |
| 0xCA | U+2569 | ╩ | Tee up |
| 0xCE | U+256C | ╬ | Cross |

Mixed single/double junctions exist too (CP437 0xB5–0xD8, 18 of them), e.g. the
mixed crosses `╫` (U+256B) and `╪` (U+256A). Use them when a single-line rule
meets a double-line frame.

## Rounded and heavy variants (Unicode only)

- Rounded corners: `╭ ╮ ╰ ╯` (U+256D–2570) — softer boxes.
- Heavy lines: `━ ┃ ┏ ┓ ┗ ┛` (U+2501, 2503, 250F, 2513, 2517, 251B).

These have no CP437 byte — modern-terminal only.

## Practical notes

- The end-of-content marker in a `.ans` file is byte **0x1A** (Ctrl-Z); it
  separates the art from an appended SAUCE record. Not needed for art embedded
  in source code.
- Whitespace matters: a run of spaces with a background color set *is* a colored
  bar. Trailing spaces are load-bearing in ANSI art.
