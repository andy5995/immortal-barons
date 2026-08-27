package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
)

// TestReadInboundAppliesCoordinatorPacketBeforeOthers is the fix for #178: a
// roster update from the Coordinator has to be adopted before anything else in
// the same batch is checked against the roster, not whenever its filename
// happens to sort. The Coordinator's file here is named to sort AFTER the
// packet that depends on it -- exactly the ordering that broke this before the
// fix, since directory order is filename order and the fix no longer follows
// it.
func TestReadInboundAppliesCoordinatorPacketBeforeOthers(t *testing.T) {
	inbound := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	// The Coordinator broadcasts a roster that, for the first time, gives the
	// receiving board ("Newbie BBS") a node number.
	coordCfg := game.DefaultConfig()
	coordCfg.BoardID = "Coordinator BBS"
	coord := game.NewWorldSeed(coordCfg, 1)
	coord.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 7, Name: "Newbie BBS"},
	}
	coord.CoordKey = priv
	coord.ExportNodeList()
	coord.StampOutbox()
	if len(coord.Outbox) != 1 {
		t.Fatalf("Coordinator did not queue a roster broadcast: %+v", coord.Outbox)
	}
	writePacket(t, inbound, "z-roster-update", coord.Outbox[0])

	// A packet from a third board, addressed ONLY by node number -- no ToBoard
	// to fall back on. Until the roster above is adopted, the receiving board
	// has no node number of its own, so this cannot be recognized as addressed
	// here at all. Its filename sorts BEFORE the roster update's.
	writePacket(t, inbound, "a-gift-for-newbie", game.Packet{
		FromBoard: "Third BBS",
		ToNode:    7,
		Seq:       1,
	})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Newbie BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.CoordPub = pub

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if len(w.LeagueNodes) != 2 {
		t.Fatalf("roster was not adopted: %+v", w.LeagueNodes)
	}
	if result.Applied != 2 {
		t.Errorf("want both packets applied (the roster update first makes the second recognizable), got applied=%d meshCopy=%d refused=%d: %+v",
			result.Applied, result.MeshCopy, result.Refused, result)
	}
}

// TestReadInboundBootstrapRosterSurvivesADecoyClaim is a regression test for
// a real finding: with no Coordinator known yet, the no-roster fallback used
// to stop at the FIRST group in scan (directory) order merely CLAIMING
// FromNode: 1, then gate only that one candidate. A decoy under a different
// board name, sorting first, cleared coordKey entirely and cost the genuine
// signed roster its carve-out -- filename order deciding the outcome again,
// in the one case (a board's first-ever roster) this exists to prevent, and
// left the receiving board's own node-addressed packet unrecognized this
// run (confirmed manually, not just here: 18/30 trials under the bug).
// Every candidate group is now scanned, and the fallback picks the first
// one that actually passes the verified-orders check, not the first one
// merely claiming it -- deterministic here since the decoy's filename is
// fixed to sort first.
func TestReadInboundBootstrapRosterSurvivesADecoyClaim(t *testing.T) {
	inbound := t.TempDir()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	coordCfg := game.DefaultConfig()
	coordCfg.BoardID = "Coordinator BBS"
	coord := game.NewWorldSeed(coordCfg, 1)
	coord.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 7, Name: "Newbie BBS"},
	}
	coord.CoordKey = priv
	coord.ExportNodeList()
	coord.StampOutbox()
	if len(coord.Outbox) != 1 {
		t.Fatalf("Coordinator did not queue a roster broadcast: %+v", coord.Outbox)
	}
	// A decoy: a different, unverifiable board, falsely claiming
	// FromNode: 1, filename sorting BEFORE the genuine roster's -- exactly
	// the arrangement that lost the carve-out under the bug.
	writePacket(t, inbound, "0-decoy", game.Packet{
		FromBoard: "Decoy BBS", FromNode: 1, Seq: 1,
	})
	writePacket(t, inbound, "1-roster-update", coord.Outbox[0])
	// Addressed ONLY by node number, recognizable only once the roster
	// above is adopted -- the packet the review's manual trials showed
	// falling through to mesh-copy under the bug.
	writePacket(t, inbound, "2-gift-for-newbie", game.Packet{
		FromBoard: "Third BBS",
		ToNode:    7,
		Seq:       1,
		News:      []string{"GIFT"},
	})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Newbie BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.CoordPub = pub

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if len(w.LeagueNodes) != 2 {
		t.Fatalf("roster was not adopted despite the decoy: %+v", w.LeagueNodes)
	}
	found := false
	for _, line := range w.NewsToday {
		if line == "GIFT" {
			found = true
		}
	}
	if !found {
		t.Errorf("the node-addressed packet was not recognized this run (fell through to wait for the next one), "+
			"got News=%v, Applied=%d MeshCopy=%d", w.NewsToday, result.Applied, result.MeshCopy)
	}
}

// TestReadInboundKeepsOneOriginsPacketsInSeqOrder locks in the half of #178
// that was already correct and must stay that way: grouping packets by origin
// must never reorder a single origin's own packets against each other, even
// when their filenames (built here to defeat plain alphabetical sort) say
// otherwise.
func TestReadInboundKeepsOneOriginsPacketsInSeqOrder(t *testing.T) {
	inbound := t.TempDir()
	// Filenames deliberately run opposite to Seq order.
	writePacket(t, inbound, "1-third", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 3, Notice: "third"})
	writePacket(t, inbound, "2-first", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 1, Notice: "first"})
	writePacket(t, inbound, "3-second", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 2, Notice: "second"})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{{Number: 5, Name: "Origin BBS"}, {Number: 9, Name: "Receiver BBS"}}

	// Notice carries no payload (see HasPayload), so nothing but SysopNotices
	// records the order ApplyPacket saw these in: a refusal notice per Notice
	// isn't posted for a plain Notice field, so record order a different way --
	// IsPacketSeen would reject a packet as a replay if it were ever applied
	// after a higher-Seq sibling already set HighSeq past it.
	//
	// result.Applied, not HighSeq, is the assertion that actually catches
	// that: an out-of-order run (3, 1, 2) sets HighSeq to 3 on the FIRST
	// packet applied, then silently drops the other two as replays --
	// HighSeq still ends at 3 either way, so checking only its final value
	// passes in the exact failure case it claims to catch (review on this
	// PR). Applied == 3 is true only when every packet actually got
	// through, which happens only in true Seq order.
	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if result.Applied != 3 {
		t.Fatalf("Applied = %d, want 3 (an out-of-order application drops at least one packet as a false replay)", result.Applied)
	}
	if w.HighSeq["Origin BBS"] != 3 {
		t.Errorf("HighSeq[Origin BBS] = %d, want 3", w.HighSeq["Origin BBS"])
	}
}

// TestShuffleGroupOrderVaries checks that shuffleGroupOrder is not a no-op
// dressed up as a shuffle: with enough elements and enough trials, seeing the
// exact same permutation every time would mean the randomness isn't reaching
// the result, not that it got unlucky (5! = 120 permutations; 25 identical
// trials in a row has probability on the order of 120^-24).
func TestShuffleGroupOrderVaries(t *testing.T) {
	first := []string{"n1", "n2", "n3", "n4", "n5"}
	same := true
	for i := 0; i < 25; i++ {
		keys := append([]string(nil), first...)
		shuffleGroupOrder(keys, rand.Reader)
		for j := range keys {
			if keys[j] != first[j] {
				same = false
			}
		}
		if !same {
			break
		}
	}
	if same {
		t.Error("shuffleGroupOrder produced the same order on every trial; expected real variation")
	}
}

type alwaysFailReader struct{}

func (alwaysFailReader) Read([]byte) (int, error) {
	return 0, errors.New("simulated randomness failure")
}

// TestShuffleGroupOrderLeavesKeysUntouchedOnSourceFailure is a fix from
// review: the first version of shuffleGroupOrder swapped elements of keys IN
// PLACE during the Fisher-Yates pass, so a source that failed partway through
// left keys partially shuffled -- neither this run's random order nor its
// original scan order, contradicting what the function documented. It now
// shuffles a scratch copy and only writes back once the whole pass succeeds,
// so keys must come back byte-for-byte identical to what went in.
func TestShuffleGroupOrderLeavesKeysUntouchedOnSourceFailure(t *testing.T) {
	original := []string{"n1", "n2", "n3", "n4", "n5"}
	keys := append([]string(nil), original...)
	shuffleGroupOrder(keys, alwaysFailReader{})
	for i := range keys {
		if keys[i] != original[i] {
			t.Fatalf("keys changed despite the randomness source failing: got %v, want %v (unchanged)", keys, original)
		}
	}
}

// TestReadInboundGroupsSameBoardTogetherRegardlessOfFromNode is a fix from
// review: originKey originally preferred FromNode when a packet carried one,
// but replay detection (IsPacketSeen/SeenPacket's HighSeq) keys on FromBoard
// alone. A board's own packets do not all carry the same FromNode within one
// batch -- an older or backlogged packet can predate the board's roster
// entry (FromNode 0) while a newer one from the same board carries it -- and
// splitting those into two groups let the group with the higher Seq apply
// first, marking the other group's lower Seq a false replay: a real packet
// silently dropped, not just misordered. Run enough trials that the old,
// shuffle-order-dependent failure (roughly half of them) would show up if
// the fix regressed.
func TestReadInboundGroupsSameBoardTogetherRegardlessOfFromNode(t *testing.T) {
	for trial := 0; trial < 40; trial++ {
		inbound := t.TempDir()
		writePacket(t, inbound, "1-old", game.Packet{FromBoard: "Origin BBS", FromNode: 0, Seq: 1})
		writePacket(t, inbound, "2-new", game.Packet{FromBoard: "Origin BBS", FromNode: 7, Seq: 2})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.LeagueNodes = []game.LeagueNode{{Number: 7, Name: "Origin BBS"}, {Number: 9, Name: "Receiver BBS"}}

		result, err := ReadInbound(w, inbound, false)
		if err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		if result.Applied != 2 {
			t.Fatalf("trial %d: want both packets applied from the same board, got applied=%d alreadySeen=%d: %+v",
				trial, result.Applied, result.AlreadySeen, result)
		}
	}
}

// TestReadInboundGroupsCoordinatorPacketsTogetherRegardlessOfFromNode is
// review issue 1 recurring one level up: the Coordinator carve-out used to
// pull a packet out by IsFromCoordinator (FromNode-preferring) BEFORE
// originKey (FromBoard-preferring) ever grouped it, so the Coordinator's own
// backlogged FromNode: 0 packet and its current FromNode: 1 packet could
// land on two different sides of that carve-out -- the carve-out always
// applies first, so the later side's lower Seq reads as a false replay, same
// consequence as the originKey bug this mirrors. The Coordinator's group is
// now identified by matching its KEY, so both packets are already unified
// before the carve-out ever looks at them.
func TestReadInboundGroupsCoordinatorPacketsTogetherRegardlessOfFromNode(t *testing.T) {
	for trial := 0; trial < 40; trial++ {
		inbound := t.TempDir()
		writePacket(t, inbound, "1-old", game.Packet{FromBoard: "Coordinator BBS", FromNode: 0, Seq: 1})
		writePacket(t, inbound, "2-new", game.Packet{FromBoard: "Coordinator BBS", FromNode: 1, Seq: 2})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.LeagueNodes = []game.LeagueNode{{Number: 1, Name: "Coordinator BBS"}, {Number: 9, Name: "Receiver BBS"}}

		result, err := ReadInbound(w, inbound, false)
		if err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		if result.Applied != 2 {
			t.Fatalf("trial %d: want both of the Coordinator's own packets applied, got applied=%d alreadySeen=%d: %+v",
				trial, result.Applied, result.AlreadySeen, result)
		}
	}
}

// TestReadInboundForgedCoordinatorClaimDoesNotJumpTheQueue is review issue 2:
// once this board's roster names a real Coordinator, a packet from some
// OTHER, ordinary board that merely sets FromNode: 1 must not buy that
// board's group a guaranteed first slot -- it is identified by its group KEY
// (its real FromBoard), not by any field a packet can just claim. Run many
// trials: under the bug this board's group was ALWAYS first (deterministic,
// not just likely); under the fix it is only sometimes first, like any other
// non-Coordinator origin in the shuffle.
func TestReadInboundForgedCoordinatorClaimDoesNotJumpTheQueue(t *testing.T) {
	appliedFirst := 0
	const trials = 60
	for trial := 0; trial < trials; trial++ {
		inbound := t.TempDir()
		// A real Coordinator packet exists in the batch too, so this is a
		// genuine steady-state league, not an unroster-ed bootstrap.
		writePacket(t, inbound, "0-coord", game.Packet{FromBoard: "Coordinator BBS", FromNode: 1, Seq: 1})
		// The forger: a real, roster-listed board that is NOT the
		// Coordinator, lying about its node number. Carries a News line so
		// application order is directly observable in w.NewsToday afterward,
		// regardless of which internal bucket either packet lands in.
		writePacket(t, inbound, "1-forger", game.Packet{FromBoard: "Forger BBS", FromNode: 1, Seq: 1, News: []string{"FORGER"}})
		writePacket(t, inbound, "2-honest", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1, News: []string{"HONEST"}})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 2, Name: "Forger BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		}

		if _, err := ReadInbound(w, inbound, false); err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		forgerAt, honestAt := -1, -1
		for i, line := range w.NewsToday {
			switch line {
			case "FORGER":
				forgerAt = i
			case "HONEST":
				honestAt = i
			}
		}
		if forgerAt == -1 || honestAt == -1 {
			t.Fatalf("trial %d: expected both News lines applied, got %v", trial, w.NewsToday)
		}
		if forgerAt < honestAt {
			appliedFirst++
		}
	}
	t.Logf("Forger BBS applied first in %d/%d trials", appliedFirst, trials)
	if appliedFirst == trials {
		t.Errorf("Forger BBS's forged FromNode: 1 claim won first place in every trial (%d/%d) -- it should only be sometimes, like any other origin",
			appliedFirst, trials)
	}
	if appliedFirst == 0 {
		t.Errorf("Forger BBS never applied first in %d trials -- either broken or statistically implausible, check the test", trials)
	}
}

// TestQuarantinePacketAvoidsNameCollision is a fix from review:
// quarantinePacket originally moved straight to dataDir/bad/<basename>,
// which os.Rename silently overwrites on Unix (destroying the first
// quarantined copy) and fails outright on Windows (propagating an error that
// would abort the whole run -- exactly the failure quarantining exists to
// avoid). It now steps around a collision instead of hitting it.
func TestQuarantinePacketAvoidsNameCollision(t *testing.T) {
	dataDir := t.TempDir()
	first := filepath.Join(t.TempDir(), "dupe.brp")
	second := filepath.Join(t.TempDir(), "dupe.brp")
	if err := os.WriteFile(first, []byte("first corrupt file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("second corrupt file"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := quarantinePacket(dataDir, first); err != nil {
		t.Fatalf("first quarantinePacket: %v", err)
	}
	if err := quarantinePacket(dataDir, second); err != nil {
		t.Fatalf("second quarantinePacket (same basename) should not fail: %v", err)
	}

	entries, err := os.ReadDir(filepath.Join(dataDir, BadDir))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 distinct files in %s after a name collision, got %d: %+v", BadDir, len(entries), entries)
	}
	firstContent, err := os.ReadFile(filepath.Join(dataDir, BadDir, "dupe.brp"))
	if err != nil {
		t.Fatal(err)
	}
	if string(firstContent) != "first corrupt file" {
		t.Errorf("the first quarantined file's content was overwritten: got %q", firstContent)
	}
}

// TestReadInboundQuarantinesUnreadablePacket is concern #3 from #178: one
// corrupt file must not stop the rest of the batch, and must not be silently
// dropped either -- it goes to BadDir for a sysop to find, and every OTHER
// packet in the same run is still applied. The corrupt file's mtime is
// backdated past quarantineGrace: a FRESH corrupt file is deliberately left
// alone (see TestReadInboundLeavesFreshUnreadablePacketAloneForGracePeriod
// for that half), so this test has to simulate one old enough to no longer
// be a plausible in-flight transfer.
func TestReadInboundQuarantinesUnreadablePacket(t *testing.T) {
	inbound := t.TempDir()
	writePacket(t, inbound, "a-good-one", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 1})
	corrupt := filepath.Join(inbound, "b-corrupt.brp")
	if err := os.WriteFile(corrupt, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * quarantineGrace)
	if err := os.Chtimes(corrupt, old, old); err != nil {
		t.Fatal(err)
	}

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound should not abort the batch on a corrupt packet, got: %v", err)
	}
	if result.Applied != 1 {
		t.Errorf("the good packet should still be applied, got applied=%d", result.Applied)
	}
	if result.Quarantined != 1 {
		t.Errorf("the corrupt packet should be counted quarantined, got %d", result.Quarantined)
	}
	if _, err := os.Stat(filepath.Join(inbound, "b-corrupt.brp")); !os.IsNotExist(err) {
		t.Error("the corrupt packet should have been moved out of the inbound directory")
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, BadDir, "b-corrupt.brp")); err != nil {
		t.Errorf("the corrupt packet should be in %s: %v", BadDir, err)
	}
	if len(w.SysopNotices) == 0 {
		t.Error("a quarantined packet should leave a sysop notice explaining why")
	}
}

// TestReadInboundUnreadableFileDoesNotAbortTheRun is a regression test for a
// real finding: a plain os.ReadFile failure (permission denied, an I/O
// fault -- not the file simply having vanished after ReadDir listed it) used
// to abort the whole run, before the JSON-unmarshal failure a few lines
// below it was ever reached. That is the same permanently-blocked-batch
// failure the parse-failure path was already fixed to avoid, just one step
// earlier: a genuinely unreadable file left in inbound would fail the same
// way, in the same place, on every later run too. It now gets the same
// defer-or-quarantine treatment as an unparseable packet, through the same
// deferOrQuarantine helper, and every other packet in the batch still
// applies.
//
// Skipped on Windows and under root, same reasoning and wording as
// TestDirUsableRejectsAnUnwritableDirectory in checkup_test.go: os.Chmod
// cannot make a file unreadable to its own owner on Windows (access lives in
// the ACL, a 0o000 mode there does not stop this process reading its own
// file back), and root ignores the mode bits entirely on the platforms
// where they do apply.
func TestReadInboundUnreadableFileDoesNotAbortTheRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Chmod cannot make a file unreadable to its own owner on Windows; access lives in the ACL")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the read-permission bit this test relies on")
	}
	inbound := t.TempDir()
	writePacket(t, inbound, "a-good-one", game.Packet{FromBoard: "Origin BBS", FromNode: 5, Seq: 1, News: []string{"GOOD"}})
	unreadable := filepath.Join(inbound, "b-no-permission.brp")
	if err := os.WriteFile(unreadable, []byte("{}"), 0o000); err != nil {
		t.Fatal(err)
	}
	// Backdated past quarantineGrace: this test is about the read failure
	// itself not aborting the run, which needs the quarantine path, not the
	// grace-period one already covered by
	// TestReadInboundLeavesFreshUnreadablePacketAloneForGracePeriod.
	old := time.Now().Add(-2 * quarantineGrace)
	if err := os.Chtimes(unreadable, old, old); err != nil {
		t.Fatal(err)
	}

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound aborted the whole run over one unreadable file: %v", err)
	}
	if result.Applied != 1 {
		t.Errorf("the good packet should still be applied despite the unreadable one, got applied=%d", result.Applied)
	}
	if result.Quarantined != 1 {
		t.Errorf("the unreadable packet should be counted quarantined, got %d", result.Quarantined)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, BadDir, "b-no-permission.brp")); err != nil {
		t.Errorf("the unreadable packet should be in %s: %v", BadDir, err)
	}
	found := false
	for _, line := range w.NewsToday {
		if line == "GOOD" {
			found = true
		}
	}
	if !found {
		t.Error("the good packet's News did not apply")
	}
}

// TestReadInboundLeavesFreshUnreadablePacketAloneForGracePeriod: a mailer
// that writes straight to a packet's final name (scp, plain FTP, several FTN
// mailers) can leave a planetary run that lands
// mid-transfer reading truncated JSON. Quarantining that permanently loses a
// packet that would have applied cleanly next run, so a file young enough to
// plausibly still be arriving is left alone instead of moved to BadDir.
func TestReadInboundLeavesFreshUnreadablePacketAloneForGracePeriod(t *testing.T) {
	inbound := t.TempDir()
	corrupt := filepath.Join(inbound, "mid-transfer.brp")
	if err := os.WriteFile(corrupt, []byte("{\"FromBoard\": \"Origin BBS\", tru"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Freshly written -- os.WriteFile's mtime is "now", well inside
	// quarantineGrace.

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if result.Quarantined != 0 {
		t.Errorf("a fresh unreadable packet must not be quarantined yet, got Quarantined=%d", result.Quarantined)
	}
	if result.Deferred != 1 {
		t.Errorf("a fresh unreadable packet left in place must be counted as Deferred, got %d "+
			"(uncounted, a scheduled non-verbose run reports \"No packets waiting\" while it sits in inbound)",
			result.Deferred)
	}
	if _, err := os.Stat(corrupt); err != nil {
		t.Errorf("the fresh packet should still be sitting in inbound to be re-read next run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.DataDir, BadDir, "mid-transfer.brp")); !os.IsNotExist(err) {
		t.Error("the fresh packet should not have been moved to bad/ yet")
	}
}

// TestLastVerifiedOrdersIndex is a direct unit test of lastVerifiedOrdersIndex:
// a packet earns the Coordinator carve-out only when it carries something
// only the Coordinator may send AND verifies against this board's CoordPub —
// neither check alone is enough, since VerifyCoordinatorOrders returns true
// for a packet carrying no coordinator orders at all, and content alone is
// spoofable. The split point it returns is the LAST such packet in Seq
// order, not the first or a simple yes/no, so the last two cases cover that
// specifically: a later verified packet must move the split point forward,
// and a trailing ordinary packet after the last verified one must not be
// pulled in with it.
func TestLastVerifiedOrdersIndex(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer := game.NewWorldSeed(game.DefaultConfig(), 1)
	signer.CoordKey = priv

	sign := func(p game.Packet) game.Packet {
		if err := signer.SignAsCoordinator(&p); err != nil {
			t.Fatalf("SignAsCoordinator: %v", err)
		}
		return p
	}

	w := game.NewWorldSeed(game.DefaultConfig(), 1)
	w.CoordPub = pub

	cases := []struct {
		name  string
		group []stagedPacket
		want  int
	}{
		{"empty group", nil, -1},
		{
			"ordinary gameplay packets only",
			[]stagedPacket{
				{packet: game.Packet{FromBoard: "Coordinator BBS", Seq: 1}},
				{packet: game.Packet{FromBoard: "Coordinator BBS", Seq: 2}},
			},
			-1,
		},
		{
			"one packet among several carries a signed, verified roster update",
			[]stagedPacket{
				{packet: game.Packet{FromBoard: "Coordinator BBS", Seq: 1}},
				{packet: sign(game.Packet{FromBoard: "Coordinator BBS", Seq: 2,
					LeagueNodes: []game.LeagueNode{{Number: 1, Name: "Coordinator BBS"}}})},
			},
			1,
		},
		{
			"league config counts",
			[]stagedPacket{{packet: sign(game.Packet{FromBoard: "Coordinator BBS", LeagueConfig: &game.LeagueConfig{}})}},
			0,
		},
		{
			"bulletins count",
			[]stagedPacket{{packet: sign(game.Packet{FromBoard: "Coordinator BBS", Bulletins: &game.BulletinSet{}})}},
			0,
		},
		{
			"a league reset counts -- dropped by the predecessor predicate",
			[]stagedPacket{{packet: sign(game.Packet{FromBoard: "Coordinator BBS", Reset: &game.LeagueReset{}})}},
			0,
		},
		{
			"claimed but UNSIGNED roster update earns nothing",
			[]stagedPacket{{packet: game.Packet{FromBoard: "Coordinator BBS",
				LeagueNodes: []game.LeagueNode{{Number: 1, Name: "Coordinator BBS"}}}}},
			-1,
		},
		{
			"claimed roster update signed with the WRONG key earns nothing",
			[]stagedPacket{func() stagedPacket {
				otherSigner := game.NewWorldSeed(game.DefaultConfig(), 1)
				_, otherPriv, err := ed25519.GenerateKey(rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				otherSigner.CoordKey = otherPriv
				p := game.Packet{FromBoard: "Coordinator BBS",
					LeagueNodes: []game.LeagueNode{{Number: 1, Name: "Coordinator BBS"}}}
				if err := otherSigner.SignAsCoordinator(&p); err != nil {
					t.Fatalf("SignAsCoordinator: %v", err)
				}
				return stagedPacket{packet: p}
			}()},
			-1,
		},
		{
			// A LATER verified-orders packet must not be
			// left behind just because an earlier one already qualified the
			// group -- the split point has to be the LAST match, index 2, not
			// the first, index 0.
			"a later verified-orders packet moves the split point forward",
			[]stagedPacket{
				{packet: sign(game.Packet{FromBoard: "Coordinator BBS", Seq: 1,
					Bulletins: &game.BulletinSet{}})},
				{packet: game.Packet{FromBoard: "Coordinator BBS", Seq: 2}},
				{packet: sign(game.Packet{FromBoard: "Coordinator BBS", Seq: 3,
					LeagueConfig: &game.LeagueConfig{}})},
			},
			2,
		},
		{
			// A trailing ordinary packet after the last verified one must NOT
			// be pulled into the prefix -- confirms this is genuinely "index
			// of the last VERIFIED packet", not "index of the last packet".
			"a trailing ordinary packet after the last verified one is excluded",
			[]stagedPacket{
				{packet: game.Packet{FromBoard: "Coordinator BBS", Seq: 1}},
				{packet: sign(game.Packet{FromBoard: "Coordinator BBS", Seq: 2,
					LeagueConfig: &game.LeagueConfig{}})},
				{packet: game.Packet{FromBoard: "Coordinator BBS", Seq: 3}},
			},
			1,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastVerifiedOrdersIndex(w, c.group); got != c.want {
				t.Errorf("lastVerifiedOrdersIndex(%s) = %d, want %d", c.name, got, c.want)
			}
		})
	}
}

// TestReadInboundCoordinatorGroupWithoutLeagueUpdateDoesNotJumpQueue: the
// Coordinator's board must not win every
// contested action on every run just by being the Coordinator's -- only a
// group that actually carries league-wide state earns the carve-out.
// Ordinary gameplay packets from the Coordinator's own board, carrying
// nothing but a Seq number, must take their chances in the shuffle like
// anyone else's. Run many trials: under the bug this board's group was
// ALWAYS first; under the fix it is only sometimes first.
func TestReadInboundCoordinatorGroupWithoutLeagueUpdateDoesNotJumpQueue(t *testing.T) {
	appliedFirst := 0
	const trials = 60
	for trial := 0; trial < trials; trial++ {
		inbound := t.TempDir()
		// The Coordinator's own board, but this packet carries nothing
		// league-wide -- an ordinary player action, e.g. a trade bid.
		writePacket(t, inbound, "0-coord", game.Packet{FromBoard: "Coordinator BBS", FromNode: 1, Seq: 1, News: []string{"COORD"}})
		writePacket(t, inbound, "1-honest", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1, News: []string{"HONEST"}})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		}

		if _, err := ReadInbound(w, inbound, false); err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		coordAt, honestAt := -1, -1
		for i, line := range w.NewsToday {
			switch line {
			case "COORD":
				coordAt = i
			case "HONEST":
				honestAt = i
			}
		}
		if coordAt == -1 || honestAt == -1 {
			t.Fatalf("trial %d: expected both News lines applied, got %v", trial, w.NewsToday)
		}
		if coordAt < honestAt {
			appliedFirst++
		}
	}
	t.Logf("Coordinator's ordinary packet applied first in %d/%d trials", appliedFirst, trials)
	if appliedFirst == trials {
		t.Errorf("the Coordinator's board won first place in every trial (%d/%d) despite carrying no league-wide state -- it should only be sometimes, like any other origin",
			appliedFirst, trials)
	}
	if appliedFirst == 0 {
		t.Errorf("the Coordinator's board never applied first in %d trials -- either broken or statistically implausible, check the test", trials)
	}
}

// TestReadInboundCoordinatorGroupWithLeagueUpdateStillAppliesFirst guards
// against a regression of #178 itself: when
// the Coordinator's group DOES carry a roster update, it must still be
// applied before anything else in the batch, every time -- the rest of the
// run's checks (AddressedToMe, Routed) read the roster that update changes.
// The roster update must be signed and verify: the
// carve-out is gated on game.CarriesCoordinatorOrders AND
// w.VerifyCoordinatorOrders together, not on claimed content alone.
func TestReadInboundCoordinatorGroupWithLeagueUpdateStillAppliesFirst(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	const trials = 20
	for trial := 0; trial < trials; trial++ {
		inbound := t.TempDir()
		signer := game.NewWorldSeed(game.DefaultConfig(), 1)
		signer.CoordKey = priv
		coordPacket := game.Packet{
			FromBoard: "Coordinator BBS", FromNode: 1, Seq: 1,
			LeagueNodes: []game.LeagueNode{
				{Number: 1, Name: "Coordinator BBS"},
				{Number: 3, Name: "Honest BBS"},
				{Number: 9, Name: "Receiver BBS"},
			},
			News: []string{"COORD"},
		}
		if err := signer.SignAsCoordinator(&coordPacket); err != nil {
			t.Fatalf("trial %d: SignAsCoordinator: %v", trial, err)
		}
		writePacket(t, inbound, "0-coord", coordPacket)
		writePacket(t, inbound, "1-honest", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1, News: []string{"HONEST"}})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.CoordPub = pub
		w.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		}

		if _, err := ReadInbound(w, inbound, false); err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		if len(w.NewsToday) < 2 || w.NewsToday[0] != "COORD" {
			t.Fatalf("trial %d: the Coordinator's roster-update group must apply first, got NewsToday=%v",
				trial, w.NewsToday)
		}
	}
}

// TestReadInboundOrdinaryCoordinatorPacketDoesNotInheritRosterPriority is
// #178's fairness goal applied to the Coordinator's own board specifically:
// an ordinary gameplay packet must not inherit priority just by riding in
// the same batch as a genuine, verified roster broadcast.
//
// Both packets are produced by the real production call sequence
// (ExportNodeList, StampOutbox) rather than hand-picked Seq values -- an
// earlier version of this test hand-picked Seq: 1 for the roster and Seq: 2
// for the gameplay packet, asserting in a comment that this was "exactly
// what ExportNodeList produces on every real planetary run" without having
// verified it, and a real Coordinator board's Seq ordering is in fact the
// reverse of that by default (a day's gameplay packets already sit in
// Outbox by the time a scheduled run appends the roster). That made the
// test pass without exercising the shape it claimed to. ExportNodeList
// (and ExportLeagueConfig, ExportBulletins) now prepend rather than append
// specifically so the real ordering matches what a correct split needs;
// deriving the Seq values here from those real functions means this test
// can never silently drift out of sync with that again.
func TestReadInboundOrdinaryCoordinatorPacketDoesNotInheritRosterPriority(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	appliedFirst := 0
	const trials = 60
	for trial := 0; trial < trials; trial++ {
		inbound := t.TempDir()

		coordCfg := game.DefaultConfig()
		coordCfg.BoardID = "Coordinator BBS"
		coord := game.NewWorldSeed(coordCfg, 1)
		coord.CoordKey = priv
		coord.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		}
		// An ordinary gameplay packet, queued the way a real player action
		// would be during the day, BEFORE the scheduled planetary run
		// exports the roster.
		coord.Outbox = append(coord.Outbox, game.Packet{FromBoard: "Coordinator BBS", News: []string{"COORD"}})
		coord.ExportNodeList()
		coord.StampOutbox()
		if len(coord.Outbox) != 2 {
			t.Fatalf("trial %d: want 2 packets queued (gameplay + roster), got %d", trial, len(coord.Outbox))
		}
		for _, p := range coord.Outbox {
			writePacket(t, inbound, fmt.Sprintf("%d-coord.brp", p.Seq), p)
		}
		writePacket(t, inbound, "honest.brp", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1, News: []string{"HONEST"}})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.CoordPub = pub
		w.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		}

		if _, err := ReadInbound(w, inbound, false); err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		coordAt, honestAt := -1, -1
		for i, line := range w.NewsToday {
			switch line {
			case "COORD":
				coordAt = i
			case "HONEST":
				honestAt = i
			}
		}
		if coordAt == -1 || honestAt == -1 {
			t.Fatalf("trial %d: expected both News lines applied, got %v", trial, w.NewsToday)
		}
		if coordAt < honestAt {
			appliedFirst++
		}
	}
	t.Logf("Coordinator's ordinary packet applied first in %d/%d trials", appliedFirst, trials)
	if appliedFirst == trials {
		t.Errorf("the Coordinator's ordinary gameplay packet won first place in every trial (%d/%d) by riding alongside its own board's verified roster broadcast -- it should only be sometimes, like any other origin",
			appliedFirst, trials)
	}
	if appliedFirst == 0 {
		t.Errorf("the Coordinator's board never applied first in %d trials -- either broken or statistically implausible, check the test", trials)
	}
}

// TestReadInboundUnsignedLeagueUpdateDoesNotJumpQueue: the carve-out gate
// used to run on claimed packet content alone, before
// any signature was examined, so an origin could buy first-mover priority
// just by setting LeagueNodes on a packet it never had a Coordinator key to
// sign -- no forged FromNode/FromBoard needed, unlike the narrower claim
// TestReadInboundForgedCoordinatorClaimDoesNotJumpTheQueue already covers.
func TestReadInboundUnsignedLeagueUpdateDoesNotJumpQueue(t *testing.T) {
	appliedFirst := 0
	const trials = 60
	for trial := 0; trial < trials; trial++ {
		inbound := t.TempDir()
		// A packet from the Coordinator's own group, claiming a roster
		// update, but never signed -- this board holds no Coordinator key
		// to have signed it with.
		writePacket(t, inbound, "0-coord", game.Packet{
			FromBoard: "Coordinator BBS", FromNode: 1, Seq: 1,
			LeagueNodes: []game.LeagueNode{{Number: 1, Name: "Coordinator BBS"}},
			News:        []string{"COORD"},
		})
		writePacket(t, inbound, "1-honest", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1, News: []string{"HONEST"}})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		_, pub, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		w.CoordPub = pub
		w.LeagueNodes = []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		}

		if _, err := ReadInbound(w, inbound, false); err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		coordAt, honestAt := -1, -1
		for i, line := range w.NewsToday {
			switch line {
			case "COORD":
				coordAt = i
			case "HONEST":
				honestAt = i
			}
		}
		if coordAt == -1 || honestAt == -1 {
			t.Fatalf("trial %d: expected both News lines applied, got %v", trial, w.NewsToday)
		}
		if coordAt < honestAt {
			appliedFirst++
		}
	}
	t.Logf("Coordinator's unsigned roster claim applied first in %d/%d trials", appliedFirst, trials)
	if appliedFirst == trials {
		t.Errorf("an unsigned claimed roster update won first place in every trial (%d/%d) -- it should only be sometimes, like any other origin",
			appliedFirst, trials)
	}
	if appliedFirst == 0 {
		t.Errorf("the Coordinator's board never applied first in %d trials -- either broken or statistically implausible, check the test", trials)
	}
}

// TestReadInboundAppliedOrderIsReproducibleWithFixedShuffleSource:
// ReadInbound used to hard-code rand.Reader at the
// shuffleGroupOrder call site, so nothing could pin the between-origin order
// of a whole batch end to end for a test or a sysop investigating a
// disputed run. inboundShuffleSrc is now a package variable a test can
// substitute. Using the existing alwaysFailReader (shuffleGroupOrder leaves
// its input in scan-built order on any read failure) makes the resulting
// order deterministic without needing a real fixed-sequence RNG -- if
// ReadInbound were still reading rand.Reader directly, repeated runs of the
// same batch would vary; substituting the package var and seeing the SAME
// order every time proves ReadInbound is actually using it.
func TestReadInboundAppliedOrderIsReproducibleWithFixedShuffleSource(t *testing.T) {
	orig := inboundShuffleSrc
	inboundShuffleSrc = alwaysFailReader{}
	defer func() { inboundShuffleSrc = orig }()

	var orders []string
	for trial := 0; trial < 5; trial++ {
		inbound := t.TempDir()
		writePacket(t, inbound, "1", game.Packet{FromBoard: "Alpha BBS", FromNode: 2, Seq: 1, News: []string{"A"}})
		writePacket(t, inbound, "2", game.Packet{FromBoard: "Beta BBS", FromNode: 3, Seq: 1, News: []string{"B"}})
		writePacket(t, inbound, "3", game.Packet{FromBoard: "Gamma BBS", FromNode: 4, Seq: 1, News: []string{"C"}})

		cfg := game.DefaultConfig()
		cfg.BoardID = "Receiver BBS"
		w := game.NewWorldSeed(cfg, 1)
		w.LeagueNodes = []game.LeagueNode{
			{Number: 2, Name: "Alpha BBS"}, {Number: 3, Name: "Beta BBS"},
			{Number: 4, Name: "Gamma BBS"}, {Number: 9, Name: "Receiver BBS"},
		}

		if _, err := ReadInbound(w, inbound, false); err != nil {
			t.Fatalf("trial %d: ReadInbound: %v", trial, err)
		}
		orders = append(orders, strings.Join(w.NewsToday, ","))
	}
	for i := 1; i < len(orders); i++ {
		if orders[i] != orders[0] {
			t.Errorf("order varied across identical trials with a fixed shuffle source (trial 0: %q, trial %d: %q) -- inboundShuffleSrc is not actually controlling ReadInbound's shuffle",
				orders[0], i, orders[i])
		}
	}
}

// TestReadInboundOrderNoticeIncludesCoordinatorGroupAndFiresOnContestedPair:
// the old audit line named `rest`, which excludes
// the Coordinator's group, so it never named the group that actually ran
// first, and its len(rest) > 1 guard suppressed the line entirely on a
// Coordinator-versus-exactly-one-other-board run -- a contested run leaving
// no audit trail at all. OrderNotice now has to name every group that ran,
// in the order they ran, and fire on exactly this two-origin case.
func TestReadInboundOrderNoticeIncludesCoordinatorGroupAndFiresOnContestedPair(t *testing.T) {
	inbound := t.TempDir()
	writePacket(t, inbound, "0-coord", game.Packet{
		FromBoard: "Coordinator BBS", FromNode: 1, Seq: 1,
		LeagueNodes: []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		},
	})
	writePacket(t, inbound, "1-honest", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 3, Name: "Honest BBS"},
		{Number: 9, Name: "Receiver BBS"},
	}

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if result.OrderNotice == "" {
		t.Fatal("a Coordinator-versus-one-other-board run is contested and must produce an OrderNotice")
	}
	if !strings.Contains(result.OrderNotice, "Coordinator BBS") {
		t.Errorf("OrderNotice must name the Coordinator's group, which ran first, not just the shuffled tail: %q", result.OrderNotice)
	}
	if !strings.Contains(result.OrderNotice, "Honest BBS") {
		t.Errorf("OrderNotice must name the other origin too: %q", result.OrderNotice)
	}
}

// TestReadInboundOrderNoticeLabelsASplitCoordinatorGroupDistinctly covers a
// real gap: when the Coordinator's group has a deferred remainder, coordKey
// legitimately names the batch's applied order twice -- once for the
// verified-orders prefix, once again wherever the deferred remainder's turn
// falls in the shuffle -- and the two are not interchangeable. Naming both
// occurrences identically leaves a sysop unable to tell which one carried
// the verified orders, and counting entries as "origins" overstates how
// many distinct boards were actually involved.
func TestReadInboundOrderNoticeLabelsASplitCoordinatorGroupDistinctly(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	inbound := t.TempDir()

	coordCfg := game.DefaultConfig()
	coordCfg.BoardID = "Coordinator BBS"
	coord := game.NewWorldSeed(coordCfg, 1)
	coord.CoordKey = priv
	coord.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 3, Name: "Honest BBS"},
		{Number: 9, Name: "Receiver BBS"},
	}
	coord.Outbox = append(coord.Outbox, game.Packet{FromBoard: "Coordinator BBS", News: []string{"COORD"}})
	coord.ExportNodeList()
	coord.StampOutbox()
	for _, p := range coord.Outbox {
		writePacket(t, inbound, fmt.Sprintf("%d-coord.brp", p.Seq), p)
	}
	writePacket(t, inbound, "honest.brp", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	w := game.NewWorldSeed(cfg, 1)
	w.CoordPub = pub
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 3, Name: "Honest BBS"},
		{Number: 9, Name: "Receiver BBS"},
	}

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("ReadInbound: %v", err)
	}
	if !strings.Contains(result.OrderNotice, "Coordinator BBS (verified orders)") {
		t.Errorf("OrderNotice must mark the verified-orders prefix distinctly: %q", result.OrderNotice)
	}
	if got := strings.Count(result.OrderNotice, "Coordinator BBS"); got != 2 {
		t.Errorf("OrderNotice mentions Coordinator BBS %d times, want 2 (the verified prefix and its deferred remainder): %q",
			got, result.OrderNotice)
	}
	if !strings.Contains(result.OrderNotice, "3 groups") {
		t.Errorf("OrderNotice must count groups (3: the verified prefix, the deferred remainder, and Honest BBS), not origins: %q",
			result.OrderNotice)
	}
}

// TestRunPlanetaryOrderNoticeReachesLogNotTheReport:
// PlanetaryRun.Notices is documented as transport faults for the sysop,
// and runFull's interactive path prints it to the same stdout an active
// door session shares with its caller. The applied-order line must reach
// the sysop's planetary log but never PlanetaryRun.Notices, or an ordinary
// player running a door session could be shown it.
func TestRunPlanetaryOrderNoticeReachesLogNotTheReport(t *testing.T) {
	inbound, outbound := t.TempDir(), t.TempDir()
	writePacket(t, inbound, "0-coord", game.Packet{
		FromBoard: "Coordinator BBS", FromNode: 1, Seq: 1,
		LeagueNodes: []game.LeagueNode{
			{Number: 1, Name: "Coordinator BBS"},
			{Number: 3, Name: "Honest BBS"},
			{Number: 9, Name: "Receiver BBS"},
		},
	})
	writePacket(t, inbound, "1-honest", game.Packet{FromBoard: "Honest BBS", FromNode: 3, Seq: 1})

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = t.TempDir()
	w := game.NewWorldSeed(cfg, 1)
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Coordinator BBS"},
		{Number: 3, Name: "Honest BBS"},
		{Number: 9, Name: "Receiver BBS"},
	}

	run, err := RunPlanetary(w, inbound, outbound, false)
	if err != nil {
		t.Fatalf("RunPlanetary: %v", err)
	}
	for _, n := range run.Notices {
		if strings.Contains(n, "applied in this order") {
			t.Errorf("the applied-order line must not reach PlanetaryRun.Notices (an interactive door session's own stdout): %q", n)
		}
	}
	logContent, err := os.ReadFile(filepath.Join(cfg.DataDir, PlanetaryLogFile))
	if err != nil {
		t.Fatalf("reading %s: %v", PlanetaryLogFile, err)
	}
	if !strings.Contains(string(logContent), "applied in this order") {
		t.Errorf("the applied-order line must reach the sysop's planetary log, got:\n%s", logContent)
	}
}

// TestQuarantinePacketFailureLeavesFileInPlaceAndDoesNotAbortBatch: the
// quarantine notice used to be appended, and the
// success path assumed, BEFORE the move was even attempted. If MkdirAll or
// the move itself failed (read-only data dir, full disk), ReadInbound
// returned the error, aborting the whole run over one bad file -- the exact
// failure quarantining exists to prevent -- and the discarded notice still
// claimed the file "has been set aside" when it never moved. Forces a real
// MkdirAll failure by pointing DataDir at a path that is a plain FILE, not a
// directory, so `<DataDir>/bad` cannot be created.
func TestQuarantinePacketFailureLeavesFileInPlaceAndDoesNotAbortBatch(t *testing.T) {
	inbound := t.TempDir()
	corrupt := filepath.Join(inbound, "b-corrupt.brp")
	if err := os.WriteFile(corrupt, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * quarantineGrace)
	if err := os.Chtimes(corrupt, old, old); err != nil {
		t.Fatal(err)
	}

	// DataDir itself is a plain file: os.MkdirAll(DataDir+"/bad", ...)
	// cannot succeed no matter what quarantinePacket does.
	notADir := filepath.Join(t.TempDir(), "datadir-is-actually-a-file")
	if err := os.WriteFile(notADir, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := game.DefaultConfig()
	cfg.BoardID = "Receiver BBS"
	cfg.DataDir = notADir
	w := game.NewWorldSeed(cfg, 1)

	result, err := ReadInbound(w, inbound, false)
	if err != nil {
		t.Fatalf("a quarantine failure must not abort the batch by returning an error, got: %v", err)
	}
	if result.Quarantined != 0 {
		t.Errorf("nothing was actually quarantined, Quarantined should be 0, got %d", result.Quarantined)
	}
	if _, err := os.Stat(corrupt); err != nil {
		t.Errorf("the file must be left in inbound to retry next run when quarantining itself fails: %v", err)
	}
	found := false
	for _, n := range w.SysopNotices {
		if strings.Contains(n, "could not be set aside") {
			found = true
		}
		if strings.Contains(n, "has been set aside") {
			t.Errorf("must not claim the file was set aside when the move failed: %q", n)
		}
	}
	if !found {
		t.Error("a quarantine failure should leave a sysop notice explaining it, distinct from the normal quarantined-successfully notice")
	}
}

// TestUniqueNameCapsAtMaxQuarantineCopies: without
// a cap, a neighbour whose transport keeps redelivering one broken file
// under the same name accumulates same-named copies without limit, and
// every new arrival re-stats every copy already there. Past
// maxQuarantineCopies, uniqueName has to fail loudly instead of continuing
// to scan an unbounded, ever-growing list.
func TestUniqueNameCapsAtMaxQuarantineCopies(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "dupe.brp")
	// Occupy every slot uniqueName would try: the bare name plus every
	// numbered copy from 2 through maxQuarantineCopies.
	if err := os.WriteFile(base, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for i := 2; i <= maxQuarantineCopies; i++ {
		name := fmt.Sprintf("%s.%d.brp", filepath.Join(dir, "dupe"), i)
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := uniqueName(base); err == nil {
		t.Fatal("expected an error once every slot up to maxQuarantineCopies is taken, got nil")
	}
}
