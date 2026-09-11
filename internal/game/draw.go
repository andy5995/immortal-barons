package game

import (
	"encoding/binary"
	"hash"
	"hash/fnv"
	"io"
)

// draw.go — the deterministic draw (#210).
//
// Several figures have to be reproducible from the state they are keyed on
// rather than drawn from an RNG: the income report must equal what CollectIncome
// credits, and the price a player is quoted must be the price they are charged.
// Each of those was building the same FNV-1a hash by hand — a fixed buffer, a
// little-endian uint32 or three, a tag, the realm's name — and a change to one
// was easy to miss in the others.
//
// What makes this delicate is that the hashed BYTES are the contract. Shift one
// and every price walk and region draw in every saved world moves. So this is
// not one shape imposed on the callers: each writes its own key material in its
// own order, and the chain below makes that order the thing you read.
//
// FNV-1a is a streaming hash over bytes, so writing three uint32s one at a time
// hashes identically to writing the twelve bytes at once, which is what the
// hand-built versions did.

// draw accumulates the key material for one deterministic number.
type draw struct{ h hash.Hash32 }

// newDraw starts a key.
func newDraw() draw { return draw{h: fnv.New32a()} }

// num adds a 32-bit value, little-endian — a game day, a turn counter, a salt.
func (d draw) num(v int) draw {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	d.h.Write(buf[:])
	return d
}

// text adds a string: a tag naming what is being drawn, or the realm's name.
func (d draw) text(s string) draw {
	io.WriteString(d.h, s)
	return d
}

// roll is the number itself, in [0, n). A non-positive bound draws nothing,
// which every caller wanted and each tested for itself.
func (d draw) roll(n int) int {
	if n <= 0 {
		return 0
	}
	return int(d.h.Sum32() % uint32(n))
}
