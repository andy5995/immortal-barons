---
category: interbbs
in_game: false
order: 9
title: 'Руководство сисопа — запуск Immortal Barons'
---

# Руководство сисопа — запуск Immortal Barons

Это руководство для оператора BBS (сисопа). Оно объясняет, как настроить
игру на вашей доске и присоединиться к межсистемной лиге. Игрокам его читать
не нужно.

## Что нужно

Игра — это одна программа: `barons-door`. Она работает как нативная дверь
под современным ПО BBS, например Synchronet или Mystic. DOSBox или DOSEMU не
нужны.

Игра хранит все файлы в одном каталоге данных (по умолчанию
`./data`). Укажите на него параметром `-data`.

## Первоначальная настройка

Запустите настройку один раз, чтобы создать файл конфигурации:

```
barons-door -setup -data /path/to/data
```

Она задаёт несколько вопросов (ходов в день, ходов защиты, число ИИ-баронов,
длительность игры) и записывает `config.json`.

## Регистрация двери

Настройте игру как внешнюю программу («дверь») в ПО вашей BBS. Ваша BBS
записывает dropfile, когда абонент запускает дверь. Игра читает его, чтобы
узнать, кто играет и сколько у него времени.

- Игра читает **DOOR32.SYS** (его пишут и Synchronet, и Mystic) или
  **DOOR.SYS**. Укажите на него параметром `-dropfile` или дайте игре найти
  его в текущем каталоге.
- Псевдоним абонента из dropfile становится названием его королевства.

Типичная командная строка:

```
barons-door -dropfile /path/to/DOOR32.SYS -data /path/to/data
```

## Ежедневное обслуживание

Запускайте обслуживание раз в день (обычно это ночное событие):

```
barons-door -maint -data /path/to/data
```

Это продвигает игру на один день за каждый прошедший день, даёт ИИ-баронам
сделать ходы и обновляет ходы каждого игрока.

## Starting a fresh game (reset)

To wipe the current game and start over, run:

```
barons-door -reset -data /path/to/data
```

This clears all empires (players re-create their realm the next time they
log in) and re-seeds the AI barons on a fresh day one. It does not pick a
winner.  The old world is saved to `world.json.bak` in the data directory
first, so you can restore it if you reset by mistake.

## Межсистемная (лиговая) игра

Лига — это группа досок, игроки которых соревнуются друг с другом. Чтобы
вступить, включите межсистемную игру и дайте своей доске имя.

Отредактируйте `config.json` и задайте:

- `"IBBS": true` — включить межсистемные меню.
- `"BoardID"` — короткое уникальное имя вашей доски (вашей «планеты»).
- `"InboundDir"` — каталог, куда приходят пакеты от других досок.
- `"OutboundDir"` — каталог, куда игра записывает пакеты для других досок.

Попросите у координатора лиги список узлов и положите его в каталог данных
как **`ibnodes.dat`** (формат см. ниже).

## Как перемещаются пакеты (расписание выбираете вы)

Игра никогда не перемещает файлы между досками. Она лишь читает и пишет
файлы пакетов в ваших входящем и исходящем каталогах. Перемещать файлы между
досками — ваша задача, и вы решаете, как часто это делать.

Межсистемный шаг:

```
barons-door -planetary -data /path/to/data
```

Он читает каждый пакет во входящем каталоге, применяет его и пишет новые
пакеты в исходящий каталог. Он также выполняется автоматически внутри
`-maint`, когда включена межсистемная игра.

Типичная схема:

1. Абонент играет.
2. После выхода абонента или по выбранному вами расписанию запустите
   `-planetary`.
3. Ваш транспортный скрипт копирует каждый файл из исходящего каталога во
   входящий каталог каждой другой доски (через FidoNet, средство
   синхронизации, scp, общий диск — что используете).
4. Следующий запуск `-planetary` на каждой доске читает и применяет эти
   файлы.

Запускайте его так часто, как хотите. Чаще — короче время в пути между
планетами. Экран «Время в пути» в игре показывает игрокам, как недавно
приходили пакеты, чтобы они знали скорость операций.

## Список узлов: `ibnodes.dat`

Список узлов перечисляет каждую доску лиги. Он использует ту же простую
разметку, что и оригинальный `BRNODES.DAT` из BRE. Каждая доска — шесть
строк, доски разделяет пустая строка:

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

Шесть строк: номер узла, имя доски (планеты), сетевой адрес, город, область
или провинция и страна. Доска номер 1 — координатор лиги.

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

## Координатор

Есть два разных понятия «координатор», и это не одно и то же:

- **Координатор лиги** — сисоп доски номер 1. Он ведёт список узлов и
  раздаёт его другим доскам.
- **Координатор BBS** — *игрок*, которого выбирает ваша доска. Игроки
  голосуют в системном меню, и игрок с наибольшим числом голосов получает
  меню координатора. Голос можно менять в любой момент.
