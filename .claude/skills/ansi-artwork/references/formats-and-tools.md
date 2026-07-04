# Formats and tools

## File formats

- **`.ans`** — a raw ANSI escape stream. No formal spec; it's scene convention.
  CP437 bytes historically, DOS CRLF line endings. Byte **0x1A** (Ctrl-Z) marks
  end-of-content and separates the art from an appended SAUCE record. This is the
  format `ansilove` and BBS clients read.
- **`.asc`** — plain CP437/ASCII with color/cursor codes stripped (SAUCE
  FileType 0). Not just "`.ans` minus metadata."
- **XBin (`.xb`)** — binary format (Tasmaniac/ACiD, 1996): 11-byte header with
  width/height, optional custom 16-color palette, optional custom font, optional
  row-wise RLE compression. Spec: acid.org/images/0896/XBIN.TXT.

## SAUCE metadata

**SAUCE** (Standard Architecture for Universal Comment Extensions, Tasmaniac/
ACiD, 1994) is a 128-byte record appended to art files. Layout: content → `0x1A`
→ optional `COMNT` comment block → 128-byte record at offset −128 from EOF.

Key fields: ID `"SAUCE"`, version `"00"`, Title(35), Author(20), Group(20),
Date(8, CCYYMMDD), FileSize(u32), DataType(u8), FileType(u8), TInfo1–4(u16),
Comments(u8), TFlags(u8), font name(22). **For character art: TInfo1 = width,
TInfo2 = lines.** TFlags bit 0 = iCE colors; bits 1–2 letter spacing; bits 3–4
aspect ratio. Spec: acid.org/info/sauce/sauce.htm.

You only need SAUCE when producing standalone `.ans` files for the scene/BBS
archives. Art embedded in source code doesn't use it.

## Editors (for drawing by hand)

| Tool | Platform | Notes |
|------|----------|-------|
| **Moebius** | Win/Linux/macOS | Modern Blocktronics editor; half-block "paint" brush. GUI only. |
| **PabloDraw** | Win/Linux/macOS (.NET) | Editor + multi-user collab; GUI only. |
| **icy_draw** | Win/Linux/macOS (Rust) | Modern GUI editor. |
| **Durdraw** | Linux/Unix/macOS | Terminal/curses editor; has `--export-ansi`. |
| **TheDraw** | DOS (DOSBox) | Classic; abandonware. Source of `.TDF` fonts. |

These are all interactive/GUI (except Durdraw). For **generating** art in code,
you're writing the escape sequences directly, not using an editor.

## ansilove — headless preview (the scriptable one)

`ansilove` is the one tool built for a non-interactive pipeline: it renders
`.ans` (and `.pcb/.bin/.adf/.idf/.tnd/.xb`) to **PNG**. Use it to put art in a
README or to eyeball a piece without a BBS client — don't screenshot a terminal.

```
ansilove -o preview.png art.ans      # basic render
ansilove -d -o preview.png art.ans   # -d = correct DOS aspect ratio
ansilove -i -o preview.png art.ans   # -i = iCE colors (16 backgrounds)
ansilove -R 2 -o preview.png art.ans # -R 2 = 2x scale
```

Packaged on Debian/Ubuntu (`apt install ansilove`), macOS (brew), BSD.

## Other useful tools

- **figlet** — plain-ASCII block-letter wordmarks. Fast monochrome banners.
- **tdfiglet** — renders TheDraw `.TDF` fonts (authentic colored-block logos).
- **chafa** — converts an **image → terminal ANSI/Sixel** (the opposite
  direction from ansilove). Good for a first-draft image-to-ANSI, then hand-clean
  it. Not a `.ans` previewer.
- **ansiart2utf8** (github.com/BourgeoisBear/ansiart2utf8) — converts CP437 `.ans`
  to UTF-8 for modern terminals.

## Rendering gotchas (recap)

- Raw CP437 high bytes to a UTF-8 terminal = mojibake. Emit Unicode/UTF-8, or
  convert (ansiart2utf8), or use a CP437 font.
- SyncTERM / NetRunner are CP437-first; modern telnet/SSH BBS software (ENiGMA½,
  x/84) translates for you.
- Preview with ansilove (→PNG) rather than screenshotting a terminal, so docs
  show the art consistently regardless of the reader's terminal font.
