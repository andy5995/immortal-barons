package game

import (
	"crypto/ed25519"
	"encoding/hex"
	"strings"
	"testing"
)

// Introducing Protocol must not invalidate a single signature already in the
// wild. That is the whole reason it is omitempty AND zeroed in
// boardSigningBytes: a board that predates the field omits it, and one that has
// it signs as though it did not, so the two agree byte for byte. Getting this
// wrong is what broke a live league when Bulletins was added.
func TestOriginSignatureIgnoresProtocol(t *testing.T) {
	base := Packet{FromBoard: "Alpha BBS", ToBoard: "Bravo BBS", Seq: 7}
	withProto := base
	withProto.Protocol = Protocol

	a, err := boardSigningBytes(base)
	if err != nil {
		t.Fatal(err)
	}
	b, err := boardSigningBytes(withProto)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("Protocol changed the signed bytes, so it breaks every older board:\n old %s\n new %s", a, b)
	}

	// And a signature made without the field still verifies with it present.
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	w := &World{}
	w.Config.BoardID = "Bravo BBS"
	w.BoardKey = priv
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "Alpha BBS", PublicKey: hex.EncodeToString(pub)}, {Number: 2, Name: "Bravo BBS"}}

	signed := base
	if err := w.SignAsBoard(&signed); err != nil {
		t.Fatal(err)
	}
	signed.Protocol = Protocol // as a newer sender would stamp it
	if ok, checked := w.VerifyBoardOrigin(signed); checked && !ok {
		t.Error("a packet signed before Protocol existed no longer verifies with it set")
	}
}

// A board that states no protocol predates the field and speaks exactly this
// format. Treating that as a mismatch would strand every board in the league on
// the release that introduces the mechanism.
func TestAnAbsentProtocolIsNotAMismatch(t *testing.T) {
	if !SpeaksOurProtocol(0) {
		t.Error("a packet with no stated protocol should be readable")
	}
	if !SpeaksOurProtocol(Protocol) {
		t.Error("this build should read its own protocol")
	}
	if SpeaksOurProtocol(Protocol + 1) {
		t.Error("a future protocol should not be treated as readable")
	}
}

// The notice names both numbers and appears once per board, not once per
// packet: a mismatch affects every packet that board sends.
func TestProtocolHoldIsAnnouncedOncePerBoard(t *testing.T) {
	w := &World{}
	w.NoteProtocolHold("Alpha BBS", 9)
	w.NoteProtocolHold("Alpha BBS", 9)
	w.NoteProtocolHold("Bravo BBS", 9)

	if len(w.SysopNotices) != 2 {
		t.Fatalf("got %d notices, want one per board: %q", len(w.SysopNotices), w.SysopNotices)
	}
	if len(w.NewsToday) != 0 {
		t.Errorf("a transport fault reached the planet's news: %q", w.NewsToday)
	}
	for _, want := range []string{"Alpha BBS", "protocol 9"} {
		if !strings.Contains(w.SysopNotices[0], want) {
			t.Errorf("notice should mention %q: %q", want, w.SysopNotices[0])
		}
	}
}
