# Web Server Guide — Running Immortal Barons in a Browser

This is experimental. The web front-end (`barons-web`) lets someone play the
game in a normal web browser, using [xterm.js](https://xtermjs.org/) to draw
the terminal screen. It is a separate program from `barons-door`; it does not
run as a BBS door.

## For players

When you open the game page, the server gives your browser a small hidden
cookie (a random ID). That cookie is how the server tells you apart from
other players — there is no login and no password. Each browser (or private/
incognito window) gets its own game session and its own empire, tied to that
cookie.

Points to know before you play:

- Just open the address the sysop gives you, for example
  `http://localhost:8080`.
- Each browser session is its own player. Opening the same page in a second
  tab of the *same* browser reuses your cookie and your existing game;
  opening it in a different browser (or private window) starts a new player.
- This front-end is early and loads xterm.js from a public CDN
  (jsdelivr.net). Do not treat it as a locked-down production deployment yet.
- If the game says "The realm is full," the server has reached its limit on
  players connected at the same time. Try again later.

## For sysops

### Build

```
go build -o barons-web ./cmd/barons-web
```

### Run

```
./barons-web -addr :8080 -data /path/to/data
```

- `-addr` — the address to listen on (default `:8080`).
- `-data` — the game data directory (default `./data`), same meaning as for
  `barons-door`: it holds `config.json` and the world save file.

There is no `-setup` flag for `barons-web`. If `-data` has no `config.json`
yet, it starts from the built-in defaults; use `barons-door -setup` first if
you want to choose the rules ahead of time.

### The web world is stand-alone

`barons-web` loads its own world from the data directory you point it at,
and holds an exclusive lock on it while it runs — the same lock
`barons-door` uses. That means:

- A `barons-web` world and a `barons-door` world are only the same world if
  you point both programs at the *same* data directory, and you must not run
  both against that directory at the same time (the second one to start
  will fail to get the lock).
- In the normal setup — a separate data directory for the web server — the
  browser players and the door's BBS callers are two different populations
  of empires. Nothing a web player does affects the door's world, and vice
  versa.

Run `barons-web -help` to see all the command-line options.
