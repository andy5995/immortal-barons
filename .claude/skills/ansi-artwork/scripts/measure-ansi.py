#!/usr/bin/env python3
"""measure-ansi.py -- measure real ANSI art or screens, instead of asserting.

Counts what a capture actually DOES: how it spends the palette, which glyph
classes carry tone versus detail, whether colour seams are blended or hard-cut,
whether text is readable, and whether lines will wrap at column 80.

    measure-ansi.py FILE [FILE...]        one block per file
    measure-ansi.py --csv FILE [FILE...]  one row per file, for comparing many

Input is a raw capture (CP437 bytes with ANSI SGR), e.g. a syncterm capture or
`tmux capture-pane -ep`.  Nothing is written; it only reports.
"""
import argparse, math, re, sys
from collections import Counter

#  CP437 block/shade glyphs, by role.
SOLID  = {0xdb: '█'}
SHADE  = {0xb0: '░', 0xb1: '▒', 0xb2: '▓'}
VHALF  = {0xdc: '▄', 0xdf: '▀'}          # split top/bottom -- vertical detail
HHALF  = {0xdd: '▌', 0xde: '▐'}          # split left/right -- rarely worth it
BLOCKS = {**SOLID, **SHADE, **VHALF, **HHALF}
#  Everything else in the CP437 line-draw span is box/UI furniture.
BOXDRAW = set(range(0xb3, 0xdb)) - set(BLOCKS)
DOUBLE  = {0xc9, 0xbb, 0xc8, 0xbc, 0xcd, 0xba, 0xcc, 0xb9, 0xcb, 0xca, 0xce}

NAMES = ['black', 'red', 'green', 'yellow', 'blue', 'magenta', 'cyan', 'white']
#  The classic CGA/ANSI palette most terminals and BBS clients approximate.
RGB = [(0,0,0),(170,0,0),(0,170,0),(170,85,0),(0,0,170),(170,0,170),(0,170,170),(170,170,170),
       (85,85,85),(255,85,85),(85,255,85),(255,255,85),(85,85,255),(255,85,255),(85,255,255),(255,255,255)]


def luminance(rgb):
    def ch(v):
        v /= 255
        return v / 12.92 if v <= 0.03928 else ((v + 0.055) / 1.055) ** 2.4
    r, g, b = (ch(x) for x in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


CONTRAST = {}
for _f in range(16):
    for _b in range(16):
        _l1, _l2 = sorted((luminance(RGB[_f]), luminance(RGB[_b])), reverse=True)
        CONTRAST[(_f, _b)] = (_l1 + 0.05) / (_l2 + 0.05)


def name(idx):
    return ('bright ' if idx >= 8 else '') + NAMES[idx % 8]


def parse(data):
    """Yield (byte, fg_index, bg_index) for every printable cell, tracking SGR."""
    bold = False
    fg, bg = 7, 0
    i, n = 0, len(data)
    sgr = blink = 0
    while i < n:
        if data[i] == 0x1b and i + 1 < n and data[i + 1] == 0x5b:
            m = re.match(rb'\x1b\[([0-9;]*)m', data[i:])
            if m:
                sgr += 1
                for p in (m.group(1) or b'0').split(b';'):
                    v = int(p or 0)
                    if v == 0:
                        bold, fg, bg = False, 7, 0
                    elif v == 1:
                        bold = True
                    elif v == 5:
                        blink += 1
                    elif v == 22:
                        bold = False
                    elif 30 <= v <= 37:
                        fg = v - 30
                    elif 40 <= v <= 47:
                        bg = v - 40
                i += m.end()
                continue
            m = re.match(rb'\x1b\[[0-9;?]*[A-Za-z]', data[i:])
            if m:
                i += m.end()
                continue
        yield data[i], fg + (8 if bold else 0), bg
        i += 1
    yield None, sgr, blink          # sentinel carries the totals


def measure(path):
    data = open(path, 'rb').read()
    g = Counter(); pair = Counter(); textpair = Counter()
    same = Counter(); cross = Counter()
    box = dbl = 0
    seq = []                        # (byte, fg, bg) for adjacency work
    sgr = blink = 0
    for b, f, bgc in parse(data):
        if b is None:
            sgr, blink = f, bgc
            break
        if b in BLOCKS:
            g[BLOCKS[b]] += 1
            pair[(f, bgc)] += 1
            (same if (f % 8) == bgc else cross)[BLOCKS[b]] += 1
            seq.append((BLOCKS[b], f, bgc))
        elif b in BOXDRAW:
            box += 1
            if b in DOUBLE:
                dbl += 1
        elif 33 <= b < 127:
            textpair[(f, bgc)] += 1

    #  Seam blending: for adjacent block cells whose HUE changes, is the
    #  boundary a shade/half glyph (blended) or a solid one (hard cut)?
    blended = hard = 0
    for (g1, f1, b1), (g2, f2, b2) in zip(seq, seq[1:]):
        if (f1 % 8, b1) != (f2 % 8, b2):
            if g2 in '░▒▓▀▄▌▐' or g1 in '░▒▓▀▄▌▐':
                blended += 1
            else:
                hard += 1

    #  Colour-run length: cells drawn before the attribute changes.
    runs, cur = [], 1
    for (_, f1, b1), (_, f2, b2) in zip(seq, seq[1:]):
        if (f1, b1) == (f2, b2):
            cur += 1
        else:
            runs.append(cur); cur = 1
    runs.append(cur)

    #  Widest rendered line -- over 79 wraps on an 80-column terminal. Only
    #  meaningful when the capture HAS line structure: a raw stream with few
    #  breaks reports one enormous "line" and the number means nothing, so it
    #  is reported as unknown rather than as a false wrap warning.
    plain = re.sub(rb'\x1b\[[0-9;?]*[A-Za-z]', b'', data)
    rows = plain.replace(b'\r\n', b'\n').replace(b'\r', b'\n').split(b'\n')
    widest = max((len(l) for l in rows), default=0)
    if len(rows) < len(plain) / 200:      # fewer breaks than ~one per 200 bytes
        widest = None

    tb = sum(g.values()); tt = sum(textpair.values())
    return dict(path=path, bytes=len(data), blocks=tb, box=box, double=dbl, sgr=sgr,
                blink=blink, text=tt, glyphs=g, pair=pair, textpair=textpair,
                same=sum(same.values()), cross=sum(cross.values()),
                blended=blended, hard=hard, runs=runs, widest=widest)


def pct(a, b):
    return f'{a * 100 / b:.0f}%' if b else '  -'


def report(m):
    tb, tt = m['blocks'], m['text']
    print(f"\n=== {m['path']}   {m['bytes']:,} bytes ===")
    print(f"  block/shade {tb:,}   box-draw {m['box']:,} ({pct(m['double'], m['box'])} double-line)"
          f"   text {tt:,}   blink {m['blink']}")
    if not tb:
        print('  (no block art)')
        return
    g = m['glyphs']
    sh = g['░'] + g['▒'] + g['▓']; vh = g['▀'] + g['▄']; hh = g['▌'] + g['▐']
    print(f"  ROLE      same-hue {pct(m['same'], tb)}   cross-hue {pct(m['cross'], tb)}")
    print(f"  GLYPH     solid {pct(g['█'], tb)}   shade {pct(sh, tb)}"
          f"   vert-half {pct(vh, tb)}   horiz-half {pct(hh, tb)}")
    seams = m['blended'] + m['hard']
    if m['sgr'] == 0:
        print("  SEAMS     n/a -- capture carries no colour (0 SGR sequences); the")
        print("            role/seam figures below are meaningless without it")
    else:
        print(f"  SEAMS     {pct(m['blended'], seams)} blended, {pct(m['hard'], seams)} hard-cut  ({seams:,} hue changes)")
    runs = m['runs']
    runs_sorted = sorted(runs)
    print(f"  RUNS      mean {sum(runs)/len(runs):.1f} cells/attribute, median {runs_sorted[len(runs)//2]}"
          f"   SGR/cell {m['sgr']/tb:.2f}")
    fgs = {f for f, _ in m['pair']}; bgs = {b for _, b in m['pair']}
    w = m['widest']
    wtxt = 'no line structure' if w is None else str(w) + ('  <-- WRAPS at 80' if w > 79 else '')
    print(f"  PALETTE   {len(fgs)}/16 foregrounds, {len(bgs)}/8 backgrounds   widest line {wtxt}")
    if tt:
        bad = sum(n for (f, b), n in m['textpair'].items() if CONTRAST[(f, b)] < 4.5)
        worst = sorted(((CONTRAST[(f, b)], f, b, n) for (f, b), n in m['textpair'].items() if n > tt * 0.005))[:3]
        print(f"  TEXT      {pct(bad, tt)} of text cells below 4.5:1 contrast")
        for c, f, b, n in worst:
            print(f"              {c:4.1f}:1  {name(f)} on {name(b)}  ({pct(n, tt)} of text)")


def main():
    ap = argparse.ArgumentParser(description=__doc__,
                                 formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument('files', nargs='+')
    ap.add_argument('--csv', action='store_true')
    a = ap.parse_args()
    ms = [measure(f) for f in a.files]
    if a.csv:
        print('file,blocks,same_pct,solid_pct,shade_pct,vhalf_pct,blended_pct,sgr_per_cell,widest')
        for m in ms:
            tb = m['blocks'] or 1; g = m['glyphs']
            sh = g['░'] + g['▒'] + g['▓']; vh = g['▀'] + g['▄']
            seams = (m['blended'] + m['hard']) or 1
            print(f"{m['path']},{m['blocks']},{m['same']*100//tb},{g['█']*100//tb},"
                  f"{sh*100//tb},{vh*100//tb},{m['blended']*100//seams},"
                  f"{m['sgr']/tb:.2f},{m['widest'] if m['widest'] is not None else ''}")
    else:
        for m in ms:
            report(m)


if __name__ == '__main__':
    main()
