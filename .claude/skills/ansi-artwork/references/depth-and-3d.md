# Depth and "3D" shading

How to make flat character cells look like rendered, lit 3D objects — spheres,
chrome, beveled logos, glows. Everything here rests on three tools from the main
skill: the shade ramp (`░▒▓█`), the half-block sub-pixel trick (`▀`/`▄`), and a
single, consistent light direction.

## Shade a sphere/orb — the canonical exercise

The method scene artists teach (Halaster, Lord Soth — roysac.com):

1. **Pick the light direction before drawing anything.** e.g. light from the
   upper-right, shadow toward lower-left. Every lit object in the piece must
   agree — inconsistent light direction is the #1 amateur tell.
2. **Block in the base shape** as a flat mid-tone fill. Nail the silhouette
   first.
3. **Place the extremes first, not a smooth ramp.** Put the brightest highlight
   (white / color 15) as a small, off-center spot near the light-facing edge,
   and the deepest shadow (black / color 0) as a small spot on the opposite
   edge. Bright and dark *spots*, not bands.
4. **Fill transitions with the shade ramp**, moving through the object's hue
   from dark to bright along the light direction. Use `░▒▓█` + half-blocks to
   blend between the discrete color steps.
5. **Break banding with swirls, not rings.** Perfect concentric rings read as
   flat and cartoonish. Use slightly irregular, curved bright/dark patches.
6. **Exaggerate contrast.** 16-color ANSI can't do subtle gradients — understated
   shading looks muddy. Push highlights brighter and shadows darker than a
   realistic reference would.
7. **Reserve half-blocks for the border between two solid color zones**, not as
   texture everywhere. A bright half-block bleeding into a shadow zone makes
   shading look disjointed.
8. **Some hues shade better than others.** Cyan shades smoothly; pure red/green/
   blue resist gradients in 16 colors — reduce their contrast or use a
   cartoon-outline treatment instead.

## The half-block resolution-doubling recipe (restated)

A cell with `fg = A`, `bg = B`, glyph `▀` shows color A on the top pixel, color
B on the bottom. `▄` flips it. Build a surface as a grid of these vertical pixel
pairs to render at ~80×50 instead of 80×25. This is the workhorse for smooth
curves; a small code helper `(top, bottom) -> ESC[3Am ESC[4Bm ▀` makes it
practical.

## 16-color ramp strategy

You can't add palette entries, so synthesize intermediate shades by density-
mixing two colors in one cell via fg/bg + a shade glyph. Pair a hue's dark
color, its bright counterpart, and black/white as endpoints.

**Example — a red sphere, light upper-right** (colors 0 black, 4 dark red,
12 bright red, 15 white):

| Zone (bright → dark) | Cell |
|----------------------|------|
| Hottest highlight | fg=15, bg=4, glyph `▓` — near-white core |
| Bright midtone | fg=12, bg=4, glyph `▓` — mostly bright red |
| Base | solid fg=12 `█` (or plain bright-red fill) |
| Transition | fg=12, bg=4, glyph `▒` — optical half-mix |
| Core shadow | solid fg=4 `█` |
| Terminator | fg=4, bg=0, glyph `▒`→`░` — fading to near-black |

Reading down is the dark-to-light ramp: ~6–7 perceived shades from only 3 real
palette colors plus white. The same pattern repeats per hue: pick
`{dark-X, bright-X, black, white}` and shade-blend adjacent pairs.

## Bevel / extrusion (raised or carved text and buttons)

The text-mode transliteration of the standard bevel recipe (not from a specific
ANSI tutorial — it's the raster convention adapted to cells):

- **One light direction**, conventionally upper-left light / lower-right shadow.
- **Highlight edge:** top and left edges get a 1-cell line brighter than the
  face (color 15, or the face's bright variant).
- **Shadow edge:** bottom and right edges get a 1-cell line darker than the face
  (color 0, or the face's dark variant).
- **Face:** fill with the base color, optionally a subtle vertical half-block
  gradient (lighter top, darker bottom) for a domed look.
- **Drop shadow:** draw a solid dark silhouette offset 1 cell down-and-right,
  *underneath* the object (render it first, art on top).
- **Carved/recessed = invert:** shadow on top-left, highlight on bottom-right —
  reads as a groove instead of a raised object.

## Material looks

- **Chrome / polished metal:** sharp, high-contrast highlight *bands* rather than
  a soft gradient. Run dark→light→dark→bright-white→dark bands perpendicular to
  the light, with the brightest band very narrow (1–2 cells) and hard-edged (no
  shade-block softening there). This reads as a specular reflection.
- **Glass / orb:** sphere shading plus a small secondary hot-spot offset from the
  main highlight, and let some background show faintly through (`░` over a
  different bg) to suggest transparency.
- **Glow:** radiate from a bright white core through the hue ramp into the
  background, ending in `░` so it feathers out instead of stopping hard.

## Scene-level depth (composition)

- **Overlap** — nearer objects drawn on top of farther ones. The strongest depth
  cue in a flat grid.
- **Size diminution** — distant objects rendered in fewer cells.
- **Atmospheric fading** — distant elements use lighter shade density (more `░`,
  less `▓█`) and lower contrast; foreground is punchy and high-contrast.
- **Horizon / vanishing** — one horizontal band sets ground vs. sky; converging
  diagonals (box-drawing `╱╲` or angled characters) suggest receding lines.
- **Foreground framing** — large, heavily-`░`-shaded low-contrast shapes at the
  canvas edges push the sharp subject into perceived middle distance.

## Example galleries and tutorials

- 16colo.rs — the living archive; browse the Blocktronics / ACiD / iCE group
  pages for dimensional pieces (`https://16colo.rs/`).
- roysac.com tutorials — Halaster's shading, Newt's greyscale, Lord Soth's tips.
- Moebius editor (`https://blocktronics.github.io/moebius/`) — its half-block
  brush is what makes the sub-pixel technique practical to draw by hand.
