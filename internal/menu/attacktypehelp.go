package menu

import (
	"fmt"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/session"
)

// attacktypehelp.go — the (?) Help item on the Attack Type menu.
//
// The original opens a BROWSER over three topics rather than printing a page:
// a rule, the three type names across it, then `Enter Topic (? for list): `
// asked again and again until the reader leaves, so all three can be read
// without going back to the menu (captured in cap/eots-ibbs-02.cap, #253). IB
// printed one long page, which also carried material the original's
// game/attack.hlp has no counterpart for — group-attack timing, the return
// path, the region picker. That material is not lost: it stays on the Attack
// Types page under Game Instructions, and only the three type topics answer
// here.
//
// The PROSE is IB's own, as everywhere. What is copied from the capture is the
// structure, the figures and the colours.

// attackTypeTopic is one entry in the browser: the name the reader types (or
// completes) and the body printed when they do.
type attackTypeTopic struct {
	name string
	body string
}

// The three topics, in the original's order. The figures are IB's, and the
// quick strike's is 120% — the original's own help text says 110% while its
// resolver loads 1.2 (BRE.OVR 0x4055a), so matching the browser's shape must
// not drag that wrong number along with it.
var attackTypeTopics = []attackTypeTopic{
	{"Normal Attack", "Your forces fight at full strength. Both sides break off once they have taken 15% losses, and you take the standard share of the defender's regions."},
	{"Quick Strike", "Surprise lets you fight at 120% of your normal strength, but the battle is short and disorganised: both sides retreat at 8% losses, and you carry off only half the land a Normal Attack would."},
	{"Extended Battle", "A grinding assault. Fatigue drops your forces to 85% strength, but they press until both sides have taken 20% losses, and they bring home 125% of a Normal Attack's land."},
}

const (
	// The rule the original draws over and under the topic list: 75 columns of
	// blue, five single bars, then fifteen double, then single to the end.
	topicRuleWidth  = 75
	topicRuleSingle = 5
	topicRuleDouble = 15
	// Each name sits in a 25-column field, so three fill the rule exactly.
	topicNameWidth = 25
)

func topicRule() string {
	return ansi.FgBlue + strings.Repeat("─", topicRuleSingle) +
		strings.Repeat("═", topicRuleDouble) +
		strings.Repeat("─", topicRuleWidth-topicRuleSingle-topicRuleDouble) + ansi.Reset
}

// showAttackTypeList draws the rule, the three names, and the rule again.
func showAttackTypeList(s session.Session) {
	fmt.Fprintf(s, "\n%s\n", topicRule())
	var b strings.Builder
	b.WriteString(ansi.FgBrightWhite)
	for _, t := range attackTypeTopics {
		fmt.Fprintf(&b, "%-*s", topicNameWidth, tr(s, t.name))
	}
	fmt.Fprintf(s, "%s%s\n", strings.TrimRight(b.String(), " "), ansi.Reset)
	fmt.Fprintf(s, "%s\n", topicRule())
}

// showAttackTypeTopic prints one type: its name bright red, the separator red,
// the prose white with its figures in cyan.
func showAttackTypeTopic(s session.Session, t attackTypeTopic) {
	head := fmt.Sprintf("%s%s%s - %s", ansi.FgBrightRed, tr(s, t.name), ansi.FgRed, ansi.FgWhite)
	body := hiNumsReset(tr(s, t.body), ansi.FgBrightCyan, ansi.FgWhite)
	fmt.Fprintf(s, "\n   %s\n%s%s\n", head, wrapHanging(body, "     ", "     "), ansi.Reset)
}

// matchAttackTypeTopic resolves what has been typed so far against the topic
// names, case-insensitively and by prefix — the same shape as the planet
// prompt, which is what the capture shows this one behaving as. It reports the
// single match and how many topics the text still fits.
func matchAttackTypeTopic(typed string) (name string, n int) {
	want := strings.ToLower(strings.TrimSpace(typed))
	if want == "" {
		return "", 0
	}
	for _, t := range attackTypeTopics {
		if strings.HasPrefix(strings.ToLower(t.name), want) {
			name, n = t.name, n+1
		}
	}
	if n != 1 {
		name = ""
	}
	return name, n
}

// showAttackTypeHelp is the (?) Help item: the topic list, then the prompt,
// asked again after each topic until the reader answers with Enter.
func showAttackTypeHelp(s session.Session, w *ctx) {
	showAttackTypeList(s)
	for {
		// The prompt's shape is the planet picker's, which the capture shows it
		// sharing — including the live completion, which finished "Quick Strike"
		// from a partial answer there.
		fmt.Fprintf(s, "\n%s%s %s(%s?%s %s%s)%s: %s",
			ansi.FgWhite, tr(s, "Enter Topic"),
			ansi.FgBrightBlack, ansi.FgBrightWhite, ansi.FgWhite, tr(s, "for list"),
			ansi.FgBrightBlack, ansi.FgWhite, ansi.FgBrightYellow)
		r, err := readKey(s)
		if err != nil {
			fmt.Fprint(s, ansi.Reset)
			return
		}
		if r == '?' {
			fmt.Fprintf(s, "?%s\n", ansi.Reset)
			showAttackTypeList(s)
			continue
		}
		// Enter alone answers None and returns to the menu, as the capture ends.
		if r == '\r' || r == '\n' {
			fmt.Fprintf(s, "%s\n", tr(s, "None"))
			fmt.Fprint(s, ansi.Reset)
			return
		}
		line, err := readCompletingAnswer(s, matchAttackTypeTopic, r)
		fmt.Fprint(s, ansi.Reset)
		if err != nil {
			return
		}
		name, _ := matchAttackTypeTopic(line)
		for _, t := range attackTypeTopics {
			if t.name == name {
				showAttackTypeTopic(s, t)
				break
			}
		}
	}
}
