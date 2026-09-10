package menu

import (
	"fmt"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/session"
)

// timezone.go — the Preferences time-zone picker (#267). Stamps are stored in
// UTC and rendered on the reader's clock, and this is where a player says which
// clock that is.
//
// Two answers are offered before any list, because they are the two most players
// want and neither asks them to know an IANA name: UTC, which every board in a
// league agrees on, and the board's own clock, which is what the game day turns
// on. The list is for a player who is not near the board they call.

// pickTimeZone asks which clock this player reads times on.
func pickTimeZone(s session.Session, w *ctx) Result {
	now := time.Now()
	items := []string{
		fmt.Sprintf(tr(s, "UTC — one clock for the whole league (%s)"), now.UTC().Format("15:04")),
		fmt.Sprintf(tr(s, "This board's clock (%s)"), now.Format("15:04 MST")),
		tr(s, "Choose a zone…"),
	}
	switch chooseFromList(s, w.Plain, tr(s, "Show times in:"), items, tr(s, "Keep current")) {
	case 0:
		return setTimeZone(s, w, "")
	case 1:
		return setTimeZone(s, w, game.LocalZone)
	case 2:
		return pickZoneFromList(s, w)
	}
	return Stay
}

// pickZoneFromList walks the region list, then the zones under it. The last
// entry of every zone list types a name instead: the picker carries the common
// zones, not all six hundred, and a player whose own is missing must not be
// stuck with somebody else's.
func pickZoneFromList(s session.Session, w *ctx) Result {
	regions := make([]string, 0, len(game.ZoneRegions)+1)
	for _, r := range game.ZoneRegions {
		regions = append(regions, tr(s, r.Name))
	}
	regions = append(regions, tr(s, "Type a zone name"))
	i := chooseFromList(s, w.Plain, tr(s, "Which part of the world?"), regions, tr(s, "Back"))
	if i < 0 {
		return Stay
	}
	if i == len(game.ZoneRegions) {
		return typeTimeZone(s, w)
	}

	region := game.ZoneRegions[i]
	names := make([]string, 0, len(region.Zones)+1)
	for _, z := range region.Zones {
		names = append(names, zoneLabel(z))
	}
	names = append(names, tr(s, "Type a zone name"))
	j := chooseFromList(s, w.Plain, tr(s, region.Name), names, tr(s, "Back"))
	if j < 0 {
		return Stay
	}
	if j == len(region.Zones) {
		return typeTimeZone(s, w)
	}
	return setTimeZone(s, w, region.Zones[j])
}

// typeTimeZone takes an IANA name by hand. It is checked against the zone
// database before it is stored, so a typo is refused here rather than leaving
// the player silently on UTC.
func typeTimeZone(s session.Session, w *ctx) Result {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgWhite,
		tr(s, "A zone name, as in America/New_York or Europe/Berlin."), ansi.Reset)
	name := strings.TrimSpace(prompt(s, tr(s, "Zone?")))
	if name == "" {
		return Stay
	}
	if !game.ValidZone(name) {
		fail(s, fmt.Errorf(tr(s, "%s is not a zone this game knows"), name))
		return Stay
	}
	return setTimeZone(s, w, name)
}

// setTimeZone stores the choice and says what a time will now look like, which
// is the only thing the player is really choosing between.
func setTimeZone(s session.Session, w *ctx, name string) Result {
	if err := w.mutatePlayer(func(p *game.Empire) error {
		p.TimeZone = name
		return nil
	}); err != nil {
		fail(s, err)
		return Stay
	}
	ok(s, "Times now read %s.", game.Stamp(time.Now(), game.Zone(name)))
	return Stay
}

// zoneLabel names a zone with the clock it is on right now, so the list can be
// read by someone who knows their own time but not their zone's name.
func zoneLabel(name string) string {
	loc := game.Zone(name)
	return fmt.Sprintf("%-30s %s", name, time.Now().In(loc).Format("15:04 MST"))
}

// timeZoneName is what the Preferences item shows beside its label: the stored
// choice in the words the picker offered it in.
func timeZoneName(lang, name string) string {
	switch name {
	case "":
		return "UTC"
	case game.LocalZone:
		return i18n.T(lang, "Board")
	}
	return name
}
