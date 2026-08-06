package docsite

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// XBitFeedURL is the BBS-scene news feed rendered on the site's News page, and
// XBitNewsURL is the page it comes from. The site URL is written out here
// rather than read from the feed's own <channel><link>, which points back at
// the XML file instead of at anything a reader would want to open.
const (
	XBitFeedURL = "https://x-bit.org/rss/rss.xml"
	XBitNewsURL = "https://x-bit.org/x-news/"
	// XBitNewsName is the visible label; XBitNewsAlt is the longer form given
	// to assistive technology and to the link's tooltip, where "X-News" alone
	// would not say whose news it is.
	XBitNewsName = "X-News"
	XBitNewsAlt  = "X-bit BBS News Feed"
)

// pageItems is how many headlines the News page lists; sidebarItems is how many
// the right-hand column shows before "All headlines". The sidebar list is kept
// short so it cannot push a long table of contents out of view.
const (
	pageItems    = 20
	sidebarItems = 6
)

// feedTimeout bounds the build's one network call. A docs build must not hang
// because someone else's server is slow.
const feedTimeout = 15 * time.Second

// rssChannel decodes only the fields the site renders. The feed also carries
// the publisher's managingEditor/webMaster addresses, which are deliberately
// not decoded: no third party's contact details belong in this repo's generated
// output or its git history.
type rssChannel struct {
	Channel struct {
		Items []rssItem `xml:"item"`
	} `xml:"channel"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
}

// news is a feed reduced to what the site renders, already validated.
type news struct {
	items []newsItem
}

type newsItem struct {
	date  string
	title string
	link  string
}

// loadNews fetches and validates the feed. A feed that cannot be read is not an
// error: the site builds without headlines rather than failing because someone
// else's server had a bad day. An empty feedURL skips the fetch entirely, which
// is what the tests and an offline build want.
func loadNews(feedURL string) news {
	if feedURL == "" {
		return news{}
	}
	ch, err := fetchFeed(feedURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "barons-docs: news feed unavailable (%v); building without headlines\n", err)
		return news{}
	}
	var out news
	for _, it := range ch.Channel.Items {
		link, ok := safeLink(it.Link)
		title := strings.TrimSpace(it.Title)
		if !ok || title == "" {
			continue
		}
		out.items = append(out.items, newsItem{date: itemDate(it.PubDate), title: title, link: link})
		if len(out.items) == pageItems {
			break
		}
	}
	return out
}

func fetchFeed(feedURL string) (*rssChannel, error) {
	client := &http.Client{Timeout: feedTimeout}
	resp, err := client.Get(feedURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: %s", feedURL, resp.Status)
	}
	// Cap the read: a feed is tens of KB, and this is untrusted input.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var ch rssChannel
	if err := xml.Unmarshal(body, &ch); err != nil {
		return nil, err
	}
	return &ch, nil
}

// newsPageMarkdown renders the News page. With no headlines it still renders,
// pointing at the feed, so the nav entry and its URL stay valid either way.
func newsPageMarkdown(n news) string {
	var b strings.Builder
	b.WriteString("# " + XBitNewsName + "\n\n")
	if len(n.items) == 0 {
		b.WriteString("Headlines are not available right now. Read them at the\n")
		b.WriteString("[X-News feed](" + XBitFeedURL + ").\n")
		return b.String()
	}
	b.WriteString("Headlines from [" + XBitNewsAlt + "](" + XBitNewsURL + "), a feed about\n")
	b.WriteString("BBSes, BBS software, door games and the scene. The list is rebuilt once a\n")
	b.WriteString("day.\n\n")
	for _, it := range n.items {
		b.WriteString("* " + it.date + " — [" + mdEscape(it.title) + "](" + it.link + ")\n")
	}
	return b.String()
}

// tocOverride renders Material's partials/toc.html with the news block appended
// below the table of contents.
//
// Two things make this the right seam. The theme includes this partial from
// base.html (the desktop right-hand column) AND from nav-item.html (the phone
// drawer, where it nests under the current page), so one override serves both
// widths — the secondary sidebar itself is display:none below 76.25em, so
// anything written only for it would be invisible on a phone. And the news
// block sits outside the <nav> that holds the TOC, so it is a separate landmark
// rather than fake entries in the page's own contents.
//
// The copied part is Material's generated toc.html verbatim. It is short and it
// still delegates each entry to the theme's own partials/toc-item.html, so a
// theme update only reaches this file if Material changes toc.html itself.
func tocOverride(n news) string {
	var b strings.Builder
	b.WriteString(`{% set title = lang.t("toc") %}
{% if config.mdx_configs.toc and config.mdx_configs.toc.title %}
  {% set title = config.mdx_configs.toc.title %}
{% endif %}
<nav class="md-nav md-nav--secondary" aria-label="{{ title | e }}">
  {% set toc = page.toc %}
  {% set first = toc | first %}
  {% if first and first.level == 1 %}
    {% set toc = first.children %}
  {% endif %}
  {% if toc %}
    <label class="md-nav__title" for="__toc">
      <span class="md-nav__icon md-icon"></span>
      {{ title }}
    </label>
    <ul class="md-nav__list" data-md-component="toc" data-md-scrollfix>
      {% for toc_item in toc %}
        {% include "partials/toc-item.html" %}
      {% endfor %}
    </ul>
  {% endif %}
</nav>
`)
	if len(n.items) == 0 {
		return b.String()
	}

	b.WriteString(`<nav class="md-nav md-nav--secondary ib-news" aria-label="` + XBitNewsAlt + `">
  <hr class="ib-news__rule">
  <a class="md-nav__title ib-news__title" href="` + XBitNewsURL + `" title="` + XBitNewsAlt + `">` + XBitNewsName + `</a>
  <ul class="md-nav__list">
`)
	for i, it := range n.items {
		if i == sidebarItems {
			break
		}
		b.WriteString(`    <li class="md-nav__item ib-news__item">` + "\n")
		b.WriteString(`      <a class="md-nav__link ib-news__link" href="` + htmlAttr(it.link) + `">` + htmlText(it.title) + `</a>` + "\n")
		b.WriteString(`      <span class="ib-news__date">` + htmlText(it.date) + `</span>` + "\n")
		b.WriteString("    </li>\n")
	}
	b.WriteString(`  </ul>
  <a class="ib-news__more" href="{{ base_url }}/news/">All headlines</a>
</nav>
`)
	return b.String()
}

// itemDate renders an RSS pubDate as an ISO date, or an em dash when it cannot
// be parsed (feeds vary, and a bad date must not cost us the headline).
func itemDate(pubDate string) string {
	pubDate = strings.TrimSpace(pubDate)
	for _, layout := range []string{time.RFC1123Z, time.RFC1123, time.RFC822Z, time.RFC822} {
		if t, err := time.Parse(layout, pubDate); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return "—"
}

// safeLink accepts only http(s) URLs, so a hostile or broken feed cannot put a
// javascript: or data: link on the site.
func safeLink(raw string) (string, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", false
	}
	return u.String(), true
}

// htmlText escapes feed text for an HTML body. The braces matter beyond the
// usual escaping: this output is a Jinja template, so a "{{" or "{%" arriving in
// a headline would otherwise be executed by the template engine.
func htmlText(s string) string {
	e := html.EscapeString(s)
	e = strings.ReplaceAll(e, "{", "&#123;")
	return strings.ReplaceAll(e, "}", "&#125;")
}

// htmlAttr escapes a validated URL for an HTML attribute (same brace rule).
func htmlAttr(s string) string { return htmlText(s) }

// mdEscape neutralises the Markdown punctuation a feed title could carry into
// the generated page.
func mdEscape(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		"`", "\\`",
		"*", `\*`,
		"_", `\_`,
		"[", `\[`,
		"]", `\]`,
		"<", `\<`,
	)
	return r.Replace(strings.TrimSpace(s))
}
