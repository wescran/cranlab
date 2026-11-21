package web

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
)

type feedItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	Description string `xml:"description"`
	Date        string `xml:"pubDate"`
}

func readFeed(t *testing.T, f *fixture) []feedItem {
	t.Helper()
	rec := get(t, f.handler, "http://preview.test/index.xml")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/rss+xml; charset=utf-8" {
		t.Fatalf("feed: %d %s", rec.Code, rec.Header())
	}
	var rss struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Channel struct {
			Link        string     `xml:"link"`
			Title       string     `xml:"title"`
			Description string     `xml:"description"`
			Items       []feedItem `xml:"item"`
		} `xml:"channel"`
	}
	if err := xml.Unmarshal(rec.Body.Bytes(), &rss); err != nil {
		t.Fatal(err)
	}
	if rss.Version != "2.0" || rss.Channel.Link != "https://cranlab.ca/" || rss.Channel.Title != "CranLab" || rss.Channel.Description == "" {
		t.Fatalf("channel: %+v", rss)
	}
	return rss.Channel.Items
}

func TestArticlesRSS(t *testing.T) {
	f := newFixture(t)
	items := readFeed(t, f)
	const link = "https://cranlab.ca/articles/welcome"
	if len(items) != 2 || items[0].Title != "My Homelab" || items[1] != (feedItem{"Welcome", link, link, "Hello there.", "Mon, 28 Sep 2026 00:00:00 GMT"}) {
		t.Fatalf("initial items: %+v", items)
	}
	refresh := func() {
		t.Helper()
		if err := f.store.RefreshArticles(t.Context(), pds.New(f.pds.URL, f.pds.Client())); err != nil {
			t.Fatal(err)
		}
	}
	f.pds.Put(pds.DocumentCollection, "3mwelcome", pds.Document{Site: testPublication, Path: "/articles/welcome", Title: "Tea & <cake>", TextContent: strings.Repeat("猫", 301)})
	refresh()
	items = readFeed(t, f)
	if len(items) != 2 || items[1] != (feedItem{"Tea & <cake>", link, link, strings.Repeat("猫", 300), ""}) {
		t.Fatalf("edited items: %+v", items)
	}
	f.pds.Respond("com.atproto.repo.listRecords", "", 200, `{"records":[]}`)
	refresh()
	if items := readFeed(t, f); len(items) != 0 {
		t.Fatalf("items after unpublish: %+v", items)
	}
}

func TestRSSWithoutSnapshot(t *testing.T) {
	fake := pdstest.New(t, testDID)
	fake.Respond("com.atproto.repo.listRecords", "", 503, `{}`)
	f := startFixture(t, fake, false)
	rec := get(t, f.handler, "/index.xml")
	if rec.Code != 503 || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("no snapshot: %d %v", rec.Code, rec.Header())
	}
}
