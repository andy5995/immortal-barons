package game

import "math/big"

// real48.go — the rounding of the original's six-byte Real, for the figures
// whose exact value depends on it: the terror-op price, what a Bomb Food
// Market or Undermine Investments run destroys, and the S3-Sabre's agent,
// airbase and food rows.
//
// BRE's runtime keeps a 40-bit significand (39 stored bits and an implicit
// leading one) and rounds every product and quotient to it, halves away from
// zero. scripts/bre_real48.py is the full port, made from the resident runtime
// in BRE.EXE; this is the part of it a multiply-and-divide chain needs, and it
// agrees with that port on every figure the tests pin.
//
// Only non-negative operands are handled: no price or holding is negative.

// real48Bits is the significand width, the implicit bit included.
const real48Bits = 40

// r48 rounds x to the nearest value a Real48 can hold.
func r48(x *big.Rat) *big.Rat {
	if x.Sign() <= 0 {
		return new(big.Rat)
	}
	num, den := x.Num(), x.Denom()
	// e is floor(log2 x): the guess from the bit lengths is exact or one high.
	e := num.BitLen() - den.BitLen()
	if cmpShifted(num, den, e) < 0 {
		e--
	}
	// The significand is x scaled into [2^39, 2^40), rounded half up.
	shift := real48Bits - 1 - e
	n, d := new(big.Int).Set(num), new(big.Int).Set(den)
	if shift >= 0 {
		n.Lsh(n, uint(shift))
	} else {
		d.Lsh(d, uint(-shift))
	}
	q, r := new(big.Int).QuoRem(n, d, new(big.Int))
	if r.Lsh(r, 1).Cmp(d) >= 0 {
		q.Add(q, big.NewInt(1))
	}
	out := new(big.Rat).SetInt(q)
	if shift >= 0 {
		return out.Quo(out, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(shift))))
	}
	return out.Mul(out, new(big.Rat).SetInt(new(big.Int).Lsh(big.NewInt(1), uint(-shift))))
}

// cmpShifted compares num with den·2^e.
func cmpShifted(num, den *big.Int, e int) int {
	if e >= 0 {
		return num.Cmp(new(big.Int).Lsh(den, uint(e)))
	}
	return new(big.Int).Lsh(num, uint(-e)).Cmp(den)
}

// r48Mul and r48Div are the runtime's multiply and divide: the exact result,
// rounded once.
func r48Mul(a, b *big.Rat) *big.Rat { return r48(new(big.Rat).Mul(a, b)) }
func r48Div(a, b *big.Rat) *big.Rat { return r48(new(big.Rat).Quo(a, b)) }

// r48Add is the runtime's add. Unlike multiply and divide it is NOT the exact
// result rounded once: 0fd0:1457 aligns the smaller operand with eight guard
// bits and discards whatever shifts past them, with no sticky bit, before it
// rounds (the same path as scripts/bre_real48.py's __add__). Both operands
// must already be Real48 values.
func r48Add(a, b *big.Rat) *big.Rat {
	if a.Sign() <= 0 {
		return r48(b)
	}
	if b.Sign() <= 0 {
		return r48(a)
	}
	ma, ea := r48Parts(a)
	mb, eb := r48Parts(b)
	if ea < eb {
		ma, ea, mb, eb = mb, eb, ma, ea
	}
	if ea-eb >= real48Bits+1 {
		return r48(a)
	}
	sum := ma<<8 + (mb<<8)>>(ea-eb)
	for sum >= 1<<(real48Bits+8) {
		sum >>= 1
		ea++
	}
	m := (sum + 0x80) >> 8
	if m >= 1<<real48Bits {
		m >>= 1
		ea++
	}
	return scaled(new(big.Int).SetUint64(m), ea-(real48Bits-1))
}

// r48Parts splits a Real48 value into its 40-bit significand and floor(log2 x).
func r48Parts(x *big.Rat) (m uint64, e int) {
	num, den := x.Num(), x.Denom()
	e = num.BitLen() - den.BitLen()
	if cmpShifted(num, den, e) < 0 {
		e--
	}
	n := scaled(num, real48Bits-1-e)
	n.Quo(n, new(big.Rat).SetInt(den))
	return n.Num().Uint64(), e
}

// scaled is n x 2^k.
func scaled(n *big.Int, k int) *big.Rat {
	if k >= 0 {
		return new(big.Rat).SetInt(new(big.Int).Lsh(n, uint(k)))
	}
	return new(big.Rat).SetFrac(n, new(big.Int).Lsh(big.NewInt(1), uint(-k)))
}

// r48Int converts a whole number, as the runtime's integer-to-real does.
func r48Int(n int64) *big.Rat { return r48(new(big.Rat).SetInt64(n)) }

// r48Trunc truncates toward zero to a whole number.
func r48Trunc(x *big.Rat) int64 {
	return new(big.Int).Quo(x.Num(), x.Denom()).Int64()
}

// r48Round rounds to the nearest whole number, halves away from zero, as the
// runtime's Round does.
func r48Round(x *big.Rat) int64 {
	n := new(big.Int).Lsh(x.Num(), 1)
	n.Add(n, x.Denom())
	return n.Quo(n, new(big.Int).Lsh(x.Denom(), 1)).Int64()
}
