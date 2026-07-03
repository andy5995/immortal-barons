package menu

import (
	"strings"
	"testing"
)

func TestHelpTopicsNonEmptyAndComplete(t *testing.T) {
	if len(helpTopics) == 0 {
		t.Fatal("expected helpTopics to be non-empty")
	}
	for i, topic := range helpTopics {
		if strings.TrimSpace(topic.Name) == "" {
			t.Errorf("helpTopics[%d] has an empty Name", i)
		}
		if strings.TrimSpace(topic.Text) == "" {
			t.Errorf("helpTopics[%d] (%q) has an empty Text", i, topic.Name)
		}
	}
}

func TestFindHelpTopicExactMatch(t *testing.T) {
	got := findHelpTopic("Troopers")
	if got == nil || got.Name != "Troopers" {
		t.Fatalf("expected exact match on Troopers, got %v", got)
	}
}

func TestFindHelpTopicCaseInsensitive(t *testing.T) {
	got := findHelpTopic("troopers")
	if got == nil || got.Name != "Troopers" {
		t.Fatalf("expected case-insensitive match on Troopers, got %v", got)
	}
}

func TestFindHelpTopicUniquePrefix(t *testing.T) {
	got := findHelpTopic("Gooie")
	if got == nil || got.Name != "Gooie Kablooie" {
		t.Fatalf("expected unique prefix match on Gooie Kablooie, got %v", got)
	}
}

func TestFindHelpTopicAmbiguousPrefixReturnsNil(t *testing.T) {
	// "T" is a prefix of multiple topics (Troopers, Turrets, Tanks, ...).
	if got := findHelpTopic("T"); got != nil {
		t.Fatalf("expected ambiguous prefix to return nil, got %v", got)
	}
}

func TestFindHelpTopicUnknownReturnsNil(t *testing.T) {
	if got := findHelpTopic("Nonexistent Topic"); got != nil {
		t.Fatalf("expected unknown topic to return nil, got %v", got)
	}
}

func TestFindHelpTopicEmptyReturnsNil(t *testing.T) {
	if got := findHelpTopic("   "); got != nil {
		t.Fatalf("expected blank query to return nil, got %v", got)
	}
}

func TestHelpDatabaseQuitsOnZero(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "B0\r0", menus.Game) // Help Database -> quit lookup -> Quit game menu
	if err != nil {
		t.Fatalf("got %v", err)
	}
	if !strings.Contains(f.out.String(), "Help Database") {
		t.Error("expected Help Database title in output")
	}
}

func TestHelpDatabaseLooksUpTopic(t *testing.T) {
	menus := BuildMenus()
	f, _, err := run(t, "BTroopers\r 0\r0", menus.Game) // Help Database -> "Troopers" -> pause -> quit lookup -> Quit game menu
	if err != nil {
		t.Fatalf("got %v", err)
	}
	out := f.out.String()
	if !strings.Contains(out, "cheapest unit") {
		t.Errorf("expected Troopers description in output, got:\n%s", out)
	}
}
