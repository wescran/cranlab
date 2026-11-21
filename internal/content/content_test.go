package content

import (
	"reflect"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

const (
	did         = "did:plc:ogl52fc2rdooori2ayf74can"
	publication = "at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26"
)

func doc(rkey, path, published string, tags ...string) pds.Record[pds.Document] {
	return pds.Record[pds.Document]{
		URI:   "at://" + did + "/site.standard.document/" + rkey,
		Value: pds.Document{Site: publication, Path: path, Title: rkey, PublishedAt: published, Tags: tags},
	}
}

func slugs(articles []Article) []string {
	out := []string{}
	for _, a := range articles {
		out = append(out, a.Slug)
	}
	return out
}

func TestArticlesFiltersAndSorts(t *testing.T) {
	other := doc("3mother", "/articles/other-site", "2026-10-05T00:00:00Z")
	other.Value.Site = "at://did:plc:someone/site.standard.publication/3mx"
	docs := []pds.Record[pds.Document]{
		doc("3mold", "/articles/old", "2026-09-01T00:00:00.000Z"),
		doc("3mnew", "/articles/new", "2026-10-20T20:18:27.757Z"),
		other,
		doc("3mnote", "/notes/not-an-article", "2026-10-10T00:00:00Z"),
		doc("3mdeep", "/articles/a/b", "2026-10-10T00:00:00Z"),
		doc("3mempty", "/articles/", "2026-10-10T00:00:00Z"),
		doc("3mmid", "/articles/mid", "2026-10-01T00:00:00Z"),
		doc("3mbad", "/articles/bad-date", "yesterday"),
	}
	got := Articles(docs, publication)
	if want := []string{"new", "mid", "old", "bad-date"}; !reflect.DeepEqual(slugs(got), want) {
		t.Errorf("slugs = %v, want %v (publication only, /articles/<slug> only, newest first, unparsable dates last)", slugs(got), want)
	}
	if got[0].Published.Format("2006-01-02T15:04:05.000") != "2026-10-20T20:18:27.757" {
		t.Errorf("Published = %v", got[0].Published)
	}
	if !got[3].Published.IsZero() {
		t.Errorf("an unparsable date should be zero, got %v", got[3].Published)
	}
}

func TestArticlesTieBreak(t *testing.T) {
	docs := []pds.Record[pds.Document]{
		doc("3mb", "/articles/b", "2026-10-01T00:00:00Z"),
		doc("3ma", "/articles/a", "2026-10-01T00:00:00Z"),
	}
	if got := slugs(Articles(docs, publication)); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Errorf("same date: slugs = %v, want [a b]", got)
	}
}

func TestArticlePermalinkPath(t *testing.T) {
	a := Article{Slug: "café #1%"}
	if got := a.PermalinkPath(); got != "/articles/caf%C3%A9%20%231%25" {
		t.Errorf("unexpected permalink path: %s", got)
	}
}

func TestFindArticle(t *testing.T) {
	articles := Articles([]pds.Record[pds.Document]{doc("3mw", "/articles/welcome", "2026-09-28T00:00:00Z")}, publication)
	if a, ok := FindArticle(articles, "welcome"); !ok || a.Slug != "welcome" {
		t.Errorf("FindArticle(welcome) = %+v, %v", a, ok)
	}
	for _, slug := range []string{"", "Welcome", "welcome/", "missing"} {
		if _, ok := FindArticle(articles, slug); ok {
			t.Errorf("FindArticle(%q) found something", slug)
		}
	}
}

func post(rkey, text string, reply bool) pds.Record[pds.Post] {
	p := pds.Record[pds.Post]{URI: "at://" + did + "/app.bsky.feed.post/" + rkey, Value: pds.Post{Text: text}}
	if reply {
		p.Value.Reply = []byte(`{"root":{},"parent":{}}`)
	}
	return p
}

func TestIsNote(t *testing.T) {
	d := doc("article", "/articles/a", "")
	d.Value.BskyPostRef = &pds.StrongRef{URI: post("linked", "", false).URI}
	ann := Announcements(Articles([]pds.Record[pds.Document]{d}, publication))
	const site = "https://cranlab.ca"
	topLevel := post("nullable", "hi", false)
	topLevel.Value.Reply = []byte("null")
	if !IsNote(topLevel, ann, site) {
		t.Error("null reply is not a reply reference")
	}
	for _, tc := range []struct {
		key, link   string
		reply, note bool
	}{
		{"post", "", false, true}, {"reply", "", true, false}, {"article", "", false, false}, {"linked", "", false, false},
		{"manual", site + "/articles/manual", false, false},
		{"port", "https://CRANLAB.ca:443/articles/a?x=y", false, false},
		{"lookalike", "https://cranlab.ca.example/articles/a", false, true},
		{"http", "http://cranlab.ca/articles/a", false, true},
		{"path", site + "/articles-other/a", false, true},
		{"escaped", site + "/articles%2Fnot-a-route", false, true},
	} {
		p := post(tc.key, "hi", tc.reply)
		if tc.link != "" {
			p.Value.Embed = &pds.Embed{Type: pds.EmbedExternal, External: &pds.External{URI: tc.link}}
		}
		if got := IsNote(p, ann, site); got != tc.note {
			t.Errorf("%s: IsNote = %v", tc.key, got)
		}
	}
}

func TestTags(t *testing.T) {
	articles := Articles([]pds.Record[pds.Document]{
		doc("3m1", "/articles/one", "2026-10-03T00:00:00Z", "homelab", " Go ", "GO", " "),
		doc("3m2", "/articles/two", "2026-10-02T00:00:00Z", "writing", "homelab"),
		doc("3m3", "/articles/three", "2026-10-01T00:00:00Z", "atproto"),
	}, publication)
	want := []TagCount{{"atproto", 1}, {"go", 1}, {"homelab", 2}, {"writing", 1}}
	if got := Tags(articles); !reflect.DeepEqual(got, want) {
		t.Errorf("Tags = %v, want %v (sorted ignoring case)", got, want)
	}
	if got := slugs(WithTag(articles, "homelab")); !reflect.DeepEqual(got, []string{"one", "two"}) {
		t.Errorf("WithTag(homelab) = %v", got)
	}
	if got := slugs(WithTag(articles, "go")); !reflect.DeepEqual(got, []string{"one"}) {
		t.Errorf("WithTag(go) = %v", got)
	}
	if got := Tags(nil); len(got) != 0 {
		t.Errorf("Tags(nil) = %v", got)
	}
}
