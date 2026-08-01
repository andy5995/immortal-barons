// Package docsite assembles the documentation website's MkDocs source tree from
// the repo's committed Markdown, so the site and the in-game help share one
// source of truth. It is build-time tooling (run by cmd/barons-docs in CI); the
// game does not import it.
//
// Output: <outDir>/site-src/<lang>/** (the mkdocs-static-i18n "folder" layout,
// English default + de/ru overlays) and <outDir>/mkdocs.yml. Missing
// translations fall back to English via the i18n plugin, so a language folder
// need only hold the pages that are actually translated.
package docsite

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/andy5995/immortal-barons/internal/help"
	"github.com/andy5995/immortal-barons/internal/i18n"
)

// extraCSS makes in-content links obviously links, and gives the left-hand nav
// a clear hierarchy: top-level destinations read as headings, the nested Game
// Instructions topics read as a quieter list beneath their category, and the
// page under the pointer gets a small tan tank, tracks rolling. Everything here
// uses Material's own CSS variables, so both palettes (light/slate) follow, and
// nothing is keyed to a viewport width — the same rules serve the desktop
// sidebar and the mobile drawer.
const extraCSS = `/* Underline in-content links so they read as links, not just tinted text. */
.md-typeset a {
  text-decoration: underline;
  text-underline-offset: 0.15em;
}
.md-typeset a:hover {
  text-decoration: underline;
}

/* --- Left-hand navigation ------------------------------------------------ */

/* Top level (Home, FAQ, Player Guide, Door Setup, …): the site's main
   destinations, so give them weight and room to breathe. */
.md-nav--primary > .md-nav__list > .md-nav__item > .md-nav__link {
  font-weight: 700;
  letter-spacing: 0.01em;
  padding-block: 0.25rem;
}

/* The active top-level entry gets a spine on the leading edge, so where you are
   is readable at a glance without adding any color of our own. */
.md-nav--primary > .md-nav__list > .md-nav__item > .md-nav__link--active {
  border-inline-start: 2px solid var(--md-accent-fg-color);
}

/* Nested levels are supporting detail: lighter than their parent, and the
   category headings (Controls, Economy, …) sit between the two. */
.md-nav--primary .md-nav .md-nav__link {
  font-weight: 400;
}
.md-nav--primary .md-nav .md-nav__item--nested > .md-nav__link {
  font-weight: 600;
  color: var(--md-default-fg-color--light);
}

/* A small tank rolls in at the trailing edge of whichever nav page is under
   the pointer. It is a MASKED pseudo-element, not an <img>: the mask carries
   the shape and CSS carries the color, so one asset serves both palettes.
   Absolutely positioned, so it can never reflow the label it decorates; empty
   content and pointer-events:none, so assistive tech and clicks pass through.
   Only <a> links get one — a nested section's toggle already owns that corner
   with its chevron. */
:root {
  --ib-tank: url("data:image/svg+xml,<svg%20xmlns='http://www.w3.org/2000/svg'%20viewBox='0%200%20192%2020'><defs><linearGradient%20id='t'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%2375603f'/><stop%20offset='.55'%20stop-color='%2354422a'/><stop%20offset='1'%20stop-color='%233c2e1c'/></linearGradient><linearGradient%20id='h'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%23cbab77'/><stop%20offset='.45'%20stop-color='%23a5804a'/><stop%20offset='1'%20stop-color='%237a5f35'/></linearGradient><linearGradient%20id='u'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%23d4b482'/><stop%20offset='.5'%20stop-color='%23ad8850'/><stop%20offset='1'%20stop-color='%23836540'/></linearGradient><linearGradient%20id='b'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%23856a42'/><stop%20offset='.32'%20stop-color='%23e8d3a8'/><stop%20offset='.62'%20stop-color='%23ab8752'/><stop%20offset='1'%20stop-color='%2363512f'/></linearGradient></defs><path%20fill='url%28%23t%29'%20fill-rule='evenodd'%20d='M4.25%2013.5H27.75A2.75%202.75%200%201%201%2027.75%2019H4.25A2.75%202.75%200%201%201%204.25%2013.5ZM4.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM9.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM14.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM19.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM24.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM0.0%2017.7h1.15v1.9h-1.15ZM3.0%2017.7h1.15v1.9h-1.15ZM6.0%2017.7h1.15v1.9h-1.15ZM9.0%2017.7h1.15v1.9h-1.15ZM12.0%2017.7h1.15v1.9h-1.15ZM15.0%2017.7h1.15v1.9h-1.15ZM18.0%2017.7h1.15v1.9h-1.15ZM21.0%2017.7h1.15v1.9h-1.15ZM24.0%2017.7h1.15v1.9h-1.15ZM27.0%2017.7h1.15v1.9h-1.15ZM30.0%2017.7h1.15v1.9h-1.15Z'/><path%20fill='url%28%23h%29'%20d='M3%2013V9.8h3.2l2.2-2.4h13.6l2.2%202.4H29V13Z'/><path%20fill='%23ecd9b0'%20opacity='.45'%20d='M6.2%209.8l2.2-2.4h13.6l.6.65H8.9l-1.9%201.75Z'/><path%20fill='%233c2e1c'%20opacity='.5'%20d='M3%2012.55h26v.45h-26Z'/><path%20fill='url%28%23u%29'%20d='M11%207.4V4.8q0-1%201-1h6q1%200%201%201v2.6Z'/><path%20fill='%23eddcbc'%20opacity='.45'%20d='M11.6%207.4V4.9q0-.55.55-.55h.7V7.4Z'/><path%20fill='url%28%23u%29'%20d='M13.1%202.4h2.6a.6.6%200%200%201%20.6.6v.8h-3.8V3a.6.6%200%200%201%20.6-.6Z'/><path%20fill='%236f5533'%20d='M18.4%204.9h1.5v2.1h-1.5Z'/><path%20fill='url%28%23b%29'%20d='M19.6%205.4h9.1v1.1h-9.1Z'/><path%20fill='url%28%23b%29'%20d='M28.2%204.8h2.3v2.3h-2.3Z'/><path%20fill='url%28%23t%29'%20fill-rule='evenodd'%20d='M36.25%2013.5H59.75A2.75%202.75%200%201%201%2059.75%2019H36.25A2.75%202.75%200%201%201%2036.25%2013.5ZM36.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM41.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM46.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM51.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM56.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM32.5%2017.7h1.15v1.9h-1.15ZM35.5%2017.7h1.15v1.9h-1.15ZM38.5%2017.7h1.15v1.9h-1.15ZM41.5%2017.7h1.15v1.9h-1.15ZM44.5%2017.7h1.15v1.9h-1.15ZM47.5%2017.7h1.15v1.9h-1.15ZM50.5%2017.7h1.15v1.9h-1.15ZM53.5%2017.7h1.15v1.9h-1.15ZM56.5%2017.7h1.15v1.9h-1.15ZM59.5%2017.7h1.15v1.9h-1.15Z'/><path%20fill='url%28%23h%29'%20d='M35%2013V9.8h3.2l2.2-2.4h13.6l2.2%202.4H61V13Z'/><path%20fill='%23ecd9b0'%20opacity='.45'%20d='M38.2%209.8l2.2-2.4h13.6l.6.65H40.9l-1.9%201.75Z'/><path%20fill='%233c2e1c'%20opacity='.5'%20d='M35%2012.55h26v.45h-26Z'/><path%20fill='url%28%23u%29'%20d='M43%207.4V4.8q0-1%201-1h6q1%200%201%201v2.6Z'/><path%20fill='%23eddcbc'%20opacity='.45'%20d='M43.6%207.4V4.9q0-.55.55-.55h.7V7.4Z'/><path%20fill='url%28%23u%29'%20d='M45.1%202.4h2.6a.6.6%200%200%201%20.6.6v.8h-3.8V3a.6.6%200%200%201%20.6-.6Z'/><path%20fill='%236f5533'%20d='M50.4%204.9h1.5v2.1h-1.5Z'/><path%20fill='url%28%23b%29'%20d='M51.6%205.4h9.1v1.1h-9.1Z'/><path%20fill='url%28%23b%29'%20d='M60.2%204.8h2.3v2.3h-2.3Z'/><path%20fill='url%28%23t%29'%20fill-rule='evenodd'%20d='M68.25%2013.5H91.75A2.75%202.75%200%201%201%2091.75%2019H68.25A2.75%202.75%200%201%201%2068.25%2013.5ZM68.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM73.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM78.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM83.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM88.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM65.0%2017.7h1.15v1.9h-1.15ZM68.0%2017.7h1.15v1.9h-1.15ZM71.0%2017.7h1.15v1.9h-1.15ZM74.0%2017.7h1.15v1.9h-1.15ZM77.0%2017.7h1.15v1.9h-1.15ZM80.0%2017.7h1.15v1.9h-1.15ZM83.0%2017.7h1.15v1.9h-1.15ZM86.0%2017.7h1.15v1.9h-1.15ZM89.0%2017.7h1.15v1.9h-1.15ZM92.0%2017.7h1.15v1.9h-1.15Z'/><path%20fill='url%28%23h%29'%20d='M67%2013V9.8h3.2l2.2-2.4h13.6l2.2%202.4H93V13Z'/><path%20fill='%23ecd9b0'%20opacity='.45'%20d='M70.2%209.8l2.2-2.4h13.6l.6.65H72.9l-1.9%201.75Z'/><path%20fill='%233c2e1c'%20opacity='.5'%20d='M67%2012.55h26v.45h-26Z'/><path%20fill='url%28%23u%29'%20d='M75%207.4V4.8q0-1%201-1h6q1%200%201%201v2.6Z'/><path%20fill='%23eddcbc'%20opacity='.45'%20d='M75.6%207.4V4.9q0-.55.55-.55h.7V7.4Z'/><path%20fill='url%28%23u%29'%20d='M77.1%202.4h2.6a.6.6%200%200%201%20.6.6v.8h-3.8V3a.6.6%200%200%201%20.6-.6Z'/><path%20fill='%236f5533'%20d='M82.4%204.9h1.5v2.1h-1.5Z'/><path%20fill='url%28%23b%29'%20d='M83.6%205.4h9.1v1.1h-9.1Z'/><path%20fill='url%28%23b%29'%20d='M92.2%204.8h2.3v2.3h-2.3Z'/><path%20fill='url%28%23t%29'%20fill-rule='evenodd'%20d='M100.25%2013.5H123.75A2.75%202.75%200%201%201%20123.75%2019H100.25A2.75%202.75%200%201%201%20100.25%2013.5ZM100.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM105.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM110.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM115.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM120.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM97.5%2017.7h1.15v1.9h-1.15ZM100.5%2017.7h1.15v1.9h-1.15ZM103.5%2017.7h1.15v1.9h-1.15ZM106.5%2017.7h1.15v1.9h-1.15ZM109.5%2017.7h1.15v1.9h-1.15ZM112.5%2017.7h1.15v1.9h-1.15ZM115.5%2017.7h1.15v1.9h-1.15ZM118.5%2017.7h1.15v1.9h-1.15ZM121.5%2017.7h1.15v1.9h-1.15ZM124.5%2017.7h1.15v1.9h-1.15Z'/><path%20fill='url%28%23h%29'%20d='M99%2013V9.8h3.2l2.2-2.4h13.6l2.2%202.4H125V13Z'/><path%20fill='%23ecd9b0'%20opacity='.45'%20d='M102.2%209.8l2.2-2.4h13.6l.6.65H104.9l-1.9%201.75Z'/><path%20fill='%233c2e1c'%20opacity='.5'%20d='M99%2012.55h26v.45h-26Z'/><path%20fill='url%28%23u%29'%20d='M107%207.4V4.8q0-1%201-1h6q1%200%201%201v2.6Z'/><path%20fill='%23eddcbc'%20opacity='.45'%20d='M107.6%207.4V4.9q0-.55.55-.55h.7V7.4Z'/><path%20fill='url%28%23u%29'%20d='M109.1%202.4h2.6a.6.6%200%200%201%20.6.6v.8h-3.8V3a.6.6%200%200%201%20.6-.6Z'/><path%20fill='%236f5533'%20d='M114.4%204.9h1.5v2.1h-1.5Z'/><path%20fill='url%28%23b%29'%20d='M115.6%205.4h9.1v1.1h-9.1Z'/><path%20fill='url%28%23b%29'%20d='M124.2%204.8h2.3v2.3h-2.3Z'/><path%20fill='url%28%23t%29'%20fill-rule='evenodd'%20d='M132.25%2013.5H155.75A2.75%202.75%200%201%201%20155.75%2019H132.25A2.75%202.75%200%201%201%20132.25%2013.5ZM132.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM137.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM142.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM147.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM152.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM130.0%2017.7h1.15v1.9h-1.15ZM133.0%2017.7h1.15v1.9h-1.15ZM136.0%2017.7h1.15v1.9h-1.15ZM139.0%2017.7h1.15v1.9h-1.15ZM142.0%2017.7h1.15v1.9h-1.15ZM145.0%2017.7h1.15v1.9h-1.15ZM148.0%2017.7h1.15v1.9h-1.15ZM151.0%2017.7h1.15v1.9h-1.15ZM154.0%2017.7h1.15v1.9h-1.15ZM157.0%2017.7h1.15v1.9h-1.15Z'/><path%20fill='url%28%23h%29'%20d='M131%2013V9.8h3.2l2.2-2.4h13.6l2.2%202.4H157V13Z'/><path%20fill='%23ecd9b0'%20opacity='.45'%20d='M134.2%209.8l2.2-2.4h13.6l.6.65H136.9l-1.9%201.75Z'/><path%20fill='%233c2e1c'%20opacity='.5'%20d='M131%2012.55h26v.45h-26Z'/><path%20fill='url%28%23u%29'%20d='M139%207.4V4.8q0-1%201-1h6q1%200%201%201v2.6Z'/><path%20fill='%23eddcbc'%20opacity='.45'%20d='M139.6%207.4V4.9q0-.55.55-.55h.7V7.4Z'/><path%20fill='url%28%23u%29'%20d='M141.1%202.4h2.6a.6.6%200%200%201%20.6.6v.8h-3.8V3a.6.6%200%200%201%20.6-.6Z'/><path%20fill='%236f5533'%20d='M146.4%204.9h1.5v2.1h-1.5Z'/><path%20fill='url%28%23b%29'%20d='M147.6%205.4h9.1v1.1h-9.1Z'/><path%20fill='url%28%23b%29'%20d='M156.2%204.8h2.3v2.3h-2.3Z'/><path%20fill='url%28%23t%29'%20fill-rule='evenodd'%20d='M164.25%2013.5H187.75A2.75%202.75%200%201%201%20187.75%2019H164.25A2.75%202.75%200%201%201%20164.25%2013.5ZM164.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM169.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM174.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM179.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM184.55%2016.25a1.45%201.45%200%201%200%202.9%200a1.45%201.45%200%201%200%20-2.9%200ZM162.5%2017.7h1.15v1.9h-1.15ZM165.5%2017.7h1.15v1.9h-1.15ZM168.5%2017.7h1.15v1.9h-1.15ZM171.5%2017.7h1.15v1.9h-1.15ZM174.5%2017.7h1.15v1.9h-1.15ZM177.5%2017.7h1.15v1.9h-1.15ZM180.5%2017.7h1.15v1.9h-1.15ZM183.5%2017.7h1.15v1.9h-1.15ZM186.5%2017.7h1.15v1.9h-1.15ZM189.5%2017.7h1.15v1.9h-1.15Z'/><path%20fill='url%28%23h%29'%20d='M163%2013V9.8h3.2l2.2-2.4h13.6l2.2%202.4H189V13Z'/><path%20fill='%23ecd9b0'%20opacity='.45'%20d='M166.2%209.8l2.2-2.4h13.6l.6.65H168.9l-1.9%201.75Z'/><path%20fill='%233c2e1c'%20opacity='.5'%20d='M163%2012.55h26v.45h-26Z'/><path%20fill='url%28%23u%29'%20d='M171%207.4V4.8q0-1%201-1h6q1%200%201%201v2.6Z'/><path%20fill='%23eddcbc'%20opacity='.45'%20d='M171.6%207.4V4.9q0-.55.55-.55h.7V7.4Z'/><path%20fill='url%28%23u%29'%20d='M173.1%202.4h2.6a.6.6%200%200%201%20.6.6v.8h-3.8V3a.6.6%200%200%201%20.6-.6Z'/><path%20fill='%236f5533'%20d='M178.4%204.9h1.5v2.1h-1.5Z'/><path%20fill='url%28%23b%29'%20d='M179.6%205.4h9.1v1.1h-9.1Z'/><path%20fill='url%28%23b%29'%20d='M188.2%204.8h2.3v2.3h-2.3Z'/></svg>");
  --ib-jet: url("data:image/svg+xml,<svg%20xmlns='http://www.w3.org/2000/svg'%20viewBox='0%200%20192%2020'><defs><linearGradient%20id='f'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%23cbab77'/><stop%20offset='.42'%20stop-color='%23a5804a'/><stop%20offset='1'%20stop-color='%237a5f35'/></linearGradient><linearGradient%20id='c'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%23e2d0ae'/><stop%20offset='1'%20stop-color='%239c7d4e'/></linearGradient><linearGradient%20id='w'%20x1='0'%20y1='0'%20x2='0'%20y2='1'><stop%20offset='0'%20stop-color='%23b3915c'/><stop%20offset='1'%20stop-color='%236f5533'/></linearGradient><linearGradient%20id='e'%20x1='1'%20y1='0'%20x2='0'%20y2='0'><stop%20offset='0'%20stop-color='%23e8b26a'/><stop%20offset='.5'%20stop-color='%23c67f34'/><stop%20offset='1'%20stop-color='%23a35a1c'%20stop-opacity='0'/></linearGradient></defs><path%20fill='url%28%23e%29'%20d='M5.5%2010.45%202.1%2011.3%205.5%2012.15Z'/><path%20fill='%236f5533'%20d='M5.5%2010.3h1.7v2h-1.7Z'/><path%20fill='url%28%23w%29'%20d='M17.2%2012.7%208.6%2017.5%203.7%2017.5%2012.2%2012.55Z'/><path%20fill='url%28%23w%29'%20d='M6.9%2012.4%204.2%2014.6%208.4%2014.4%2010.2%2012.6Z'/><path%20fill='url%28%23f%29'%20d='M6.6%209.5%205.1%204.2%209.4%205.05%2010.3%209.1Z'/><path%20fill='url%28%23f%29'%20d='M30.5%2010.8L22%208.75%2014%208.6%207%209.5V12.5L16%2013.2%2024%2012.75Z'/><path%20fill='%233c2e1c'%20opacity='.35'%20d='M7%2012.05l9%20.75%208-.45%206.5-1.55v.03L24%2012.75%2016%2013.2%207%2012.5Z'/><path%20fill='url%28%23c%29'%20d='M19.6%208.65q1.6-2.3%204.3-1.15l.15%201.5Z'/><path%20fill='url%28%23e%29'%20d='M37.5%2010.45%2032.7%2011.3%2037.5%2012.15Z'/><path%20fill='%236f5533'%20d='M37.5%2010.3h1.7v2h-1.7Z'/><path%20fill='url%28%23w%29'%20d='M49.2%2012.7%2040.6%2017.5%2035.7%2017.5%2044.2%2012.55Z'/><path%20fill='url%28%23w%29'%20d='M38.9%2012.4%2036.2%2014.6%2040.4%2014.4%2042.2%2012.6Z'/><path%20fill='url%28%23f%29'%20d='M38.6%209.5%2037.1%204.2%2041.4%205.05%2042.3%209.1Z'/><path%20fill='url%28%23f%29'%20d='M62.5%2010.8L54%208.75%2046%208.6%2039%209.5V12.5L48%2013.2%2056%2012.75Z'/><path%20fill='%233c2e1c'%20opacity='.35'%20d='M39%2012.05l9%20.75%208-.45%206.5-1.55v.03L56%2012.75%2048%2013.2%2039%2012.5Z'/><path%20fill='url%28%23c%29'%20d='M51.6%208.65q1.6-2.3%204.3-1.15l.15%201.5Z'/><path%20fill='url%28%23e%29'%20d='M69.5%2010.45%2065.7%2011.3%2069.5%2012.15Z'/><path%20fill='%236f5533'%20d='M69.5%2010.3h1.7v2h-1.7Z'/><path%20fill='url%28%23w%29'%20d='M81.2%2012.7%2072.6%2017.5%2067.7%2017.5%2076.2%2012.55Z'/><path%20fill='url%28%23w%29'%20d='M70.9%2012.4%2068.2%2014.6%2072.4%2014.4%2074.2%2012.6Z'/><path%20fill='url%28%23f%29'%20d='M70.6%209.5%2069.1%204.2%2073.4%205.05%2074.3%209.1Z'/><path%20fill='url%28%23f%29'%20d='M94.5%2010.8L86%208.75%2078%208.6%2071%209.5V12.5L80%2013.2%2088%2012.75Z'/><path%20fill='%233c2e1c'%20opacity='.35'%20d='M71%2012.05l9%20.75%208-.45%206.5-1.55v.03L88%2012.75%2080%2013.2%2071%2012.5Z'/><path%20fill='url%28%23c%29'%20d='M83.6%208.65q1.6-2.3%204.3-1.15l.15%201.5Z'/><path%20fill='url%28%23e%29'%20d='M101.5%2010.45%2096.3%2011.3%20101.5%2012.15Z'/><path%20fill='%236f5533'%20d='M101.5%2010.3h1.7v2h-1.7Z'/><path%20fill='url%28%23w%29'%20d='M113.2%2012.7%20104.6%2017.5%2099.7%2017.5%20108.2%2012.55Z'/><path%20fill='url%28%23w%29'%20d='M102.9%2012.4%20100.2%2014.6%20104.4%2014.4%20106.2%2012.6Z'/><path%20fill='url%28%23f%29'%20d='M102.6%209.5%20101.1%204.2%20105.4%205.05%20106.3%209.1Z'/><path%20fill='url%28%23f%29'%20d='M126.5%2010.8L118%208.75%20110%208.6%20103%209.5V12.5L112%2013.2%20120%2012.75Z'/><path%20fill='%233c2e1c'%20opacity='.35'%20d='M103%2012.05l9%20.75%208-.45%206.5-1.55v.03L120%2012.75%20112%2013.2%20103%2012.5Z'/><path%20fill='url%28%23c%29'%20d='M115.6%208.65q1.6-2.3%204.3-1.15l.15%201.5Z'/><path%20fill='url%28%23e%29'%20d='M133.5%2010.45%20129.9%2011.3%20133.5%2012.15Z'/><path%20fill='%236f5533'%20d='M133.5%2010.3h1.7v2h-1.7Z'/><path%20fill='url%28%23w%29'%20d='M145.2%2012.7%20136.6%2017.5%20131.7%2017.5%20140.2%2012.55Z'/><path%20fill='url%28%23w%29'%20d='M134.9%2012.4%20132.2%2014.6%20136.4%2014.4%20138.2%2012.6Z'/><path%20fill='url%28%23f%29'%20d='M134.6%209.5%20133.1%204.2%20137.4%205.05%20138.3%209.1Z'/><path%20fill='url%28%23f%29'%20d='M158.5%2010.8L150%208.75%20142%208.6%20135%209.5V12.5L144%2013.2%20152%2012.75Z'/><path%20fill='%233c2e1c'%20opacity='.35'%20d='M135%2012.05l9%20.75%208-.45%206.5-1.55v.03L152%2012.75%20144%2013.2%20135%2012.5Z'/><path%20fill='url%28%23c%29'%20d='M147.6%208.65q1.6-2.3%204.3-1.15l.15%201.5Z'/><path%20fill='url%28%23e%29'%20d='M165.5%2010.45%20161.1%2011.3%20165.5%2012.15Z'/><path%20fill='%236f5533'%20d='M165.5%2010.3h1.7v2h-1.7Z'/><path%20fill='url%28%23w%29'%20d='M177.2%2012.7%20168.6%2017.5%20163.7%2017.5%20172.2%2012.55Z'/><path%20fill='url%28%23w%29'%20d='M166.9%2012.4%20164.2%2014.6%20168.4%2014.4%20170.2%2012.6Z'/><path%20fill='url%28%23f%29'%20d='M166.6%209.5%20165.1%204.2%20169.4%205.05%20170.3%209.1Z'/><path%20fill='url%28%23f%29'%20d='M190.5%2010.8L182%208.75%20174%208.6%20167%209.5V12.5L176%2013.2%20184%2012.75Z'/><path%20fill='%233c2e1c'%20opacity='.35'%20d='M167%2012.05l9%20.75%208-.45%206.5-1.55v.03L184%2012.75%20176%2013.2%20167%2012.5Z'/><path%20fill='url%28%23c%29'%20d='M179.6%208.65q1.6-2.3%204.3-1.15l.15%201.5Z'/></svg>");
  /* Which vehicle, and how big. Overridden per nav depth below; the ::after
     inherits these from the link it decorates, so one rule set serves both. */
  --ib-art: var(--ib-tank);
  --ib-art-w: 1.84rem;    /* one sprite frame == the element's width */
  --ib-art-h: 1.15rem;
  --ib-frames: 6;

  /* Material draws the drawer's page-contents toggle with a different glyph
     from the chevron on a nested section, so two visually identical rows offer
     two different affordances. Both are theme variables, so pointing one at
     the other is the whole fix. */
  --md-toc-icon: var(--md-nav-icon--next);
}

/* Sub-menu items fly a jet instead, at 75% of the tank. */
.md-nav--primary .md-nav .md-nav__link {
  --ib-art: var(--ib-jet);
  --ib-art-w: 1.38rem;
  --ib-art-h: 0.86rem;
}

/* The vehicle parks in a gutter reserved on the leading edge. The gutter is
   PERMANENT (not added on hover) — that is the whole point: the label never
   moves when the tank appears or leaves. Every primary nav link gets it, at
   every depth, so the left edge of the text stays on one vertical line. */
.md-nav--primary .md-nav__link {
  padding-inline-start: 2.2rem;
}

/* Gated on a real pointer: a touch device would otherwise leave the tank stuck
   on the last-tapped item with no way to un-hover it. */
@media (hover: hover) {
  .md-nav--primary .md-nav__link {
    position: relative;
  }
  .md-nav--primary .md-nav__link::after {
    content: "";
    position: absolute;
    inset-inline-start: 0.25rem;
    top: 50%;
    width: var(--ib-art-w);
    height: var(--ib-art-h); /* deliberately taller than the label — it fills the
                        vertical gap between items rather than sitting in the
                        text's own line box */
    opacity: 0;
    transform: translate(-0.35rem, -50%);
    /* A horizontal SPRITE: --ib-tank-frames copies of the tank, each with its
       tread notches advanced a fraction of one tooth. Sized so exactly one
       frame fills the element; the animation steps background-position frame
       by frame, and the tracks appear to roll. Animating the SVG's own innards
       is not an option — an SVG used as a CSS image is rendered statically, so
       SMIL/CSS inside it never runs.

       This is a background-image, NOT a mask: a mask is single-channel alpha,
       so it can only ever paint one flat colour. The shading (gradient-lit
       hull, a cylinder-shaded gun, a contact shadow under the hull) has to
       live in the artwork, which means the colour is baked in rather than
       themed from a CSS variable. Desert tan, darkened one step so the hull,
       turret and gun — the largest areas — each clear WCAG 1.4.11's 3:1
       non-text contrast against BOTH the white and the slate background
       (measured, not eyeballed: 3.3-3.6:1 either way). */
    background-image: var(--ib-art);
    background-repeat: no-repeat;
    background-size: calc(var(--ib-frames) * var(--ib-art-w)) 100%;
    background-position: 0 center;
    transition: opacity 120ms ease, transform 120ms ease;
    pointer-events: none;
  }
  /* Drives in from behind the leading edge, gun forward and tracks rolling — or,
     one level down, a jet with its afterburner flickering. The
     animation is attached on hover only, so it is not burning frames on ~50
     invisible pseudo-elements. */
  .md-nav--primary .md-nav__link:hover::after {
    opacity: 1;
    transform: translate(0, -50%);
    animation: ib-art-roll 2s steps(var(--ib-frames)) infinite;
  }
  /* One keyframe pair serves both vehicles: the distance is expressed in the
     element's own --ib-art-w, which the jet overrides. */
  @keyframes ib-art-roll {
    from { background-position: 0 center; }
    to   { background-position: calc(-1 * var(--ib-frames) * var(--ib-art-w)) center; }
  }
  @media (prefers-reduced-motion: reduce) {
    .md-nav--primary .md-nav__link::after,
    .md-nav--primary .md-nav__link:hover::after {
      transition: opacity 120ms ease;
      transform: translate(0, -50%);
      animation: none; /* a parked tank for anyone who asked for less motion */
    }
  }
}
`

// siteLang is one language column of the site: its code, its endonym (shown in
// the language switcher), and whether it is the default/fallback.
type siteLang struct {
	code    string
	name    string
	isFirst bool
}

// languages are the site's languages in menu order; English is the default and
// the fallback for untranslated pages, followed by every translated language
// from the single source of truth in i18n.Languages.
var languages = func() []siteLang {
	ls := []siteLang{{"en", "English", true}}
	for _, l := range i18n.Languages {
		ls = append(ls, siteLang{l.Code, l.Name, false})
	}
	return ls
}()

// Assemble reads the doc sources under repoRoot and writes the MkDocs source
// tree + mkdocs.yml under outDir (which is cleared first).
func Assemble(repoRoot, outDir string) error {
	siteSrc := filepath.Join(outDir, "site-src")
	if err := os.RemoveAll(outDir); err != nil {
		return err
	}
	if err := os.MkdirAll(siteSrc, 0o755); err != nil {
		return err
	}

	// English help topics define the Guide's structure (categories/order); all
	// languages reuse it, with each language's translated body where present.
	enTopics, err := loadTopics(repoRoot, "en")
	if err != nil {
		return err
	}
	lk := newLinker(repoRoot, enTopics)
	srcRel := func(abs string) string { r, _ := filepath.Rel(repoRoot, abs); return filepath.ToSlash(r) }

	for _, lang := range languages {
		langDir := filepath.Join(siteSrc, lang.code)
		// Home (README) and the two doc pages: only if this language has them.
		for _, pg := range []struct{ src, dst, siteRel string }{
			{homeSource(repoRoot, lang.code), filepath.Join(langDir, "index.md"), "index.md"},
			{guideIntroSource(repoRoot, lang.code), filepath.Join(langDir, "guide", "index.md"), "guide/index.md"},
			{doorSetupSource(repoRoot, lang.code), filepath.Join(langDir, "door-setup", "index.md"), "door-setup/index.md"},
			{charsetSource(repoRoot, lang.code), filepath.Join(langDir, "charset", "index.md"), "charset/index.md"},
			{cmdRefSource(repoRoot, lang.code), filepath.Join(langDir, "command-reference", "index.md"), "command-reference/index.md"},
			{faqSource(repoRoot, lang.code), filepath.Join(langDir, "faq", "index.md"), "faq/index.md"},
			{translatingSource(repoRoot, lang.code), filepath.Join(langDir, "translating", "index.md"), "translating/index.md"},
		} {
			if err := copyIfExists(pg.src, pg.dst, srcRel(pg.src), pg.siteRel, lk); err != nil {
				return err
			}
		}

		// Help topics for this language (may be a partial set; the rest fall
		// back to English).
		topics, err := loadTopics(repoRoot, lang.code)
		if err != nil {
			return err
		}
		for _, t := range topics {
			dst := filepath.Join(langDir, "guide", filepath.FromSlash(t.relPath))
			if err := copyMarkdown(t.absPath, dst, srcRel(t.absPath), "guide/"+t.relPath, lk); err != nil {
				return err
			}
		}
	}

	// Developer docs are English-only.
	if err := copyTree(filepath.Join(repoRoot, "docs", "dev"), filepath.Join(siteSrc, "en", "developers"), repoRoot, "developers", lk); err != nil {
		return err
	}

	// Custom CSS lives in the default-language folder (served at the site root,
	// so all languages pick it up).
	cssPath := filepath.Join(siteSrc, "en", "stylesheets", "extra.css")
	if err := os.MkdirAll(filepath.Dir(cssPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(cssPath, []byte(extraCSS), 0o644); err != nil {
		return err
	}

	nav, err := buildNav(repoRoot, enTopics)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(outDir, "mkdocs.yml"), []byte(mkdocsYAML(nav)), 0o644)
}

// topic is a help article located on disk, with its path relative to the
// language's content root (e.g. "regions/regions.md").
type topic struct {
	help.Topic
	relPath string
	absPath string
}

// contentDir is the on-disk help content root for a language.
func contentDir(repoRoot, lang string) string {
	base := filepath.Join(repoRoot, "internal", "help", "content")
	if lang == "en" {
		return base
	}
	return base + "." + lang
}

// loadTopics walks a language's help content, parsing each topic's frontmatter
// with the same parser the game uses.
func loadTopics(repoRoot, lang string) ([]topic, error) {
	root := contentDir(repoRoot, lang)
	// A language may have its UI translated but not its help pages (the two are
	// separate catalogs). With no content.<lang> dir, return no topics so the
	// language reuses the English structure and bodies — the same fallback an
	// individual untranslated page gets.
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, nil
	}
	var topics []topic
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		topics = append(topics, topic{
			Topic:   help.ParseTopic(string(raw)),
			relPath: filepath.ToSlash(rel),
			absPath: path,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(topics, func(i, j int) bool { return topics[i].relPath < topics[j].relPath })
	return topics, nil
}

// homeSource / guideIntroSource / doorSetupSource resolve a
// doc's file for a language: <base>.md for English, <base>.<lang>.md otherwise.
func homeSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "README.md"), lang)
}
func guideIntroSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "playing.md"), lang)
}
func doorSetupSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "door-setup.md"), lang)
}
func charsetSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "charset.md"), lang)
}

func cmdRefSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "command-reference.md"), lang)
}
func faqSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "faq.md"), lang)
}
func translatingSource(repoRoot, lang string) string {
	return langFile(filepath.Join(repoRoot, "docs", "translating.md"), lang)
}

func langFile(enPath, lang string) string {
	if lang == "en" {
		return enPath
	}
	return strings.TrimSuffix(enPath, ".md") + "." + lang + ".md"
}
