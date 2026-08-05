package main

import (
	"bytes"
	"flag"
	"strings"
	"testing"
)

// groupedUsage prints flags under named sections, and — crucially — any flag not
// listed in a section still appears under "Other", so -help can never silently
// omit a flag someone forgot to group.
func TestGroupedUsageCoversEveryFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("local", false, "play in your own terminal")
	fs.Bool("mystery", false, "deliberately not placed in any group")
	var buf bytes.Buffer
	fs.SetOutput(&buf)

	groupedUsage(fs, "")()
	out := buf.String()

	if !strings.Contains(out, "-local") {
		t.Errorf("a grouped flag should appear in -help\n%s", out)
	}
	if !strings.Contains(out, "-mystery") {
		t.Errorf("an ungrouped flag must still appear (under Other), else -help drops it\n%s", out)
	}
}

// The section headings should be present so the output reads as grouped, not a
// flat alphabetical dump.
func TestGroupedUsageShowsSections(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("local", false, "play locally")
	fs.Bool("utf8", false, "force UTF-8")
	var buf bytes.Buffer
	fs.SetOutput(&buf)

	groupedUsage(fs, "")()
	out := buf.String()

	for _, section := range []string{"Play", "Terminal"} {
		if !strings.Contains(out, section) {
			t.Errorf("expected section heading %q in -help\n%s", section, out)
		}
	}
}
