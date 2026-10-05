package docsite

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSummarizeStripsTheReadMoreTail: the feed ends most blurbs with an
// invitation and one or more bracketed links to the item's own page. The
// headline above is already a link there, so the whole tail comes off — and the
// author's line breaks survive it, because nothing in this feed is hard-wrapped.
func TestSummarizeStripsTheReadMoreTail(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"invitation and link",
			"This is the fourth release candidate.\n\nWould you like to know more?\n[https://tinyurl.com/syncterm-19rc4]",
			"This is the fourth release candidate.",
		},
		{
			"several trailing links",
			"See below!\n\n[https://x-bit.org/spitfire/]\n[https://x-bit.org/spitfire/blog/]",
			"See below!",
		},
		{
			"line breaks are the author's and are kept",
			"TLDR:\nSpitfire works again.\n\n1) Y2K fixes\n2) A tosser",
			"TLDR:\nSpitfire works again.\n\n1) Y2K fixes\n2) A tosser",
		},
		{
			"runs of spaces and blank lines are tidied",
			"  Lots   of  space \n\n\n\n and gaps  ",
			"Lots of space\n\nand gaps",
		},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := summarize(c.in); got != c.want {
			t.Errorf("%s: summarize(%q)\n got %q\nwant %q", c.name, c.in, got, c.want)
		}
	}
}

// TestSummaryKeepsABracketedURLMidText checks only a TRAILING read-more link is
// removed — a URL the publisher wrote into the middle of a sentence is theirs to
// keep.
func TestSummaryKeepsABracketedURLMidText(t *testing.T) {
	in := "See [https://example.org/a] for the patch, then reboot."
	if got := summarize(in); got != in {
		t.Errorf("summarize stripped mid-text content:\n got %q\nwant %q", got, in)
	}
}

// TestNewsPageShowsTheBlurb renders the page and checks each entry carries the
// headline, its date and the feed's own text, with the text escaped — it is
// somebody else's, and it reaches the page as HTML.
func TestNewsPageShowsTheBlurb(t *testing.T) {
	n := news{src: xNews, items: []newsItem{{
		date:    "05 Aug 2026",
		title:   "SyncTERM v1.9rc4 released",
		link:    "https://x-bit.org/a",
		summary: `The fourth candidate for <b>1.9</b> & the last.`,
	}}}
	out := newsPageMarkdown(n)
	for _, want := range []string{
		`<a class="ib-newslist__title" href="https://x-bit.org/a">SyncTERM v1.9rc4 released</a>`,
		`<p class="ib-newslist__date">05 Aug 2026</p>`,
		"The fourth candidate for &lt;b&gt;1.9&lt;/b&gt; &amp; the last.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("news page missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "<b>1.9</b>") {
		t.Error("the feed's markup reached the page unescaped")
	}
}

// TestNewsPageWithoutABlurb checks an item whose feed entry carried no text
// still renders — headline and date, no empty paragraph.
func TestNewsPageWithoutABlurb(t *testing.T) {
	n := news{src: xNews, items: []newsItem{{date: "05 Aug 2026", title: "Quiet item", link: "https://x-bit.org/b"}}}
	out := newsPageMarkdown(n)
	if !strings.Contains(out, "Quiet item") {
		t.Fatalf("headline missing:\n%s", out)
	}
	if strings.Contains(out, `class="ib-newslist__summary"`) {
		t.Errorf("an item with no text rendered an empty blurb:\n%s", out)
	}
}

// TestSummarizeKeepsALinkItsSentenceNeeds: some authors write the link into a
// sentence. Removing it there leaves the blurb hanging on a colon, so the tail
// stays — the repetition is the lesser problem.
func TestSummarizeKeepsALinkItsSentenceNeeds(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{
			"lead-in sentence keeps its link",
			"Runs on DOS, Win32 and the Pi. For more info and to download visit:\n\n[https://example.org/a]",
			"Runs on DOS, Win32 and the Pi. For more info and to download visit:\n\n[https://example.org/a]",
		},
		{
			"finished prose loses the tail",
			"Runs on DOS, Win32 and the Pi.\n\nWould you like to know more?\n[https://example.org/a]",
			"Runs on DOS, Win32 and the Pi.",
		},
		{
			"a blurb that is nothing but a link keeps it",
			"[https://example.org/x]",
			"[https://example.org/x]",
		},
	}
	for _, c := range cases {
		if got := summarize(c.in); got != c.want {
			t.Errorf("%s: summarize(%q)\n got %q\nwant %q", c.name, c.in, got, c.want)
		}
	}
}

// TestFeedFetchNamesItself: the build sends its own user agent, which is what a
// host is asked to allow, and a feed without blurbs keeps only the headline.
func TestFeedFetchNamesItself(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		fmt.Fprint(w, `<rss><channel><item><title>New thread</title><link>https://example.org/t/1</link>`+
			`<pubDate>Sun, 04 Oct 2026 12:00:00 +0000</pubDate><description>&lt;p&gt;A long post&lt;/p&gt;</description></item></channel></rss>`)
	}))
	defer srv.Close()

	n := loadNews(feedSource{name: "Forum", feedURL: srv.URL}, false)
	if gotUA != feedUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, feedUserAgent)
	}
	if len(n.items) != 1 || n.items[0].title != "New thread" || n.items[0].date != "2026-10-04" {
		t.Fatalf("items = %+v", n.items)
	}
	if n.items[0].summary != "" {
		t.Errorf("a feed without blurbs kept the post text: %q", n.items[0].summary)
	}
}

// TestSidebarHasABlockPerFeed: each feed with headlines gets its own block,
// linking to its own page; a feed with none draws nothing.
func TestSidebarHasABlockPerFeed(t *testing.T) {
	item := []newsItem{{date: "2026-10-04", title: "Headline", link: "https://example.org/a"}}
	out := tocOverride([]news{{src: xNews, items: item}, {src: sysopsFinest, items: item}})
	for _, want := range []string{
		`aria-label="` + xNews.alt + `"`, `{{ base_url }}/news/`,
		`aria-label="` + sysopsFinest.alt + `"`, `{{ base_url }}/sysops-finest/`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sidebar missing %q", want)
		}
	}
	if out := tocOverride([]news{{src: xNews}, {src: sysopsFinest, items: item}}); strings.Contains(out, xNews.alt) {
		t.Error("a feed with no headlines drew a block")
	}
}
