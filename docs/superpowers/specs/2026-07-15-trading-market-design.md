# General Trading Market — design (#17, Phase 1)

Status: draft for review. Scope: the **general Trading Market** only. Negotiated
empire-to-empire **trade deals with goods + demands** (BRE's other trading half)
are Phase 2, a separate spec.

## Why

IB's trading is a stub: "Send Trade Deal" only sends gold, and there is no
general market. BRE's Trading menu has two parts — trade deals and a *general
market* where "you also can put items for sale on a general market at any price
you choose" (`breins.txt`). The market is the payoff for Industry
Specialization. This spec builds the market.

## Ground truth (BRE, gathered live 2026-07-15 via dosemu + `capture-pane -e`)

- **Menu path:** `System → Trading → (2) Trading Market`. (Trading submenu:
  `(1) Trading` = trade deals, `(2) Trading Market` = the general market.)
- **Goods traded:** Troopers, Jets, Turrets, Bombers, **Food**, **Agents**,
  Tanks, Carriers. NOT Regions or HeadQuarters (key 6 absent). Units + food +
  agents.
- **Setup screen columns:** `Key · Name · Your Prices · Owned · For Sale · Total
  For Sale`. Per item you set your own sell price and a quantity for sale.
- **Listing escrows the goods:** setting 50 troopers For Sale dropped Owned
  100→50 — the listed units leave your inventory and sit in the market.
- **Pull goods back** via `[C] Change your setup` (set For Sale lower / to 0);
  you cannot "unsell" any other way.
- **Buy flow:** pick an item → `[B] Buy from Market` → `Choose a Target [A-Y]`
  → browse `Id · Empire Name · For Sale · Price` → buy from a specific empire's
  listing. **You cannot buy your own listing** (BRE silently refuses).
- **Total For Sale** = the planet-wide pool for that item (sum of all empires').
- **Protection-gated:** a realm under new-realm protection cannot enter the
  market ("Your dominion is still under protection.").
- **Settlement:** a daily-maintenance step "Depositing trading market money"
  settles proceeds. Exact commission/timing NOT observed (blocked: needs two
  non-protected empires; self-buy refused). Treated as an IB tunable.
- **Colors** (captured, for the UI match): setup table — parens yellow(33), key
  bright-yellow(93), name white(37), values bright-cyan(96)/bright-yellow(93),
  separator red(91). Buy browse — `Choose a Target` white with red brackets,
  bright-yellow keys; separator yellow(33). Prompts — white text, bright-cyan
  names/values, blue `(default)` parens.

## Data model

A planet-wide market of per-empire listings, held on `World` (persisted in
world.json):

```go
// One empire's listing of one good on the general market.
type MarketListing struct {
    Owner string // empire Owner handle
    Good  string // "trooper","jet","turret","bomber","food","agent","tank","carrier"
    Qty   int    // units escrowed and offered
    Price int    // gold per unit, seller-set
}
// World.Market []MarketListing  (empty slice = nothing listed)
// World.MarketProceeds map[string]int  (owner -> gold owed, paid at day-end)
```

- **Escrow:** listing moves `Qty` out of the empire's unit field into the
  listing. Delisting (Change setup to a lower Qty) returns the difference.
- Goods and their empire fields reuse the existing unit accessors; food and
  agents are included.

## Flows

1. **Trading Market screen** (`internal/menu`, new `tradingMarket` action under
   the Trading submenu): render the `Your Prices / Owned / For Sale / Total For
   Sale` table for the player, matching BRE's columns and captured colors. Gate
   on protection (reuse the Attack-menu `Protection > 0` pattern).
2. **Change setup** (`[C]`): pick good → prompt new For Sale qty (max = Owned +
   currently listed) → prompt price. Adjust escrow: move goods in/out of the
   listing so Owned + For Sale is conserved.
3. **Buy** (`[B]`): pick good → list empires with a live listing for it (`Id ·
   Empire · For Sale · Price`, excluding the buyer) → pick target → prompt qty
   (max = min(their For Sale, gold/price)) → transfer: buyer gold -=
   qty*price, buyer's good += qty, seller's listing Qty -= qty, seller proceeds
   credited (see settlement). Refuse buying your own listing.
4. **Settlement (tunable):** buyer pays the full price immediately (verified live
   — no buyer-side markup); the seller's proceeds accrue in `World.MarketProceeds`
   and are deposited at **day-end maintenance** (`settleMarketProceeds` in
   DailyMaintenance — BRE's "Depositing trading market money" is a sysop-
   maintenance step, verified live), minus a `MarketCommissionPct` tunable
   (default 0 — BRE's exact cut was too noisy to isolate live).

**Anti-exploit (implemented):** escrowed goods are safe from pirates/attacks —
an *intended* BRE strategy (Cash-On-Wheels FAQ #41: park military to evade
pirates). But listing must not dodge your own economy: escrowed **military units
still cost maintenance** (`listedForcesUpkeep` in ForcesDue) and escrowed **food
still spoils** (`spoilListedFood` in processEconomy). So the market protects from
outside threats only, not from upkeep/decay.

## Config / balance (balance.go)

- `MarketCommissionPct = 0` — cut the market takes on a sale (tunable; BRE's
  exact value unobserved).
- Goods list + their unit-field accessors reused from the Spending/Sell menus.

## BombTradingMarket

Today it abstracts a gold loss. Rebuild it to hit the target's **real market
position**: destroy a share of the target's listed goods (escrowed Qty) and/or
pending `MarketProceeds`, per the existing covert-op damage pattern. Values in
balance.go.

## Protection gating (confirmed with Andy)

Add the Trading Market to the protection gate: a realm with `Protection > 0`
cannot list or buy (message reconstructed in IB's words). Trade deals (Phase 2)
inherit the same gate.

## Testing

- Escrow conservation: list N, Owned drops N, delist returns N; Owned + listed
  invariant holds.
- Buy transfers goods and gold correctly; cannot buy own listing; cannot exceed
  For Sale or affordability.
- Settlement pays proceeds at day-end, applies commission, empties the pool.
- Protection blocks list/buy.
- Concurrency: two empires trading under the file store (mirror
  `TestCrossProcessConcurrentPlay`).

## Deliberate divergences from BRE

- Settlement commission is an IB tunable (BRE's exact value not observed).
- Number formatting and column widths are locale-aware (see "Menu fidelity vs
  BRE" in mechanics-reference.md).

## Out of scope (Phase 2)

Negotiated empire-to-empire trade deals carrying goods with demands in return +
async accept/reject (mirror `TreatyOffers`/Mail). Separate spec.
