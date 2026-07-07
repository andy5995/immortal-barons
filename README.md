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

To install Go, download an official installer or tarball from
<https://go.dev/doc/install>. That same page also has the step-by-step install
instructions for Windows, macOS, Linux, and the BSDs, and always offers a
current version. On macOS you can also run `brew install go`; on any Linux with
snap (preinstalled on Ubuntu), `snap install go --classic` tracks a current Go
too. On Linux and the BSDs the
package manager's Go is usually called `go` or `golang` (for example `go` on
Arch, Alpine, and FreeBSD; `golang` on Debian, Ubuntu, and Fedora). A distro's
package can be older than 1.26, though — if it is, use the official tarball.

```
go build ./cmd/immortal-barons
./immortal-barons -local
```

On Windows the binary is `immortal-barons.exe`. Run it as `immortal-barons -local`
from Command Prompt, or `./immortal-barons -local` from PowerShell.

Or run it directly:

```
go run ./cmd/immortal-barons -local
```

## Running as a BBS door

Immortal Barons can run as a native door under modern BBS software such as
Synchronet or Mystic. The BBS writes a dropfile (`DOOR32.SYS` or `DOOR.SYS`)
when a caller starts the door, and the caller shares one persistent world with
everyone else on the board — each empire is saved between calls, keyed by its
BBS handle.

Registering the door, the dropfile, daily maintenance (`immortal-barons -maint`),
the node list, inter-BBS play, and starting a fresh game (`immortal-barons -reset`)
are all covered in the
[Door Setup guide](https://andy5995.github.io/immortal-barons/door-setup/).

## Running in a browser (experimental)

Immortal Barons also has an experimental browser front-end (`immortal-barons-web`),
separate from the BBS door. See the
[Web Server Guide](https://andy5995.github.io/immortal-barons/web-server/) for
how to build and run it, how players are tracked, the full list of options, and
why its world is separate from a BBS door's world.

## How to play

The game uses single keypresses — you do not press Enter to choose a menu
item. See the
[player's guide](https://andy5995.github.io/immortal-barons/guide/) for the
menus, the controls, and strategy.

## Design

The design notes are in
[`docs/superpowers/specs/`](docs/superpowers/specs/).

The code is split so that the game logic does not know how it is shown to
the player. The engine only reads keypresses from, and writes text to, a
`Session`. This makes it possible to add a BBS-door front-end and a web
front-end later without changing the game rules.

- `cmd/immortal-barons` — the door and local terminal front-end (`-local` for local play)
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
* [X-Bit BBS](https://x-bit.org/info)
