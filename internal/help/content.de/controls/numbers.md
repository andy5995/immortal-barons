---
category: controls
in_game: true
order: 2
title: 'Zahlen eingeben'
---

# Zahlen eingeben

Wenn das Spiel nach einer Menge fragt, zeigt es zwei Zahlen in Klammern, wie
`How many? (0; 40):`.

- Die linke Zahl ist ein Vorschlagswert. Drücken Sie die Eingabetaste, um
  ihn zu übernehmen.
- Die rechte Zahl ist das Höchste, was Sie wählen können. Geben Sie `>` ein,
  um das Feld mit diesem Maximum zu füllen. Sie können danach weiterhin eine
  kleinere Zahl eingeben.
- `k` fügt beim Tippen drei Nullen hinzu. Drücken Sie `1` und dann `k`, und
  Sie sehen `1000`.
- `m` fügt sechs Nullen hinzu. Drücken Sie `2` und dann `m`, und Sie sehen
  `2000000`.
- `b` adds nine zeros. Press `3` then `b` and you see `3000000000`.

Wenn Sie eine Zahl eingeben, die größer als das Maximum ist, senkt das Spiel
sie beim Drücken der Eingabetaste auf das Maximum.

## Very large numbers

A number of a billion or more is shown in short form with a capital `B`, to
keep it inside its column: `1.0000B` is one billion, and `1.8473B` is a
little over one and four fifths of a billion. The four digits after the
point are the part below a billion, cut off rather than rounded up, so a
figure just short of the next billion never looks like it got there. You
still type these numbers in full, or with the `b` key above.
