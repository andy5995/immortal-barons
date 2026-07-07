# Frequently Asked Questions

## Is winsock supported?

- Yes, on Windows. The game attaches to the caller's socket handle that the BBS
  passes on line 2 of `DOOR32.SYS`.
- The Windows socket path is built but has not been tested with a live BBS yet.
- See [How the game talks to the caller](https://andy5995.github.io/immortal-barons/door-setup/#how-the-game-talks-to-the-caller)
  in the Door Setup guide for the socket and stdio details.

## How is Immortal Barons different from Barren Realms Elite?

- It is a new game, written from scratch in Go, inspired by BRE. It shares no
  code, text, or art with the original.
- It does not run under DOS. It is a modern native program that runs as a door
  under BBS software on any platform Go supports.
- It has a [web browser version](https://andy5995.github.io/immortal-barons/web-server/),
  which BRE does not. It also plays in a local terminal.
- Some game values — such as attack rewards, unit prices, and economy rates —
  may not match BRE exactly. The game aims to get as close as it can, using
  BRE's own numbers where they are known. A few mechanics are still being built.

## Can several people play as a door at the same time?

- Not yet. As a BBS door, one caller plays at a time right now; a second caller
  is asked to try again shortly.
- Concurrent multi-node play (several callers in one game at once) is planned.
- The web version already serves several players at once, but it runs as its
  own separate game, not as a BBS door.
