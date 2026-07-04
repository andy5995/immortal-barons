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

func TestControlsCategoryLoads(t *testing.T) {
	if Categories()[0] != "controls" {
		t.Errorf("controls should sort first, got %v", Categories())
	}
	topics := Topics("controls")
	if len(topics) < 2 {
		t.Fatalf("expected the migrated controls topics, got %d", len(topics))
	}
	if topics[0].Order > topics[1].Order {
		t.Error("topics should be sorted by order")
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
