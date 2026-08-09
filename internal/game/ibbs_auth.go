package game

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// League packet authentication (#53).
//
// BRE guarded its inter-BBS traffic with CRCs, duplicate detection and a binary
// COORD.KEY that authenticated the League Coordinator. IB's transport is a plain
// file drop, so the two threats worth spending code on are the ones BRE's own
// error strings name: a packet applied twice ("Duplicate or Late Attack Return
// Recieved"), and someone other than the Coordinator dictating the league's
// rules.
//
// A shared league secret cannot do the second job — every member would hold it
// and could forge with it. So the Coordinator holds an ed25519 private key and
// every board holds the matching public key; only the Coordinator can produce a
// signature that verifies, and holding the public key lets nobody forge one.
//
// What this does NOT do, and BRE could not do either: stop a sysop inventing
// their own board's figures. A board is trusted to report itself honestly. The
// point is to stop a board dictating to OTHER boards, and to stop the same
// packet landing twice.
var (
	ErrNotCoordinator = errors.New("this board is not the League Coordinator")
	ErrNoCoordKey     = errors.New("no coordinator key is loaded")
	ErrBadSignature   = errors.New("packet signature does not verify")
)

// signedPayload is the part of a packet that the Coordinator's signature covers:
// the fields that dictate to other boards. Scores and strikes are self-reported
// and deliberately left out — signing them would imply a guarantee nothing can
// give.
type signedPayload struct {
	FromBoard    string
	Seq          uint64
	LeagueConfig *LeagueConfig
	LeagueNodes  []LeagueNode
	Reset        *LeagueReset
}

// signingBytes renders the signed part of p in a stable form. encoding/json
// writes struct fields in declaration order, so the same packet always produces
// the same bytes.
func signingBytes(p Packet) ([]byte, error) {
	return json.Marshal(signedPayload{
		FromBoard:    p.FromBoard,
		Seq:          p.Seq,
		LeagueConfig: p.LeagueConfig,
		LeagueNodes:  p.LeagueNodes,
		Reset:        p.Reset,
	})
}

// carriesCoordinatorOrders reports whether a packet contains anything only the
// Coordinator may send. Those parts are the ones that need a signature.
func carriesCoordinatorOrders(p Packet) bool {
	return p.LeagueConfig != nil || len(p.LeagueNodes) > 0 || p.Reset != nil
}

// SignAsCoordinator signs a packet's coordinator-authored parts. A no-op for a
// packet that carries none.
func (w *World) SignAsCoordinator(p *Packet) error {
	if !carriesCoordinatorOrders(*p) {
		return nil
	}
	if len(w.CoordKey) != ed25519.PrivateKeySize {
		return ErrNoCoordKey
	}
	msg, err := signingBytes(*p)
	if err != nil {
		return err
	}
	p.Signature = ed25519.Sign(ed25519.PrivateKey(w.CoordKey), msg)
	return nil
}

// VerifyCoordinatorOrders reports whether a packet's coordinator-authored parts
// are genuine. A packet carrying no such parts is fine unsigned. When this board
// holds no public key it cannot check, and refuses the orders rather than
// trusting them — an unverifiable instruction is not a safer one.
func (w *World) VerifyCoordinatorOrders(p Packet) bool {
	if !carriesCoordinatorOrders(p) {
		return true
	}
	if len(w.CoordPub) != ed25519.PublicKeySize || len(p.Signature) == 0 {
		return false
	}
	msg, err := signingBytes(p)
	if err != nil {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(w.CoordPub), msg, p.Signature)
}

// NextSeq is this board's next outbound packet number. Sequence numbers only
// ever go up, which is what lets the far side spot a packet it has already
// applied.
func (w *World) NextSeq() uint64 {
	w.OutSeq++
	return w.OutSeq
}

// SeenPacket reports whether a packet has already been applied here, and records
// it if not. A packet with no sequence number is fingerprinted by its contents
// instead, so an older board that sends none is still protected.
func (w *World) SeenPacket(p Packet) bool {
	key := packetKey(p)
	if w.SeenPackets == nil {
		w.SeenPackets = map[string]bool{}
	}
	if w.SeenPackets[key] {
		return true
	}
	// A sequence number that has gone backwards is a replay of something already
	// superseded, even if this exact packet has not been seen before.
	if p.Seq > 0 && p.FromBoard != "" {
		if w.HighSeq == nil {
			w.HighSeq = map[string]uint64{}
		}
		if p.Seq <= w.HighSeq[p.FromBoard] {
			return true
		}
		w.HighSeq[p.FromBoard] = p.Seq
	}
	w.SeenPackets[key] = true
	return false
}

// packetKey identifies a packet for replay detection: its sender and sequence
// when it has one, otherwise a hash of what it carries.
func packetKey(p Packet) string {
	if p.Seq > 0 {
		var b [8]byte
		binary.BigEndian.PutUint64(b[:], p.Seq)
		return p.FromBoard + "#" + fmt.Sprintf("%x", b)
	}
	body, err := json.Marshal(p)
	if err != nil {
		return p.FromBoard + "#" + p.Date
	}
	return fmt.Sprintf("%s#%x", p.FromBoard, sha256.Sum256(body))
}

// StampOutbox numbers every queued packet and signs the ones carrying
// Coordinator orders, just before they are written out. Doing it here rather
// than at each enqueue means nothing can be queued unnumbered (#53).
func (w *World) StampOutbox() {
	for i := range w.Outbox {
		w.Outbox[i].League = w.Config.LeagueNumber
		if w.Outbox[i].Seq == 0 {
			w.Outbox[i].Seq = w.NextSeq()
		}
		if len(w.Outbox[i].Signature) == 0 {
			// A board with no key simply sends unsigned, and the far side refuses
			// any orders in it — which is the intended outcome, not a failure to
			// report here.
			_ = w.SignAsCoordinator(&w.Outbox[i])
		}
	}
}
