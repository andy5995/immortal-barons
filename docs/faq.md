# Frequently Asked Questions

## Is winsock supported?

- Yes, on Windows. The game attaches to the caller's socket handle that the BBS
  passes on line 2 of `DOOR32.SYS`.
- On Linux, macOS, and the BSDs the game uses standard input and output instead
  (the BBS handles the network), so winsock is not used there.
- The Windows socket path is built but has not been tested with a live BBS yet.

## How is Immortal Barons different from Barren Realms Elite?

- It is a new game, written from scratch in Go. It copies BRE's rules and
  mechanics, but shares no code, text, or art with the original.
- It does not run under DOS. BRE was a DOS program; Immortal Barons is a modern
  native program for Linux, macOS, Windows, and the BSDs.
- It runs as a native door on modern BBS software (Synchronet, Mystic), with no
  DOSBox or DOSEMU.
- It has a web browser version, which BRE does not. It also plays in a local
  terminal.
- Some game values — such as attack rewards, unit prices, and economy rates —
  may not match BRE exactly. The game aims to get as close as it can, using
  BRE's own numbers where they are known. A few mechanics are still being built.
- It is an independent tribute. It is not affiliated with, or endorsed by, the
  authors of Barren Realms Elite.

## Can several people play as a door at the same time?

- Not yet. As a BBS door, one caller plays at a time right now; a second caller
  is asked to try again shortly.
- Concurrent multi-node play (several callers in one game at once) is planned.
- The web version already serves several players at once, but it runs as its
  own separate game, not as a BBS door.
