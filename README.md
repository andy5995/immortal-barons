# Immortal Barons

Immortal Barons is a turn-based strategy game for the terminal. You rule
a barony in a ruined, post-apocalyptic world. You buy land, feed your
people, build an army, and fight rival empires. The goal is to have the
highest net worth when the turns run out, or to conquer everyone else.

The game is written in Go. It is an independent remake, inspired by the
1990s BBS door game *Barren Realms Elite*. See [Heritage](#heritage) below.

## Status

Early, but playable. You can play a full single-player game against three
computer empires from start to finish.

**Works now:**

- Full menu system with single-key commands
- Economy: buy and sell land, buy food, recruit troopers, build jets and tanks
- Bank: deposit, withdraw, take a loan, repay, invest
- War: attack a rival empire, with plunder and captured land
- Turn engine: taxes, food, army upkeep, population growth, bank interest
- Three computer empires that grow and attack
- Scores, net worth, and win/lose endings

**Planned (not built yet):**

- Nuclear, chemical, and biological attacks
- Covert operations, trading, diplomacy, and messages
- Inter-BBS (IBBS) play: sending game data between servers
- A BBS "door" front-end and a web (browser) front-end

## Build and run

You need Go 1.26 or newer.

```
go build -o barons ./cmd/barons
./barons
```

Or run it directly:

```
go run ./cmd/barons
```

## How to play

The game uses single keypresses. You do not press Enter to choose a menu
item. When the game asks for a number (for example, how much land to buy),
type the number and press Enter.

From the main menu:

- `B` — Buy or sell land, food, and military units
- `K` — Visit the bank
- `W` — Attack a rival empire
- `D` — Display your empire status and the scores
- `N` — End your turn (this runs the daily update for the whole world)
- `Q` — Quit

Each game lasts 20 turns. Build your army before you attack. A rival
defends its home land, so a weak attack will lose. Watch your food: if
your people run out of food, they starve and your empire shrinks.

## Design

The design notes are in
[`docs/superpowers/specs/`](docs/superpowers/specs/).

The code is split so that the game logic does not know how it is shown to
the player. The engine only reads keypresses from, and writes text to, a
`Session`. This makes it possible to add a BBS-door front-end and a web
front-end later without changing the game rules.

- `cmd/barons` — the local terminal front-end
- `internal/session` — the input/output abstraction
- `internal/ansi` — terminal color and screen codes
- `internal/menu` — the menu engine and the menu tree
- `internal/game` — the world, economy, turn engine, and combat

## Heritage

Immortal Barons is a tribute to *Barren Realms Elite*, created by Mehul
Patel and later maintained by John Dailey. This project shares that game's
ideas and feel, but it does not use any of its code, text, or artwork.
Game rules and mechanics are not covered by copyright, so this is a fresh
implementation of the concept. It is not affiliated with, or endorsed by,
the original authors.

## License

Released into the public domain under the [Unlicense](LICENSE).
