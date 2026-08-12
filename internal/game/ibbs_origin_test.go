package game

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// signedBoard returns a world configured as `board`, holding a fresh signing
// key, with `peer` on its roster carrying peer's public key.
func signedBoard(t *testing.T, board, peer string, peerPub ed25519.PublicKey) *World {
	t.Helper()
	w := NewWorldSeed(Config{BoardID: board}, 1)
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w.BoardKey = sec
	w.LeagueNodes = []LeagueNode{
		{Number: 1, Name: board, PublicKey: hex.EncodeToString(pub)},
		{Number: 2, Name: peer, PublicKey: hex.EncodeToString(peerPub)},
	}
	return w
}

// TestForgedPacketIsRefused is the issue's own scenario (#118): a file written
// into an inbound directory, naming a board it did not come from. Both named
// harms ride the same packet — units handed to a local realm, and land taken
// off one — so both are checked against the same refusal.
func TestForgedPacketIsRefused(t *testing.T) {
	peerPub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w := signedBoard(t, "Home", "Neighbour", peerPub)
	e := w.AddHuman("tester", "Testland")
	e.Protection = 0
	e.Troopers = 5000
	e.Land = 1000
	before := *e

	// Unsigned, and claiming to be the roster board whose key we hold.
	forged := Packet{
		FromBoard: "Neighbour",
		ToBoard:   "Home",
		// Both harms the issue names, on one packet: survivors credited to a
		// local realm's owner, and a strike aimed at that realm.
		Results: []AttackResult{{ID: 1, Won: true,
			Survivors: []Contribution{{Owner: "tester", AttackForce: AttackForce{Troopers: 999999}}}}},
		Attacks: []RemoteAttack{{ID: 2, FromBoard: "Neighbour", TargetEmpire: "Testland", Offense: 1 << 30}},
	}
	w.ApplyPacket(forged)

	after := w.FindByOwner("tester")
	if after.Troopers != before.Troopers {
		t.Errorf("forged results changed troopers: %d -> %d", before.Troopers, after.Troopers)
	}
	if after.Land != before.Land {
		t.Errorf("forged attack took land: %d -> %d", before.Land, after.Land)
	}
	if !strings.Contains(newsText(w), "did not match that board's key") {
		t.Errorf("the refusal was not reported to the planet:\n%s", newsText(w))
	}
}

// TestSignedPacketIsApplied is the other half. Without it the test above passes
// just as well against a build that refuses everything.
func TestSignedPacketIsApplied(t *testing.T) {
	peerPub, peerSec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w := signedBoard(t, "Home", "Neighbour", peerPub)

	sender := NewWorldSeed(Config{BoardID: "Neighbour"}, 1)
	sender.BoardKey = peerSec
	sender.Outbox = []Packet{{
		FromBoard: "Neighbour",
		ToBoard:   "Home",
		Scores:    []RemoteScore{{Empire: "Farland", NetWorth: 12345}},
	}}
	sender.StampOutbox()
	p := sender.Outbox[0]
	if len(p.BoardSig) == 0 {
		t.Fatal("StampOutbox left the packet unsigned")
	}

	w.ApplyPacket(p)
	if len(w.RemoteBoards) == 0 {
		t.Fatalf("a correctly signed packet was not applied; news:\n%s", newsText(w))
	}
}

// TestTamperedPacketIsRefused covers the case a plain origin check misses: a
// real signed packet whose contents were edited in transit. The signature
// covers the payload, so the edit has to break it.
func TestTamperedPacketIsRefused(t *testing.T) {
	peerPub, peerSec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w := signedBoard(t, "Home", "Neighbour", peerPub)

	sender := NewWorldSeed(Config{BoardID: "Neighbour"}, 1)
	sender.BoardKey = peerSec
	sender.Outbox = []Packet{{
		FromBoard: "Neighbour",
		ToBoard:   "Home",
		Scores:    []RemoteScore{{Empire: "Farland", NetWorth: 100}},
	}}
	sender.StampOutbox()
	p := sender.Outbox[0]
	p.Scores[0].NetWorth = 999999999 // edited after signing

	if ok, checked := w.VerifyBoardOrigin(p); !checked || ok {
		t.Errorf("tampered packet verified: ok=%v checked=%v", ok, checked)
	}
}

// TestHopsDoNotBreakTheSignature pins the one field a forwarding hub is meant
// to change. If Hops were covered, every routed packet would look forged the
// moment it passed through an uplink — which is how signing breaks HOST routing.
func TestHopsDoNotBreakTheSignature(t *testing.T) {
	peerPub, peerSec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w := signedBoard(t, "Home", "Neighbour", peerPub)

	sender := NewWorldSeed(Config{BoardID: "Neighbour"}, 1)
	sender.BoardKey = peerSec
	sender.Outbox = []Packet{{FromBoard: "Neighbour", ToBoard: "Home", Scores: []RemoteScore{{Empire: "Farland"}}}}
	sender.StampOutbox()
	p := sender.Outbox[0]
	p.Hops += 2 // as a hub does

	if ok, checked := w.VerifyBoardOrigin(p); !checked || !ok {
		t.Errorf("a forwarded packet failed its origin check: ok=%v checked=%v", ok, checked)
	}
}

// TestKeylessRosterStillApplies pins the deliberate transition state: a league
// whose Coordinator has not published keys yet keeps working. This is the
// remaining gap, so it is pinned as a decision rather than left to be
// discovered as a surprise.
func TestKeylessRosterStillApplies(t *testing.T) {
	w := NewWorldSeed(Config{BoardID: "Home"}, 1)
	w.LeagueNodes = []LeagueNode{{Number: 2, Name: "Neighbour"}} // no PublicKey
	p := Packet{FromBoard: "Neighbour", ToBoard: "Home", Scores: []RemoteScore{{Empire: "Farland"}}}

	if _, checked := w.VerifyBoardOrigin(p); checked {
		t.Error("a roster entry with no key must report itself unchecked, not failed")
	}
	w.ApplyPacket(p)
	if len(w.RemoteBoards) == 0 {
		t.Error("an unsigned packet must still apply while the roster names no key")
	}
}

// TestCoordinatorSignatureIsCoveredByTheOriginSignature closes the lift-and-
// shift: a genuine Coordinator order copied into a packet from another board.
// The origin signature is taken over the Coordinator signature, so the graft
// cannot survive both checks.
func TestCoordinatorSignatureIsCoveredByTheOriginSignature(t *testing.T) {
	coordPub, coordSec, _ := ed25519.GenerateKey(rand.Reader)
	attackerPub, attackerSec, _ := ed25519.GenerateKey(rand.Reader)

	// The Coordinator authors a real roster order.
	lc := NewWorldSeed(Config{BoardID: "Coord"}, 1)
	lc.CoordKey = coordSec
	lc.Outbox = []Packet{{FromBoard: "Coord", ToBoard: "Home",
		LeagueNodes: []LeagueNode{{Number: 9, Name: "Ninth"}}}}
	lc.StampOutbox()
	genuine := lc.Outbox[0]

	// Another board lifts the signed orders into a packet of its own and signs
	// that packet correctly with its OWN key.
	graft := Packet{FromBoard: "Attacker", ToBoard: "Home",
		LeagueNodes: genuine.LeagueNodes, Signature: genuine.Signature, Seq: genuine.Seq}
	att := NewWorldSeed(Config{BoardID: "Attacker"}, 1)
	att.BoardKey = attackerSec
	att.Outbox = []Packet{graft}
	att.StampOutbox()

	// The Coordinator is node 1 of the roster (CoordinatorBoardID), so "Coord"
	// has to sit there for the graft to be worth attempting at all.
	w := signedBoard(t, "Home", "Attacker", attackerPub)
	w.CoordPub = coordPub
	w.LeagueNodes = append([]LeagueNode{{Number: 1, Name: "Coord"}}, w.LeagueNodes[1:]...)
	w.ApplyPacket(att.Outbox[0])

	for _, n := range w.LeagueNodes {
		if n.Name == "Ninth" {
			t.Error("lifted Coordinator orders were applied from a board that is not the Coordinator")
		}
	}
}

// TestEndToEndSignedExchangeOverTheFileDrop is the whole chain as a sysop runs
// it: two boards, keys generated, the Coordinator's signed roster carrying the
// public halves, and a packet that survives being written to disk and read
// back. The JSON trip is the point — signatures are []byte, which marshals as
// base64, so a packet that verifies in memory can still fail on arrival: two boards, keys generated, the
// Coordinator's signed roster carrying the public halves, and a packet that
// survives being marshalled to disk and back.
func TestEndToEndSignedExchangeOverTheFileDrop(t *testing.T) {
	aPub, aSec, _ := ed25519.GenerateKey(rand.Reader)
	bPub, bSec, _ := ed25519.GenerateKey(rand.Reader)
	roster := []LeagueNode{
		{Number: 1, Name: "Alpha", PublicKey: hex.EncodeToString(aPub)},
		{Number: 2, Name: "Bravo", PublicKey: hex.EncodeToString(bPub)},
	}
	alpha := NewWorldSeed(Config{BoardID: "Alpha"}, 1)
	alpha.BoardKey, alpha.LeagueNodes = aSec, roster
	bravo := NewWorldSeed(Config{BoardID: "Bravo"}, 1)
	bravo.BoardKey, bravo.LeagueNodes = bSec, roster

	alpha.Outbox = []Packet{{FromBoard: "Alpha", ToBoard: "Bravo",
		Scores: []RemoteScore{{Empire: "Alphaland", NetWorth: 4242}}}}
	alpha.StampOutbox()

	// Through the file drop: marshalled out, read back in.
	blob, err := json.Marshal(alpha.Outbox[0])
	if err != nil {
		t.Fatal(err)
	}
	var arrived Packet
	if err := json.Unmarshal(blob, &arrived); err != nil {
		t.Fatal(err)
	}
	if ok, checked := bravo.VerifyBoardOrigin(arrived); !checked || !ok {
		t.Fatalf("a real packet failed after a JSON round trip: ok=%v checked=%v", ok, checked)
	}
	bravo.ApplyPacket(arrived)
	if len(bravo.RemoteBoards) == 0 {
		t.Fatal("the packet was not applied")
	}
}

func newsText(w *World) string { return strings.Join(w.NewsToday, "\n") }

// TestForgeryCannotPoisonTheReplayCounter pins the ORDER of the two ingest
// guards. SeenPacket raises HighSeq[FromBoard] as a side effect, so checking
// replay before origin lets a forged packet carrying a large sequence number
// silently drop every genuine packet from the board it impersonates — a denial
// of service cheaper to mount than the forgery the origin check exists to stop.
// Both guards refuse the same packet either way, which is why only a test that
// looks at what happens AFTERWARDS can tell the orderings apart.
func TestForgeryCannotPoisonTheReplayCounter(t *testing.T) {
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	w := NewWorldSeed(Config{BoardID: "Home"}, 1)
	w.LeagueNodes = []LeagueNode{{Number: 2, Name: "Neighbour", PublicKey: hex.EncodeToString(pub)}}

	w.ApplyPacket(Packet{FromBoard: "Neighbour", ToBoard: "Home", Seq: 999999,
		Scores: []RemoteScore{{Empire: "Junk"}}})
	if got := w.HighSeq["Neighbour"]; got != 0 {
		t.Errorf("a refused packet moved the replay counter to %d", got)
	}

	sender := NewWorldSeed(Config{BoardID: "Neighbour"}, 1)
	sender.BoardKey = sec
	sender.Outbox = []Packet{{FromBoard: "Neighbour", ToBoard: "Home",
		Scores: []RemoteScore{{Empire: "Farland", NetWorth: 500}}}}
	sender.StampOutbox()
	w.ApplyPacket(sender.Outbox[0])

	if len(w.RemoteBoards) == 0 {
		t.Fatal("a forged packet locked out every genuine packet from that board")
	}
}
