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

**IB marks a faction that raided you since your last play.** BRE's list has
no such flag. IB writes `->` immediately left of the name — same idiom and
color treatment as the online mark (`(O)`, above): the shaft is `90` dark gray
(decoration) and the arrowhead is `37` gray (the part that must carry the
meaning alone), reserving the same width on every row so an unmarked faction's
name still starts in the same column. The mark is cleared and reset each time
the income report runs, so it flags only the raiders named in that turn's
report, not an older one.

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
──────────────────────────────────────────────────────────────────────
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
to one house width. Measured across the captures in this file: 23 (Attack Menu),
28 (Diplomacy), 38 (InterBBS Scores), 44 (Spending), 46 (Industrial Production),
47 (opening), 61 (Crazy Gold Bank), 62 (InterPlanetary Ops), 64 (Food
Unlimited), 69/75 (System Menu). IB's `rule` constant is 62; a screen that draws
its own box should take its width from its own capture instead. IB sets `Width`
per menu where a capture gives one: Attack 23 (its content measures 23 too, so
the fit is exact), Spending 44, Industrial Production and Specialization 46.
**System is 59, not BRE's 69/75** — BRE lays that menu out in three columns and
its box tracks whichever items are showing, IB uses two, so IB follows the
principle (size to content) rather than the number. Specialization
is not captured — IB draws it at 46, matching Industrial Production beside it,
which is a reasoned guess rather than a verified figure.

| Menu | Accent |
|------|--------|
| Opening menu (`Barren Realms Elite`) | `35` magenta |
| See Scores / Regions buy / Attack Menu / Advisors | `35` magenta |
| Diplomacy Menu | `36` cyan |
| Crazy Gold Bank / Food Unlimited | `36` cyan |
| Spending Menu / Industrial Production / Trading / Specialization | `31` red |
| System Menu | `34` blue |
| InterPlanetary Operations | `33` yellow |
| InterBBS Scores / Attack Pirates | `90` bright-black (gray) |
| Sell Menu | `32` green |

Data-value convention (status, income, bank, food, end-of-turn): the **numbers**
are `96` bright-cyan, the surrounding label text `37` white. Maintenance-paid /
food-consumed lines and the Spending-Menu Price/#Owned columns use `97`
bright-white numbers instead.

### Opening menu (top-level)

Shown after login and after each news screen. Magenta accent; two columns.

```
─────────────[Barren Realms Elite]─────────────
(1) Play Game             (8) Game Bulletins
(2) See Status            (9) InterPlanetary Ops
(3) See Scores            (A) Game Instructions
(4) See Today's News      (B) Help Database
(5) See Yesterday's News  (P) Preferences
(6) Read Messages         (0) Quit
(7) Send Messages
────────────────────────────────────────────────
Choice> Play Game
```

### Daily News File / Daily Bulletin

Header line: `93` bright-yellow `Barren Realms Elite ` + `97` `v0.988` + `37`
`: News File` with the date right-aligned. Then a blank line, then a **centered**
banner: `31` red `──` + `91` bright-red `»` + `97` bright-white
`The Queen's Quadrant` + `91` `«` + `31` `──`, then a full-width `33` yellow rule.

The Daily Bulletin box has a `34` blue border with `97` bright-white
`Daily Bulletin` in the top edge. Three rows; label `37` white, value `36` cyan,
`Change:` `37` white. **Positive** change = `92` bright-green `+` then `96`
bright-cyan value; **negative** change = `96` bright-cyan for the whole thing
(minus sign included — direction is not color-coded).

```
                        ──»The Queen's Quadrant«──
───────────────────────────────────────────────────────────────────────────
    ╔══════════════════════════Daily Bulletin══════════════════════════╗
    ║  Total Population: 185,861             Change: +19243              ║
    ║  Total Regions:    25,283              Change: +1167               ║
    ║  Total Net Worth:  2720k               Change: +616k               ║
    ╚═══════════════════════════════════════════════════════════════════╝
```

Below the box, the news items. Each item starts with `31` red `──` + `91`
bright-red `─` arrow, then `37` white body; wrapped continuation lines indent 5
spaces. **A blank line separates every item.** In-line highlights: your own
empire `93` bright-yellow, other empires `97` bright-white or `93`, planet names
`96` bright-cyan, numbers `93` bright-yellow, timestamps `97` bright-white, the
`The Queen Royale` actor `91` bright-red. Today's News and Yesterday's News use
this same layout. (IB is free to reword the prose — clean-room — this records
BRE's wording and coloring only.)

### See Scores (local planet)

Title `-*` `91` red + `97` bright-white `Barren Realms Elite` + `*-` red. Column
header row `97` bright-white. The border mixes single `─` and a `══` double-line
accent over the name column. Each row: `35`(`(`)` + `97` key + `35`)` + a flag
column (`+` = participating this reset, space = not) + `37` white name + `95`
bright-magenta Territory + `97` bright-white Score + `37` white Net Worth.

```
-*Barren Realms Elite*-

Id  Empire Name                          Territory     Score    Net Worth
─────══════════─────────────────────────────────────────────────────────
(A)+Empire One                               6936    536115     1344439
(B)+Empire Two                               5749    442683      692769
(E) Your Empire                              1140      4260       24814
─────══════════─────────────────────────────────────────────────────────
```

**IB adds an online mark (#123).** BRE has no online indicator; IB writes `(O)`
for a baron who acted within `OnlineWindowSecs`, hugging the LEFT of the empire
name inside the name field. Your own realm is never marked. Every local roster
carries it: the attack target picker and the recipient picker share
`scoreTableRow`/`nameCell`, and the Coordinator's Player List puts it in its
indent. The inter-BBS scores screen does NOT — those figures arrive in packets
that may be hours old, so there is nothing to report.

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
status block bordered by `34` blue rules (single/double mix), then the
maintenance-paid lines (`97` bright-white numbers).

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
no rule, no fill. IB drew it as two boxed pages of its own invention until
v0.0.4; both are gone and IB now renders the block above.

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
  `balance.go`, "Population / migration"), so the line reads
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
──────────────────────[Crazy Gold Bank]──────────────────────
(C) Cash Relief / Loans       (L) List Investments / Loans
(D) Deposit Funds             (V) View Bank Rates
(W) Withdraw Funds            (0) Quit
(I) Investments
──────────────────────────────────────────────────────────────
You have 4,582,875 gold in hand and 7,255,312 gold in the bank.
Choice> Quit
```

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
─────────────────────────
(C) Coastal          1088
(R) River              24
(A) Agricultural       14
(D) Desert              5
(I) Industrial          0
(U) Urban               4
(M) Mountain            5
(T) Technology          0
(*) Advisors
─────────────────────────
Your choice?
```

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

Cyan accent, single column. (Reached pre-turn in this build, and from the System
Menu.)

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
label — **the launcher's region count × 64**, so it climbs as the realm buys
land. The `72,960` below is 1,140 regions; a second capture
(`cap/eots-ibbs-01.cap`) gives 541,824 / 542,144 / 542,336 / 542,976 against
8,466 / 8,471 / 8,474 / 8,484 regions, every one exact. The derivation and the
one term still unpinned are in `docs/mechanics-reference.md`.

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

**Travel Times** (T): `Average Turn Around Times to All BBSes`, one
`planet    N.NN hours` row per board.

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

Colors: labels white `37`, every figure bright-yellow `1;33`, the Note line gray
`1;30`. After funding it prints `N Gold added.` then `Current SDI Strength: N%`
again. See the SDI section at the end of this file for the seventeen captured
funding levels and what they establish.

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

                          »Planetary Post«

      Name                               Score
────────────────────────────────────────────────────
(  1) Planet A                         1321561
(  2) Planet B                         1254755
```

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

Blue border rules. Opens with a support-flavor line, then population change and
food spoilage (numbers `96` bright-cyan), then `Do you wish to continue? (Y/n)`.

```
End of Turn Statistics
───────────────────────────────────────────────────────────────────────────
Your people have great faith in you as an excellent ruler!

Your dominion gained 380 million people.
57 units of food spoiled.
───────────────────────────────────────────────────────────────────────────
Do you wish to continue? (Y/n) Yes
```

### Food Unlimited (Food Market)

Cyan accent, single row of options. Preceded by the market line (`available`,
`buying for`, `selling for` — all figures `96` bright-cyan).

```
We have 6,851,917 units of food available today.
We are buying for 9 and selling for 26.

────────────────────────[Food Unlimited]────────────────────────
(B) Buy Food    (S) Sell Food   (V) Visit Bank  (0) Quit
─────────────────────────────────────────────────────────────────
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
────────────────────────────────────────────────────────────────────────────
```

- **Set Tax Rate** (R): `New Tax Rate [0-100, Current Tax Rate = 20]?`
- **Game Setup** (G): the read-only ruleset dump —

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

Two pages, moved through with **PG-DN / PG-UP**; `[ESC]` saves and quits. Fields
are **colour-coded by type, and every value is the BRIGHT form of its label's
colour** — labels `3x`, values `9x`:

| field type | label | value | seen on |
|---|---|---|---|
| dates | `32` green | `92`/`97` bright | Game Start Date, Game Join Date |
| numbers | `36` cyan | `96` bright-cyan | Turns Per Day, Turns of Protection, rates, caps |
| presets | `35` magenta | `95` bright-magenta | Maintenance/Trade Deal/Region Costs, Attack Damage |

The `: ` separator and the trailing pad are `37` white. A `*` prefix marks an
InterBBS-only setting; the footer states it.

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

The market itself lists the eight tradeable goods — note keys 1-5 and 7-9, with
**no key 6** (regions are not tradeable):

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

## Covert Operations (System Menu → C) — colors NOT captured

**The menu's text is binary-verified; its colors and decorations are not.** The
item labels and their order come from the overlay's string table, where Turbo
Pascal stores them consecutively in declaration order, which is render order
(`BRE.OVR 0x1731a` onward):

```
Send Spy · Stir Revolts · Set Up · Support Dissensions · Demoralize Forces
Spy on Relations · Bomb Enemy Targets · Bribery · Expose Enemy Ops · Visit Bank
```

The status line under it is assembled from `'You have '` + gold + `' gold and '`
+ agents + `' agents.'`, and `'Limit one try per turn!'` sits just below in the
same unit — the one-effect-op-per-turn cap.

The **Bomb Enemy Targets** set and the interplanetary Special Operations menu
share ONE table at `BRE.OVR 0x2981b`, in this order, with a zero-length entry
between Nuclear Assault and Chemical Bombing that a naive ShortString walk will
mistake for the end of the table:

```
Bomb Food Market · Bomb Trading Market · Bomb Trade Routes · Undermine
Investments · Nuclear Assault · <len 0> · Chemical Bombing · S3-Sabre · Send SpyGuy
```

The local menu takes the first seven, the interplanetary one all eight. The
500-bomber gate string follows immediately, which is where
`BombingBombersRequired` comes from.

**From the draw routine** (`run_covert_operations_menu`, `BRE.OVR 0x17469`),
read directly rather than inferred:

- **The hotkeys are binary-verified.** Each item block opens `mov al,<char>` with
  `0x31`–`0x39` then `0x56` — `1`-`9` then `V`, in the same order as the strings.
- **The per-op costs are a runtime table, not constants in the file.** Each item
  pushes a different consecutive dword from `DS:0x63e`, stepping 4 bytes per item
  (`[0x63e]`, `[0x642]`, …). That is why searching either binary for 5,000 or
  600,000 finds nothing, and it confirms the note in `balance.go` that other BRE
  setups scale these — IB's figures are the default setup sampled live, which is
  the most that can be pinned without reproducing BRE's setup arithmetic.
- **The entry gate is a signed 32-bit `agents > 0`** on the *held* agent count
  (`+0x26f`), tested high word then low. Agents escrowed on the Trading Market
  live at `+0x229` and are a separate field, so listing agents for sale can close
  the menu — IB matches, since listing moves them out of `Empire.Agents`. The
  per-turn covert step carries an extra gate on the byte at `DS:0x6d52`, which is
  the preference; the System-menu item does not, matching what the captures show.

**The colors are still unverified, and the disassembly does not settle them.**
Neither binary contains a single literal ANSI escape, so BRE holds color as a
Turbo Pascal attribute and builds the escape at write time; the draw routine
passes only key, label and cost, and neither it nor `enter_covert_operations_menu`
sets an attribute. The color therefore comes from the shared output helpers
(`0dc9:0608` formats the line, `0735:0000` writes it), which every screen uses —
so identifying it means tracing those and finding what sets the attribute
per-screen, not reading this routine. IB draws the menu bright-green, which
remains a choice rather than a finding. A `script`-wrapped live capture is still
the cheap way to close it (see the `bre-gather` skill).

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
pirate faction names) are recorded here for fidelity analysis but should be
**renamed** in IB, as with Gooie Kablooie → Clingy Annihilator and S3-Sabre →
R5-Slappenheimer.

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

**Unresolved:** Diplomacy passes 0 for the roster flag, which lists
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

**Incoming proposal**, in the "since your last play" block — after the numbered
recap entries and before the mail. It carries NO rule and NO timestamp: unlike a
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

**View Treaties -> `-*Relations*-`.** Lists EVERY living realm, "None" included —
the roster doubles as the empire-letter key. Title `-*`/`*-` blue (`0;40;34`)
around a bright-white word; heading bright white; a **75-column** inset rule
(5 `─`, 15 `═`, 55 `─`) in blue above and below the rows. A row is `[X]  ` then
the name in a **40-column** field: brackets blue, letter bright white, name
bright cyan (`1;36`), relation bright blue (`1;34`).
```
Id   Empire Name                             Relations
─────═══════════════───────────────────────────────────────────────────────
[A]  Endor                                   None
[F]  Opium                                   Full Defense Alliance
```

**Alliance Strength.** White headings, a **51-column** inset rule (5 `─`, 10 `═`,
36 `─`) in red above the rows and above the total, ally names bright white,
figures bright yellow and ungrouped, a zero shown as `NONE`. Data columns are
name 21, then 9 / 10 / 10; BRE's *heading* row is one column wider on Troopers
(10), so the headings sit one column right of the figures.

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

─────(1)────────────────────────────────────────07/31/2026  07:43:11────────
Opium accepted your Full Defense Alliance proposal.

─────(2)────────────────────────────────────────07/31/2026  09:07:07────────
DTF accepted your Full Defense Alliance proposal.
```

Colors:

- Header line `Since your last play, this has happened:` — plain white `37`.
- The rule, both sides of the counter and the date/time inside it — bright-black
  `1;30` (dim gray). The `(` `)` around the counter are yellow `0;40;33`; the
  counter digit itself is bright-yellow `1;33`.
- Body: realm name and treaty type bright-cyan `1;36`; the connecting words
  plain white `0;40;37`.

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

Colors: banner white `37`; ruler plain cyan `36`; the line-number prompt bright
green `92` with the typed text bright white `97`. A line re-opened by backspacing
off the start of the one below it is prompted in bright **red** `91`.

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
so IB's bright-red for it is an inference, not an observation.

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
this is read from the binary rather than observed. It is reached from the System
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
prompt `Change status to War, None, Peace, or Ally?` (keys `WNPAU` at 0x23594,
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
capture's realm held, read off the Terrorist Ops price (`regions x 64` =
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

**Attack Type menu.** A 21-column box, sized to its own content as BRE always
does. Note the `(?) Help` item and that **Enter takes Quit**, not the first item:

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
troopers only**; whether the other three types cost the same is not known.

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
key is a guess at wording, not a captured string.

An empty force prints `Attack Aborted` (capital A on both words), where the
attack-type menu's quit path prints `Attack aborted.` with a period.
