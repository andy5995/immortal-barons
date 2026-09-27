<!-- Extracted from SKILL.md so the always-loaded core stays small. The
skill points here; load it when the task actually needs it. -->

# BRE licensing: the background

The working rules are in SKILL.md's license section. This is the reasoning behind
them.

**What BRE's own license says (scanned 2026-07, `docs/bre.doc` — the license
lives only there; `register.doc`/`whatsnew.doc`/`*.hlp`/`breins.txt` add no
terms).** The BRE "SOFTWARE LICENSE AGREEMENT" has **no anti-disassembly or
anti-reverse-engineering clause** — reading/disassembling the binary for study
is not addressed. What it *does* forbid: "You may not **alter** the
machine-readable object files or program documentation files" (esp. to defeat
the registration key or modify the copyright/text), removing/modifying the
copyright notice, and selling or bundling-for-fee. So disassembling BRE to learn
a constant/formula for OUR own implementation is not prohibited by its license;
altering `BRE.EXE`/`BRE.OVR` is. (Editing a local *save* file — `game.dat` — for
study is a different thing from altering the object files, but revert it and
never redistribute.) Not legal advice; absence of a clause ≠ affirmative
permission, and jurisdictions differ — the clean-room "no copying expression"
posture in SKILL.md is the safe line regardless.

**BRE is shareware** (60-day evaluation + registration key), and under US law
disassembling software to reach its *unprotected functional elements* (ideas,
mechanics, constants) is settled **fair use** — *Sega v. Accolade*, 977 F.2d
1510 (9th Cir. 1992) and *Sony v. Connectix*, 203 F.3d 596 (9th Cir. 2000): the
"intermediate copying" that disassembly requires is fair use when it's the only
way to get at the functional ideas and the result is an independent
implementation (exactly the clean-room clone here). The one live caveat is DMCA
§1201 anti-circumvention — but that bites only if you defeat the registration
"key system," which we never do (we read game math, not the key). Interpol is
irrelevant (a police-coordination body, not a lawmaker); cross-border norms come
from treaties (Berne/TRIPS) and the EU Software Directive 2009/24/EC, which
*expressly* permits decompilation for interoperability. Verified current 2026-07;
no ruling has disturbed Sega/Sony. Sources: EFF Coders' Rights Reverse
Engineering FAQ (`eff.org/issues/coders/reverse-engineering-faq`), *Sega v.
Accolade* (Wikipedia / BitLaw), *Sony v. Connectix* (digital-law-online.info).

Not legal advice. The idea/expression line above reflects general copyright
principles, not a ruling on any specific case. When unsure whether something has
crossed from *mechanic* into *expression*, ask a project maintainer or check the
copyright law that applies in your jurisdiction rather than assume — and when in
doubt, don't copy.
