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

## Status bar (bottom line, every screen)

`44`(blue bg)` BOB ` then `94` bright-blue `│` separators, fields in `37` white:
`│ A │ Asgard │`, ending with `F2=Extra Information`.

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

### Force selection

`You have ` white, counts in `96` bright-cyan. Each prompt: label white, then
`94` blue `(` + `96` bright-cyan (suggested/default) + `94` `; ` + `36` cyan
(max) + `94` `)`, input echoed `97` bright-white.

```
You have 4359 Troopers, 5000 usable Jets, 26,374 Tanks, and 0 Bombers
Send how many Troopers? (4359; 4359)
     How many Jets? (5000; 5000)
     How many Tanks? (26,374; 26,374)
     How many Bombers? (0; 0)
```

### Post-battle result (WIN)

Note BRE breaks down **both sides' losses by unit type**, and uses the same
"Exhausted from battle" header even on a win. The captured count is `96`
bright-cyan; all other unit numbers are `97` bright-white; labels `37` white.

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
in `97` bright-white. Rule: `95` bright-magenta, `─────═════...` (short double
accent in a single line). Each row: `35` magenta `(` + `97` bright-white
key-letter + `35` `)` + `93` bright-yellow region name + `97` bright-white Owned
count. `(*) Advisors` uses the same coloring. **Owned counts are the values
BEFORE this allocation** — BRE applies the picked amounts at the end.

Picker prompt (distinct from the buy/sell "Your choice?"): `34` blue `[` + `96`
bright-cyan (count) + `37` ` Regions left` + `34` `]` + `37` ` Your choice? `.
After a type is chosen: `How many <Type> regions? ` white + `94` blue `(` + `96`
`0` + `94` `; ` + `36` cyan (count) + `94` `)`. The picker loops, decrementing
the count, and auto-exits when it reaches 0.

```
Key Name            Owned
─────═════──────────────────
(C) Coastal           131
(R) River               0
(A) Agricultural       12
(D) Desert              5
(I) Industrial        311
(U) Urban               0
(M) Mountain          139
(T) Technology         58
(*) Advisors
─────═════──────────────────
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
──────────────────────────────────────────────────────────────────────
Choice> Quit
```

---

## Set Industries (System Menu → 1)

Title `31` red rule + `91` bright-red `[` + `97` `Industrial Production` + `91`
`]` + `31`. Each row: label `37` white + `:` + percent value `93` bright-yellow
+ `%` white + `31` red `(` + `91` bright-red (per-year count) + `37` ` per year`
+ `31` `)`. `Specialized` tag in `93` bright-yellow.

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

The advisor is **named** ("Hi, I'm Joe, your military advisor."). Advice lines
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

## Clean-room note

BRE is proprietary (John Dailey Software; design by Mehul Patel). This file
records *observed* screen behavior to guide an independent reimplementation; it
is not a copy of BRE's source or assets. Distinctive coined strings/names (e.g.
pirate faction names) are recorded here for fidelity analysis but should be
**renamed** in IB, as with Gooie Kablooie → Doomer Kaboomer and S3-Sabre →
R5-Slappenheimer.
