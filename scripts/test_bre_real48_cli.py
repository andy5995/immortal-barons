#!/usr/bin/env python3
"""CLI tests for the BRE Real48 calculator."""

import importlib.util
from pathlib import Path
import sys
import unittest


SCRIPT = Path(__file__).with_name("bre-real48.py")
SPEC = importlib.util.spec_from_file_location("bre_real48_cli", SCRIPT)
cli = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
sys.path.insert(0, str(SCRIPT.parent))
SPEC.loader.exec_module(cli)


class CalculatorCLITests(unittest.TestCase):
    def test_decode_is_an_identity_operation(self):
        args = type(
            "Args",
            (),
            {"operation": "decode", "values": ["mem:810000000000"], "seed": 0},
        )()
        self.assertEqual(cli.calculate(args).to_decimal(), "1")

    def test_eval_rounds_each_operation_as_real48(self):
        result = cli.evaluate("100000 + (50 / 5)")
        self.assertEqual(result.to_memory(""), "mem:910000005543")
        self.assertEqual(cli.evaluate("trunc(mem:80454772f97f + 1)"), 1)

    def test_eval_rejects_non_calculator_syntax(self):
        with self.assertRaisesRegex(ValueError, "expressions support"):
            cli.evaluate("__import__('os')")


if __name__ == "__main__":
    unittest.main()
