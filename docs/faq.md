# Frequently Asked Questions

## Is winsock supported?

- Yes, on Windows. The game attaches to the caller's socket handle that the BBS
  passes on line 2 of `DOOR32.SYS`.
- A sysop has run the Windows socket path with a live caller and reports that it
  works. More reports are still welcome.
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
- Several BBS nodes can play the same game at the same time, which BRE could not
  do. See [Can several people play as a door at the same time?](#can-several-people-play-as-a-door-at-the-same-time).
- A new player can end their own new-realm protection early, from the System
  menu, to start fighting sooner. BRE does not offer this.
- Immortal Barons hides realms under new-realm protection from the attack list.
- Some game values — such as attack rewards, unit prices, and economy rates —
  may not match BRE exactly. The game aims to get as close as it can, using
  BRE's own numbers where they are known. A few mechanics are still being built.
- When a sysop starts a fresh game, they can keep their old settings. Those
  settings are stored in a `config.json` file. Running
  `immortal-barons -reset-from-config` clears the world but reuses that file, so
  you do not have to choose the settings again each time you reset.

## How do I add AI barons (computer opponents)?

- Set the number of AI barons before you start a game. A fresh game has none by
  default.
- Open the Configuration Editor (run `immortal-barons -reset`, or the
  Coordinator menu in game) and set the **AI empires** field.
- The AI barons are created when you reset the game. To add more to a game that
  is already running — **without** resetting — run
  `immortal-barons -add-ai N` (it adds N new AI barons and exits). Lowering the
  count still needs a reset.
- Once in the game, the AI barons take their turns on their own during daily
  maintenance. See [Daily maintenance](https://andy5995.github.io/immortal-barons/door-setup/#daily-maintenance)
  for when that runs.

## What happens when my empire is destroyed?

- If your realm is wiped out — its people or land reach zero, from an attack or a
  weapon — your empire is gone, and you cannot keep playing it.
- You build a **new realm the next day**: log in again on a later day and you
  are prompted to name a new realm (it need not match your old one), starting
  fresh under the same handle.

## Can several people play as a door at the same time?

- Yes. Several BBS nodes can play the same game at once. Each node's changes —
  new scores, mail, attacks — become visible to the others between actions.
- Each action reads and writes the shared game file under a brief lock, so
  nodes wait for each other only for the instant of a change, not for a whole
  session.
- The web version also serves several players at once (as its own separate
  game, not as a BBS door).
