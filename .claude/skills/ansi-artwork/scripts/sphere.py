#!/usr/bin/env python3
"""Render a lit 3D sphere as ANSI art using the half-block sub-pixel technique.

Demonstrates the skill's core depth method concretely: each character cell is
two vertical pixels (via '▀': top=fg color, bottom=bg color), and each pixel's
brightness comes from a Lambert dot-product of the sphere's surface normal with
a light direction. Brightness maps onto a 4-stop color ramp for the chosen hue.

Usage:
    python3 sphere.py [hue] [radius]
      hue    = green | red | blue | cyan | amber | gray   (default green)
      radius = cell-height radius, ~8-16 looks good        (default 11)

Output is raw ANSI to stdout — redirect to a .ans file or pipe to a terminal.
Stdlib only; emits UTF-8 (the Unicode half-block U+2580)."""

import math
import sys

# Per-hue ramp: dark -> mid -> bright -> highlight, as SGR foreground codes.
# The renderer converts an fg code to its bg code with +10 (30->40, 92->102).
RAMPS = {
    "green": [30, 32, 92, 97],   # black, green, bright green, white
    "red":   [30, 31, 91, 97],
    "blue":  [30, 34, 94, 97],
    "cyan":  [30, 36, 96, 97],
    "amber": [30, 33, 93, 97],   # black, brown, bright yellow, white
    "gray":  [30, 90, 37, 97],   # black, dark gray, light gray, white
}

# Light direction: upper-right, toward the viewer. y is negative-up.
LIGHT = (0.55, -0.55, 0.62)


def _norm(v):
    m = math.sqrt(sum(c * c for c in v)) or 1.0
    return tuple(c / m for c in v)


LIGHT = _norm(LIGHT)


def brightness(nx, ny):
    """Brightness 0..1 at a point (nx, ny) on the unit disc, or None if off it."""
    r2 = nx * nx + ny * ny
    if r2 > 1.0:
        return None
    nz = math.sqrt(1.0 - r2)
    diffuse = max(0.0, nx * LIGHT[0] + ny * LIGHT[1] + nz * LIGHT[2])
    ambient = 0.10
    b = ambient + (1.0 - ambient) * diffuse
    # A very tight specular hot-spot so the top (white) tone stays a small dot.
    b += 0.30 * (diffuse ** 40)
    return max(0.0, min(1.0, b))


# Brightness thresholds picking a ramp stop. Tuned so the top stop (white
# highlight) covers only the small region facing the light, and the bottom
# stop (shadow) the far lower-left — the rest is the hue's mid tones.
THRESHOLDS = [0.30, 0.60, 0.86]  # < -> stop 0,1,2 ; >= last -> top stop


def color_for(b, ramp):
    """Map brightness 0..1 to an SGR fg code on the ramp via thresholds."""
    if b is None:
        return None
    for i, t in enumerate(THRESHOLDS):
        if b < t:
            return ramp[i]
    return ramp[-1]


def render(hue="green", radius=11):
    ramp = RAMPS.get(hue, RAMPS["green"])
    R = radius
    # Cells: width spans -R..R horizontally; each cell is 2 vertical pixels.
    # Horizontal step is doubled because character cells are ~half as wide as
    # tall, so we sample 2 horizontal units per cell to keep the sphere round.
    # Terminal cells are ~1:2 (w:h) and half-blocks give 2 vertical pixels per
    # cell, so a round sphere needs ~2x as many columns as rows. XR is the
    # horizontal radius in columns; R the vertical radius in rows.
    XR = 2 * R
    lines = []
    for row in range(-R, R + 1):
        cells = []
        for col in range(-XR, XR + 1):
            x = col / XR
            top = brightness(x, (row - 0.25) / R)
            bot = brightness(x, (row + 0.25) / R)
            ct = color_for(top, ramp)
            cb = color_for(bot, ramp)
            if ct is None and cb is None:
                cells.append(" ")
            else:
                fg = ct if ct is not None else 30
                bg = (cb if cb is not None else 30) + 10
                cells.append(f"\x1b[{fg};{bg}m▀")
        cells.append("\x1b[0m")
        lines.append("".join(cells))
    return "\n".join(lines) + "\n"


if __name__ == "__main__":
    hue = sys.argv[1] if len(sys.argv) > 1 else "green"
    rad = int(sys.argv[2]) if len(sys.argv) > 2 else 11
    sys.stdout.write(render(hue, rad))
