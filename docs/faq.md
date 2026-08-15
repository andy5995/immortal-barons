# Frequently Asked Questions

## How is Immortal Barons different from Barren Realms Elite?

- All features are enabled and free to all sysops and players. There is no
  "registered" or "unregistered" version.
- Several BBS nodes can play the same game at the same time, which BRE could not
  do. See [Can several people play as a door at the same time?](#can-several-people-play-as-a-door-at-the-same-time).
- It also plays in a local terminal, without a BBS, so you can try it on your
  own computer.
- It does not run under DOS. It is a modern native program that runs as a door
  under BBS software on any platform Go supports.
- A new player can end their own new-realm protection early, from the System
  menu, to start fighting sooner. BRE does not offer this.
- Immortal Barons hides realms under new-realm protection from the attack list.
- When a sysop starts a fresh game, they can keep their old settings. Those
  settings are stored in a `config.json` file. Running
  `immortal-barons -reset-from-config` clears the world but reuses that file, so
  you do not have to choose the settings again each time you reset.
- River regions pay gold **and** food every turn. In BRE, a river does only one
  of the two each turn, and you do not know which one until the turn's report
  appears. Over many turns a river earns the same total either way, so this
  removes the surprise, not the value. The reason is planning: a player with many
  rivers could see millions of gold appear one turn and vanish the next, and a
  food supply that only arrives about one turn in three is hard to count on when
  people are hungry.
- View Treaties shows the treaty offers you have sent and nobody has answered
  yet. In BRE you cannot see them at all: a realm you asked for a pact looks the
  same as a realm you never wrote to, so it is easy to send the same request
  twice, or to wait for a reply to a request you never sent. This tells you only
  about your own offers, so it does not show you anything about other players.
- Send Message can write to all your allies at once — every realm on your board
  that you hold a treaty with, of any kind.
- Your sysop sets how much gold you can hold. BRE stops at about two billion;
  here that is only the starting value, and it can go as high as 999 billion.
- Gold above the limit is still lost, but the game now tells you when it happens,
  and a full bank pays its interest into your gold in hand rather than losing it.
- A reset option turns the pirates off. BRE always has them.
- Some game values — such as attack rewards, unit prices, and economy rates —
  may not match BRE exactly. The game aims to get as close as it can, using
  BRE's own numbers where they are known. A few mechanics are still being built.

## Is winsock supported?

- Yes, on Windows. The game attaches to the caller's socket handle that the BBS
  passes on line 2 of `DOOR32.SYS`.
- A sysop has run the Windows socket path with a live caller and reports that it
  works. More reports are still welcome.
- See [How the game talks to the caller](https://andy5995.github.io/immortal-barons/door-setup/#how-the-game-talks-to-the-caller)
  in the Door Setup guide for the socket and stdio details.

## The characters look wrong. What can I do?

Try `-ascii`, which sends plain 7-bit ASCII that no terminal can mis-decode.
See the [Character Set guide](https://andy5995.github.io/immortal-barons/charset/)
for what each setting does.

## The screen is full of things like `[94m`. What can I do?

That is a terminal printing the game's colour codes instead of acting on them.
The door sends plain text on its own when your BBS reports that a caller has no
ANSI support, so first check that the caller has graphics turned on in their BBS
profile. To see the plain version yourself, run with `-no-ansi`. Lists that
normally use a moving highlight are numbered in that mode, so they stay usable.

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
- A board in a league game has no AI barons, and none can be added. League play
  is between the boards' own players.

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
- If your empire is attacked while you are at the menus, a notice appears the
  next time you press a key. Orders you already typed are adjusted to what you
  still hold, and the game tells you when that happens.
