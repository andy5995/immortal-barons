# Economy Rebalance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace IB's ~1/150-scale region income with BRE's disassembly-verified per-region figures, fix the Coastal support curve and the Urban/Technology gold error, keep the money path int32-safe, and format large numbers per locale.

**Data/code separation (decided with Andy):** all tunable balance numbers
live in one new file `internal/game/balance.go` — a declarative, commented Go
data file (a "config not changed at runtime": compiled in, type-safe, supports
the BRE-source/tuning notes JSON can't). The formula code references it. The
constants shown in the tasks below are *declared in balance.go* and referenced
from `turn.go`/`economy.go`/`specials.go`/`game.go`.

**Architecture:** The income math lives in `internal/game`. `IncomeThisTurn` is the single itemized income function (used for both the income-report display and `CollectIncome`), so it must stay a *pure, deterministic function of empire state + game day* — the per-turn "yield" variance is derived from a hash of (day, empire, region) so display and apply always agree without stored state. Prices and the tax coefficient scale by a single factor S=150 to stay proportionate to the new income; maintenance is left unchanged (it becomes trivial, matching BRE). Display uses the existing `comma()` helper in `internal/menu`, extended to a locale-aware `formatGold`.

**Tech Stack:** Go 1.26, standard library. `internal/game` (economy), `internal/menu` (display). Fixed-seed RNG (`game.NewSeed`) keeps tests deterministic.

## Global Constraints

- **int32 is the hard ceiling** (2,147,483,647): 32-bit Win32 door builds are in scope. Money fields stay Go `int`; `MoneyCap` stays 2,000,000,000. Any `gold × factor` / `gold + bank` that can exceed int32 at cap scale must use an `int64(a)*int64(b)/c` intermediate; storage stays `int`.
- **Source of truth is the BRE disassembly** (constants below), not `docs/mechanics-reference.md` — update the doc from this plan.
- **Recovered BRE per-region constants (HIGH confidence, BRE.OVR 0x342C0–0x34A4E):** Mountain RATE 400 BASE 3550; Coastal RATE 1000 BASE 3750 (× supportFactor); Desert RATE 2000 BASE 3000; Industrial RATE 100 BASE 2500; River RATE 100 BASE 5000. Coastal `supportFactor = 0.10 + 0.90×(Support/100)`. Urban & Technology: **no gold**. Food: River ×20, Agricultural ×5.
- **Yield band:** `yield ∈ [1.0, 1.5]`, tunable (`YieldMin=100, YieldMax=150` as integer percents). Deterministic per (day, empire, region).
- **Scale factor S=150** for prices and the tax coefficient (region income uses BRE absolute numbers).
- **Deferred to #20 (out of scope here):** the tech-factor's role as an income multiplier (the existing `scale()` wrapper stays as-is); region-maintenance tech reduction.
- Run `gofmt -w .` and `go vet ./...` before each commit; keep `go test ./...` green per task.
- No BRE code/text copied into the repo; constants only.

---

### Task 1: Region income core (BRE RATE/BASE + yield + Coastal curve; remove Urban/Tech gold)

**Files:**
- Modify: `internal/game/turn.go` (`IncomeThisTurn` ~185-200; `IncomeBreakdown` struct 172-181; add income constants + yield helper)
- Modify: `internal/game/regions.go` (`RegionMix.income()` — remove; it is test-only)
- Modify: `internal/game/regions_test.go` (drop the `income()` test)
- Test: `internal/game/turn_test.go`

**Interfaces:**
- Produces: `func (w *World) regionYield(e *Empire, salt int) int` → integer percent in [YieldMin, YieldMax], deterministic from (w.Day, e.Name, salt). `IncomeThisTurn` unchanged signature (`func (w *World) IncomeThisTurn(e *Empire) IncomeBreakdown`), still pure/deterministic.

- [ ] **Step 1: Add constants** near the income code in `turn.go`:

```go
const (
	YieldMin = 100 // percent; yield band lower bound (income = BASE at yield=1.0)
	YieldMax = 150 // percent; upper bound (BRE's ~1.5 amplitude, reconstructed/tunable)

	// BRE-verified per-region income (BRE.OVR 0x342C0-0x34A4E): perRegion = yield*RATE + BASE.
	MountainRate, MountainBase = 400, 3550
	CoastalRate, CoastalBase   = 1000, 3750
	DesertRate, DesertBase     = 2000, 3000
	IndustrialRate, IndustrialBase = 100, 2500
	RiverRate, RiverBase       = 100, 5000
)
```

- [ ] **Step 2: Add the deterministic yield helper** to `turn.go`. It must NOT consume the world RNG (that would make repeated `IncomeThisTurn` calls differ); derive from a per-(day,empire,region) hash:

```go
// regionYield returns this turn's income yield (percent, YieldMin..YieldMax)
// for empire e's region type identified by salt. Deterministic in (w.GameDay,
// e.Name, salt) so IncomeThisTurn is pure — display and apply always agree
// (variance is per game-day, i.e. a good/bad "year" lasts the day).
func (w *World) regionYield(e *Empire, salt int) int {
	h := fnv.New32a()
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:4], uint32(w.GameDay))
	binary.LittleEndian.PutUint32(buf[4:8], uint32(salt))
	h.Write(buf[:])
	io.WriteString(h, e.Name)
	span := YieldMax - YieldMin + 1
	return YieldMin + int(h.Sum32()%uint32(span))
}
```
(add imports `hash/fnv`, `encoding/binary`, `io`.)

- [ ] **Step 3: Rewrite `IncomeThisTurn` region terms.** Remove `Urban` and `Technology` fields from `IncomeBreakdown` and from `Gold()`. Keep the `scale()` tech wrapper (deferred to #20). Salts: Mountain=1, Coastal=2, Desert=3, River=4, Industrial=5.

```go
func (w *World) IncomeThisTurn(e *Empire) IncomeBreakdown {
	tf := e.TechFactor()
	scale := func(n int) int { return n * (100 + tf) / 100 }
	perRegion := func(salt, rate, base int) int { return w.regionYield(e, salt)*rate/100 + base }

	support := 10 + 90*e.Support/100 // supportFactor ×100: 0.10 + 0.90*(Support/100)
	return IncomeBreakdown{
		Taxes:      scale(e.People * e.Tax / 100 * TaxGoldPerCapita), // TaxGoldPerCapita from Task 4
		Ore:        scale(perRegion(1, MountainRate, MountainBase) * e.Regions.Mountain),
		Tourism:    scale(perRegion(2, CoastalRate, CoastalBase) * support / 100 * e.Regions.Coastal),
		Solar:      scale(perRegion(3, DesertRate, DesertBase) * e.Regions.Desert),
		Rivers:     scale(w.riverGold(e) * e.Regions.River),
		Industrial: scale(w.industrialGold(e) * e.Regions.Industrial), // efficiency in Task 2
		Trade:      w.tradeIncome(e),
		Food:       e.Regions.foodProduced(),
	}
}
```
(Note: `int64` widening for the `*count` products is added in Task 5; here keep readable. `riverGold`/`industrialGold` are Task 2 helpers — for this task stub `riverGold` = `perRegion(4,RiverRate,RiverBase)` and `industrialGold` = `perRegion(5,IndustrialRate,IndustrialBase)`, refined in Task 2.)

- [ ] **Step 4: Remove `RegionMix.income()`** from `regions.go` and its assertion in `regions_test.go` (it duplicated the weights; `IncomeThisTurn` is now the single source). Remove the "weights mirror RegionMix.income()" comment.

- [ ] **Step 5: Write/adjust tests** in `turn_test.go`. Because yield is deterministic, assert exact values for a fixed world/day. Example: a realm with 10 Desert regions at Support 100, tf 0, day D: `Solar = (regionYield*2000/100 + 3000) * 10`. Compute the expected `regionYield` for that (day, name) or assert the value is in `[10*(1.0*2000+3000), 10*(1.5*2000+3000)] = [50000, 60000]`. Add a test that Coastal at Support 0 is non-zero (floor): `Tourism ≈ perRegion*0.10*count > 0`. Add a test that Urban/Technology regions contribute **zero** gold.

- [ ] **Step 6:** `gofmt -w . && go vet ./... && go test ./internal/game/ -run Income -v` → PASS.

- [ ] **Step 7: Commit** `feat(economy): BRE-verified region income + Coastal support floor; drop Urban/Tech gold`.

---

### Task 2: Consolidate Industrial gold (remove the double-count), River dud

**Files:**
- Modify: `internal/game/turn.go` (`Manufacture` ~393-396; add `industrialGold`, `riverGold`)
- Test: `internal/game/turn_test.go`

**Interfaces:**
- Produces: `func (w *World) industrialGold(e *Empire) int` (BRE Industrial perRegion × industry-efficiency), `func (w *World) riverGold(e *Empire) int` (BRE River perRegion with bad-year dud).

- [ ] **Step 1:** Industrial gold is currently paid **twice** — once in `IncomeThisTurn.Industrial` and again as `IndustryGold = Industrial * IndustryGoldPerRegion(250)` in `Manufacture`. Consolidate: `industrialGold` returns the BRE per-region value scaled by the same industry-efficiency modifier `Manufacture` uses; **remove** the `e.Gold += e.IndustryGold` credit from `Manufacture` (Manufacture now produces units only). Keep `e.IndustryGold` field set (for the income-report "Industry" line — verify Task 6/display), sourced from `industrialGold(e)*Industrial`.

```go
func (w *World) industrialGold(e *Empire) int {
	base := w.regionYield(e, 5)*IndustrialRate/100 + IndustrialBase
	return base * (100 + e.industryEfficiencyPct()) / 100 // reuse existing efficiency; if none, factor=100
}
```
(If no standalone `industryEfficiencyPct` exists, inline the same modifier `ProjectedProduction`/`Manufacture` applies; keep it a single helper — DRY.)

- [ ] **Step 2:** `riverGold` adds BRE's bad-year dud: a small deterministic chance (derive from `regionYield(e, 40)`) that River yields only its BASE (no swing) or a reduced value:

```go
func (w *World) riverGold(e *Empire) int {
	if w.regionYield(e, 40) < YieldMin+5 { // ~10% bad year, tunable
		return RiverBase / 2
	}
	return w.regionYield(e, 4)*RiverRate/100 + RiverBase
}
```

- [ ] **Step 3:** Delete `IndustryGoldPerRegion` const if now unused (grep first). Update any income-report display that read `IndustryGold` to keep working.

- [ ] **Step 4: Tests** — assert Industrial regions pay gold exactly once (a realm with N Industrial regions and no others: `CollectIncome` credits `industrialGold*N`, and `Manufacture` credits **0** gold). Assert River dud path.

- [ ] **Step 5:** `gofmt -w . && go vet ./... && go test ./internal/game/ -run 'Industr|River|Manufacture' -v` → PASS.

- [ ] **Step 6: Commit** `fix(economy): single-count Industrial gold; River bad-year variance`.

---

### Task 3: Scale prices ×S to the new income magnitude

**Files:**
- Modify: `internal/game/game.go` (`Prices{...}` default ~290)
- Modify: `internal/game/economy.go` (`HQCost`, `FoodBuyPrice`, `FoodSellPrice`)
- Modify: `internal/game/specials.go` (`NukeCost`, `ChemCost`, `BioCost`, `DoomerCost`, `SDIStep`)
- Test: `internal/game/economy_test.go`, `specials_test.go`

**Interfaces:** Consumes nothing new; changes constant values. Ratios preserved (~7 troopers/tank, ~6 jets/tank).

- [ ] **Step 1:** Set prices (clean values near ×150, ratios preserved):

```go
Prices{Land: 15000, Food: 300, Trooper: 7500, Jet: 9000, Turret: 9000,
	Tank: 50000, Carrier: 6000, Agent: 15000, Bomber: 30000}
```

- [ ] **Step 2:** Costs (×150, rounded):

```go
HQCost = 750000
FoodBuyPrice = 3000
FoodSellPrice = 1000
NukeCost = 7500000
ChemCost = 6000000
BioCost  = 6000000
DoomerCost = 75000000
SDIStep  = 1500000 // gold per +1% SDI
```
(`LandPriceStep = 50` is a divisor ratio — leave unchanged.)

- [ ] **Step 3:** Update the many gold-amount assertions in `economy_test.go` / `specials_test.go` to the new values. Prefer asserting *derived from the constants* (e.g. `n * FoodBuyPrice`) rather than hardcoded literals where the test allows, so future tuning doesn't re-break them.

- [ ] **Step 4:** `gofmt -w . && go vet ./... && go test ./internal/game/ -run 'Buy|Cost|Food|Land|HQ|Nuke|Chem|Bio|Doomer|SDI' -v` → PASS.

- [ ] **Step 5: Commit** `balance(economy): scale prices/costs to BRE income magnitude`.

---

### Task 4: Scale the tax coefficient (extract constant)

**Files:**
- Modify: `internal/game/turn.go` (the inline `* 8` in `IncomeThisTurn.Taxes`; add `TaxGoldPerCapita`)
- Test: `internal/game/turn_test.go`

- [ ] **Step 1:** Extract the tax coefficient to a named, tunable constant and scale ×S so tax stays the meaningful engine it already is (currently ~= region income for a small realm):

```go
const TaxGoldPerCapita = 1200 // was inline 8; ×150 to track the new income scale (tunable — top playtest knob)
```
Reference it in `IncomeThisTurn` (already wired in Task 1 Step 3).

- [ ] **Step 2:** Test: a realm of People=2000, Tax=7, tf=0 → `Taxes = 2000*7/100*1200 = 168000`. Assert exact.

- [ ] **Step 3:** `gofmt -w . && go vet ./... && go test ./internal/game/ -run Tax -v` → PASS.

- [ ] **Step 4: Commit** `balance(economy): scale tax income to new economy (TaxGoldPerCapita)`.

---

### Task 5: int32-overflow safety (int64 intermediates)

**Files:**
- Modify: `internal/game/turn.go` (interest step ~220; income `*count` products), `bank.go`, `trade.go`, `economy.go` (MoneyCap adds), plus any total-wealth sum found by audit.
- Test: `internal/game/bank_test.go`, `turn_test.go`

- [ ] **Step 1: Audit.** `grep -rn 'InterestCap\|MoneyCap\|Bank \*\|Gold \*\|\* InterestRate\|Bank +\|Gold +' internal/game/*.go`. For each `a*b` (or `a+b`) that can exceed 2,147,483,647 with a,b at cap scale, widen the intermediate.

- [ ] **Step 2:** Interest step (`turn.go:220`):
```go
e.Bank += int(int64(min(e.Bank, InterestCap)) * int64(w.Config.InterestRate) / 5000)
```

- [ ] **Step 3:** Income `perRegion * count`: at cap-scale region counts the product stays < int32, but a full realm's summed `Gold()` across sources can approach cap — ensure no single intermediate (e.g. `perRegion * count` for a very large realm, or `People * TaxGoldPerCapita`) overflows; widen the tax product: `int(int64(e.People) * int64(e.Tax) / 100 * TaxGoldPerCapita)` if `People` can be large. Cap-clamp `e.Gold`/`e.Bank` to `MoneyCap` after crediting (existing behavior — verify).

- [ ] **Step 4:** Add a regression test that simulates a near-cap Bank + high InterestRate and asserts the interest result is correct (would be negative/garbage under int32 wraparound). Build-tag or comment it as the 32-bit-safety guard.

- [ ] **Step 5:** `gofmt -w . && go vet ./... && go test ./internal/game/ -v` → PASS. Cross-check 32-bit build: `GOARCH=386 go build ./...` → success.

- [ ] **Step 6: Commit** `fix(economy): int64 intermediates for 32-bit int overflow safety`.

---

### Task 6: Locale-aware number formatting (`formatGold`)

**Files:**
- Modify: `internal/menu/input.go` (`comma()` ~180) — extend or add `formatGold`
- Modify: callers that show gold/bank/net-worth: `internal/menu/empire_status_screen.go` (35-37), `actions_info.go` (178-179), `menu.go` (347,350)
- Test: `internal/menu/*_test.go` (add a format test)

**Interfaces:**
- Produces: `func formatGold(n int, lang string) string` — groups thousands with the locale separator (en `,` / de `.` / ru ` `). `comma(n)` becomes `formatGold(n, "")` (English default).

- [ ] **Step 1:** Add a separator map + `formatGold`:
```go
var groupSep = map[string]byte{"": ',', "en": ',', "de": '.', "ru": ' '}

func formatGold(n int, lang string) string {
	sep, ok := groupSep[lang]
	if !ok { sep = ',' }
	// ... same grouping as comma(), using sep ...
}
func comma(n int) string { return formatGold(n, "") }
```

- [ ] **Step 2:** At the money-display call sites, pass the caller's language. Use the existing per-session language (the `langSession`/`playerLang` mechanism — `empire_status_screen.go` already has the player `p`, so `formatGold(p.Gold, p.Language)`; in CP437 mode `playerLang` yields English, so the comma is used). Where only `comma(x)` is available without a language in scope, thread the language or leave `comma` (English) — do not force a language that isn't available.

- [ ] **Step 3:** Test: `formatGold(1847392104, "de") == "1.847.392.104"`, `formatGold(1847392104, "ru") == "1 847 392 104"`, `formatGold(-1234567, "en") == "-1,234,567"`.

- [ ] **Step 4:** `gofmt -w . && go vet ./... && go test ./internal/menu/ -v` → PASS.

- [ ] **Step 5: Commit** `feat(menu): locale-aware thousands separator for gold display`.

---

### Task 7: Update docs + memories; resolve #4

**Files:**
- Modify: `docs/mechanics-reference.md` (economy/region section — replace the non-RE figures)
- Modify: memory `bre-binary-verified-math.md` (add the recovered income constants + offsets), `economy-scale-vs-bre.md` (mark resolved), `MEMORY.md` pointer if needed
- Modify: `CLAUDE.md` "Known scale gap" note (remove/soften — resolved)

- [ ] **Step 1:** Rewrite the mechanics-reference region-income section with the RATE/BASE table, the Coastal support curve (`0.10 + 0.90·support`), Urban/Tech no-gold, food multipliers, and a note that these are disassembly-verified (BRE.OVR 0x342C0–0x34A4E). Note the reconstructed items (yield band, price scale S=150, `TaxGoldPerCapita`).
- [ ] **Step 2:** Add the constants to `bre-binary-verified-math`; update `economy-scale-vs-bre` to "resolved (#4): rebalanced to BRE figures".
- [ ] **Step 3:** Update `CLAUDE.md` — remove the "region income is ~100× smaller" known-gap line.
- [ ] **Step 4:** `go test ./...` full suite green.
- [ ] **Step 5: Commit** `docs(economy): mechanics-reference to BRE-verified income; resolve #4 scale gap`.

---

## Self-Review

- **Spec coverage:** §1 income model → Tasks 1,2. §1.1 Coastal curve → Task 1. §1.2 River dud → Task 2. §1.3 Urban/Tech no-gold → Task 1. §2 prices/maintenance(unchanged)/tax → Tasks 3,4. §3 int32 safety → Task 5. §4 display → Task 6. §5 docs/tests → Task 7 + per-task tests. Covered.
- **Ambiguity resolved inline:** yield determinism (hash of day/empire/region, not RNG draw) so display==apply; Industrial single-count; `income()` removed; tech-factor income role explicitly deferred to #20.
- **Reconstructed/tunable values flagged:** YieldMin/Max, price scale S=150, `TaxGoldPerCapita`, River dud probability. All are named constants for easy playtest tuning.
- **Out of scope:** int64 money, cap raise, population formula, exact BRE prices/maintenance, tech-factor income-multiplier role (#20).
