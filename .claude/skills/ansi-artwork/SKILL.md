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
5. **Preview honestly, and LOOK at the render.** Print it to a real terminal, or
   render to PNG — **ansilove** (`ansilove -o out.png art.ans`) for 16-color
   CP437 pieces, **textimg** for 256-color/UTF-8 ones (ansilove renders the
   legacy palette, so a 256-color ramp comes out wrong). CP437 source has to be
   transcoded first: `python3 -c "import sys;sys.stdout.buffer.write(
   open('art.ans','rb').read().decode('cp437').encode('utf-8'))" | textimg -o
   out.png`. Then actually open the PNG — legibility faults (see the wordmark
   section) are invisible in the source and obvious in the image.
   See `references/formats-and-tools.md`.

## 3D and dimensional shading

Making flat cells look like rendered 3D objects is its own craft — spheres,
chrome, beveled logos, glows. It leans entirely on the shade ramp, half-blocks,
and disciplined light direction. **When the task involves any curved/lit/raised
surface, read `references/depth-and-3d.md`** — it has the step-by-step
sphere-shading method, the bevel/extrusion recipe, chrome/glass/glow material
looks, and concrete 16-color gradient ramps (e.g. synthesizing 6–7 perceived
shades of red from just colors 0/4/12/15 + the shade blocks).

## Planet / celestial surfaces: give a sphere terrain, not just a gradient

A Lambert-shaded sphere with one color ramp reads as a billiard ball. Real
worlds need **two ramps and a surface function**: an ocean ramp and a landmass
ramp, chosen per pixel by thresholding a deterministic noise value over the
surface point, then indexed by the same brightness the lighting already gave
you. Layered trig is enough — no noise library:

```python
v  = sin(2.1*nx + seed) * cos(1.7*ny - seed)
v += 0.70 * sin(3.1*nz + 1.7*seed)
v += 0.40 * cos(4.3*nx*ny + seed*2.1)
ramp = land if v > sealevel else sea      # then ramp[int(brightness*len(ramp))]
```

Three rules learned the hard way on the Immortal Barons splash:

- **The land ramp must differ from the sea ramp in HUE *and* VALUE, and be the
  darker of the two.** A land ramp that is brighter at equal brightness reads as
  a lit patch of ocean — a shading artifact, not a continent. Basalt against
  lava, olive against teal, tundra against deep blue.
- **Vary `sealevel` per world** so they are not obviously the same function
  reskinned: a scorched world mostly crust, an ocean world mostly water.
- **Polar caps** are nearly free and sell the sphere instantly: swap in a third
  ramp where `abs(ny) > ~0.8`, wobbled by the terrain value
  (`> capfrom + 0.10*t`) so the ice line is ragged rather than a painted band.
  A cap also proves the shape is a globe rather than a disc, because it curves
  with the limb. **Its ramp has two constraints at once**, and missing either
  one is visible: it must span the same brightness range as the sphere (a ramp
  that starts light renders the shadowed pole as bright as the lit one — a flat
  shelf stuck under the globe), *and* its floor must stay clearly paler than the
  sea's floor (give it the sea's dark end and the shadowed pole vanishes into
  deep water, so only one of the two caps reads as ice).

### Anti-alias the limb, or the sides look flat

Even with the aspect right, a binary inside/outside test staircases badly at the
**left and right limbs**, where the circle's curve runs nearly vertical and
several pixel rows land in the same column. That flat run is what a viewer
reports as "the sides look flattened" — and it is worst on the SMALLEST sphere,
where it is the largest fraction of the diameter (~31% of the height at r=11
against ~25% at r=12.5).

Sample each pixel on a 4x4 sub-grid, take the fraction inside the circle as
coverage, and **multiply the brightness by it** so partly-covered pixels fall
down the existing ramp instead of being fully lit or absent:

```python
cov = sum(1 for sy in range(4) for sx in range(4)
          if ((x+(sx+.5)/4-.5-cx)/r)**2 + ((y+(sy+.5)/4-.5-cy)/ry)**2 <= 1) / 16
if cov == 0: continue
b = shade(nx, ny, nz) * cov
```

No new glyphs and no palette work — the ramp you already have does the fading.
(The `░▒▓` shade ramp is the alternative when you only have 16 colors, but it
costs the cell its second half-block pixel, so prefer coverage-dimming whenever
you have 256 colors.)

### Cell aspect: measure it, do not assume 2:1

A sphere spanning `2r` columns and `r` character rows is round **only if a cell
is exactly 2:1 (w:h)**. Real cells are taller, and the error is not subtle: a
2.25:1 cell makes the circle 12% too tall, a 2.6:1 cell 30%. The tell is not
"it looks tall" — it is **"the sides look flattened"**, because on a vertical
ellipse the curve runs nearly vertical for longer at the left and right limbs.

Ask the terminal instead of guessing:

```sh
printf "\e[14t\e[18t"; read -t 1 -d t A; read -t 1 -d t B; echo "px:$A chars:$B"
```

`\e[14t` reports the window in pixels, `\e[18t` in characters; divide to get the
cell. Then squash the vertical radius by `ASPECT = 2 / (cell_h / cell_w)` —
1.0 for a classic VGA 8x16, ~0.89 for a 12x27 cell.

**Your PNG preview does not settle this.** ansilove/textimg render at their own
font's cell aspect, so art tuned for the target terminal looks wrong in the
preview and vice versa. Once tuned, re-stretch the preview to the measured
aspect before judging it (`im.resize((w, int(h*cell_h/cell_w/2)), NEAREST)`).

Keep the light direction identical to every other object in the piece; terrain
must not fight the lighting.

## Wordmarks at small sizes: a bitmap font beats a smaller FIGlet font

When a block-font wordmark has to shrink, the obvious move — a shorter FIGlet
font — usually abandons the solid-block look for line-art or ASCII (`Rectangles`,
`Small`, `Small Block` all do), which changes the identity rather than the size.
Draw the letters as a **5x7 bitmap on the same half-block canvas** instead: 7
pixel rows is 4 character rows, against 6 for ANSI Shadow, and 6 columns per
character, so an 8-letter word costs 47 columns instead of 68 — about 30% off
both axes with the solid-block feel intact.

**The drop shadow is what breaks legibility at this size.** A diagonal (+1,+1)
shadow crosses the one-column letter gap and welds each letter to the next:
"IMMORTAL" rendered as "INNORTAL", with the M's centre diagonal lost in the
neighbouring shadow. Two fixes, both needed: put the shadow **directly beneath**
(+0,+1) so it never enters the gap, and thin any glyph whose diagonal is only
one pixel wide (an `M` apex on one row, not two). This is invisible in the
source — only a render shows it.

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

Two header treatments, pick by weight:
- **Filled title bar** (a run of spaces with a background color + text) — heavy,
  formal, draws the eye. Good for a top-of-screen banner. But a solid *blue*
  panel in particular can read as dated/clunky; if it feels off, try the rule.
- **Box-drawing separator rule** — a line of `─` with a short `═` (double-line)
  accent inset near the left, e.g. `─────═════──────────────────`. Lighter and
  more authentic to classic BBS doors (this is how BRE frames its status
  blocks); transcodes to CP437 safely (box-drawing only), and it lets the
  content below (a bright name/version headline) be the only bright thing
  instead of competing with a filled bar. Framing content with the *same* rule
  top and bottom reads as a clean panel without any solid fill.

## Raised panels and 3D tables (functional UI, not just art)

Data tables and status panels in a terminal app can be given a **raised,
beveled** look with the same lighting logic as a 3D object: **light on the
top-left, dark on the bottom-right.** This reads as "lifted off the page"
without any pictorial art, and stays readable (unlike dithered cell fills,
which fight the text — keep the shade ramp *off* data cells).

The recipe (proven in this repo's Empire Status tables, `internal/menu`):

- **Light top edge.** Give the header row a bright background — `BgWhite` with
  black text. A zebra pattern (white header, light-gray value rows,
  `48;5;252`) reinforces "top is lit."
- **Dark right + bottom drop-shadow.** Cast a shadow in a *dark-but-not-black*
  gray (`48;5;238` works; pure black just looks like a gap). Two pieces:
    - **Right edge:** one shadow-gray cell printed *after* each row's closing
      border (`Reset` the row bg first, then `Bg<shadow> " " Reset`).
    - **Bottom edge:** one shadow-gray bar *offset one cell to the right*,
      printed under the last row (`"   "` = 2-space indent + 1 offset, then the
      bar). The offset is what makes it a drop-shadow, exactly like the dark
      offset copy behind extruded scene-ANSI wordmarks.
- **Cell borders** (`│` between columns) inside a solid background bar read as
  dividers; set the row's bg + fg once and print the whole `│ … │ … │` string,
  so the borders inherit the background.
- **Fixed cell width** (pad every cell to the widest heading/value) makes
  stacked sub-tables line up column-for-column.

256-color backgrounds (`\x1b[48;5;Nm`) are the practical way to get true light
and mid grays that the 16-color palette lacks (it only has black / bright-black
/ white / bright-white). Modern terminals and xterm.js render them; legacy
CP437 BBS clients may not, so treat the 3D shading as progressive enhancement.

## Lightbar menus (moving-highlight navigation)

A **lightbar** is a menu where a reverse-video/colored bar highlights the
current item and the arrow keys move it (Enter selects) — the interactive cousin
of static art. It is half *rendering* (drawing and repainting a full-width
background bar) and half *input loop* (raw mode + parsing arrow-key escape
sequences). Note it is a *modern* BBS-door convention; classic hotkey menus
(one letter acts immediately, no bar — what BRE and IB use) are a different
style, and a good lightbar keeps a hotkey fallback so it degrades to one.

**When building a lightbar menu, read `references/lightbar.md`** — it covers the
bar-rendering recipe (fixed-width background rectangle, full-redraw vs minimal
repaint, cursor hide/save/restore), the input state machine (raw mode, the
`\x1b[A`-vs-`\x1bOA` two-encoding trap, the bare-ESC ambiguity, bytes splitting
across reads, wrap-around indexing), the hotkey-fallback rationale, and a
language-neutral loop sketch.

## Telling pure 16-color ANSI from extended-palette / truecolor

Modern "ANSI" scene pieces (e.g. on 16colo.rs) often *look* like they use
dozens of colors — deep gray gradients, shaded skin/cloth. Usually that is
**16-color iCE ANSI faking tone by dithering** (4 grays + `░▒▓` + fg/bg mixing
→ ~7 perceived shades per hue), the same trick this skill's depth section
describes. But it can also be **XBin** (custom 256-entry palette + custom font)
or **24-bit truecolor** ANSI (Moebius/PabloDraw support both). You cannot tell
which from a rendered PNG alone — only the source `.ANS`/`.XB` settles it. When
in doubt, assume 16-color + dithering (it's the most common and the most
portable) and say the color-depth is unconfirmed.

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
- `references/lightbar.md` — lightbar (moving-highlight) menus: bar rendering,
  repaint strategies, raw-mode input, arrow-key escape-sequence parsing, the
  selection state machine, and the hotkey fallback.

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

## Keeping this skill current

Whenever you learn something new about ANSI/ASCII art while using this skill — a
glyph or shading trick, an encoding gotcha, a header/border treatment that reads
better, a palette or contrast rule, a terminal-rendering quirk, a technique that
worked or clearly failed — **fold it back into the relevant section here in the
same pass**, with the concrete glyph / escape code / example, not a vague note.
This skill is meant to accumulate hard-won text-mode craft so the next piece
starts ahead of where the last one did (the same discipline the bre-gather skill
uses). Update it as part of the work that produced the insight, not "later."
