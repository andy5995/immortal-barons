# Turn-sequence flow rework (design)

Date: 2026-07-03
Status: design (approved direction)

## Why

Our menu is a free-roam action menu. The original BRE plays as a **guided
turn pipeline**: an outer Game Menu, then "Play Game" walks a fixed sequence
of screens, with a Spending hub and a `*` System Menu. This spec restructures
`internal/menu`'s *flow* to match that feel. Game logic in `internal/game`
is reused as-is; only presentation/flow changes.

Copyright note: the original's animated rocket splash and its exact "Helpful
Hints" wording are copyrighted — we write our OWN splash art and hint text.
The turn *flow* is mechanics and is fine to mirror.

## Target flow

### Splash (original, ours)
An Immortal Barons ASCII banner + a short "helpful hints" panel (our wording)
covering the input shortcuts and the `*` System Menu and `?` help. Then
"Continue? (Y/n)". Shown once at session start.

### Outer Game Menu (loops until Quit)
```
(1) Play Game        (6) Read Messages
(2) See Status       (7) Send Messages
(3) See Scores       (8) Game Bulletins
(4) Today's News     (A) Instructions
(5) Yesterday's News (P) Preferences
                     (0) Quit
```
"Play Game" runs a turn (below) if the player has turns left; else says so.
The other items reuse existing actions (status, scores, messages, bulletin,
prefs).

### A turn (Play Game) — a scripted pipeline
Each step is a screen or small sub-menu; the player steps through in order:
1. **Event log** — "Since your last play, this has happened:" (our existing
   `Events`), shown then cleared.
2. **Income report** — narrate income by source using region types:
   taxes, "Ore Mines" (Mountain), "Tourism" (Coastal), "Solar Power"
   (Desert), river income, and "Food units grown". This is what
   `RegionMix.income()`/`foodProduced()` already compute — just itemized.
3. **Status** — empire status screen (existing `empireStatus`), then
   "Visit the Bank? (y/N)".
4. **Payment/maintenance** — prompt to pay armed-forces upkeep, region
   maintenance, and taxes (auto-filled defaults, `>` = full).
5. **Food Market** — [Food Unlimited] Buy/Sell Food, Visit Bank, Quit; then
   "Your people need N units of food" feed prompt.
6. **Bank** — Deposit/Withdraw/View Rates/Quit.
7. **Spending Menu (hub)** — the item table (Troopers/Jets/Turrets/Bombers/
   HeadQuarters/Regions/Covert Agents/Tanks/Carriers, with Price and #Owned),
   Sell, Visit Bank, `*` = System Menu, Help, Quit.
8. **Attack Menu** — Regular/Nuclear/Chemical/Biological/Attack Pirates/
   Gooie/SDI/Alliance Strength/Visit Bank/Quit.
9. **Trading** — Trading / Trade Deal / Visit Bank / Quit; then "Send a
   message? (y/N)".
10. **End-of-Turn Statistics** — popular-support flavor line, population
    change, food spoilage; then "Continue? (Y/n)". This is where `PlayTurn`
    runs (income/food/upkeep/growth). On "yes" and turns remaining, loop to
    step 2 for the next turn; else return to the outer Game Menu.

### System Menu (`*` from the Spending hub)
Abdicate, Visit Advisors, Diplomacy, Empire Status, Food Market, Game Setup,
Messages, Preferences, Set Tax Rate, See Scores, Trading, Visit Bank,
Set Industries, Show Instructions, Quit. (Reuses existing actions; a few —
Advisors, Macros, Specialize — stay stubs.)

## Preferences drive the pipeline
The original's Preferences menu controls the turn pipeline. Map to our
existing (currently-inert) pref fields, adding the three "visit" toggles:
- **Visit Covert / Trading / Message Menu** (yes/no) — whether those pipeline
  stages are shown. (Add `VisitCovert`, `VisitTrading`, `VisitMessage`.)
- **Use Enter to exit BUY menu** (`EnterExitsBuy`) — pressing Enter at the
  Spending hub leaves it.
- **Deposit gold at end of turn** (`DepositEndTurn`) — auto-bank spare gold
  at end of turn.
- **Auto-Pay Maintenance** (`AutoPayMaint`) — auto-fill the payment step.
- **Auto-Feed Empire** (`AutoFeed`) — auto-fill the feed step.
This makes the pipeline customizable and finally gives the pref toggles
effect. Preferences are editable from the System Menu and the outer Game Menu.

## Industry / specialization (deferred, simplified for now)
The original produces military from **Industrial regions** each turn via a
production-percentage screen ("Set Industries"), and lets an empire
**Specialize** in one unit type. We currently buy units directly from the
Spending hub. For this rework, keep direct-buy; leave "Set Industries" and
"Specialize Industry" as System-Menu stubs. A later slice can add
industrial per-turn production + specialization on top of the Industrial
region type we already track.

## Input shortcuts (all numeric prompts)
- `>` = the maximum sensible value for that prompt (e.g. max affordable /
  all you have). Prompts pass their max to `promptInt`.
- `m` = multiply the typed number by 1,000,000.
- `k` = multiply by 1,000.
Display prompts as `(default; max)` like the original.

## Implementation slices
1. **Input shortcuts + original splash** — `promptInt` variants (`m`/`k`/`>`
   with a max), and a `splash`/`hints` screen (ours), wired at session start.
2. **Outer Game Menu + turn pipeline skeleton** — `menu.GameLoop(s, w)` that
   `play.Run` calls instead of `menu.Run(main)`; Play Game runs the pipeline
   calling existing sub-menus/actions in order; end-of-turn runs `PlayTurn`
   and the continue loop. Reuse the existing Attack/Trading/Bank/Diplomacy/
   Spending menus as pipeline stages.
3. **Narration steps** — income-by-source report and end-of-turn statistics
   text (popular support, population change, food spoilage).

## Reuse / keep
`internal/game` unchanged. The `Menu`/`Item`/`Run` framework stays for the
sub-menus; the outer flow is a scripted sequence on top of it. Existing
actions (buy/bank/attack/trade/diplomacy/messages/status/scores) are the
stage bodies.
