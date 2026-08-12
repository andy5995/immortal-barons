#!/usr/bin/env python3
"""Exact Real48 arithmetic used by the runtime linked into BRE 0.988.

The six-byte memory layout is::

    exponent, mantissa bits 0..7, ..., mantissa bits 32..38 + sign bit

Non-zero values have a bias-129 exponent and an implicit leading mantissa bit.
All basic arithmetic is integer based; Python binary floating point is never
used.  The standard functions use the constants and rounded operation sequence
from BRE.EXE's resident Turbo Pascal runtime at logical segment 0fd0.
"""

from __future__ import annotations

from dataclasses import dataclass
from decimal import Decimal, localcontext
from fractions import Fraction
import re


SIGN = 1 << 39
HIDDEN = 1 << 39
MANTISSA_MASK = HIDDEN - 1
MAX_MANTISSA = (1 << 40) - 1


class Real48Error(ArithmeticError):
    """Base class for BRE Turbo Pascal Real48 runtime errors."""

    runtime_error: int

    def __init__(self, message: str, runtime_error: int):
        super().__init__(message)
        self.runtime_error = runtime_error


class Real48DivideByZero(Real48Error):
    def __init__(self):
        super().__init__("Real48 division by zero", 200)


class Real48Overflow(Real48Error):
    def __init__(self):
        super().__init__("Real48 overflow", 205)


class Real48InvalidArgument(Real48Error):
    def __init__(self, operation: str):
        super().__init__(f"invalid argument to Real48 {operation}", 207)


_DECIMAL_RE = re.compile(
    r"^[+-]?(?:(?:[0-9]+(?:\.[0-9]*)?)|(?:\.[0-9]+))(?:[eE][+-]?[0-9]+)?$"
)


def _round_ratio_half_up(numerator: int, denominator: int) -> int:
    quotient, remainder = divmod(numerator, denominator)
    return quotient + (remainder * 2 >= denominator)


def _floor_log2(value: Fraction) -> int:
    numerator = value.numerator
    denominator = value.denominator
    guess = numerator.bit_length() - denominator.bit_length()
    if guess >= 0:
        return guess if numerator >= denominator << guess else guess - 1
    return guess if numerator << -guess >= denominator else guess - 1


@dataclass(frozen=True, slots=True)
class Real48:
    """One canonical Turbo Pascal six-byte Real value."""

    bits: int = 0

    def __post_init__(self) -> None:
        if not 0 <= self.bits < 1 << 48:
            raise ValueError("Real48 bits must fit in six bytes")
        if self.exponent == 0 and self.bits != 0:
            object.__setattr__(self, "bits", 0)

    @property
    def exponent(self) -> int:
        return self.bits & 0xFF

    @property
    def negative(self) -> bool:
        return bool(self.bits & (1 << 47)) and self.exponent != 0

    @property
    def mantissa(self) -> int:
        if self.exponent == 0:
            return 0
        stored = (self.bits >> 8) & MAX_MANTISSA
        return HIDDEN | (stored & MANTISSA_MASK)

    @classmethod
    def from_components(cls, negative: bool, exponent: int, mantissa: int) -> "Real48":
        if exponent <= 0 or mantissa == 0:
            return cls()
        if exponent > 0xFF or not HIDDEN <= mantissa <= MAX_MANTISSA:
            raise Real48Overflow()
        stored = mantissa & MANTISSA_MASK
        if negative:
            stored |= SIGN
        return cls(exponent | (stored << 8))

    @classmethod
    def from_bytes(cls, memory: bytes | bytearray | memoryview) -> "Real48":
        raw = bytes(memory)
        if len(raw) != 6:
            raise ValueError("a Real48 memory value is exactly six bytes")
        return cls(int.from_bytes(raw, "little"))

    @classmethod
    def from_memory(cls, text: str) -> "Real48":
        """Parse six bytes as compact hex or separated byte values.

        Accepted examples: ``870000000048``, ``87:00:00:00:00:48``, and
        ``0x87,0,0,0,0,0x48``.
        """
        value = text.strip()
        if value.lower().startswith("mem:"):
            value = value[4:].strip()
        compact = value.removeprefix("0x").removeprefix("0X")
        if re.fullmatch(r"[0-9a-fA-F]{12}", compact):
            return cls.from_bytes(bytes.fromhex(compact))
        fields = [field for field in re.split(r"[\s,:;]+", value) if field]
        if len(fields) != 6:
            raise ValueError("memory format needs six bytes")
        parsed = []
        for field in fields:
            if field.lower().startswith("0x"):
                byte = int(field, 16)
            elif re.search(r"[a-fA-F]", field) or len(field) == 2:
                byte = int(field, 16)
            else:
                byte = int(field, 10)
            if not 0 <= byte <= 0xFF:
                raise ValueError(f"memory byte is out of range: {field}")
            parsed.append(byte)
        return cls.from_bytes(bytes(parsed))

    @classmethod
    def from_decimal(cls, text: str) -> "Real48":
        value = text.strip()
        if not _DECIMAL_RE.fullmatch(value):
            raise ValueError(f"invalid decimal Real48 value: {text!r}")
        sign = -1 if value.startswith("-") else 1
        unsigned = value.lstrip("+-")
        mantissa_text, marker, exponent_text = unsigned.lower().partition("e")
        decimal_exponent = int(exponent_text) if marker else 0
        whole, dot, fractional = mantissa_text.partition(".")
        digits = (whole or "0") + fractional
        numerator = int(digits or "0") * sign
        scale = len(fractional) - decimal_exponent
        fraction = (
            Fraction(numerator, 10**scale)
            if scale >= 0
            else Fraction(numerator * 10 ** -scale)
        )
        return cls.from_fraction(fraction)

    @classmethod
    def parse(cls, text: str) -> "Real48":
        value = text.strip()
        if (
            value.lower().startswith("mem:")
            or re.fullmatch(r"0[xX][0-9a-fA-F]{12}", value)
            or re.fullmatch(r"[0-9a-fA-F]*[a-fA-F][0-9a-fA-F]*", value)
        ):
            return cls.from_memory(value)
        return cls.from_decimal(value)

    @classmethod
    def from_int(cls, value: int) -> "Real48":
        return cls.from_fraction(Fraction(value))

    @classmethod
    def from_fraction(cls, value: Fraction) -> "Real48":
        if not value:
            return cls()
        negative = value < 0
        magnitude = abs(value)
        binary_exponent = _floor_log2(magnitude)
        exponent = binary_exponent + 129
        if exponent <= 0:
            return cls()
        if exponent > 0xFF:
            raise Real48Overflow()
        shift = 39 - binary_exponent
        if shift >= 0:
            numerator = magnitude.numerator << shift
            denominator = magnitude.denominator
        else:
            numerator = magnitude.numerator
            denominator = magnitude.denominator << -shift
        mantissa = _round_ratio_half_up(numerator, denominator)
        if mantissa == 1 << 40:
            mantissa >>= 1
            exponent += 1
            if exponent > 0xFF:
                raise Real48Overflow()
        return cls.from_components(negative, exponent, mantissa)

    def to_bytes(self) -> bytes:
        return self.bits.to_bytes(6, "little")

    def to_memory(self, separator: str = " ") -> str:
        """Return an explicitly decorated six-byte memory literal."""
        return "mem:" + separator.join(f"{byte:02x}" for byte in self.to_bytes())

    def to_fraction(self) -> Fraction:
        if self.exponent == 0:
            return Fraction()
        value = Fraction(self.mantissa)
        shift = self.exponent - 168
        value = value * (1 << shift) if shift >= 0 else value / (1 << -shift)
        return -value if self.negative else value

    def to_decimal(self, *, exact: bool = False) -> str:
        """Return an exact or shortest round-tripping decimal representation."""
        value = self.to_fraction()
        if not value:
            return "0"
        with localcontext() as context:
            context.prec = 180
            decimal_value = Decimal(value.numerator) / Decimal(value.denominator)
            if exact:
                result = format(decimal_value, "f")
                return result.rstrip("0").rstrip(".") if "." in result else result
            for digits in range(1, 16):
                candidate = format(decimal_value, f".{digits}g")
                try:
                    if Real48.from_decimal(candidate) == self:
                        return candidate.replace("E", "e")
                except Real48Overflow:
                    continue
        raise AssertionError("a Real48 value must round-trip within 15 decimal digits")

    def __str__(self) -> str:
        return self.to_decimal()

    def __bool__(self) -> bool:
        return self.exponent != 0

    def __neg__(self) -> "Real48":
        return Real48() if not self else Real48(self.bits ^ (1 << 47))

    def __abs__(self) -> "Real48":
        return -self if self.negative else self

    def _coerce(self, other: object) -> "Real48":
        if isinstance(other, Real48):
            return other
        if isinstance(other, int):
            return Real48.from_int(other)
        return NotImplemented

    def __add__(self, other: object) -> "Real48":
        right = self._coerce(other)
        if right is NotImplemented:
            return NotImplemented
        if not self:
            return right
        if not right:
            return self

        # 0fd0:1457 aligns the smaller operand with eight guard bits.  Bits
        # shifted beyond that guard byte are discarded, not represented as a
        # sticky bit, so reproduce that path rather than adding Fractions.
        if self.exponent > right.exponent:
            larger, smaller = self, right
        else:
            larger, smaller = right, self
        difference = larger.exponent - smaller.exponent
        if difference >= 41:
            return larger
        large_extended = larger.mantissa << 8
        small_extended = (smaller.mantissa << 8) >> difference
        total = (-large_extended if larger.negative else large_extended) + (
            -small_extended if smaller.negative else small_extended
        )
        if not total:
            return Real48()
        negative = total < 0
        extended = abs(total)
        exponent = larger.exponent
        while extended >= 1 << 48:
            extended >>= 1
            exponent += 1
        while extended < 1 << 47:
            extended <<= 1
            exponent -= 1
            if exponent <= 0:
                return Real48()
        mantissa = (extended + 0x80) >> 8
        if mantissa >= 1 << 40:
            mantissa >>= 1
            exponent += 1
        return Real48.from_components(negative, exponent, mantissa)

    def __radd__(self, other: object) -> "Real48":
        return self + other

    def __sub__(self, other: object) -> "Real48":
        right = self._coerce(other)
        return NotImplemented if right is NotImplemented else self + -right

    def __rsub__(self, other: object) -> "Real48":
        left = self._coerce(other)
        return NotImplemented if left is NotImplemented else left - self

    def __mul__(self, other: object) -> "Real48":
        right = self._coerce(other)
        if right is NotImplemented:
            return NotImplemented
        return Real48.from_fraction(self.to_fraction() * right.to_fraction())

    def __rmul__(self, other: object) -> "Real48":
        return self * other

    def __truediv__(self, other: object) -> "Real48":
        right = self._coerce(other)
        if right is NotImplemented:
            return NotImplemented
        if not right:
            raise Real48DivideByZero()
        return Real48.from_fraction(self.to_fraction() / right.to_fraction())

    def __rtruediv__(self, other: object) -> "Real48":
        left = self._coerce(other)
        return NotImplemented if left is NotImplemented else left / self

    def square(self) -> "Real48":
        return self * self

    def compare(self, other: "Real48") -> int:
        left_value = self.to_fraction()
        right_value = other.to_fraction()
        return (left_value > right_value) - (left_value < right_value)

    def __lt__(self, other: object) -> bool:
        right = self._coerce(other)
        return NotImplemented if right is NotImplemented else self.compare(right) < 0

    def __le__(self, other: object) -> bool:
        right = self._coerce(other)
        return NotImplemented if right is NotImplemented else self.compare(right) <= 0

    def __gt__(self, other: object) -> bool:
        right = self._coerce(other)
        return NotImplemented if right is NotImplemented else self.compare(right) > 0

    def __ge__(self, other: object) -> bool:
        right = self._coerce(other)
        return NotImplemented if right is NotImplemented else self.compare(right) >= 0

    def trunc(self) -> int:
        value = self.to_fraction()
        result = abs(value.numerator) // value.denominator
        result = -result if value < 0 else result
        if not -(1 << 31) <= result < 1 << 31:
            raise Real48InvalidArgument("truncation")
        return result

    def round(self) -> int:
        value = self.to_fraction()
        magnitude = abs(value)
        result = _round_ratio_half_up(magnitude.numerator, magnitude.denominator)
        result = -result if value < 0 else result
        if not -(1 << 31) <= result < 1 << 31:
            raise Real48InvalidArgument("rounding")
        return result

    def integral(self) -> "Real48":
        value = self.to_fraction()
        result = abs(value.numerator) // value.denominator
        return Real48.from_int(-result if value < 0 else result)

    def fractional(self) -> "Real48":
        return self - self.integral()

    def sqrt(self) -> "Real48":
        if not self:
            return self
        if self.negative:
            raise Real48InvalidArgument("sqrt")
        # 0fd0:1841 seeds Newton-Raphson by halving the biased exponent, then
        # iterates (x/y+y)/2 until the correction exponent reaches e/2-20.
        signed_exponent = (self.exponent + 0x80) & 0xFF
        if signed_exponent & 0x80:
            signed_exponent -= 0x100
        estimate_exponent = ((signed_exponent >> 1) + 0x80) & 0xFF
        threshold = (estimate_exponent - 0x14) & 0xFF
        estimate = Real48.from_components(False, estimate_exponent & 0xFF, self.mantissa)
        while True:
            updated = (self / estimate + estimate).scale2(-1)
            correction = updated - estimate
            estimate = updated
            if correction.exponent < threshold:
                return estimate

    def scale2(self, amount: int) -> "Real48":
        if not self:
            return self
        exponent = self.exponent + amount
        if exponent <= 0:
            return Real48()
        return Real48.from_components(self.negative, exponent, self.mantissa)

    def sin(self) -> "Real48":
        if not self:
            return self
        if self.exponent < 0x6C:
            return self
        value = self
        if abs(value) >= TWO_PI:
            value = (value / TWO_PI).fractional() * TWO_PI
        if value.negative:
            value = value + TWO_PI
        negate_result = value >= PI
        if negate_result:
            value = value - PI
        if value >= HALF_PI:
            value = PI - value
        result = _odd_polynomial(value, SIN_COEFFICIENTS)
        return -result if negate_result else result

    def cos(self) -> "Real48":
        return (self + HALF_PI).sin()

    def ln(self) -> "Real48":
        if not self or self.negative:
            raise Real48InvalidArgument("ln")
        power = self.exponent - 129
        normalized = Real48.from_components(False, 129, self.mantissa)
        z = normalized * INV_SQRT_TWO
        ratio = (z - ONE) / (z + ONE)
        series = _odd_polynomial(ratio, LN_COEFFICIENTS).scale2(1) + HALF_LN_TWO
        result = Real48.from_int(power) * LN_TWO + series
        return Real48() if result and result.exponent < 0x67 else result

    def exp(self) -> "Real48":
        if not self:
            return ONE
        negative = self.negative
        quotient = abs(self) / LN_TWO
        if quotient.exponent >= 0x88:
            raise Real48Overflow()
        twice = quotient.scale2(1)
        k = twice.round()
        reduced = quotient - Real48.from_int(k).scale2(-1)
        result = _polynomial(reduced, EXP_COEFFICIENTS)
        half_power, odd = divmod(k, 2)
        if odd:
            result = result * SQRT_TWO
        result = result.scale2(half_power)
        return ONE / result if negative else result

    def atan(self) -> "Real48":
        if not self:
            return self
        negative = self.negative
        value = abs(self)
        complement = False
        if value >= ONE:
            value = ONE / value
            complement = True
        if value < ATAN_SMALL_LIMIT:
            result = _odd_polynomial(value, ATAN_COEFFICIENTS)
        else:
            selected = ATAN_REDUCTIONS[-1]
            for limit, tangent, angle in ATAN_REDUCTIONS:
                selected = (limit, tangent, angle)
                if value < limit:
                    break
            _limit, tangent, angle = selected
            reduced = (value - tangent) / (ONE + value * tangent)
            result = _odd_polynomial(reduced, ATAN_COEFFICIENTS) + angle
        if complement:
            result = HALF_PI - result
        return -result if negative else result


def _memory(hex_bytes: str) -> Real48:
    return Real48.from_memory(hex_bytes)


def _polynomial(value: Real48, coefficients: tuple[Real48, ...]) -> Real48:
    accumulator = coefficients[0]
    for coefficient in coefficients[1:]:
        accumulator = accumulator * value + coefficient
    return accumulator * value + ONE


def _odd_polynomial(value: Real48, coefficients: tuple[Real48, ...]) -> Real48:
    return value * _polynomial(value.square(), coefficients)


ZERO = Real48()
ONE = _memory("810000000000")
HALF_PI = -_memory("8121a2da0fc9")
PI = HALF_PI.scale2(1)
TWO_PI = _memory("8321a2da0f49")
INV_SQRT_TWO = _memory("80fb33f30435")
SQRT_TWO = _memory("81fb33f30435")
LN_TWO = _memory("80d2f7177231")
HALF_LN_TWO = _memory("7fd2f7177231")

SIN_COEFFICIENTS = tuple(
    _memory(value)
    for value in (
        "589d399f3fd7", "60439d309230", "67aa3f2832d7",
        "6eb62a1def38", "740dd0000dd0", "7a8888888808",
        "7eabaaaaaaaa",
    )
)
LN_COEFFICIENTS = tuple(
    _memory(value)
    for value in (
        "7d8a9dd8891d", "7de9a28b2e3a", "7d8ee3388e63",
        "7e4992244912", "7ecdcccccc4c", "7fabaaaaaa2a",
    )
)
EXP_COEFFICIENTS = tuple(
    _memory(value)
    for value in (
        "6d2e1d116031", "70462cfee57f", "74367c898421",
        "77533cffc32e", "7ad27d5b951d", "7c25b8465863",
        "7e16fceffd75", "80d2f7177231",
    )
)
ATAN_COEFFICIENTS = tuple(
    _memory(value)
    for value in (
        "7de8a28b2eba", "7d8ee3388e63", "7e4992244992",
        "7ecdcccccc4c", "7fabaaaaaaaa",
    )
)
ATAN_SMALL_LIMIT = _memory("7e4a8ee96f0c")
ATAN_REDUCTIONS = (
    (_memory("7fe7cfcc1354"), _memory("7ff6f4a23009"), _memory("7f6ac1910a06")),
    (_memory("80b59e8a6f44"), _memory("80822c3acd13"), _memory("806ac1910a06")),
    (ONE, ONE, _memory("8021a2da0f49")),
)


class TurboPascalRandom:
    """The 32-bit RNG recurrence linked into BRE 0.988."""

    def __init__(self, seed: int = 0):
        self.seed = seed & 0xFFFFFFFF

    def step(self) -> int:
        self.seed = (self.seed * 0x08088405 + 1) & 0xFFFFFFFF
        return self.seed

    def random_int(self, bound: int) -> int:
        if not 0 <= bound <= 0xFFFF:
            raise ValueError("Turbo Pascal Random(n) bound must be a word")
        return (self.step() * bound) >> 32

    def random_real(self) -> Real48:
        return Real48.from_fraction(Fraction(self.step(), 1 << 32))


OPERATIONS = (
    "add", "sub", "square", "mul", "div", "compare", "float",
    "trunc", "round", "int", "frac", "sqrt", "sin", "cos", "ln",
    "exp", "atan", "random-int", "random-real",
)
