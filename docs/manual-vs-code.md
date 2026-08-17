# Manual vs. Code

Immortal Barons is a from-scratch clone, so every mechanic has to be sourced from
somewhere. Barren Realms Elite ships a manual, a help database and a changelog,
and they are cheap to read and usually right — which is what makes the exceptions
expensive, because nothing in the text marks the wrong line.

These are the places where the original's own documentation turned out to
describe something the shipped program does not do, plus a couple of outright
defects in the program itself. Each says what the clone does about it — and in
almost every case the answer is that it follows the program, not the paperwork.
The exception is the last entry, where the original is not wrong so much as
terse, and readability won.

This page is not a list of the ways Immortal Barons differs from the original by
choice; those are recorded per mechanic in the developer reference.

The order of preference behind all of this: a screen captured from a running
copy beats a reading of the compiled code, a reading of the code beats the text
strings inside it, and those beat the documentation.

## Where the documentation and the program disagree

### A quick strike hits harder than advertised

The attack help puts a quick strike at 110% of normal strength. The invasion
resolver (`BRE.OVR` `0x4055a`) switches on the arriving attack's type byte and
multiplies the incoming force by a 6-byte Turbo Pascal real: **1.2** for a quick
strike, **0.85** for an extended battle, and nothing at all for a normal one.

What settles the reading is the *second* constant each branch loads — the
fraction of its force a side has left when it breaks off:

| Attack type | Strength | Breaks off at | Losses |
|---|---|---|---|
| Quick Strike | **1.2** | 0.92 | 8% |
| Normal Attack | 1.0 | 0.85 | 15% |
| Extended Battle | 0.85 | 0.80 | 20% |

Those loss figures are exactly the three the same paragraph of `attack.hlp`
publishes, which pins each branch to its attack type beyond argument. The help is
right about three numbers and wrong about the fourth, written inches apart.

**Immortal Barons** uses 120%.

### Declaring war is not the polite option

The manual presents a declaration of war as the way to end an agreement without
causing trouble at home, and says the treaty does not really break until the
other realm has been told.

Neither holds. On confirmation (`BRE.OVR` `0x01a838`) the original divides
**both** popular support and military morale by four and multiplies by three —
integer division first, so 99 support keeps 72 rather than 74 — and clears the
relation row on both realms immediately. Nothing waits on the notification, and the game's own on-screen
warning about revolts contradicts the manual a few lines later.

The corollary is sharper: no attack path in the original ever looks at the
relationship, so **betraying** a partner by attacking them costs nothing at all.
The crown charges for the public act and leaves the private one to be punished by
the other players.

**Immortal Barons** matches both halves. It used to charge for the betrayal and
nothing for the declaration, which was the original's asymmetry exactly
backwards.

### Protective Trade has nothing to maintain

The manual says the pact makes trade deals cheaper to send *and to maintain*.

The send discount is real: `create_trade_offer` (`BRE.OVR` `0x0268bc`) compares
the recipient's relation against the Protective Trade value and divides the
per-day transit cost by three, before the span is chosen. The other half
describes nothing — the cost routine is called from two sites, charged once up
front, and no recurring trade cost exists anywhere in the program.

**Immortal Barons** implements the discount and does not invent an upkeep to
discount.

### A High terrorism setting makes terrorism rarer, not commoner

The sysop help for the Terrorist Costs option (`game/reset.hlp`) says the choice
sets what an interplanetary terrorist operation charges, and that "a High
setting will make terrorism more popular".

It does the opposite. The pricing routine (`BRE.OVR` `0x02aca8`) reads the
option's byte and, for High, multiplies the price by three; for Low it divides by
five, and for None it zeroes the charge. The byte's encoding is Medium 0, None 1,
Low 2, High 3, which the settings screen's own name lookup confirms. Tripling the
price of an operation makes it rarer.

The sentence reads as though someone described the setting from the sysop's point
of view — a high setting turns terrorism up — while the code turns the *cost* up.
Nothing else in the entry is wrong, which is what makes the line easy to take at
face value.

**Immortal Barons** follows the code: None free, Low a fifth, Medium the base
price, High triple.

## Where the changelog describes a version you do not have

Worth separating from the two above. A changelog can be a perfectly truthful
record of a build that is not the one that shipped, so an entry being wrong about
v0.988 does not mean it was ever wrong.

### Tanks do not defend against chemical missiles

The changelog announces that they do. No chemical, biological or nuclear routine
in the shipped version reads the target's tanks, turrets or missile defence at
all — the only things any of them look at are the realm's name, its region count,
its population, its morale and its support.

**Immortal Barons** used to reduce all three weapons by the target's SDI. That
was the clone's own invention and is gone.

### Missile bases do not stop a neighbour's nuclear strike

Same shape. The local nuclear routine reads a realm's name and region count and
nothing else. The "up to 50% of incoming missiles" the documentation advertises
belongs to the interplanetary path, not to a strike from the realm next door.

**Immortal Barons** lets a local strike always land.

## Defects in the original

Not a documentation problem — the program does something its author plainly did
not intend. A clone has to decide case by case whether being faithful means
reproducing the bug.

### Spying ignores the defender entirely

The success roll for Send Spy is built from two calls to the same strength
routine, one meant for the attacker and one for the defender. Both are passed the
**attacker**. The instruction bytes are identical — `8A 46 10` at both `0x4BAE7`
and `0x4BB03` — and only the mode byte differs.

With no alliances in play the two terms reduce to `agents / kind` and `agents`,
the agent count cancels, and the roll collapses to a constant:

```
chance = 0.1 + 0.9 x 1/(1+1) = 55%
```

Fifty-five percent, whatever either realm holds. Buying agents does nothing for
spying, and defending against a spy is impossible.

It reads as a slip rather than a design, because the same routine is called
correctly at `0x4AA5E`, passing a different empire with the other mode.

It also has a consequence its author cannot have wanted. A Terrorist Prevention
treaty raises the defending term — which, because that term is really the
attacker's own agent count, *lowers the holder's own spy success*. Signing a
counter-espionage pact makes you worse at espionage.

**Immortal Barons** reproduces this, defect included, as a deliberate choice. It
was written here that only Send Spy went through the defective routine and that
the effect operations kept a correct attacker-against-defender roll; walking the
resolver's call sites showed otherwise. Every local operation reaches this one
roll — the effect ones by way of the queue daily maintenance drains — so none of
them is defended against by agents.

## Mistaken for a defect

The traffic runs both ways, and calling something a bug closes the question
early.

### "Funding / Region: 0,000 Gold"

The missile-defence screen prints that line at every funding level, up to seven
million. It was written down here for months as unexplained and probably a fault.

It is a display convention. The program stores the funding in whole thousands and
the screen appends a literal `,000` to whatever it prints. The per-region figure
is smaller than a thousand, so it renders as `0` and the line reads `0,000` —
correct by its own rules.

It only became clear while deriving how much shield a given funding level buys,
which is held in the same units:

```
strength = trunc( sqrt( funding / (10 x (regions + 1)) ) )     clamped to 0..100
```

(`BRE.EXE` `056d:1139`). That formula reproduces all sixteen captured screens
exactly — and only does so if the stored funding is read as thousands.

**Immortal Barons** prints the gold figure the label describes — the one place
on this page where the clone does not follow the program. Nothing is being
corrected: the original is right by its own convention, and a line that reads
`0,000` at every funding level simply tells a player less than the number does.

## Reading this the right way

None of the above is an argument against the documentation. Sweeping it first is
still the fastest way to learn what a mechanic is and roughly where to look —
between them, the manual and the help gave the whole treaty catalogue, the
group-attack timing bounds, and three of the four attack figures correctly.

The rule that came out of it is narrower: read the prose to find the question,
and read the program for the number you are going to rely on. Where the two
disagree, the program wins, and the disagreement is worth writing down next to
the answer — otherwise the next person to look will quietly correct it back.
