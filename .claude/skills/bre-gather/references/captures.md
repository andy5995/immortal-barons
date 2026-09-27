<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. -->

# Reading BRE captures

## Deciding a screen is NOT in the captures — the two ways that goes wrong

Concluding "no capture covers this" is a claim, and a wrong one sends people off
to re-drive BRE for something already on disk. Both failures below happened on
2026-08-17, on the same screen, within an hour.

- **Search for a menu ITEM plus its value, not for the status line.** The Covert
  Operations menu was declared missing because the grep was keyed on its footer,
  `You have N gold and N agents.`, and the pattern did not match the real
  spacing. A screen's most greppable feature is a distinctive label next to a
  number — `'Stir Revolts'` with `25,000` — not its prose furniture.
- **BRE's help-topic INDEX lists every menu item, so it looks exactly like the
  menu.** The follow-up search found the nine covert operation names, read the
  screen they sat on, saw a plain list of all of them, and concluded the hits
  were only the topic index. They were — at that offset. The real menu was
  elsewhere in the same file. **A hit inside the topic index does not rule out a
  hit on the menu; keep walking the matches.** The index has no prices and no
  `(n)` keys; the menu has both.

**Before reporting a screen as uncaptured, say which files you searched and with
what pattern.** And check the file's mtime: a capture Andy took minutes ago is
new data, and an earlier "not present" was true when it was made.

## Parsing a `.cap` capture — four traps that produced wrong findings

The economy parser is `scripts/bre-econ.py` in this project's Claude dir (per-turn
income, region counts, purchase markers, back-computed yields). Prefer it to
ad-hoc greps. Its `--shapes` mode censuses every distinct message form in a file
— **run that first**, so no relevant line escapes notice.

- **Count with `grep -oc`, never `grep -c`.** These captures are `\r`-separated,
  so a whole screen can be one "line": `grep -c` reported 1 fishing turn in a
  capture that contained 6.
- **Never pipe a survey through `head`.** A truncated survey once "proved" that
  rivers never produce food, when the line was simply below the cut.
- **The Regions display WRAPS onto a second line.** Parsing only the first loses
  Mountains, Coastal and Technology — that mistake hid 18 tourism samples.

- **Record how a figure is SPELLED before reading it as a value.** Converting
  on sight destroys the evidence. On 2026-08-30 a grep for battle casualties
  returned `You lost 116k Tanks!` and `You destroyed 1111 Troopers, 115k
  Turrets, and 105k Tanks!`; both were read for magnitude, `115k` became
  115,000, and the fact that BRE ABBREVIATES went unnoticed until Andy said so.
  It was on screen twice. When a task touches how a figure is displayed, run the
  census first — it is one command and it answers the whole question:

      grep -aoE "[0-9]+[km]\b.{0,40}" cap/*.cap | sed -E 's/[0-9]+/N/g' \
        | sort | uniq -c | sort -rn | head -40

  That sweep also turns up formats you were not looking for: it found a fourth
  number style in the Daily Bulletin (`12,468k` — divided once, suffixed, THEN
  grouped, never reaching `m`) that no other screen uses.

- **Count news events against `news.dat`'s templates, never with an ad-hoc
  grep.** Each category holds a handful of one-line variants with `%F`/`%T`
  placeholders (`^NUKE` has four), and BRE draws ONE per event — so counting the
  four exact template shapes both proves the one-line-per-event mapping and
  gives an exact total. An improvised pattern gets it wrong in both directions:
  excluding lines that name two realms (to separate attacker from target) threw
  away `EXTRA!  EXTRA!  %T was hit by Nuclear Missiles launched by %F!` and
  undercounted a realm's strikes by a third, on a count the whole finding rested
  on. Read the category out of `game/news.dat` first, then count its shapes.

- **A capture starts mid-game; state from before it still counts.** A group
  join reported a 6.06% share that no weighting of the visible figures could
  produce, because the share covers everything the baron had committed, and some
  jets had gone in before the capture began. When a figure will not reconcile,
  ask what happened off the record before doubting the formula.

- **An echoed menu selection is not a completed action.** These captures echo the
  chosen item beside the prompt (`Choice> Quit    Undermine Investments`), and
  that records the keypress only — the op may have been abandoned at the next
  prompt. Reading a run of echoes as a run of actions produced a wrong claim
  about which per-day allowance covers what, caught only because the sysop
  remembered not going through with it. The tells for a completed action are a
  state change on the next screen (a menu row gone, gold moved), not the echo.

**And before explaining any per-unit figure, check whether the count you divided
by changed that turn.** Purchases land between the report and the Regions
display, so a total can print next to a STALE count. Twice in one session a
changed denominator was mistaken for a changed mechanic.

### Past sessions are a capture archive — grep them before asking for a new run

Every screen ever scraped in this project sits in the session transcripts at
`~/.claude/projects/-home-andy-src-andy5995-immortal-barons/*.jsonl`. They are
JSON-escaped (`\\u001b`, `\\r\\n`) but a small Python pass unescapes and strips
ANSI, and they hold the *surrounding* screens that a summarized table in
`docs/dev/` dropped. That is what settled the SDI curve: the write-up recorded
funding and strength but not the realm's region count, and the count was sitting
in a menu two screens away in the transcript.

**A price can stand in for a figure the capture never printed.** Reverse an
already-verified formula: Terrorist Ops is `total regions x 64`, so a menu
showing `532,544` pins the realm at 8,321 regions without a Regions display. The
region-maintenance and nuclear-price formulas work the same way.

### First ask whether the capture EXERCISED the feature at all

A capture that mentions a feature thousands of times may never have used it
once. BRE presents the Attack Menu and the InterPlanetary Ops menu **every
turn, automatically**, so their item labels accumulate once per turn whether or
not the player pressed anything. A 27 MB capture matched "Group Attack" 5,385
times and looked like a goldmine; it contained no interplanetary attack at all.

**The tell is equal counts across sibling items.** Count several items from the
same menu at once — when `Regular Attack`, `Nuclear Attack`, `Attack Pirates`
and `Alliance Strength` all land on 2,498, that is the menu being redrawn 2,498
times, not four features being used. A number that *breaks* from the cluster is
the one worth chasing (`Spy Database` at 13,155 against a 2,534 menu count is
real use; the 317 non-menu `Group Attack` hits were `BRE PLANETARY` step lines).

**Prove absence with the binary's own result strings, not with guessed wording.**
Harvest the Pascal ShortStrings from `BRE.OVR`, filter to the mechanic's
vocabulary, and test each against the capture — that answers "is this flow here"
without depending on how you remember the prompt being worded:

```
python3 - "$BRE/BRE.OVR" plain.txt <<'EOF'
import re,sys
ovr=open(sys.argv[1],'rb').read(); cap=open(sys.argv[2],'rb').read()
c=[m.group(2)[:m.group(1)[0]] for m in re.finditer(rb'([\x08-\x3c])([ -~]{8,60})',ovr)
   if len(m.group(2)[:m.group(1)[0]])==m.group(1)[0]]
kw=(b'attack',b'strike',b'battle')          # the mechanic's vocabulary
for s in sorted({x for x in c if any(k in x.lower() for k in kw)} , key=lambda x:-cap.count(x)):
    if cap.count(s): print(f"{cap.count(s):>6}  {s.decode('latin-1')}")
EOF
```

Run this **before** planning a mining session. It takes one command and it is
the difference between an afternoon of analysis and knowing in a minute that the
data is not there.

**`grep` calls these captures binary and silently reports nothing.** CP437 high
bytes in a UTF-8 locale make GNU grep exit 1 with no output and no "Binary file
matches" — the same trap the `breins.txt` section describes, in a new place. Use
`LC_ALL=C` **and** `grep -a`; a bare grep returning zero on a capture is not
evidence of absence until both are set.

### Auto-Pay turns are a stronger probe than the itemised prompts

With **Auto-Pay Maintenance ON**, BRE collapses the whole maintenance sequence
into a single `N Gold paid.` line. That is *more* informative than the separate
prompts, not less, because every component has to reconcile against one number at
once:

```
Gold paid = regionUpkeepPerRegion × regions      (constant for a given realm)
          + perUnitMaint × units held
          + trunc(turn income × PlanetaryTaxRate / 1000)
```

Guess one unknown, solve for another, then check whether the answer stays
constant across turns where the *first* quantity moved. **A constant that
survives a changing denominator is the signal; a "constant" that drifts with
income is a wrong assumption.** This settled whether industrial gold is inside
the crown-tax base in ten turns of already-captured data: assuming it is taxed
leaves region maintenance at exactly 913.000/region across three different region
counts, assuming it is not leaves a figure wandering 974–992.

It also yields per-unit maintenance for free — on turns holding manufactured
units the per-region figure sits slightly above the constant, and the gap is
`units × perUnitMaint`.

So when a capture is needed to settle an arithmetic question, **ask for Auto-Pay
ON**, not off. Auto-Pay OFF is for learning the prompt *sequence*, wording, and
colors (see the maintenance-flow section of `docs/mechanics-reference.md`).

## Reading a capture as bytes

**Per-character color in a capture is DATA, not decoration.** `cat -v` a `.cap`
and read where the escape sequences fall between characters: BRE colors each
letter of a lottery draw as it prints it, so one captured line
(`ESC[0;40;31m D ESC[31m K … ESC[1;33m I …` against the ticket `AGNTYI`) proves
the scoring rule — the yellow letter is at a different position in the draw than
on the ticket, so matching cannot be positional. A whole disassembly session was
about to be spent on that question. Look for the color capture before deciding a
rule needs code: ANSI-stripped skimming throws exactly this away.

**In a capture, count the characters echoed after a prompt.** One echoed key
means a single-key prompt; a run of them (`Send to: EFHIJKMNOP`) means
multi-select, and a run with embedded `\b`/space/`\b` means the keys *toggle*.
This is visible in a plain `cat -v` of the capture and it settles the question
before any disassembly — but only if the capture is read as bytes rather than
skimmed after ANSI stripping, which turns the erase sequences into
innocuous-looking spaces.
