# Mod system: decisions and invariants

`docs/mods.md` and the generated `docs/mod-reference.md` are the user
contract: every key, limit, file and command. The code is how each part works. This page holds what neither states as a rule:
the decisions behind the mod system and the invariants a change must keep. The
design came out of discussion #202 and PR #350. Read this before changing how
mods are read, applied, checked or sent.

## Where a mod's values live

- **The base game is the rules in `Rules`, built from the `balance*.go`
  constants.** A mod changes a copy (`game.BaseRules`), never the constants.
  The binary-verified constants stay the fidelity contract for an unmodded
  game, and `-print-base-mod` shows them.
- **Rules belong to the `World`, not to package variables.** Tests of the mod
  loader change rules for one world without touching any other test running in
  parallel, and one world plays one mod set.
- **Every key is named in one table** (`unitFields`, `hqFields`,
  `game.CombatFields`, `game.RegionFields`, `game.AttackFields`). Reading, `-check-mod`, `-print-base-mod` and `Diff` all
  go through it, so a key cannot be readable in one place and missing in
  another.
- **Every figure has limits tighter than its type**, chosen so the engine's
  arithmetic stays safe on the 32-bit door builds. A figure times a region or
  unit count is worked in `int64` (or `big.Int`), and a total saturates rather
  than wraps (`fitInt`). Money stays `int64` all the way to `creditGold`, which
  records what its cap discards. A new key needs a limit chosen this way, and a
  `GOARCH=386` test when it multiplies a count.

## Attack effects

- **One effect set, two sides.** Every kind of attack has a `target` set and an
  `attacker` set of the same effects (`game.EffectSet`), applied target first,
  each in the fixed order of `AttackFields`. That order and the rule that a
  `Roll` draws a random number only when its `up` or `down` is above 0 are part
  of the contract (`capture_random` too: its draw is made only when it is set): they decide every later draw. A mod that sets nothing draws
  nothing, so the base game is draw-for-draw the original (the equivalence test
  keeps the original routines as references).
- **Rounding is not a key.** Whether morale is rounded or truncated is a fixed
  property of the kind (`attackKinds`), because the original's two missiles
  differ and the binary-verified figures depend on it.
- **A kind is a dotted name, so a section's depth varies.** `[attacks.nuclear.target]`
  is kind `nuclear`; a kind with cases later is `conventional.won`. The reader
  walks the nested tables down to `target` or `attacker` (`attackSections`).
- **An option reference stands for a flat percent.** An array holds plain
  numbers. The TOML decoder reports every key under `[attacks]` as undecoded
  because the tables are read as `any`, so `decodeTOML` skips that subtree and
  `applyAttacks` reports anything unknown itself.

## Options

- **The manifest declares the option IDs a mod accepts.** An ID uses the same
  name rule as a mod folder. Only `bool` and `number` exist. A number needs a
  default from `min` to `max`; a bool's default is `true` or `false`. The reader checks
  all of this before it compiles a script.
- **One set of chosen values, carried everywhere.** `Config.ModOptions` holds
  what the next reset will use, by mod name and option ID. A reset records the
  values in each `ModRef.Options`, which travels in the world, the packet and
  the league reset order. A missing ID means the declaration's default. This
  keeps a reader that has only the config, only the world or only the order on
  the same value.
- **Options enter through the same resolver as written figures.** A number
  used by `values.toml` is checked against its own range and then against the
  key's normal limit. A bool selects a `when` section after the plain values
  have been read. A script gets the chosen value through `option()`, and its
  compiled copy is keyed by the values, so two games with different choices do
  not share a script instance.
- **Screens read a mod with the game's choices, not with defaults.**
  `ReadWithOptions` and `ManifestFromRef` let the editors, `-check-mod` and
  Game Setup show the values in force. A screen that only reads never writes
  to the world.
- **Chosen values are part of the ruleset fingerprint.** Two boards with
  different options hold each other's packets, as they do with different mod
  hashes. Adding a new option does not raise `mod_api`; changing what an
  existing ID means does.

## Player preferences

- **A preference is read by scripts only.** Rules are world-wide, so a
  player's choice cannot select a `values.toml` section; `realm.pref(id)` in
  `raid` and `region` is the whole surface. Adding it did not raise `mod_api`:
  it is a new manifest table and a new field of `realm`.
- **Stored per realm, as an explicit choice only.** `Empire.ModPrefs` is keyed
  `<mod>/<id>` and `omitempty`. A missing key means the manifest's default,
  which the mod package resolves, so the game package holds no mod logic and an
  old save loads unchanged. A mod that is no longer in play leaves keys nobody
  reads; a reset clears them with the empires.
- **Never transmitted.** No packet carries an `Empire`, so there is no
  `game.Protocol` change and a remote board never sees a player's choice. The
  declarations ride in the manifest, which the mod's hash already covers.
- **The views carry the choices as one comparable string.** `RegionView` is a
  cache key, so `ModPrefs` is part of it and a toggle cannot return a stale
  answer. A script is given only its own mod's keys. Reading one draws no
  random number.
- **The menu adds rows through the engine** (`Menu.Extra`), after the built-in
  toggles and sharing their column. Mod text goes through the same escape
  filter as every other mod label, and a toggle is the only write.

## When a mod changes

- **Rules change only at a reset.** A game keeps the copy of each mod saved
  when it started, and warns when the folder has changed since.
- **`-reload-mods` is the one exception, and it is for testing.** It puts the
  folder's current values into a game in progress. It is refused on a league
  board.

## Versions

- **`version` is the author's**, for them to change as they like.
- **`mod_api` is the mod format's**, and every mod must give it. It covers the
  folder layout, the `values.toml` keys and what each one means, and what a
  script is given and returns. Raise `mod.ModAPI` for any change that could make
  a mod that worked before fail to load, or load and play differently, and say
  in the ChangeLog what changed. Adding a key or a script field does not raise
  it. The game refuses an older or newer mod by name before play.

## Scripts

- **Starlark, run in a worker process the OS can limit.** The threat model is
  a sysop who installs a mod, or accepts a league's, without reading its
  scripts. Starlark has no file, network, exec or clock access and the only
  names IB adds are `rand`, `day`, `option` and a realm's `pref`, so the harm
  left is to availability: CPU, memory, a hang. Starlark has no allocation
  accounting, and an in-process limit cannot stop a built-in stuck in Go. An
  earlier version rewrote the script's parse tree to size-check operators and
  abandoned a stuck goroutine; it made scripts behave unlike plain Starlark,
  could not stop everything, and switched all scripts off for the process after
  one stuck run. It was removed for this.
  - **The worker** is the game's own executable, started with
    `IB_SCRIPT_WORKER=1`. `init()` in `internal/mod` (`worker.go`) takes over
    before `main`, so every binary and test binary that imports the package is
    its own worker with nothing to wire; the parent finds itself with
    `os.Executable()`. The game talks to it in JSON lines over pipes made by
    `exec.Cmd`. The worker's stdin and stdout are those pipes, never the
    game's (on a door they are the caller), its stderr goes to a 2 KB buffer
    that names the cause when it dies, its environment is two variables, and it
    is given no data directory. It `os.Stdout = os.Stderr` first, so a stray
    write cannot corrupt the conversation. On Unix the game marks every descriptor
    from 3 up close-on-exec before it starts one (`protectInheritedFDs`): Go
    sets the flag on what it opens, but one the BBS passed down may lack it.
    Windows needs nothing, since `exec.Cmd` passes only the standard handles.
  - **Protocol** (`wireMsg`): `compile` (script id, kind, file name, source,
    option values) and `call` (id, the view, the mod's preference defaults) from
    the game; `result` from the worker. The worker holds compiled globals, the
    game never holds a Starlark value, and keeps only sources, so a new worker
    is sent `compile` again before its first call of a script (`worker.ensure`).
    `rand(n)` inside `raid()` sends a `rand` request mid-call and the game answers
    from `PirateView.Rand`, so the world's generator is drawn in the order the
    script asks, as before (`TestRandRoundTripsInOrder`,
    `TestScriptGameReplaysForASeed`). `day()` and a realm's preferences travel
    in the view. The orders and the region row are read into Go values in the
    worker and sent as JSON.
  - **Limits.** *Time* is the game's: `ScriptTimeout` (3 s) from sending a
    request, then `Process.Kill`. The call fails, the worker is dropped, and
    the next call starts a new one and recompiles; there is no global switch.
    *Memory* (`ScriptMemory`, 256 MiB): on Linux and the BSDs the worker sets
    its own `RLIMIT_DATA` to that plus 128 MiB before it reads anything (on
    Linux it counts private writable mappings, which is Go's heap; measured:
    the runtime runs with 8 MiB over a 64 MiB limit, and a single 1 GB string
    dies of `cannot allocate memory`); on Windows the game puts it in a Job
    Object with a process memory limit and kill-on-close; on macOS the system
    enforces neither, so the worker's own watchdog (`workerWatchdog`, heap
    objects sampled every 5 ms, a collection before it decides, `os.Exit(3)`)
    is the limit. The watchdog runs everywhere (a test turns the system limit off
    to prove it). `RLIMIT_AS` was not used: the race detector and the Go runtime
    reserve address space that it counts. Under the race detector (tests only)
    the system limit is skipped for the same reason (`race_on.go`). *CPU*:
    before each request the worker sets `RLIMIT_CPU` soft to what it has used
    plus 10 s, and exits on the `SIGXCPU` that follows (the Go runtime ignores
    it unless asked), so a worker left running by a dead game stops; the parent's pipe
    closing ends an idle one. *Steps*: `ScriptSteps` stays in the worker.
  - **Pool.** Workers are made on demand, as many as `GOMAXPROCS` up to four
    (`workerPoolMax`), one call in flight on each; a goroutine that finds all
    busy waits. A single worker behind a mutex would have made one slow script
    block every other and a kill fail the calls queued behind it; separate
    processes make a kill touch one call. Each can hold `ScriptMemory`, so four
    is the cap on that, too.
  - **No worker is not in-process.** If `os.Executable()` or the start fails,
    `validate` warns once and the mod loads; every call of its scripts fails
    with the reason and the game plays its built-in rules.
    `TestNoWorkerMeansAClearFailureNotAnInProcessRun`.
  - **Pinned surface and language options.** `scriptFileOptions` is
    `syntax.FileOptions{}`: no `while`, recursion, `set`, global reassignment
    or top-level `if`/`for`, and `load()` is refused. `print` is discarded.
    `TestScriptSurfaceIsPinned` fails when the predeclared names or Starlark's
    own universe gain one.
  - **Mod files**: at most `MaxModFileSize` (256 KiB) each and `MaxModFiles`
    (64), on the folder reader, a saved copy and a league order alike
    (`checkFileSet`). The folder reader follows a link only to a regular file
    inside the mod folder, opens it with `O_NOFOLLOW`, checks the file it
    opened (`Fstat`: regular, size, link count) and reads through a limit;
    a file with more than one hard link is refused, since a hard link to,
    say, `/etc/passwd` would otherwise ride a league's reset order to every
    board (`files_unix.go`, and `files_windows.go`, which reads the count from
    the open handle; Windows has no `O_NOFOLLOW`, so there the folder walk's
    link check is the only one).
  - **Text**: everything a script or mod chose that reaches stderr or a log goes
    through `clean`, which writes control characters and Unicode format
    characters (bidi overrides among them) as `\uXXXX`. An error names a script's
    value by its type, or by a string cut to 40 characters and cleaned (`show`).
    The worker's reply is cleaned and cut again by the game, and so is its
    stderr. Manifest and option text is refused if it holds one, and
    `ManifestFromRef` holds a saved copy to the same checks.
  - **Cost.** About 28 us a call over the pipe, 32 us with one `rand` round
    trip (`BenchmarkWorkerRegionCall`, `BenchmarkWorkerRaidWithDraws`);
    100 scripted turns of a one-realm world took 9.5 ms against 3.3 ms in
    process. Region answers are cached, so a screen pays once per view.
  - **Residual risk.** (1) A script that fails every call costs up to the timeout
    each time and a worker start; there is no circuit breaker. (2) Memory is per
    worker; four can each use their limit. (3) macOS has no system limit, and the
    watchdog samples, so one large allocation can pass it. (4) The BSD and Windows
    limits are written to the platform's documentation and have not been run.
    (5) The worker has the game's user rights; it contains a script, it is not
    a boundary against a flaw in Go or the interpreter. (6) The code that
    limits scripts was written by an LLM and has had no review by a human
    security expert; the user docs say so.
- **A region script must answer the same question the same way.** It gets no
  `rand()` and no `day()` builtin, and `realm` carries no gold, because a screen
  asks for a projection and the turn asks again when it pays. What the view does
  carry (`turns_played`, `people`, `support`) changes between turns, so a
  projection holds for the realm as it stands, not for next turn.
- **A script's globals are frozen once its top level has run.** It keeps
  nothing between calls.
- **A failing script falls back to the type's row (regions) or the built-in raid
  (pirates)**, and each different failure is reported once per run, up to
  `ScriptWarnings`.

## Leagues

A stand-alone board's sysop chooses its mods. In a league the Coordinator
chooses them, and members install nothing.

- **The Coordinator's `-league-reset` order carries every file of every mod**,
  and the Coordinator's signature covers it. A member rebuilds its rules from
  those files with the same reader and limits a stand-alone reset uses
  (`mod.LoadForLeague`, installed as `game.LoadLeagueMods`).
- **A member that cannot play the order's mods does not reset**, and tells its
  sysop why. The refused order is kept on the world (`PendingLeagueReset`) and
  retried at the start of every planetary run: once the sysop makes the mods
  playable, that order resets the board with its original date and mods. Its
  old game is held apart by the ruleset check until then. A reset under other
  rules would feed a different game into everyone else's. The order is kept
  through any reset of the board's own, and discarded once the board is in
  another league (a different league number or Coordinator key).
- **Scripts run on a member only when its sysop allows it** (`AcceptLeagueScripts`
  in `bbs.cfg`, off by default). The check must come before anything compiles a
  script, since compiling runs its top level.
- **The ruleset fingerprint includes each mod's name and hash**, and every
  packet carries it. With no mods it is the settings' fingerprint unchanged. A
  member stamps the hash of the copy it actually plays, so a copy edited in
  `world.json` is held like any other difference in rules. This catches a
  misconfigured board, not a lying one: a modified binary can state any
  fingerprint.
- **Mod files are not scrubbed** in an arriving packet, because their tabs and
  newlines are part of what the hash covers. Every other field of a mod
  reference is scrubbed like the rest of the packet, and the mod reader refuses
  a control character in each manifest field it shows.
- **Arriving missiles, the S3-Sabre and interplanetary attacks are moddable;
  the rest of interplanetary play is not.** An arriving missile resolves on
  the target's board under that board's rules, and every board in a league
  plays the same mods (the ruleset fingerprint holds a packet from a board
  whose rules differ), so the kinds `ip.nuclear`, `ip.chemical` and `sabre.*`
  read the attack table. Only their target set applies, since the attacker's
  realm is on another board, and `Rules.Validate` refuses an attacker set or a
  `capture` on them; no packet carries a second realm's change. Still
  constant, with no key yet: the remote tank curve, the share of a
  HeadQuarters a winning invasion takes, and a Sabotage HQ agent's points.
  Bombing runs and terror ops are out of scope: they hit planets and markets,
  not a realm's effect set.
- **One pair of kinds, `ip.attack.won` and `ip.attack.lost`, serves every
  interplanetary attack** (Quick Strike, Normal Attack, Extended Battle and
  Group Attack). The resolver's battle, capture, losses and HQ share stay
  built in (`resolveRemoteAttack` is not moved into the table), except the
  capture share's level ladder, which `[combat]` sets; the sets add
  to them, and only when a battle was fought, never for NOT FOUND or
  PROTECTED. The Target set runs on the defending board in
  `resolveRemoteAttack`, after the built-in outcome, on each realm that stood.
  Its `capture` is added to `LandTaken`, so it travels home and the existing
  spoils code splits it among the contributors; nobody on the defending board
  receives it. The Attacker set runs on the attacker's own board in
  `applyAttackResult`, picked by `res.Won`, on each contributor's realm there,
  in full, as if it had attacked alone; its `capture` is refused by `Validate`
  because the target is on another board. The one exception is its unit
  capture (below), which runs on the defending board. Both halves read the local board's
  rules, which is safe because a league plays one mod set. What the target
  set took travels home as figures, `AttackResult.Extra` (an omitempty
  field, so a packet without it is byte-identical; it rides Protocol 8),
  and not as words, because the home board splits it among the contributors
  by the same share as their kills. Words could not be split.
- **Unit capture is the one attacker-set effect that runs on the defending
  board (#373).** `capture_<unit>` (a percent per military unit),
  `capture_random` and `capture_from_stock` are allowed on the four battle
  kinds only (`conventional.won/lost`, `ip.attack.won/lost`); `Validate`
  refuses them on every missile kind. A capture runs after the kills of the
  same set, so a unit a kill destroyed is not captured, and its base is the
  units still alive: the attacker's base is the survivors of the force it sent
  (never what it held back); a defender's is the units that stood in the
  defense (`defenseGoods`: troopers, jets, turrets, tanks), or with
  `capture_from_stock` in a target set its whole remaining stock; an ally's is
  the survivors of the 30% detachment it sent, never its stock, and
  `capture_from_stock` does not widen it. A percent is drawn once per unit and
  applied to every holding. `capture_random` takes its share of all the
  holdings' units together, spread across them by the normal approximation of
  the hypergeometric draw (one draw per holding, so cost does not follow the
  unit count); it is drawn only when set, so the base game makes no new draw.
  Interplanetary: the target set's captured units are `EffectLoss.Taken`
  (inside `AttackResult.Extra`) and are split among the contributors by
  `splitSpoils`, exactly, like the land. The attacker set cannot run at home,
  where the defenders' realms are not, so `remoteSeize` runs it on the
  defending board while the result is built: it takes from `res.Survivors`,
  gives the units to the defenders that stood (evenly, remainder to the
  first), and lists what it took in `AttackResult.Seized` so each contributor
  is told; `homeAttackEffects` leaves the unit keys out. The home board does
  not believe the packet for units: `capturedHome` credits only a unit its own
  rules let that kind capture, never below 0 and never past `MaxUnits`, and
  `seizedOf` holds the report's figure to what the contributor sent. Both
  fields are omitempty and ride Protocol 8. A `capture_turret` or
  `capture_carrier` in an attacker set is refused, since no attack sends them.
- **The combat levels are one table per figure, and an interplanetary capture
  resolves on the defending board (#359).** `[combat]` holds a percent for
  each Attack Rewards level (same-planet and interplanetary) and each Attack
  Damage level, the same-planet capture floor and the score per region
  (`game.CombatRules`). The sysop's level picks the figure, so a mod
  redefines a level and never reads which one is chosen; there is no offset
  key, because a per-level table already expresses one and can also reorder
  the levels. `resolveRemoteAttack` runs on the defending board and reads
  that board's rules and its own `Config.AttackRewards`, the same as the
  arriving missiles do. That is the right place because the ruleset
  fingerprint covers each mod's name, hash and chosen options, so a packet
  from a board playing other figures is held, and a league plays one mod set.
  Nothing new travels: the share is applied there and its result comes home
  as the existing `LandTaken`, so there is no protocol change. Still
  constant: an interplanetary battle's loss (the attack type's own rate
  rescaled by the Attack Damage level, which has no per-level table of its
  own) and the interplanetary floor of 10.
- **The gates, the dial and the caps stay built in.** The misfire, SDI and
  garrison rolls, the Sabre's aim, jitter and backfire, and the score draw are
  not effects. The per-kind caps (`IPMissileRegionCap`, `IPMissilePeopleCap`)
  and how a row rounds (the Sabre truncates what is kept, and three of its
  rows keep the original's Real48 arithmetic while their figure is the base
  one) are fixed properties of the kind in `attackKinds`, because the
  binary-verified figures depend on them.

## Not moddable yet

- **The AI.** It is lowest priority (Andy, 2026-11-24).
- **New units, and the number of pirate factions.** Both are fixed in the save
  format and the packets.
- **Further pirate figures** are #351.
