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

## 6. Config-editor field bounds

The Configuration screens list "(default; max)" per field. Find them near the
config strings:

```
strings -n 3 "$BRE/BRE.EXE" "$BRE/BRE.OVR" | grep -iE "maximum|default|per day|rate"
```

Cross-check against `internal/game/config.go` constants and
`docs/mechanics-reference.md`.
