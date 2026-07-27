# BRE build-up / economy strategy (for staging test scenarios)

**Purpose.** How to build a BRE empire up (regions, income, military)
efficiently when driving the original binary headless to stage a test scenario —
for example an attacker versus an allied pair. This is play/economy technique for
setting up test worlds, not a mechanics spec (numbers live in
`docs/mechanics-reference.md`) and not emulator plumbing (driving/scraping lives
in the `bre-gather` skill).

## Tax

Set **12%**. It never triggers tax riots, still earns tax income, and keeps
popular support near 100% so Coastal output does not slump.

## Region priority

Buy in this order:

1. **Coastal** first — Tourism is the cash engine. Going 3 -> 33 Coastal took
   Tourism from ~13k to ~146k gold/turn in a single turn; it compounds hard.
2. **Agricultural** — food for a large empire and army.
3. **Industrial** — needed to PRODUCE military units, especially tanks.

Coastal output "slumps drastically" at low public support, so keep tax low and
support high (see Tax).

## Buy cadence (per the project owner)

Do NOT buy a little every turn. Bootstrap with a **max-Coastal buy early** to
kick off Tourism compounding, THEN accumulate gold across several turns and
**bulk-buy in large batches**. Rationale: region prices ratchet up as you buy —
both within one purchase and persistently across the turn — so spacing out many
small buys pays the rising price repeatedly.

## Turn economy

20 turns/day. Use the end-of-turn `Do you wish to continue? (Y/n)` prompt — `Y`
chains straight into the next turn without returning to the main menu. When turns
run out, advance the DOS `DATE` by a day and relaunch to refill 20 more.

## Producing military

Two paths:

- **Industrial regions + System Menu -> Set Industries** (set the Tanks %)
  produce units per turn.
- **Buy units directly for gold** at the Spending Menu (Tanks ~2163 gold each at
  base).

Which is faster depends on Industrial count versus available gold — a large
Industrial base out-produces direct buys over several turns, while a gold surplus
buys a batch instantly.

## Snapshotting

Full-directory snapshots of the game data (see the bre-snapshot-workflow) let you
save a built-up world for reuse. BRE's `game.dat` has integrity checks, so you
cannot byte-edit it — build the world in-game, then snapshot the directory.

## Verified numbers (live run, 2026-07-27, BRE v0.988)

Measured while building three empires headless:

- **Turns:** 20 per empire per day. Each empire has its own counter (they do not
  share). When exhausted, advance the DOS `DATE` a day for 20 more.
- **Land-market price ratchet is PER-EMPIRE, not global.** One empire buying ~750
  regions pushed *its own* region price from 1,412 to ~27,020, while a second
  empire in the same world still started at 1,412. So one empire's expansion does
  not inflate another's land cost.
- **Region marginal price rises within a purchase and ratchets up persistently**
  for that empire ("the price shown is only for the first piece"). Buy in the
  order Coastal → Agricultural → (Industrial), and expect the price to climb.
- **Coastal → Tourism compounding (with 100% support, tax 12%):**
  3 Coastal ≈ 13k Tourism/turn; 33 Coastal ≈ 146k; ~547 Coastal ≈ ~2M;
  679 Coastal ≈ 2.9M gold/turn. Coastal is the whole economy.
- **Tank price is FLAT (~2,027–2,164 each) — no ratchet.** Buying is purely
  gold-limited, so 10k tanks ≈ ~20–21M gold. At ~3M gold/turn income that is
  ~7 turns of max-buying. This makes DIRECT PURCHASE the fast path to a big tank
  force (no Industrial regions needed) once Coastal income is large.
- **Food:** population grows fast as you buy regions; food demand scales with it.
  With only the 2 starting Agricultural regions a rapid Coastal build WILL hit a
  shortage. Buy ~100 Agricultural early (≈30k food/turn, comfortably ahead even
  at ~3B population), or let the driver buy food from the market as backup.
- **Empire letters** are assigned per enrollment order (1st enrolled = A, 2nd = B, …).
- **Per-turn prompts** a blind macro must handle: the Diplomacy menu opens EVERY
  turn (not just the first); a "Queen Royale requires N gold for Taxes" prompt
  appears some turns; underpaying food/taxes triggers a "DISASTEROUS results —
  reconsider? (Y/n)" warning. Drive by matching the active screen, never a fixed
  key count.
