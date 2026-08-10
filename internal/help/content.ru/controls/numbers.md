---
category: controls
in_game: true
order: 2
title: 'Ввод чисел'
---

# Ввод чисел

Когда игра просит указать количество, она показывает два числа в скобках,
например `How many? (0; 40):`.

- Левое число — рекомендуемое значение. Нажмите Enter, чтобы принять его.
- Правое число — максимум, который можно выбрать. Введите `>`, чтобы
  заполнить поле этим максимумом. После этого всё ещё можно ввести меньшее
  число.
- `k` добавляет три нуля при вводе. Нажмите `1`, затем `k` — и вы увидите
  `1000`.
- `m` добавляет шесть нулей. Нажмите `2`, затем `m` — и вы увидите
  `2000000`.
- `b` adds nine zeros. Press `3` then `b` and you see `3000000000`.

Если ввести число больше максимума, игра снизит его до максимума при нажатии
Enter.

## Very large numbers

A number of a billion or more is shown in short form with a capital `B`, to
keep it inside its column: `1.0000B` is one billion, and `1.8473B` is a
little over one and four fifths of a billion. The four digits after the
point are the part below a billion, cut off rather than rounded up, so a
figure just short of the next billion never looks like it got there. You
still type these numbers in full, or with the `b` key above.
