#!/usr/bin/env python3
"""Calculator for the Real48 runtime linked into BRE 0.988."""

from __future__ import annotations

import argparse
import ast
import json
import re
import sys

from bre_real48 import OPERATIONS, Real48, Real48Error, TurboPascalRandom


def value(text: str) -> Real48:
    try:
        return Real48.parse(text)
    except (ValueError, Real48Error) as exc:
        raise ValueError(str(exc)) from exc


def result_fields(result: Real48) -> dict[str, str]:
    return {
        "decimal": result.to_decimal(),
        "exact_decimal": result.to_decimal(exact=True),
        "memory": result.to_memory(),
        "compact_memory": result.to_memory(""),
    }


def show(result: Real48, output: str) -> None:
    fields = result_fields(result)
    if output == "all":
        print(json.dumps(fields, indent=2))
    else:
        print(fields[output])


MEMORY_LITERAL = re.compile(r"mem:[0-9a-fA-F]{12}")


def evaluate(expression: str) -> int | Real48:
    """Evaluate a small arithmetic expression with Real48 rounding at each step."""
    memories: dict[str, Real48] = {}

    def replace_memory(match: re.Match[str]) -> str:
        name = f"_memory_{len(memories)}"
        memories[name] = value(match.group(0))
        return name

    rewritten = MEMORY_LITERAL.sub(replace_memory, expression)
    try:
        tree = ast.parse(rewritten, mode="eval")
    except SyntaxError as exc:
        raise ValueError(f"invalid expression: {exc.msg}") from exc

    constants = {
        **memories,
        "zero": Real48(),
        "one": Real48.from_int(1),
    }
    unary_functions = {
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

    def real(node: ast.AST) -> Real48:
        result = visit(node)
        if not isinstance(result, Real48):
            raise ValueError("integer-valued operations may only appear at the expression root")
        return result

    def visit(node: ast.AST) -> int | Real48:
        if isinstance(node, ast.Constant) and isinstance(node.value, (int, float)):
            source = ast.get_source_segment(rewritten, node)
            if source is None:
                raise ValueError("cannot recover numeric literal")
            return Real48.from_decimal(source.replace("_", ""))
        if isinstance(node, ast.Name) and node.id in constants:
            return constants[node.id]
        if isinstance(node, ast.UnaryOp) and isinstance(node.op, (ast.UAdd, ast.USub)):
            operand = real(node.operand)
            return operand if isinstance(node.op, ast.UAdd) else -operand
        if isinstance(node, ast.BinOp):
            left, right = real(node.left), real(node.right)
            if isinstance(node.op, ast.Add):
                return left + right
            if isinstance(node.op, ast.Sub):
                return left - right
            if isinstance(node.op, ast.Mult):
                return left * right
            if isinstance(node.op, ast.Div):
                return left / right
        if (
            isinstance(node, ast.Call)
            and isinstance(node.func, ast.Name)
            and not node.keywords
        ):
            if node.func.id in unary_functions and len(node.args) == 1:
                return unary_functions[node.func.id](real(node.args[0]))
            if node.func.id == "compare" and len(node.args) == 2:
                return real(node.args[0]).compare(real(node.args[1]))
        raise ValueError(
            "expressions support decimal or compact mem: values, + - * /, "
            "parentheses, and named Real48 operations"
        )

    return visit(tree.body)


def calculate(args: argparse.Namespace) -> int | Real48:
    values = args.values
    operation = args.operation
    if operation == "decode":
        if len(values) != 1:
            raise ValueError("decode needs one value; pass several values directly to the CLI")
        return value(values[0])
    if operation == "eval":
        if len(values) != 1:
            raise ValueError("eval needs one quoted expression")
        return evaluate(values[0])
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
    parser.add_argument("operation", choices=(*OPERATIONS, "decode", "eval"))
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
        if args.operation == "decode" and len(args.values) > 1:
            decoded = [result_fields(value(item)) for item in args.values]
            if args.output == "all":
                print(json.dumps(decoded, indent=2))
            else:
                print("\n".join(item[args.output] for item in decoded))
            return 0
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
