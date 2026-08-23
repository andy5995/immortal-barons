package game

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	Bulletins    *BulletinSet
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
		Bulletins:    p.Bulletins,
	})
}

// carriesCoordinatorOrders reports whether a packet contains anything only the
// Coordinator may send. Those parts are the ones that need a signature.
func carriesCoordinatorOrders(p Packet) bool {
	return p.LeagueConfig != nil || len(p.LeagueNodes) > 0 || p.Reset != nil || p.Bulletins != nil
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

// IsPacketSeen reports whether a packet has already been applied here, without
// recording it. Use this for a read-only check — for example, counting skipped
// packets in ReadInbound — where the caller will call SeenPacket or ApplyPacket
// afterwards and needs the side effects to fire exactly once.
func (w *World) IsPacketSeen(p Packet) bool {
	key := packetKey(p)
	if w.SeenPackets != nil && w.SeenPackets[key] {
		return true
	}
	if p.Seq > 0 && p.FromBoard != "" && w.HighSeq != nil && p.Seq <= w.HighSeq[p.FromBoard] {
		return true
	}
	return false
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
//
// The broadcast fan-out happens FIRST, so each addressed copy is signed with
// the destination it will carry. It used to happen further down, in
// store.WriteOutbox, which left every copy signed as the unaddressed broadcast
// it was made from and refused by every board that checked it. The fan-out
// belongs with the signing for that reason: separate them again and the
// signature stops covering what is sent.
func (w *World) StampOutbox() {
	w.Outbox = w.addressBroadcasts(w.Outbox)
	for i := range w.Outbox {
		w.Outbox[i].League = w.Config.LeagueNumber
		// EVERY packet says what this board runs, not just the ones whose
		// builders remembered to. A board's version is a property of the board,
		// and the receiving end tests it on everything that arrives: a mail or
		// recon packet that omitted it read as "states no version", which fails
		// a Coordinator's requirement and got the board bounced while it was
		// running the required version all along. Forwarded packets are left
		// alone — Transit is not stamped here — so a relayed packet keeps the
		// version of the board that wrote it.
		w.Outbox[i].Version = Version
		w.Outbox[i].FromNode = w.NodeNumber(w.Config.BoardID)
		if w.Outbox[i].ToBoard != "" {
			w.Outbox[i].ToNode = w.NodeNumber(w.Outbox[i].ToBoard)
		}
		if w.Outbox[i].Seq == 0 {
			w.Outbox[i].Seq = w.NextSeq()
		}
		if len(w.Outbox[i].Signature) == 0 {
			// A board with no key simply sends unsigned, and the far side refuses
			// any orders in it — which is the intended outcome, not a failure to
			// report here.
			_ = w.SignAsCoordinator(&w.Outbox[i])
		}
		// The origin signature goes on LAST, so it covers the Coordinator
		// signature just written and nothing can lift that onto another packet.
		// A board with no key of its own sends unsigned, exactly as above: the
		// far side applies it only while its roster names no key for us, which
		// is the transition state, not an error to report from here.
		w.Outbox[i].BoardSig = nil
		_ = w.SignAsBoard(&w.Outbox[i])
	}
}

// Per-board packet origin (#118).
//
// The Coordinator signature above answers "may this board dictate to me". It
// says nothing about the rest of a packet, so scores, strikes, terror ops,
// results and mail were applied on the strength of a plain FromBoard string —
// the same string the comment above calls "just a string a file can claim".
// Writing one file into an inbound directory was enough to grant a realm an
// army, or take regions off one.
//
// So every board now signs every packet with a key of its own, and the roster
// carries the matching public half. The two mechanisms have different jobs and
// different trust roots on purpose:
//
//   - coord.key / coord.pub — recorded once by hand, and the anchor. It is what
//     makes the ROSTER trustworthy.
//   - board.key + LeagueNode.PublicKey — distributed inside that signed roster.
//     It is what makes every OTHER packet trustworthy.
//
// The chain closes: a sysop records one key out of band, and every board key
// after that arrives inside something already verifiable.
//
// What it still does not do, and cannot: vouch for the figures a board reports
// about itself, or stop a transport quietly dropping traffic. Origin is not
// honesty, and it is not delivery.

// boardSigningBytes renders a packet for its origin signature: everything it
// carries, with the two fields that legitimately change in transit zeroed.
// BoardSig is the signature itself, and Hops is incremented by each forwarding
// hub. The Coordinator's own Signature IS covered, so it cannot be lifted from
// one packet onto another.
func boardSigningBytes(p Packet) ([]byte, error) {
	p.BoardSig = nil
	p.Hops = 0
	return json.Marshal(p)
}

// SignAsBoard stamps a packet with this board's origin signature.
func (w *World) SignAsBoard(p *Packet) error {
	if len(w.BoardKey) != ed25519.PrivateKeySize {
		return errors.New("no board key is loaded")
	}
	msg, err := boardSigningBytes(*p)
	if err != nil {
		return err
	}
	p.BoardSig = ed25519.Sign(ed25519.PrivateKey(w.BoardKey), msg)
	return nil
}

// BoardPublicKey returns the roster's public key for a board, and whether the
// roster names one. Matched by roster node number when the packet carries one
// (#105) — a number cannot collide the way two boards sharing a name could,
// and survives that board renaming — falling back to the name for a packet, or
// a roster, that predates node identity. A roster written before either
// existed carries no key at all, which is the difference between "cannot
// check" and "failed the check".
func (w *World) BoardPublicKey(board string, node int) (ed25519.PublicKey, bool) {
	if board == "" && node == 0 {
		return nil, false
	}
	for _, n := range w.LeagueNodes {
		if node != 0 {
			if n.Number != node {
				continue
			}
		} else if n.Name != board {
			// Exact match, as fromCoordinator and IsLeagueCoordinator compare
			// board names. A case-insensitive match here would answer "which
			// board is this" differently from the two checks either side of it.
			continue
		}
		raw, err := hex.DecodeString(strings.TrimSpace(n.PublicKey))
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return nil, false
		}
		return ed25519.PublicKey(raw), true
	}
	return nil, false
}

// OriginRefused reports whether p fails the origin check ApplyPacket enforces,
// so a caller can say what happened to it. ApplyPacket answers with an empty
// packet whether it refused one, dropped a replay, or simply had no reply to
// send, and a refusal counted as an application is how a league whose every
// broadcast was being rejected went on reporting "Applied N packets".
func (w *World) OriginRefused(p Packet) bool {
	ok, checked := w.VerifyBoardOrigin(p)
	return checked && !ok
}

// VerifyBoardOrigin checks a packet against the sending board's roster key.
// checked is false when the roster names no key for that board, which is the
// state every league is in until its Coordinator publishes one — see
// ApplyPacket for what is done about it.
func (w *World) VerifyBoardOrigin(p Packet) (ok, checked bool) {
	pub, found := w.BoardPublicKey(p.FromBoard, p.FromNode)
	if !found {
		return false, false
	}
	if len(p.BoardSig) == 0 {
		return false, true
	}
	msg, err := boardSigningBytes(p)
	if err != nil {
		return false, true
	}
	return ed25519.Verify(pub, msg, p.BoardSig), true
}
