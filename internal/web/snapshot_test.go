package web

import (
	"strings"
	"testing"

	"github.com/wescran/cranlab/internal/pds"
)

func TestRefreshKeepsLastGoodArticlesAndTags(t *testing.T) {
	for _, fail := range []string{"later page", "missing records", "foreign owner", "transaction"} {
		t.Run(fail, func(t *testing.T) {
			f := newFixture(t)
			// The first record would change both a title and its Tag index if published.
			f.pds.Put(pds.DocumentCollection, "3mwelcome", pds.Document{Site: testPublication, Path: "/articles/welcome", Title: "Changed title", Tags: []string{"changed"}})
			const list = "com.atproto.repo.listRecords"
			switch fail {
			case "later page":
				f.pds.Respond(list, "3mforeign", 503, `{}`)
			case "missing records":
				f.pds.Respond(list, "", 200, `{}`)
			case "foreign owner":
				f.pds.Respond(list, "", 200, `{"records":[{"uri":"at://did:plc:other/site.standard.document/doc","cid":"cid","value":{"site":"`+testPublication+`","path":"/articles/welcome","title":"Foreign title"}}]}`)
			case "transaction":
				// Two different documents with one public URL violate the unique slug index, after DELETE.
				f.pds.Put(pds.DocumentCollection, "3mduplicate", pds.Document{Site: testPublication, Path: "/articles/welcome"})
			}
			err := f.store.RefreshArticles(t.Context(), pds.New(f.pds.URL, f.pds.Client()))
			if err == nil {
				t.Fatalf("%s: refresh unexpectedly succeeded", fail)
			}
			_, body := f.get(t, "/articles/welcome")
			mustContain(t, body, "Hello there.")
			mustNotContain(t, body, "Changed title")
			_, body = f.get(t, "/tags")
			mustContain(t, body, `writing <span>2</span>`)
			mustNotContain(t, body, "changed")
		})
	}
}

func TestArticleEditUpdatesSummaryBodyAndTags(t *testing.T) {
	f := newFixture(t)
	f.pds.Put(pds.DocumentCollection, "3mwelcome", pds.Document{Site: testPublication, Path: "/articles/welcome", Title: "Edited", PublishedAt: "yesterday", TextContent: "<b>fallback</b> " + strings.Repeat("word ", 200), Tags: []string{" New ", "NEW"}, Content: []byte(`{"$type":"other","text":{"markdown":"<em>untrusted</em>"}}`)})
	if err := f.store.RefreshArticles(t.Context(), pds.New(f.pds.URL, f.pds.Client())); err != nil {
		t.Fatal(err)
	}
	_, body := f.get(t, "/articles")
	mustContain(t, body, "Edited", "2 min read")
	mustNotContain(t, body, "0001-01-01")
	_, body = f.get(t, "/articles/welcome")
	mustContain(t, body, "&lt;b&gt;fallback&lt;/b&gt;", "2 min read")
	mustNotContain(t, body, "<em>untrusted</em>", "0001-01-01")
	_, body = f.get(t, "/tags")
	mustContain(t, body, `new <span>1</span>`, `writing <span>1</span>`)
}

func TestMissingArticleStore(t *testing.T) {
	h, err := New(testConfig(), Deps{})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/articles", "/articles/missing", "/tags", "/tags/missing"} {
		if rec := get(t, h, path); rec.Code != 503 {
			t.Errorf("missing store: GET %s = %d", path, rec.Code)
		}
	}
}

func TestCompleteEmptySnapshot(t *testing.T) {
	f := newFixture(t)
	f.pds.Respond("com.atproto.repo.listRecords", "", 200, `{"records":[]}`)
	if err := f.store.RefreshArticles(t.Context(), pds.New(f.pds.URL, f.pds.Client())); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/articles", "/tags"} {
		code, body := f.get(t, p)
		if code != 200 {
			t.Fatalf("empty %s: %d", p, code)
		}
		mustNotContain(t, body, `href="/articles/welcome"`, `href="/tags/writing"`)
	}
	for _, p := range []string{"/articles/welcome", "/tags/writing"} {
		if code, _ := f.get(t, p); code != 404 {
			t.Fatalf("removed %s: %d", p, code)
		}
	}
}
