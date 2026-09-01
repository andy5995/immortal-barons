# Immortal Barons

Immortal Barons is a turn-based strategy game for the terminal. You rule
a barony in a ruined, post-apocalyptic world. You buy land, feed your
people, build an army, and fight rival empires. The goal is to have the
highest net worth when the turns run out, or to conquer everyone else.

The game is written in Go. It is an independent remake, inspired by the
1990s BBS door game *Barren Realms Elite*. See [Heritage](#heritage) below.

> **Early release.** The game is playable and inter-BBS (IBBS) league play
> works, but it is not finished: some game values are still being balanced, and
> a league has so far only been run between boards belonging to one operator.

## Status

Immortal Barons is a persistent, multi-user BBS door game: each caller has a
lasting empire in one shared world; you play your turns, and other barons (and
any AI) act on their own turns. The game runs locally or as a native BBS door.

The core game is playable — economy, military and war, covert operations,
diplomacy, and messaging all work. Leagues and inter-BBS play are built as well:
boards trade packets, and barons attack, message, and buy from an ally's market
across planets. That has been run between two boards on one machine; a league
spread across boards run by different sysops has not been tried yet.

Balance is the unfinished part. The values read from the original's own program
— region income, unit costs, industrial output, the crown tax — now match it,
but the rest are still being tuned, so parts of the game may be too easy or too
hard.

Learn more, follow progress, or get involved:

- [Website](https://andy5995.github.io/immortal-barons/)
- [Issue tracker](https://github.com/andy5995/immortal-barons/issues)
- [Discussions](https://github.com/andy5995/immortal-barons/discussions)

We strongly recommend reading the [FAQ](docs/faq.md) first.

## Where to play

You do not need to download or install the game to play it. It runs on the board
or the server you connect to. You can play on a BBS, or over SSH.

### On a BBS

Some boards run Immortal Barons as a door game. Connect with your usual BBS
client. [Boards hosting Immortal
Barons](https://github.com/andy5995/immortal-barons/discussions/132) lists them,
and is where you can list your own board.

### Over SSH

A public server runs the game over SSH:

```
ssh -p 2222 yourname@85.119.83.118
```

Use any name you like. That name is your account on the server. The first time
you play, the game asks you to name your realm, and that is the name other
barons see.

The server takes an SSH key, not a password. Your existing key works. If you do
not have one, make one first:

```
ssh-keygen -t ed25519
```

The first time you connect, the server saves the key you used and links it to
the name you chose. After that, only that key can log in under that name.

If you prefer a graphical client, set the host to `85.119.83.118`, the port to
`2222`, and the user name to the name you want. The client must let you choose a
key file, because the server does not accept passwords.
[PuTTY](https://putty.org/index.html) and
[SyncTERM](https://syncterm.bbsdev.net/) can both do this.

The server runs [DeuceGate](https://deucegate.bbsdev.net/), which starts the game
as soon as you log in.

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

Tagged releases and development snapshots are on the
[releases page](https://github.com/andy5995/immortal-barons/releases). Builds
are provided for Windows, Linux, and macOS.

See [Download](docs/download.md) for which file to pick, and what to expect the
first time you run it.

This repository is also a Homebrew tap, for macOS and Linuxbrew. See
[Download](docs/download.md#install-with-homebrew) for how to install from it.

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
go build ./cmd/immortal-barons ./cmd/barons-ftn
```

This produces the `immortal-barons` game and the optional `barons-ftn` inter-BBS
transport helper. See "Running" above to start the game; the Door Setup guide
explains the helper.

## Character set

See the [Character Set guide](https://andy5995.github.io/immortal-barons/charset/)
for how to choose between CP437 and UTF-8.

## Running as a BBS door

Immortal Barons can run as a native door under BBS software on any platform Go
supports. Callers share one persistent world, each empire saved between calls
and keyed by its BBS handle, and several nodes can play at the same time.

The [Door Setup guide](https://andy5995.github.io/immortal-barons/door-setup/)
covers registering the door, the dropfile, daily maintenance
(`immortal-barons -maint`) and starting a fresh game (`immortal-barons -reset`).
[Inter-BBS Leagues](https://andy5995.github.io/immortal-barons/inter-bbs/)
covers linking your board to others.

## How to play

See the [player's guide](https://andy5995.github.io/immortal-barons/guide/).

## Heritage

Immortal Barons is a tribute to *Barren Realms Elite*, created by Mehul
Patel and now owned by [John Dailey Software](https://www.johndaileysoftware.com/).
This project shares that game's ideas and feel, but it does not use any of its
code, text, or artwork.
Game rules and mechanics are not covered by copyright, so this is a fresh
implementation of the concept. It is not affiliated with, nor endorsed by, the
current or past owner(s) of *Barren Realms Elite*.

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
* [X-Bit BBS](https://x-bit.org/info/)
* [LibreGaming](https://libregaming.org/)
* [r/BBS](https://www.reddit.com/r/bbs/)
