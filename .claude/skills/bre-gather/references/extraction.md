# BRE extraction cookbook

Concrete commands for pulling ground truth out of BRE's files. Set `BRE` to
wherever you unpacked the original BRE distribution (see SKILL.md), e.g.
`BRE=~/path/to/bre-dos`.

## 1. Find a menu's labels

```
strings -n 3 "$BRE/BRE.OVR" | grep -iE "group attack|travel time|terrorist|sdi|gooie|spy database"
```

Broad first, then narrow. Labels appear in **declaration order**, which is the
rendered menu order (columns fill top-to-bottom, then the lettered items).

## 2. Locate a string's byte offset

```
grep -aboF "Create Group Attack" "$BRE/BRE.OVR" | head -1 | cut -d: -f1
```

`-a` treat binary as text, `-b` print byte offset, `-o` match only, `-F` fixed
string. Beware duplicate matches — a label like "Terrorist Ops" also appears
inside a config string ("Maximum Terrorist Ops Per Day"). Pick the offset that
sits in the menu cluster (dump and eyeball).

## 3. Dump the record structure around an offset

```
off=$(grep -aboF "Gooie Kablooie Ops" "$BRE/BRE.OVR" | head -1 | cut -d: -f1)
dd if="$BRE/BRE.OVR" bs=1 skip=$((off-64)) count=200 2>/dev/null | xxd | head -14
```

Read the hex: each Pascal ShortString is `<len byte><bytes>`. Consecutive
strings = consecutive menu items. Worked example (IP-menu cluster):

```
0d "View IPScores"  0d "Terrorist Ops"  0f "Send Trade Deal"  13 "Create Group Attack"
11 "Join Group Attack"  13 "Indiv. Attack Force"  0c "Send Message"  12 "Special Operations"
12 "Gooie Kablooie Ops"  0b "SDI Program"  0e "Diplomacy List"  0c "Spy Database"  0c "Travel Times"
```

`0x0d`=13=len("View IPScores"), `0x13`=19=len("Create Group Attack"), etc. The
length byte is the ONLY metadata beside the string — no color, no hotkey.

## 4. A guided flow vs a sub-menu

Some entries (e.g. Gooie Kablooie Ops) are not a multi-option menu but a
prompt-driven flow. You spot this when the nearby strings are questions and
fill-ins rather than a clean list:

```
strings -n 3 "$BRE/BRE.OVR" | grep -iE "would you like|how many|awaiting launch|do you wish"
```

## 5. Colors — not here

Colors are draw-code attributes, not in the string table. Read them off a
rendered screenshot (run BRE, or ask a contributor who can) or a disassembly of
the original binary. Do not try to infer them from the OVR.

## 6. Disassembling code (when strings aren't enough)

Reading BRE's *code* (menu dispatch, the shape of a routine) with `ndisasm`:

```
# function prologues are 55 89 E5 (push bp; mov bp,sp); find them:
grep -aboF $'\x55\x89\xe5' "$BRE/BRE.OVR" | head
# disassemble a slice as 16-bit real-mode from a file offset:
ndisasm -b 16 -o <off> <(dd if="$BRE/BRE.OVR" bs=1 skip=<off> count=320 2>/dev/null)
```

**Code masquerading as strings (a second length-prefix trap).** A run like
`<1u <2u <3u <4u` right after a menu-name cluster (e.g. Civilian/Economic/
Military/Technology) is **not** strings — it's the menu **dispatch code**:
`3C 31 75` = `cmp al,'1'` / `jnz`, one compare-and-branch per item. `strings`
prints it as `<1u` because those bytes are printable. So a "cluster" of tiny
2–3 char strings after a real string list is usually a `cmp al,'N'; jnz` chain —
disassemble it to confirm the menu's item→routine order (which strings alone
cannot give you — see SKILL.md on reachability).

**The real-arithmetic wall — why exact magnitude formulas resist disassembly.**
BRE is Turbo Pascal, and its 6-byte `Real` type is **software** floating point:
add/mul/div/compare are done by **far calls to RTL helper routines** (the
`call word 0xfd0:0x…` you'll see everywhere), not FPU ops or readable
`imul`/`idiv`. Combined with overlay relocation (targets are fixed up at load
time, so file offsets ≠ runtime segment:offset) and no symbols, a per-turn
magnitude formula reads as RTL-call soup, not arithmetic. This is the same wall
that blocked the **score formula**; the **tech-efficiency formula/cap** hit it
too (the "Because of technology…" report routine at ~0x32D1B is pure display —
it `call`s string-builders on already-computed percentages; the formula lives in
a separate real-arithmetic unit).

**So, for magnitudes: black-box, don't disassemble.** Recover the *effect list /
structure* from strings + help + dispatch disassembly (all reliable), but get
the *numbers* by varying inputs in live play and reading BRE's own reported
output — e.g. vary Technology-vs-total regions and read the "% strength / %
production" the tech report prints; fit the curve and the cap. This is exactly
how the score formula was cracked (`Score += round(day-start net worth)`), and
it beats reversing software-float assembly. If static reversing is truly needed,
**Ghidra** (free, 16-bit decompiler) handles the segmentation far better than
`ndisasm`; radare2 helps navigate but not with the software-float problem.

## 7. Config-editor field bounds

The Configuration screens list "(default; max)" per field. Find them near the
config strings:

```
strings -n 3 "$BRE/BRE.EXE" "$BRE/BRE.OVR" | grep -iE "maximum|default|per day|rate"
```

Cross-check against `internal/game/config.go` constants and
`docs/mechanics-reference.md`.
