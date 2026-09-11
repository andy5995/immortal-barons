package game

// balance_regions.go — region income and the crown tax.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

// --- Region income (BRE-verified: BRE.OVR 0x342C0–0x34A4E) ---
//
// Per region, per turn: perRegion = Base + a uniform draw in [0, Rate), so Rate
// is the full width of the swing and Base its floor (see World.regionDraw, which
// mirrors BRE's Random(Rate) + Base). Each region type draws independently —
// there is no planet-wide year factor; in one live turn desert drew near the top
// of its band while agriculture drew zero.
//
// The band is live-verified across four region types in two separate games, by
// back-computing the draw from these Rate/Base pairs: desert 2.6–99.8% of Rate,
// tourism 0.4–99.8, ore 0.5–99.8, hydro 2.0–99.0. Two earlier readings were
// wrong and are recorded so nobody re-derives them: a reconstructed 1.0–1.5
// multiplier ran high, and a later 0.30–0.80 band came from 12 turns — the
// sample size at which any uniform looks narrow.
//
// Coastal is additionally multiplied by a support factor (see IncomeThisTurn);
// Urban and Technology produce NO direct gold (housing / maintenance-reduction
// only).
const (
	MountainRate, MountainBase = 400, 3550  // ore — smallest Rate, most stable
	CoastalRate, CoastalBase   = 1000, 3750 // tourism — × support factor
	DesertRate, DesertBase     = 2000, 3000 // solar — widest swing
	RiverRate, RiverBase       = 100, 5000  // hydro — highest base
)

// A River's hydropower has NO "bad year": IB carried a RiverDudChancePct of 10
// that halved the yield, and the production routine in the original binary has
// no such branch — its only fork is the Random(4) fishing roll. Removing it is
// what makes IB's river pay BRE's expected gold exactly (see riverGold).

// CrownTaxSupportPenalty bounds the popular support a baron loses by failing to
// pay the Queen's tax in full. It is a ceiling the penalty approaches but never
// reaches: the +1 on each side means paying nothing costs 14 points, not 15.
// Binary-verified (BRE.OVR 0x2FA90–0x2FAEE):
//
//	penalty = trunc((1 - (paid+1)/(required+1)) * CrownTaxSupportPenalty)
//
// BRE uses the same channel for region-maintenance shortfall but at 50, and
// docks morale rather than support at 40 for unpaid forces. The tax is the
// mildest of the three.
const CrownTaxSupportPenalty = 15
