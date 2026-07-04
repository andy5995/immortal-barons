# Plain ASCII art (7-bit)

Art made only from printable 7-bit ASCII (`0x20`–`0x7E`): letters, digits, and
punctuation. No color and no block/shade glyphs are required, so it renders
identically everywhere — any terminal, any encoding, code comments, READMEs,
commit messages, logs, email. This is the maximally-portable text-art medium and
the classic "ASCII art" look, distinct from the colored block "ANSI" art the
main SKILL covers. (You *can* layer ANSI color on top of ASCII glyphs; the point
here is that the shapes come from characters, not from colored cells.)

## Density ramps (the ASCII shade ramp)

Shading in ASCII means choosing characters by how much ink they put in a cell.
Order them light→dark and walk the ramp to fake gradients and shade 3-D forms —
the same idea as the block shade ramp, but with letters.

- **Long ramp (70 steps), light→dark:**
  `` `.'^",:;Il!i><~+_-?][}{1)(|/tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$ ``
- **Short ramp (10 steps), light→dark:** `` .:-=+*#%@ ``
- **Very short (5 steps):** `` .:-=# ``

Caveats: exact perceived brightness depends on the font, and ramps assume a dark
background (light glyph on dark). On a light background, reverse the ramp. Keep
one ramp per piece; don't mix.

## Line and outline art

Trace silhouettes with the directional punctuation:

- `/` `\` — diagonals and rounded corners
- `_` — flat tops and bottoms (a `_` sits on the baseline, `‾` is not ASCII —
  use `-` for a raised bar)
- `|` — verticals; `(` `)` `<` `>` — curves and points
- `.` `'` — corner softening (`.` for a top corner, `'` for a bottom corner)

Example (a simple cube, outline only):

```
    +------+
   /      /|
  +------+ |
  |      | +
  |      |/
  +------+
```

## Boxes, rules, and diagrams

- Frames: `+---+`, `|   |`, corners `+`. Heavier rules: `===`, `***`, `###`.
- Arrows and flow: `->`, `=>`, `-->`, `<-`, `|` / `v` for vertical flow.
- Tables: `+`, `-`, `|` grid with `+` at every junction.

```
+--------+     +---------+
| input  | --> | output  |
+--------+     +---------+
```

## Shading a form (worked example: a sphere)

Same discipline as block art: pick one light direction (here upper-left), place
the brightest highlight and darkest shadow first, then fill with the ramp.

```
      .-"""-.
    .'@@@@@@@'.
   /@@@%%%###@@\
  |@@%%%####++@@|
  |@%%####++==-@|
   \%###++==--:/
    '.#+=--:.'
      '-...-'
```

The highlight (`.`,`:`) sits upper-left where light hits; density climbs to `@`
in the lower-right shadow. Consistent ramp + one light source = a round read.

## Big text (banners)

Don't hand-place letters for a wordmark — generate them:

- `figlet "TITLE"` — FIGlet fonts (many styles via `-f`), plain ASCII output.
- `toilet "TITLE"` — FIGlet-compatible, adds filters/colors if wanted.
- `figlet -f small`, `-f banner`, `-f slant`, `-f big` are common looks.

Pipe to a file and paste; the output is already width-bounded ASCII.

## Workflow

1. Pick the canvas width (≤ 80 is safe for terminals and code) and the light
   direction.
2. Block in the silhouette with outline characters.
3. Choose one density ramp; place the extremes (brightest, darkest) first.
4. Fill transitions along the ramp; step gradually — don't jump `.`→`@`.
5. Preview in a real monospace context at the target width; punctuation spacing
   that looks right in a proportional editor will drift in a terminal.

## When plain ASCII vs. block/ANSI

- **Plain ASCII:** plain-text targets (comments, `--help`, READMEs, logs,
  email), unknown/legacy terminals, copy-paste portability, retro ASCII look.
- **Block/ANSI (main SKILL):** you control the terminal and want smooth,
  colored, dimensional images; half-block resolution; CP437/UTF-8 handled.
