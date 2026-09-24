---
category: controls
in_game: true
order: 1
title: 'Durch die Menüs navigieren'
---

# Durch die Menüs navigieren

Drücken Sie eine Taste zur Auswahl. Sie drücken nicht die Eingabetaste, um
einen Menüpunkt zu wählen. Jeder Punkt zeigt seine Taste in runden Klammern,
wie `(1)`. Drücken Sie diese Taste, und er wird sofort ausgeführt.

Jedes Menü verlassen Sie mit der Taste `0`, beschriftet "Beenden". In einem
Zugmenü (Ausgaben, Angriff, Verdeckt, Handel) bringt Beenden Sie zum
nächsten Schritt Ihres Zuges, und der Zug kehrt nicht zu diesem Schritt
zurück. In einem Nebenmenü (etwa der Bank oder dem Systemmenü) bringt
Beenden Sie dorthin zurück, wo Sie waren.

Das Systemmenü öffnet sich mit der Taste `*` im Ausgabenmenü. Es enthält
zusätzliche Optionen wie Einstellungen, Steuersatz festlegen und Anweisungen
anzeigen.

Die Eingabetaste ohne weitere Taste wählt ebenfalls Beenden. Die
Eingabezeile zeigt "Beenden", damit Sie sehen, was die Eingabetaste
bewirkt. Im Menü Ausgaben geschieht das nur, wenn Sie "Mit Enter das
Kaufmenü verlassen" in den Einstellungen einschalten. Im Startmenü wählt die
Eingabetaste Spielen, solange Sie noch Züge haben. In jedem anderen Menü
wählt die Eingabetaste immer Beenden.

Der Hilfebrowser und andere Themenlisten bewegen eine Markierung mit den
Pfeiltasten: Eingabe wählt die markierte Zeile, das Tippen einiger
Buchstaben springt zu einem Titel, und Rücktaste oder `Q` führt zurück. Wenn
Ihr Terminal weder Farbe noch Cursorsteuerung darstellen kann, sind diese
Listen stattdessen nummeriert - tippen Sie die Nummer und drücken Sie
Eingabe.

## Eine Antwort eintippen

Wo eine Eingabezeile etwas Getipptes statt einer einzelnen Taste erwartet -
eine Zahl von Soldaten, einen Reichsnamen, eine Zeile einer Nachricht -,
löscht die Rücktaste das letzte Zeichen und **Strg-U löscht die ganze
Antwort**, so dass Sie ohne Eingetipptes wieder an der Eingabezeile
stehen. Das geht schneller, als die Rücktaste über einer vertippten
1000000000 gedrückt zu halten. Im Nachrichteneditor löscht sie die Zeile, in
der Sie stehen, nicht die Nachricht - das tut weiterhin `/C`.

## Wählen, an wen eine Aktion geht

Nachricht senden fragt `(A-Y,Z=All,?=List) Send to:` und nimmt eine ganze
Liste entgegen, nicht einen einzelnen Namen. Drücken Sie den Buchstaben
eines Reiches, um es aufzunehmen, und denselben Buchstaben erneut, um es
wieder zu streichen. `Z` markiert alle auf einmal, `?` zeigt die Liste der
Reiche, und `*` markiert Ihre Vertragspartner. **Drücken Sie Eingabe, wenn
die Liste stimmt** - das ist es, was den Editor öffnet. Eingabe ohne jede
Markierung beendet den Vorgang, ohne zu senden.

Interplanetare Operationen -> Nachricht senden -> Einzelner Planet nutzt dieselbe
Eingabezeile für die Barone auf dem genannten Planeten.

Jede Diplomatie-Option, die ein Reich benennt, nimmt dieselbe Liste: Bieten
Sie mehreren Reichen zugleich einen Vertrag an, oder erklären Sie mehreren
den Krieg. Dort zeigt `?` Ihre Beziehungen statt der Punkte. Markieren Sie
nur ein Reich, verhandeln Sie mit ihm: Sie schlagen den Pakt vor, oder Sie
nehmen ihn an, wenn dieses Reich ihn Ihnen schon angeboten hat. Um einen
Pakt zu beenden, verwenden Sie die Kriegserklärung.

Die Buchstaben gehören den Reichen, nicht den Zeilen, daher kann ein
Buchstabe in der Liste fehlen: Er ist entweder Ihr eigener oder gehört einem
gefallenen Reich. Ein Reich behält seinen Buchstaben, solange es besteht,
gleich wer sonst hinzukommt oder fällt, und es ist auf jedem Bildschirm
derselbe Buchstabe - die Spalte `Id` in Punkte ansehen ist ebenfalls dieser
Buchstabe, weshalb jene Zeilen nicht nach Buchstaben geordnet sind. Die
Buchstaben aller Empfänger einer Nachricht stehen beim Lesen oben in ihr.

Ein Buchstabe wird frei, wenn das Reich, das ihn hält, von der Karte getilgt
wird, und ein späterer Baron kann ihn erhalten. Ein Buchstabe nennt also den
heutigen Inhaber, nicht den, der ihn hielt, als eine alte Nachricht
geschrieben wurde.

## Wer sonst noch online ist

Ein `O` neben dem Buchstaben eines Reiches - in Punkte ansehen, in den
Ziellisten für Angriff und Nachricht und in der Verträge ansehen-Liste -
bedeutet, dass dieser Baron mit Ihnen auf dem Board ist. Ihr eigenes Reich
trägt es nie. Es verschwindet, wenn er sich abmeldet, und auch einige
Minuten nach seinem letzten Tastendruck, so dass jemand, der auf einem
Bildschirm verweilt, von der Liste fallen kann, ohne gegangen zu sein.

In einem Menü, das Hilfe anbietet, öffnet `?` diese Hilfe.
