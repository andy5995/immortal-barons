# ib-sysop: the sysop panel

A desktop window for a sysop or League Coordinator to see each board's league
state and run the game's maintenance commands. It is not for playing the game.

```
ib-sysop [DATADIR ...]
```

Each game data directory opens in its own tab; **Open…** adds another, and
accepts either the data directory itself or the door folder that holds it as
`data/`. With no arguments the panel reopens the tabs that were open when it
last closed. That list is kept in the user's config directory
(`immortal-barons/ib-sysop.json`), never in a data directory.

Each tab has four views:

- **Boards**: every other board in the league, as BBSINFO lists it, plus how
  long each has been silent, its average packet round trip and when a probe
  last came back. Status is written out (`ok`, `never heard`, `below min`,
  `other rules`, `held`), never shown by color alone.
- **In flight**: every strike, terrorist or special operation and trade bid
  that has left this board and not been answered, with when the lost-forces
  timer will give it up.
- **Held packets**: the files in the held directory, why each is held, when it
  expires, and whether it is pausing the lost-forces timer.
- **Run**: runs one `immortal-barons` command and shows its output. The
  League Coordinator's commands appear only on node #1, and the two that act on
  every board (`-league-freeze`, `-league-reset`) ask first.

Click a column heading to sort by it; click it again to reverse.

## What it touches

It never writes the world. A refresh takes the world lock, reads, and lets go,
exactly as a door node's read does; the lock is exclusive, so a refresh waits
for a running `-maint` to finish and door nodes wait for a refresh. That is why
auto-refresh is off by default and runs once a minute at most.

Every command is the `immortal-barons` program run as a separate process, from
the data directory's parent folder, with `-data` naming the directory. The
panel looks for that program beside itself first and then on `PATH`; the
**Program** box on the Run view overrides it. **Stop** interrupts the process,
which loses the run in progress and nothing else: the game writes its files by
replacing them whole, and its lock ends with it.

## Building

The panel is its own Go module, so the game's `go.mod` never carries its GUI
toolkit, [Gio](https://gioui.org/). Build it from this directory:

```
cd cmd/ib-sysop
go build
```

Windows needs nothing else, and cross-builds from Linux (`GOOS=windows`, amd64
or 386). macOS needs Xcode's command-line tools. Linux needs cgo and these
headers (Debian and Ubuntu package names):

```
libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev
libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev libxfixes-dev
```
