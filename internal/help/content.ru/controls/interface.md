---
category: controls
in_game: true
order: 1
title: 'Перемещение по меню'
---

# Перемещение по меню

Нажмите одну клавишу для выбора. Не нужно нажимать Enter, чтобы выбрать
пункт меню. У каждого пункта клавиша показана в скобках, например
`(1)`. Нажмите эту клавишу — и он сразу выполнится.

Every menu leaves with the `0` key, labeled "Quit". On a turn menu
(Spending, Attack, Covert, Trading), Quit moves you to the next step of your
turn; you cannot go back to that menu until your next turn. On a side menu
(like the Bank or the System Menu), Quit takes you back to where you were.

Системное меню открывается клавишей `*` из меню Расходов. В нём собраны
дополнительные пункты: Настройки, Задать налог и Показать инструкции.

Pressing Enter with no other key also chooses Quit — the prompt shows "Quit"
so you can see what Enter will do. On a turn menu, this only happens if you
turn on "Enter exits Buy menu" in Preferences; on a side menu, Enter always
chooses Quit.

The help browser and other pick-a-topic lists move a highlight with the
arrow keys: Enter chooses the marked row, typing a few letters jumps to a
title, and Backspace or `Q` goes back. If your terminal cannot show colour
and cursor control, those lists are numbered instead — type the number and
press Enter.

## Choosing who an action goes to

Send Message asks `(A-Y,Z=All,?=List) Send to:` and takes a whole list, not
one name. Press a realm's letter to add it and press the same letter again
to take it off. `Z` marks everyone at once, `?` shows the roster, and `*`
marks your treaty partners. **Press Enter when the list is right** — that is
what opens the editor. Enter with nothing marked leaves without sending.

InterPlanetary Ops -> Send Message -> Single Planet uses the same prompt for the
barons on the planet you named.

Every Diplomacy option that names a realm takes the same list: offer a
treaty to several realms at once, or declare war on several. There `?` shows
your relations rather than the scores. Mark one realm only and you get the
full negotiation with it — you can accept its offer or break the pact you
hold.

The letters belong to the realms, not to the rows, so a letter may be
missing from the list: it is either yours or a realm that has fallen. A
realm keeps its letter for as long as it stands, whoever else joins or
falls, and it is the same letter on every screen — the `Id` column on See
Scores is that letter too, which is why those rows are not in letter
order. The letters of everyone a message went to appear at the top of it
when it is read.

A letter is freed when the realm holding it is swept from the map, and a
later baron may be given it. So a letter names whoever holds it today, not
whoever held it when an old message was written.

## Who else is on

An `O` beside a realm's letter — on See Scores, the attack and message
target lists, and the View Treaties roster — means that baron is on the
board with you. Your own realm never carries it. It clears when they log
off, and also a few minutes after their last keypress, so someone sitting on
one screen can drop off the list without having left.

Нажмите `?` в любом меню, чтобы открыть эту справку.
