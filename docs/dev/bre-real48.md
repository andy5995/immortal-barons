# BRE 0.988 Real48 calculator

`scripts/bre_real48.py` is an integer-only port of the six-byte real-number
runtime linked into BRE 0.988. It is deliberately scoped to that executable,
not to a generic or later implementation of Turbo Pascal `Real`.

The port was made from the resident runtime at logical segment `0fd0` in the
pinned `BRE.EXE`. It includes the complete Real48 operation surface present in
that runtime:

| Operation | Python | Calculator |
|---|---|---|
| addition and subtraction | `a + b`, `a - b` | `add`, `sub` |
| square and multiplication | `a.square()`, `a * b` | `square`, `mul` |
| division | `a / b` | `div` |
| comparison | `a.compare(b)` | `compare` |
| signed-long conversion | `Real48.from_int(n)` | `float` |
| truncate and round to signed long | `a.trunc()`, `a.round()` | `trunc`, `round` |
| real integral/fractional part | `a.integral()`, `a.fractional()` | `int`, `frac` |
| standard functions | `sqrt`, `sin`, `cos`, `ln`, `exp`, `atan` methods | same names |
| TP random outputs | `TurboPascalRandom` | `random-int`, `random-real` |

The corresponding resident entry points are `RAdd` `0fd0:1768`, `RSub`
`0fd0:176e`, `RSqr` `0fd0:1774`, `RMul` `0fd0:177a`, `RDiv` `0fd0:1780`,
`RCmp` `0fd0:178a`, `RFloat` `0fd0:178e`, `RTrunc` `0fd0:1792`, and
`RRound` `0fd0:179a`. The real-valued integral/fractional helpers begin at
`0fd0:17dc` and `0fd0:182d`; the standard-function bodies are `RSqrt`
`0fd0:1841`, the shared sine/cosine reduction and polynomial at
`0fd0:18a0..1913`, `RLn` `0fd0:193e`, `RExp` `0fd0:19e7`, and `RArcTan`
`0fd0:1a8a`. `RandInt` and `RandReal` are `0fd0:1c27` and `0fd0:1c44`.

## Representation and rounding

A memory value is six little-endian bytes. Byte zero is the exponent. The next
39 bits are the stored fraction and the high bit of byte five is the sign. A
nonzero number has an implicit leading significand bit and exponent bias 129:

```text
value = sign * (implicit_one_and_39_fraction_bits / 2^39)
             * 2^(exponent - 129)
```

Exponent zero is zero; there are no denormals, infinities, or NaNs. Arithmetic
normalizes back to a 40-bit significand. Addition follows the linked kernel's
eight guard-bit path, including its lack of a sticky bit, and other operations
round halfway values away from zero as the kernel's `+0x80` path does. Runtime
errors are exposed as exceptions with their TP numbers: division by zero 200,
overflow 205, and invalid arguments/conversions 207.

Neither arithmetic nor decimal conversion passes through a Python `float`.
`to_decimal(exact=True)` prints the exact finite decimal value of the stored
binary number. The default decimal form is the shortest form that parses back
to the same six bytes.

## Calculator

Values can be decimal or explicitly marked memory bytes:

```sh
python3 scripts/bre-real48.py div 100 3
python3 scripts/bre-real48.py sqrt mem:820000000000
python3 scripts/bre-real48.py mul mem:870000000048 0.25 --output memory
python3 scripts/bre-real48.py random-real --seed 0x12345678
```

The default JSON result includes a readable round-tripping decimal, its exact
decimal expansion, spaced memory bytes, and a compact memory form. Every
emitted memory value carries a `mem:` prefix, including compact values, so an
LLM or person cannot mistake it for a hexadecimal integer. `--output` selects
one of those forms. On input, compact hex that contains `a` through `f`, an
explicit `0x` prefix, or a `mem:` prefix is a memory value; all-digit input is
decimal so a twelve-digit decimal is not accidentally interpreted as bytes.

The polynomial coefficients, range-reduction constants, Newton stopping rule,
and RNG recurrence in this module are those embedded in this BRE executable.
Do not replace them with Python `math`, host `libm`, IEEE-754 intermediates, or
constants copied from another Turbo Pascal release when using the calculator
as disassembly evidence.
