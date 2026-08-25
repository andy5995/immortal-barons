package game

// balance_start.go — the new-realm starting setup.
// Split out of balance.go; the provenance rules in that file's header
// apply here too, and each section below carries its own.

// --- New-realm starting setup ---
//
// A fresh realm's regions and units. The region mix, trooper count, and food
// are BRE-verified from a live BRE new-empire screen (2 Agricultural, 5 Desert,
// 5 Mountain, 3 Coastal = 15 regions; 100 troopers; 1000 food; full morale, no
// other units). Gold, population, and tax are IB's own start values.
const (
	StartAgricultural = 2
	StartDesert       = 5
	StartMountain     = 5
	StartCoastal      = 3
	StartTroopers     = 100
	// StartRegions is the land a new realm opens with, and the only thing besides
	// its troopers that its net worth is made of — which is what ScorePerTurn is
	// derived from.
	StartRegions = StartAgricultural + StartDesert + StartMountain + StartCoastal
	StartFood    = 1000
	StartGold    = 10000
	StartPeople  = 2000
	StartTax     = 15
)
