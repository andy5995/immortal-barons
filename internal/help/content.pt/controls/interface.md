---
category: controls
in_game: true
order: 1
title: 'Como Navegar pelos Menus'
---

# Como Navegar pelos Menus

Pressione uma tecla para escolher. Você não aperta Enter para escolher um
item do menu. Cada item mostra a sua tecla entre parênteses, como
`(1)`. Pressione essa tecla e o item roda na hora.

Todo menu é encerrado com a tecla `0`, chamada "Sair". Num menu de turno
(Gastos, Ataque, Operações Secretas, Comércio), Sair leva você para a
próxima etapa do turno, e o turno não volta àquela etapa. Num menu lateral
(como o Banco ou o Menu do Sistema), Sair leva você de volta para onde
estava.

O Menu do Sistema abre com a tecla `*` a partir do menu de Gastos. Ele
guarda opções extras como Preferências, Definir o Imposto e Mostrar as
Instruções.

Pressionar Enter sem nenhuma outra tecla também escolhe Sair. O prompt
mostra "Sair" para você ver o que o Enter vai fazer. No menu de Gastos, isso
só acontece se você ligar "Use Enter para sair do menu de compra" nas
Preferências. No menu de Entrada, o Enter escolhe Jogar enquanto você ainda
tem turnos. Em todos os outros menus, o Enter sempre escolhe Sair.

O navegador da ajuda e as outras listas de escolher um tópico movem um
destaque com as setas: o Enter escolhe a linha marcada, digitar algumas
letras salta para um título, e Backspace ou `Q` volta. Se o seu terminal não
mostra cores nem controla o cursor, essas listas aparecem numeradas — digite
o número e pressione Enter.

## Digitando uma resposta

Onde um prompt espera algo digitado em vez de uma tecla só (um número de
soldados, o nome de um reino, uma linha de mensagem), o Backspace apaga o
último caractere e o **Ctrl-U apaga a resposta inteira**, deixando você de
volta no prompt sem nada digitado. É mais rápido do que segurar o Backspace
sobre um 1000000000 digitado errado. No editor de mensagens, ele limpa a
linha em que você está, não a mensagem; para isso o `/C` continua valendo.

## Escolhendo para quem vai uma ação

Enviar Mensagem pergunta `(A-Y,Z=Todos,?=Lista) Enviar para:` e aceita uma
lista inteira, não um nome só. Pressione a letra de um reino para incluí-lo
e pressione a mesma letra de novo para tirá-lo. `Z` marca todo mundo de uma
vez, `?` mostra a lista de reinos e `*` marca seus parceiros de
tratado. **Pressione Enter quando a lista estiver certa** — é isso que abre
o editor. Enter sem nada marcado sai sem enviar.

Operações Interplanetárias -> Enviar Mensagem -> Um Planeta usa o mesmo prompt para os
barões do planeta que você indicou.

Toda opção de Diplomacia que indica um reino aceita a mesma lista: ofereça
um tratado a vários reinos de uma vez, ou declare guerra a vários. Ali o `?`
mostra as suas relações em vez do placar. Marque um único reino e você
negocia com ele: você propõe o pacto, ou o aceita se aquele reino já o
ofereceu a você. Para encerrar um pacto, use Declaração de Guerra.

As letras pertencem aos reinos, não às linhas, então pode faltar uma letra
na lista: ou ela é a sua, ou é de um reino que caiu. Um reino guarda a sua
letra enquanto estiver de pé, não importa quem mais entre ou caia, e é a
mesma letra em todas as telas — a coluna `Id` em Ver o Placar é essa letra
também, e é por isso que aquelas linhas não estão em ordem alfabética. As
letras de todos para quem a mensagem foi aparecem no topo dela quando é
lida.

Uma letra fica livre quando o reino que a tinha é varrido do mapa, e um
barão mais novo pode recebê-la. Então uma letra nomeia quem a tem hoje, não
quem a tinha quando uma mensagem antiga foi escrita.

## Quem mais está conectado

Um `O` ao lado da letra de um reino — em Ver o Placar, nas listas de alvos
de ataque e de mensagem, e na lista de Ver os Tratados — quer dizer que
aquele barão está conectado junto com você. O seu próprio reino nunca tem
essa marca. Ela some quando a pessoa se desconecta, e também alguns minutos
depois da última tecla dela, então alguém parado numa tela pode sumir da
lista sem ter saído.

Num menu que lista Ajuda, pressione `?` para abrir esta ajuda.
