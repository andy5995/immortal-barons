package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/i18n"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// dropfileUnsetMsg is what both places that notice an unconfigured drop file
// say: the note after a reset, and the door's own refusal to start. One string
// so the two never drift into telling the sysop different things.
const dropfileUnsetMsg = "No drop file format is set. If you intend to run this as a BBS door, run -set-dropfile to choose the format your BBS writes."

// noteDropfileUnset reminds the sysop after a reset that the door still needs a
// drop file format. A note, not a prompt: the world it just seeded is also what
// -local play and the maintenance modes use, and none of those reads a drop
// file. A
// door.json that can't be read is left to the door to report.
func noteDropfileUnset(dataDir string) {
	if dc, err := store.LoadDoorConfig(dataDir); err == nil && dc.DropfileFormat == "" {
		fmt.Println(dropfileUnsetMsg)
	}
}

// findDropfile locates the configured format's drop file when -dropfile isn't
// given. A format whose spec defines an environment variable for the path is
// asked there first — BBSDEV.DRP makes that variable the standard discovery
// mechanism, and it is the only one that survives a BBS launching the door from
// a directory of its own. Otherwise it looks in the working directory, either
// letter case. The real door invocation passes -dropfile explicitly.
func findDropfile(format string) string {
	f, ok := door.FormatByID(format)
	if !ok {
		return ""
	}
	if f.Env != "" {
		if p := os.Getenv(f.Env); p != "" {
			return p
		}
	}
	for _, n := range []string{f.File, strings.ToLower(f.File)} {
		if _, err := os.Stat(n); err == nil {
			return n
		}
	}
	return ""
}

// peekDataDir scans os.Args for -data/--data so the -dropfile help text can name
// the configured format before flag.Parse runs. Best-effort; defaults to ./data.
func peekDataDir() string {
	args := os.Args[1:]
	for i, a := range args {
		switch {
		case a == "-data" || a == "--data":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(a, "-data="):
			return strings.TrimPrefix(a, "-data=")
		case strings.HasPrefix(a, "--data="):
			return strings.TrimPrefix(a, "--data=")
		}
	}
	return "./data"
}

// dropfileUsage builds the -dropfile help text: it names the configured format,
// or points an unconfigured sysop at -set-dropfile.
func dropfileUsage(lang, format string) string {
	base := i18n.T(lang, "path to the BBS drop file")
	if f, ok := door.FormatByID(format); ok {
		return base + " (" + f.Name + ")"
	}
	return base + " (" + i18n.T(lang, "run -set-dropfile first") + ")"
}

// runSetDrop lets the sysop choose which drop file format the BBS writes and
// saves it to door.json (BRE asks this during its one-time install). The door
// then reads that format; -reset runs this automatically when it isn't set yet.
func runSetDrop(dataDir string) error {
	dc, err := store.LoadDoorConfig(dataDir)
	if err != nil {
		return err
	}
	c := session.NewConsole()
	format, ok := chooseDropfile(c, dc.DropfileFormat)
	c.Close()
	if !ok {
		fmt.Println("\nDrop file format left unchanged.")
		return errCancelled
	}
	dc.DropfileFormat = format
	if err := store.SaveDoorConfig(dataDir, dc); err != nil {
		return err
	}
	f, _ := door.FormatByID(format)
	fmt.Printf("\nDrop file format set to %s. Configure your BBS to launch the door with the %s path.\n", f.Name, f.File)
	return nil
}

// chooseDropfile shows the drop file-format selection screen (styled like the
// reset screens: the BRE inset rule, a numbered list, the shared menu prompt)
// and returns the chosen Format ID. ok is false if the sysop quits. current is
// the presently-configured format, marked in the list.
func chooseDropfile(s session.Session, current string) (string, bool) {
	fmt.Fprintf(s, "\n%s%s%s\n", ansi.FgBrightRed, menu.InsetRule, ansi.Reset)
	fmt.Fprintf(s, "%s  Drop File Format%s\n\n", ansi.FgBrightWhite, ansi.Reset)
	fmt.Fprint(s, "  Which drop file does your BBS write when it launches a door?\n\n")
	for i, f := range door.Formats {
		cur := ""
		if f.ID == current {
			cur = ansi.FgWhite + "  (current)" + ansi.Reset
		}
		fmt.Fprintf(s, "  %s%d)%s %s%s\n", ansi.FgBrightYellow, i+1, ansi.Reset, f.Name, cur)
	}
	fmt.Fprintf(s, "  %s0)%s Quit (leave unchanged)\n", ansi.FgBrightYellow, ansi.Reset)

	// The shared prompt the menu engine uses for every numbered list, so this
	// screen reads and behaves like the rest of the game (one keypress, Enter
	// selects the shown Quit default).
	n := menu.ChoiceQuit(s, len(door.Formats))
	if n == 0 {
		return "", false
	}
	return door.Formats[n-1].ID, true
}
