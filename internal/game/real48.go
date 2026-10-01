package game

import "math/big"

// real48.go — the rounding of the original's six-byte Real, for the one price
// whose exact gold figure depends on it.
//
// BRE's runtime keeps a 40-bit significand (39 stored bits and an implicit
// leading one) and rounds every product and quotient to it, halves away from
// zero. scripts/bre_real48.py is the full port, made from the resident runtime
// in BRE.EXE; this is the part of it a multiply-and-divide chain needs, and it
// agrees with that port on every figure the tests pin.
//
// Only non-negative operands are handled: no price is negative.

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

// r48Int converts a whole number, as the runtime's integer-to-real does.
func r48Int(n int64) *big.Rat { return r48(new(big.Rat).SetInt64(n)) }

// r48Trunc truncates toward zero to a whole number.
func r48Trunc(x *big.Rat) int64 {
	return new(big.Int).Quo(x.Num(), x.Denom()).Int64()
}
