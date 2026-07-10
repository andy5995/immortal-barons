package menu

import (
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestRegionFieldOrdering ensures regionTypeNames, regionTypeKeys, and
// regionField all agree on the field order, so a future insertion can't
// silently mislabel a region type.
func TestRegionFieldOrdering(t *testing.T) {
	// Assert slice lengths match.
	if len(regionTypeNames) != 8 {
		t.Errorf("regionTypeNames len = %d, want 8", len(regionTypeNames))
	}
	if len(regionTypeKeys) != 8 {
		t.Errorf("regionTypeKeys len = %d, want 8", len(regionTypeKeys))
	}
	// Selection keys must be unique, or a letter would pick the wrong type.
	seen := map[byte]bool{}
	for _, k := range regionTypeKeys {
		if seen[k] {
			t.Errorf("duplicate region key %q", k)
		}
		seen[k] = true
	}

	// For each region type, verify regionField points to the correct struct field.
	for i, name := range regionTypeNames {
		t.Run(name, func(t *testing.T) {
			p := &game.Empire{}
			initial := p.Regions.Total()

			// Increment via regionField.
			field := regionField(p, i)
			*field++

			// Verify Total increased by exactly 1.
			final := p.Regions.Total()
			if final != initial+1 {
				t.Errorf("Total: %d -> %d, want +1", initial, final)
			}

			// Verify the correct field was incremented by checking the
			// named field directly.
			switch name {
			case "Coastal":
				if p.Regions.Coastal != 1 {
					t.Errorf("Coastal = %d, want 1", p.Regions.Coastal)
				}
			case "Mountain":
				if p.Regions.Mountain != 1 {
					t.Errorf("Mountain = %d, want 1", p.Regions.Mountain)
				}
			case "Desert":
				if p.Regions.Desert != 1 {
					t.Errorf("Desert = %d, want 1", p.Regions.Desert)
				}
			case "River":
				if p.Regions.River != 1 {
					t.Errorf("River = %d, want 1", p.Regions.River)
				}
			case "Agricultural":
				if p.Regions.Agricultural != 1 {
					t.Errorf("Agricultural = %d, want 1", p.Regions.Agricultural)
				}
			case "Urban":
				if p.Regions.Urban != 1 {
					t.Errorf("Urban = %d, want 1", p.Regions.Urban)
				}
			case "Industrial":
				if p.Regions.Industrial != 1 {
					t.Errorf("Industrial = %d, want 1", p.Regions.Industrial)
				}
			case "Technology":
				if p.Regions.Technology != 1 {
					t.Errorf("Technology = %d, want 1", p.Regions.Technology)
				}
			default:
				t.Errorf("unknown region type: %s", name)
			}
		})
	}
}
