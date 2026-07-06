# Immortal Barons

Immortal Barons is a turn-based strategy game for the terminal. You rule
a barony in a ruined, post-apocalyptic world. You buy land, feed your
people, build an army, and fight rival empires. The goal is to have the
highest net worth when the turns run out, or to conquer everyone else.

The game is written in Go. It is an independent remake, inspired by the
1990s BBS door game *Barren Realms Elite*. See [Heritage](#heritage) below.

> **Early work in progress.** Immortal Barons is not ready for real
> play-testing or inter-BBS (IBBS) play yet. Mechanics are still changing.

## Status

In development — not yet released. Immortal Barons is a persistent, multi-user
BBS door game: each caller has a lasting empire in one shared world; you play
your turns, and other barons (and any AI) act on their own turns. The game runs
locally or as a native BBS door.

**Works now:**

- Full menu system with single-key commands
- Persistent shared world: your empire is saved between calls, keyed by your
  BBS handle; one player at a time (exclusive lock); a daily maintenance step
- Per-turn economy (idle empires stagnate); turns-per-day and new-realm
  protection
- Economy: buy and sell land at a **rising market price**, buy food, recruit
  troopers, build jets/turrets/tanks/carriers, recruit covert agents
- Bank: deposit, withdraw, take a loan, repay, invest (about 1%/turn)
- War: conventional attack (offense vs. defense), plus **nuclear, chemical,
  and biological** strikes and **pirate raids**
- Covert ops (agent-count decides success): spy for intel, sabotage, and
  strikes on the enemy's intelligence, airbases, food stores, and HeadQuarters
- Region types (coastal, mountain, desert, river, agricultural, urban,
  industrial, technology) and a food market
- Diplomacy: typed treaties (defense, trade, intelligence) with their effects
- Sysop configuration editor
- Industry: set production percentages per unit, or specialize (permanent)
- Messages: player-to-player mail with a multi-line editor, and a planetary
  bulletin
- A planetary news feed: battles, nuclear/chemical/biological strikes, pirate
  raids, and tax riots are broadcast to everyone
- Write Macros: record a Ctrl-key to replay a sequence of keystrokes
- Localization: menus, prompts, and the help database render in the caller's
  language (English, German, and Russian so far)
- "While you were away" event log for asynchronous play
- Scores and net worth

**Planned (not built yet):**

- Leagues that reset on a timer and crown a winner
- The Quick Strike and Extended Battle attack variants
- Inter-BBS (IBBS) live play (the packet exchange — scores, news, and group
  attacks — works and is tested; validation under real BBS software is still
  pending)

## Build and run

You need Go 1.26 or newer. The game builds and runs anywhere Go does — Linux,
macOS, Windows, and the BSDs (DOS is not supported). It plays locally or as a
native BBS door.

```
go build -o barons ./cmd/barons
./barons
```

Or run it directly:

```
go run ./cmd/barons
```

## Running as a BBS door

Immortal Barons can run as a native door under modern BBS software such as
Synchronet or Mystic. Build the door front-end:

```
go build -o barons-door ./cmd/barons-door
```

The BBS writes a dropfile when a caller starts the door. Configure your BBS
to run `barons-door` and pass the dropfile path:

```
barons-door -dropfile /path/to/node/DOOR32.SYS
```

It reads `DOOR32.SYS` (preferred) or `DOOR.SYS` to learn the caller's name,
node, time left, and whether their terminal supports ANSI. The BBS connects
the caller to the door over standard input and output. With no `-dropfile`,
it looks for `DOOR32.SYS` or `DOOR.SYS` in the working directory.

Callers share one persistent world: each caller's empire is saved between
calls, keyed by their BBS handle. A nightly maintenance step
(`barons-door -maint`) advances the world.

Full setup instructions for local and inter-BBS play — registering the door,
daily maintenance, the node list, and starting a fresh game with
`barons-door -reset` — are in [`docs/sysop-guide.md`](docs/sysop-guide.md).

## Running in a browser (experimental)

A browser front-end serves the game over Server-Sent Events with xterm.js:

```
go build -o barons-web ./cmd/barons-web
./barons-web -addr :8080
```

Open `http://localhost:8080`. Each browser gets its own game session. This is
early and loads xterm.js from a CDN; the assets should be vendored (or
integrity-pinned) before any real deployment.

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
Patel and later maintained by [John Dailey](https://www.johndaileysoftware.com/).
This project shares that game's
ideas and feel, but it does not use any of its code, text, or artwork.
Game rules and mechanics are not covered by copyright, so this is a fresh
implementation of the concept. It is not affiliated with, or endorsed by,
the original authors.

## Tools

Immortal Barons is made with these open-source tools:

- The title banner uses the "ANSI Shadow" FIGlet font, rendered with
  [figlet](https://github.com/damienbutt/figlet) (a Go version of the classic
  tool).
- ANSI art is previewed by rendering it to a PNG image:
  [ansilove](https://www.ansilove.org/) for 16-color CP437 pieces, and
  [textimg](https://github.com/jiro4989/textimg) for 256-color / UTF-8 art
  like the title splash.
- The documentation is translated with [po4a](https://po4a.org/).

## License

Released into the public domain under the [Unlicense](LICENSE).

## Related Realms

* [Medusa's Barren Realms Elite (BRE) Discord](https://discord.gg/5wTZpstcjM)
