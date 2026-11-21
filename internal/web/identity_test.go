package web

import (
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/pds/pdstest"
	"golang.org/x/net/html"
	"willnorris.com/go/microformats"
)

func TestArticleMicroformats(t *testing.T) {
	f := newFixture(t)
	_, body := f.get(t, "/articles")
	feed := findItem(parseMF(t, body), "h-feed")
	if feed == nil || len(feed.Children) != 2 {
		t.Fatalf("Article feed: %+v", feed)
	}
	summary := feed.Children[0]
	if !slices.Contains(summary.Type, "h-entry") || property(summary, "name") != "My Homelab" || property(summary, "summary") != "Racks and cables." || property(summary, "url") != "https://cranlab.ca/articles/homelab" {
		t.Fatalf("summary entry: %+v", summary)
	}
	_, body = f.get(t, "/articles/welcome")
	data := parseMF(t, body)
	entry := findItem(data, "h-entry")
	if entry == nil || property(entry, "name") != "Welcome" || property(entry, "url") != "https://cranlab.ca/articles/welcome" || !strings.Contains(property(entry, "content"), "Hi") || property(entry, "syndication") != "https://bsky.app/profile/"+testDID+"/post/3mwelcome" {
		t.Fatalf("Article entry: %+v", entry)
	}
	authors := entry.Properties["author"]
	if len(authors) != 1 {
		t.Fatalf("authors: %+v", authors)
	}
	author, ok := authors[0].(*microformats.Microformat)
	if !ok || !slices.Contains(author.Type, "h-card") || property(author, "name") != "Wesley Cranston" || property(author, "url") != "https://cranlab.ca/" {
		t.Errorf("author: %+v", author)
	}
	if !slices.Contains(data.Rels["site.standard.document"], "at://"+testDID+"/site.standard.document/3mwelcome") {
		t.Errorf("record link: %v", data.Rels["site.standard.document"])
	}
}

func TestEmptyArticleMicroformatFeed(t *testing.T) {
	f := startFixture(t, pdstest.New(t, testDID), true)
	_, body := f.get(t, "/articles")
	feed := findItem(parseMF(t, body), "h-feed")
	if feed == nil || len(feed.Children) != 0 {
		t.Fatalf("expected empty h-feed: %+v", feed)
	}
}

func TestSocialPreview(t *testing.T) {
	f := newFixture(t)
	for _, tc := range []struct{ path, title, kind, description, imagePrefix string }{
		{"/articles/homelab", "My Homelab", "article", "Racks and cables.", "/static/images/card-default.png?v="},
		{"/about", "About Me", "website", "About Wesley Cranston", "/static/images/card-default.png?v="},
		{"/articles/welcome", "Welcome", "article", "Hello there.", "/media/" + catCID},
	} {
		t.Run(tc.path, func(t *testing.T) {
			target := "http://preview.test" + tc.path
			_, body := f.get(t, target)
			doc, err := html.Parse(strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			base, _ := url.Parse(target)
			rels := microformats.ParseNode(doc, base).Rels
			const site = "https://cranlab.ca"
			if !slices.Contains(rels["canonical"], site+tc.path) || !slices.Contains(rels["alternate"], site+"/index.xml") {
				t.Errorf("canonical: %v; alternate: %v", rels["canonical"], rels["alternate"])
			}
			meta := map[string]string{}
			for node := range doc.Descendants() {
				if node.Type != html.ElementNode || node.Data != "meta" || node.Parent.Data != "head" {
					continue
				}
				var prop, content string
				for _, a := range node.Attr {
					switch a.Key {
					case "property":
						prop = a.Val
					case "content":
						content = a.Val
					}
				}
				meta[prop] = content
			}
			for key, want := range map[string]string{"og:url": site + tc.path, "og:type": tc.kind, "og:title": tc.title, "og:description": tc.description} {
				if meta[key] != want {
					t.Errorf("%s=%q, want %q", key, meta[key], want)
				}
			}
			if !strings.HasPrefix(meta["og:image"], site+tc.imagePrefix) {
				t.Errorf("og:image=%q", meta["og:image"])
			}
		})
	}
}

func TestWellKnownPublication(t *testing.T) {
	cfg := testConfig()
	cfg.PublicationURI = "at://did:plc:example/site.standard.publication/local"
	handler, err := New(cfg, Deps{})
	if err != nil {
		t.Fatal(err)
	}
	rec := get(t, handler, "/.well-known/site.standard.publication")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "text/plain; charset=utf-8" || rec.Body.String() != cfg.PublicationURI {
		t.Fatalf("publication: %d %s %q", rec.Code, rec.Header(), rec.Body.String())
	}
}
