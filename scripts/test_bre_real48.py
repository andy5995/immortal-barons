#!/usr/bin/env python3
"""Tests for the BRE 0.988-linked Turbo Pascal Real48 implementation."""

from fractions import Fraction
import unittest

from scripts.bre_real48 import (
    HALF_PI,
    LN_TWO,
    ONE,
    PI,
    Real48,
    Real48DivideByZero,
    Real48InvalidArgument,
    Real48Overflow,
    TurboPascalRandom,
)


class FormatTests(unittest.TestCase):
    def test_documented_memory_values(self):
        self.assertEqual(Real48.from_decimal("100").to_memory(""), "mem:870000000048")
        self.assertEqual(Real48.from_memory("87:00:00:00:00:48").to_fraction(), 100)
        self.assertEqual(Real48.from_memory("0x87,0,0,0,0,0x48"), Real48.from_int(100))

    def test_decimal_round_trip(self):
        for text in ("0", "-1.5", "0.1", "913", "1e-38", "1.7e38"):
            value = Real48.from_decimal(text)
            self.assertEqual(Real48.from_decimal(value.to_decimal()), value)
            self.assertEqual(Real48.from_decimal(value.to_decimal(exact=True)), value)

    def test_bias_and_implicit_bit(self):
        self.assertEqual(ONE.to_memory(""), "mem:810000000000")
        self.assertEqual(Real48.from_decimal("0.5").to_memory(""), "mem:800000000000")
        self.assertEqual(Real48.from_decimal("-1").to_memory(""), "mem:810000000080")

    def test_range_boundaries_and_canonical_zero(self):
        minimum = Real48.from_memory("010000000000")
        self.assertEqual(minimum.to_fraction(), Fraction(1, 1 << 128))
        self.assertEqual(Real48.from_memory("00ffffffff7f"), Real48())
        maximum = Real48.from_memory("ffffffffff7f")
        with self.assertRaises(Real48Overflow):
            maximum.scale2(1)

    def test_all_digit_input_is_decimal_not_memory(self):
        self.assertEqual(Real48.parse("123456789012"), Real48.from_int(123456789012))
        self.assertEqual(Real48.parse("0x810000000000"), ONE)


class ArithmeticTests(unittest.TestCase):
    def test_arithmetic(self):
        a = Real48.from_decimal("12.5")
        b = Real48.from_decimal("2.25")
        self.assertEqual((a + b).to_fraction(), Fraction(59, 4))
        self.assertEqual((a - b).to_fraction(), Fraction(41, 4))
        self.assertEqual((a * b).to_fraction(), Fraction(225, 8))
        self.assertEqual((a / b).to_memory(""), "mem:83c7711cc731")
        self.assertEqual(b.square().to_fraction(), Fraction(81, 16))

    def test_compare_and_integer_conversions(self):
        self.assertEqual(Real48.from_decimal("-2.5").compare(Real48.from_int(0)), -1)
        self.assertEqual(Real48.from_decimal("-2.5").trunc(), -2)
        self.assertEqual(Real48.from_decimal("-2.5").round(), -3)
        self.assertEqual(Real48.from_decimal("12.75").integral(), Real48.from_int(12))
        self.assertEqual(Real48.from_decimal("12.75").fractional(), Real48.from_decimal(".75"))

    def test_adder_discards_values_beyond_its_guard_path(self):
        large = Real48.from_int(1)
        too_small = Real48.from_components(False, large.exponent - 41, 1 << 39)
        self.assertEqual(large + too_small, large)

    def test_runtime_errors(self):
        with self.assertRaises(Real48DivideByZero):
            ONE / Real48()
        with self.assertRaises(Real48InvalidArgument):
            Real48.from_int(-1).sqrt()
        with self.assertRaises(Real48InvalidArgument):
            Real48().ln()


class StandardFunctionTests(unittest.TestCase):
    def assertNear(self, actual: Real48, expected: str, tolerance: str = "1e-10"):
        difference = abs(actual.to_fraction() - Real48.from_decimal(expected).to_fraction())
        self.assertLessEqual(difference, Real48.from_decimal(tolerance).to_fraction())

    def test_sqrt(self):
        self.assertNear(Real48.from_int(2).sqrt(), "1.41421356237")
        self.assertEqual(Real48().sqrt(), Real48())

    def test_trigonometric_functions(self):
        self.assertNear(Real48().sin(), "0")
        self.assertNear(HALF_PI.sin(), "1")
        self.assertNear(Real48().cos(), "1")
        self.assertNear(Real48.from_decimal(".5").atan(), "0.463647609001")

    def test_logarithm_and_exponential(self):
        self.assertNear(ONE.ln(), "0")
        self.assertNear(Real48.from_int(2).ln(), LN_TWO.to_decimal())
        self.assertNear(LN_TWO.exp(), "2")
        self.assertNear(Real48.from_int(-1).exp(), "0.367879441171")

    def test_pi_constants_are_the_linked_runtime_values(self):
        self.assertEqual(HALF_PI.to_memory(""), "mem:8121a2da0f49")
        self.assertEqual(PI.to_memory(""), "mem:8221a2da0f49")


class RandomTests(unittest.TestCase):
    def test_runtime_recurrence_and_outputs(self):
        rng = TurboPascalRandom(0)
        self.assertEqual(rng.step(), 1)
        self.assertEqual(rng.step(), 0x08088406)
        rng = TurboPascalRandom(0)
        self.assertEqual(rng.random_int(100), 0)
        rng = TurboPascalRandom(0)
        self.assertEqual(rng.random_real().to_fraction(), Fraction(1, 1 << 32))


if __name__ == "__main__":
    unittest.main()
