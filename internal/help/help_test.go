package help

import (
	"strings"
	"testing"
)

func TestParseTopicFrontmatter(t *testing.T) {
	raw := "---\ntitle: Hello\ncategory: controls\norder: 3\nin_game: true\n---\n# Hello\n\nBody."
	top := parseTopic(raw)
	if top.Title != "Hello" || top.Category != "controls" || top.Order != 3 || !top.InGame {
		t.Errorf("frontmatter parse wrong: %+v", top)
	}
	if !strings.HasPrefix(top.Body, "# Hello") {
		t.Errorf("body should start after frontmatter: %q", top.Body)
	}
}

func TestParseTopicDocsOnly(t *testing.T) {
	if parseTopic("---\ntitle: X\ncategory: economy\nin_game: false\n---\nbody").InGame {
		t.Error("in_game: false should be respected")
	}
}

func TestParseTopicStripsQuotedTitle(t *testing.T) {
	// po4a quotes frontmatter values in the generated translations.
	if got := parseTopic("---\ntitle: 'Durch die Menüs'\ncategory: controls\n---\nx").Title; got != "Durch die Menüs" {
		t.Errorf("quoted title not unquoted: %q", got)
	}
}

func TestTopicsLocalized(t *testing.T) {
	en := Topics("controls", "")
	de := Topics("controls", "de")
	if len(en) != len(de) {
		t.Fatalf("topic count differs by language: en=%d de=%d", len(en), len(de))
	}
	// Structure (order/path) matches; at least one title is actually translated.
	translated := false
	for i := range en {
		if en[i].Order != de[i].Order || en[i].path != de[i].path {
			t.Errorf("structure drifted at %d: %q vs %q", i, en[i].path, de[i].path)
		}
		if en[i].Title != de[i].Title {
			translated = true
		}
	}
	if !translated {
		t.Error("expected at least one German title to differ from English")
	}
}

func TestUnknownLanguageFallsBackToEnglish(t *testing.T) {
	en := Topics("controls", "")
	xx := Topics("controls", "xx")
	for i := range en {
		if en[i].Title != xx[i].Title || en[i].Body != xx[i].Body {
			t.Fatalf("unknown language should equal English at %d", i)
		}
	}
}

func TestControlsCategoryLoads(t *testing.T) {
	if Categories()[0] != "controls" {
		t.Errorf("controls should sort first, got %v", Categories())
	}
	topics := Topics("controls", "")
	if len(topics) < 2 {
		t.Fatalf("expected the migrated controls topics, got %d", len(topics))
	}
	if topics[0].Order > topics[1].Order {
		t.Error("topics should be sorted by order")
	}
}

func TestInstructionsAssembly(t *testing.T) {
	seq := Instructions("")
	if len(seq) == 0 {
		t.Fatal("Instructions returned nothing")
	}
	// The overview leads the read-through.
	if seq[0].Category != "introduction" {
		t.Errorf("first item should be the introduction overview, got category %q", seq[0].Category)
	}
	// It stitches in real help topics from more than one category, so it is the
	// whole manual, not just the intro.
	cats := map[string]bool{}
	for _, top := range seq {
		cats[top.Category] = true
	}
	if !cats["military"] || !cats["economy"] {
		t.Errorf("expected topics from multiple categories, got %v", cats)
	}
	// The docs-only overview must never leak into the ? browser's category list.
	for _, c := range Categories() {
		if c == "introduction" {
			t.Error("introduction is docs-only and must not be a browsable category")
		}
	}
}

func TestRenderANSIWrapsToWidth(t *testing.T) {
	top := Topic{Body: "# Title\n\nThis is a fairly long line of prose that should wrap onto several lines when the width is small."}
	out := top.RenderANSI(30)
	if !strings.Contains(out, "Title") {
		t.Error("heading text should render")
	}
	for _, line := range strings.Split(out, "\n") {
		vis := line
		for _, code := range []string{"\x1b[96m", "\x1b[97m", "\x1b[0m"} {
			vis = strings.ReplaceAll(vis, code, "")
		}
		if len([]rune(vis)) > 30 {
			t.Errorf("line exceeds width 30: %q (%d cols)", vis, len([]rune(vis)))
		}
	}
}
