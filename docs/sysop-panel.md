# Sysop Panel

`ib-sysop` is a desktop window for a sysop or League Coordinator. It shows a
board's league state and runs the game's maintenance commands. It is not for
playing the game.

It builds for Windows, Linux and macOS. It needs a desktop, so it does not run
on a server you reach only over SSH.

## Getting it

Each release from v0.2.1 on has an `ib-sysop` archive for every platform,
beside the game's own archives. Unpack it in the same place as the game
archive: both unpack into the same folder, so `ib-sysop` ends up next to
`immortal-barons`. The Homebrew formula installs both programs.

On macOS, a downloaded binary is blocked until you remove the quarantine flag:

```
xattr -d com.apple.quarantine ib-sysop
```

To build it from the source instead, run this in the top folder of the
repository:

```
go build -C cmd/ib-sysop -buildmode=pie -o ../..
```

This puts `ib-sysop` next to `immortal-barons`, which the panel runs for every
command. Keep the two together.

- **Windows:** nothing else is needed. You can also build it on Linux with
  `GOOS=windows` (amd64 or 386).
- **macOS:** needs Xcode's command-line tools.
- **Linux:** needs cgo and these headers (Debian and Ubuntu package names):

  ```
  libwayland-dev libx11-dev libx11-xcb-dev libxkbcommon-x11-dev libgles2-mesa-dev
  libegl1-mesa-dev libffi-dev libxcursor-dev libvulkan-dev libxfixes-dev
  ```

## Starting it

```
ib-sysop [DATADIR ...]
```

Each game data directory opens in its own tab. **Open…** adds another tab. It
takes the data directory itself, or the door folder that holds it as `data/`.

With no arguments, the panel opens the tabs that were open when it last closed,
at the same size. It keeps this in your user config directory
(`immortal-barons/ib-sysop.json`), never in a data directory.

A saved folder that does not open, such as a drive that is not mounted yet,
stays on the list for the next start. **Forget** removes it.

**A−** and **A+** at the top right make the text and spacing smaller or larger.
Ctrl+minus, Ctrl+plus and Ctrl+0 do the same (Cmd on macOS). Ctrl+0 goes back to
the default size, 125%.

**Light**, **Dark** and **System** at the top right choose the colors. System
follows your desktop's light or dark setting, and checks it again every minute.
When the desktop does not say, System is light. The panel remembers your
choice.

## The views

Each tab has five views. Click a column heading to sort by it. Click it again
to reverse the order.

- **Boards:** every other board in the league, as the `-bbsinfo` report lists
  it. It also shows how long each board has been silent, its average packet
  round trip, and when a probe last came back. The status is a word (`ok`,
  `never heard`, `below min`, `other rules`, `held`), not only a color.
- **In flight:** every strike, terrorist operation, special operation and trade
  bid that left this board and has no answer yet. Each row shows when the
  lost-forces timer will give it up. An item stays here for the whole round
  trip, which is usually hours.
- **Held packets:** the files in the held directory, why each one is held, when
  it expires, and whether it pauses the lost-forces timer.
- **Run:** runs one `immortal-barons` command and shows its output as it runs.
  **Detailed** adds `-detailed`, and is grayed out for commands that ignore it.
  The League Coordinator's commands appear only on node #1. The two that act on
  every board, `-league-freeze` and `-league-reset`, ask before they run.
- **bbs.cfg:** the board's `bbs.cfg`. **Edit in default editor** opens it in
  your desktop's default editor. When the panel cannot find one, it says so and
  names the file. A board with no `bbs.cfg` gets one written from its current
  settings before the editor opens. The view reads the file again when it
  changes.

## What it changes

It never writes the world. A refresh takes the world lock, reads, and lets go,
the same as a door node that only reads. It holds the lock for a few
milliseconds, so callers do not notice it. A refresh started during `-maint`
waits for `-maint` to finish. Auto-refresh is off by default and runs once a
minute.

The only file it may write in a data directory is `bbs.cfg`, and only when the
board has none and you click **Edit**.

Every command is the `immortal-barons` program, run as a separate process with
`-data` set to that tab's directory. The panel looks for the program in its own
folder first, then on `PATH`. The **Program** box on the Run view changes that.

Use a game from the same release as the panel. An older `immortal-barons`
ignores **Detailed** with `-maint`, and says nothing about it.

**Stop** ends the command. You lose that run and nothing else: the game writes
each file in one step, and its lock ends when it does.
