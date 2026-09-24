---
category: controls
in_game: true
order: 1
title: "Door de menu's bewegen"
---

# Door de menu's bewegen

Druk op één toets om te kiezen. U drukt niet op Enter om een menukeuze te
maken. Elke keuze toont zijn toets tussen haakjes, zoals `(1)`. Druk op die
toets en hij wordt meteen uitgevoerd.

Elk menu verlaat u met de toets `0`, met het label "Stoppen". In een
beurtmenu (Uitgaven, Aanval, Geheime operaties, Handel) brengt Stoppen u
naar de volgende stap van uw beurt, en de beurt komt niet naar die stap
terug. In een zijmenu (zoals de Bank of het Systeemmenu) brengt Stoppen u
terug waar u was.

Het Systeemmenu opent met de toets `*` vanuit het menu Uitgaven. Het bevat
extra keuzes zoals Voorkeuren, Belasting instellen en Instructies tonen.

Enter indrukken zonder andere toets kiest ook Stoppen. Bij de prompt staat
"Stoppen", zodat u ziet wat Enter doet. In het menu Uitgaven gebeurt dat
alleen als u "Enter om koopmenu te verlaten" aanzet in Voorkeuren. In het
menu Start kiest Enter Spelen zolang u nog beurten over hebt. In elk ander
menu kiest Enter altijd Stoppen.

De hulpbrowser en de andere kies-een-onderwerplijsten verplaatsen een
markering met de pijltjestoetsen: Enter kiest de gemarkeerde regel, een paar
letters typen springt naar een titel, en Backspace of `Q` gaat terug. Kan uw
terminal geen kleur of cursorbesturing tonen, dan zijn die lijsten genummerd
- typ het nummer en druk op Enter.

## Een antwoord typen

Waar een prompt iets getypts verwacht in plaats van één toets - een aantal
soldaten, een rijknaam, een regel van een bericht - wist Backspace het
laatste teken en **Ctrl-U wist het hele antwoord**, zodat u weer bij de
prompt staat met niets getypt. Dat gaat sneller dan Backspace ingedrukt
houden boven een verkeerd getypte 1000000000. In de berichteneditor wist hij
de regel waarop u staat, niet het bericht - daar is `/C` nog steeds voor.

## Kiezen naar wie een actie gaat

Bericht verzenden vraagt `(A-Y,Z=Allen,?=Lijst) Sturen naar:` en neemt een
hele lijst, niet één naam. Druk op de letter van een rijk om het toe te
voegen en op dezelfde letter om het er weer af te halen. `Z` markeert
iedereen tegelijk, `?` toont de lijst, en `*` markeert uw
verdragspartners. **Druk op Enter als de lijst klopt** - dat opent de
editor. Enter met niets gemarkeerd sluit af zonder te versturen.

Interplanetaire operaties -> Bericht verzenden -> Eén planeet gebruikt dezelfde prompt voor de
baronnen op de planeet die u noemde.

Elke keuze in Diplomatie die een rijk noemt, neemt dezelfde lijst: bied een
verdrag aan meerdere rijken tegelijk aan, of verklaar er meerdere de
oorlog. Daar toont `?` uw relaties in plaats van de scores. Markeer maar één
rijk, en u onderhandelt ermee: u stelt het pact voor, of u aanvaardt het als
dat rijk het u al heeft aangeboden. Om een pact te beëindigen gebruikt u
Oorlogsverklaring.

De letters horen bij de rijken, niet bij de regels, dus er kan een letter
ontbreken in de lijst: die is dan van u, of van een rijk dat gevallen
is. Een rijk houdt zijn letter zolang het overeind staat, wie er verder ook
bijkomt of wegvalt, en het is op elk scherm dezelfde letter - de kolom `Id`
bij Scores bekijken is ook die letter, en daarom staan die regels niet op
letter gesorteerd. De letters van iedereen naar wie een bericht ging, staan
bovenaan als het gelezen wordt.

Een letter komt vrij zodra het rijk dat hem had van de kaart geveegd is, en
een latere baron kan hem krijgen. Een letter noemt dus wie hem vandaag
heeft, niet wie hem had toen een oud bericht geschreven werd.

## Wie er verder online is

Een `O` naast de letter van een rijk - bij Scores bekijken, in de
doellijsten voor aanvallen en berichten, en op de lijst van Verdragen
bekijken - betekent dat die baron samen met u online is. Uw eigen rijk
krijgt hem nooit. Hij verdwijnt als iemand uitlogt, en ook een paar minuten
na de laatste toetsaanslag, dus wie stil op één scherm zit kan van de lijst
vallen zonder weg te zijn.

In een menu met Help drukt u op `?` om deze hulp te openen.
