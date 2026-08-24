package game

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
)

// legacyPayload is the payload builds before 1da5698 signed, frozen here as a
// fixture. It is deliberately a hand-written struct rather than a call into the
// production code: a bug that renames or reorders a field would otherwise move
// both sides at once and the test would go on passing.
type legacyPayload struct {
	FromBoard    string
	Seq          uint64
	LeagueConfig *LeagueConfig
	LeagueNodes  []LeagueNode
	Reset        *LeagueReset
}

// currentPayload is the same fixture for the shape signed today.
type currentPayload struct {
	FromBoard    string
	Seq          uint64
	LeagueConfig *LeagueConfig
	LeagueNodes  []LeagueNode
	Reset        *LeagueReset
	Bulletins    *BulletinSet
}

// signLegacy signs p the way a Coordinator running a pre-bulletins build did.
func signLegacy(t *testing.T, key ed25519.PrivateKey, p Packet) []byte {
	t.Helper()
	msg, err := json.Marshal(legacyPayload{
		FromBoard:    p.FromBoard,
		Seq:          p.Seq,
		LeagueConfig: p.LeagueConfig,
		LeagueNodes:  p.LeagueNodes,
		Reset:        p.Reset,
	})
	if err != nil {
		t.Fatalf("marshalling the legacy payload: %v", err)
	}
	return ed25519.Sign(key, msg)
}

func sigCompatMember(pub []byte) *World {
	cfg := DefaultConfig()
	cfg.BoardID, cfg.IBBS, cfg.GameLength = "BravoBBS", true, 7
	w := NewWorldSeed(cfg, 1)
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	w.CoordPub = pub
	return w
}

// The bytes a signature is taken over are the wire format, so they must be
// exactly what encoding/json writes for a struct of those fields in that order —
// that is what every deployed signature in every league already covers.
func TestSignedPayloadBytesMatchTheStructFormTheyReplaced(t *testing.T) {
	p := Packet{
		FromBoard:    "AlphaBBS",
		Seq:          9,
		LeagueConfig: &LeagueConfig{GameLength: 42, TurnsPerDay: 15},
		LeagueNodes:  []LeagueNode{{Number: 1, Name: "AlphaBBS"}},
		Bulletins:    &BulletinSet{Files: []BulletinFile{{Name: "rules.txt", Title: "House rules", Data: []byte("play nice")}}},
	}

	want, err := json.Marshal(currentPayload{
		FromBoard:    p.FromBoard,
		Seq:          p.Seq,
		LeagueConfig: p.LeagueConfig,
		LeagueNodes:  p.LeagueNodes,
		Reset:        p.Reset,
		Bulletins:    p.Bulletins,
	})
	if err != nil {
		t.Fatalf("marshalling the fixture: %v", err)
	}
	got, err := signingBytes(p, shapeCurrent)
	if err != nil {
		t.Fatalf("signingBytes: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("current shape renders\n %s\nwant\n %s", got, want)
	}

	p.Bulletins = nil
	want, err = json.Marshal(legacyPayload{
		FromBoard:    p.FromBoard,
		Seq:          p.Seq,
		LeagueConfig: p.LeagueConfig,
		LeagueNodes:  p.LeagueNodes,
		Reset:        p.Reset,
	})
	if err != nil {
		t.Fatalf("marshalling the fixture: %v", err)
	}
	got, err = signingBytes(p, shapePreBulletins)
	if err != nil {
		t.Fatalf("signingBytes: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("legacy shape renders\n %s\nwant\n %s", got, want)
	}
	if n := len(signedFields(p)); n != shapeCurrent {
		t.Errorf("shapeCurrent is %d but the payload has %d fields; append the old length to payloadShapes", shapeCurrent, n)
	}
}

// A Coordinator on a build older than the bulletins field signs a five-field
// payload. A board built since must still obey its orders — this is the break
// that stopped a live six-board league for weeks.
func TestOrdersSignedBeforeBulletinsExistedAreStillObeyed(t *testing.T) {
	priv, pub := testCoordKeys(t)
	p := Packet{FromBoard: "AlphaBBS", Seq: 1, LeagueConfig: &LeagueConfig{GameLength: 42, TurnsPerDay: 15}}
	p.Signature = signLegacy(t, ed25519.PrivateKey(priv), p)

	w := sigCompatMember(pub)
	if !w.VerifyCoordinatorOrders(p) {
		t.Fatal("a legacy-shaped signature was refused")
	}
	w.ApplyPacket(p)
	if w.Config.GameLength != 42 {
		t.Errorf("the order was verified but not applied: game length %d, want 42", w.Config.GameLength)
	}
}

// The fallback must not become a hole: a legacy signature covers no bulletins,
// so a packet carrying them cannot be accepted on one. Anyone could otherwise
// staple a bulletin set onto a genuinely signed roster packet.
func TestALegacySignatureCannotCarryBulletins(t *testing.T) {
	priv, pub := testCoordKeys(t)
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	p := Packet{FromBoard: "AlphaBBS", Seq: 1, LeagueNodes: roster}
	p.Signature = signLegacy(t, ed25519.PrivateKey(priv), p)

	// The signature was taken before this was stapled on.
	p.Bulletins = &BulletinSet{Files: []BulletinFile{{Name: "forged.txt", Title: "Forged", Data: []byte("obey me")}}}

	w := sigCompatMember(pub)
	if w.VerifyCoordinatorOrders(p) {
		t.Fatal("a legacy signature was accepted for a packet carrying bulletins")
	}
	w.ApplyPacket(p)
	if len(w.BulletinDigest) != 0 {
		t.Errorf("unsigned bulletins were recorded: %v", w.BulletinDigest)
	}
}

// The current shape is what a current Coordinator signs, bulletins and all.
func TestOrdersSignedWithTheCurrentPayloadVerify(t *testing.T) {
	priv, pub := testCoordKeys(t)
	signer := sigCompatMember(pub)
	signer.CoordKey = priv

	p := Packet{
		FromBoard:    "AlphaBBS",
		Seq:          1,
		LeagueConfig: &LeagueConfig{GameLength: 42, TurnsPerDay: 15},
		Bulletins:    &BulletinSet{Files: []BulletinFile{{Name: "rules.txt", Title: "House rules", Data: []byte("play nice")}}},
	}
	if err := signer.SignAsCoordinator(&p); err != nil {
		t.Fatalf("signing: %v", err)
	}

	w := sigCompatMember(pub)
	if !w.VerifyCoordinatorOrders(p) {
		t.Fatal("a current-shape signature was refused")
	}
	w.ApplyPacket(p)
	if w.Config.GameLength != 42 {
		t.Errorf("the order was verified but not applied: game length %d, want 42", w.Config.GameLength)
	}
	// The counterpart of the refusal above: this is what a recorded bulletin
	// set looks like, so its absence there means something.
	if len(w.BulletinDigest) == 0 {
		t.Error("the Coordinator's bulletins were not recorded")
	}
}

// Neither shape may rescue a packet that was tampered with or signed by someone
// else. Trying a second shape widens what verifies, so this is the check that
// it did not widen it to everything.
func TestTamperedOrdersAreRefusedUnderEveryShape(t *testing.T) {
	priv, pub := testCoordKeys(t)
	signer := sigCompatMember(pub)
	signer.CoordKey = priv

	genuine := Packet{FromBoard: "AlphaBBS", Seq: 1, LeagueConfig: &LeagueConfig{GameLength: 42, TurnsPerDay: 15}}
	if err := signer.SignAsCoordinator(&genuine); err != nil {
		t.Fatalf("signing: %v", err)
	}
	w := sigCompatMember(pub)
	if !w.VerifyCoordinatorOrders(genuine) {
		t.Fatal("the genuine packet did not verify, so the tamper checks below prove nothing")
	}

	tampered := genuine
	tampered.LeagueConfig = &LeagueConfig{GameLength: 99, TurnsPerDay: 15}
	if w.VerifyCoordinatorOrders(tampered) {
		t.Error("an altered league config verified")
	}

	renamed := genuine
	renamed.FromBoard = "CharlieBBS"
	if w.VerifyCoordinatorOrders(renamed) {
		t.Error("a packet re-attributed to another board verified")
	}

	otherPriv, _ := testCoordKeys(t)
	forged := Packet{FromBoard: "AlphaBBS", Seq: 2, LeagueConfig: &LeagueConfig{GameLength: 98}}
	forged.Signature = signLegacy(t, ed25519.PrivateKey(otherPriv), forged)
	if w.VerifyCoordinatorOrders(forged) {
		t.Error("a legacy-shaped signature made with the wrong key verified")
	}
}
