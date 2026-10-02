---
category: controls
in_game: true
order: 1
title: 'Navigera i menyerna'
---

# Navigera i menyerna

Tryck en tangent för att välja. Du trycker inte Enter för att välja ett
menyval. Varje val visar sin tangent inom parentes, som `(1)`. Tryck den
tangenten och det körs genast.

Varje meny lämnas med tangenten `0`, märkt "Avsluta". I en dragmeny
(Utgifter, Anfall, Hemliga operationer, Handel) flyttar Avsluta dig till
nästa steg i draget, och draget går inte tillbaka till det steget. I en
sidomeny (som Banken eller Systemmenyn) tar Avsluta dig tillbaka dit du var.

Systemmenyn öppnas med tangenten `*` från Utgifter-menyn. Den innehåller
extra alternativ som Inställningar, Ange skattesats och Visa instruktioner.

Att trycka Enter utan någon annan tangent väljer också Avsluta. Prompten
visar "Avsluta" så att du ser vad Enter gör. I Utgifter-menyn sker detta
bara om du slår på "Använd Enter för att lämna KÖP-menyn" i Inställningar. I
Ingångsmenyn väljer Enter Spela så länge du har drag kvar. I alla andra
menyer väljer Enter alltid Avsluta.

Hjälpläsaren och andra listor där du väljer ett ämne flyttar en markering
med piltangenterna: Enter väljer den markerade raden, några bokstäver hoppar
till en titel, och Backsteg eller `Q` går tillbaka. Om din terminal inte kan
visa färg och markörstyrning är listorna numrerade i stället - skriv numret
och tryck Enter.

## Att skriva ett svar

Där en prompt tar emot något skrivet istället för en tangent - ett antal
soldater, ett rikesnamn, en rad i ett meddelande - raderar Backspace det
senaste tecknet och **Ctrl-U raderar hela svaret**, så att du står tillbaka
vid prompten utan något skrivet. Det är snabbare än att hålla Backspace
nedtryckt över ett feltippat 1000000000. I meddelanderedigeraren rensar det
raden du står på, inte meddelandet - `/C` gör fortfarande det.

## Välja vem en åtgärd riktas mot

Skicka meddelande frågar `(A-Y,Z=All,?=List) Send to:` och tar emot en hel
lista, inte ett namn. Tryck ett rikes bokstav för att lägga till det och
tryck samma bokstav igen för att ta bort det. `Z` markerar alla på en gång,
`?` visar listan, och `*` markerar dina fördragspartner. **Tryck Enter när
listan är rätt** - det är det som öppnar redigeraren. Enter utan något
markerat lämnar utan att skicka.

Interplanetära operationer -> Skicka meddelande -> Enskild planet använder samma prompt för
baronerna på planeten du namngav.

Varje diplomatialternativ som namnger ett rike tar samma lista: erbjud ett
fördrag till flera riken samtidigt, eller förklara krig mot flera. Där visar
`?` dina relationer istället för poängen. Markera bara ett rike, och du
förhandlar med det: du föreslår pakten, eller accepterar den om det riket
redan har erbjudit dig den. För att avsluta en pakt, använd Krigsförklaring.

Bokstäverna hör till rikena, inte till raderna, så en bokstav kan saknas i
listan: den är antingen din eller ett rike som har fallit. Ett rike behåller
sin bokstav så länge det står, oavsett vem annan som ansluter eller faller,
och det är samma bokstav på varje skärm - kolumnen `Id` i Visa poäng är
också den bokstaven, vilket är varför de raderna inte står i
bokstavsordning. Bokstäverna för alla ett meddelande gick till visas överst
i det när det läses.

En bokstav frigörs när riket som har den sveps bort från kartan, och en
senare baron kan tilldelas den. Så en bokstav namnger den som har den idag,
inte den som hade den när ett gammalt meddelande skrevs.

## Vem annan är inloggad

Ett `O` bredvid ett rikes bokstav - i Visa poäng, anfalls- och
meddelandemållistorna, samt listan i Visa fördrag - betyder att den baronen
är inloggad på brädan med dig. Ditt eget rike bär aldrig det. Det försvinner
när de loggar ut, och även några minuter efter deras senaste
tangenttryckning, så någon som sitter stilla på en skärm kan falla ur listan
utan att ha lämnat.

I en meny som listar Hjälp, tryck `?` för att öppna den här hjälpen.
