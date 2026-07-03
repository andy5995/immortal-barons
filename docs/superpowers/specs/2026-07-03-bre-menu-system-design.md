# BRE Clone — Menu System Design

Date: 2026-07-03
Status: approved (design), implementation in progress

## Background

Barren Realms Elite (BRE) is a 1990s inter-BBS strategy "door game"
originally created by Mehul Patel and later maintained by John Dailey.
Players run a barony/empire: buy land and food, manage people and gold,
build armies (troopers, jets, tanks, carriers, bombers), run covert ops,
trade, form alliances, and wage war (conventional, nuclear, chemical,
biological "Gooie Kablooie"). Its defining feature is *inter-BBS* play:
turn and attack data are exchanged as packets between participating
boards.

This clone is written from scratch in Go. Game *rules and mechanics* are
not copyrightable; only the original's specific code, art, and text are.
No verbatim copyrighted display text or art from the original is copied
into this project. The extracted strings were used only as behavioral
reference to reconstruct the menu structure.

## Goals for this slice

Build the **menu system**: a reusable, presentation-abstracted menu
engine plus the full BRE menu tree, runnable from a local console. Every
leaf action is a stub until the game logic behind it exists.

Out of scope for this slice (deferred): the turn/economy engine, combat
math, IBBS packet exchange, the BBS-door front-end (dropfile + socket),
the web front-end (xterm.js + websocket), save/load, and sysop tools.

## Architecture

One Go engine, multiple front-ends. The engine only ever reads keypresses
from, and writes ANSI bytes to, an abstract `Session` (a byte stream). No
game code knows whether that stream is a local console, a BBS socket, or a
websocket. Chosen web strategy is terminal-in-browser (xterm.js renders the
same ANSI bytes), so there is a single rendering path for all front-ends.

### Package layout

```
bre/
├── go.mod                      module: github.com/andy5995/bre
├── cmd/
│   └── bre-local/main.go       wires a local console Session, starts the menu loop
├── internal/
│   ├── session/
│   │   ├── session.go          Session interface (ReadKey + io.Writer)
│   │   └── console.go          local stdin/stdout implementation (raw keypress mode)
│   ├── ansi/
│   │   └── ansi.go             color, clear-screen, cursor, box-drawing helpers
│   └── menu/
│       ├── menu.go             Menu, Item, Action, Result + generic Run loop
│       ├── tree.go             the BRE menu tree (main + submenus), stubbed actions
│       └── menu_test.go        scripted-keystroke tests
```

`session` and `ansi` carry no BRE knowledge (reusable, testable). `menu.go`
is the generic framework; `tree.go` is the BRE-specific content. That split
is the seam that lets the tree grow without touching the engine.

## Menu framework API

```go
// session package
type Session interface {
    ReadKey() (rune, error) // one keypress, de-buffered
    io.Writer               // engine writes ANSI bytes here
}
```

```go
// menu package
type Result int
const (
    Stay Result = iota // redraw current menu
    Back               // pop to parent menu
    Quit               // exit the session
)
// Goto(*Menu) Result pushes a submenu onto the stack.

type Action func(s session.Session, g *Game) Result

type Item struct {
    Key    rune
    Label  string
    Do     Action           // nil for a heading/separator
    Hidden func(*Game) bool  // nil = always shown
}

type Menu struct {
    Title  string
    Items  []Item
    Status func(*Game) string // optional status bar under the menu
}
```

`Run(s, g, root)` keeps a menu stack: draw top menu, read a key, match an
`Item`, call `Do`, act on the `Result` (push via `Goto` / pop via `Back` /
`Quit` / redraw via `Stay`). The loop stays flat — no recursion — so it is
easy to reason about and to test with a scripted `Session`.

For this slice, `Game` is a minimal placeholder struct; leaf actions print
`"[<name> — not yet implemented]"` and return `Stay`.

## Menu tree

Reconstructed from the original's strings. Hotkeys and a few groupings are a
reasonable reconstruction, adjustable freely since all actions are stubs.

- **Main:** (B) Buy/Sell, (K) Bank, (W) War/Attack, (C) Covert, (T) Trading,
  (R) Diplomacy, (M) Messages, (D) Display, (P) Preferences,
  (Y) Sysop/Coordinator [gated], (?) Instructions, (Q) Quit/End Turn.
- **Buy:** Buy Land, Buy Food, Recruit Troopers, Build Jets/Tanks/Carriers/
  Bombers, Build HeadQuarters, Sell, Visit Bank, Return.
- **Bank:** Deposit, Withdraw, Take Loan, Repay Loan, Invest, Return.
- **Attack:** Regular, Nuclear, Chemical, Biological, Attack Pirates,
  Create Group Attack, Join Group Attack, Terrorist Ops, Gooie Kablooie Ops,
  SDI Program, Travel Times, Return.
- **Covert:** Send Spy, Spy on Relations, Spy Database, Bribery,
  Special Operations, Visit Bank, Return.
- **Trading:** Food Market, Send Trade Deal, View IPScores, Buy/Sell,
  Visit Bank, Return.
- **Diplomacy:** Modify Diplomacy, View Diplomacy, Diplomacy List, Return.
- **Messages:** Read Messages, Send Message, Planetary Post, Return.
- **Display:** Empire Status, Visit Advisors, See Scores, InterBBS Scores,
  Spy Database, Diplomacy List, Travel Times, Return.
- **Preferences (toggles):** Enter exits Buy menu, Deposit gold end-of-turn,
  Auto-pay maintenance, Auto-feed people/army, Return.
- **Coordinator (gated by a coordinator flag; hidden in normal play):**
  Configuration Editor, Modify League Diplomacy, Player List, Return.

## Testing

Because `Run` takes a `Session` interface, tests feed a scripted key
sequence through a fake session and assert on captured output bytes — no
real terminal needed. Cover: entering and leaving each submenu, `Quit` from
the main menu, an unknown key being ignored, and a `Hidden` item not being
selectable.
