#!/usr/bin/env python3
"""Calculator for the Real48 runtime linked into BRE 0.988."""

from __future__ import annotations

import argparse
import json
import sys

from bre_real48 import OPERATIONS, Real48, Real48Error, TurboPascalRandom


def value(text: str) -> Real48:
    try:
        return Real48.parse(text)
    except (ValueError, Real48Error) as exc:
        raise ValueError(str(exc)) from exc


def show(result: Real48, output: str) -> None:
    fields = {
        "decimal": result.to_decimal(),
        "exact_decimal": result.to_decimal(exact=True),
        "memory": result.to_memory(),
        "compact_memory": result.to_memory(""),
    }
    if output == "all":
        print(json.dumps(fields, indent=2))
    else:
        print(fields[output])


def calculate(args: argparse.Namespace) -> int | Real48:
    values = args.values
    operation = args.operation
    unary = {
        "square": Real48.square,
        "trunc": Real48.trunc,
        "round": Real48.round,
        "int": Real48.integral,
        "frac": Real48.fractional,
        "sqrt": Real48.sqrt,
        "sin": Real48.sin,
        "cos": Real48.cos,
        "ln": Real48.ln,
        "exp": Real48.exp,
        "atan": Real48.atan,
    }
    binary = {
        "add": Real48.__add__,
        "sub": Real48.__sub__,
        "mul": Real48.__mul__,
        "div": Real48.__truediv__,
        "compare": Real48.compare,
    }
    if operation == "float":
        if len(values) != 1:
            raise ValueError("float needs one signed 32-bit integer")
        integer = int(values[0], 0)
        if not -(1 << 31) <= integer < 1 << 31:
            raise ValueError("float input is outside signed 32-bit range")
        return Real48.from_int(integer)
    if operation in ("random-int", "random-real"):
        rng = TurboPascalRandom(args.seed)
        if operation == "random-real":
            if values:
                raise ValueError("random-real takes no value")
            return rng.random_real()
        if len(values) != 1:
            raise ValueError("random-int needs one word bound")
        return rng.random_int(int(values[0], 0))
    parsed = [value(item) for item in values]
    if operation in unary:
        if len(parsed) != 1:
            raise ValueError(f"{operation} needs one value")
        return unary[operation](parsed[0])
    if len(parsed) != 2:
        raise ValueError(f"{operation} needs two values")
    return binary[operation](parsed[0], parsed[1])


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("operation", choices=OPERATIONS)
    parser.add_argument(
        "values", nargs="*",
        help="decimal or mem:hhhhhhhhhhhh values (integer/bound for float/random-int)",
    )
    parser.add_argument("--seed", type=lambda text: int(text, 0), default=0)
    parser.add_argument(
        "--output",
        choices=("decimal", "exact_decimal", "memory", "compact_memory", "all"),
        default="all",
    )
    args = parser.parse_args()
    try:
        result = calculate(args)
        if isinstance(result, Real48):
            show(result, args.output)
        else:
            print(result)
        return 0
    except (ValueError, Real48Error) as exc:
        if isinstance(exc, Real48Error):
            print(f"bre-real48: runtime error {exc.runtime_error}: {exc}", file=sys.stderr)
        else:
            print(f"bre-real48: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
