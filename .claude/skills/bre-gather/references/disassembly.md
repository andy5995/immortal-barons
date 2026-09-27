<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. Organized by topic:
add a new lesson to the section it belongs to, and merge it with the entry that
already says the same thing rather than appending a near-duplicate. -->

# Reading the BRE disassembly

## The static catalog is the way in

`scripts/bre-disasm.py` maps every overlay unit and resident routine, so nothing
here needs a guessed segment base. (Base-solving by plausibility score found the
attack unit once and dead-ended silently on everything else, 2026-08-11. Do not
go back to it.) The catalog's format, its durable IDs, the naming statuses and
the address model are in `docs/dev/bre-disassembly.md`; read that rather than
re-deriving any of it. The commands:

```
python3 scripts/bre-disasm.py check-catalog
python3 scripts/bre-disasm.py find-string --directory "$BRE" "technology"
python3 scripts/bre-disasm.py list --kind procedure --filter technology
python3 scripts/bre-disasm.py lookup technology_report          # no --directory; returns a JSON list
python3 scripts/bre-disasm.py xrefs technology_report --directory "$BRE" --show-sites --direction callers
python3 scripts/bre-disasm.py list --kind dispatch
python3 scripts/bre-disasm.py map --directory "$BRE" --ovr-offset 0x32d1b
python3 scripts/bre-disasm.py disasm --directory "$BRE" --procedure technology_report
python3 scripts/bre-disasm.py disasm --directory "$BRE" --around 0x32d1b --instructions 40
```

`lookup` wants a name or a durable id: a bare address fails with `no catalog
name or alias matches`, so feed it the `bre0988:ovr:procedure:04dc1d` a
`callees[]` entry gave you. `--around` takes an OVR file offset, a resident
`SEG:OFF`, a site id or a procedure.

The loop that keeps paying off:

1. **Find the message string's users** with `find-string`, and search sibling
   wording too: BRE duplicates subsystems, and the variant can lead to different
   code.
2. **Walk the graph by durable id**: `callers[].from_id` up, `callees[].to_id`
   down. A `calculated_call` edge carries a `dispatch_id`, and that record is the
   complete target set. All 23 reachable indirect-call sites fall into 13 proven
   closed sets, so an indirect call is never a reason to launch an emulator.
3. **Disassemble at catalog boundaries** (`--procedure`, or `--around` a site),
   never a guessed slice from an arbitrary byte.
4. **Find a constant** by searching `struct.pack("<H", value)` and checking the
   byte before it for an imm16 opcode (`b8` mov ax, `05` add ax, `b9` mov cx …).
5. **Calculate Real48 exactly** with `scripts/bre-real48.py` (below).
6. **Validate against play.** A reading is not a finding until it reproduces
   captured figures exactly. Rounding against truncation and the order of
   operations each move a result by a unit.

Use the Xvfb DOSBox debugger (last section) only when bytes need independent
validation or the evidence is outside the catalog. Memory dumps and traces are
original program material: keep them out of the repository.

**A far call's target is a catalog id: `seg*16 + off`.** `disasm` prints resident
calls raw (`call word 0x56d:word 0xec6`); `0x56d0 + 0x0ec6 = 0x6596` is
`bre0988:exe:procedure:06596`, `total_regions`. Do this for every call in a
routine before summarizing it. The Planetary Master was recorded as "highest net
worth" for weeks; the comparison called `total_regions` (2026-09-24).

**Resident file offsets add the MZ header.** The catalog reports a resident span
in image offsets (`segment*16 + offset`); a byte search on `BRE.EXE` needs file
offsets, `0x2940 + segment*16 + offset`. The SDI strength routine is `0x06809`
in one and `0x9149` in the other, and the difference reads as the catalog being
wrong. It is not.

**A disassembler that prints nothing has not told you the region is empty.**
`disasm` once exited 0 with no output because ndisasm 3.02's `-k` flag stops
disassembly outright instead of skipping a span; the script now runs one ndisasm
per code region (`code_regions`). Do not reintroduce `-k`. When any step returns
empty, try a window you know holds code before concluding anything.

## Finding a constant or a string

**`find-string` with an EMPTY query and a `--function` filter dumps every string
one routine touches**, which gives a screen's whole structure before any code is
read:

```
python3 scripts/bre-disasm.py find-string --directory "$BRE" \
  --function resolve_returning_attack --details "" | jq -r '.matches[].text'
```

The interplanetary returning-attack report (header, four verdict words, per-unit
lines) came out of one call. `--details` output is proprietary; never commit it.

**An empty `find-string` for text `strings` can see means a code-segment
constant**, not an unused string. `DATA\SPY.BRU` returned nothing because its
routines load it as `mov di,0xe0` / `push cs`, which the index does not record.
Take the offset from `grep -abo`, then `bre-disasm.py list | grep <unit>` for the
unit spanning it.

**A menu's key-to-label map comes from the DRAW calls, not the string order.**
Declaration order equals menu order often enough to be a trap. Each row is
`mov al,'<key>'`, the label's unit-relative offset in `di` pushed with `cs`, then
one call to the item printer; the dispatch below (`cmp al,'1'` …) confirms it.
The Coordinator Ops menu is the worked example (`ovr_015dbf`, keys `0x31`-`0x34`
against offsets 0x00 / 0x11 / 0x22 / 0x37).

**Strings pushed as `mov di,<off>` / `push cs` are ShortStrings at that offset
inside the same unit.** The offset points at the length byte. Decoding `d[off]`
from the unit names a routine without `--details`, and a run such as
`0x0f "Dismantle Gooie"` `0x00` `0x10 "Modify Diplomacy"` holds a ZERO-length
string at `0x10`: the blank annotation argument, not a fifth label. `xxd` the pool
before interpreting it.

**"Not a literal in the binary" means you searched BOTH binaries, in the right
encoding.**

- `BRE.EXE` holds the initialized DGROUP. The covert fees were written up as a
  runtime table after a search of `BRE.OVR` alone; the nine dwords sit in
  `BRE.EXE` at `0x14EDE` (the goods table at `0x157b7` is the other case). A
  `DS:` displacement is an offset into that data: solve the base against a known
  landmark (`"Covert Operations"` at `DS:0x662`, pinned by the menu's
  `mov di,0x662`), not by plausibility.
- **A 32-bit constant is two 16-bit immediates, not contiguous bytes.** Turbo
  Pascal loads a longint as `mov ax,imm16` / `mov dx,imm16`, with the second
  opcode between the halves: 2,000,000,000 in `run_bank` is `B8 00 94  BA 35 77`.
  A search for `00 94 35 77` finds nothing. Search the halves (`python3 -c
  "v=2000000000; print(hex(v&0xffff), hex(v>>16))"`), and read the instruction
  after any bare `mov ax,imm`.

**A setting missing from `bre.doc` may still exist.** RESOURCE.DAT's loader
stores its keywords as consecutive ShortStrings; the full sweep is written up in
`docs/dev/bre-resource-dat.md`, so read that first. Working backwards is as
cheap: a global tested by a mechanic (`cmp byte [0x76c0],0x0` at the head of
`run_lottery`) has one writer, the settings loader loading that keyword's label.
A RESOURCE.DAT keyword is the local sysop's, never the Coordinator's.

**A Configuration Editor knob is one config byte and a four-way ladder.**

```
les di,[0x28b4]              ; the config record
mov al,[es:di+0x18N]         ; the knob
cmp al,0 / 1 / 2 / 3 -> four arms
```

One regex for `26 8a 85 <disp16>` lists every knob site. The presets sit at
`0x180`-`0x186` in reset.hlp order (Maintenance Costs, Attack Damage, Attack
Costs, Attack Rewards, Terrorist Costs, Region Costs, Trade Deal Costs), the
bools at `0x18a`-`0x18d`. The `0x4d4c4` and `0x144cb` clusters are the editor
itself. **The byte encoding is Medium 0, None 1, Low 2, High 3**: the arm that
leaves the value alone is Medium, the arm that zeroes it None. **Each knob keeps
its own spread** (Attack Costs 0/20/100/300 %, Attack Damage 0/50/100/150 %), and
the same knob is often implemented twice: Terrorist Costs has a Real48 arm set
(`0x2ad1a`) and an integer one (`0x2ad9f`) stating the percentages outright.

**Prices: find the table write, not the buy handler.** The buy routine reads an
`int32` table at `DS:0x2216` indexed by the menu key (HeadQuarters `'5'` is
`0x22EA`), filled once per turn from per-empire record fields. Search for the
WRITE to the slot (`a3 <lo> <hi>`); the HQ formula sat 20 bytes above it. BRE
keeps unit prices per empire.

## Record fields

**A raw figure carries BRE's unit, and the string printed beside it says which.**
BRE counts population in millions; IB counts twenty people to BRE's one
(`PopBREUnitScale`), so a per-head rate lifted from the binary is per MILLION.
Percentages need no conversion. The chemical strike prints its share of `+0x62`
with `" million civilians were killed!"`: one `find-string` for where a field
reaches the screen settles its unit. This went wrong three times, last on the
chemical missile's price. **A field can also hold thousands**: the SDI pot is
stored as `7078` and printed with a literal `,000` suffix, so a formula reading
the field is a thousand times smaller than the screen. When a result is orders of
magnitude off, suspect the field's unit before the reading.

**Record layout and helper addresses are in `docs/dev/bre-save-format.md`.**
Extend that file rather than re-deriving.

**The −3949 base.** The current empire is `les di,[0x28d8]` with small positive
offsets (`+0x1f` name, `+0x286` Score). Any other realm is `mov al,<letter>;
mov dx,0x42d; mul dx; les di,[0x28b0]; add di,ax` then a large NEGATIVE disp16,
and `offset = disp + 3949`. Every inter-empire effect (WMD strikes, covert ops,
treaty contagion, the AI's limits) lives in that second form.

- **`add di,0xf093` (−3949) before a call is the record BASE**, offset 0, passed
  to a helper that decides what is read. `call 056d:0ec6` after it is
  `total_regions`, the nine region fields summed: look up the callee before
  reading anything into the displacement.
- **Scan both forms at once**, or you map half a mechanic. Morale and support
  gave 62 `BRE.OVR` sites and 4 in `BRE.EXE`, 30 of them the arbitrary form,
  including every weapon:

  ```python
  mods = {0x80 + (r << 3) + 5 for r in range(8)}     # mod=10, rm=101 -> [di+disp16]
  for d in (off, off - 3949):
      for m in re.finditer(re.escape(struct.pack("<h", d)), buf):
          if buf[m.start()-1] in mods and buf[m.start()-3] == 0x26:   # es: prefix
              ...
  ```

- **An empty byte-pattern scan is still not absence.** The Technology research
  writer reaches its field with a separate `add di,0xbe` and a plain `[es:di]`,
  so no displacement appears at all, and a `rep stos` over a whole record
  matches no pattern. Say which idioms you searched.
- **A displacement is a field only after you subtract the index.** `[es:di+0xae]`
  is the Mountain count, but after `shl ax,1; add di,ax` it is the relation with
  empire `ax/2`: BRE indexes that array by the raw letter and folds `base − 2*'A'`
  into the displacement. Dump ~12 bytes BEFORE each hit and sort by whether an
  index was added.
- **A letter in a picker is an array index, not a row number.** The letters carry
  gaps for dead realms and for the caller; never renumber a list to close them.

**A whole mechanic falls out of scanning for the RECORD BYTE it hangs on.** The
SpyGuy's seven sites came from one regex at its displacement (one store, one
daily inc/dec, four compares that are its gates):

```
python3 - <<'EOF'
import re
d=open('BRE.OVR','rb').read()
for name,p in {
 'store':  rb'\x26\x88[\x45\x4d\x55\x5d\x65\x6d\x7d]\x6f',
 'load':   rb'\x26\x8a[\x45\x4d\x55\x5d\x65\x6d\x7d]\x6f',
 'cmp':    rb'\x26\x80\x7d\x6f',
 'incdec': rb'\x26\xfe[\x45\x4d]\x6f',
}.items():
    for m in re.finditer(p,d): print(name, hex(m.start()))
EOF
```

The displacement means something only with its base (`+0x6f` there is an array
in the record at `[0x28b4]`), and a word-sized field needs the 16-bit forms.
**For an unknown field, scan the code shape**: `26 83 (85|bd) <disp16> <imm8>`
filtered to plausible immediates found HeadQuarters as the only `cmp …,100` at
an unmapped offset. **Resolve a hit list to names in one run**: load the catalog
JSON, flatten every unit's blocks and data chunks plus each root's
`body_ranges` (offset by `ovr.code_offset`), and bisect.

**A field's meaning comes from its WHOLE access list — open every site.** An
inc/dec pair around a block looked like a re-entrancy guard; `+0x2b8` is the
turn-stage counter, compared against 1 and 20 elsewhere and reset at commit. "HQ
affects only tanks" was asserted with four reads unexamined, and was right only
by luck. The tell is a persisted field doing a local variable's job: BRE does not
spend save-file bytes on a guard.

**A live save names a field when 80 call sites will not.** Technology Agreement
`+0x5d` was tested at 83 sites, all `> 0`. Dumping it across every slot of
`data/game.dat` (`BASE, STRIDE = 2489, 1069`) showed `-1` in every empty slot and
a serial in the occupied ones: a slot-in-use counter. Find the record base from
the realm-name ShortStrings' spacing, and note that unused slots carry the
starting template, so they look like plausible realms.

## Names, callers and prompts

**A catalog name is a hypothesis; the callers and the field offsets decide.**
- `allocate_turn_budget__apply_support_boost_payment` prices the MORALE boost:
  it sums unit counts against Real48 weights, and support is priced off
  population.
- `resolve_received_invasion__calculate_attacker_strength` sums Turrets and
  morale, a DEFENSE expression.
- `launch_gooie_kablooie` is the defender's jet attack (`Only Jets can attack
  Gooie Kablooies`), and its one caller, `run_player_turn`, shows it is offered
  every turn.
- Two region-price routines in `ovr_02ff05` compute the same way; the one called
  only by `calculate_waste_decontamination_cost` prices decontamination, the one
  called by `purchase_regions` and the buy menus is the land price.

Read the offsets against `docs/dev/bre-save-format.md` before the name, and say
"the block the catalog calls X" until they agree.

**Name an unnamed routine by what its callers share.** `04ef:002f` was called
from WMD launches, `report_spy_result`, `break_diplomatic_treaty`,
`process_trade_offer` and the covert resolver, all of which run while the
recipient is NOT logged in: the "since your last play" filer, settled without
decoding its body. Screen printers are called from menu handlers, recap filers
from resolvers in someone else's turn, disk writers from maintenance and packet
paths. **Which argument slot carries the recipient** then says who was told: the
defender's notice passed `[bp-0x3]`, the ally's `[bp-0x1]`. Quote the slot.

**Name a predicate from the call site where it gates a message.** `056d:19b5` was
"some per-empire predicate" until one of its 27 sites printed "Our empire is in
protection, my lord." on true. Sort the sites by nearness to a string table.

**A prompt's TEXT is not its behavior: read the caller.** The routine that
prints a prompt usually does nothing else. `(A-Y,Z=All,?=List) Send to:` was
cloned as one keypress for a year; its caller (`BRE.OVR` 0x1b65e) is a toggling
multi-select closed by RETURN, and `Z` marks every letter.

**Count references before hypothesising.** A predicate called 27 times across
unrelated contexts is a generic guard, not an event gate. A global with 104
reads and zero writes (`[0x28dc]`) is an index, not a tunable; grep the store
opcodes (`a2`, `c6 06`, `88 26`) as well as the loads. **Say whether you counted
instructions or routines**: the SDI percentage is seven call instructions in four
routines, and the next reader's five-byte regex finds the seven.

## Reading control flow

**A refusal string's OWNER is not its condition — disassemble the compare, which
sits BEFORE the refusal block.** "You do not have relations with that realm."
belongs to `create_trade_offer`, but the branch right below it calls the
protection predicate. The guard itself gave the rule and its threshold:

```
17E5  cmp word [es:di+0xae],0x1   ; the pair's relation
17EB  jnl 0x1817                  ; >= 1 proceeds
17ED  <the "no relations" refusal block>
```

Against the relation enum (−1 Enemy, 0 None, 1..7 pacts, 8 War) that is "any pact
at all". Aim `--around` a few dozen bytes before the block `find-string` reports.

**A guard often tests more than the thing you came for; read to the flag.**
`region_price_over_range` (`BRE.OVR 0x030237`) calls `is_under_protection` and
zeroes its flag before it compares the region count to `0x12c`. Reading only the
compare shipped a rule right about the threshold and wrong about who it applies
to.

**A turn stage's gate is in `run_player_turn`, not in the stage.** `allocate_food`
opens the Food Market unconditionally; the stage dispatch (`cmp al,0x5`) skips it
when the realm can cover the food bill and Auto-Feed (`+0x33a`) is set. The
preference bytes are consecutive at `+0x335..+0x33a` in Preferences-menu order.

**Check where a conditional jump lands.** A `jz` usually skips a sub-block: in
the Queen Royale refund a false predicate meant "pay uncapped", not "pay
nothing". **A routine with no `Random()` (`0c03:0ed0`) always acts**, so an event
that sometimes does not happen is decided by its caller.

**The field next to the one you want disassembles just as plausibly.** The
S3-Sabre dial is `[bp-0x3]`; `[bp-0x4]` beside it switches on 1/2/3 and read
like "three dial settings do anything" for months. Count branches against the
data (seven `^SABREHIT` lines against three cases is a mismatch), and follow the
value to where it is CONSUMED, not just compared.

**Read the sibling you already understand, side by side.** BRE duplicates
subsystems, so the chemical and biological strikes laid beside the nuclear one
differed only in immediates, and every difference was a finding. A MISSING call
is evidence too: the biological routine never reaches the region-to-waste helper.
The same goes for captures: one item's drifting price is noise, all eight side by
side showed HeadQuarters as the only ratchet.

## Real48

Turbo Pascal passes a 6-byte real as a register triple or a run of
`mov word [bp-N]`. The left operand is `dx:bx:ax`, the right `cx:si:di`, and
`RDiv`/`RSub`/`real_compare` compute LEFT op RIGHT. A constant loads as
`mov cx,0x8a / xor si,si / mov di,0x3b80`: `cx` low is the exponent, then the
mantissa bytes, giving `mem:8a000000803b` = 750. **Decode, never eyeball**:

```
python3 scripts/bre-real48.py decode mem:810000000000 mem:800000000000
python3 scripts/bre-real48.py div 100 3
python3 scripts/bre-real48.py mul mem:870000000048 0.25 --output all
python3 scripts/bre-real48.py eval '100000 + (50 / 5)'
```

`eval` rounds to Real48 after every operation. Never use Python `float` or
borrow another Turbo Pascal release's constants; `docs/dev/bre-real48.md` covers
the representation. A round result (1000, 0.3) confirms the byte order; garbage
means the bytes are in the wrong order. `bre-disasm.py list | grep real_` names
the resident helpers.

**A push/pop between the operands SWAPS them.** Pascal evaluates the right
operand first when it is not a constant: compute, `push dx/bx/ax`, compute the
left, `pop cx/si/di`. So the value computed FIRST is the RIGHT operand. The
interplanetary air roll read as "the defender's jets die in proportion to their
own number" until the `pop` was followed (2026-08-24). **When a rule comes out
absurd, suspect the operand order before the game.**

**Count the pushes to count the terms.** One long expression is term,
`push`, term, …, then `pop`+add pairs; adds plus one is the term count, and the
final truncation applies once. Twelve terms showed the armed-forces food bill
covers six unit types twice.

**A price built through `real_to_integer_round` carries a half step**: the land
price is `0x384 + climb x (owned + 1/2)`, so a base fitted at one climb is 17
gold short at another.

**An unexplained multiplier is usually the technology factor.** `0x56d:0x1a07`
takes a Real48 cap and a slot and returns that domain's factor; grepping its call
bytes (`9a 07 1a 6d 05`) and decoding the loads before each prints the cap table
(2.0 food, 1.5 gold, 1.4 military, 1.35 production, 5.0 decay). The cap says
which domain a routine belongs to. A per-region figure drifting across turns may
be the factor creeping, not a `Random()`.

## Captures before and after the disassembly

**Census every instance before disassembling.** The Queen Royale refund was filed
from one 1,000,000 payout; the captures held 47 from 354 to 14,000,000, fourteen
of them exactly 1,000,000, which is the shape of a CAP. Two signatures: one value
many times with others spread around it is a clamp; 999,999 beside many
1,000,000s is a cap applied through a six-byte real, not a separate case.

**Kill hypotheses with the captures, then disassemble.** Realm size and Score
each died in one query as drivers of the HQ price; the binary gave
`5000 + 75×turnsPlayed + Random(300)`, capped at `100000 − Random(1000)`, in
minutes. **Beware the proxy**: Score rises 213 a turn, so it correlates with
anything driven by turns; BRE's driver was a separate counter (`+0x281`). **Then
re-validate against every capture at once**, deduped: 161 of 163 distinct HQ
prices landed in the 300-wide window, spread flat.

**A doc figure may be a mid-curve value.** `breins.txt` calls a tank "about four
Troopers"; the binary weighs it `1.5 + HQ/100` against a trooper's `0.5`, so 4 is
the value at HQ 50 % and the range is 3 to 5. When prose gives a flat number for
something it also says scales, check the code before hard-coding it.

## Porting a screen: Pascal's `write(x:w)` is not Go's `%*s`

A screen routine writes each field through `text_write_shortstring`
(`0fd0:0964`), and the word pushed just before that call is the field width;
`xor ax,ax / push ax` means no width. Turbo Pascal prints a string UNPADDED when
the width is shorter than it, zero or negative included. Go's `fmt` reads a
negative `%*s` width as the `-` flag and left-justifies, so the padding lands on
the wrong side. List Investments / Loans writes the hold as
`write(thousands : invW-4)` then `",000"`; with small figures the width went
negative and the port printed `$5  ,000` where BRE prints `$5,000` (2026-09-27).
**Clamp every width taken from the disassembly with `max(w, 0)`**, above all
one built by subtraction (`maxW - 4`, `16 - maxW`).

`draw_horizontal_rule` (`0851:00bf`) takes `(width, color)` and draws BRE's inset
rule (5 `─`, then `═`, then `─`), not a plain line; IB's `insetRule` is its port.
Count its calls: the one right after the header's `text_write_line` is a SECOND
rule, which a first reading missed until the capture showed two.

## The heavy debugger works — build it, it is three minutes

`bre-disasm.py debugger --run` wants a `dosbox` binary with the HEAVY debugger,
and no packaged build has one: Arch's `dosbox`, the AUR `dosbox-x` and the
dosbox-x AppImage all ship without it. The AppImage accepts `-break-start` and
does nothing, which reads like the debugger failing rather than being absent;
check with `strings <binary> | grep -c BPM`.

Build it from Andy's 0.74-3 tree with `scripts/build-dosbox-debugger.sh` (this
project's Claude scripts dir); the binary lands at
`~/src/dosbox-0.74-3-debug/dosbox-debug`. DOSBox 0.74 reads `WINDOW`'s internals,
which ncurses 6 hides, so it needs `-DNCURSES_OPAQUE=0` **at configure time**;
on the make command line it wipes the SDL include path.

Driving it headlessly (proven 2026-09-12):

- **The debugger renders to the launching TERMINAL**, so run it in tmux and
  scrape the pane.
- **The DOS screen is the SDL window** on an Xvfb display: type with
  `xdotool key --window $W`, read with `import -window $W shot.png`.
- **Keys go to different places**: DOS input to the SDL window, debugger commands
  to the tmux pane. Re-break with `alt+Pause` before each command and check the
  `DEBUG:` line appeared.
- **A leading Enter RESUMES execution.** Break, then send the command directly;
  the tell is an Output pane that keeps scrolling while nothing you type echoes.

**Chain the breakpoints: DOS write first, memory watchpoint second.** `BPM` on
the address the running game uses found nothing for the pirate seeds, because
`BRE RESET` builds the records elsewhere and writes them straight to
`DATA\GAME.TMP`.

1. `BPINT 21 40` breaks on every DOS write. Read `CX` (length) and `DS:DX`
   (buffer) at each; the piece you want has the structure's length (513 = 9 x 57
   for the faction records).
2. `BPM` on that buffer and re-run; the break lands in the code that wrote it.
3. Find those bytes in `BRE.OVR` with `grep -abo` and read them with the catalog.

A breakpoint holds for ONE process load; set it during the run that matters.
Watch a byte that MUST change (`BPM` fires only on a change), or zero the region
first.
