package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

func TestParseNodeList(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brnodes.dat")
	// Two node blocks separated by a blank line (docs/brnodes.sam format).
	content := "1\nAvalon\n363/277\nOrlando\nFL\nUSA\n\n5\nThe Holodeck BBS\n260/249\nRochester\nNY\nUSA\n\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := ParseNodeList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("want 2 nodes, got %d: %+v", len(nodes), nodes)
	}
	if nodes[0].Number != 1 || nodes[0].Name != "Avalon" || nodes[0].Address != "363/277" {
		t.Errorf("node 0 mismatch: %+v", nodes[0])
	}
	if nodes[1].Number != 5 || nodes[1].City != "Rochester" || nodes[1].State != "NY" {
		t.Errorf("node 1 mismatch: %+v", nodes[1])
	}
}

func TestParseBoardConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bbs.cfg")
	content := "John Dailey\nAvalon\n363/277\nC:\\FD\\FILES\nC:\\FD\\NETMAIL\n900\nFRONTDOOR\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseBoardConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.PlanetName != "Avalon" || cfg.Address != "363/277" || cfg.League != 900 || cfg.Mailer != "FRONTDOOR" {
		t.Errorf("board config mismatch: %+v", cfg)
	}
}

// A sysop-edited brnodes.dat with a broken block must not take the good ones
// down with it: the parser's documented tolerance is to SKIP a block whose
// node number is not numeric or that is truncated, and keep parsing.
func TestParseNodeListSkipsMalformedBlocks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "brnodes.dat")
	content := "one\nBad Number BBS\n1/2\nTown\nST\nUSA\n\n" + // non-numeric node number
		"9\nTruncated BBS\n3/4\n\n" + // 3 lines, needs 6
		"5\nGood BBS\n260/249\nRochester\nNY\nUSA\n\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := ParseNodeList(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 1 || nodes[0].Number != 5 || nodes[0].Name != "Good BBS" {
		t.Errorf("want only the good block parsed, got %+v", nodes)
	}
}

// TestNodeListKeyLineIsOptional pins the roster's back-compatibility both ways
// (#118). The signing key is a SEVENTH line so an existing roster parses
// unchanged and a keyless one is rewritten byte for byte — an older board reads
// blocks by index and simply never looks at index six.
func TestNodeListKeyLineIsOptional(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "old.dat")
	// A roster exactly as written before keys existed.
	const legacy = "1 HOST 2\nAlpha\n1:1/1\nCity\nST\nUS\n\n2\nBravo\n1:1/2\nTown\nST\nUS\n"
	if err := os.WriteFile(old, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := ParseNodeList(old)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("parsed %d nodes, want 2", len(nodes))
	}
	for _, n := range nodes {
		if n.PublicKey != "" {
			t.Errorf("%s got a key from a roster that has none: %q", n.Name, n.PublicKey)
		}
	}
	// Rewriting must not change a keyless roster at all.
	round := filepath.Join(dir, "round.dat")
	if err := WriteNodeList(round, nodes); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(round)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != legacy {
		t.Errorf("keyless roster changed on rewrite:\n got %q\nwant %q", got, legacy)
	}

	// With a key, it survives the trip and lands on the right board.
	nodes[1].PublicKey = "abc123"
	keyed := filepath.Join(dir, "keyed.dat")
	if err := WriteNodeList(keyed, nodes); err != nil {
		t.Fatal(err)
	}
	back, err := ParseNodeList(keyed)
	if err != nil {
		t.Fatal(err)
	}
	if back[0].PublicKey != "" || back[1].PublicKey != "abc123" {
		t.Errorf("keys after round trip: %q, %q", back[0].PublicKey, back[1].PublicKey)
	}
	if back[0].Hosts == nil || back[1].Name != "Bravo" {
		t.Errorf("the key line disturbed the other fields: %+v", back)
	}
}

// NodeNumber answers 0 for a name it cannot find, so a roster entry numbered 0
// would be handed to any board whose own name is absent, as its own origin.
func TestNodeListRejectsNodeNumberZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), NodeListFile)
	body := "0\nGhost BBS\n1:1/0\nLocal\nXX\nUSA\n\n2\nBravo BBS\n1:1/2\nLocal\nXX\nUSA\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, err := ParseNodeList(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if n.Number < 1 {
			t.Errorf("node %q parsed with number %d", n.Name, n.Number)
		}
	}
	if len(nodes) != 1 || nodes[0].Name != "Bravo BBS" {
		t.Errorf("want only the valid entry, got %+v", nodes)
	}
}

// TestNodeListRejectsNodeNumberAboveTheCeiling is #180: a node number is
// rendered in base 36 into every packet filename its board writes, and that name
// has to fit an FTN Type-2 subject beside a directory path, so the roster holds
// the same 1-999 range as the league number.
func TestNodeListRejectsNodeNumberAboveTheCeiling(t *testing.T) {
	path := filepath.Join(t.TempDir(), NodeListFile)
	body := "1000\nOversized BBS\n1:1/0\nLocal\nXX\nUSA\n\n" +
		"999\nCeiling BBS\n1:1/1\nLocal\nXX\nUSA\n\n" +
		"2\nBravo BBS\n1:1/2\nLocal\nXX\nUSA\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, problems, err := ParseNodeListReport(path)
	if err != nil {
		t.Fatal(err)
	}
	// 999 is inside the range and must survive: a ceiling that excludes its own
	// top value would quietly narrow the documented range.
	if len(nodes) != 2 || nodes[0].Number != 999 || nodes[1].Number != 2 {
		t.Fatalf("roster parsed as %+v", nodes)
	}
	// The dropped board is named, because a board missing from the roster is
	// unreachable and its packets are refused.
	if len(problems) != 1 || !strings.Contains(problems[0], "Oversized BBS") {
		t.Fatalf("problems reported: %v", problems)
	}
	if !strings.Contains(problems[0], "1 to 999") {
		t.Errorf("the problem does not state the range: %q", problems[0])
	}
}

// A roster with nothing wrong reports nothing, so the checkup line only appears
// when there is something to fix.
func TestCleanNodeListReportsNoProblems(t *testing.T) {
	path := filepath.Join(t.TempDir(), NodeListFile)
	body := "1\nAlpha BBS\n1:1/1\nLocal\nXX\nUSA\n\n2\nBravo BBS\n1:1/2\nLocal\nXX\nUSA\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, problems, err := ParseNodeListReport(path)
	if err != nil || len(nodes) != 2 || len(problems) != 0 {
		t.Fatalf("nodes=%+v problems=%v err=%v", nodes, problems, err)
	}
}

// A roster name is bounded so a malformed file cannot carry an unbounded string
// into the world and out on every packet — but the cap sits far above any real
// board name on purpose. Routing compares board NAMES when a packet carries no
// node number, so a cut that a real roster could reach would let two boards
// answer to each other's mail.
func TestRosterNameIsCappedFarAboveAnyRealName(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("A", game.MaxNodeNameLen+50)
	path := filepath.Join(dir, "ibnodes.dat")
	body := "1\n" + long + "\n1:1/1\nCity\nST\nUSA\n\n2\nShort BBS\n1:1/2\nCity\nST\nUSA\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	nodes, problems, err := ParseNodeListReport(path)
	if err != nil {
		t.Fatalf("ParseNodeListReport: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("got %d nodes, want 2", len(nodes))
	}
	if got := len(nodes[0].Name); got != game.MaxNodeNameLen {
		t.Errorf("name kept %d bytes, want it cut to %d", got, game.MaxNodeNameLen)
	}
	// The Coordinator has to be told, or the roster silently means something else
	// on their board than the file says.
	if len(problems) == 0 {
		t.Error("the cut was not reported; -league-check would say nothing")
	}
	// An ordinary name is untouched — the cap must never reach a real roster.
	if nodes[1].Name != "Short BBS" {
		t.Errorf("ordinary name became %q", nodes[1].Name)
	}
}
