<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. -->

# Staging a scenario in BRE

A test no longer needs days of in-game build-up. BRE checks each empire record
at load, so a raw field edit is discarded (see `docs/dev/bre-save-format.md`); a
local helper outside this repo, `scripts/bre-stage.py` in this project's Claude
dir, resets a record so BRE accepts it — `dump` / `verify` /
`set GAME.DAT SLOT FIELD=VALUE...`. **How it does that stays out of this repo**:
it is in that script and in `scripts/BRE-STAGING.md` beside it, never in a
tracked file.

Two traps, each of which cost a run on 2026-08-30: **clone a realm that has
survived maintenance** instead of authoring one from scratch — the daily idle
purge eats a staged realm whose last-played stamps look stale, silently, and
the roster just shrinks; and **the slot letter is `(fileoffset − 2489) / 1069`**
— the roster's `?=List` at any target picker is the cheap way to confirm who
BRE thinks exists. Proof of the method: `cap/small-vs-large-20260830.cap`, six
staged battles the binary accepted and fought.

**The HEADER is checked too, and `bre-stage.py` does not handle it.** Patch a
setting in the 2489 bytes before slot A and BRE refuses the whole game with
*"Status File has been tampered with!"*; how to make it accept one is in
`scripts/BRE-STAGING.md` (private). That is what makes a **config knob**
testable, not just a realm's fields. Mapped
so far, all confirmed against the Game Setup screen: **+0x00/+0x02/+0x04** the
game-start date (year, month, day), **+0x36** Turns per day, **+0x38** Turns of
Protection, **+0x185** Region Cost Change. Changing one and re-reading the
screen is the cheapest way to prove what a byte means.

**"Computer Clock has been tampered with" means the DOS date is BEHIND the date
the game last ran at, not that anything is broken.** An install driven with
synthetic dates sits in the future, so `DATE` must be set forward past it —
bisect a few years, it costs one launch each (this install needed 06-30-2027).
Do NOT go hunting for a corrupted file; the message names the clock and means
it.

**Read a price off the screen that quotes it, not out of arithmetic.** The
**Spending Menu prints the region price directly** in its Price column beside
`# Owned`, so one staged realm gives the formula at that size with no algebra.
The purchase screen's *"You can afford N regions"* looks equivalent and is not:
it is `min(affordable, Max Purchasable Regions)`, so a run of identical N across
wildly different gold is the CAP, not a price — 750 in one captured game, and
mistaking it for affordability produced a wrong price model here on 2026-09-08.
