package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// The sample BRE ships (docs/route.sam), minus the comment marker its lines all
// carry, plus the mailer-priority lines a file drop has no use for.
func TestParseRouteFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, RouteFile)
	body := ";InterBBS Routing Configuration File\n" +
		"ROUTE 5 10\n" +
		"ROUTE * 8\n" +
		"ROUTE 5 5\n" +
		"CRASH 5\n" +
		"HOLD *\n" +
		"\n" +
		"garbage line\n" +
		"ROUTE nine 4\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := ParseRouteFile(path)
	if err != nil {
		t.Fatalf("ParseRouteFile: %v", err)
	}
	want := []game.RouteRule{{Dest: 5, Via: 10}, {Dest: 0, Via: 8}, {Dest: 5, Via: 5}}
	if len(got) != len(want) {
		t.Fatalf("parsed %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rule %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The HOST line is the Coordinator's routing, so a member board that adopts a
// broadcast roster and writes it back must not lose it — the roster file is
// where the routing lives between runs.
func TestNodeListRoundTripsHostRouting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, NodeListFile)
	nodes := []game.LeagueNode{
		{Number: 1, Hosts: []int{2, 4}, Name: "Avalon", Address: "1:363/277", City: "Orlando", State: "FL", Country: "USA"},
		{Number: 2, Hosts: []int{3}, Name: "Pier 7", Address: "1:106/477", City: "Houston", State: "TX", Country: "USA"},
		{Number: 3, Name: "The Realms", Address: "1:153/945", City: "Vancouver", State: "BC", Country: "Canada"},
	}
	if err := WriteNodeList(path, nodes); err != nil {
		t.Fatalf("WriteNodeList: %v", err)
	}
	got, err := ParseNodeList(path)
	if err != nil {
		t.Fatalf("ParseNodeList: %v", err)
	}
	if !game.SameRoster(got, nodes) {
		t.Errorf("round trip = %+v, want %+v", got, nodes)
	}
}

// A roster hand-written in BRE's own format, HOST line and all.
func TestParseNodeListReadsBREHostLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, NodeListFile)
	body := "2 HOST 3 4 8\nMy BBS\n1:222/333\nSomewhere\nST\nUSA\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := ParseNodeList(path)
	if err != nil {
		t.Fatalf("ParseNodeList: %v", err)
	}
	if len(got) != 1 || got[0].Number != 2 {
		t.Fatalf("parsed %+v, want one node numbered 2", got)
	}
	want := []int{3, 4, 8}
	if len(got[0].Hosts) != len(want) {
		t.Fatalf("Hosts = %v, want %v", got[0].Hosts, want)
	}
	for i := range want {
		if got[0].Hosts[i] != want[i] {
			t.Errorf("Hosts = %v, want %v", got[0].Hosts, want)
		}
	}
}
