# Download

## Tagged releases

Stable versions are on the
[releases page](https://github.com/andy5995/immortal-barons/releases).

## Development snapshots

[Snapshots](https://github.com/andy5995/immortal-barons/releases/tag/snapshot)
are built automatically from the latest source. They change often and may be
less stable than a tagged release.

**Do not run a snapshot on a board in an inter-BBS league.** A snapshot is
rebuilt on every push and the previous one is not kept, so two boards that
download hours apart end up on different builds — and the earlier build cannot
be fetched again, so they cannot be brought back into step even deliberately.
When a change lands that alters what boards send each other, boards on either
side of it stop understanding one another. A league on snapshots hit exactly
that in August 2026: every order the Coordinator sent was refused for a day,
and nothing was misconfigured. Use a tagged release, and have the whole league
move between releases together.

## Which file do I need?

Builds are provided for these systems:

| System | Notes |
| --- | --- |
| Windows | 32-bit and 64-bit. The 32-bit build also runs on Windows 7 |
| Linux | x86-64 and ARM64 |
| macOS | Intel and Apple Silicon |

There is nothing to install: unpack the archive and run `immortal-barons`. It
also contains `barons-ftn`, an optional helper for leagues carried over FTN.

## Notes before you run it

- **Windows may show a warning.** The binaries are not signed, so Windows
  SmartScreen may say the file is not recognised. This is expected.
- **macOS may block the file** the first time, for the same reason. Open it
  from the Finder with a right-click, then choose Open.
- **On Windows 7, use the 32-bit build.** The 64-bit one needs Windows 10 or
  later, because that is where the Go compiler's own support now starts. The
  32-bit build is made with a Go fork that keeps Windows 7, 8, 8.1 and Server
  2008 R2 through 2012 R2 working.

## Install with Homebrew

This repository is also a [Homebrew](https://brew.sh) tap, for macOS and
Linuxbrew:

```
brew tap andy5995/immortal-barons https://github.com/andy5995/immortal-barons
brew trust andy5995/immortal-barons
brew install immortal-barons
```

Homebrew does not load formulas from a third-party tap until you mark the tap
as trusted. That is what the `brew trust` line does.

Homebrew builds the game from source, so the first install takes a little
longer. macOS does not block a game installed this way.

## Build it yourself

You can build the game from source with Go 1.26 or newer. The
[README](../README.md) explains how.
