package main

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/i18n"
)

// usageGroups organizes the command-line flags into audience-based sections for
// -help, instead of the flat alphabetical list Go's flag package prints by
// default. The grouping mirrors docs/command-reference.md — keep the two in sync
// (GH #34). Any flag NOT listed here still prints, under "Other" (see
// groupedUsage), so a newly-added flag can never be silently dropped from -help.
var usageGroups = []struct {
	title string
	flags []string
}{
	{"Play", []string{"local", "name", "dropfile", "data"}},
	{"Terminal (output)", []string{"utf8", "cp437", "ascii", "no-ansi"}},
	{"Sysop / game admin", []string{"set-dropfile", "reset", "reset-from-config", "add-ai", "maint"}},
	{"Inter-BBS", []string{"planetary", "league-config", "export", "import"}},
	// Development tools, kept out of the sysop section: a board never needs
	// these, and both of them advance or expose game state.
	{"Testing and balance", []string{"dump", "spectate"}},
	{"Info", []string{"version"}},
}

// groupedUsage returns a flag.Usage function that prints fs's flags under the
// usageGroups sections, then any ungrouped flag under a final "Other" section.
// lang localizes the section headings and the intro line; the per-flag
// descriptions come from the flag definitions themselves (already localized), so
// there is no duplicated help text.
func groupedUsage(fs *flag.FlagSet, lang string) func() {
	return func() {
		w := fs.Output()
		fmt.Fprintf(w, "%s\n\n", i18n.T(lang, "immortal-barons — a Barren Realms Elite clone. Options:"))

		grouped := map[string]bool{}
		for _, g := range usageGroups {
			var shown []*flag.Flag
			for _, name := range g.flags {
				if f := fs.Lookup(name); f != nil {
					shown = append(shown, f)
					grouped[name] = true
				}
			}
			if len(shown) == 0 {
				continue
			}
			fmt.Fprintf(w, "%s:\n", i18n.T(lang, g.title))
			for _, f := range shown {
				printUsageFlag(w, f)
			}
			fmt.Fprintln(w)
		}

		// Any flag not placed in a group above (a stray or newly-added one) still
		// appears, so -help stays complete without per-flag maintenance.
		var other []*flag.Flag
		fs.VisitAll(func(f *flag.Flag) {
			if !grouped[f.Name] {
				other = append(other, f)
			}
		})
		if len(other) > 0 {
			fmt.Fprintf(w, "%s:\n", i18n.T(lang, "Other"))
			for _, f := range other {
				printUsageFlag(w, f)
			}
		}
	}
}

// Help output is written for an 80-column BBS terminal. Descriptions used to run
// past 140 characters on one line, which wraps raggedly on a real caller's
// screen. helpIndent is a fixed run of spaces rather than Go's "    \t": a tab's
// width varies by terminal, so wrapping to a column budget cannot be done
// correctly with one.
const (
	helpWidth  = 78
	helpIndent = "      "
)

// printUsageFlag prints one flag in the two-line style Go's flag package uses:
// the flag and its value placeholder, then the description, wrapped and
// indented, with the default value when it is meaningful (not the empty, false
// or zero default).
func printUsageFlag(w io.Writer, f *flag.Flag) {
	valueName, usage := flag.UnquoteUsage(f)
	line := "  -" + f.Name
	if valueName != "" {
		line += " " + valueName
	}
	fmt.Fprintln(w, line)
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
		usage += fmt.Sprintf(" (default %q)", f.DefValue)
	}
	for _, l := range wrapText(usage, helpWidth-len(helpIndent)) {
		fmt.Fprintf(w, "%s%s\n", helpIndent, l)
	}
}

// wrapText breaks text into lines of at most width runes, on spaces. A word
// longer than width gets its own line rather than being split, so a long flag
// name or URL stays selectable.
func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	cur := words[0]
	for _, word := range words[1:] {
		if utf8.RuneCountInString(cur)+1+utf8.RuneCountInString(word) <= width {
			cur += " " + word
			continue
		}
		lines = append(lines, cur)
		cur = word
	}
	return append(lines, cur)
}
