#!/usr/bin/env python3
"""Print an ANSI art reference card to the terminal: the 16-color palette,
the shade ramp, and a half-block sub-pixel demo. Stdlib only, no arguments.

Run it in the terminal you'll be viewing/testing your art in, so the colors
you pick match what you'll actually see. Assumes a UTF-8 terminal (emits the
Unicode block glyphs directly)."""

RESET = "\x1b[0m"

# (index, name, sgr_fg, sgr_bg)
COLORS = [
    (0, "black", 30, 40), (1, "red", 31, 41),
    (2, "green", 32, 42), (3, "yellow/brown", 33, 43),
    (4, "blue", 34, 44), (5, "magenta", 35, 45),
    (6, "cyan", 36, 46), (7, "white", 37, 47),
    (8, "br black", 90, 100), (9, "br red", 91, 101),
    (10, "br green", 92, 102), (11, "br yellow", 93, 103),
    (12, "br blue", 94, 104), (13, "br magenta", 95, 105),
    (14, "br cyan", 96, 106), (15, "br white", 97, 107),
]

SHADES = [" ", "░", "▒", "▓", "█"]  # (space) ░ ▒ ▓ █


def rule(title):
    print(f"\n\x1b[1m{title}\x1b[0m")


def palette():
    rule("16-color palette  (index / name / fg swatch / bg swatch)")
    for i, name, fg, bg in COLORS:
        fg_sw = f"\x1b[{fg}m███{RESET}"
        bg_sw = f"\x1b[{bg}m   {RESET}"
        print(f"  {i:>2}  {name:<12}  fg {fg_sw}  bg {bg_sw}  (fg={fg} bg={bg})")


def ramp():
    rule("shade ramp  (space / ░ / ▒ / ▓ / █  =  0/25/50/75/100%)")
    for i, name, fg, _bg in COLORS[1:8] + COLORS[9:16]:
        cells = "".join(f"\x1b[{fg}m{s * 4}" for s in SHADES)
        print(f"  {name:<12} {cells}{RESET}")


def halfblock():
    rule("half-block demo  (▀: top=fg, bottom=bg  -> two pixels per cell)")
    print("  a vertical gradient in one row of cells, blue family:")
    # Each pair is (top fg SGR, bottom fg SGR); the bottom becomes a bg via +10
    # (30->40, 94->104, ...). Ramp: black, blue, bright blue, bright white.
    pairs = [(30, 34), (34, 94), (94, 97), (97, 94), (94, 34), (34, 30)]
    cells = "".join(f"\x1b[{top}m\x1b[{bot + 10}m▀▀▀▀" for top, bot in pairs)
    print(f"  {cells}{RESET}")
    print("  (each cell shows the top color over the bottom color; stack rows")
    print("   of these for smooth curves at double vertical resolution)")


if __name__ == "__main__":
    palette()
    ramp()
    halfblock()
    print()
