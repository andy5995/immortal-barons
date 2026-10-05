package docsite

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// feedSource is one outside feed the site renders: a page of its headlines, and
// a block of the newest under the table of contents. The page URL (siteURL) is
// written out here rather than read from the feed's own <channel><link>, which
// for X-News points back at the XML file instead of at anything a reader would
// want to open.
type feedSource struct {
	// name is the visible label; alt is the longer form given to assistive
	// technology and to the link's tooltip, where "X-News" alone would not say
	// whose news it is.
	name, alt string
	feedURL   string
	siteURL   string
	// slug is the site path of the feed's own page.
	slug string
	// about follows the link to the source in the page's opening sentence.
	about string
	// blurbs shows each item's text under its headline. A forum feed carries
	// whole posts, in markup, so only a news feed's short blurbs are shown.
	blurbs bool
}

var (
	xNews = feedSource{
		name:    "X-News",
		alt:     "X-bit BBS News Feed",
		feedURL: "https://x-bit.org/rss/rss.xml",
		siteURL: "https://x-bit.org/x-news/",
		slug:    "news",
		about:   "a feed about BBSes, BBS software, door games and the scene",
		blurbs:  true,
	}
	sysopsFinest = feedSource{
		name:    "Sysops Finest",
		alt:     "Sysops Finest BBS community forum",
		feedURL: "https://www.sysops-finest.org/forum/forums/-/index.rss",
		siteURL: "https://www.sysops-finest.org/",
		slug:    "sysops-finest",
		about:   "a forum for BBS sysops; these are its newest threads",
	}
	// feedSources is every feed the site renders, in sidebar order.
	feedSources = []feedSource{xNews, sysopsFinest}
)

// feedUserAgent names this build to the hosts it fetches from, so a sysop
// reading a log or a security plugin can tell who is asking and why. Go's
// default, "Go-http-client/1.1", is the one most scrapers send, and some hosts
// block it outright.
const feedUserAgent = "ImmortalBaronsDocs/1.0 (+https://andy5995.github.io/immortal-barons/)"

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
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

// news is a feed reduced to what the site renders, already validated.
type news struct {
	src   feedSource
	items []newsItem
}

type newsItem struct {
	date    string
	title   string
	link    string
	summary string // the feed's own blurb, shown on the News page only
}

// loadNews fetches and validates one feed. A feed that cannot be read is not an
// error: the site builds without its headlines rather than failing because
// someone else's server had a bad day. offline skips the fetch entirely, which
// is what the tests and an offline build want.
func loadNews(src feedSource, offline bool) news {
	out := news{src: src}
	if offline {
		return out
	}
	ch, err := fetchFeed(src.feedURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "barons-docs: %s feed unavailable (%v); building without its headlines\n", src.name, err)
		noteNetworkOwner()
		return out
	}
	for _, it := range ch.Channel.Items {
		link, ok := safeLink(it.Link)
		title := strings.TrimSpace(it.Title)
		if !ok || title == "" {
			continue
		}
		item := newsItem{date: itemDate(it.PubDate), title: title, link: link}
		if src.blurbs {
			item.summary = summarize(it.Description)
		}
		out.items = append(out.items, item)
		if len(out.items) == pageItems {
			break
		}
	}
	return out
}

// noteNetworkOwner prints which network the build is fetching from, once a
// build, the first time a feed fails. A host that refuses the build can allow
// it by network (ASN), and the answer belongs beside the error that raises the
// question rather than in every green build's log, where nobody reads it.
var noteNetworkOwner = sync.OnceFunc(func() {
	client := &http.Client{Timeout: feedTimeout}
	resp, err := client.Get("https://ipinfo.io/org")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if org := strings.TrimSpace(string(body)); resp.StatusCode == http.StatusOK && org != "" {
		fmt.Fprintf(os.Stderr, "barons-docs: this build fetches from %s\n", org)
	}
})

// readMoreTail matches how the feed ends most of its blurbs: an invitation and
// one or more bracketed links to the item's own page. Both are noise here — the
// headline above the blurb is already a link there.
var (
	// readMoreTail is how the feed ends most blurbs: an optional invitation and
	// one or more bracketed links back to the item's own page. The headline above
	// already links there, so it is repetition on every entry.
	readMoreTail = regexp.MustCompile(`(?is)\s*(would you like to know more\??)?(\s*\[\s*https?://[^\]]*\])+\s*$`)
	blankLines   = regexp.MustCompile(`\n{3,}`)
)

// summarize tidies a feed blurb without rewriting it. Runs of spaces collapse
// and the "read more" tail comes off, but the author's LINE BREAKS are kept:
// nothing in this feed is hard-wrapped (its longest lines run past 200
// characters), so every newline in it was put there on purpose — a lead
// paragraph, a numbered list of what changed. The page renders them with
// white-space: pre-line rather than reflowing the text into one block.
//
// The tail comes off only when what remains still reads as finished prose. Some
// authors write the link INTO a sentence ("For more info and to download
// visit:"), and taking it away there leaves the blurb hanging on a colon, which
// reads like a truncation bug — worse than the repetition it was meant to save.
//
// The result is escaped by the caller: this is somebody else's text.
func summarize(desc string) string {
	full := tidyLines(desc)
	trimmed := tidyLines(readMoreTail.ReplaceAllString(desc, ""))
	if trimmed == "" || strings.HasSuffix(trimmed, ":") {
		return full
	}
	return trimmed
}

// tidyLines collapses runs of spaces within each line and runs of blank lines
// between them, leaving the line structure otherwise as written.
func tidyLines(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.Join(strings.Fields(l), " ")
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

func fetchFeed(feedURL string) (*rssChannel, error) {
	req, err := http.NewRequest(http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", feedUserAgent)
	client := &http.Client{Timeout: feedTimeout}
	resp, err := client.Do(req)
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

// newsPageMarkdown renders a feed's page. With no headlines it still renders,
// pointing at the feed, so the nav entry and its URL stay valid either way.
func newsPageMarkdown(n news) string {
	src := n.src
	var b strings.Builder
	b.WriteString("# " + src.name + "\n\n")
	if len(n.items) == 0 {
		b.WriteString("Headlines are not available right now. Read them at\n")
		b.WriteString("[" + src.alt + "](" + src.siteURL + ").\n")
		return b.String()
	}
	b.WriteString("Headlines from [" + src.alt + "](" + src.siteURL + "), " + src.about + ".\n")
	b.WriteString("The list is rebuilt once a day.\n\n")
	// Written as HTML rather than Markdown bullets: each entry is a headline, a
	// date and the feed's own blurb, and the blurb is somebody else's text that
	// has to be escaped anyway. Escaping it into HTML is one rule instead of two
	// (Markdown's escapes AND the theme's). No headings, so twenty entries do not
	// flood the page's table of contents.
	b.WriteString(`<ul class="ib-newslist">` + "\n")
	for _, it := range n.items {
		b.WriteString(`  <li class="ib-newslist__entry">` + "\n")
		b.WriteString(`    <a class="ib-newslist__title" href="` + htmlAttr(it.link) + `">` + htmlText(it.title) + `</a>` + "\n")
		b.WriteString(`    <p class="ib-newslist__date">` + htmlText(it.date) + `</p>` + "\n")
		if it.summary != "" {
			b.WriteString(`    <p class="ib-newslist__summary">` + htmlText(it.summary) + `</p>` + "\n")
		}
		b.WriteString("  </li>\n")
	}
	b.WriteString("</ul>\n")
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
func tocOverride(feeds []news) string {
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
	for _, n := range feeds {
		writeNewsBlock(&b, n)
	}
	return b.String()
}

// writeNewsBlock is one feed's block under the table of contents. A feed with
// no headlines draws nothing: its page still says where to read them.
func writeNewsBlock(b *strings.Builder, n news) {
	if len(n.items) == 0 {
		return
	}
	src := n.src
	b.WriteString(`<nav class="md-nav md-nav--secondary ib-news" aria-label="` + src.alt + `">
  <hr class="ib-news__rule">
  <a class="md-nav__title ib-news__title" href="` + src.siteURL + `" title="` + src.alt + `">` + src.name + `</a>
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
  <a class="ib-news__more" href="{{ base_url }}/` + src.slug + `/">All headlines</a>
</nav>
`)
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
