# Bulletin Files

This page is for the BBS operator (the sysop). Players do not need to read it.
For the rest of setting the game up, see [Door Setup](door-setup.md).

The game writes its scoreboard, its news and its rankings out as files, so your
BBS can show them on its bulletin menu and your website can carry them as pages.
Each one is a screen the game shows a caller, drawn to a file instead of a
terminal, so what a reader sees is what a player sees.

## Turning them on

Set `BulletinDir` in `bbs.cfg` to a directory:

```
BulletinDir   /home/bbs/xtrn/imb/bull
```

The game writes the files there whenever it runs its daily or planetary step.
Leave the setting blank and none are written.

A path with no drive letter and no leading slash is read as being inside the
data directory, the same as the packet directories.

## What gets written

Every board writes three:

| File | Shows |
| --- | --- |
| `scores` | The scoreboard |
| `tdynews` | Today's planet news |
| `yesnews` | Yesterday's planet news |

A board in a league writes nine more. These belong to the league, so a board
playing alone has nothing to put in them and writes none of them:

| File | Shows |
| --- | --- |
| `world` | Every battle fought anywhere in the league |
| `bbsscore` | Top planets by score |
| `bbsworth` | Top planets by net worth |
| `bbsland` | Top planets by land |
| `bbswland` | Top planets by net worth per region |
| `plyscore` | Top players by score |
| `plyworth` | Top players by net worth |
| `plyland` | Top players by land |
| `plywland` | Top players by net worth per region |

`bbswland` and `plywland` rank by net worth **per region**, not by land. The
name does not give that away, so it is worth saying once.

## For your BBS menu

Each bulletin is written twice: `.ans` with colour and `.txt` without. Point a
bulletin menu entry at whichever your callers can read.

Both are CP437, the character set BBS software and ANSI art programs expect, so
the rules and blocks come out as single characters. You can open a `.ans` in
PabloDraw or Moebius and it will look right.

## For your website

Each bulletin is written as two HTML files as well, because boards do two
different things with a scoreboard:

- `scores.html` — the whole page, to link to
- `scores.inc.html` — the block on its own, to drop into a page you already
  build

You get both, so you do not have to choose.

### The three files you own

The first run that finds them missing writes three more:

- `header.html` — everything above the bulletin, including the whole `<head>`
- `footer.html` — everything below it
- `bulletin.css` — the stylesheet

**The game never writes over these three again**, so your edits survive every
later run. Delete one and the next run puts a fresh copy back, which is how you
start over.

Your nav, your back-link and any meta tags go in the header and footer. Nothing
about them is fixed: the game only fills in six words, and the rest of the
markup is yours.

| Word | Filled in with |
| --- | --- |
| `{{title}}` | The name of the bulletin, "Scoreboard" or "Top Planets by Land" |
| `{{bbs}}` | What your board is called — `BBSName` in `bbs.cfg`, else `BoardID` |
| `{{boardurl}}` | Your board's own website — `BoardURL` in `bbs.cfg` |
| `{{pageurl}}` | The address of this page — `BulletinURL` plus the file name |
| `{{date}}` | The game day the bulletin was written for |
| `{{game}}` | The name of the game |

Delete a word and it simply does not appear.

The three `bbs.cfg` lines they read are covered in
[the settings below](#the-bbscfg-lines).

### Meta tags

Add them to `header.html`. The words above work inside a tag, so a page can
carry its own title, address and date:

```
<link rel="canonical" href="{{pageurl}}">
<meta name="description" content="{{title}} on {{bbs}}, a {{game}} planet.">
<meta property="og:title" content="{{title}}">
<meta property="og:site_name" content="{{bbs}}">
<meta property="og:url" content="{{pageurl}}">
<meta property="og:updated_time" content="{{date}}">
```

A tag whose word comes out blank is taken out of the page rather than left
empty. So you can add all of these now and fill in `BulletinURL` later; until
you do, those pages simply carry no address. A link is handled the same way but
keeps its text, so your board's name reads as plain text when `BoardURL` is not
set, instead of linking back to the page being read.

### Colours and the stylesheet

The colours arrive as class names, `<span class="ansi-fg-14">` and
`<span class="ansi-bg-4">`, numbered 0 to 15. A stylesheet can repaint a class;
it could not repaint a colour written into the tag.

`bulletin.css` is a starting point rather than a requirement. The only thing
naming it is a `<link>` in the default header, so pointing that at your own
sheet is a one-line edit and the game's copy then goes unused.

The pages it draws are black to the edges and set in one monospace face, so the
bulletin sits on the same ground it was drawn for.

The game's own name, at the top of every bulletin, links to the game's website.
It carries no underline and keeps whatever colour the screen drew it in; a
highlight bar marks it and brightens when you point at it, the way a DOS menu
marks the line under the cursor.

If you write your own sheet, note that the dark half of the sixteen colours is
too dim to read as body text on black. The game's own screens keep every figure
and heading on a bright colour or white and use the dark ones for parentheses
and banner dashes, so this is a limit of the palette rather than of any one
page. `bulletin.css` lifts the dark colours for a reader whose system asks for
more contrast.

## The `bbs.cfg` lines

Four settings in `bbs.cfg` belong to the bulletins. All four are optional except
`BulletinDir`, without which nothing is written at all.

```
BulletinDir   bull
BBSName       The Dog House BBS
BoardURL      https://doghouse.example/
BulletinURL   https://doghouse.example/bulletins/
```

- **`BulletinDir`** — where the files are written.
- **`BBSName`** — what your board is called on the web pages. Write it however
  you want it read. It is separate from `BoardID` because `BoardID` has to match
  the league roster character for character, and a name written for a reader
  should not be pinned to that. Left blank, the pages use `BoardID`.
- **`BoardURL`** — your board's own website. The header links your board's name
  back to it.
- **`BulletinURL`** — the address the bulletin directory is served from. The
  game knows where it writes the files and nothing about how your web server
  reaches them, so this is the one thing it cannot work out for itself. It is
  what makes `{{pageurl}}` work.

See [Your board's own settings](inter-bbs.md#your-boards-own-settings-bbscfg)
for the rest of the file.
