---
name: ansi-artwork
description: >-
  Create ANSI / text-mode artwork — splash screens, logos, banners, menu
  headers, borders, and dimensional "3D" shaded pieces — for terminals, BBS
  door games, and CLI tools. Use this whenever the user wants ASCII or ANSI
  art, a terminal splash or banner, a text-mode logo or wordmark, block/shade
  art, a colored menu header or box border, or asks to make terminal / CLI /
  BBS output look good — even phrased loosely as "make a cool banner", "add
  art to the splash screen", or "the header looks plain." Covers the
  CP437/Unicode block glyphs, the 16-color ANSI palette, half-block sub-pixel
  shading, 3D depth and lighting, the UTF-8-vs-CP437 encoding trap, and
  embedding finished art into source code.
---

# ANSI / Text-Mode Artwork

ANSI art is drawing with a grid of character cells. Each cell holds one glyph,
one foreground color, and one background color. You "paint" by choosing glyphs
(mostly block and shade characters) and colors, and the terminal renders the
bytes. There is no anti-aliasing and no true blending — the craft is getting
smooth, dimensional-looking images out of a coarse grid and a 16-color palette.

This is a different medium from web/GUI design. Do not reach for HTML/CSS or
image tools to make a terminal splash — the output must be bytes a terminal
renders. That constraint is the whole point.

## Start here: the mental model

- **Canvas:** default to **80 columns** wide. It is the one width nearly every
  terminal and BBS client assumes without negotiation. Height is flexible; a
  full DOS screen is 25 rows (or 50 in the tall mode).
- **A cell** = glyph + foreground color + background color. Color is set with
  ANSI escape (SGR) codes and stays in effect until you change it.
- **The palette is 16 colors** (8 normal + 8 bright). Backgrounds are often
  limited to the 8 normal colors on legacy clients. See `references/color.md`.
- **The glyphs that matter** are the block and shade characters, not letters.
  The four shade blocks and the half blocks do 90% of the work. See
  `references/glyphs.md`.

If you only remember three things, remember the shade ramp, the half-block
trick, and the encoding rule below.

## The shade ramp (fake gradients)

Five density levels from a single color — the built-in dither ramp:

```
(space)  ░       ▒        ▓        █
 0%      25%     50%      75%      100%
         U+2591  U+2592   U+2593   U+2588
```

Walk this ramp, optionally swapping fg/bg colors as you go, to fake a gradient
the flat 16-color palette can't show directly. Denser block = more "ink" of the
foreground color over the background.

**Blend adjacent color bands — do not hard-cut between them.** The most common
amateur mistake is stepping through solid color rows (row of amber, then row of
red) with a hard boundary, which reads as flat stripes. Instead, put a
transition cell *between* two colors: set `fg` = the brighter color, `bg` = the
darker color, and a shade glyph (`▒` for a even mix, `░`/`▓` to bias) so the two
optically blend at the seam. On a multi-color wordmark or object, every place
two color zones meet wants a `░`/`▒`/`▓` transition, not a clean edge. This is
what turns "3 solid stripes" into "a smooth gradient." A hue's family
(dark → bright → white) blended this way yields 6–7 perceived shades from 3
palette colors — see `references/depth-and-3d.md`.

## The half-block trick (double your resolution)

This is the single most important technique. A cell has one fg and one bg
color. The glyph `▀` (U+2580, upper half block) paints the **top half of the
cell in the foreground color** and leaves the **bottom half showing the
background color**. `▄` (U+2584) is the reverse.

So one cell becomes **two independently-colored vertical pixels**. An 80×25
canvas becomes an effective 80×50 pixel grid. This is how ANSI art gets smooth
curves and rendered-looking surfaces instead of chunky 80×25 blocks.

Concrete: to stack a bright-blue pixel above a dark-blue pixel in one cell, set
`fg = bright blue`, `bg = blue`, glyph = `▀`. Rows of these give smooth vertical
transitions. (Quarter blocks `▘▝▖▗▚▞▙▟▛▜` split a cell 2×2, but only two colors
still apply per cell.)

## The encoding rule (the #1 gotcha)

Classic ANSI art used **CP437** — a single-byte code page where `0xDB` is `█`,
`0xB0` is `░`, etc. Modern terminals speak **UTF-8**, where those same glyphs
are multi-byte Unicode code points (`█` = U+2588, `░` = U+2591). **These are not
interchangeable at the byte level.**

- Emit a **raw CP437 high byte** (≥ 0x80) to a UTF-8 terminal and it decodes as
  garbage (mojibake): bytes ≥ 0xC0 look like the start of a multi-byte sequence
  and swallow following bytes; bytes 0x80–0xBF show as stray `�`.
- **Recommendation for modern terminals and Unix-native BBS doors: use the
  Unicode code points and emit UTF-8.** A string literal `"█▀░▒▓"` in a
  UTF-8 source file (Go, Python, etc.) is already correct UTF-8 — it just works.
- **Legacy BBS caveat:** SyncTERM and NetRunner are CP437-first and may *not*
  render UTF-8 block glyphs. If you must support them, either detect the client
  and emit CP437 bytes, or restrict art to the common block/shade glyphs and
  document CP437-only clients as a known gap. Modern telnet/SSH BBS software
  (ENiGMA½, x/84) translates CP437↔UTF-8 for you.

`references/glyphs.md` has the full CP437-hex ↔ Unicode ↔ glyph table for every
block, shade, and box-drawing character.

## Plain ASCII art (7-bit) — the portable sibling medium

Everything above draws with block/shade glyphs and color. **Plain ASCII art** is
a different medium: it uses only printable 7-bit ASCII (`0x20`–`0x7E` — letters,
digits, and punctuation like `/ \ | _ - = + * . : # @`), no color required, no
block glyphs. Because every byte is plain ASCII, it renders identically in *any*
terminal, encoding, code comment, README, commit message, log file, or email —
none of the CP437/UTF-8 gotcha above applies. It is also the right choice for a
deliberate retro "ASCII" look (as distinct from colored "ANSI" block art).

**Reach for plain ASCII when** the target is a plain-text context (source
comments, `--help` output, READMEs, diagrams in docs), an unknown/legacy
terminal, or you want art that survives copy-paste anywhere. **Reach for the
block/color approach** (the rest of this skill) when you control the terminal
and want smooth, dimensional, colored images.

The core techniques:

- **The ASCII density ramp** — the plain-text analog of the shade ramp. Order
  characters by how much ink they put on the cell:
  `` .`'^",:;Il!i><~+_-?][}{1)(|/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$ ``
  A short, reliable subset is `` .:-=+*#%@ `` (light→dark). Walk it to fake
  gradients and shade 3-D forms, exactly like the block ramp but with letters.
- **Line / outline art** — trace silhouettes with `/ \ | _ - ( ) < >`; use
  `/ \` for diagonals and corners, `_` for flat tops/bottoms, `|` for verticals.
- **Boxes and rules** — `+---+`, `|   |` for frames; `===` / `***` for heavier
  rules; `->`, `=>`, `-->` for arrows in diagrams.
- **Big text** — `figlet` / `toilet` render FIGlet fonts (large ASCII letters)
  for banners without hand-placing glyphs.

The shading discipline carries over unchanged: pick one light direction, keep a
consistent density ramp, assume a monospace grid, and test at the target width.
`references/plain-ascii.md` has the full density ramps, worked examples
(sphere, banner, box), and tool notes.

## Building a piece: workflow

1. **Decide the size and the light direction first.** Pick a canvas width
   (≤ 80) and, for any shaded/3D element, a single light-source direction
   (e.g. upper-right). *Every* shaded object must agree on that direction —
   inconsistent lighting is the number-one tell of amateur work.
2. **Block in silhouettes** with flat mid-tone fills. Get shapes and layout
   right before shading — like a cartoon flat-color pass.
3. **Place the extremes, then fill the ramp.** Drop the brightest highlight and
   the darkest shadow as small spots first, then fill transitions with the
   shade ramp + half-blocks. Exaggerate contrast — 16-color ANSI makes subtle
   shading look muddy and flat.
4. **Add depth cues:** drop shadows (render a dark offset copy first, art on
   top), bevels, overlap. See the 3D section below.
5. **Preview honestly.** Print it to a real terminal, or render to PNG with
   **ansilove** (`ansilove -o out.png art.ans`) for docs instead of
   screenshotting. See `references/formats-and-tools.md`.

## 3D and dimensional shading

Making flat cells look like rendered 3D objects is its own craft — spheres,
chrome, beveled logos, glows. It leans entirely on the shade ramp, half-blocks,
and disciplined light direction. **When the task involves any curved/lit/raised
surface, read `references/depth-and-3d.md`** — it has the step-by-step
sphere-shading method, the bevel/extrusion recipe, chrome/glass/glow material
looks, and concrete 16-color gradient ramps (e.g. synthesizing 6–7 perceived
shades of red from just colors 0/4/12/15 + the shade blocks).

## Logos and wordmarks

- **Quick monochrome:** `figlet` produces plain-ASCII block letters — easy to
  generate and paste. Good for a fast banner.
- **Authentic colored-block scene look:** TheDraw `.TDF` fonts render each
  letter as colored block glyphs (tools: `tdfiglet`). This is what real scene
  wordmarks use.
- **Hand-built:** for a distinctive logo, draw the letterforms yourself with
  block/half-block glyphs and shade them like any 3D object (bevel the edges,
  light from one direction, add a drop shadow).

## Boxes, borders, and menu headers

Single- and double-line box-drawing characters frame menus and panels. Double
lines (`╔═╗║╚╝`) read as heavier/more formal; single (`┌─┐│└┘`) as lighter.
Full tables (including tee/cross junctions and mixed single/double joints) are
in `references/glyphs.md`. A colored title bar is often just a run of spaces
with a background color set, plus text — see the embedding section.

## Embedding finished art into source code

For a program that prints art (a BBS door, a CLI tool), embed the art as a
**UTF-8 string literal** and write the ANSI color codes around it.

- Go, Python, Rust, etc. source files are UTF-8, so `"█▀░▒▓─│"` literals are
  correct with no escaping.
- Keep color escapes in one place. If the project already has an ANSI helper
  (e.g. this repo's `internal/ansi` package with `FgBrightCyan`, `BgBlue`,
  `Reset`), use it rather than hand-writing `\x1b[` codes, so the art matches
  the rest of the UI and stays readable.
- A colored bar/header = set a background color, print a run of spaces (or text
  padded to width), then `Reset`. A gradient bar = a row of `▀`/`▄`/shade
  glyphs stepping through fg/bg colors.
- **A half-block helper pays for itself.** A tiny function `(topColor,
  bottomColor) -> "\x1b[3Xm\x1b[4Ym▀"` lets you build images as a grid of
  vertical pixel pairs. If you generate more than a couple of pieces, write it
  once.
- Run `scripts/palette.py` to print a live reference card (the 16 colors, the
  shade ramp, and a half-block demo) in your own terminal to calibrate color
  choices before you commit them to code.

## Originality and copyright

Reproduce *techniques*, never someone else's *art*. Do not copy a company's or
another artist's logo/splash/ANSI piece byte-for-byte into a project. Scene art
on 16colo.rs is authored work — study it for technique, then draw your own. (In
this repo specifically: the original BRE's splash art is copyrighted; make
original art.)

## Reference files

- `references/glyphs.md` — CP437-hex ↔ Unicode ↔ glyph tables: blocks, shades,
  half/quarter blocks, single/double/mixed box drawing.
- `references/color.md` — the 16-color VGA palette (names, SGR fg/bg codes,
  RGB), the bright/bold and iCE-color mechanics, 256-color and truecolor.
- `references/depth-and-3d.md` — sphere/orb shading, bevels/extrusion,
  chrome/glass/glow, 16-color ramp strategy, scene-depth composition.
- `references/formats-and-tools.md` — `.ans`/`.asc`/XBin/SAUCE formats, the
  editor/tool landscape, and `ansilove` for headless PNG previews.
- `references/plain-ascii.md` — plain 7-bit ASCII art: density ramps, line/box
  art, shading a form, figlet/toilet banners, and when to use it vs. block art.

## scripts/

- `scripts/palette.py` — prints a 16-color + shade-ramp + half-block reference
  card to your terminal (stdlib only, no dependencies). Run it to see the real
  colors before choosing them.
- `scripts/sphere.py` — renders a lit 3D sphere with the half-block technique
  (`python3 sphere.py [green|red|blue|cyan|amber|gray] [radius]`, output is raw
  ANSI to stdout). A working reference implementation of light-source shading:
  each cell is two vertical pixels, brightness comes from a Lambert dot-product
  with the light direction, mapped onto a per-hue ramp. Read it to see the
  depth math concretely, or use it to drop an orb into a piece. Note the
  aspect fix inside: it uses ~2× as many columns as rows because terminal
  cells are ~1:2, so a symmetric loop renders a tall ellipse, not a circle.
