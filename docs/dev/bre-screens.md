# BRE screen reference — literal output, layout, and colors

This is a **ground-truth capture** of Barren Realms Elite's actual on-screen
output: exact wording, borders/decorations, numeric prompts, and the ANSI
**colors** of each element. It exists so Immortal Barons can match BRE's
presentation without re-driving the DOS binary every time. It is a reference for
*our* implementation — no BRE code or art is copied into the game; this file
records observed behavior for fidelity work (see the clean-room note at the end).

Captured live 2026-07 by driving BRE v0.988 under dosemu2 and scraping the pane
with `tmux capture-pane -ep` (the `-e` flag keeps ANSI escapes — this is the
proven way to read BRE's colors headlessly; see the `bre-gather` skill).

## Audit status — and the UNVERIFIED marker

**Every claim in this file is either backed by a capture or marked
`**UNVERIFIED**`.** That literal string is the one marker; `grep -n UNVERIFIED
docs/dev/bre-screens.md` lists every gap in the file. A marked claim is a
recorded guess or a reading taken from somewhere other than a live screen — do
not build a colour or layout decision on one without re-capturing first. An
unmarked claim was read off a capture, and the entry says which.

**Captures are not in git** (`cap/` is excluded and `*.cap` is gitignored as
verbatim proprietary output), so a future audit needs them re-taken, and nothing
verbatim beyond what is already recorded here should be pasted in.

**One exception to the rule above, and it is a defect in this file rather than a
kind of claim.** The entries stamped **re-captured 2026-08-16** were read off a
live session whose raw log was NOT kept: that pass drove BRE in a throwaway
directory and deleted it when it finished, taking the `script` output with it.
Copying the game aside to protect the sysop's save was right; putting the
capture there too was not. Nothing here contradicts those readings and they are
probably correct — but they cannot be re-checked against anything on disk, which
is the standard every other unmarked claim meets. Treat them as sound but
unauditable until someone re-drives those screens and keeps the log in `cap/`,
per the rule now in the `bre-gather` skill.

Nothing is parked on this any more. The last item that was — the **incoming
treaty-offer prompt's position relative to the numbered recap entries** — was
settled from the DISASSEMBLY instead, because no capture can settle it: a survey
of all 249 recaps across six captures found no screen showing an offer and
numbered entries together. `run_player_turn` (`BRE.EXE 0x36E1`) prints the
header, then calls `process_trade_offer` (`0x3855`), `process_diplomatic_proposal`
(`0x385A`), `write_data_report` (`0x385F`) for the entries, and
`read_local_messages` (`0x3869`). So both offers precede the entries, and the
trade barter precedes the treaty. IB had both the other way round.

The same routine says **which of those stops a player out of turns still gets**:
the two offer calls sit behind a turns-remaining test (`0x3842`), while the
entries and the mailbox are called on either path (`0x385F`), and "Sorry, you
have used all of your turns today." is only reached afterwards (`0x38D7` ->
`0x3F8D`). So entering with no turns left shows the recap and the mail and asks
nothing about pending offers. Neither stop has a per-day gate: a pending barter
is put again on every entry that has a turn to play, which is what #175 asks.

The **Covert Operations box width** was parked here too and is now settled: it
was re-captured on 2026-08-17 with the log kept as `cap/covert-menu-20260817.cap`
(32 columns; see that section).

### 2026-08-16, pass one — the file against the captures on disk

Every claim was checked against four colour captures plus one large monochrome
public-board session. Roughly seventeen screens were misdescribed and corrected;
each correction below says so and names the capture. That pass also found about
a third of the file resting on no capture at all, which is what pass two went
after.

### 2026-08-16, pass two — driving BRE for the screens with no evidence

A fresh session under dosemu2 (`script`-wrapped, so SGR colour survives) against
a three-realm local game, then a `BRE RESET` for the editor. These went from
recorded-but-unverified to capture-backed:

| Screen | Outcome |
|---|---|
| Status bar | CORRECTED — three fields, and their pre-login values |
| Regular Attack force entry | confirmed |
| Regular Attack post-battle report (WIN) + region picker | confirmed |
| Covert Operations menu | CORRECTED — accent is green, and the fees are on screen |
| SDI Program colours | confirmed, plus the post-funding echo |
| Configuration Editor | confirmed, plus the highlight colour and the edit screen |
| Message editor | confirmed |
| Game Setup | CORRECTED — it is bracketed by a rule this file omitted |
| Travel Times | CORRECTED — the unmeasured case |
| Alliance Strength with a real ally row | confirmed |
| `-*Relations*-` | CORRECTED — the caller's own realm is not listed |
| Incoming treaty-offer prompt | CORRECTED — it comes *before* the recap entries |

**Still not reached, and still marked in place:** the Attack Type menu was not
re-reached this run (it needs a live two-board league with recon data), but it
is *not* unverified — it was captured end-to-end on 2026-08-11 and its entry
carries those colours. What genuinely remains unverified is listed at each
screen; the biggest are the interplanetary screens that need a second board
running, and the Diplomacy roster-flag contradiction recorded under Full Defense
Alliance.

## How to read the color notes

Colors are given as ANSI SGR codes (what BRE actually emits):

| Code | Color | Code | Color (bright) |
|------|-------|------|----------------|
| 30 | black | 90 | bright-black (gray) |
| 31 | red | 91 | bright-red |
| 32 | green | 92 | bright-green |
| 33 | yellow | 93 | bright-yellow |
| 34 | blue | 94 | bright-blue |
| 35 | magenta | 95 | bright-magenta |
| 36 | cyan | 96 | bright-cyan |
| 37 | white | 97 | bright-white |

Backgrounds: `40`–`47` (and `100`+) — e.g. `44` = blue background. Border rules
mix the single horizontal `─` (U+2500) and double `═` (U+2550) — e.g. a short
`═══` accent set inside a longer `───` line.

## How BRE prints a figure of a billion or more

**In full, with its thousands comma-grouped** — no suffix, no decimal form.
`Bank: 2,000,000,000` appears 8,581 times in `cap/121125-666H4H_Camembert_Public.cap`
alongside `You have 0 gold in hand and 2,000,000,000 gold in the bank.`, and the
bank-history rows run `09/25/2013   $1,001,235,538` / `Today        $1,846,153,847`.
2,000,000,000 is the money cap, so that is the top of BRE's own range and its
screens are sized for the thirteen characters.

**One exception, and it is a prompt bound rather than a displayed figure.**
`cap/shsbbs.cap` carries `How many Jets? (0; 1.0737B):` on the trade deal's
Request side — 1,073,741,824, i.e. 2^30, the sentinel that stands in for "what
the other realm holds is not yours to see". That is the only decimal `B` form in
any capture here. IB prints no ceiling at all on that prompt (see Send Trade
Deal, below).

**Abbreviated columns use a lowercase suffix, and each column keeps ONE suffix
however large its figure runs — the suffix is not a magnitude threshold.** The
See Scores board puts `1962k` and `12m` side by side in a single row
(`cap/20240527-134Pho_Lazarus_Public.cap`): the Score column is fixed on `k` and
the Net Worth column on `m`, at the same magnitude. The Daily Bulletin's third
row stays on `k` well past a million — `Total Net Worth:  25,750k` in
`cap/eots-ibbs-01.cap`, with `2720k` and `Change: +616k` in a smaller game. No
capture reaches billion scale in either column, and a lowercase `b` appears
nowhere.

**IB steps the suffix instead**, `k`/`m`/`b` by magnitude and nothing below a
thousand, one rule on every screen (`numfmt.Abbrev`). A deliberate divergence:
two columns spelling the same magnitude two ways is the part of BRE's behaviour
worth losing, and stepping holds any figure to four digits and a letter.

## Status bar (bottom line, every screen)

Captured 2026-08-16. The whole line carries a `44` blue background. It opens
with the **BBS caller's handle** in `37` white, padded to 20 columns, then three
`94` bright-blue `│` separators with `37` white fields between them — the
empire **letter**, the **realm name**, and last the `■` (CP437 `0xFE`) marker
immediately followed by `F2=Extra Information`:

```
 BORON               │ A │ Boron │ ■F2=Extra Information
```

**Before the caller is matched to an empire** — during the splash and the
new-realm flow — the second and third fields read `UNKNOWN` and `Who knows!`.
That is BRE's own placeholder text, so the bar is drawn before the login is
resolved rather than being suppressed.

This section said the bar opened with the *realm* name and showed two fields; it
is the BBS handle, and there are three.

---

## Attack Menu

Title border: `35` magenta `─────` + `95` bright-magenta `[` + `97` bright-white
`Attack Menu` + `95` `]` + `35` `─────`. Each item: `35`(`+key+`)` with the key
letter in `95` bright-magenta and the parens/label as `35` magenta `(`/`)` and
`37` white label. The default (`Quit`) prompt: `Choice` white, `97` `> `, `37`
`Quit`.

```
─────[Attack Menu]─────
(R) Regular Attack
(N) Nuclear Attack
(C) Chemical Attack
(B) Biological Attack
(P) Attack Pirates
(A) Alliance Strength
(V) Visit Bank
(?) Help
(0) Quit
───────────────────────
Choice> Quit
```

---

## Regular Attack

### Target list

`Choose a Target ` (white) then `31` red `[` + `93` bright-yellow `A` + `37` `-`
+ `93` `Y` + `37` `,` + `93` `?` + `37` `=List` + `31` ` RETURN to Abort]`.
`?` lists players. Header line `-*Barren Realms Elite*-` has `-*`/`*-` in `31`
red and the title in `97`. Column header row in `97` bright-white. Rows:
`35`(`(`)`97`(id-letter)`35`(`)+`)`37`(name) with **Territory** in `95`
bright-magenta, **Score** in `97` bright-white, **Net Worth** in `37` white.
The `+` before a realm name marks it in the list (seen on both own and rival).

```
Id  Empire Name                          Territory     Score    Net Worth
─────═══════════════───────────────────────────────────────────────────────
(A)+Asgard                                     656      5903       47474
(B)+Rome                                        45       213         587
─────═══════════════───────────────────────────────────────────────────────

Choose a Target [A-Y,?=List RETURN to Abort]
```

**IB brackets the id: `[A]`, where the capture reads `(A)`** (#214). The
divergence is deliberate and it is not local to this screen — See Scores and the
recipient picker draw the same table and change with it, and `-*Relations*-`
already brackets its id in the original. Parentheses now say something else on a
roster row: `(O)` for a baron who is online and `(P)` for a realm under New Realm
Protection, so the shape alone separates the key a player presses from the state
being reported. Spelling the same id two ways in one game is what this trades
away.

**IB flags a realm under New Realm Protection `(P)`**, one space after the name,
inside the name field so the figures beside it hold — the same shape as the
pirate raider mark, and the same reason it reserves no column on an unflagged
row. BRE flags nothing; it has no need to, since its own list is drawn before
any of this is decided. The realm KEEPS its selection letter and pressing it is
answered with the refusal by name. IB withheld the letter until 2026-08-26,
which told a player that something about the row was different without saying
what. An ALLIED realm still shows no letter: that standing belongs to the
diplomacy screens, and the player made it themselves.

### Force selection

Re-captured 2026-08-16, unchanged. `You have ` white, counts in `96`
bright-cyan. Each prompt: label white, then `94` blue `(` + `96` bright-cyan
(suggested/default) + `94` `; ` + `36` cyan (max) + `94` `)`, input echoed `97`
bright-white. The default **is** the maximum here, so Enter commits everything —
unlike the interplanetary force prompts, whose defaults are all 0.

```
You have 4359 Troopers, 5000 usable Jets, 26,374 Tanks, and 0 Bombers
Send how many Troopers? (4359; 4359)
     How many Jets? (5000; 5000)
     How many Tanks? (26,374; 26,374)
     How many Bombers? (0; 0)
```

### Post-battle result (WIN)

Re-captured live 2026-08-16 (a winning attack that took 79 regions), and the
description below held on every point. BRE breaks down **both sides' losses by
unit type**, and uses the same "Exhausted from battle" header even on a win. The
captured count is `96` bright-cyan; all other unit numbers are `97`
bright-white; labels `37` white.

**The line before it is an animation.** `And the battle begins` is followed by a
run of about forty dots drawn alternately `31` red and `34` blue, which reads as
a twinkling progress bar on a real terminal. The dot count varies. IB does not
reproduce it, which is fine — but this is where the pause lives, so a
reimplementation that prints the report instantly loses the beat.

```
Your forces have returned..Exhausted from battle...
You lost 43 Troopers, 50 Jets, 263 Tanks, and 0 Bombers.

You destroyed 20 Troopers, 0 Turrets, 0 Tanks, and 0 Jets.

You won the battle!  You captured 15 Regions!
```

On a win with captured land, the **captured-region picker** (below) opens
immediately after this. A win capturing 0 regions shows no picker.

> IB divergence (2026-07): IB's report reads `Victory! You captured N regions.\n
> You lost N units; the enemy lost N.` — a single total per side, no per-unit
> breakdown and different wording. Reconcile toward the BRE text above.

---

## Captured-region picker (shared: regular attack AND pirate raid)

The **same** picker BRE shows after a winning Regular Attack that took land and
after a winning pirate raid that took land. Header `Key Name            Owned`
in `97` bright-white. Rule: `95` bright-magenta, **25 columns** — 5 `─`, 5 `═`,
15 `─` (measured in `cap/kd3-01.cap` and `cap/eots-ibbs-01.cap`; the same rule
the Regions buy screen draws, since one routine serves both). Each row: `35`
magenta `(` + `97` bright-white
key-letter + `35` `)` + `93` bright-yellow region name + `97` bright-white Owned
count. `(*) Advisors` uses the same coloring. **Owned counts are the values
BEFORE this allocation** — BRE applies the picked amounts at the end.

Picker prompt (distinct from the buy/sell "Your choice?"): `34` blue `[` + `96`
bright-cyan (count) + `37` ` Regions left` + `34` `]` + `37` ` Your choice? `.
After a type is chosen: `How many ` white + the type name `96` bright-cyan +
` regions? ` white + `94` blue `(` + `96`
`0` + `94` `; ` + `36` cyan (count) + `94` `)`. The picker loops, decrementing
the count, and auto-exits when it reaches 0.

```
Key Name            Owned
─────═════───────────────
(C) Coastal           131
(R) River               0
(A) Agricultural       12
(D) Desert              5
(I) Industrial        311
(U) Urban               0
(M) Mountain          139
(T) Technology         58
(*) Advisors
─────═════───────────────
[15 Regions left] Your choice?
How many Coastal regions? (0; 15)
```

---

## Attack Pirates

### Faction list — each faction has its own color

`90` gray `(` + `97` bright-white digit + `90` `) ` then the faction name in a
**distinct** color:

| # | Faction | Name color |
|---|---------|-----------|
| 1 | Humans | `92` bright-green |
| 2 | Barbarians | `93` bright-yellow |
| 3 | Solarians | `91` bright-red |
| 4 | Sharks | `31` red |
| 5 | Mechanoids | `35` magenta |
| 6 | Rexxogans | `95` bright-magenta |
| 7 | Xandorians | `94` bright-blue |
| 8 | Monitorians | `36` cyan |
| 9 | Spacians | `96` bright-cyan |

`(0) Quit` in `37` white. (Names are BRE-original; IB should rename the coined
ones per the clean-room rule — tracked separately.)

### Force selection

Same shape as Regular Attack: `Send how many Troopers? (0; MAX)` etc. Pirate
raids commit Troopers / Jets / Tanks (no Turrets/Bombers).

### Post-raid result (WIN)

The faction name keeps its list color. Loot **taken** numbers are `93`
bright-yellow; labels `37` white; own **losses** are `97` bright-white. Regions
appear in the loot line **only when the faction holds land**; otherwise the loot
line omits "Regions" and no picker opens.

```
Your efforts against Xandorians have brought you success!
You took 78k Gold, 8 Regions, 525 Troopers, 481 Jets, 606 Turrets, and 175 Tanks.
You lost 150 Troopers, 350 Jets, and 1000 Tanks.
```

Then the captured-region picker (above) opens for the won regions.

**IB marks a faction that raided you since your last play.** BRE's list has
no such flag. IB writes `«` after the name, one space clear of it — CP437 174,
pointing back at the name it marks, in the `37` gray the online mark (`(O)`,
above) gives the part that must carry its meaning alone. A faction that did not
raid carries nothing at all.

**The head is drawn without a shaft on purpose.** `«` is a text glyph and sits
off the horizontal centerline `═`/`─` are drawn on, so `«═` reads as two marks
side by side rather than as one arrow. A session held to 7-bit ASCII gets `<=`
instead, where both characters share a baseline and the shaft costs nothing —
the general substitution would have reduced the guillemet to a bare `<`. The mark is
cleared and reset each time the income report runs, so it flags only the raiders
named in that turn's report, not an older one.

The mark sat to the LEFT of the name until 2026-08-25, with every unmarked row
holding that column blank so the names lined up. On the common screen — nothing
raided you — the whole list read as indented for no reason, which is how it was
reported (#197).

**IB status (2026-08-09):** matched, with three recorded divergences. IB prints
the gold figure in full where BRE abbreviates it (`78k Gold`), so a full tally
can pass 80 columns and is wrapped at display; IB adds an `Agents` field, which
BRE's raid has no counterpart for; and IB omits the `You lost` line on a win,
because a winning raid costs the attacker nothing here — BRE charges one, and
setting a rate needs evidence rather than a guess. The headline is one of five
picked at random (`raidWinLines` / `raidFailLines` in `internal/game/pirates.go`),
BRE's own wording first and four of IB's after it — a deliberate divergence, so
a player raiding the same faction repeatedly is not read the same sentence.

---

## System Menu

Reached from the Spending Menu via `(*) System Menu`. Border `34` blue rule +
`94` bright-blue `[` + `97` `System Menu` + `94` `]` + `34`. Items in three
columns: `34` blue `(` + `94` bright-blue key + `34` `) ` + `37` white label.

```
────────────────────────────[System Menu]────────────────────────────
(#) Abdicate           (M) Messages           (1) Set Industries
(A) Visit Advisors     (P) Preferences        (2) Show Instructions
(D) Diplomacy          (R) Set Tax Rate       (4) Spy Database
(E) Empire Status      (S) See Scores         (5) Coordinator Vote
(F) Food Market        (T) Trading            (*) Coordinator Menu
(G) Game Setup         (V) Visit Bank         (0) Quit
(I) InterBBS Scores    (W) Write Macros
─────────────────────────────────────────────────────────────────────
Choice> Quit
```

`(C) Covert Operations` sits between `(A) Visit Advisors` and `(D) Diplomacy`
when the realm holds at least one covert agent; it is absent above because that
capture had none. Verified across the `cap/` captures: the item appears at the
exact point agents go from 0 to nonzero (eots, shsbbs, bre-9-eots), and it shows
even with the "Visit Covert Menu" preference set to No, so the preference gates
only the per-turn step.

---

## Set Industries (System Menu → 1)

Title `31` red rule + `91` bright-red `[` + `97` `Industrial Production` + `91`
`]` + `31`. Each row: label `37` white + `:` + percent value `93` bright-yellow
+ `%` white + `31` red `(` + `91` bright-red (per-year count) + `37` ` per year`
+ `31` `)`. `Specialized` tag in `93` bright-yellow.

**IB divergences:** IB writes `* * *` in bright-yellow where BRE
writes `Specialized`, in the same position at the end of the row — a marker with
no word to translate. IB also lists a **Gold** row last, with no per-year figure,
so capacity can be reserved for gold rather than only left over; it starts at 0%,
and unallocated capacity still pays gold as before. The Gold row carries a
per-year figure like the units, from the same function that credits it. Figures
in that column are comma-grouped, which BRE does not do — the same divergence
recorded for figures elsewhere. Gold is not offered as a specialization.

```
───────────[Industrial Production]────────────
Troopers        :   0%       (0 per year)
Jets            :   0%       (0 per year)
Turrets         :   0%       (0 per year)
Bombers         :   0%       (0 per year)
Tanks           : 100%       (2496 per year)  Specialized
Carriers        :   0%       (0 per year)
──────────────────────────────────────────────
Change Production? (y/N)
```

`Change Production?` prompt: white + `36` cyan `(` + `96` bright-cyan `y` + `36`
`/` + `96` `N` + `36` `)`. On `y`, per-unit percentage prompts follow:
`Troopers       (0; 100)` — label white, `94` blue `(` + `96` `0` + `94` `; ` +
`36` `100` + `94` `)`. Percentages allocate across the six unit types.

---

## Advisors (System Menu → A)

Menu: `35` magenta rule + `95` bright-magenta `[` + `97` `Advisors` + `95` `]` +
`35`. Four advisors, items `35`(`(`)`95`(key)`35`(`) `)`37`(label):

```
───[Advisors]───
(1) Civilian
(2) Economic
(3) Military
(4) Technology
(0) Quit
```

Each advisor prints prose (no inner border); key numbers are highlighted. The
`─»>Paused<«─` bar is `36` cyan with `96` bright-cyan `Paused`.

**Deliberate divergence — IB gives all four advisors a named greeting, in `96`
bright-cyan.** The original names only the MILITARY one: `", your military
advisor."` is the single greeting string in `BRE.OVR`, and the capture shows no
opening line for the other three and no colour for advisor prose. IB coins a
name and a line of character for each of the four, which Andy approved on
2026-08-15. Do not "correct" this back.

### (1) Civilian — food outlook

Numbers `97` bright-white; a deficit figure in `93` bright-yellow (warning).

```
We currently produce a minimum of 3708 units of food per year.
We occasionally produce an additional 0 units from rivers.
The empire consumes 4928 units of food yearly.

This gives the empire a maximum food deficit of 1220 units per year.
At our current population, we can survive for at least 2 years.
```

### (2) Economic — income and efficiency

All figures `93` bright-yellow.

```
Your yearly income was 1,273,887 Gold, 100% of the world total.
Your barony's efficiency is approximately 1941 Gold per Region.
The global average is approximately 1941 Gold per Region.
```

### (3) Military — named advisor + conditional advice + force tally

The advisor is **named** ("Hi, I'm Joe, your military advisor."), and the name
itself is `97` bright-white against the `37` white greeting. Advice lines
are conditional on empire state (missing HQ, carrier shortage, etc.); unit types
named in advice are `93` bright-yellow. Force counts `97` bright-white.

```
Hi, I'm Joe, your military advisor.
   Sire, your headquarters really needs to be constructed.  It makes a large
      difference in the strength of your Tanks.
   We have a shortage of carriers in the empire.  All of our jets cannot
      be put to use in offensive attacks as it is right now.


   Your total military force consists of 4359 Troopers, 7581 Jets,
      6360 Turrets, 24k Tanks, and 0 Bombers.
```

### (4) Technology — the full effect list

Each aspect name `97` bright-white, each percentage `93` bright-yellow. Confirms
the Technology-region mechanic's full set of effects. The NOTE has `96`
bright-cyan `NOTE:` + `36` cyan body.

```
Because of technology...
  Our military forces are functioning at 101% strength.
  Our gold producing regions are at 101% of normal production.
  Our food production techniques increased efficiency to 103%.
  Our industries are running at 101% efficiency.
  Our maintenance costs have been reduced to 99% of standard costs.
  Our SDI yearly funding needs have been lowered to 99% normal expenses.
  Food decay is at 94% of standard levels.

NOTE: Technology levels are relative to the number of regions
      in the empire.  Larger empires need more advanced technology to
      maintain the same efficiency as smaller realms.
```

---

## Second capture (full IBBS turn, 2026-07)

The sections below were added from a second live capture: a complete play-turn on
a 4-board InterBBS game (v0.988), scraped with ANSI escapes intact. All real
empire, player, planet, board, and operator names — and the operator locations in
the Spy Database — are **placeholders** here; only BRE's own UI text, layout, and
colors are ground truth. This was an InterBBS game with **local player-vs-player
attacks disabled**, so the Attack Menu is the reduced variant (see below).

### Per-menu accent color

The menu grammar from the top of this file (border + `(`key`)` + white label +
`Choice> Quit`) is constant, but **each menu recolors the accent** (the border
and the parens/hotkey). The title text inside `[ ]` is always `97` bright-white;
the accent is:

**IB drops the word "Menu" from menu TITLES** — `[Attack]`, `[Spending]`,
`[System]` where BRE writes `[Attack Menu]`, `[Spending Menu]`, `[System Menu]`,
and the opening menu is `[Entry]` where BRE writes `[Barren Realms Elite]`. A deliberate divergence: the
box is plainly a menu, so the word is a wasted word in every title and in every
translation of it. The *items* that navigate to a menu keep it (`(*) System
Menu`), since there the word says where the item goes. The German and Russian
catalogs had already dropped their "Menü"/"Меню" element for Diplomacy before
this change.

**Box widths vary per screen** — BRE sizes each box to its content rather than
to one house width. Re-measured 2026-08-16 across every capture on disk
(`cap/`, plus the two older colour captures), counting the title line and the
closing rule of each box: 14/16 (Specialization), 17 (Attack Pirates), 16
(Advisors), 20 (Trading), 23 (Attack Menu, Terrorist Ops), 26 (Coordinator
Ops), 28 (Diplomacy), 36 (Preferences), 37 (Special Operations), 38 (InterBBS
Scores), 44 (Spending, Sell), 46 (Industrial Production), 52 (opening), 60
(Crazy Gold Bank), 58/62/64/68 (InterPlanetary Ops), 64 (Food Unlimited), 69/75
(System Menu).

**The closing rule is exactly the box width.** True of 18 of the 19 box titles
found across the five captures; Specialization is the one exception, and only
because its title is wider than its content (below). Several code blocks in this
file used to draw the closing rule one column wider than its own title line —
those were the blocks being wrong, not BRE being inconsistent.

IB's `rule` constant is 62; a screen that draws
its own box should take its width from its own capture instead. IB sets `Width`
per menu where a capture gives one: Attack 23 (its content measures 23 too, so
the fit is exact), Spending 44, Industrial Production 46, Specialization 14.
**System is 59, not BRE's 69/75** — BRE lays that menu out in three columns and
its box tracks whichever items are showing, IB uses two, so IB follows the
principle (size to content) rather than the number.

**Specialization IS captured** (this file said it was not until 2026-08-16).
BRE draws its title line as a bare `[Specialization]` with **no fill at all** —
16 columns, because the title is wider than the menu — and closes it with a
**14**-column rule, the width of its widest item (`(1) Troopers  `). So the box
is content-sized like every other, and the title simply overhangs it. **IB now
draws it at 14**, having drawn it at 46 until 2026-08-16 to sit beside Industrial
Production. `titleRule` collapses its fill runs to zero rather than falling back
to a bare label, so an overhanging title keeps the bright brackets and
bright-white text a filled one has — which is what the capture shows. IB's item
labels are still laid out its own way (`  1) Troopers`, unpadded, where BRE pads
each to the full 14), and the width grows past 14 when a translated label needs
it, since sizing to content is what produces 14 in the first place.

**InterPlanetary Operations is the one box whose width moves** — 58, 62, 64 and
68 all appear, because the Terrorist Ops price column grows with the figure it
carries. Take 62 from the block below only as one sample.

| Menu | Accent |
|------|--------|
| Opening menu (`Barren Realms Elite`) | `35` magenta |
| See Scores / Regions buy / Attack Menu / Advisors | `35` magenta |
| Diplomacy Menu / Sell Menu | `32` green |
| Crazy Gold Bank / Food Unlimited / Preferences | `36` cyan |
| Spending Menu / Industrial Production / Trading / Specialization / Special Operations / Terrorist Ops | `31` red |
| System Menu | `34` blue |
| InterPlanetary Operations | `33` yellow |
| InterBBS Scores / Attack Pirates | `90` bright-black (gray) |

The whole table was re-read from the captures on 2026-08-16 by matching the SGR
run in front of each `[Title]`. Every menu is consistent across all five
captures. **Diplomacy is GREEN, not cyan** — this file said cyan until then, and
all four captures that contain the menu draw its rule `32` and its brackets
`1;32`. (IB's code was already green; only the doc was wrong.)

The two gray menus put the accent on the rule and the parentheses only: their
title brackets are plain white `37` and their hotkeys `1;37` bright-white, where
every other menu brightens its own accent for both.

Data-value convention (status, income, bank, food, end-of-turn): the **numbers**
are `96` bright-cyan, the surrounding label text `37` white. Maintenance-paid /
food-consumed lines and the Spending-Menu Price/#Owned columns use `97`
bright-white numbers instead — and so does a **menu's own status footer**
(`You have N gold and N turns.`, `…and N units of food.`), on every menu that
carries one.

### Daily maintenance (login, before the opening menu)

Captured live in `cap/treaty-order-20260817.cap`. The header carries a `93`
bright-yellow `■` marker and `37` white text; each task is its own line
indented 5 columns, printed as it is carried out and scrolled up a line at a
time; the closing line is `97` bright-white. The tasks BRE names in order:
checking for dead empires, the news bulletin, covert operations, investment and
loan information, packing trade deals / covert operations / treaties, awarding
the Planetary Master, depositing trading market money, duplicate scanning
files, inbound and outbound doomsday attacks, old recons, transfer times,
duplicate users, SpyGuys, packing data packets.

```
■  Running Daily Maintenance
     Checking for Dead Empires
     Updating Daily News Bulletin
     Processing Covert Operations
     ...
     Daily Maintainence Complete
```

**Deliberate divergence:** IB names its own tasks, in its own words, and lists
only the ones that had something to do — its maintenance is not BRE's, and half
those lines belong to file formats IB does not have. The shape (marker, header,
indented task lines, bright closing line) is the same.

### Opening menu (top-level)

Shown after login and after each news screen. Magenta accent; two columns.

```
───────────────[Barren Realms Elite]────────────────
(1) Play Game             (8) Game Bulletins
(2) See Status            (9) InterPlanetary Ops
(3) See Scores            (A) Game Instructions
(4) See Today's News      (B) Help Database
(5) See Yesterday's News  (P) Preferences
(6) Read Messages         (0) Quit
(7) Send Messages
────────────────────────────────────────────────────
Choice> Play Game
```

**52 columns**, not the 47/48 this block used to draw: 15 `─`, `[Barren Realms
Elite]`, 16 `─`, closing rule 52. That is exactly its content — two 26-column
item cells (`(1) ` plus a 22-column label field).

### Daily News File / Daily Bulletin

Header line: `93` bright-yellow `Barren Realms Elite ` + `97` `v0.988` + `37`
`: News File` with the date right-aligned. Then a blank line, then the banner,
indented 24 columns — centred over the 75-column rule below it, not over the
80-column screen: `31` red `──` + `91` bright-red `═` + `97` bright-white
`The Queen's Quadrant` + `91` `═` + `31` `──`, then a full-width `33` yellow rule.

**The banner's inner glyphs are `═` (CP437 `0xCD`), not `»`/`«`.** This file
recorded them as guillemets until 2026-08-16; the bytes around the title in
`bre-01-color.cap` read `c4 c4 … cd … cd … c4 c4`. The same idiom draws the
`──═Planetary Post═──` and `──═Planetary Treaties═──` banners, which this file
already had right — three screens, one decoration. (The `─»>Paused<«─` bar is a
different decoration and really is `0xAF`/`0xAE`.)

The Daily Bulletin box has a `34` blue border with `97` bright-white
`Daily Bulletin` in the top edge. Three rows; label `37` white, value `36` cyan,
`Change:` `37` white. **Positive** change = `92` bright-green `+` then `96`
bright-cyan value; **negative** change = `96` bright-cyan for the whole thing
(minus sign included — direction is not color-coded).

**IB matched this on 2026-08-16**, having drawn a rise in `32` green and a fall
in `31` red. The old colouring was also a contrast defect: `31` red on black
measures **2.7:1** on the VGA palette (#AA0000, relative luminance 0.0853,
`(0.0853 + 0.05) / 0.05`), under the 4.5:1 text minimum. `96` bright cyan is
17.1:1 and `92` bright green 15.8:1, both far clear of it. Nothing is lost by
dropping the red/green pairing, because the `+`/`-` sign already carries
direction on its own — including in a monochrome or ANSI-less session.

**The box is drawn in SINGLE-line CP437, `┌ ─ ┐ │ └ ┘`, and is 66 columns wide**
— not the double `╔ ═ ╗ ║ ╚ ╝` at 68 this block used to show. Indented 4; top
edge is `┌` + 25 `─` + `Daily Bulletin` + 25 `─` + `┐`, bottom `└` + 64 `─` +
`┘`. Measured identically in `bre-01-color.cap` and the public-board capture.

In a league IB puts the board's name and an em dash ahead of `Daily Bulletin` in
that top edge, and the edge is sized to the title. The width has to be measured
in the caller's OWN charset: a CP437 or ASCII terminal is sent `--` for the em
dash, so a rune count made the top edge a column longer than the box under it
(#192).

```
                        ──═The Queen's Quadrant═──
───────────────────────────────────────────────────────────────────────────
    ┌─────────────────────────Daily Bulletin─────────────────────────┐
    │  Total Population: 185,861             Change: +19243          │
    │  Total Regions:    25,283              Change: +1167           │
    │  Total Net Worth:  2720k               Change: +616k           │
    └────────────────────────────────────────────────────────────────┘
```

Below the box, the news items. Each item starts with `31` red `──` + `91`
bright-red `─` arrow, then `37` white body; wrapped continuation lines indent 5
spaces. **A blank line separates every item.** In-line highlights, read off a
live capture in `cap/` (2026-08-13) and confirmed against a second session two
weeks earlier:

| element | code |
|---|---|
| every empire — the reader's own included | `1;33` bright-yellow |
| pirate factions (Humans, Barbarians, Spacians, …) | `31` red, returning to `37` |
| numbers | `1;37` bright-white |
| `Planetary Master` title | `1;37` bright-white |
| `The Queen Royale` actor | `1;31` bright-red |

**BRE gives your own realm no distinct color in the news** — three different
realms in a single captured screen all render `1;33`. A realm name has no one
color in BRE: the same name renders `1;36` on the recap and in message headers,
`1;33` here and at the target-picker echo, and `37` in plain lists. Take the
color from the screen, never from a sibling screen.

**Deliberate divergence: IB paints pirate factions `1;31` bright-red, not BRE's
`31`.** Plain red on black measures 2.71:1, below the 4.5:1 floor for text, and
the faction name is the payload of a raid item. Brightening keeps BRE's hue and
reaches ~5.2:1. Everything else on this screen matches.

Today's News and Yesterday's News use this same layout. (IB is free to reword
the prose — clean-room — this records BRE's wording and coloring only.)

### See Scores (local planet)

Title `-*` `91` red + `97` bright-white `Barren Realms Elite` + `*-` red. Column
header row `97` bright-white. The border mixes single `─` and a `══` double-line
accent over the name column. Each row: `35`(`(`)` + `97` key + `35`)` + a flag
column + `37` white name + `95`
bright-magenta Territory + `97` bright-white Score + `37` white Net Worth.

**The `+` flag is BRE's own, meaning unverified.** This file has called it
"participating this reset", and elsewhere just "marks it in the list". Neither
survives the 2026-08-16 capture: across one session the flag moved on and off
individual realms with no reset in sight — a three-realm game showed all three
flagged, then `A +`, `B —`, `C +` after `A` played and attacked `B`, then
`A +`, `B +`, `C —` after `B` played. It is per-viewer or per-day state of some
kind and nobody has pinned which. Do not build anything on the current wording.

IB uses it for its own "played today" marker: a realm that played today but is
not online now gets `+` in the same column. The `(O)` online mark takes priority
when both apply.

**IB diverges from BRE by suppressing the `+` for your own realm.** BRE's
capture shows the flag on the caller's own row too (`(E)+Your Empire`); IB
omits it there — your own status is obvious from context, and the row is
already singled out by its bright-yellow name.

```
-*Barren Realms Elite*-

Id  Empire Name                          Territory     Score    Net Worth
─────═══════════════───────────────────────────────────────────────────────
(A)+Empire One                               6936    536115     1344439
(B)+Empire Two                               5749    442683      692769
(E) Your Empire                              1140      4260       24814
─────═══════════════───────────────────────────────────────────────────────
```

The rule is **75 columns — 5 `─`, 15 `═`, 55 `─`** in `35` magenta, the same
figure the attack picker and the recipient picker draw, since one routine serves
all three. This block showed 72 (with a 10-column `═` accent) until 2026-08-16;
no capture produces that. Note the *heading* row is 72, three columns short of
its own rule — that mismatch is BRE's.

**IB adds an online mark (#123).** BRE has no online indicator; IB writes `(O)`
for a baron who acted within `OnlineWindowSecs`, hugging the LEFT of the empire
name inside the name field. Your own realm is never marked. Every local roster
carries it: the attack target picker and the recipient picker share
`scoreTableRow`/`nameCell`, and the Coordinator's Player List puts it in its
indent. The inter-BBS scores screen does NOT — those figures arrive in packets
that may be hours old, so there is nothing to report.

The protection flag `(P)` rides in the same field, after the name rather than
before it (see "Target list" above, #214). It reserves no column on an unflagged
row.

**The name field is measured in the caller's own charset.** `ValidRealmName`
accepts any printable rune, so a realm may be named `Iron—Fist`; the CP437 and
plain-ASCII writers rewrite that em dash as two hyphens below every layer that
counts columns, and a rune count walked the figures one column right for the
default charset. `nameCell` takes `Term` and measures through `visWidth` /
`fitColumn`, the same pair the InterBBS Scores tables use (#192, #196).

An unmarked row reserves the mark's width, so the names stay in one column
either way; `markWidth` measures the translated letter rather than assuming one
character, since the `O` goes through `tr`.

The parentheses are `90` dark gray and the letter `37` gray. Measured on black:
the letter is **9.0:1** on the VGA palette and 16.7:1 on xterm's, comfortably
over 4.5:1. The parentheses are **2.8:1** on VGA (5.3:1 on xterm), under the 3:1
non-text minimum — they are deliberately dim framing, and the letter inside them
carries the meaning on its own, including on a monochrome or ANSI-less session.
Brightening them to `37` as well would make the mark compete with the empire
name it sits against, which is the thing a reader is scanning for.

An earlier revision drew the mark as a reverse-video `93`+`7` cell in the ID
column, in the slot BRE uses for its participation `+`. Changing it moved the
mark next to the name it describes; `3c4a789` is the commit.

### Turn income, status block, maintenance

At turn start BRE prints the income lines (numbers `96` bright-cyan), then the
status block bordered by `34` blue rules, then the maintenance-paid lines (`97`
bright-white numbers).

**Two different rule widths are in play here, both `34` blue inset rules.** The
income lines open under a **75**-column one (5 `─`, 15 `═`, 55 `─`); the status
block is bracketed by a **70**-column one (5 `─`, 14 `═`, 51 `─`) above and
below. Nothing closes the maintenance lines — the pause follows them directly.
The rules are omitted from the block below only to keep the field list readable.

**Deliberate divergence:** BRE ends each manufacturing line with "were
manufactured by Industrial Zones." IB says that once, as a heading, and lists
the units under it — one per line, figure first, and no line for a type that
built none.

**The bank's returns close the block.** After the manufacturing lines BRE prints
`N gold was earned from investment returns.` — the day's matured investments, and
the SAME figure on every turn of that day (`cap/eots-ibbs-01.cap`: one day's
14,699,020 repeated across all ten turns). **IB adds a line above it** for the
savings interest credited at the end of the previous turn, which BRE reports
nowhere; the wording is IB's own, in the style of the lines around it (#216).

```
227,717 gold was earned in taxes.
19,510 gold was produced from the Ore Mines.
4,380,288 gold was earned in Tourism.
15,490 gold was earned by Solar Power Generators.
120,384 gold was created by Hydropower.
4228 Food units were grown.

-*Your Empire*-
Turns: 10
Score: 4473
Gold: 6,156,219
Bank: 7,255,312
Population: 3386 Million (Tax Rate: 12%)
Popular Support: 100%
Food: 6441
Military: [42,259 Troopers]
          [100% Morale]
Regions:  [24 Rivers] [14 Agricultural] [5 Desert] [4 Urban]
          [5 Mountains] [1088 Coastal]
You have 100 Years of Protection Left.
```

The status title `-*name*-` uses `96` bright-cyan `-*`/`*-` with `97` bright-white
name; every field value is `96` bright-cyan, labels `37` white.

**There is no box.** The block is a plain run of lines at column 0 — no border,
no side rails, no fill. IB drew it as two boxed pages of its own invention until
v0.0.4; both are gone and IB now renders the block above.

This paragraph said "no rule" until 2026-08-16, which the captures contradict:
the 70-column blue inset rule described above sits immediately before `-*name*-`
and immediately after the protection line. A rule above and below is not a box,
and both statements have to be made separately. **IB draws both rules** as of
2026-08-16; it drew neither before.

**Deliberate divergence:** IB itemizes the income differently from BRE —
right-aligned amounts in one column, a subtotal rule, and a total line BRE does
not print. The lines and their wording are BRE's.

IB drew a blue-backed `Income Report` bar of its own above them until
2026-08-25, where BRE has the 75-column rule and no heading at all. Corrected:
the rule is what opens the block now. That bar was the only filled-background
heading in the game, so the style went with it.

**The capture is of a realm that holds little, and the block is shorter than the
field set.** The status routine (`BRE.OVR` `show_empire_status`, 0x193b2) is a
straight run of `if figure > 0` guards, so a zero field prints nothing at all.
Its string table declares every label in display order:

```
-*name*-  Turns  Score  Gold  Bank  Population (Tax Rate)  Popular Support
Food  Agents  Headquarters (N% Complete)  Military  SDI Strength  Regions
```

Only Score, Population, Popular Support and Regions are unconditional; the rest
appear when their figure leaves zero. `Military:` prints `None` for an empty
army, and the Morale cell is left out entirely with it.

The **Military and Regions rows** are cells of `[figure Label]`, and both break
onto continuation lines indented ten columns — under the first bracket, since
`Military: ` and `Regions:  ` are both ten wide. The break is by cell count, not
width: a shared helper takes the threshold as an argument and is passed **3** for
Military and **4** for Regions. It compares the running count for **equality**,
so the row breaks exactly ONCE however many cells follow — a realm holding all
nine region types gets four cells then five. Morale always sits alone on the
line after the units.

Military cells are declared Troopers, Jets, Turrets, Tanks, Bombers, Carriers.
Region cells are Rivers, Agricultural, Desert, Industrial, Urban, Mountains,
Coastal, Technology, **Waste** — the ninth count at record `+0xb6`, absent from
this capture only because the realm held none. That row order is the status
block's alone: the record itself, the Buy Regions screen and every other display
lead with Coastal (`bre-save-format.md`, `+0x96`).

**Deliberate divergences, all of them IB's:**

- **Population is a head count, not millions.** BRE stores population as a count
  of millions and writes `3386 Million`; IB counts people directly (see
  `balance_costs.go`, "Population / migration"), so the line reads
  `Population: 3,386 (Tax Rate: 12%)` — the same figure the Civilian advisor and
  the Daily Bulletin's Total Population row print, in the same unit.
- **Figures are comma-grouped**, as everywhere else in IB.
- **Regions run in IB's one region order** — the Buy Regions order the region
  table, the pickers and the record all use, Waste last. BRE re-sorts this row
  and nothing else; IB keeps a single order rather than carry a second one for
  one screen.
- **Region and unit names are IB's singular labels** (`[14 River]`), the same
  strings the region table prints, rather than the plurals BRE writes here.
- **The rows break every 3 / 4 cells, not once**, and break early when the next
  cell would reach the 80th column. IB's figures carry thousands separators and
  its region counts have passed 250,000 in a long game, where BRE's single break
  overruns the line and the terminal wraps it on top of the next.
- **Nothing is added.** IB shows BRE's field set and no more — no Debt, no net
  worth. `Score:` is `Empire.Score`, the cumulative score the scores board
  ranks on (BRE reads it from record `+0x286`), not net worth.

A pirate raid suffered since the last play is reported as the report's **last**
line, after the Industrial Zones production lines and before the pause, at
column 0:

```
Rexxogans has captured 927 Turrets
```

The faction name carries its own color (`1;35` here — the per-faction palette in
`pirateColors`), `has captured ` is `37` white, the figure `1;36` bright cyan,
the unit word white. No trailing punctuation. It names exactly ONE thing: the
raid draws a single category (see `docs/mechanics-reference.md`). **Deliberate
divergences:** IB writes "have captured", since the faction names are plural;
indents the line two columns to sit with the rest of its income report; and puts
a blank line above it, where BRE runs it straight on from the last production
line.

The closing line is one of a pair — `BRE.OVR` holds `You have  Years of
Protection Left.` beside `This is year  of your freedom.`, so the same slot
counts protection down and then counts up. BRE's turn is its year: protection is
a turn count it prints as years. **Deliberate divergences:** IB writes the first
as `You have N turns of protection left.`, naming the unit `Empire.Protection`
actually holds — Andy has confirmed the altered wording. And it writes the second
as `This is year N of your reign.` "Freedom" reads as though the realm had been
captive, when what ended was the shield.

### Crazy Gold Bank

Cyan accent, two columns. Followed by the gold-in-hand / in-bank line (both
figures `97` bright-white).

```
─────────────────────[Crazy Gold Bank]──────────────────────
(C) Cash Relief / Loans       (L) List Investments / Loans
(D) Deposit Funds             (V) View Bank Rates
(W) Withdraw Funds            (0) Quit
(I) Investments
────────────────────────────────────────────────────────────
You have 4,582,875 gold in hand and 7,255,312 gold in the bank.
Choice> Quit
```

**60 columns** (21 `─`, `[Crazy Gold Bank]`, 22 `─`), closing rule 60 — this
block drew 61/62 until 2026-08-16.

Deposit / Withdraw prompt form: `Withdraw how many gold? (0; 7,255,312)` — the
parenthetical is `(minimum; maximum)`.

### Spending Menu

Red accent. Decoration line is **44 columns wide** (14 fill + `[Spending Menu]` +
15 fill). Columnar: `Key Item / Price / # Owned` header (`37` white); each row has
`31`(`(`)` + `91` key + `31`)` + `37` white label, then `97` bright-white Price
and `37` white # Owned right-aligned. `(*) System Menu`, `(S) Sell`,
`(V) Visit Bank`, `(?) Help`, `(0) Quit` carry no price/count.

```
──────────────[Spending Menu]───────────────
Key Item                 Price       # Owned
(*) System Menu
(1) Troopers              250         42259
(2) Jets                  318             0
(3) Turrets               359             0
(4) Bombers              3006             0
(5) HeadQuarters         6580             0
(6) Regions             38537          1140
(7) Covert Agents         895             0
(8) Tanks                2020             0
(9) Carriers             5315             0
(S) Sell
(V) Visit Bank
(?) Help
(0) Quit
────────────────────────────────────────────
You have 4,582,875 gold and 9 turns.
```

Prices drift turn to turn (region-cost-change setting); `# Owned` is live.

The footer's turn count is the turns remaining **after** the current one, so it
is one less than the Empire Status "Turns:" line (10 there, "9 turns" here); the
last turn of the day reads "0 turns".

### Regions buy screen (Spending → Regions)

```
There are 138,322 Regions available.
Note: Region prices are constantly changing.  Therefore, the region price
      shown is only the price for the first piece of territory you buy.
You can afford 274 regions.

Key Name            Owned
─────═════───────────────
(C) Coastal          1088
(R) River              24
(A) Agricultural       14
(D) Desert              5
(I) Industrial          0
(U) Urban               4
(M) Mountain            5
(T) Technology          0
(*) Advisors
─────═════───────────────
Your choice?
```

Same 25-column rule and the same colors as the captured-region picker above —
one routine draws both. `There are N Regions available` and `You can afford N`
carry their figures in `96` bright-cyan against `37` white body.

Picking a region type prints its one-paragraph blurb, then
`Buy how many <Type> regions? (0; <max>)`.

After a purchase, BRE stays on the region-type picker (`Your choice?`) so more
types can be bought in the same visit — **unless no further region could be
bought**, in which case it returns to the Spending Menu. Two separate causes were
observed: gold exhausted (five max-affordable buys, each exiting), and the land
allowance exhausted (a 51-region buy that left 668,204 gold against a 24,281
price, and still exited). A partial buy with both gold and allowance remaining
(15 of 44) stayed on the picker. So the trigger is "cannot buy another region",
not "bought the maximum offered" and not gold alone.

The buy screen states the land remaining (`There are 49,451 Regions available`).
This is the realm's **own Daily Land Creation allowance**, not a shared planetary
pool — see the binary-verified note on `Empire.LandAvailable` in
`internal/game/game.go`. Live captures agree: the figure fell 49,623 → 49,451
across a 172-region purchase, i.e. by exactly what this realm bought.

Once the allowance is spent, selecting `(6) Regions` prints
`no land is available at this time` above a redrawn Spending Menu — the region
screen never appears. Observed to persist across a turn boundary, so the top-up
comes with daily maintenance rather than each turn. `You can afford N` is
`min(affordable, allowance remaining)`, so a quoted N is not by itself evidence
about price.

IB's `buyLand` (`internal/menu/actions_regions.go`) loops the picker until the
player quits with `0`, which matches the partial-purchase case but keeps looping
when gold runs out.

### Diplomacy Menu

**Green accent** (`32` rule and parens, `1;32` brackets and hotkeys), single
column. (Reached pre-turn in this build, and from the System Menu.) This section
said cyan until 2026-08-16; see the accent table above for the evidence.

```
──────[Diplomacy Menu]──────
(1) Tariff Trade Agreement
(2) Protective Trade
(3) Free Trade Agreement
(4) Terrorist Prevention
(5) Intelligence Alliance
(6) Technology Agreement
(7) Full Defense Alliance
(8) Declaration Of War
(9) View Treaties
(?) Help
(0) Quit
────────────────────────────
Choice> Quit
```

### Industrial Production (Change Production display)

Red accent. Shown before the `Change Production? (y/N)` prompt.

```
───────────[Industrial Production]────────────
Troopers        :   0%       (0 per year)
Jets            :   0%       (0 per year)
Turrets         :   0%       (0 per year)
Bombers         :   0%       (0 per year)
Tanks           :   0%       (0 per year)
Carriers        :   0%       (0 per year)
──────────────────────────────────────────────
Change Production? (y/N) No
```

### InterPlanetary Operations

Yellow accent, two columns. `Terrorist Ops` shows a cost figure next to the
label — **the launcher's region count × 64 at the default Terrorist Costs
setting**, so it climbs as the realm buys land. The `72,960` below is 1,140
regions; a second capture (`cap/eots-ibbs-01.cap`) gives 541,824 / 542,144 /
542,336 / 542,976 against 8,466 / 8,471 / 8,474 / 8,484 regions, every one
exact; a third on 2026-08-16 gives 47,232 against 738. The derivation and the
one term still unpinned are in `docs/mechanics-reference.md`.

**The 64 is the Medium figure — the sysop's `Terrorist Costs` knob scales it.**
Set to **High**, a 15-region realm was quoted **2,880**, which is
`15 × 64 × 3`. So the multiplier is a preset factor and 64 is its Medium value;
`×3` for High is one sample and the Low / None factors were not measured, so
**UNVERIFIED** beyond that. This is the opposite of the local covert fees, which
the same experiment proved do not move at all (see Covert Operations).

Colors, from `cap/eots-ibbs-01.cap`: the rule is `33` yellow with the brackets
`1;33` bright yellow and the title `1;37` bright white; each item is a `33`
yellow paren around a `1;33` bright-yellow key, then a `37` white label, and the
cost figure is `37` white. This is the same accent/dim/bright-white split the
menu engine's `titleRule` already draws.

```
─────────────────[InterPlanetary Operations]──────────────────
(1) View IPScores              (9) Gooie Kablooie Ops
(2) Terrorist Ops       72,960 (A) SDI Program
(3) Send Trade Deal            (D) Diplomacy List
(4) Create Group Attack        (S) Spy Database
(5) Join Group Attack          (T) Travel Times
(6) Indiv. Attack Force        (V) Visit Bank
(7) Send Message               (?) Help
(8) Special Operations         (0) Quit
──────────────────────────────────────────────────────────────
You have 0 gold.
```

Gates seen: `Sorry....You are under New Realm Protection!` (Terrorist / Special
Ops while protected), `There are not any attack parties at this time.` (Join
Group Attack).

**Send Trade Deal** (3). **The sending flow is not captured**; it was read out of
the binary instead (#195). A different routine from the local deal —
`send_trade_offer` (BRE.OVR 0x24212), whose only caller is the InterBBS menu.
The block that used to sit here was the LOCAL deal and has moved to the Trading
section, which is what wired IB's interplanetary item to the local mechanic.

The order it asks in: **planet, then realm, then goods, then the price.**
`select_planet` (0x021dd9) takes a planet name or number, or a key that lists
them, and refuses two answers outright by returning 0 — **your own planet**
(`cmp ax,[0x6795]` at 0x18a6) and a planet you hold no data on — so the item
cannot reach a realm at home. It then prints your standing with the planet
chosen. `select_player` (0x022a0c) takes a letter A-Y, a raw index into that
planet's 25-slot roster with the gaps left in, validated against the slot's own
id being non-zero; `?` lists the roster and `/` searches it by name.

**A flat fee, no day span, no treaty.** It calls the same cost routine the local
deal does (`calculate_trade_offer_cost`, 0x0513e7) and then simply stops: no
multiply by days, no Protective Trade divisor, no days prompt. The formula is the
nine goods against fixed weights, over five, plus a 100,000 base, scaled by the
sysop's Trade Deal Costs ladder and capped at two billion. Neither realm needs a
pact — the routine reads no relation field and holds no relation string, where
the local one refuses a realm you have no pact with.

**The arrival gets no choice.** `resolve_received_trade_offer` (0x043df1) checks
for a duplicate, checks the target's stored id against the letter, and credits
every good straight onto the realm, each capped at two billion. It files a
private report for each side and writes **no news at all** — no `.dat` template
carries a trade-deal category. A deal aimed at a realm under New Realm Protection
is DESTROYED: the routine returns, and neither side is told. IB refuses that
target at the picker instead; see `docs/mechanics-reference.md`.

**Carriers are sized to the cargo** (`ovr_050dfb_entry_0436`): troopers 1,000 to
a carrier, jets 100, turrets 1,000, tanks 5,000, gold 100,000, with food,
bombers, agents and carriers riding free — summed and rounded up as a total.
A capture has the original asking for twenty.

**The receiving side IS captured**: `cap/20240527-134Pho_Lazarus_Public.cap`
carries 13 arrivals, each a header naming the realm and its planet over an
indented list of what was shipped. Gold is in eleven of them, one is a mixed
shipment, ten are the same sender to the same recipient in a single run — so
nothing caps how many a realm may send in a day.

**Travel Times** (T). Captured 2026-08-16: a `37` white heading
`Average Turn Around Times to All BBSes`, then the **78**-column inset rule
(5 `─`, 15 `═`, 58 `─`) in `90` gray above and below the rows. Each row is the
planet name in `37` white in a 30-column field, then the figure.

```
Average Turn Around Times to All BBSes
─────═══════════════───────────────────────────────────────────────────────
Test Planet Two               No Data
─────═══════════════───────────────────────────────────────────────────────
```

**A board with no measured round trip reads `No Data`, in `31` red** — not a
zero and not a blank. The `planet    N.NN hours` form this file recorded is the
measured case only; both belong in the same column. (`31` red on black measures
2.7:1 on the VGA palette, under the 4.5:1 minimum — IB should not copy the
colour for its own "no data" state, and the word carries the meaning without it.)

**SDI Program** (A):
```
Total Funding: 0,000 Gold
Yearly Maintenance: 0 Gold
Funding / Region: 0,000 Gold
Current SDI Strength: 0%

Maximum productive spending this year is: 250,000 Gold.
Note: You should only fund the SDI in increments of 1000 Gold.
Add how much gold for funding? (0; 0)
```

Colors, re-captured 2026-08-16 and confirmed: labels white `37`, every figure
bright-yellow `1;33`, the Note line gray `1;30`. After funding it prints
`N Gold added.` then `Current SDI Strength: N%` again — and **those two lines
put their figures in `97` bright-white**, not the bright-yellow the report lines
above use. See the SDI section at the end of this file for the seventeen
captured funding levels and what they establish.

The same run funded an empty program with 250,000 gold at 738 regions and got
**5%**, which is `trunc(sqrt(250000 / (10 × 739)))` exactly — an independent
check of the curve on a realm three orders of magnitude smaller than the one it
was derived from.

The menu's own keys, from the capture: Gooie Kablooie Ops is **9**, not a
letter, and Terrorist Ops carries a gold cost in the menu's price column
(471,360 / 532,544 / 533,568 across the capture — it grows with something, and
three points do not say what).

**Special Operations** (8) is numbered **1-8** with no Help item, and prices its
first four entries: Bomb Food Market 10,000,000; Bomb Trading Market 25,000,000;
Bomb Trade Routes 25,000,000; Undermine Investments 75,000,000. Nuclear Assault,
Chemical Bombing, S3-Sabre and Send SpyGuy show no price on the menu — Send
SpyGuy quotes its own ("A SpyGuy will cost 10,810,500 gold per day") after the
target is named.

**Diplomacy List** (D): the `Planetary Treaties` chart — see the section at the
end of this file for its measurements, colors and the screen that edits it.

**Spy Database** (S): prompts `Select Planet to view players:` /
`Enter Planet Name or Number (? for list):`. `?` lists planets with an operator
location column (`## Planet Name   Location`); selecting one prints
`Our current relations with <planet>: <relation>` then any known players.

The planet prompt is shared by every interplanetary screen that asks for one,
and its colors are (from `cap/eots-ibbs-01.cap`): the label and the trailing
`: ` white (`0;40;37`), the parens bright black (`1;30`), the `?` bright white
(`1;37`), and what the caller types echoing bright yellow (`1;33`).
```
Enter Planet Name or Number (? for list): Starship Junkyard
```

### InterBBS Scores (IP Ops → View IPScores)

**IB substitutes its own name in the title, deliberately.** The original heads
this screen with its product name (`Barren Realms Elite: Top Planets by Score`),
so cloning the layout faithfully means putting OURS there — a title is branding,
not a mechanic, and the licence line is that IB may replicate the original's
shape but never present itself as the original. IB printed the original's name
here until 2026-08-23, which is the one place in the whole game it ever did.
Colours and geometry are unchanged.

Gray accent. A menu of eight ranking views + Quit; each opens a `»Planetary
Post«` table.

```
──────────[InterBBS Scores]───────────
(1) Top Planets by Score
(2) Top Planets by Net Worth
(3) Top Planets by Land
(4) Top Planets by Net Worth Density
(5) Top Players by Score
(6) Top Players by Net Worth
(7) Top Players by Land
(8) Top Players by Net Worth Density
(0) Quit
──────────────────────────────────────
```

Planet table: `(  n) <name>   <value>` (value right-aligned; header names the
metric, e.g. `Score`, `Net Worth`, `Land`, `Net Worth / Region`). Player tables
add a `Planet` column.

```
Barren Realms Elite: Top Planets by Score

                          ──═Planetary Post═──

      Name                               Score
────────────────────────────────────────────────────────────────────────
(  1) Planet A                         1321561
(  2) Planet B                         1254755
```

Colors and geometry, from `bre-01-color.cap`: the title line is `97`
bright-white `Barren Realms Elite` + `90` gray `: ` + `91` bright-red metric
name. The banner is the same `──═…═──` decoration as the news masthead — `31`
red `──`, `91` bright-red `═`, `97` bright-white title — indented 26. The
heading row is `97` bright-white and the rule under it is **72** columns of
plain `─` in `90` gray (this block drew 52 until 2026-08-16, and the banner as
`»…«`). Each row: `31` red `(` + `91` bright-red right-aligned number + `31`
`) ` + `37` white name + `97` bright-white value.

**The player tables put `Planet` LAST, after the metric** — captured in
`cap/20240527-134Pho_Lazarus_Public.cap` and
`cap/121125-666H4H_Camembert_Public.cap`. All four player views share one
geometry: `Name` at column 6, the metric **right-aligned to column 46** (the
same column the planet table's metric ends on, so the two shapes agree), then
`Planet` beginning at column 51, for a 57-column heading under the same
72-column rule.

```
      Name                                Land     Planet
      Name                               Score     Planet
      Name                           Net Worth     Planet
      Name                  Net Worth / Region     Planet
```

**IB draws this shape** as of 2026-08-26; it had the two reversed — `Planet`
second at column 33 and the metric last, a 66-column heading — until #196, and
its planet table ended on column 53 rather than 46. Not established: the
`Planet` column's width and whether it truncates. Neither capture has a data row
under that heading, so only the geometry above is proven; IB fits `Planet` to 21
columns, which ends it on the rule.

**The rule is WIDER than the table on purpose.** 72 columns of `─` over a
46-column planet table and a 57-column player table. It is not misalignment and
must not be "corrected".

### Attack Menu (InterBBS, local attacks OFF)

With local player-vs-player attacks disabled, the Attack Menu collapses to just
the pirate/alliance options (contrast the full Attack Menu near the top of this
file, which lists Regular/Nuclear/Chemical/Biological). Magenta accent.

```
─────[Attack Menu]─────
(P) Attack Pirates
(A) Alliance Strength
(V) Visit Bank
(?) Help
(0) Quit
───────────────────────
```

**Attack Pirates** (gray accent) — BRE's nine faction names (IB renames these):

```
[Attack Pirates]
(1) Humans
(2) Barbarians
(3) Solarians
(4) Sharks
(5) Mechanoids
(6) Rexxogans
(7) Xandorians
(8) Monitorians
(9) Spacians
(0) Quit
```

**Alliance Strength** (A): a `Name / Troopers / Tanks / Agents` table ending in a
`Total Forces  NONE  NONE  NONE` line when you have no allies.

### End of Turn Statistics

Heading `97` bright-white, then `34` blue **inset** rules — 75 columns, 5 `─`,
15 `═`, 55 `─`, the same rule that opens the income lines. Opens with a
support-flavor line, then population change and food spoilage (numbers `96`
bright-cyan), then `Do you wish to continue? (Y/n)`.

```
End of Turn Statistics
─────═══════════════───────────────────────────────────────────────────────
Your people have great faith in you as an excellent ruler!

Your dominion gained 380 million people.
57 units of food spoiled.
─────═══════════════───────────────────────────────────────────────────────
Do you wish to continue? (Y/n) Yes
```

A turn with no migration still prints the population line, as `Your dominion
gained 0 million people.` (twice in `cap/kd3-01.cap`, both on riot turns).
`process_end_of_turn` (BRE.OVR 0x00ce97) references only two strings for it,
`Your dominion gained ` and `Your dominion lost `, so zero has no wording of its
own. **IB printed nothing at all on a zero turn** until 2026-08-18 — most often
hit by a realm with an empty granary, whose growth is forced to zero.

This block drew both rules as plain 75-column runs until 2026-08-16; the `═`
accent is in `bre-01-color.cap` and `cap/eots-ibbs-01.cap` alike. **IB draws
both rules** as of 2026-08-16; it drew a bare heading and no rules before. Its
heading keeps IB's own bright-cyan `End of Turn Statistics:` where BRE writes it
bright-white without the colon, and its body lines are indented two columns.

### Food Unlimited (Food Market)

Cyan accent, single row of options. Preceded by the market line (`available`,
`buying for`, `selling for` — all figures `96` bright-cyan).

```
We have 6,851,917 units of food available today.
We are buying for 9 and selling for 26.

────────────────────────[Food Unlimited]────────────────────────
(B) Buy Food    (S) Sell Food   (V) Visit Bank  (0) Quit
────────────────────────────────────────────────────────────────
You have 3,229,328 gold and 5321 units of food.
```

Then the per-turn feed prompts: `Your People Need N units of food` /
`How much will you give? (N; N)` and `Your Armed Forces Require N units of food`.

### Gold maintenance sequence (captured live 2026-07-29)

Runs at the start of a turn with Auto-Pay Maintenance off, in this exact order.
Each figure is bright cyan against plain white body text, with the `(…; …)` pair
bright blue / bright cyan / dark cyan as elsewhere.

```
Your Armed Forces Require 682 gold.
How much will you give? (682; 682)

267,340 gold is required to maintain your regions.
How much will you give? (267,340; 267,340)

50,386 gold is requested to boost popular support.
How much will you give? (50,386; 75,579)

The Queen Royale requires 52,415 gold for Taxes.
How much will you give? (52,415; 780,295)
```

Two things to note in the prompt pairs:

- Forces and regions pass `(required; required)` — the maximum **is** the amount
  owed, so a solvent baron cannot overpay them.
- The crown tax passes `(required; gold on hand)` — you may hand the Queen far
  more than she asks. The support boost is similar but capped at 1.5x requested.
- The low number is a **suggestion, not a floor**: typing `0` at any of these is
  accepted, and was confirmed live at the crown-tax prompt.

Underpaying any required item raises, after the sequence:

```
Your actions may lead to DISASTEROUS results.
Would you like to reconsider? (Y/n)
```

`DISASTEROUS` is the original's own spelling and capitalisation, recorded here
for fidelity reference; IB uses its own wording (see the clean-room note).
Answering yes restarts the whole sequence from the bank prompt.

With Auto-Pay Maintenance **on** and enough gold on hand, the entire sequence
above collapses to two lines in the post-status block, with no prompts at all:

```
5,707,154 Gold paid.
29,164 units of Food consumed.
```

That single total is the best arithmetic probe in the game — see the Auto-Pay
section of the `bre-gather` skill for how to decompose it.


### System Menu (InterBBS grid)

Blue accent, three columns. In InterBBS mode it carries two extra rows —
`(G) Game Setup` and `(I) InterBBS Scores` — beyond the base set.

```
───────────────────────────────[System Menu]───────────────────────────────
(#) Abdicate             (M) Messages             (W) Write Macros
(A) Visit Advisors       (P) Preferences          (1) Set Industries
(D) Diplomacy            (R) Set Tax Rate         (2) Show Instructions
(E) Empire Status        (S) See Scores           (3) Specialize Industry
(F) Food Market          (T) Trading              (4) Spy Database
(G) Game Setup           (V) Visit Bank           (0) Quit
(I) InterBBS Scores
───────────────────────────────────────────────────────────────────────────
```

- **Set Tax Rate** (R): `New Tax Rate [0-100, Current Tax Rate = 20]?`
- **Game Setup** (G): the read-only ruleset dump. **Re-captured 2026-08-16, and
  it is bracketed above and below by a 78-column inset rule (5 `─`, 15 `═`,
  58 `─`) in `90` gray**, which this block omitted entirely. Labels are `37`
  white and values `97` bright-white — except the board count in the InterBBS
  sentence and the three `Gooie Kablooies` / `Bombing Ops` / `Missile Ops`
  toggles on the last line, which are `93` bright-yellow.

```
Game Started:        7/8/2026
Turns per day:       10
Protection Turns:    120
Daily Land Creation: 3000
Planetary Tax Rate:  10.0%
Maximum Players:     25
Bank Interest Rate:  10.0%
Investment Rate:     8.0%
Maintenance Costs:   Medium          Region Cost Change:  Medium
Trade Deal Costs:    Medium          Attack Damage:       Medium
Attack Rewards:      Medium
Military purchasing: Enabled
This game is setup in InterBBS mode with 4 boards in the game.
Attack Costs:        Medium          Terrorism Costs:       Medium
Maximum Individual Attacks Per Day: 2
Maximum Group Attacks Per Day:      2
Maximum Terrorist Ops Per Day:      15
Maximum Bombing Operations Per Day: 5
Days before "lost" forces returned: 1
Gooie Kablooies: Enabled     Bombing Ops: Enabled     Missile Ops: Enabled
```

- **Specialize Industry** (3): a blurb, then a red-accent `[Specialization]` menu
  (Troopers/Jets/Turrets/Bombers/Tanks/Carriers/Quit); declining prints
  `Your industries have not been specialized.`
- **Trading** (T): a small red-accent menu — `(1) Trading` `(2) Trading Market`
  `(V) Visit Bank` `(0) Quit`.

---

## Configuration Editor (`BRE RESET`, captured live 2026-08-11)

Reached by `BRE RESET` from the DOS prompt: `Are you sure you wish to reset the
Game? (Y/n)` → the editor → `[ESC]` to save → `Would you like this to be a
league-wide reset? (Y/n)` → `Your BRE game has been reset...Thank you!`

**That league-wide prompt only appears on a board BRE recognises as a league
member** (its `bbs.cfg` FTN address matching an entry in `BRNODES.DAT`). A
standalone install never asks it, which makes it the quickest confirmation that
an InterBBS test board is configured correctly.

Two pages, moved through with **PG-DN / PG-UP**; `[ESC]` saves and quits.
**Re-captured with colour 2026-08-16** — the colour-by-type table below had been
presented as fact with no capture behind it, and it holds. Fields are
**colour-coded by type, and every value is the BRIGHT form of its label's
colour** — labels `3x`, values `9x`:

| field type | label | value | seen on |
|---|---|---|---|
| dates | `32` green | `92` bright-green | Game Start Date, Game Join Date |
| numbers | `36` cyan | `96` bright-cyan | Turns Per Day, Turns of Protection, rates, caps |
| presets | `35` magenta | `95` bright-magenta | Buy Military, Maintenance/Trade Deal/Region Costs, Attack Damage, Sabre Handling |

Three additions the earlier entry did not have:

- **The highlighted field is `97` bright-white on BOTH label and value**, which
  is how the cursor is shown — the type colour is replaced, not brightened. The
  `92`/`97` alternative the old table gave for a date value was this highlight
  being read as a date colour.
- **`Buy Military` is a preset field**, not a number, and takes the magenta pair
  even though its values are `Yes`/`No`.
- The footer's `*` and the three bracketed key names (`PG-DN`, `PG-UP`, `ESC`)
  are `93` bright-yellow inside `37` white brackets.

The `: ` separator and the trailing pad are `37` white. A `*` prefix marks an
InterBBS-only setting; the footer states it. The title rule is **72 columns** —
5 `─`, 2 `═`, the 40-column title, 6 `═`, 19 `─` — and it is drawn in mixed
colours: the outer `─` runs `90` gray, the inner `─` and `═` `37` white, with
the title itself split across `91` bright-red and `31` red and `97` runs. That
last is not a scheme worth copying; record it and move on.

```
─────══Barren Realms Elite Configuration Editor══════───────────────────
Game Start Date               : 08/11/2026  14:48:46
Game Join Date                : 01/01/2999  00:00:00
Turns Per Day                 : 8
Turns of Protection           : 20
Initial Market Land           : 0
Land Created / Day            : 1000
Interest Rate                 : 50
Standard Investment Rate      : 35
Steady Investment Rate        : Disabled
Tax Rate                      : 50
Max Purchasable Regions       : 500
Max Players Per BBS           : 25
Buy Military                  : Yes
Maintenance Costs             : Medium
Trade Deal Costs              : Medium
Region Costs                  : Medium
Attack Damage                 : Medium
* = InterBBS Setting Only   [PG-DN] For More Options   [ESC] to Save & Quit
```

Page two — everything starred is InterBBS-only:

```
Attack Rewards                : Medium
Sabre Handling                : User Select/Original Setting
*Attack Costs                 : Medium
*Terrorist Costs              : Medium
*Individual Attacks / Day     : 1
*Group Attacks / Day          : 1
*Terrorist Attacks / Day      : 10
*Bombings / Day               : 5
*Days for "Lost Attacks"      : 7
*Gooie Kablooies              : Enabled
*Bombing Operations           : Enabled
*Missile Operations           : Enabled
*Local Attacks                : Enabled
*Local Attack Scoring         : Disabled
*Dupe Checking                : Enabled
* = InterBBS Setting Only   [PG-UP] For More Options   [ESC] to Save & Quit
```

### Editing a field — the help screen IS the edit screen

Enter on a field replaces the list with a full-screen page (captured 2026-08-16):

```
                    ──═Configuration Help Information═──
─────═══════════════──────────────────────────────────────────────────────────
  ──═GamePlay: Maintenance Costs═──
  The GamePlay options may be set to the following values: High, Medium
  Low, or None.  [H,M,L,N]  ...
─────═══════════════──────────────────────────────────────────────────────────

Possible Settings: High
                   Medium
                   Low
                   None
New Setting:
```

The outer banner is the `──═…═──` decoration in `34` blue / `94` bright-blue
around a `97` bright-white title; the field's own banner repeats it in `32`
green / `92` bright-green. Both rules are the **78**-column inset (5 `─`,
15 `═`, 58 `─`) in `90` gray. Body text `37` white with the setting's key words
`92` bright-green.

**A preset field commits on ONE key** — `H` sets High and returns to the list
with no Enter. A numeric field takes digits and Enter. The old note calling the
editor read-only under `dosemu -t` is wrong and has been removed: every field
edited cleanly this run.

## Trading (System Menu → T) and the Trading Market

The Trading submenu's accent is **red**: `31` red parens and rule, `91`
bright-red key letter, `37` white label.

```
─────[Trading]──────
(1) Trading
(2) Trading Market
(V) Visit Bank
(0) Quit
────────────────────
Choice> Quit
```

### Send Trade Deal — the local flow (captured in `cap/kd3-01.cap`)

This is `create_trade_offer` (BRE.OVR 0x260cd), the LOCAL deal, reached from
this menu. **It was filed under the InterPlanetary Operations menu's (3) for
months** — the two items are different routines, and that misattribution is what
wired IB's interplanetary item to the local mechanic (#195). Every string below
is this routine's: the per-day rate, the minimum span and the arrival-turn
sentence have no counterpart in the interplanetary one.

After the goods are chosen it states the transport requirement, the per-day
cost and the two-day floor, prompts for a span, and then — the part IB had no
equivalent of — tells the sender which turn of the recipient's day it lands on:

```
Trade Deal requires 20 Carriers.
Send Trade Deal? (Y/n) Yes

This trade deal will cost 120,000 gold per day to send.
Trade deals must be sent for a minimum of 2 days.

How many days would you like to send this deal for? (2; 10) 2

Trade Deal sent to <realm>
It will arrive at earliest on Turn #<n> of that empire today.
```

The turn number is `0e` bright-yellow against `07` grey body text. The figure
is the sender's own turn of the day; the mechanic behind it is in
`docs/mechanics-reference.md`. **Deliberate divergence:** IB says the same thing
in its own words rather than reproducing BRE's sentence.

The market itself lists the eight tradeable goods — note keys 1-5 and 7-9, with
**no key 6**. Key 6 is **Gold** (`empire_trade_good_pointer`, BRE.OVR 0x051133,
record `+0x66`, and the good-name table at `DS:0xb11` agrees). It is missing from
the MARKET because gold cannot be sold for gold, not because it is untradeable —
it rides a trade deal, local or interplanetary, like anything else. This file
said "regions are not tradeable" until #195; regions have no key here at all.

```
Trading Market
Key Name               Your Prices       Owned      For Sale   Total For Sale
─────═══════════════──────────────────────────────────────────────────────────
(1) Trooper                    500          27            73               73
(2) Jet                          0           0             0                0
(3) Turret                       0           0             0                0
(4) Bomber                       0           0             0                0
(5) *Food                        0        1840             0                0
(7) Agent                        0           0             0                0
(8) Tank                         0           0             0                0
(9) Carrier                      0           0             0                0
─────═══════════════──────────────────────────────────────────────────────────
Your Choice?
```

**The market table does NOT take the Trading menu's accent** (`cap/eots-ibbs-01.cap`):
its 78-column inset rule is `91` bright-red, the row parentheses are `33` yellow
around a `93` bright-yellow key, the good's name is `37` white, and the four
figure columns each carry their own color — Your Prices `97` bright-white, Owned
`96` bright-cyan, For Sale and Total For Sale `93` bright-yellow. The heading row
is plain `37`.

Picking a good gives `[C] Change your setup, or [B] Buy from Market:`, then
`Enter new amount of <Good> for sale:(MAX=n)` and `Set new <Good> price: (0)`.
Listing moves the goods out of `Owned` into `For Sale` immediately.

**The market exits on ESC — not `0`, not Enter.** Its screen lists no Quit key
and the `Your Choice?` prompt silently redraws on both, so a driver that presses
`0` until it recognises a screen will loop forever. `tmux send-keys -H 1b` leaves
it, and pops the Trading submenu with it, landing on the System Menu.

**The `*` marks a good that is not a military unit.** It is part of the stored
name, not something the draw routine adds: BRE's shared goods table lives in
**`BRE.EXE` at 0x157b7** (fixed-width slots, Pascal ShortStrings) and reads

```
Trooper  Jet  Turret  Bomber  *Food  *Gold  Agent  Tank  Carrier
```

so the seven military units are bare and only **Food and Gold** carry the
marker. Gold is not one of the market's eight rows, which is why the capture
above shows exactly one `*`. Searching `BRE.OVR` for it finds nothing — the
table is in the executable, not the overlay, which is the trap here.

**IB status (2026-08-13):** matched, with one recorded divergence.

- **The exit key.** IB leaves on ESC as BRE does, and *also* on `0` and Enter,
  which BRE ignores. A screen that lists no Quit key and refuses the key every
  other menu quits on is a trap for a player, not a fidelity detail worth
  keeping; `0` quits everywhere else in IB.

The column geometry above is pinned by `TestMarketTableMatchesTheCapturedGeometry`
as golden literals (edges 33/45/59/76, and the 5/15/58 inset rule), so a change
to the format must produce new evidence rather than quietly following the code.

## Covert Operations (System Menu → C)

**Captured live 2026-08-16, and RE-captured 2026-08-17 with the log kept —
`cap/covert-menu-20260817.cap`.** This section said "colors NOT captured" before
that. The second pass exists because the first one's log was not retained; every
figure below now has a file behind it.

The menu's accent is **green**, and it prices every operation on the menu
itself. Colours, byte for byte: border `32` green, brackets `92` bright-green,
title `97` bright-white, item parens `32`, item keys `92`, labels and prices
`37` white. IB chose bright-green for this menu before any capture existed, and
the capture bears that out.

**The box is 32 columns** — `6 + [Covert Operations] + 7`, closing rule 32,
obeying the rule that a closing rule equals the box width.

**IB draws 34, and that is the faithful answer rather than a divergence.** BRE
sizes the box to its widest row; IB's same row is two columns wider, because the
menu engine indents every item two columns and IB comma-groups figures BRE
prints bare (both long-standing). Forcing 32 clips every priced row, which is a
worse likeness than a box two columns wider. IB drew 62 until this was captured.

IB additionally prints a `Key Item Price` header row that BRE has no equivalent
for — **UNVERIFIED** whether BRE omits it everywhere or only here.

**`cap/kd3-01.cap` holds this screen too**, from a real board with 11,652 agents,
and independently gives the same 32-column box and the same nine prices. It was
missed on two earlier passes: a search keyed on the status line's wording did not
match the real spacing, and the follow-up found only the help-topic index, which
lists all nine operation names and reads like the menu. **Grep an operation name
plus a price, not the status line, and do not stop at the topic index.**

**The same capture shows what BRE says when an operation is ORDERED**, which no
other screen records:

```
Demoralize Forces
Choose a Target [A-Y,?=List RETURN to Abort] Endor
Covert Agent Sent out
──────[Covert Operations]───────
```

The menu redraws immediately and no outcome is shown, because an effect operation
is queued and resolved at daily maintenance rather than on the spot. That is the
observed counterpart to the queue mechanism read out of the binary.

**The menu is hidden until the realm holds covert agents.** Observed directly: at
0 agents the System Menu has no `(C)` item and its right column starts at
`(1) Set Industries`; buying 20 agents makes `(C) Covert Operations` appear.

**The advertised price for Spy on Relations is a lie.** The menu says 100,000;
`report_spy_result` subtracts the Send Spy fee of 5,000 for both info ops. The
screen above is what BRE shows, not what it charges.

```
──────[Covert Operations]───────
(1) Send Spy               5000
(2) Stir Revolts         25,000
(3) Set Up               50,000
(4) Support Dissensions  80,000
(5) Demoralize Forces    80,000
(6) Spy on Relations    100,000
(7) Bomb Enemy Targets  100,000
(8) Bribery             200,000
(9) Expose Enemy Ops    600,000
(V) Visit Bank
(?) Help
(0) Quit
────────────────────────────────
You have 4,732,202 gold and 100 agents.
Choice> Quit
```

Colours: the rule and the parentheses `32` green, the title brackets and the
hotkeys `92` bright-green, the title `97` bright-white, and both the item label
**and its price** `37` white. The footer follows the menu-footer convention —
body `37` white, the two figures `97` bright-white. **32 columns** (6 `─` +
`[Covert Operations]` + 7 `─`), closing rule 32.

IB drew this menu bright-green before the capture existed, so its choice turns
out to match. The price column is IB's to add or not; BRE shows it.

**`Limit one try per turn!` did not appear** on either capture of this screen,
though the string sits beside the footer's in the same overlay unit. Not
explained — record it as a string that exists rather than as a line the menu
draws.

**The fees are flat constants, not scaled by the sysop's cost levels.** Two
independent lines agree. Live: a game with Maintenance / Trade Deal / Region /
Attack / Terrorist Costs all set to **High** draws the identical nine figures as
one with them at Medium/None, and a Send Spy moved gold by exactly 5,000. Static:
the nine dwords the menu reads from `DS:0x63E` are **initialized DGROUP data in
`BRE.EXE` at file offset `0x14EDE`**, not a runtime table — nothing anywhere in
either binary writes them, the charge is a bare 32-bit `sub`/`sbb` with no
multiply, and the covert overlay unit never loads the config record at all. See
GitHub issue #143.

### What the strings and the disassembly give

The item labels and their order come from the overlay's string table, where Turbo
Pascal stores them consecutively in declaration order, which is render order
(`BRE.OVR 0x1731a` onward):

```
Send Spy · Stir Revolts · Set Up · Support Dissensions · Demoralize Forces
Spy on Relations · Bomb Enemy Targets · Bribery · Expose Enemy Ops · Visit Bank
```

The status line under it is assembled from `'You have '` + gold + `' gold and '`
+ agents + `' agents.'`, and `'Limit one try per turn!'` sits just below in the
same unit — the one-effect-op-per-turn cap.

The interplanetary **Special Operations** menu has its own table at
`BRE.OVR 0x2981b`, in this order, with a zero-length entry between Nuclear
Assault and Chemical Bombing that a naive ShortString walk will mistake for the
end of the table:

```
Bomb Food Market · Bomb Trading Market · Bomb Trade Routes · Undermine
Investments · Nuclear Assault · <len 0> · Chemical Bombing · S3-Sabre · Send SpyGuy
```

**All eight belong to the interplanetary menu, and none to the local one.** The
table is read by `run_bombing_operations_menu` (`BRE.OVR 0x029EA9`) alone, whose
only caller is `run_interbbs_menu`. The 500-bomber gate string follows
immediately, and the gate itself (`+0x1015`) and the 500 Bombers each launch
consumes (`+0x1146`, `+0x1233`, `+0x1689`) are all inside that same procedure —
the local Covert menu tests no bomber count anywhere. The local **Bomb Enemy
Targets** is ONE flat op that rolls `Random(6)+1` over six holdings in the
resolver; see `docs/mechanics-reference.md`. IB read this table as "the local
menu takes the first seven" and built a lettered submenu on it, which was wrong
and has been removed.

**From the draw routine** (`run_covert_operations_menu`, `BRE.OVR 0x17469`),
read directly rather than inferred:

- **The hotkeys are binary-verified.** Each item block opens `mov al,<char>` with
  `0x31`–`0x39` then `0x56` — `1`-`9` then `V`, in the same order as the strings.
- **The per-op costs sit in a DS table, indexed by keycode.** Each item pushes a
  different consecutive dword from `DS:0x63e`, stepping 4 bytes per item
  (`[0x63e]`, `[0x642]`, …). This was read as a *runtime* table until 2026-08-16,
  on the strength of a byte search finding nothing — but the search had been run
  over the overlay. The dwords are initialized data in `BRE.EXE` (above), which
  is why they never change.
- **The entry gate is a signed 32-bit `agents > 0`** on the *held* agent count
  (`+0x26f`), tested high word then low. Agents escrowed on the Trading Market
  live at `+0x229` and are a separate field, so listing agents for sale can close
  the menu — IB matches, since listing moves them out of `Empire.Agents`. The
  per-turn covert step carries an extra gate on the byte at `DS:0x6d52`, which is
  the preference; the System-menu item does not, matching what the captures show.

**Why the disassembly could not have settled the colours** — worth keeping,
because the same reasoning applies to every screen whose colours are still
missing. Neither binary contains a single literal ANSI escape, so BRE holds
colour as a Turbo Pascal attribute and builds the escape at write time; the draw
routine passes only key, label and cost, and neither it nor
`enter_covert_operations_menu` sets an attribute. The colour comes from the
shared output helpers (`0dc9:0608` formats the line, `0735:0000` writes it),
which every screen uses. **A `script`-wrapped live capture is the only cheap way
to get a screen's colours**, which is how this one was finally closed.

Corroboration worth keeping: a 2012 public-board capture renders the Help "List
of Topics" divider as CP437 `ÄÄÄÄÄÍÍÍÍÍÍÍÍÍÍÍÍÍÍÍÄÄÄ…` — 5 `─` then 15 `═` — the
same inset rule recorded for the Relations table and the Trading Market, from an
entirely separate board and year. That the ratio recurs across screens and
releases is why `insetRule` is the shared helper rather than a per-screen
constant.

## Clean-room note

BRE is proprietary (John Dailey Software; design by Mehul Patel). This file
records *observed* screen behavior to guide an independent reimplementation; it
is not a copy of BRE's source or assets. Distinctive coined strings/names (e.g.
pirate faction names) are recorded here for fidelity analysis; whether IB uses
one or coins its own is Andy's call case by case. The Gooie Kablooie and the
S3-Sabre keep the original's names (#218); the pirate factions carry IB's own.

## Full Defense Alliance — Diplomacy, Alliance Strength, battle (captured live 2026-07-27)

### Diplomacy Menu (opens every turn before income, and via System Menu -> D)
Items: (1) Tariff Trade Agreement, (2) Protective Trade, (3) Free Trade Agreement,
(4) Terrorist Prevention, (5) Intelligence Alliance, (6) Technology Agreement,
(7) Full Defense Alliance, (8) Declaration Of War, (9) View Treaties, (?) Help,
(0) Quit.

### Proposing (7) Full Defense Alliance
Shows the treaty blurb then `(A-Y,Z=All,?=List) Send to:` -> pick target letter
-> `Would you like to attach a message? (y/N)`. `?` prints the shared realm
roster and re-prompts (captured at a `Send to:` prompt in `cap/bre-3-color.cap`):
```
-*Barren Realms Elite*-
Id  Empire Name                          Territory     Score    Net Worth
─────═══════════════───────────────────────────────────────────────────────
(A)+Dynoland                                  8391    687985     1883573
(B) Bran The Warrior                          2135      6390       26687
─────═══════════════───────────────────────────────────────────────────────
(A-Y,Z=All,?=List) Send to:
```
The `+` marks a realm in the list; figures are ungrouped. This is the same table
the Regular Attack picker lists, so one routine serves both.

**The prompt is the MULTI-select one** (see "Send Message: the recipient picker
is MULTI-select" below for how it behaves), and the Diplomacy menu is its other
caller: the selection routine at `BRE.OVR` 0x1b65e is reached from exactly three
sites, one on the message path and two inside `run_diplomacy_menu` (0x1c800
+0x08e7 and +0x0a79) — a treaty proposal and the Declaration Of War. So both
address several realms in one action, and the capture above sits at a prompt that
is waiting for RETURN, not for one letter. The prompt text is the same for both
callers: its printer (0x1b0a3) has the selection routine as its only caller, so
there is no per-item wording.

**UNVERIFIED — Diplomacy's roster flag.** Diplomacy passes 0 for the roster flag, which lists
`-*Relations*-` rather than the score table — yet the capture above is filed
under a treaty proposal and shows the score table. One of the two is mislabelled
and it has not been re-captured; the capture's own note only says it was taken
"at a `Send to:` prompt", which the message path also has. IB follows the flag
and lists Relations at a Diplomacy prompt.

Recipient sees at next login:
`<Name> proposes a Full Defense Alliance.` / `Regions: N; Net Worth: N; Score: N;
Accept? (Y/n)`. (9) View Treaties -> `-*Relations*-` table: each empire letter +
name + Relations ("None" / "Full Defense Alliance").

### Attack Menu -> (A) Alliance Strength
```
Your allies will send the following to aid you in defense:
Name                   Troopers     Tanks    Agents
Carbon                      29      3271      NONE
Total Forces                29      3271      NONE
```
(= 30% of the ally's troopers/tanks/agents; no jets/turrets/bombers/carriers.)

### Measurements and colors for those three (captured live 2026-08-01, league game)

**Incoming proposal**, in the "since your last play" block — **immediately after
the `Since your last play, this has happened:` header and BEFORE the numbered
recap entries**, not after them. Re-captured 2026-08-16 and every colour below
held; only the position was wrong. It carries NO rule and NO timestamp: unlike a
recap entry it is a prompt, not a log line. Two lines, unindented, the second
running straight into the prompt; the figures are NOT comma-grouped:
```
<Name> proposes a <Treaty>.
Regions: 19445; Net Worth: 146868315; Score: 2442093; Accept? (Y/n) Yes
```
Colors: realm name and treaty type bright cyan (`1;36`), every connecting word
white (`0;40;37`), the three figures bright yellow (`1;33`). The y/n hint is
BRE's usual cyan parens (`36`) around bright-cyan letters, and the answer echoes
as the whole word (`Yes`/`No`) in bright white (`1;37`).

BRE writes `proposes a <Treaty>` with no article agreement, so an
`Intelligence Alliance` reads `proposes a Intelligence Alliance`. Recorded for
accuracy; IB fixes the article, which is a divergence not worth reversing.

**View Treaties -> `-*Relations*-`.** Re-captured 2026-08-16. Lists every living
realm **except the caller's own**, "None" included — the roster doubles as the
empire-letter key, minus your own letter. (This file said it listed every living
realm; a three-realm capture from realm `A` lists only `[B]` and `[C]`.) Title
`-*`/`*-` blue (`0;40;34`) around a bright-white word; heading bright white; a
**75-column** inset rule (5 `─`, 15 `═`, 55 `─`) in blue above and below the
rows. A row is `[X]  ` then the name in a **40-column** field: brackets blue,
letter bright white, name bright cyan (`1;36`), relation bright blue (`1;34`).
```
Id   Empire Name                             Relations
─────═══════════════───────────────────────────────────────────────────────
[A]  Endor                                   None
[F]  Opium                                   Full Defense Alliance
```

**Alliance Strength.** Re-captured 2026-08-16 with a real ally row — until then
only the empty `Total Forces … NONE` case was on disk — and every figure below
held. White headings, a **51-column** inset rule (5 `─`, 10 `═`, 36 `─`) in red
above the rows and above the total, ally names bright white, figures bright
yellow and ungrouped, a zero shown as `NONE`. Data columns are name 21, then
9 / 10 / 10; BRE's *heading* row is one column wider on Troopers (10), so the
headings sit one column right of the figures.

```
Your allies will send the following to aid you in defense:
Name                   Troopers     Tanks    Agents
─────══════════────────────────────────────────────
Carbon                      29      3271      NONE
─────══════════────────────────────────────────────
Total Forces                29      3271      NONE
```

**IB deviates on readability (deliberate, Andy's call).** Two changes, both to
make a dense line scan faster:

- **Number grouping.** BRE prints the figures on both of these screens ungrouped
  — `19445`, `963016`. IB comma-groups them; the Alliance Strength columns are
  widened to 12 and its rule follows, and the heading drops BRE's off-by-one.
  BRE is not consistent about this itself — its Queen Royale refund line groups.
- **Field separator.** The offer's stats line uses `│` where BRE uses `; `, so
  the three figures read as separate fields rather than as prose. It costs the
  same width as the semicolon (one glyph per gap), which bracketing the values
  would not — `[…]` reaches 78 columns with a 9-digit net worth and wraps at 10.
- **Online mark (#123).** Relations shows the same `(O)` as See Scores, hugging
  the left of the realm name in the two spaces BRE leaves after `[X]`, so the
  40-column name field and the relation at column 45 are untouched. A headed
  column of its own would have moved both, and this is one of the
  closest-matched screens; the cost is that the mark carries no heading here.

Everything else on the two screens is the original's.

**Do not raise the y/n prompt mismatch.** Two known differences in the shared
`AskYesNo`: BRE echoes the whole word (`Yes`/`No`) in bright white where IB
echoes `y`/`n`, and BRE's hint parens are plain cyan (`36`) where IB uses bright
blue. Both are recorded here so the capture stays accurate. Andy has parked
them — leave them alone unless he brings them up.

### Regular Attack flow + battle report (attacker's view)
Attack Menu -> (R) -> `Choose a Target [A-Y,?=List RETURN to Abort]` -> letter ->
`You have N Troopers, N usable Jets, N Tanks, and N Bombers` ->
`Send how many Troopers? (def; max)` / `How many Jets?` / `How many Tanks?` /
`How many Bombers?` (`>`=max; all-zero aborts). A losing attack against an allied
defender:
```
The empire's allies send 29 Troopers and 3271 Tanks.
And the battle begins.......
Your forces have returned..Exhausted from battle...
You lost 0 Troopers, 0 Jets, 2206 Tanks, and 0 Bombers.
You destroyed 14 Troopers, 0 Turrets, 1948 Tanks, and 0 Jets.
You lost the battle!
```

## "Since your last play" event log (captured live 2026-07-31, league game)

The recap shown at the start of a turn. Every entry is wrapped in its own rule
carrying a **1-based counter and a timestamp**; entries are separated by a blank
line.

```
Since your last play, this has happened:

─────(1)────────────────────────────────07/31/2026  07:43:11────────
Opium accepted your Full Defense Alliance proposal.

─────(2)────────────────────────────────07/31/2026  09:07:07────────
DTF accepted your Full Defense Alliance proposal.
```

Measured: 5 `─`, `(N)`, **40** `─`, the 20-column stamp, 8 `─` — **76 columns**,
with the stamp starting at column 48. This block showed 48 fill columns, which
would run the line to 84 and wrap; 13 rules across two captures all measure 76.
(IB's `eventRuleWidth`/`eventStampColumn` were already 76/48 and are correct.)

Colors:

- Header line `Since your last play, this has happened:` — plain white `37`.
- The rule, both sides of the counter and the date/time inside it — bright-black
  `1;30` (dim gray). The `(` `)` around the counter are yellow `0;40;33`; the
  counter digit itself is bright-yellow `1;33`.
- Body: on a **treaty** entry the realm name and treaty type are bright-cyan
  `1;36`; the connecting words plain white `0;40;37`.
- **The body colors are per entry TYPE, not per kind of thing.** A trade entry
  in `cap/eots-ibbs-01.cap` renders the buying realm `1;33` bright-yellow, the
  good `1;31` bright-red and the quantity `1;37` — the same realm name that a
  treaty entry two rules above draws `1;36`. Do not generalise one entry's
  palette to the rest of the recap.

Treaty replies read `<Realm> accepted your <Treaty> proposal.` and
`<Realm> rejected your <Treaty> proposal.` — **rejected**, not "declined".

### Message box inside the recap (same capture)

Mail is read as part of the recap, straight after the numbered event rules — no
count line, no "read them now?" gate. An empty inbox prints `You have no
messages.` in that spot instead.

```
Since your last play, this has happened:

┌──────────────────────────────────────────────────07/30/2026  00:24:05─────
│ Message From: <realm>
│ Message To  : ABCE
├─────════════───────────────────────────
│ Savings full. Investments start coming in 8/1.
[R] Reply, [D] Delete, [I] Ignore, or [Q] Quit> Delete
```

Measurements: the top rule is **76** columns (`┌` + 50 `─` + the 20-column stamp
+ 5 `─`); the header separator is a short **41** (`├` + 5 `─` + 8 `═` + 27 `─`)
and does NOT span the box. `Message To` carries the recipient empire letters.

Colors: frame cyan `36`; timestamp bright-white `1;37`; the `Message From:` /
`Message To  :` labels white `37`; sender bright-cyan `1;36`; recipient letters
bright-green `1;32`; body white, quoted lines bright-blue `1;34`; the action keys
bright-cyan `1;36` inside blue `34` brackets.

`Message To` is a run of letters, not a name — the reader loops `A`..`Y` over the
message's own recipient table and prints every letter that is in it, so a message
sent to several realms shows them all, in letter order, and every copy carries
the same run.

## Send Message: the recipient picker is MULTI-select (System Menu → M)

The `(A-Y,Z=All,?=List) Send to:` prompt is **not** answered with one keypress.
It is a toggling list, closed by RETURN. Read from a live capture and confirmed
against the selection routine at `BRE.OVR` 0x1b65e and the per-letter toggle it
calls at 0x1b575:

```
(A-Y,Z=All,?=List) Send to: EFHIJKMNOP

    You have 20 lines for your message.  /S=save /A=abort /C=clear
    [---+----|----+----|----+----|----+----|----+----|----+----|----+----]
 1> 
```

- **A letter toggles that realm.** Selecting echoes the letter in bright cyan
  (`1;36`) and returns the colour to `37`; pressing it again writes `BS`/space/`BS`
  to rub it out and re-draws the letters that are left (0x1034 adds and echoes,
  0x10d2 removes and erases).
- **`Z` toggles every letter A..Y in turn** — the same toggle applied to the whole
  range (0x1427). It does not end the prompt, and on a list that already has
  selections it *inverts* them.
- **`?` prints the roster and re-draws the prompt.** Letters already chosen stay
  chosen but are not re-echoed on the new line. The roster it prints depends on a
  flag the caller passes: Send Message passes 1 and gets the See Scores table
  (Id / Empire Name / Territory / Score / Net Worth); the Diplomacy menu passes 0
  and gets the `-*Relations*-` table instead.
- **RETURN closes the list**; with nothing selected that is a cancel and the flow
  returns to the menu.
- **Anything else is ignored with no echo** — including a letter with no living
  realm behind it and the caller's own letter (0x12b2 rejects self, 0x12c8
  rejects an empty slot).

**The letter is the realm's SLOT, not its row number.** BRE indexes its empire
array with the letter itself, so a dead realm or the caller's own leaves a **gap**
— which is why the `-*Relations*-` capture above runs `[A] [C] [D] [E] [G] …`.
The prompt therefore always names the full `A-Y` range whatever the realm count:
the two-realm capture in the Full Defense Alliance section shows the same
`(A-Y,Z=All,?=List)`.

After a message is saved BRE asks `Do you wish to send another message? (y/N)`
and loops straight back to the `Send to:` prompt.

**IB uses this picker at both of BRE's call sites**: Send Message, and every
Diplomacy action that names a realm — each treaty type and Declaration Of War
(see the Diplomacy section above). The roster flag is the `relations` field of
`pickOpts`. IB adds `*=All Allies`, which marks the realms it holds a treaty with
and leaves the list open, so letters can still be added or taken off before
RETURN.

## IP Messages: the recipient picker (InterPlanetary Ops -> Send Message -> Single Planet)

The interplanetary twin of the picker above, captured live in
`cap/eots-ibbs-01.cap` (a four-planet league, 2026-08) and read out of
`send_interbbs_message` (`BRE.OVR` 0x1f335). The flow is: the IP Messages box
(Single Planet / Select Planets / All Planets / Allied Planets / Planet
Coordinator / Quit), the shared `Enter Planet Name or Number (? for list):`
prompt, the "Our current relations with" line, and THEN
`(A-Y,Z=All,?=List) Send to:` over the barons on the chosen planet. The editor
comes after that, not straight after the planet.

The roster `?` draws is headed `-*Players at <planet>*-` in bright white on
blue-rules, over the same `Id   Empire Name   Territory   Score   Networth`
columns the local See Scores table uses, closed by the `[BRE vX]  <date>`
footer. It is drawn by the same routine the Spy Database and the attack target
list use (`show_player_list`, `BRE.OVR` 0x2224cf).

**The toggle is visible in the capture's BYTES.** At one prompt the echo after
`Send to: ` runs four letters in bright cyan (`1;36`, returning to `0;40;37`
after each), then four erase groups:

```
<L><L><L><L>  BS SP BS      -> the last letter taken off, nothing re-drawn
              BS SP BS      -> and the next
              BS SP BS BS <L>  -> one taken from the MIDDLE: an extra BS per
                                 letter after it, then the survivors re-drawn
              BS SP BS      -> the last one off
CR
```

and the next thing on screen is the "send another message?" question, NOT the
editor's banner. That settles three things a reading of the code leaves open:
the letters toggle, the erase-and-redraw is what a middle deselect looks like,
and **RETURN with nothing marked sends nothing** (the code simply never fills
its recipient table in that case, which is undefined rather than defined
behaviour; the capture defines it).

**`?` is not echoed.** The capture shows the key produce a colour change and a
CRLF, then the roster, then the prompt again. IB echoes the word "List" instead,
as it already does on the local picker.

**The letters have gaps and they persist.** The same planet's roster reads
`(A) (B) (D) (E)` on three separate days a week apart, and a later capture shows
`(A) (B) (C) (D)` once the C slot has been refilled by a different realm — so
the letter is the realm's own slot on its home planet, not its row number, and
BRE knows it because it holds that planet's whole 25-slot array.

**IB divergences here**, both recorded in `docs/mechanics-reference.md` under
"IP Messages": IB letters the rows of the received scores packet consecutively,
because `game.RemoteScore` carries no slot; and a planet IB has no scores packet
for skips the prompt and takes a planet-wide message rather than refusing.

**The planet prompt autocompletes in the original.** The capture shows two typed
characters erased (`BS SP BS` twice) and replaced with the full planet name. IB
does not do this — a separate request, noted on #193 and not built there.

### The message editor

20 lines, under a **68-column** ruler — note the last group is short, `----+----]`
rather than a seventh `|`, so the whole ruler including brackets is 70 columns.
Line prompts are ` 1> ` upward. `/` as the first key of a line opens
`/-Command?  [A,S,C]`; BRE erases that prompt (20 columns of `BS`/space/`BS`)
before printing the outcome, and prints an abort in red (`0;40;31`).

IB divergences here, both cosmetic and deliberate: IB shows no block cursor while
typing (BRE draws `█` then backspaces over it per character), and it prints the
`/`-command outcome in place rather than erasing the prompt first.
## Message editor (captured live 2026-08-14, standalone board)

Reached from the main menu's `(7) Send Messages` once a recipient is picked, and
from a reply in either message reader. All five entry points call one routine
(`compose_message`, BRE.OVR 0x0492ba), so the editor behaves the same
everywhere; only the line allowance differs, and the 3-line banner in the string
table belongs to the trade-deal note, not to this screen.

```
    You have 20 lines for your message.  /S=save /A=abort /C=clear
    [---+----|----+----|----+----|----+----|----+----|----+----|----+----]
 1> aaaaa bbbbb ccccc ddddd eeeee fffff ggggg hhhhh iiiii jjjjj kkkkk
 2> lllll mmmmm
```

Measurements: the ruler spans **68** columns between its brackets. Counting the
`[` itself as column 1, a `+` marks every fifth column and a `|` every tenth.
Banner and ruler are indented four columns, which puts the `[` directly over the
first text column of a `NN> ` prompt — so the ruler's own last `-` sits one
column past where the line stops taking text.

Colors, re-captured 2026-08-16 and confirmed: banner white `37`; ruler plain
cyan `36`; the line-number prompt bright green `92` with the typed text bright
white `97`. A line re-opened by backspacing off the start of the one below it is
prompted in bright **red** `91`. The `/`-command prompt draws its `[A,S,C]` keys
`96` bright-cyan inside `37` white brackets, and `ABORTED!` lands in `31` red.

### Wrapping at the margin

A line takes 68 characters. The 69th printable key wraps rather than extending
the line or being refused (`compose_message` compares the next column against
`0x45`). The wrap:

- Scans back from column 68 for a space, giving up below column 56 — a 13-column
  window. A space found at column *k* breaks there: columns *k*+1..68 are erased
  from the screen with `\b \b`, along with the space at *k*, and re-echoed after
  the next line's prompt. The key that triggered the wrap lands on the new line
  after them.
- Splits at the margin when no space is in reach, carrying nothing. A word
  longer than 13 columns therefore gets cut mid-word, not carried whole.
- Keeps the space at *k* in the stored line even though it erased it from the
  screen, so a wrapped line is saved with a trailing space.
- Erases one screen cell in the margin-split case although nothing was carried,
  so the line renders one character shorter than it saves. Confirmed against
  `data/msgs.dat` and by backspacing into the line, which redrew all 68.
- Opens a **21st** line when it fires on line 20, accepts the carried word plus
  the triggering key there, then refuses every further key — and drops that line
  when saving, losing the word.

Backspace at column 1 takes the line above back out of the message and re-opens
it with the cursor at its end. BRE does not move the cursor up a row; it draws
the line again below, under the red prompt noted above.

Quoted lines in a reply are placed in the buffer and echoed as they stand. Only
a keypress can wrap, so they are never re-flowed however long they are.

**IB's deliberate divergences**, all in `internal/menu/actions_message.go`:

- On line 20 IB stops taking keys at the margin instead of opening a 21st line.
  BRE's 21st line is discarded on save, so the text the player watched himself
  type is lost either way; stopping at the margin keeps the 68 columns.
- IB erases nothing on a margin split, so the screen matches what is saved.
- IB trims the space it broke at rather than storing it.
- IB draws the banner and ruler bright cyan. That predates this capture and is
  recorded here rather than changed, since the colour is not what the wrap fix
  was about.

## Planetary diplomacy (captured live 2026-08-08, league game)

Two screens and one line, all reading the same per-board chart. It is a local
annotation, not a treaty system — the editing screen says so itself.

### Diplomacy List (InterPlanetary Ops → D)

```
──═Planetary Treaties═──
─────══════════───────────────────────────────────
( 1) Nova Hub                           None
( 2) Starship Junkyard                  Allied
( 4) The Eclipse                        Peace
─────══════════───────────────────────────────────
```

Measurements: the rule is **50** columns (5 `─`, 10 `═`, 35 `─`); each row is
`(` + a 2-column number + `) ` then the planet name in **35** columns, with the
status after it. The board the player is on is **not** listed (the capture is
from planet 3 and jumps from 2 to 4).

Colors: the title's `──` red `31` and its `═` bright-red `1;31` around a
bright-white `1;37` title; the rules gray `1;30`; the row parentheses gray
`1;30` around a bright-white number; the planet name bright-white `1;37`. The
status carries its own color — **None** white `37`, **Peace** bright-green
`1;32`, **Allied** bright-blue `1;34`. **Enemy** never appeared in the capture,
so IB's bright-red for it is an inference, not an observation — **UNVERIFIED**.

### The relations line

Printed by the shared planet prompt, so it follows **every** planet the player
names — a terror op, an IP message, a Spy Database lookup:

```
Our current relations with Starship Junkyard: Allied
```

White `37` body, the planet name bright-white `1;37`, the status in its own
color as above.

### Diplomacy Modification (BBS Coordinator only)

Not reachable in the capture — the caller was not the elected Coordinator — so
this is read from the binary rather than observed, so its layout and colours are **UNVERIFIED**. It is reached from the System
Menu's `Coordinator Menu` item (`BRE.OVR` 0x13920).

The menu has **four items, keyed `1`-`4`**, and its handler settles both the
labels and the mapping. `run_interbbs_operations_menu` (`BRE.OVR` 0x015e3a, unit
`ovr_015dbf`) draws each row with `mov al,0x31`..`0x34` against the four label
offsets 0x00 / 0x11 / 0x22 / 0x37, which are the four strings stored together at
`BRE.OVR` 0x15dbf, then dispatches on `cmp al,'1'`..`'4'`:

| Key | Item | Opens |
|---|---|---|
| 1 | Dismantle Gooie | confirms `Are you positive?` first |
| 2 | Modify Diplomacy | the `Diplomacy Modification` screen below |
| 3 | Global Recon Request | posts `Recon Requests Created to All BBSs` |
| 4 | View Diplomacy | the `Planetary Treaties` chart |

A fifth key, read from a variable rather than a literal, quits; anything else
redraws the menu.

**IB's keys match this table as of 2026-08-18 and did not before**: it numbered
the three items it had built `1`-`3`, so each sat on the original's key for its
neighbour, and its own Player List sat on `4`, View Diplomacy's key. Key `1` now
holds IB's Dismantle Gooie (#45), and Player List has moved to `5`,
past the original's four.

**The Coordinator gets no player list in the original.** The two coordinator
roles are different offices: the **BBS Coordinator** whose menu this is was
elected by the planet's own barons and is an ordinary player, while the
**League Coordinator** operates the coordinating board. Neither implies the
other, and neither implies a sysop — a board's operator may be playing or may
not be, so "the sysop knows the handles anyway" is no argument for handing them
to whoever wins an election.

BRE's player list belongs to the second: `PLAYERLIST`, a command-line switch its
manual documents as "for League Coordinators only", writing `PLAYERS.LST` from
`DATA\DUPE.BR`, the duplicate-user file. That file is the only place a caller's
BBS account name is printed anywhere near the game. No screen shows one. The
empire record holds the BBS name at +0x00 and the realm name at +0x1f (both
`String[30]`, proven against a live `game.dat` where a caller `WRAPTEST` holds a
realm `Wraptest`), and See Scores, the coordinator vote and the interplanetary
Player Information screen all print +0x1f — as does the recon packet, whose
`PlanetInfo.Names` `build_recon_record` fills from the same field, so a realm's
owner never crosses to another planet either. IB's Player List carried an Owner
column of BBS handles until 2026-08-18; it now names realms only.

So the menu **item** is `Modify Diplomacy`; `Diplomacy Modification` is the title
of the screen it opens.

**Correction.** This section previously recorded "eight hotkeys (`DEFRIOKL`)"
from `BRE.EXE` 0x14e50 and called the mapping unestablished. That string is not
this menu's key set: it sits in a run of `GAME\BREINS.TXT` help-topic names
(`Spending Menu`, `System Menu`, `Coordinator Ops`, `Sell Menu`, `Covert
Operations`, `Preferences`), and the handler above proves the keys are `1`-`4`.
The note that IB's `D` was "probably the original's key for Dismantle Gooie" was
wrong for the same reason.

`BRE.OVR` 0x23530 carries the screen: the title `Diplomacy Modification`, the
prompt `Change status to War, None, Peace, or Ally?` (keys `WNPA` at 0x23594 —
this file read that string as `WNPAU` until 2026-08-18, but the length byte
ahead of it is `0x04` and the `U` is the `0x55 push bp` of the routine that
follows, the length-prefix trap; there is no fifth key, and IB's four match,
with the tails `ar, ` / `one, ` / `eace, or ` / `lly? ` stored separately, so
each key character is printed highlighted ahead of the rest of its word), and
at 0x23425 the note that governs the whole mechanic:

> NOTE:  Planetary Diplomacy is *not* official.  This is used for you to
> allow the gamers to learn of your status with other Planets.  None
> of the info in this chart is official.  (ie, none of this is forced
> nor reported to the other planets.

The four display words are a table at `BRE.EXE` 0x158b5: `Enemy`, `None`,
`Peace`, `Allied` — so the prompt's *War* files *Enemy* and its *Ally* files
*Allied*.

## SDI program funding (captured live 2026-08-08, league game)

Seventeen consecutive SDI Program screens across one game, from an empty program
to just over seven million gold. Funding is the screen's Total Funding; upkeep
its Yearly Maintenance; allowance its "Maximum productive spending this year".

| Funding | Upkeep | Allowance | Strength |
|---|---|---|---|
| 0 | 0 | 250,000 | 0% |
| 250,000 | 10,000 | 250,000 | 1% |
| 500,000 | 20,000 | 250,000 | 2% |
| 750,000 | 30,000 | 250,000 | 3% |
| 1,000,000 | 40,000 | 250,000 | 3% |
| 1,250,000 | 50,000 | 250,000 | 3% |
| 1,500,000 | 60,000 | 300,000 | 4% |
| 1,800,000 | 72,000 | 360,000 | 4% |
| 2,160,000 | 86,400 | 432,000 | 5% |
| 2,592,000 | 103,680 | 518,400 | 5% |
| 3,110,000 | 124,400 | 622,000 | 6% |
| 3,732,000 | 149,280 | 746,400 | 6% |
| 4,478,000 | 179,120 | 895,600 | 7% |
| 5,373,000 | 214,920 | 1,074,600 | 8% |
| 5,899,000 | 235,960 | 1,179,800 | 8% |
| 7,078,000 | 283,120 | 1,415,600 | 9% |

Two exact fits across every row:

- **upkeep = 4% of funding**
- **allowance = max(250,000, 20% of funding)**

Both are per **turn**, not per game year: the allowance dropped to 0 immediately
after being spent and was back at its full value the next turn, and the upkeep
was billed in every turn's maintenance sequence.

The funding column is truncated to whole thousands — adding a 518,400 allowance
to 2,592,000 gives 3,110,400 but the next screen reads 3,110,000 — which matches
the screen's own note about funding in increments of 1000.

**Strength is a function of funding AND land**, so the funding column alone
cannot give it. The curve came out of the binary instead
(`trunc(sqrt(funding / (10 x (regions+1))))`, see `docs/mechanics-reference.md`),
and every row above reproduces exactly at **8,321 regions** — the count this
capture's realm held, read off the Terrorist Ops price (`regions × 64` (base at opsToday=0) =
532,544) on the menu the SDI screen opens from.

The `Funding / Region: 0,000 Gold` line is not a defect: the program is stored in
whole thousands and the screen appends a literal `,000`, so a realm funding under
1,000 gold per region reads as zero.

The upkeep is billed in the maintenance sequence between the region maintenance
and the crown tax:

```
6,724,245 gold is required to maintain your regions.
How much will you give? (6,724,245; 6,724,245)
Your SDI Program requires 10,000 gold.
How much will you give? (10,000; 10,000)
The Queen Royale requires 2,409,303 gold for Taxes.
```

Figures in that prompt are bright-cyan `1;36` on white `37`, like the region and
crown lines around it.

## Indiv. Attack Force, end to end (captured live 2026-08-11, two-board league)

The first fully end-to-end interplanetary strike driven against the original:
sent from ALPHA (node 1) at BRAVO (node 2), resolved on the far board, and the
result read back in the sender's news. Everything below is literal output.

**Gate.** The IP options refuse to open until a turn has been played this entry:

```
You must play at least one turn per entry in the game to access this option.
```

**Target picker** (the same `Enter Planet Name or Number` prompt BRE uses
everywhere, then a lettered roster):

```
Enter Planet Name or Number (? for list): Test Planet Two
Our current relations with Test Planet Two: None
Choose a target[A-Y, ?, /Search, [ENTER]=Abort]
-*Players at Test Planet Two*-
Id   Empire Name                          Territory   Score   Networth
─────═══════════════───────────────────────────────────────────────────────
(A)  Bravo                                       15     426        212
─────═══════════════───────────────────────────────────────────────────────
                                                [BRE v0.988]   8/15/2026
```

**IB's interplanetary baron list is NUMBERED**, where this capture shows the
same lettered roster the local screens draw. That is an old divergence and it is
not addressed here. What did change (#214): a baron the last scores packet had
under New Realm Protection is listed with the `(P)` flag after the name and the
strike is refused when that row is picked. Those barons were HIDDEN from the list
until 2026-08-26, with a count printed beneath it saying how many had been held
back, which named nothing and left a planet's roster disagreeing with its target
list. The flag is a courtesy either way — a packet can be days old, so the target
board still refuses an arriving strike on its own authority.

**The interplanetary picker's prompt is colored differently from the local
one**, though the wording is nearly the same (`cap/eots-ibbs-01.cap`): here the
brackets are `34` blue and every key inside them `97` bright-white, where the
local `Choose a Target [A-Y,?=List RETURN to Abort]` uses `31` red brackets and
`93` bright-yellow keys. Another case of the color belonging to the screen
rather than to the kind of thing.

**Attack Type menu.** A 21-column box, sized to its own content as BRE always
does. Note the `(?) Help` item and that **Enter takes Quit**, not the first item.
(Not re-reached on 2026-08-16 — the menu needs a live two-board league with recon
data, and a single-board attempt stops at `Sorry, we don't have any information
on that planet yet.` The colours below are from the 2026-08-11 capture and stand.)

```
────[Attack Type]────
(1) Normal Attack
(2) Quick Strike
(3) Extended Battle
(?) Help
(0) Quit
─────────────────────
Choice> Quit
```

Colours: rules, parentheses and the closing rule are **red (31)**; the item
digits and the `[` `]` around the title are **bright red (91)**; the title
`Attack Type` is **bright white (97)**; item labels are **white (37)**. The
prompt is `Choice` in white, `>` in bright white, and the `Quit` default in
white — the ordinary menu-engine prompt.

**Force prompts and confirmation.** Unlike the group-attack picker, this one
asks about **every** unit type, including ones held at zero, and each default is
0 rather than "send everything":

```
Choice> Quick Strike
Send how many Troopers? (0; 100) 100
Send how many Jets? (0; 0) 0
Send how many Tanks? (0; 0) 0
Send how many Bombers? (0; 0) 0
This attack will cost 100 gold.
Send this Attack? (Y/n) Yes
```

100 troopers cost 100 gold, so the rate is 1 gold per unit — **verified for
troopers only**; whether the other three types cost the same is **UNVERIFIED**.

**Result, in the sender's news after the round trip:**

```
───Alpha's forces have returned with news of failure from Bravo of Test
     Planet Two!
```

**Deliberate divergences in IB.** IB keeps its own `(?) Help` content (the
Attack Types topic) rather than BRE's `attack.hlp` wording, and comma-groups the
gold figure, as it does everywhere.

## Create Group Attack (captured live 2026-08-11, two-board league)

Driven on the same pair, in a played turn. Literal output:

```
Choice> Create Group Attack
Enter Planet Name or Number (? for list): Test Planet Two
Our current relations with Test Planet Two: None
Do you wish to target (O)ne Dominion or (A)ll? Entire Planet
Wait how many Hours (12-120)? (12; 120) 12
Send how many Troopers? (0; 89) 0
Send how many Jets? (0; 0) 0
Send how many Tanks? (0; 0) 0
Send how many Bombers? (0; 0) 0
Attack Aborted
```

**The force prompts are identical to the individual strike's** — all four unit
types, including ones held at zero, every default 0. There is no attack-type
menu here: a group attack has no type choice.

**The departure delay is in HOURS**, `Wait how many Hours (12-120)?`, with a
12-hour floor and a 120-hour ceiling — and it is asked BEFORE the force prompts,
not after. IB matches this (#124): it asks the same question in the same place
and stores the answer as a departure instant, because the binary does
(`hours/24` added to the clock — see docs/mechanics-reference.md).

**Whole-planet vs one baron is a single keypress**, `Do you wish to target
(O)ne Dominion or (A)ll?`, echoing "Entire Planet", and it is asked BEFORE the
roster — so a planet-wide strike never draws the baron list. IB matches this
(#125); it used to offer "(the whole planet)" as the first row of a numbered
list. The capture shows only the "A" echo, so IB's "One Dominion" for the other
key is a guess at wording, not a captured string — **UNVERIFIED**.

An empty force prints `Attack Aborted` (capital A on both words), where the
attack-type menu's quit path prints `Attack aborted.` with a period.

## The lottery (first play of a game day)

Captured with colour in `cap/kd3-01.cap`, declined in `cap/eots-ibbs-01.cap`,
and played several times in `cap/20240527-134Pho_Lazarus_Public.cap` (that one
holds no escape sequences). It follows the Queen Royale's tax refund and is the
second half of the same first-play block.

The offer is BRE's ordinary y/n prompt, cyan `(Y/n)` with a bright-white echo of
the full word `Yes` or `No`. Then, on separate lines, a prompt for the ticket
and one for the draw — from `cap/kd3-01.cap`, with the escapes shown:

```
Choose your 6 letters: ESC[1;36m AGNTYI
Winning Letters: ESC[0;40;31m D ESC[31m K ESC[31m M ESC[31m M ESC[1;33m I ESC[0;40;31m U
```

| Element | Colour |
|---|---|
| ticket letters, as they are typed | `1;36` bright cyan |
| a drawn letter that matched | `1;33` yellow |
| a drawn letter that did not | `0;40;31` dark red |

That one capture proves the whole scoring rule on its own: the ticket is
`AGNTYI`, the draw `DKMMIU`, and the single yellow letter is the `I` — which
sits at position 5 in the draw and position 6 on the ticket, so matching cannot
be positional.

The six ticket letters are **six separate keypresses with no Enter to finish**,
each accepted only if it is A–Z; the count of echoed characters after the prompt
is what shows this. Enter fills the slot with a random letter, so the echo never
stalls on a player who just wants it over with.

**IB deviates on two points.** An unmatched letter is drawn in bright red, not
dark red, which sits at about 2:1 against black; and the result line states the
match count in words, so nothing depends on telling two colours apart. Both are
deliberate — see `docs/mechanics-reference.md`.

## The Coordinator notice (top of every InterBBS turn)

Printed the moment Play Game is chosen, before the "Since your last play"
header, and only in an InterBBS game. One of two lines, chosen by comparing the
elected Coordinator's id against the caller's own (`run_door_session`,
`BRE.EXE 013a:0cf7`).

To the Coordinator (`cap/treaty-order-20260817.cap`, and colour-stripped in
`cap/20240527-134Pho_Lazarus_Public.cap`) — the office is the bright segment,
BRE's colour `0x0f`, and the line returns to `0x07` after it:

```
You are currently the ESC[0fm BBS Coordinator ESC[07m
```

To everyone else (`cap/eots-ibbs-01.cap`), the voted realm is the bright
segment, and a second line follows in the body colour:

```
Your current vote for BBS coordinator is: ESC[1;37m Dynoland ESC[0;40;37m
You may change your vote in the system menu
```

Note the casing: the office is capitalised on the first line and not on the
second.

A baron who has not voted has `No one` in the name slot, from
`format_no_recipient` (`BRE.OVR 0x0176f`), so the line never changes shape. IB
prints its own wording in the same place, the same shape and the same three
cases.

## Game Setup and Send Trade Deal (captured live 2026-08-23, `cap/shsbbs.cap`)

### The panel (System Menu → G)

The panel is **78 columns** and pairs settings two to a line, the second block
starting at column 38. Labels carry their own colon and are padded to 21, so a
value always begins at column 22 of its block. It is one panel, not two — the
whole ruleset fits a screen because of the pairing:

```
──────═══════════════─────────────────────────────────────────────────────────
Game Started:        6/27/2025
Turns per day:       8
Protection Turns:    20
Daily Land Creation: 1000
Planetary Tax Rate:  6.9%
Maximum Players:     25
Bank Interest Rate:  5.0%
Investment Rate:     5.2%
Maintenance Costs:   Medium          Region Cost Change:  Medium
Trade Deal Costs:    Medium          Attack Damage:       Medium
Attack Rewards:      Medium
Military purchasing: Enabled
This game is setup for Local mode only.
──────═══════════════─────────────────────────────────────────────────────────
```

On a **league** board the mode line changes and the interplanetary rules follow
it, still inside the same single panel — 21 content lines, no pause
(`cap/20240527-134Pho_Lazarus_Public.cap`, a 13-board league):

```
Military purchasing: Enabled
This game is setup in InterBBS mode with 13 boards in the game.
Attack Costs:        Medium          Terrorism Costs:       Medium
Maximum Individual Attacks Per Day: 2
Maximum Group Attacks Per Day:      2
Maximum Terrorist Ops Per Day:      10
Maximum Bombing Operations Per Day: 15
Days before "lost" forces returned: 2
Gooie Kablooies: Enabled     Bombing Ops: Enabled     Missile Ops: Enabled
```

Note the three label widths in use: 21 for the paired rows, 36 for the
"Maximum ... Per Day" block, and a three-up line for the switches whose labels
carry their own leading spaces (`'Gooie Kablooies: '`, `'     Bombing Ops: '`,
`'     Missile Ops: '`).

**BRE names no league anywhere on this screen — neither a number nor a name.**
The board count is the whole of it (`show_game_settings`, BRE.OVR 0x13c44:
`'This game is setup in '`, `'InterBBS'`, `' mode with '`, `' boards in the
game.'`). IB shows a **league group first**, carrying the league's number, this planet,
the roster size and the board holding the League Coordinator — an addition, not
a fidelity fix. Those are what identify a league: the number is what keeps two
sharing one inbound directory apart, and the Coordinator's board is who set the
rules below.
IB has no league NAME at all. It carried one until 2026-08-23 — its own
invention, with no counterpart in the original — and dropped it: hardly any
league ever set one, and a row that came and went left the group a different
shape on every board a player visited.

IB also diverges by grouping under captioned dividers and by paging in two.
BRE fits one panel because its ruleset is 21 lines; IB's has grown well past
that, and pairing settings two to a line is what holds the result to two pages.

**This capture settles the crown tax.** `Planetary Tax Rate: 6.9%` beside a
first turn earning 53,740 gold and charged 3,708 — `trunc(53740 x 0.069)` — so
the rate is a sysop's setting and the formula in
`docs/mechanics-reference.md` is unchanged.

### Send Trade Deal — the Request side shows no counts at all

**Read from the binary, not from a capture.** `cap/shsbbs.cap` switches from BRE
to Immortal Barons partway through (the A-Net game server's own door menu, `[0]
Immortal Barons`), and the trade-deal screens in it are IB's own — a trap worth
naming, because the two games look alike enough that IB's output was briefly
written up here as the original's.

BRE draws both baskets from ONE routine, `compose_trade_demand` (BRE.OVR
0x25ab4), with a Send/Demand flag at `[bp+0x6]`. Three things branch on it, so
the Demand table is a genuinely different screen and not the same one with a
blank column:

- the header tail `'       # Owned'` prints only on the Send side (0x25d47);
- the divider is **48** characters with it and **34** without (0x25d85);
- the row format differs, and only the Send row looks the owned figure up
  (`0504:003e`, 0x25db6).

Its strings: `'Stuff to '` + `'Send'`/`'Demand'` for the title, `'Key  Item
              '`, `'  # to '`, `'       # Owned'`, and
`'Trade Deal requires N Carriers.'` with a separate `'WARNING:'` +
`' You do not have enough carriers.'`.

The quantity prompts are `'Send how many '` / `'Demand how many '` + the item +
`'? '`. BRE's bounded input helper (`056d:01bf` -> `0851:0bd9`) is a raw
key-by-key editor that prints no bounds of its own; the `(N; M)` hints elsewhere
in BRE are printed by their callers.

**The SEND side does have one; the DEMAND side does not** — this file said
neither did until #195. `create_trade_offer` calls the hint helper
(`0851:08b2`, which builds `"(min; max) "`) at unit offset 0x1a4c, immediately
before the Send prompt's input call, and makes no such call before the Demand
prompt at 0x1ce7. The helper prints only `"(min) "` when the max is negative
(`BRE.EXE 0x8dd1`), which is exactly the Demand side's `0xFFFFFFFF` sentinel — so
even where it is called on a sentinel it quotes no ceiling. The interplanetary
routine calls it too (`send_trade_offer` 0x0395). And the bounds themselves say
the same thing the columns do (`create_trade_offer`, BRE.OVR 0x2624c):

    Send prompt   bounded by the player's own count of that item   (0x2679d)
    Demand prompt bounded by 0xFFFFFFFF — a sentinel, not a figure  (0x26a4b)

**IB matches the important half and diverges on the rest.** No Owned column on
the Request side, and no ceiling quoted in its prompt — IB used to print the
sentinel as `(0; 1.0737B)`, which reads like information about the other realm
and is not. IB keeps its `(suggested)` hint on both sides where BRE has none,
and keeps a real 2^30 ceiling internally so the figure stays inside an `int` on
a 32-bit door build.

## Travel Times (InterPlanetary Ops → T) — captured live, `cap/eots-ibbs-01.cap`

**The board you are calling is not in the list, and only it is missing.** The
same session's planet roster a few hundred lines earlier (line 20389) shows four
planets and *does* include the caller, which is what rules out the alternative
reading of a three-board league:

```
## Planet Name                Location
 1 Nova Hub                   Brisbane, QL, AUS
 2 Starship Junkyard          Brisbane, QL, AUS
 3 Eye of the Storm           New Plymouth, TN, NZ      <- the board being called
 4 The Eclipse                Sydney, NS, AUS
```

The screen itself (line 20613) lists the other three:

```
Average Turn Around Times to All BBSes
──────═══════════════──────────────────────────────────────────────────────
Nova Hub                      0.80 hours
Starship Junkyard             0.80 hours
The Eclipse                   1.00 hours
──────═══════════════──────────────────────────────────────────────────────
```

Geometry, measured off the capture: a **75**-column rule with a **15**-column
double run starting at column 6, and the planet name in **30** columns before
its figure. Hours print to two decimals over a value quantized to a tenth
(`0.80`, `1.00`) — the format and the quantization are different widths, which
is easy to get backwards.

`show_interbbs_turnaround_times` (`BRE.OVR 0x23d76`) holds exactly four strings:
`'Average Turn Around Times to All BBSes'`, `'No Data'`, `' hours'`, `' days'`.
`No Data` matters to the finding: BRE **does** print a row for a board it has
never measured, so leaving the caller out is a deliberate omission rather than a
row suppressed for want of a measurement.

IB matches all of it — `KnownBoards()` drops `Config.BoardID` and nothing else,
and the constants are `travelRuleWidth`/`travelRuleDouble`/`travelNameWidth`.
Its sub-hour tiers (seconds, minutes) are IB's own; BRE never needed them,
because no 1990s FidoNet link answered in under a minute.
