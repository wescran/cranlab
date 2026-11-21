package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/wescran/cranlab/internal/pds"
	"github.com/wescran/cranlab/internal/pds/pdstest"
	"github.com/wescran/cranlab/internal/store"
)

const (
	testDID         = "did:plc:ogl52fc2rdooori2ayf74can"
	testPublication = "at://did:plc:ogl52fc2rdooori2ayf74can/site.standard.publication/3mxcsltpg2k26"
	catCID          = "bafkreicatcatcatcatcat"
	htmlCID         = "bafkreihtmlhtmlhtml"
	thumbCID        = "bafkreithumbthumb"
)

type fixture struct {
	pds     *pdstest.Server
	handler http.Handler
	store   *store.Store
}

// newFixture serves the site from a fake PDS holding:
//   - articles "welcome" (2026-09-28, tag writing, announced as post
//     3mwelcome) and "homelab" (2026-10-15, tags homelab and writing), plus a
//     document from another publication;
//   - posts: the announcement 3mwelcome, a reply 3mreply, and 25 notes
//     3n0001…3n0025 (newer has the larger key). 3n0025 has a link facet and an
//     image; 3n0024 has an escaped-text check and a link card;
//   - blobs: a PNG (catCID) and an HTML file (htmlCID).
func newFixture(t *testing.T) *fixture {
	t.Helper()
	fake := pdstest.New(t, testDID)

	fake.Put(pds.DocumentCollection, "3mwelcome", map[string]any{
		"$type": "site.standard.document", "site": testPublication, "path": "/articles/welcome",
		"title": "Welcome", "description": "Hello there.", "publishedAt": "2026-09-28T00:00:00.000Z",
		"tags": []string{"writing"}, "textContent": "Hi. A cat.",
		"content": map[string]any{
			"$type": "at.markpub.markdown", "flavor": "gfm",
			"text": map[string]any{"$type": "at.markpub.text", "markdown": "## Hi\n\n![a cat](https://weslandia.ca/xrpc/com.atproto.sync.getBlob?did=did%3Aplc%3Aogl52fc2rdooori2ayf74can&cid=" + catCID + ")\n"},
		},
		"bskyPostRef": map[string]string{"uri": "at://" + testDID + "/app.bsky.feed.post/3mwelcome", "cid": "bafyreipost"},
	})
	fake.Put(pds.DocumentCollection, "3mhomelab", map[string]any{
		"$type": "site.standard.document", "site": testPublication, "path": "/articles/homelab",
		"title": "My Homelab", "publishedAt": "2026-10-15T12:00:00.000Z",
		"tags": []string{"homelab", "writing"}, "textContent": "Racks and cables.",
		"content": map[string]any{
			"$type": "at.markpub.markdown", "flavor": "gfm",
			"text": map[string]any{"$type": "at.markpub.text", "markdown": "Racks and cables."},
		},
	})
	fake.Put(pds.DocumentCollection, "3mforeign", map[string]any{
		"site": "at://did:plc:someone/site.standard.publication/3mx", "path": "/articles/foreign",
		"title": "Not Mine", "publishedAt": "2026-10-20T00:00:00Z",
	})

	fake.Put(pds.PostCollection, "3mwelcome", map[string]any{"text": "New article: Welcome", "createdAt": "2026-09-28T00:01:00Z"})
	fake.Put(pds.PostCollection, "3mreply", map[string]any{
		"text": "A reply to someone", "createdAt": "2026-10-30T00:00:00Z",
		"reply": map[string]any{"root": map[string]string{"uri": "at://x/app.bsky.feed.post/1", "cid": "c"}, "parent": map[string]string{"uri": "at://x/app.bsky.feed.post/1", "cid": "c"}},
	})
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	for i := 1; i <= 25; i++ {
		note := map[string]any{"text": fmt.Sprintf("Note number %d", i), "createdAt": start.Add(time.Duration(i) * time.Hour).Format(time.RFC3339)}
		switch i {
		case 25:
			note["text"] = "Reading go.dev today"
			note["facets"] = []any{map[string]any{
				"index":    map[string]int{"byteStart": 8, "byteEnd": 14},
				"features": []any{map[string]string{"$type": "app.bsky.richtext.facet#link", "uri": "https://go.dev"}},
			}}
			note["embed"] = map[string]any{"$type": "app.bsky.embed.images", "images": []any{map[string]any{
				"alt": "a cat", "image": map[string]any{"$type": "blob", "ref": map[string]string{"$link": catCID}, "mimeType": "image/png", "size": 3},
			}}}
		case 24:
			note["text"] = "<b>escape me</b>"
			note["embed"] = map[string]any{"$type": "app.bsky.embed.external", "external": map[string]any{
				"uri": "https://example.com/", "title": "<script>alert(1)</script>", "description": "A page.",
				"thumb": map[string]any{"$type": "blob", "ref": map[string]string{"$link": thumbCID}, "mimeType": "image/jpeg", "size": 3},
			}}
		}
		fake.Put(pds.PostCollection, fmt.Sprintf("3n%04d", i), note)
	}

	fake.PutBlob(catCID, "image/png", []byte("png bytes"))
	fake.PutBlob(htmlCID, "text/html", []byte("<script>alert(1)</script>"))

	return startFixture(t, fake, true)
}

func startFixture(t *testing.T, fake *pdstest.Server, expectRefresh bool) *fixture {
	t.Helper()
	cfg := testConfig()
	client := pds.New(fake.URL, fake.Client())
	s, err := store.Open(filepath.Join(t.TempDir(), "cache.db"), cfg.DID, cfg.PublicationURI)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.RefreshArticles(t.Context(), client); (err == nil) != expectRefresh {
		t.Fatalf("startup refresh: %v", err)
	}
	h, err := New(cfg, Deps{
		Store: s, PDS: client,
		Now: func() time.Time { return time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC) },
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return &fixture{pds: fake, handler: h, store: s}
}
