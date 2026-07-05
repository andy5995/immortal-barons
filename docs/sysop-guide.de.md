# Sysop-Anleitung — Immortal Barons betreiben

Diese Anleitung ist für den BBS-Betreiber (den Sysop). Sie erklärt, wie Sie
das Spiel auf Ihrem Board einrichten und einer Inter-BBS-Liga
beitreten. Spieler müssen sie nicht lesen.

## Was Sie brauchen

Das Spiel ist ein einziges Programm: `barons-door`. Es läuft als nativer
Door unter moderner BBS-Software wie Synchronet oder Mystic. Sie brauchen
weder DOSBox noch DOSEMU.

Das Spiel hält alle seine Dateien in einem Datenverzeichnis (Standard
`./data`). Richten Sie die Option `-data` darauf.

## Ersteinrichtung

Führen Sie die Einrichtung einmal aus, um die Konfigurationsdatei zu
erstellen:

```
barons-door -setup -data /path/to/data
```

Es stellt ein paar Fragen (Züge pro Tag, Schutzzüge, Anzahl der KI-Barone,
Spieldauer) und schreibt `config.json`.

## Den Door registrieren

Richten Sie das Spiel als externes Programm (einen „Door“) in Ihrer
BBS-Software ein. Ihr BBS schreibt eine Dropfile-Datei, wenn ein Anrufer den
Door startet. Das Spiel liest sie, um zu erfahren, wer spielt und wie viel
Zeit er hat.

- Das Spiel liest **DOOR32.SYS** (Synchronet und Mystic schreiben beide
  diese) oder **DOOR.SYS**. Weisen Sie das Spiel mit `-dropfile` darauf hin,
  oder lassen Sie es das aktuelle Verzeichnis durchsuchen.
- Der Handle des Anrufers aus der Dropfile wird zum Namen seines Reichs.

Eine typische Befehlszeile:

```
barons-door -dropfile /path/to/DOOR32.SYS -data /path/to/data
```

## Tägliche Wartung

Führen Sie die Wartung einmal am Tag aus (ein nächtliches Ereignis ist der
übliche Ort):

```
barons-door -maint -data /path/to/data
```

Dies bringt das Spiel für jeden vergangenen Tag einen Tag voran, lässt die
KI-Barone ihre Züge machen und erneuert die Züge jedes Spielers.

## Starting a fresh game (reset)

To wipe the current game and start over, run:

```
barons-door -reset -data /path/to/data
```

This clears all empires (players re-create their realm the next time they
log in) and re-seeds the AI barons on a fresh day one. It does not pick a
winner.  The old world is saved to `world.json.bak` in the data directory
first, so you can restore it if you reset by mistake.

## Inter-BBS-(Liga-)Spiel

Eine Liga ist eine Gruppe von Boards, deren Spieler gegeneinander
antreten. Um beizutreten, schalten Sie das Inter-BBS-Spiel ein und geben
Ihrem Board einen Namen.

Bearbeiten Sie `config.json` und setzen Sie:

- `"IBBS": true` — die Inter-BBS-Menüs einschalten.
- `"BoardID"` — ein kurzer, eindeutiger Name für Ihr Board (Ihren
  „Planeten“).
- `"InboundDir"` — das Verzeichnis, in dem Pakete von anderen Boards
  eintreffen.
- `"OutboundDir"` — das Verzeichnis, in das das Spiel Pakete für andere
  Boards schreibt.

Bitten Sie Ihren Liga-Koordinator um die Knotenliste und legen Sie sie als
**`ibnodes.dat`** in Ihr Datenverzeichnis (siehe Format unten).

## Wie Pakete sich bewegen (Sie wählen den Zeitplan)

Das Spiel bewegt niemals Dateien zwischen Boards. Es liest und schreibt nur
Paketdateien in Ihren Eingangs- und Ausgangsverzeichnissen. Die Dateien
zwischen den Boards zu bewegen, ist Ihre Aufgabe, und Sie bestimmen, wie oft
das geschieht.

Der Inter-BBS-Schritt ist:

```
barons-door -planetary -data /path/to/data
```

Es liest jedes Paket in Ihrem Eingangsverzeichnis, wendet es an und schreibt
neue Pakete in Ihr Ausgangsverzeichnis. Es läuft auch automatisch innerhalb
von `-maint`, wenn das Inter-BBS-Spiel eingeschaltet ist.

Ein üblicher Aufbau:

1. Ein Anrufer spielt das Spiel.
2. Nachdem der Anrufer beendet hat, oder nach einem von Ihnen gewählten
   Zeitplan, führen Sie `-planetary` aus.
3. Ihr Transportskript kopiert jede Datei aus Ihrem Ausgangsverzeichnis in
   das Eingangsverzeichnis jedes anderen Boards (über FidoNet, ein
   Sync-Werkzeug, scp, ein gemeinsames Laufwerk — was auch immer Sie
   nutzen).
4. Der nächste `-planetary`-Lauf auf jedem Board liest und wendet diese
   Dateien an.

Führen Sie es so oft aus, wie Sie möchten. Öfter bedeutet kürzere
Reisezeiten zwischen Planeten. Der Ingame-Bildschirm „Reisezeiten“ zeigt den
Spielern, wie kürzlich Pakete eingetroffen sind, damit sie wissen, wie
schnell Operationen ablaufen.

## Die Knotenliste: `ibnodes.dat`

Die Knotenliste nennt jedes Board in der Liga. Sie verwendet dasselbe
einfache Layout wie die ursprüngliche BRE-`BRNODES.DAT`. Jedes Board umfasst
sechs Zeilen, und eine Leerzeile trennt die Boards:

```
1
Avalon
363/277
Orlando
FL
USA

2
Pier 7
106/477
Houston
TX
USA
```

Die sechs Zeilen sind: Knotennummer, Board-(Planeten-)Name, Netzwerkadresse,
Stadt, Bundesland oder Provinz und Land. Board Nummer 1 ist der
Liga-Koordinator.

## League-wide rules (Coordinator only)

The League Coordinator sets the rules that must match across the whole
league.  These are all the fields marked with a star in the Configuration
Editor: turns per day, protection turns, game length, land market and daily
land, interest and investment rates, tax and region and player limits,
buy-military mode, and the cost, damage, and reward levels. Set them in the
Coordinator's own `config.json`, then broadcast them to every board:

```
barons-door -league-config -data /path/to/data
```

This writes a settings packet to your outbound directory. Each member board
adopts the settings on its next `-planetary` run. Member boards accept these
settings only from the Coordinator's board (node 1), so no one else can
change the league rules. Only the Coordinator's board may send this packet.

## Der Koordinator

Es gibt zwei verschiedene „Koordinator“-Begriffe, und sie sind nicht
dasselbe:

- **Liga-Koordinator** — der Sysop von Board Nummer 1. Diese Person pflegt
  die Knotenliste und gibt sie an die anderen Boards weiter.
- **BBS-Koordinator** — ein *Spieler*, den Ihr Board wählt. Spieler stimmen
  im Systemmenü ab, und der Spieler mit den meisten Stimmen erhält das
  Koordinatormenü. Stimmen können jederzeit geändert werden.
