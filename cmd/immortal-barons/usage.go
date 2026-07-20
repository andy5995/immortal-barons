package main

import (
	"flag"
	"fmt"
	"io"

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
	{"Character set (output)", []string{"utf8", "cp437"}},
	{"Sysop / game admin", []string{"reset", "reset-from-config", "add-ai", "maint", "dump"}},
	{"Inter-BBS", []string{"planetary", "league-config", "export", "import"}},
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

// printUsageFlag prints one flag in the two-line style Go's flag package uses:
// the flag and its value placeholder, then an indented description with the
// default value when it is meaningful (not the empty/false/zero default).
func printUsageFlag(w io.Writer, f *flag.Flag) {
	valueName, usage := flag.UnquoteUsage(f)
	line := "  -" + f.Name
	if valueName != "" {
		line += " " + valueName
	}
	fmt.Fprintln(w, line)
	fmt.Fprintf(w, "    \t%s", usage)
	if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" {
		fmt.Fprintf(w, " (default %q)", f.DefValue)
	}
	fmt.Fprintln(w)
}
