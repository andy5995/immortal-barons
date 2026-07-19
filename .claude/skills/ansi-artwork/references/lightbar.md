# Lightbar menus

A **lightbar** is a menu where one item is drawn highlighted — a reverse-video
or colored bar sits over the current choice — and the user moves that bar with
the arrow keys, pressing Enter to select. It is the interactive cousin of the
static art in the rest of this skill: half of it is *rendering* (drawing and
repainting the bar), half is an *input loop* (reading arrow keys and tracking
which item is lit).

Contrast with a **hotkey menu**, where each item has a letter/number and one
keypress acts immediately (no highlight, no arrows). Hotkey menus are the older
DOS-BBS convention; many classic doors (including BRE) use them. A lightbar is a
more modern feel. The two are not exclusive — the best lightbar menus *also*
accept the hotkey for each item, so they degrade gracefully when arrow keys are
unavailable (see "Always keep a hotkey fallback" below).

This is a genuinely different medium from static ANSI art: nothing here renders
correctly unless the terminal is in **raw mode** and you are parsing input byte
by byte. Both halves are covered below.

## The rendering half

### The bar is a full-width background rectangle, not just colored text

The highlight must read as a solid bar, so set a **background color** and pad the
item text to a **fixed width** (the width of the widest item, or the menu box
interior). Coloring only the glyphs of the text leaves a ragged highlight.

- Quick way: reverse video — `\x1b[7m` swaps fg/bg for whatever the current
  colors are, then `\x1b[27m` (or `\x1b[0m`) turns it off. Cheap and portable.
- More control: set explicit colors, e.g. `\x1b[47m\x1b[30m` (white bg, black
  fg) for the lit row, and the menu's normal colors for the rest. Explicit
  colors let the bar match a scene palette and behave predictably across
  clients (some render reverse video oddly).
- Pad to width: print the text, then enough trailing spaces to fill the row, so
  the background color extends the full bar — `"  Attack        "` not
  `"  Attack"`.

An unselected row uses the menu's normal fg/bg; the selected row uses the bar
colors. That difference *is* the lightbar.

### Repainting when the bar moves — two strategies

1. **Full redraw (simplest).** After every move, reposition the cursor to the
   menu's top row and reprint every item, choosing bar-colors for the newly
   selected one. On modern terminals a single buffered write repaints without
   visible flicker. This is the easiest to get right and the recommended default
   — you never have to track individual row positions.

2. **Minimal repaint (two cells of work).** Only the old row and the new row
   change, so un-highlight the row you left and highlight the row you entered,
   using absolute cursor positioning to jump straight to each. This needs you to
   know the menu's **origin row** on screen so you can compute each item's row as
   `originRow + index`. Faster over a slow link, but more state to manage.

### Positioning without disturbing the rest of the screen

- **Absolute move:** `\x1b[<row>;<col>H` puts the cursor at an exact cell (rows
  and columns are 1-based). This is how you jump to a specific menu row for a
  minimal repaint, or back to the menu top for a full redraw.
- **Save / restore:** `\x1b[s` saves the cursor position, `\x1b[u` restores it —
  handy to bracket a repaint so the cursor returns to where the rest of your
  output expects it. (Support is near-universal but not guaranteed; absolute
  positioning is the more reliable primitive.)
- **Hide the caret while navigating:** `\x1b[?25l` hides the cursor, `\x1b[?25h`
  shows it again. Do this around the menu loop so a blinking caret doesn't sit
  inside the bar. Always restore it on exit, including on error paths.

## The input half

### Raw mode is mandatory

In the terminal's default **canonical (cooked) mode**, input is line-buffered:
your program receives nothing until the user presses Enter, and keystrokes are
echoed by the terminal. A lightbar needs each keypress *immediately and
unechoed*, which means **raw mode** — disable canonical mode and echo before the
loop, restore the previous settings after.

- On Unix this is `termios` (`cfmakeraw`, or clear `ICANON`/`ECHO`); Go's
  `golang.org/x/term.MakeRaw` does it. Windows uses `SetConsoleMode`.
- **The door/BBS trap:** when the game runs behind a pty or a BBS that hands you
  a line-buffered stream, *you* may not own the tty, and it can arrive in
  canonical mode — so arrow keys queue up until Enter and the lightbar appears
  frozen. This repo hit exactly that: a door attached to a pty in canonical mode
  line-buffered every keypress. If a lightbar "doesn't respond to arrows,"
  suspect cooked mode first.

### Arrow keys are multi-byte escape sequences

An arrow key does not send one byte — it sends an escape sequence:

| Key   | Normal (CSI)      | Application (SS3)  |
|-------|-------------------|--------------------|
| Up    | `ESC [ A` = `\x1b[A` | `ESC O A` = `\x1bOA` |
| Down  | `ESC [ B` = `\x1b[B` | `ESC O B` = `\x1bOB` |
| Right | `ESC [ C` = `\x1b[C` | `ESC O C` = `\x1bOC` |
| Left  | `ESC [ D` = `\x1b[D` | `ESC O D` = `\x1bOD` |

**Accept both forms.** In *application cursor key* mode (which some terminals,
keypads, and BBS clients enable) the middle byte is `O`, not `[`. Handling only
`\x1b[A` means arrows silently break on those clients. Optional longer
sequences: Home `\x1b[H`, End `\x1b[F`, PgUp `\x1b[5~`, PgDn `\x1b[6~`.

### The bare-ESC ambiguity

`\x1b` (ESC, 0x1b) is both the **first byte of every arrow sequence** *and* the
**Escape key** pressed on its own. When you read an ESC you must look at what
follows:

- Next byte is `[` or `O` → it's a control sequence; read the final letter to
  decide which arrow.
- Nothing follows (within a short read timeout) → it was a real Escape press
  (use it to cancel/back out).

With a blocking reader, read the next byte to disambiguate; for standalone-ESC
support, put a small timeout on that follow-up read so a lone Escape doesn't hang
waiting for a `[` that never comes.

### Bytes can split across reads

Over telnet, a socket, or a door pipe, a single `\x1b[A` may arrive in more than
one `read()` — you cannot assume the whole three bytes land together. Feed input
through a small **incremental parser / buffer**: accumulate bytes, recognize a
complete sequence when you have it, and keep leftover bytes for the next round.
Do not `read(3)` and assume you got a whole arrow.

### The state machine

Maintain a `selected` index over `n` items:

- **Up:** `selected = (selected - 1 + n) % n` (the `+ n` before `% n` keeps it
  non-negative so it wraps to the bottom).
- **Down:** `selected = (selected + 1) % n` (wraps to the top).
- **Enter / Space:** return `selected` as the chosen item.
- **A hotkey byte** matching an item: jump straight to it (select or act).
- **Escape:** cancel, if the menu allows backing out.

After any move, repaint the highlight (full redraw or minimal), then wait for the
next key.

## Always keep a hotkey fallback

Accept each item's letter/number as a shortcut *alongside* the arrows. Three
reasons:

1. **Robustness** — if a client sends arrow sequences you don't recognize, or the
   tty is stuck cooked, the user can still drive the menu by hotkey.
2. **Speed** — experienced users jump directly instead of arrowing down a long
   list. This is the BRE/DOS-door muscle memory.
3. **Accessibility** — not every keyboard or assistive setup produces clean arrow
   escapes.

A lightbar with hotkeys is strictly better than either alone: it looks modern,
degrades to a plain hotkey menu, and never traps the user.

## Portability caveats

- **Raw mode required** — and you may not own the tty in a door/BBS context (see
  the cooked-mode trap above).
- **Two arrow encodings** — handle `\x1b[A` *and* `\x1bOA`.
- **Split reads** — parse incrementally; never assume a whole sequence per read.
- **Reverse video vs explicit colors** — legacy CP437 clients may render
  `\x1b[7m` differently than expected; explicit fg/bg is more predictable. Treat
  fancy bar coloring as progressive enhancement, plain reverse video as the
  floor.
- **Restore terminal state on every exit path** — cursor visibility and the saved
  termios settings must be put back even when the menu is left via error or
  signal, or you leave the user's shell in raw mode with a hidden cursor.

## Minimal loop (language-neutral sketch)

```
saved = enterRawMode(tty)
hideCursor()
defer { showCursor(); restore(tty, saved) }

selected = 0
render(items, selected)          // full redraw: print each row, bar-color the selected one
loop:
    b = readByte(tty)
    if b == ESC:
        n = readByteWithTimeout(tty)
        if n == '[' or n == 'O':
            switch readByte(tty):
                'A': selected = (selected - 1 + len) % len; render(items, selected)
                'B': selected = (selected + 1) % len;       render(items, selected)
                // 'C'/'D' left/right if the menu uses them
        else:
            return CANCELLED          // bare ESC
    else if b == '\r' or b == '\n' or b == ' ':
        return selected
    else if hotkey := matchHotkey(items, b):
        return hotkey
```

Everything IB-specific (how items are defined, how a choice is dispatched) is
menu-engine territory, not this skill's; the sketch stays generic on purpose.
