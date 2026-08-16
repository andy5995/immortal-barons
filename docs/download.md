# Download

## Tagged releases

Stable versions are on the
[releases page](https://github.com/andy5995/immortal-barons/releases).

## Development snapshots

[Snapshots](https://github.com/andy5995/immortal-barons/releases/tag/snapshot)
are built automatically from the latest source. They change often and may be
less stable than a tagged release.

## Which file do I need?

Builds are provided for these systems:

| System | Notes |
| --- | --- |
| Windows | 32-bit and 64-bit |
| Linux | x86-64 and ARM64 |
| macOS | Intel and Apple Silicon |

There is nothing to install: unpack the archive and run `immortal-barons`. It
also contains `barons-ftn`, an optional helper for leagues carried over FTN.

## Notes before you run it

- **Windows may show a warning.** The binaries are not signed, so Windows
  SmartScreen may say the file is not recognised. This is expected.
- **macOS may block the file** the first time, for the same reason. Open it
  from the Finder with a right-click, then choose Open.

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
