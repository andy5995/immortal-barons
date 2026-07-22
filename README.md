# Immortal Barons

Immortal Barons is a turn-based strategy game for the terminal. You rule
a barony in a ruined, post-apocalyptic world. You buy land, feed your
people, build an army, and fight rival empires. The goal is to have the
highest net worth when the turns run out, or to conquer everyone else.

The game is written in Go. It is an independent remake, inspired by the
1990s BBS door game *Barren Realms Elite*. See [Heritage](#heritage) below.

> **Early release.** The game is playable but not finished. Game values are
> still being balanced, and live inter-BBS (IBBS) play is not ready yet.

## Status

Immortal Barons is a persistent, multi-user BBS door game: each caller has a
lasting empire in one shared world; you play your turns, and other barons (and
any AI) act on their own turns. The game runs locally or as a native BBS door.

The core game is already playable — economy, military and war, covert
operations, diplomacy, and messaging all work, but the game is not yet balanced.
Many values do not match BRE's yet, so parts of the game are far too easy or too
hard. Larger features, such as leagues and live inter-BBS play, are still to
come.

Learn more, follow progress, or get involved:

- [Website](https://andy5995.github.io/immortal-barons/)
- [Issue tracker](https://github.com/andy5995/immortal-barons/issues)
- [Discussions](https://github.com/andy5995/immortal-barons/discussions)

We strongly recommend reading the [FAQ](docs/faq.md) first.

## Running

The game plays locally in your terminal or as a native BBS door. Once you have
the `immortal-barons` binary — download it (see below) or build it from source —
play locally:

```
./immortal-barons -local
```

On Windows the binary is `immortal-barons.exe`. Run it as `immortal-barons -local`
from Command Prompt, or `./immortal-barons -local` from PowerShell. From a source
checkout you can also run it without building first with
`go run ./cmd/immortal-barons -local`.

Run `immortal-barons -help` (or `-h`) to see all the command-line options; the
[Command Reference](docs/command-reference.md) explains every option in one
place. To run the game as a native BBS door instead, see "Running as a BBS door"
below.

## Download

Tagged releases are on the
[releases page](https://github.com/andy5995/immortal-barons/releases).

**Development snapshots** are also available. Snapshots are rebuilt
automatically from the latest source, so they change often and may be less
stable than a tagged release. Get them from the
[snapshot release](https://github.com/andy5995/immortal-barons/releases/tag/snapshot).

Builds are provided for Windows (32-bit and 64-bit), Linux (x86-64 and ARM64),
and macOS (Intel and Apple Silicon). The binaries are not signed, so Windows may
show a SmartScreen warning the first time you run one.

To build from source instead, see below.

## Build

You only need this to build from source; if you downloaded a binary above, skip
it. To build, you need Go 1.26 or newer. The game builds anywhere Go does —
Linux, macOS, Windows, and the BSDs (DOS is not supported).

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
```

This produces the `immortal-barons` binary; see "Running" above to start it.

## Character set

See the [Character Set guide](https://andy5995.github.io/immortal-barons/charset/)
for how to choose between CP437 and UTF-8.

## Running as a BBS door

Immortal Barons can run as a native door under BBS software on any platform Go
supports. Callers share one persistent world, each empire saved between calls
and keyed by its BBS handle, and several nodes can play at the same time.

The [Door Setup guide](https://andy5995.github.io/immortal-barons/door-setup/)
covers the details: registering the door, the dropfile, daily maintenance
(`immortal-barons -maint`), the node list, inter-BBS play, and starting a fresh
game (`immortal-barons -reset`).

## Running in a browser (experimental)

Immortal Barons also has an experimental browser front-end (`immortal-barons-web`),
separate from the BBS door. See the
[Web Server Guide](https://andy5995.github.io/immortal-barons/web-server/) for
how to build and run it, how players are tracked, the full list of options, and
why its world is separate from a BBS door's world.

## How to play

See the [player's guide](https://andy5995.github.io/immortal-barons/guide/).

## Design

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

Free software under the [MIT License](LICENSE).

## Related Realms

* [Medusa's Barren Realms Elite (BRE) Discord](https://discord.gg/5wTZpstcjM)
* [Immortal Barons Discord channel on Lazarus Project server](https://discord.gg/weAEgZ2re6)
* [Shurato's Heavenly Sphere BBS](https://shsbbs.net/ad.html)
* [X-Bit BBS](https://x-bit.org/info)
* [LibreGaming](https://libregaming.org/)
