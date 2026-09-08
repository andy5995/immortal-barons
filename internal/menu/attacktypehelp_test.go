package menu

import (
	"strings"
	"testing"
)

// The (?) Help item on the Attack Type menu opens a BROWSER over three topics,
// not a page (#253): the list, then a prompt that comes back after each topic
// so all three can be read without leaving.
func TestAttackTypeHelpBrowsesThreeTopics(t *testing.T) {
	// "quic" completes to Quick Strike, then Enter answers None and leaves.
	f := &fakeSession{keys: []rune("quic\r\r")}
	showAttackTypeHelp(f, &ctx{})
	out := stripANSI(f.out.String())

	for _, want := range []string{"Normal Attack", "Quick Strike", "Extended Battle"} {
		if !strings.Contains(out, want) {
			t.Errorf("the topic list is missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "Enter Topic") {
		t.Fatalf("never reached the topic prompt:\n%s", out)
	}
	if !strings.Contains(out, "120%") {
		t.Errorf("the chosen topic's body never printed:\n%s", out)
	}
	// The page's group-attack material belongs to Game Instructions now, not here.
	if strings.Contains(out, "12 and 120") {
		t.Errorf("the browser printed the whole help page:\n%s", out)
	}
	if !strings.Contains(out, "None") {
		t.Errorf("Enter alone should answer None:\n%s", out)
	}
}

// Enter at the first prompt leaves at once, printing no topic body.
func TestAttackTypeHelpEnterLeavesImmediately(t *testing.T) {
	f := &fakeSession{keys: []rune("\r")}
	showAttackTypeHelp(f, &ctx{})
	out := stripANSI(f.out.String())

	if !strings.Contains(out, "Normal Attack") {
		t.Fatalf("the list is drawn on entry:\n%s", out)
	}
	if strings.Contains(out, "120%") {
		t.Errorf("no topic body should print:\n%s", out)
	}
}

// The three names fill the rule exactly, which is what the 25-column fields are
// for: a translation that overflows would ragged the row against the rule.
func TestAttackTypeTopicNamesFitTheirColumns(t *testing.T) {
	for _, topic := range attackTypeTopics {
		if len(topic.name) >= topicNameWidth {
			t.Errorf("%q is %d wide, the field is %d", topic.name, len(topic.name), topicNameWidth)
		}
	}
	if got := topicRuleSingle + topicRuleDouble; got >= topicRuleWidth {
		t.Errorf("the rule's accents (%d) do not fit its width (%d)", got, topicRuleWidth)
	}
	if len(attackTypeTopics)*topicNameWidth != topicRuleWidth {
		t.Errorf("%d topics x %d columns should fill the %d-column rule",
			len(attackTypeTopics), topicNameWidth, topicRuleWidth)
	}
}

// A partial answer resolves to one topic, and an ambiguous one to none.
func TestMatchAttackTypeTopic(t *testing.T) {
	for _, tc := range []struct {
		typed string
		want  string
		n     int
	}{
		{"quic", "Quick Strike", 1},
		{"EXTENDED", "Extended Battle", 1},
		{"n", "Normal Attack", 1},
		{"", "", 0},
		{"zzz", "", 0},
	} {
		if got, n := matchTopic(attackTypeTopics, tc.typed); got != tc.want || n != tc.n {
			t.Errorf("%q -> %q,%d; want %q,%d", tc.typed, got, n, tc.want, tc.n)
		}
	}
}
