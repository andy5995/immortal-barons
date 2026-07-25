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

Yellow accent, two columns. `Terrorist Ops` shows a running cost figure next to
the label.

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

**Diplomacy List** (D): `»Planetary Treaties«` box, one
`( n) <planet>    <relation>` row per board.

**Spy Database** (S): prompts `Select Planet to view players:` /
`Enter Planet Name or Number (? for list):`. `?` lists planets with an operator
location column (`## Planet Name   Location`); selecting one prints
`Our current relations with <planet>: <relation>` then any known players.

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

## Clean-room note

BRE is proprietary (John Dailey Software; design by Mehul Patel). This file
records *observed* screen behavior to guide an independent reimplementation; it
is not a copy of BRE's source or assets. Distinctive coined strings/names (e.g.
pirate faction names) are recorded here for fidelity analysis but should be
**renamed** in IB, as with Gooie Kablooie → Doomer Kaboomer and S3-Sabre →
R5-Slappenheimer.
