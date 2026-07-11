# Frequently Asked Questions

## Is winsock supported?

- Yes, on Windows. The game attaches to the caller's socket handle that the BBS
  passes on line 2 of `DOOR32.SYS`.
- The Windows socket path is built but has not been tested with a live BBS yet.
- See [How the game talks to the caller](https://andy5995.github.io/immortal-barons/door-setup/#how-the-game-talks-to-the-caller)
  in the Door Setup guide for the socket and stdio details.

## The characters look wrong. What can I do?

See the [Character Set guide](https://andy5995.github.io/immortal-barons/charset/).

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

## How do I add AI barons (computer opponents)?

- Set the number of AI barons before you start a game. A fresh game has none by
  default.
- Open the Configuration Editor (run `immortal-barons -reset`, or the
  Coordinator menu in game) and set the **AI empires** field.
- The AI barons are created when you reset the game. Changing the number later
  does not add or remove them from a game already in progress — reset to apply a
  new count.
- Once in the game, the AI barons take their turns on their own during daily
  maintenance. See [Daily maintenance](https://andy5995.github.io/immortal-barons/door-setup/#daily-maintenance)
  for when that runs.

## Can several people play as a door at the same time?

- Yes. Several BBS nodes can play the same game at once. Each node's changes —
  new scores, mail, attacks — become visible to the others between actions.
- Each action reads and writes the shared game file under a brief lock, so
  nodes wait for each other only for the instant of a change, not for a whole
  session.
- The web version also serves several players at once (as its own separate
  game, not as a BBS door).
