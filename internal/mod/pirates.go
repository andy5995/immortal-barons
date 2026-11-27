package mod

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/session"
)

// namesKey is the one key of [pirates] that is not a whole number: the nine
// faction names, in slot order.
const namesKey = "names"

// applyPirates writes the [pirates] section's keys into r. A name must be
// text a CP437 door can show; its length, characters and uniqueness are
// Rules.Validate's, checked once every mod has been applied.
func (v values) applyPirates(r *game.Rules) error {
	figures := map[string]any{}
	for k, x := range v.Pirates {
		if k != namesKey {
			figures[k] = x
		}
	}
	errs := []error{applyFields("pirates", game.PirateFields, figures, &r.PirateFigures)}
	if raw, ok := v.Pirates[namesKey]; ok {
		names, err := readNames(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("pirates.%s: %w", namesKey, err))
		} else {
			r.PirateFigures.Names = names
		}
	}
	return errors.Join(errs...)
}

// readNames reads the names array: a list of text a CP437 door can show.
func readNames(raw any) ([]string, error) {
	list, ok := raw.([]any)
	if !ok {
		return nil, errors.New("want a list of names")
	}
	var names []string
	for i, x := range list {
		s, ok := x.(string)
		switch {
		case !ok:
			return nil, fmt.Errorf("name %d is not text", i)
		case !session.CP437Encodable(s):
			return nil, fmt.Errorf("name %d (%s) has characters a CP437 door cannot show; use Latin letters", i, strconv.Quote(clean(truncate(s, maxShown))))
		}
		names = append(names, s)
	}
	return names, nil
}

// diffPirates lists the [pirates] values that differ between a and b.
func diffPirates(a, b *game.Rules) []string {
	out := diffFields("pirates", game.PirateFields, a.PirateFigures, b.PirateFigures)
	if !slices.Equal(a.PirateFigures.Names, b.PirateFigures.Names) {
		out = append(out, fmt.Sprintf("pirates.%s: %s -> %s", namesKey, nameList(a), nameList(b)))
	}
	return out
}

// nameList is the names as a values.toml writes them, cleaned for a screen.
func nameList(r *game.Rules) string {
	q := make([]string, len(r.PirateFigures.Names))
	for i, n := range r.PirateFigures.Names {
		q[i] = strconv.Quote(clean(n))
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// writePirates prints the [pirates] section as a values.toml would set it.
func writePirates(b *strings.Builder, r *game.Rules) {
	writeFields(b, "pirates", game.PirateFields, r.PirateFigures)
	fmt.Fprintf(b, "%s = %s\n", namesKey, nameList(r))
}
