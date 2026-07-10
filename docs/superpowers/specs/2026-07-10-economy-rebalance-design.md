# Economy Rebalance — Design Spec

**Issue:** #4 (region income ~100× smaller than BRE's documented figures — scale decision)

**Status:** design, pending Andy's review. No code written yet.

> Drafted by Claude (Opus 4.8) at Andy's direction. Decisions in this spec were
> made by Andy in the brainstorming dialogue; the recovered BRE constants came
> from a disassembly of BRE.OVR (recorded below and in the
> `bre-binary-verified-math` memory).

## Goal

Bring Immortal Barons' region income up to Barren Realms Elite's actual
per-region figures, recovered from the binary, and fix two modeling errors
exposed along the way (Coastal support curve; Urban/Technology producing gold
they should not). Keep the whole money path safe on 32-bit builds, and format
the now-large numbers per the caller's locale.

## Background / why now

IB's region income (Coastal 25, Mountain 12, Desert 20, River 30 gold/region)
is ~1/150–1/200 of BRE's. The figures in `docs/mechanics-reference.md` were
**not** disassembly-derived, so they are not authoritative — this rebalance
replaces them with values read directly from the BRE.OVR turn-economy routine
and updates the doc.

The scales in IB today are **not** a uniform 1/100: income is ~1/150, unit
maintenance is ~10× BRE's guide figure, and the money/interest caps, net-worth
weights, and pirate caps table are BRE-*exact*. So there is no single
multiplier — this is a targeted rebalance to the recovered BRE numbers, not a
blanket scale.

## Decisions (locked in the brainstorming dialogue)

1. **Source of truth:** the BRE disassembly (below), not mechanics-reference.
   Update the doc from it.
2. **Direction:** full BRE-figure rebalance (not a magnitude-preserving bump).
3. **Income model:** BASE + a tunable random variance band (below).
4. **Money cap:** keep BRE's 2,000,000,000 cap; keep money fields as Go `int`.
   32-bit Win32 door binaries are in real use, so **int32 (max 2,147,483,647)
   is the hard ceiling** — no `int64` conversion, no raising the cap.
5. **Number display:** locale-aware thousands separator, keyed off
   `Empire.Language`.

## 1. Region income model

Recovered BRE formula (per region, per turn), replacing IB's flat
`scale(count × rate)`:

```
yield     = random in [YieldMin, YieldMax]   // tunable band; IB's fixed-seed RNG
perRegion = trunc(yield × RATE) + BASE
income   += perRegion × count
```

Per-region constants — read directly as immediate operands from the BRE.OVR
turn-economy routine (file offsets 0x342C0–0x34A4E; **HIGH** confidence;
cross-checked against `bre.doc`):

| Region | RATE | BASE | Special |
|---|---:|---:|---|
| Mountain (ore) | 400 | 3,550 | smallest RATE → most stable |
| Coastal (tourism) | 1,000 | 3,750 | × supportFactor (§1.1) |
| Desert (solar) | 2,000 | 3,000 | widest swing |
| Industrial | 100 | 2,500 | × IB's existing industry-efficiency factor |
| River (hydro) | 100 | 5,000 | highest base; occasional bad-year dud (§1.2) |
| Urban | — | — | **no gold** (housing/population only) |
| Technology | — | — | **no gold** (maintenance reduction only) |

Food output is unchanged in shape: River ×20 food/region, Agricultural ×5
food/region (also confirmed in the same routine).

### 1.1 Coastal support curve (modeling fix)

Recovered exactly (three Turbo-Pascal Real constants, byte-verified):

```
supportFactor = 0.10 + 0.90 × (Support / 100)
coastalGold   = trunc(yield × 1000 + 3750) × supportFactor × coastalCount
```

Floor is **0.10** — Coastal never drops to zero. At 0% support a region still
yields ~375 gold; at 100% support ~4,750. This **replaces** IB's current
`Coastal × 25 × Support/100`, which wrongly zeroes tourism at 0% support.

### 1.2 River bad-year dud

The River block has an early random gate that can skip hydro income (the
"most years, but sometimes less" behavior). Exact probability was not
recoverable; model as a small tunable chance of a reduced/zero River yield for
the turn.

### 1.3 Urban / Technology produce no direct gold (mechanic fix)

BRE gives Urban and Technology **no** direct gold. IB currently gives Urban ×8
and Technology ×15 gold/region — remove both. Their value comes through
existing systems:

- **Urban** — population housing capacity (already a weighted term in IB's
  population model).
- **Technology** — maintenance reduction (via `TechFactor`) and the Industrial
  industry-efficiency multiplier. This subsumes part of the open
  technology-mechanic issue (#20). Region maintenance not currently reduced by
  tech (a #20 gap) is in scope to fix here if cheap; otherwise it stays tracked
  under #20.

## 2. Prices, maintenance, tax

BRE's binary does **not** store unit prices or per-unit maintenance as
constants (computed inline), so these remain IB's reconstructed values,
re-anchored to the new scale. All flagged as reconstructed/tunable.

- **Prices:** scale IB's current unit / land / food / WMD / SDI / HQ gold costs
  up to the new income magnitude, **keeping IB's current ratios** (which match
  BRE: ~7 troopers per tank, ~6 jets per tank). Target: a unit costs a
  sensible fraction of a few regions' income, as today.
- **Maintenance:** **leave unchanged.** At ~5,000 gold/region income, IB's
  current per-unit upkeep becomes trivial — which is exactly BRE's balance
  (upkeep is a rounding error there). No change; it becomes BRE-like for free.
- **Tax / population:** IB keeps its own (small-scale) population formula
  (unchanged — a prior deliberate decision). Scale the tax coefficient
  (`People × Tax/100 × 8`) up so tax income stays a **meaningful** engine at
  the new scale (BRE calls population/tax "a major income engine"). The BRE
  coefficient was only partially recovered (shape `6 − f(tax)` × Population),
  so this value is reconstructed/tunable.

## 3. Int32 overflow safety

Money fields stay `int`; the cap stays 2,000,000,000; but at BRE scale
**intermediate products can overflow int32 even when the stored value cannot.**
Example (`turn.go:220`): `min(e.Bank, InterestCap) * InterestRate / 5000` —
`InterestCap` is 1,599,999,999, so `Bank × InterestRate` overflows int32 for
any `InterestRate ≥ 2` on a 32-bit build, wrapping before the `/5000`.

Audit every `gold × factor` and `gold + bank` that can exceed 2,147,483,647 at
cap scale, and widen the **intermediate** to `int64(a) * int64(b) / c` (fields
stay `int`; only the arithmetic widens). Known sites to check: the interest
step, income accumulation (`perRegion × count`), tax income, trade transfers,
bank deposit/withdraw against `MoneyCap`, and any total-wealth sum.

## 4. Number display

A helper `formatGold(n, lang)` groups thousands with the locale separator:

| Language | Separator | Example |
|---|---|---|
| English (en) | `,` | 1,847,392,104 |
| German (de) | `.` | 1.847.392.104 |
| Russian (ru) | space | 1 847 392 104 |

Keyed off `Empire.Language`, via a small per-language separator map (no
`x/text/message` CLDR dependency — a 3-entry map suffices). All three
separators are ASCII, so CP437-safe; in CP437 mode IB already forces English,
so it uses the comma. Applied on the Empire Status, Income Report, and scores
screens (wherever gold/bank/net-worth is shown).

## 5. Docs & tests

- **Update `docs/mechanics-reference.md`** economy section from the recovered
  constants (RATE/BASE per region, the Coastal support curve, Urban/Tech
  no-gold, food multipliers). Record the constants + the BRE.OVR offsets in the
  `bre-binary-verified-math` memory; resolve the `economy-scale-vs-bre` memory
  (decision made).
- **Tests:** many game-package tests assert specific gold amounts and will need
  updating to the new values. The yield band uses IB's fixed-seed RNG
  (`game.NewSeed`), so income stays deterministic in tests — expected values
  are computable. The plan sequences the test updates alongside each constant
  change so the suite stays green per step.

## Open items for review (recommended defaults)

1. **Yield band default** — the exact BRE distribution is unrecoverable
   (amplitude 1.5, in an unmapped helper segment). Recommend `[1.0, 1.5]`
   (income never below BASE); Andy may prefer a wider band to match "swings
   widely."
2. **Maintenance left as-is** (becomes trivial, BRE-style) vs. re-tuned.
3. **Tax coefficient scale factor** — reconstructed; pick a value that keeps
   tax a meaningful fraction of total income.

## Out of scope

- `int64` money fields / raising the cap (ruled out — 32-bit Win32 support).
- Changing IB's population formula (a prior deliberate decision).
- Exact BRE unit prices / maintenance (not stored as constants in the binary;
  reconstructed here).
- Interplanetary / news / net-worth-weight changes (BRE-exact; untouched).
